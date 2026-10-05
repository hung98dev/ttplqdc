// Package auth implements the 12-route HTTPS contract of auth.md
// § HTTPS Endpoints: password registration/login, federated login
// (Apple/Google/Steam), refresh rotation with reuse detection, logout,
// credential changes, account view/deletion and the gameplay ticket.
package auth

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/edge/session"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Credential TTLs of auth.md § Credential Types.
const (
	AccessTTL         = 15 * time.Minute
	RefreshTTL        = 30 * 24 * time.Hour
	FamilyAbsoluteTTL = 90 * 24 * time.Hour
	// RefreshGraceMs: a presented generation N-1 is accepted once while its
	// successor N was issued ≤60 s ago and N-1 was never presented.
	RefreshGrace = 60 * time.Second
	// ReAuthFreshness: provider credentials used for account.delete must be
	// minted ≤ 5 min before presentation.
	ReAuthFreshness = 5 * time.Minute
	// GuardDuration: takeover rule sets credential_guard_until = now+24 h.
	GuardDuration = 24 * time.Hour
	// TakeoverWindow: a credential change within 1 h of a new-origin login
	// arms the takeover rule.
	TakeoverWindow = time.Hour
	// DeletionWindow: scheduled deletion is deletion_requested_at + 7 d.
	DeletionWindow = 7 * 24 * time.Hour
	// HistoryWindow: is_new_origin rows count within 90 days.
	HistoryWindow = 90 * 24 * time.Hour
)

// LoginMeta is the request-scoped signal input for one auth call.
type LoginMeta struct {
	IP               net.IP
	DeviceID         string // raw client device_id (UUID string)
	Platform         string // WINDOWS | ANDROID
	AppVersion       string
	DeviceModelClass string
}

// TokenResponse is the credential bundle returned by login-family
// endpoints (auth.md § Token Response).
type TokenResponse struct {
	AccountID        id.UUID
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	IsNewAccount     bool
}

// ProviderLink is one row of the providers list.
type ProviderLink struct {
	ProviderID string    `json:"provider_id"`
	LinkedAt   time.Time `json:"linked_at"`
}

// AccountView is the GET /api/v1/account response.
type AccountView struct {
	AccountID           string         `json:"account_id"`
	Providers           []ProviderLink `json:"providers"`
	Status              string         `json:"status"`
	PendingDeletion     bool           `json:"pending_deletion"`
	DeletionScheduledAt *time.Time     `json:"deletion_scheduled_at,omitempty"`
}

// accessClaims is one in-memory access token (never persisted).
type accessClaims struct {
	accountID id.UUID
	familyID  id.UUID
	issuedAt  time.Time
	expiresAt time.Time
}

// TicketIssuer mints gameplay tickets / queue positions (edge/session).
type TicketIssuer interface {
	IssueTicket(ctx context.Context, accountID id.UUID, familyID id.UUID,
		clientBuild uint32, platform protocolv1.ClientPlatform,
		protocolMinor uint32, contentRevision string) (session.Ticket, error)
}

// TicketResult mirrors session.Ticket for the auth edge.
type TicketResult struct {
	Credential       string
	ExpiresAt        time.Time
	QueuePosition    int32
	RetryAfterMs     int64
	WSSURL           string
	ProtocolMinMinor uint32
	MinBuild         uint32
	ContentRevision  string
}

// SessionRevoker closes live sessions on revocation.
type SessionRevoker interface {
	// RevokeAccountSessions pushes SESSION_REPLACED{REVOKED} to every live
	// session of the account and releases their slots.
	RevokeAccountSessions(ctx context.Context, accountID id.UUID)
	// RevokeSessionsExcept revokes the account's live session only when it
	// belongs to a login family other than keepFamily.
	RevokeSessionsExcept(ctx context.Context, accountID id.UUID, keepFamily id.UUID)
	// RevokeFamilySessions revokes the account's live session only when it
	// belongs to the given login family.
	RevokeFamilySessions(ctx context.Context, accountID id.UUID, familyID id.UUID)
}

// Config wires the service.
type Config struct {
	Salt             account.Salt
	Store            *account.Store
	Providers        map[string]ProviderClient
	Tickets          TicketIssuer
	Revoker          SessionRevoker
	WSSURL           string // public WSS endpoint announced in ticket responses
	MinBuild         uint32
	ProtocolMinorMin uint32
	// ServerContentRevision is the canonical revision the server runs.
	ContentRevision string
	Now             func() time.Time
}

// Service is the auth endpoint owner.
type Service struct {
	cfg   Config
	store *account.Store
	limit *Limiter

	mu     sync.Mutex
	access map[string]accessClaims
}

// New builds the service.
func New(cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		cfg:    cfg,
		store:  cfg.Store,
		limit:  NewLimiter(cfg.Store, cfg.Salt, cfg.Now),
		access: make(map[string]accessClaims),
	}
}

// ---------------------------------------------------------------------------
// access tokens (in-memory only — restart forces refresh per auth.md)

// issueAccess mints an opaque access token for the family.
func (s *Service) issueAccess(accountID, familyID id.UUID) (string, accessClaims, error) {
	cred, _, err := NewOpaqueCredential()
	if err != nil {
		return "", accessClaims{}, err
	}
	now := s.cfg.Now()
	c := accessClaims{accountID: accountID, familyID: familyID,
		issuedAt: now, expiresAt: now.Add(AccessTTL)}
	s.mu.Lock()
	s.access[cred] = c
	s.mu.Unlock()
	return cred, c, nil
}

// ValidateAccess resolves a bearer token to its claims.
func (s *Service) ValidateAccess(cred string) (accessClaims, error) {
	s.mu.Lock()
	c, ok := s.access[cred]
	s.mu.Unlock()
	if !ok {
		return accessClaims{}, ErrAuthInvalid
	}
	if s.cfg.Now().After(c.expiresAt) {
		return accessClaims{}, ErrAuthExpired
	}
	return c, nil
}

// dropFamilyAccess removes every access token of a family.
func (s *Service) dropFamilyAccess(familyID id.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, c := range s.access {
		if c.familyID == familyID {
			delete(s.access, k)
		}
	}
}

// dropAccountAccess removes every access token of an account.
func (s *Service) dropAccountAccess(accountID id.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, c := range s.access {
		if c.accountID == accountID {
			delete(s.access, k)
		}
	}
}

// ---------------------------------------------------------------------------
// shared helpers

// deviceScopeValue renders the device part of IP_DEVICE keys.
func deviceScopeValue(deviceID string) string { return deviceID }

// ipScope is the /32 IPv4 or /64 IPv6 scope value for rate keys.
func ipScope(ip net.IP) string { return account.IPScopeValue(ip) }

// ipDeviceScope builds the IP_DEVICE scope value (ip + ':' + device_id).
func ipDeviceScope(ip net.IP, deviceID string) string {
	return ipScope(ip) + ":" + deviceScopeValue(deviceID)
}

// recordLogin writes one login-history row (success only; refresh
// excluded by the callers).
func (s *Service) recordLogin(ctx context.Context, tx account.Tx, accountID id.UUID, meta LoginMeta) (isNewOrigin bool, err error) {
	devHash := s.cfg.Salt.DeviceIDHash([]byte(meta.DeviceID))
	ipHash := s.cfg.Salt.IPPrefixHash(meta.IP)
	now := s.cfg.Now()
	seen, err := s.store.SeenOrigin(ctx, tx, accountID, devHash[:], ipHash[:], now.Add(-HistoryWindow))
	if err != nil {
		return false, err
	}
	err = s.store.InsertLoginHistory(ctx, tx, account.LoginHistoryRow{
		AccountID:      accountID,
		ObservedAt:     now,
		DeviceIDHash:   devHash[:],
		IPPrefix16Hash: ipHash[:],
		IsNewOrigin:    !seen,
	})
	return !seen, err
}

// newFamily creates the session family + first refresh generation.
func (s *Service) newFamily(ctx context.Context, tx account.Tx, accountID id.UUID,
	providerID string, meta LoginMeta) (family account.FamilyRow, refreshCred string, err error) {
	now := s.cfg.Now()
	devHash := s.cfg.Salt.DeviceIDHash([]byte(meta.DeviceID))
	switch meta.Platform {
	case "WINDOWS", "ANDROID":
	default:
		// client_platform has a CHECK constraint — reject at the edge
		// rather than surfacing a durable write failure as a 503.
		return family, "", ErrBadRequest
	}
	family = account.FamilyRow{
		SessionFamilyID:   id.NewV7(now),
		AccountID:         accountID,
		ProviderID:        providerID,
		ClientPlatform:    meta.Platform,
		AppVersion:        meta.AppVersion,
		DeviceIDHash:      devHash[:],
		AbsoluteExpiresAt: now.Add(FamilyAbsoluteTTL),
		CreatedAt:         now,
		LastRefreshedAt:   now,
		ExpiresAt:         now.Add(RefreshTTL),
	}
	if meta.DeviceModelClass != "" {
		family.DeviceModelClass = &meta.DeviceModelClass
	}
	if err = s.store.CreateFamily(ctx, tx, family); err != nil {
		return family, "", err
	}
	cred, hash, err := NewOpaqueCredential()
	if err != nil {
		return family, "", err
	}
	if err = s.store.InsertRefreshCredential(ctx, tx, account.RefreshCredentialRow{
		CredentialHash:  hash,
		SessionFamilyID: family.SessionFamilyID,
		Generation:      1,
		IssuedAt:        now,
		ExpiresAt:       family.ExpiresAt,
	}); err != nil {
		return family, "", err
	}
	return family, cred, nil
}

// respond mints access + wraps the family into a TokenResponse.
func (s *Service) respond(accountID, familyID id.UUID, refreshCred string,
	refreshExpiresAt time.Time, isNew bool) (TokenResponse, error) {
	access, claims, err := s.issueAccess(accountID, familyID)
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccountID:        accountID,
		AccessToken:      access,
		AccessExpiresAt:  claims.expiresAt,
		RefreshToken:     refreshCred,
		RefreshExpiresAt: refreshExpiresAt,
		IsNewAccount:     isNew,
	}, nil
}

// checkAccountStatus applies the login-time status gates.
func checkLoginStatus(st string) error {
	switch st {
	case account.StatusBanned:
		return ErrBanned
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/password/register

// Register creates a password account + session family in one tx.
func (s *Service) Register(ctx context.Context, username, email, password string,
	meta LoginMeta) (TokenResponse, error) {
	var resp TokenResponse

	// Capacity buckets first (rate_limits.md): IP 60/h, IP_DEVICE 5/h —
	// the IP_DEVICE bound is the enumeration control.
	if err := s.limit.Check(ctx, ActionRegister, ScopeIP, ipScope(meta.IP), 60, time.Hour); err != nil {
		return resp, err
	}
	if err := s.limit.Check(ctx, ActionRegister, ScopeIPDevice,
		ipDeviceScope(meta.IP, meta.DeviceID), 5, time.Hour); err != nil {
		return resp, err
	}

	ukey, err := ValidateUsername(username)
	if err != nil {
		return resp, err
	}
	ekey, email, err := ValidateEmail(email)
	if err != nil {
		return resp, err
	}
	if err := ValidatePassword(password, ukey, ekey); err != nil {
		return resp, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return resp, err
	}

	now := s.cfg.Now()
	accountID := id.NewV7(now)
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if taken, err := s.store.UsernameKeyTaken(ctx, tx, ukey); err != nil {
			return err
		} else if taken {
			return ErrUsernameTaken
		}
		if taken, err := s.store.EmailKeyTaken(ctx, tx, ekey); err != nil {
			return err
		} else if taken {
			return ErrEmailTaken
		}
		if err := s.store.CreateAccount(ctx, tx, accountID, now); err != nil {
			return err
		}
		if err := s.store.InsertPasswordCredential(ctx, tx, account.PasswordCredentialRow{
			AccountID: accountID, UsernameKey: ukey, Email: email, EmailKey: ekey,
			PasswordHash: hash, ParamsVersion: ArgonParamsVersion,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		fam, refresh, err := s.newFamily(ctx, tx, accountID, account.ProviderPassword, meta)
		if err != nil {
			return err
		}
		if _, err := s.recordLogin(ctx, tx, accountID, meta); err != nil {
			return err
		}
		resp, err = s.respond(accountID, fam.SessionFamilyID, refresh, fam.ExpiresAt, true)
		return err
	})
	return resp, err
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/password/login

// PasswordLogin verifies username+password under the L2 rules: IP
// capacity first, then verify, then failure buckets — a correct password
// always succeeds through the lock.
func (s *Service) PasswordLogin(ctx context.Context, username, password string,
	meta LoginMeta) (TokenResponse, error) {
	var resp TokenResponse

	// IP request-capacity bucket (hard limit even for correct passwords).
	if err := s.limit.Check(ctx, ActionPasswordLogin, ScopeIP, ipScope(meta.IP), 20, time.Minute); err != nil {
		return resp, err
	}

	ukey, verr := ValidateUsername(username)
	if verr != nil {
		// Malformed username still verifies a dummy hash to keep the
		// AUTH_INVALID shape/latency uniform.
		ukey = strings.ToLower(strings.TrimSpace(username))
	}

	cred, err := s.store.GetPasswordByUsernameKey(ctx, nil, ukey)
	phc := ""
	known := true
	switch {
	case errors.Is(err, account.ErrNotFound):
		known = false
	case err != nil:
		return resp, err
	default:
		phc = cred.PasswordHash
	}
	if !known {
		phc, err = DummyHash()
		if err != nil {
			return resp, err
		}
	}
	ok, err := VerifyPassword(password, phc)
	if err != nil {
		return resp, err
	}
	if !ok || !known {
		// Fully verified failure: backoff rows + USERNAME counter.
		return resp, s.failPasswordLogin(ctx, ukey, meta)
	}
	if relocked, retry, lerr := s.limit.BackoffLocked(ctx, ActionPasswordLogin, ScopeUsername, ukey); lerr != nil {
		return resp, lerr
	} else if relocked {
		// Locked-but-correct bypasses the lock and clears it.
		_ = relocked
		_ = retry
	}
	// Success: clear the USERNAME failure counter + backoff rows
	// (USERNAME and source-IP backoff; IP capacity stays).
	if err := s.limit.Clear(ctx, ActionPasswordLogin, ScopeUsername, ukey); err != nil {
		return resp, err
	}
	if err := s.limit.ClearBackoff(ctx, ActionPasswordLogin, ScopeUsername, ukey); err != nil {
		return resp, err
	}
	if err := s.limit.ClearBackoff(ctx, ActionPasswordLogin, ScopeIP, ipScope(meta.IP)); err != nil {
		return resp, err
	}

	arow, err := s.store.GetAccount(ctx, nil, cred.AccountID)
	if err != nil {
		return resp, err
	}
	if err := checkLoginStatus(arow.Status); err != nil {
		return resp, err
	}
	// Rehash-on-login when params_version < current.
	if cred.ParamsVersion < ArgonParamsVersion {
		if newHash, herr := HashPassword(password); herr == nil {
			_ = s.store.UpdatePasswordHash(ctx, nil, cred.AccountID, newHash,
				ArgonParamsVersion, s.cfg.Now())
		}
	}

	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, cred.AccountID); err != nil {
			return err
		}
		fam, refresh, err := s.newFamily(ctx, tx, cred.AccountID, account.ProviderPassword, meta)
		if err != nil {
			return err
		}
		if _, err := s.recordLogin(ctx, tx, cred.AccountID, meta); err != nil {
			return err
		}
		resp, err = s.respond(cred.AccountID, fam.SessionFamilyID, refresh, fam.ExpiresAt, false)
		return err
	})
	return resp, err
}

// failPasswordLogin records a verified failure and returns the outward
// error: RATE_LIMITED (uniform shape) while the username/IP lock holds,
// AUTH_INVALID otherwise.
func (s *Service) failPasswordLogin(ctx context.Context, ukey string, meta LoginMeta) error {
	now := s.cfg.Now()
	// USERNAME verified-failure bucket: 10/60s.
	uerr := s.limit.Check(ctx, ActionPasswordLogin, ScopeUsername, ukey, 10, time.Minute)
	for _, sc := range [][2]string{
		{ScopeUsername, ukey},
		{ScopeIP, ipScope(meta.IP)},
	} {
		if err := s.limit.RecordFailure(ctx, ActionPasswordLogin, sc[0], sc[1]); err != nil {
			return err
		}
	}
	// Locked rows delay failures: same shape as AUTH_INVALID.
	for _, sc := range [][2]string{
		{ScopeUsername, ukey},
		{ScopeIP, ipScope(meta.IP)},
	} {
		if locked, retry, err := s.limit.BackoffLocked(ctx, ActionPasswordLogin, sc[0], sc[1]); err != nil {
			return err
		} else if locked {
			return ErrRateLimited(retry.Milliseconds())
		}
	}
	if uerr != nil {
		// USERNAME bucket exhausted → RATE_LIMITED (the failure already
		// consumed budget via Check).
		var ae *APIError
		if errors.As(uerr, &ae) {
			return ae
		}
		return uerr
	}
	_ = now
	return ErrAuthInvalid
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/login/{provider}

// FederatedLogin verifies the provider token and resolves/creates the
// account (auth.md § Federated Login: unknown subject → create).
func (s *Service) FederatedLogin(ctx context.Context, providerID, providerToken,
	nonce string, meta LoginMeta) (TokenResponse, error) {
	var resp TokenResponse
	if err := s.limit.Check(ctx, ActionFederatedLogin, ScopeIP, ipScope(meta.IP), 30, time.Minute); err != nil {
		return resp, err
	}
	p, ok := s.cfg.Providers[providerID]
	if !ok {
		return resp, ErrAuthInvalid
	}
	subject, err := p.Verify(ctx, providerToken, nonce)
	if err != nil {
		return resp, err
	}
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		ident, err := s.store.GetIdentity(ctx, tx, providerID, subject)
		var accountID id.UUID
		isNew := false
		switch {
		case errors.Is(err, account.ErrNotFound):
			isNew = true
			accountID = id.NewV7(s.cfg.Now())
			if err := s.store.CreateAccount(ctx, tx, accountID, s.cfg.Now()); err != nil {
				return err
			}
			if err := s.store.InsertIdentity(ctx, tx, account.IdentityRow{
				ProviderID: providerID, ProviderSubject: subject,
				AccountID: accountID, LinkedAt: s.cfg.Now(),
			}); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			accountID = ident.AccountID
		}
		arow, err := s.store.GetAccount(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if err := checkLoginStatus(arow.Status); err != nil {
			return err
		}
		if err := account.LockAccount(ctx, tx, accountID); err != nil {
			return err
		}
		fam, refresh, err := s.newFamily(ctx, tx, accountID, providerID, meta)
		if err != nil {
			return err
		}
		if _, err := s.recordLogin(ctx, tx, accountID, meta); err != nil {
			return err
		}
		resp, err = s.respond(accountID, fam.SessionFamilyID, refresh, fam.ExpiresAt, isNew)
		return err
	})
	return resp, err
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/refresh

// Refresh rotates a refresh credential under the reuse/grace rules of
// auth.md § Refresh Rotation: gen N-1 is accepted once when its successor
// N was issued ≤60 s ago and N-1 was never presented; any other replay
// revokes the whole family.
func (s *Service) Refresh(ctx context.Context, refreshToken, deviceID string) (TokenResponse, error) {
	var resp TokenResponse
	hash, ok := CredentialHash(refreshToken)
	if !ok {
		return resp, ErrAuthInvalid
	}
	row, err := s.store.GetRefreshCredential(ctx, nil, hash)
	if errors.Is(err, account.ErrNotFound) {
		return resp, ErrAuthInvalid
	}
	if err != nil {
		return resp, err
	}
	// Refresh limit is ACCOUNT-scoped.
	if err := s.limit.Check(ctx, ActionRefresh, ScopeAccount,
		account.Int64ScopeValue(0)+row.SessionFamilyID.String(), 30, time.Minute); err != nil {
		return resp, err
	}

	var reuseDetected bool
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		fam, err := s.store.GetFamily(ctx, tx, row.SessionFamilyID)
		if err != nil {
			return err
		}
		now := s.cfg.Now()
		if fam.RevokedAt != nil {
			return ErrAuthInvalid
		}
		if now.After(fam.AbsoluteExpiresAt) || now.After(row.ExpiresAt) {
			return ErrAuthExpired
		}
		devHash := s.cfg.Salt.DeviceIDHash([]byte(deviceID))
		sameDevice := len(fam.DeviceIDHash) == len(devHash) &&
			string(fam.DeviceIDHash) == string(devHash[:])

		latest, err := s.store.LatestRefreshCredential(ctx, tx, fam.SessionFamilyID)
		if err != nil {
			return err
		}
		switch {
		case row.Generation == latest.Generation && row.RotatedAt == nil:
			// normal rotation: presented newest unrotated generation
			if err := s.store.MarkRefreshPresented(ctx, tx, row.CredentialHash, now); err != nil {
				return err
			}
			if err := s.store.MarkRefreshRotated(ctx, tx, row.CredentialHash, now); err != nil {
				return err
			}
		case row.Generation == latest.Generation-1 &&
			row.RotatedAt != nil && latest.FirstPresentedAt == nil &&
			latest.RotatedAt == nil &&
			now.Sub(latest.IssuedAt) <= RefreshGrace && sameDevice:
			// lost-response grace: the presented N-1 is accepted once;
			// the never-presented N is marked rotated and N+1 issued.
			if err := s.store.MarkRefreshPresented(ctx, tx, row.CredentialHash, now); err != nil {
				return err
			}
			if err := s.store.MarkRefreshRotated(ctx, tx, latest.CredentialHash, now); err != nil {
				return err
			}
		default:
			// Reuse: revoke the whole family (REUSE_DETECTED).
			if err := s.store.RevokeFamily(ctx, tx, fam.SessionFamilyID,
				account.RevokeReuseDetected, now); err != nil {
				return err
			}
			if err := s.store.InsertRevocation(ctx, tx, id.NewV7(now),
				account.RevocationScopeSessionFamily, &fam.AccountID,
				&fam.SessionFamilyID, nil, now, now, now.Add(FamilyAbsoluteTTL)); err != nil {
				return err
			}
			// Commit the revocation; the outward AUTH_INVALID returns
			// after the tx so the writes survive.
			reuseDetected = true
			return nil
		}
		// Slide the 30-day window; the 90-day absolute cap is authoritative.
		newExpires := now.Add(RefreshTTL)
		if newExpires.After(fam.AbsoluteExpiresAt) {
			newExpires = fam.AbsoluteExpiresAt
		}
		if err := s.store.TouchFamilyRefresh(ctx, tx, fam.SessionFamilyID, now, newExpires); err != nil {
			return err
		}
		cred, credHash, err := NewOpaqueCredential()
		if err != nil {
			return err
		}
		if err := s.store.InsertRefreshCredential(ctx, tx, account.RefreshCredentialRow{
			CredentialHash:  credHash,
			SessionFamilyID: fam.SessionFamilyID,
			Generation:      latest.Generation + 1,
			IssuedAt:        now,
			ExpiresAt:       newExpires,
		}); err != nil {
			return err
		}
		resp, err = s.respond(fam.AccountID, fam.SessionFamilyID, cred, newExpires, false)
		return err
	})
	if reuseDetected {
		return resp, ErrAuthInvalid
	}
	return resp, err
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/logout

// Logout revokes one family (SESSION) or every family of the account
// (ALL), dropping live in-memory access tokens and bound sessions.
func (s *Service) Logout(ctx context.Context, accessToken, scope string) error {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return err
	}
	now := s.cfg.Now()
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		switch scope {
		case "SESSION":
			if err := s.store.RevokeFamily(ctx, tx, claims.familyID,
				account.RevokeLogout, now); err != nil {
				return err
			}
			if err := s.store.InsertRevocation(ctx, tx, id.NewV7(now),
				account.RevocationScopeSessionFamily, &claims.accountID,
				&claims.familyID, nil, now, now, now.Add(FamilyAbsoluteTTL)); err != nil {
				return err
			}
			s.dropFamilyAccess(claims.familyID)
		case "ALL":
			if err := s.store.RevokeFamiliesByAccount(ctx, tx, claims.accountID,
				id.UUID{}, account.RevokeAccountRevoke, now); err != nil {
				return err
			}
			if err := s.store.InsertRevocation(ctx, tx, id.NewV7(now),
				account.RevocationScopeAccount, &claims.accountID,
				nil, nil, now, now, now.Add(FamilyAbsoluteTTL)); err != nil {
				return err
			}
			s.dropAccountAccess(claims.accountID)
		default:
			return ErrBadRequest
		}
		return nil
	})
	if err != nil {
		return err
	}
	// The revoked credentials' live gameplay connection is deauthorized
	// (session.md § Logout).
	if s.cfg.Revoker != nil {
		switch scope {
		case "SESSION":
			s.cfg.Revoker.RevokeFamilySessions(ctx, claims.accountID, claims.familyID)
		case "ALL":
			s.cfg.Revoker.RevokeAccountSessions(ctx, claims.accountID)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/password/change

// PasswordChange verifies the current password, stores the new hash and
// revokes every OTHER session family. Runs the takeover rule when a
// new-origin login happened within the last hour.
func (s *Service) PasswordChange(ctx context.Context, accessToken,
	currentPassword, newPassword string) (TokenResponse, error) {
	var resp TokenResponse
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return resp, err
	}
	if err := s.limit.Check(ctx, ActionPasswordChange, ScopeAccount,
		claims.accountID.String(), 5, time.Hour); err != nil {
		return resp, err
	}
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		arow, err := s.store.GetAccount(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		cred, err := s.store.GetPasswordByAccount(ctx, tx, claims.accountID)
		if errors.Is(err, account.ErrNotFound) {
			return ErrInvalidState // password change on a federated-only account
		}
		if err != nil {
			return err
		}
		ok, err := VerifyPassword(currentPassword, cred.PasswordHash)
		if err != nil {
			return err
		}
		if !ok {
			return ErrAuthInvalid
		}
		// Guard: while credential_guard_until > now a valid
		// current_password is supplied → allowed (the bypass).
		if err := ValidatePassword(newPassword, cred.UsernameKey, cred.EmailKey); err != nil {
			return err
		}
		hash, err := HashPassword(newPassword)
		if err != nil {
			return err
		}
		now := s.cfg.Now()
		if err := s.store.UpdatePasswordHash(ctx, tx, claims.accountID, hash,
			ArgonParamsVersion, now); err != nil {
			return err
		}
		if err := s.applyTakeoverIfRecentNewOrigin(ctx, tx, claims.accountID, claims.familyID, now); err != nil {
			return err
		}
		// Revoke all other families; this session's family survives.
		if err := s.store.RevokeFamiliesByAccount(ctx, tx, claims.accountID,
			claims.familyID, account.RevokePasswordChange, now); err != nil {
			return err
		}
		if err := s.store.InsertRevocation(ctx, tx, id.NewV7(now),
			account.RevocationScopeAccount, &claims.accountID,
			nil, nil, now, now, now.Add(FamilyAbsoluteTTL)); err != nil {
			return err
		}
		_ = arow
		// Rotate the surviving family: issue a fresh refresh generation.
		credTok, credHash, err := NewOpaqueCredential()
		if err != nil {
			return err
		}
		latest, err := s.store.LatestRefreshCredential(ctx, tx, claims.familyID)
		if err != nil {
			return err
		}
		newExpires := now.Add(RefreshTTL)
		fam, err := s.store.GetFamily(ctx, tx, claims.familyID)
		if err != nil {
			return err
		}
		if newExpires.After(fam.AbsoluteExpiresAt) {
			newExpires = fam.AbsoluteExpiresAt
		}
		if err := s.store.InsertRefreshCredential(ctx, tx, account.RefreshCredentialRow{
			CredentialHash:  credHash,
			SessionFamilyID: claims.familyID,
			Generation:      latest.Generation + 1,
			IssuedAt:        now,
			ExpiresAt:       newExpires,
		}); err != nil {
			return err
		}
		resp, err = s.respond(claims.accountID, claims.familyID, credTok, newExpires, false)
		return err
	})
	if err != nil {
		return resp, err
	}
	// Drop access tokens for the revoked families (in-memory filter: keep
	// only this family for this account).
	s.mu.Lock()
	for k, c := range s.access {
		if c.accountID == claims.accountID && c.familyID != claims.familyID {
			delete(s.access, k)
		}
	}
	s.mu.Unlock()
	// Force session invalidation for the revoked families' live gameplay
	// connection (session.md § Security Events); the caller's survives.
	if s.cfg.Revoker != nil {
		s.cfg.Revoker.RevokeSessionsExcept(ctx, claims.accountID, claims.familyID)
	}
	return resp, nil
}

// applyTakeoverIfRecentNewOrigin arms the takeover rule when the account
// saw an is_new_origin login within the last hour (anti_cheat.md
// § Account takeover).
func (s *Service) applyTakeoverIfRecentNewOrigin(ctx context.Context, tx account.Tx,
	accountID, keepFamily id.UUID, now time.Time) error {
	hit, err := s.store.RecentNewOriginLogin(ctx, tx, accountID, now.Add(-TakeoverWindow))
	if err != nil {
		return err
	}
	if !hit {
		return nil
	}
	if err := s.store.RevokeFamiliesByAccount(ctx, tx, accountID, keepFamily,
		account.RevokeTakeoverRule, now); err != nil {
		return err
	}
	if err := s.store.SetCredentialGuardUntil(ctx, tx, accountID, now.Add(GuardDuration)); err != nil {
		return err
	}
	return s.store.InsertAuditEvent(ctx, tx, account.AuditEvent{
		EventID:          id.NewV7(now),
		OccurredAt:       now,
		ActorKind:        "SYSTEM",
		SubjectAccountID: &accountID,
		Action:           "ACCOUNT_SECURITY_REVIEW",
	})
}

// ---------------------------------------------------------------------------
// POST /api/v1/auth/link/{provider} · unlink/{provider}

// Link attaches a federated provider to the logged-in account.
func (s *Service) Link(ctx context.Context, accessToken, providerID,
	providerToken, currentPassword, nonce string) ([]ProviderLink, error) {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return nil, err
	}
	if providerID == account.ProviderPassword {
		return nil, ErrBadRequest
	}
	if err := s.limit.Check(ctx, ActionLinkUnlink, ScopeAccount,
		claims.accountID.String(), 10, time.Hour); err != nil {
		return nil, err
	}
	p, ok := s.cfg.Providers[providerID]
	if !ok {
		return nil, ErrAuthInvalid
	}
	if err := s.checkCredentialGuard(ctx, claims.accountID, currentPassword); err != nil {
		return nil, err
	}
	subject, err := p.Verify(ctx, providerToken, nonce)
	if err != nil {
		return nil, err
	}
	var out []ProviderLink
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		// Subject already linked anywhere → PROVIDER_ALREADY_LINKED.
		if existing, err := s.store.GetIdentity(ctx, tx, providerID, subject); err == nil {
			_ = existing
			return ErrProviderAlreadyLinked
		} else if !errors.Is(err, account.ErrNotFound) {
			return err
		}
		if err := s.store.InsertIdentity(ctx, tx, account.IdentityRow{
			ProviderID: providerID, ProviderSubject: subject,
			AccountID: claims.accountID, LinkedAt: s.cfg.Now(),
		}); err != nil {
			if errors.Is(err, account.ErrConflict) {
				return ErrProviderAlreadyLinked
			}
			return err
		}
		rows, err := s.store.ListIdentities(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		out = providerLinks(rows)
		return nil
	})
	return out, err
}

// Unlink removes one provider link; the last usable login method cannot
// be removed. The takeover rule applies on unlink.
func (s *Service) Unlink(ctx context.Context, accessToken, providerID,
	currentPassword, providerToken, nonce string) ([]ProviderLink, error) {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return nil, err
	}
	if err := s.limit.Check(ctx, ActionLinkUnlink, ScopeAccount,
		claims.accountID.String(), 10, time.Hour); err != nil {
		return nil, err
	}
	var out []ProviderLink
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		// Guard: current_password or a fresh token of ANOTHER linked
		// provider passes the check.
		if err := s.checkCredentialGuardTx(ctx, tx, claims.accountID,
			currentPassword, providerToken, nonce); err != nil {
			return err
		}
		// Last-method check: password cred + identities count must be >1.
		_, pwErr := s.store.GetPasswordByAccount(ctx, tx, claims.accountID)
		rows, err := s.store.ListIdentities(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		usable := len(rows)
		if pwErr == nil {
			usable++
		}
		// unlinking the only federated method while no password exists → LAST_LOGIN_METHOD
		var target *account.IdentityRow
		remaining := 0
		for i := range rows {
			if rows[i].ProviderID == providerID {
				target = &rows[i]
			} else {
				remaining++
			}
		}
		_ = target
		if usable-1 <= 0 {
			return ErrLastLoginMethod
		}
		deleted, err := s.store.DeleteIdentity(ctx, tx, claims.accountID, providerID)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrAuthInvalid
		}
		now := s.cfg.Now()
		if err := s.applyTakeoverIfRecentNewOrigin(ctx, tx, claims.accountID, claims.familyID, now); err != nil {
			return err
		}
		rows, err = s.store.ListIdentities(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		out = providerLinks(rows)
		return nil
	})
	return out, err
}

func providerLinks(rows []account.IdentityRow) []ProviderLink {
	out := make([]ProviderLink, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProviderLink{ProviderID: r.ProviderID, LinkedAt: r.LinkedAt})
	}
	return out
}

// checkCredentialGuard implements the credential_guard_until rule for
// link: while the guard holds, credential changes need the account's
// current password.
func (s *Service) checkCredentialGuard(ctx context.Context, accountID id.UUID,
	currentPassword string) error {
	return s.store.InTx(ctx, func(tx account.Tx) error {
		return s.checkCredentialGuardTx(ctx, tx, accountID, currentPassword, "", "")
	})
}

// checkCredentialGuardTx is the tx-scoped guard: passes when the guard is
// inactive or a valid current_password / fresh other-provider token is
// supplied.
func (s *Service) checkCredentialGuardTx(ctx context.Context, tx account.Tx, accountID id.UUID,
	currentPassword, providerToken, nonce string) error {
	arow, err := s.store.GetAccount(ctx, tx, accountID)
	if err != nil {
		return err
	}
	now := s.cfg.Now()
	if arow.CredentialGuardUntil == nil || !arow.CredentialGuardUntil.After(now) {
		return nil
	}
	if currentPassword != "" {
		cred, err := s.store.GetPasswordByAccount(ctx, tx, accountID)
		if err != nil {
			return err
		}
		ok, err := VerifyPassword(currentPassword, cred.PasswordHash)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return ErrCredentialChangeLock
	}
	if providerToken != "" {
		// A fresh token of ANOTHER linked provider also passes.
		rows, err := s.store.ListIdentities(ctx, tx, accountID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			p := s.cfg.Providers[r.ProviderID]
			if p == nil {
				continue
			}
			if _, err := p.Verify(ctx, providerToken, nonce); err == nil {
				return nil
			}
		}
	}
	return ErrCredentialChangeLock
}

// ---------------------------------------------------------------------------
// GET /api/v1/account

// GetAccount returns the account view.
func (s *Service) GetAccount(ctx context.Context, accessToken string) (AccountView, error) {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return AccountView{}, err
	}
	arow, err := s.store.GetAccount(ctx, nil, claims.accountID)
	if err != nil {
		return AccountView{}, err
	}
	rows, err := s.store.ListIdentities(ctx, nil, claims.accountID)
	if err != nil {
		return AccountView{}, err
	}
	v := AccountView{
		AccountID: claims.accountID.String(),
		Status:    arow.Status,
		Providers: providerLinks(rows),
	}
	if arow.DeletionRequestedAt != nil {
		v.PendingDeletion = true
		sched := arow.DeletionRequestedAt.Add(DeletionWindow)
		v.DeletionScheduledAt = &sched
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// POST /api/v1/gameplay/ticket

// Ticket mints a gameplay ticket or answers the queue position.
func (s *Service) Ticket(ctx context.Context, accessToken string,
	clientBuild uint32, platform protocolv1.ClientPlatform,
	protoMajor, protoMinor uint32, contentRevision string) (TicketResult, error) {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return TicketResult{}, err
	}
	if err := s.limit.Check(ctx, ActionGameplayTicket, ScopeAccount,
		claims.accountID.String(), 20, time.Minute); err != nil {
		return TicketResult{}, err
	}
	arow, err := s.store.GetAccount(ctx, nil, claims.accountID)
	if err != nil {
		return TicketResult{}, err
	}
	// Status gates: banned never; pending-deletion still tickets.
	if arow.Status == account.StatusBanned {
		return TicketResult{}, ErrBanned
	}
	if s.cfg.Tickets == nil {
		return TicketResult{}, ErrTemporaryDependency
	}
	t, err := s.cfg.Tickets.IssueTicket(ctx, claims.accountID, claims.familyID,
		clientBuild, platform, protoMinor, contentRevision)
	if err != nil {
		return TicketResult{}, err
	}
	return TicketResult{
		Credential:       t.Credential,
		ExpiresAt:        t.ExpiresAt,
		QueuePosition:    t.QueuePosition,
		RetryAfterMs:     t.RetryAfterMs,
		WSSURL:           s.cfg.WSSURL,
		ProtocolMinMinor: s.cfg.ProtocolMinorMin,
		MinBuild:         s.cfg.MinBuild,
		ContentRevision:  s.cfg.ContentRevision,
	}, nil
}

// ---------------------------------------------------------------------------
// POST /api/v1/account/delete · delete/cancel

// DeleteAccount marks the account PENDING_DELETION after re-auth:
// current_password for password accounts, or a fresh (≤5 min) provider
// token for federated ones. It revokes every family with
// revoke_reason=ERASURE_REQUEST and does NOT write erasure_intents —
// the post-window erasure worker owns those.
func (s *Service) DeleteAccount(ctx context.Context, accessToken,
	currentPassword, providerToken, nonce string) error {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return err
	}
	err = s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		arow, err := s.store.GetAccount(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		if arow.DeletionRequestedAt != nil {
			return ErrInvalidState
		}
		// Re-authenticate.
		ok, err := s.reAuthenticate(ctx, tx, claims.accountID, currentPassword, providerToken, nonce)
		if err != nil {
			return err
		}
		if !ok {
			return ErrAuthInvalid
		}
		now := s.cfg.Now()
		if err := s.store.BeginDeletion(ctx, tx, claims.accountID, now); err != nil {
			return err
		}
		if err := s.store.RevokeFamiliesByAccount(ctx, tx, claims.accountID,
			id.UUID{}, account.RevokeErasureRequest, now); err != nil {
			return err
		}
		return s.store.InsertRevocation(ctx, tx, id.NewV7(now),
			account.RevocationScopeAccount, &claims.accountID,
			nil, nil, now, now, now.Add(FamilyAbsoluteTTL))
	})
	if err != nil {
		return err
	}
	if s.cfg.Revoker != nil {
		s.cfg.Revoker.RevokeAccountSessions(ctx, claims.accountID)
	}
	return nil
}

// reAuthenticate verifies the deletion re-auth credential.
func (s *Service) reAuthenticate(ctx context.Context, tx account.Tx, accountID id.UUID,
	currentPassword, providerToken, nonce string) (bool, error) {
	if currentPassword != "" {
		cred, err := s.store.GetPasswordByAccount(ctx, tx, accountID)
		if err != nil {
			return false, err
		}
		return VerifyPassword(currentPassword, cred.PasswordHash)
	}
	if providerToken != "" {
		// Fresh provider token ≤5 min: verify against each linked provider
		// and check the token's issued-at where the provider exposes it.
		rows, err := s.store.ListIdentities(ctx, tx, accountID)
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			p := s.cfg.Providers[r.ProviderID]
			if p == nil {
				continue
			}
			if fp, ok := p.(FreshnessProvider); ok {
				sub, iat, err := fp.VerifyFresh(ctx, providerToken, nonce)
				if err != nil || sub != r.ProviderSubject {
					continue
				}
				if s.cfg.Now().Sub(iat) <= ReAuthFreshness {
					return true, nil
				}
			} else if sub, err := p.Verify(ctx, providerToken, nonce); err == nil && sub == r.ProviderSubject {
				return true, nil
			}
		}
		return false, nil
	}
	return false, ErrAuthInvalid
}

// FreshnessProvider exposes the token issued-at for providers that carry
// one (Apple/Google iat claim).
type FreshnessProvider interface {
	VerifyFresh(ctx context.Context, token, nonce string) (subject string, issuedAt time.Time, err error)
}

// CancelDeletion clears the pending deletion under the account lock,
// re-evaluates the 180-day refund-consumed score and restores status
// (SUSPENDED_PAYMENT_RECONCILIATION iff score ≥ 2 else ACTIVE; a BANNED
// account never returns ACTIVE). 204 when not pending (idempotent); 409
// INVALID_STATE once erasure_started_at is set.
func (s *Service) CancelDeletion(ctx context.Context, accessToken string) error {
	claims, err := s.ValidateAccess(accessToken)
	if err != nil {
		return err
	}
	if err := s.limit.Check(ctx, ActionDeleteCancel, ScopeAccount,
		claims.accountID.String(), 10, time.Hour); err != nil {
		return err
	}
	return s.store.InTx(ctx, func(tx account.Tx) error {
		if err := account.LockAccount(ctx, tx, claims.accountID); err != nil {
			return err
		}
		arow, err := s.store.GetAccount(ctx, tx, claims.accountID)
		if err != nil {
			return err
		}
		if arow.ErasureStartedAt != nil {
			return ErrInvalidState
		}
		if arow.DeletionRequestedAt == nil || arow.Status != account.StatusPendingDeletion {
			return nil // idempotent
		}
		score, err := s.store.RefundConsumedScore(ctx, tx, claims.accountID,
			s.cfg.Now().Add(-180*24*time.Hour))
		if err != nil {
			return err
		}
		newStatus := account.StatusActive
		if score >= 2 {
			newStatus = account.StatusSuspendedReconciliation
		}
		if arow.Status == account.StatusBanned {
			newStatus = account.StatusBanned
		}
		return s.store.CancelDeletion(ctx, tx, claims.accountID, newStatus)
	})
}

// CleanupStaleRateRows deletes limiter rows unchanged for 24 h
// (external_integrations.md §3 cleanup job).
func (s *Service) CleanupStaleRateRows(ctx context.Context) error {
	return s.store.DeleteStaleRateRows(ctx, s.cfg.Now().Add(-24*time.Hour))
}
