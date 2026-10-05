// Package account owns row access for the account/auth/session tables of
// data_model.md § Accounts + § Auth sessions and the L2 limiter tables of
// external_integrations.md § 3. Lock discipline: tables covered by
// lockorder.lockColumns are locked through lockorder.Acquire on their
// account/character owner key before mutation; rate_limit_counters and
// auth_failure_backoff are keyed on hash values and use atomic upserts
// (handoff D-5).
package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// DBTX is the query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

var (
	// ErrNotFound is returned when a lookup finds no row.
	ErrNotFound = errors.New("account: not found")
	// ErrConflict is a uniqueness/constraint violation surfacing as the
	// caller's taken/already-linked mapping.
	ErrConflict = errors.New("account: conflict")
)

// Status values of accounts.status (data_model.md § Accounts).
const (
	StatusActive                  = "ACTIVE"
	StatusSuspendedReconciliation = "SUSPENDED_PAYMENT_RECONCILIATION"
	StatusBanned                  = "BANNED"
	StatusPendingDeletion         = "PENDING_DELETION"
	StatusTombstone               = "TOMBSTONE_ERASED"
)

// Provider ids of auth_session_families.provider_id /
// account_identities.provider_id.
const (
	ProviderPassword = "password"
	ProviderApple    = "apple"
	ProviderGoogle   = "google"
	ProviderSteam    = "steam"
)

// Revocation scopes of auth_revocations.scope.
const (
	RevocationScopeSessionFamily = "SESSION_FAMILY"
	RevocationScopeAccount       = "ACCOUNT"
	RevocationScopeProviderLink  = "PROVIDER_LINK"
)

// Revoke reasons of auth_session_families.revoke_reason.
const (
	RevokeLogout         = "LOGOUT"
	RevokeReuseDetected  = "REUSE_DETECTED"
	RevokeAccountRevoke  = "ACCOUNT_REVOKE"
	RevokePasswordChange = "PASSWORD_CHANGE"
	RevokeTakeoverRule   = "TAKEOVER_RULE"
	RevokeErasureRequest = "ERASURE_REQUEST"
	RevokeAdmin          = "ADMIN"
)

// Store is the row-access owner for the account tables.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore binds the store to the shared pool.
// q normalizes a nil DBTX to the pool for non-tx calls.
func (s *Store) q(db DBTX) DBTX {
	if db == nil {
		return s.pool
	}
	return db
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pool for composition (queue/store wiring).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// InTx runs fn inside one transaction.
func (s *Store) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LockAccount locks every account-scoped table of lockColumns the fn will
// mutate, in canonical order (database.md § 5).
func LockAccount(ctx context.Context, tx pgx.Tx, accountID id.UUID) error {
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("accounts", accountID),
		lockorder.RowLock("account_password_credentials", accountID),
		lockorder.RowLock("account_identities", accountID),
		lockorder.RowLock("auth_session_families", accountID),
		lockorder.RowLock("auth_revocations", accountID),
		lockorder.RowLock("account_login_history", accountID),
	)
}

// LockCharacter locks the character-scoped tables (attach/detach path).
func LockCharacter(ctx context.Context, tx pgx.Tx, characterID id.UUID) error {
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", characterID),
		lockorder.RowLock("character_activity", characterID),
		lockorder.RowLock("character_attach_events", characterID),
	)
}

// ---------------------------------------------------------------------------
// accounts

// AccountRow is one accounts row.
type AccountRow struct {
	AccountID              id.UUID
	Status                 string
	DeletionRequestedAt    *time.Time
	ErasureStartedAt       *time.Time
	ErasedAt               *time.Time
	CredentialGuardUntil   *time.Time
	EconomyReviewFlaggedAt *time.Time
	CreatedAt              time.Time
}

func scanAccount(r pgx.Row) (AccountRow, error) {
	var a AccountRow
	err := r.Scan(&a.AccountID, &a.Status, &a.DeletionRequestedAt, &a.ErasureStartedAt,
		&a.ErasedAt, &a.CredentialGuardUntil, &a.EconomyReviewFlaggedAt, &a.CreatedAt)
	return a, err
}

const accountCols = `account_id, status, deletion_requested_at, erasure_started_at,
	erased_at, credential_guard_until, economy_review_flagged_at, created_at`

// GetAccount loads one account row.
func (s *Store) GetAccount(ctx context.Context, db DBTX, accountID id.UUID) (AccountRow, error) {
	db = s.q(db)
	row, err := scanAccount(db.QueryRow(ctx,
		`SELECT `+accountCols+` FROM accounts WHERE account_id = $1`, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountRow{}, ErrNotFound
	}
	return row, err
}

// CreateAccount inserts one account row.
func (s *Store) CreateAccount(ctx context.Context, db DBTX, accountID id.UUID, createdAt time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO accounts (account_id, status, created_at) VALUES ($1, $2, $3)`,
		accountID, StatusActive, createdAt)
	return err
}

// SetStatus updates accounts.status.
func (s *Store) SetStatus(ctx context.Context, db DBTX, accountID id.UUID, status string) error {
	db = s.q(db)
	_, err := db.Exec(ctx, `UPDATE accounts SET status = $2 WHERE account_id = $1`, accountID, status)
	return err
}

// SetCredentialGuardUntil sets accounts.credential_guard_until.
func (s *Store) SetCredentialGuardUntil(ctx context.Context, db DBTX, accountID id.UUID, until time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE accounts SET credential_guard_until = $2 WHERE account_id = $1`, accountID, until)
	return err
}

// BeginDeletion marks PENDING_DELETION at the request time.
func (s *Store) BeginDeletion(ctx context.Context, db DBTX, accountID id.UUID, requestedAt time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE accounts SET status = '`+StatusPendingDeletion+`', deletion_requested_at = $2
		 WHERE account_id = $1`, accountID, requestedAt)
	return err
}

// CancelDeletion clears the deletion window and restores status.
func (s *Store) CancelDeletion(ctx context.Context, db DBTX, accountID id.UUID, newStatus string) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE accounts SET status = $2, deletion_requested_at = NULL WHERE account_id = $1`,
		accountID, newStatus)
	return err
}

// ---------------------------------------------------------------------------
// account_login_history

// LoginHistoryRow is one signal row (90-day retention).
type LoginHistoryRow struct {
	AccountID      id.UUID
	ObservedAt     time.Time
	DeviceIDHash   []byte
	IPPrefix16Hash []byte
	IsNewOrigin    bool
}

// SeenOrigin reports whether the (device_id_hash, ip_prefix16_hash) pair
// was recorded for the account within the last 90 days.
func (s *Store) SeenOrigin(ctx context.Context, db DBTX, accountID id.UUID, deviceHash, ipHash []byte, since time.Time) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account_login_history
		  WHERE account_id = $1 AND device_id_hash = $2 AND ip_prefix16_hash = $3 AND observed_at >= $4)`,
		accountID, deviceHash, ipHash, since).Scan(&ok)
	return ok, err
}

// InsertLoginHistory appends one login-signal row (one per successful
// login, refresh excluded). The (account_id, observed_at) primary key
// makes same-instant writes collide; observed_at bumps past the
// account's newest row by one µs so every login keeps its own row.
func (s *Store) InsertLoginHistory(ctx context.Context, db DBTX, row LoginHistoryRow) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO account_login_history (account_id, observed_at, device_id_hash, ip_prefix16_hash, is_new_origin)
		 VALUES (
		   $1,
		   GREATEST($2, COALESCE((
		     SELECT max(observed_at) FROM account_login_history
		     WHERE account_id = $1) + interval '1 microsecond', '-infinity'::timestamptz)),
		   $3,$4,$5)`,
		row.AccountID, row.ObservedAt, row.DeviceIDHash, row.IPPrefix16Hash, row.IsNewOrigin)
	return err
}

// ---------------------------------------------------------------------------
// account_password_credentials

// PasswordCredentialRow is one password-provider record.
type PasswordCredentialRow struct {
	AccountID     id.UUID
	UsernameKey   string
	Email         string
	EmailKey      string
	PasswordHash  string
	ParamsVersion int16
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func scanPasswordCredential(r pgx.Row) (PasswordCredentialRow, error) {
	var c PasswordCredentialRow
	err := r.Scan(&c.AccountID, &c.UsernameKey, &c.Email, &c.EmailKey,
		&c.PasswordHash, &c.ParamsVersion, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

const passwordCredCols = `account_id, username_key, email, email_key, password_hash, params_version, created_at, updated_at`

// GetPasswordByUsernameKey loads the credential row by login key.
func (s *Store) GetPasswordByUsernameKey(ctx context.Context, db DBTX, usernameKey string) (PasswordCredentialRow, error) {
	db = s.q(db)
	c, err := scanPasswordCredential(db.QueryRow(ctx,
		`SELECT `+passwordCredCols+` FROM account_password_credentials WHERE username_key = $1`, usernameKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return PasswordCredentialRow{}, ErrNotFound
	}
	return c, err
}

// GetPasswordByAccount loads the credential row by account.
func (s *Store) GetPasswordByAccount(ctx context.Context, db DBTX, accountID id.UUID) (PasswordCredentialRow, error) {
	db = s.q(db)
	c, err := scanPasswordCredential(db.QueryRow(ctx,
		`SELECT `+passwordCredCols+` FROM account_password_credentials WHERE account_id = $1`, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return PasswordCredentialRow{}, ErrNotFound
	}
	return c, err
}

// InsertPasswordCredential creates the password row for a new account.
func (s *Store) InsertPasswordCredential(ctx context.Context, db DBTX, c PasswordCredentialRow) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO account_password_credentials
		 (account_id, username_key, email, email_key, password_hash, params_version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.AccountID, c.UsernameKey, c.Email, c.EmailKey, c.PasswordHash,
		c.ParamsVersion, c.CreatedAt, c.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// UpdatePasswordHash stores a re-hashed/changed password.
func (s *Store) UpdatePasswordHash(ctx context.Context, db DBTX, accountID id.UUID, hash string, version int16, at time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE account_password_credentials SET password_hash = $2, params_version = $3, updated_at = $4
		 WHERE account_id = $1`, accountID, hash, version, at)
	return err
}

// UsernameKeyTaken reports username_key occupancy.
func (s *Store) UsernameKeyTaken(ctx context.Context, db DBTX, usernameKey string) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account_password_credentials WHERE username_key = $1)`,
		usernameKey).Scan(&ok)
	return ok, err
}

// EmailKeyTaken reports email_key occupancy.
func (s *Store) EmailKeyTaken(ctx context.Context, db DBTX, emailKey string) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account_password_credentials WHERE email_key = $1)`,
		emailKey).Scan(&ok)
	return ok, err
}

// ---------------------------------------------------------------------------
// account_identities

// IdentityRow is one provider link.
type IdentityRow struct {
	ProviderID      string
	ProviderSubject string
	AccountID       id.UUID
	LinkedAt        time.Time
}

// GetIdentity resolves one (provider_id, provider_subject) pair.
func (s *Store) GetIdentity(ctx context.Context, db DBTX, providerID, subject string) (IdentityRow, error) {
	db = s.q(db)
	var r IdentityRow
	err := db.QueryRow(ctx,
		`SELECT provider_id, provider_subject, account_id, linked_at FROM account_identities
		 WHERE provider_id = $1 AND provider_subject = $2`, providerID, subject).
		Scan(&r.ProviderID, &r.ProviderSubject, &r.AccountID, &r.LinkedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdentityRow{}, ErrNotFound
	}
	return r, err
}

// ListIdentities returns the account's provider links.
func (s *Store) ListIdentities(ctx context.Context, db DBTX, accountID id.UUID) ([]IdentityRow, error) {
	db = s.q(db)
	rows, err := db.Query(ctx,
		`SELECT provider_id, provider_subject, account_id, linked_at FROM account_identities
		 WHERE account_id = $1 ORDER BY linked_at, provider_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentityRow
	for rows.Next() {
		var r IdentityRow
		if err := rows.Scan(&r.ProviderID, &r.ProviderSubject, &r.AccountID, &r.LinkedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertIdentity links a provider subject to the account.
func (s *Store) InsertIdentity(ctx context.Context, db DBTX, r IdentityRow) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO account_identities (provider_id, provider_subject, account_id, linked_at)
		 VALUES ($1,$2,$3,$4)`, r.ProviderID, r.ProviderSubject, r.AccountID, r.LinkedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// DeleteIdentity unlinks one provider row.
func (s *Store) DeleteIdentity(ctx context.Context, db DBTX, accountID id.UUID, providerID string) (bool, error) {
	db = s.q(db)
	tag, err := db.Exec(ctx,
		`DELETE FROM account_identities WHERE account_id = $1 AND provider_id = $2`, accountID, providerID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---------------------------------------------------------------------------
// auth_session_families

// FamilyRow is one refresh/session family.
type FamilyRow struct {
	SessionFamilyID   id.UUID
	AccountID         id.UUID
	ProviderID        string
	ClientPlatform    string
	AppVersion        string
	DeviceModelClass  *string
	DeviceIDHash      []byte
	AbsoluteExpiresAt time.Time
	CreatedAt         time.Time
	LastRefreshedAt   time.Time
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	RevokeReason      *string
}

func scanFamily(r pgx.Row) (FamilyRow, error) {
	var f FamilyRow
	err := r.Scan(&f.SessionFamilyID, &f.AccountID, &f.ProviderID, &f.ClientPlatform,
		&f.AppVersion, &f.DeviceModelClass, &f.DeviceIDHash, &f.AbsoluteExpiresAt,
		&f.CreatedAt, &f.LastRefreshedAt, &f.ExpiresAt, &f.RevokedAt, &f.RevokeReason)
	return f, err
}

const familyCols = `session_family_id, account_id, provider_id, client_platform, app_version,
	device_model_class, device_id_hash, absolute_expires_at, created_at, last_refreshed_at,
	expires_at, revoked_at, revoke_reason`

// CreateFamily inserts one session family.
func (s *Store) CreateFamily(ctx context.Context, db DBTX, f FamilyRow) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO auth_session_families
		 (session_family_id, account_id, provider_id, client_platform, app_version,
		  device_model_class, device_id_hash, absolute_expires_at, created_at,
		  last_refreshed_at, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		f.SessionFamilyID, f.AccountID, f.ProviderID, f.ClientPlatform, f.AppVersion,
		f.DeviceModelClass, f.DeviceIDHash, f.AbsoluteExpiresAt, f.CreatedAt,
		f.LastRefreshedAt, f.ExpiresAt)
	return err
}

// GetFamily loads one family row.
func (s *Store) GetFamily(ctx context.Context, db DBTX, familyID id.UUID) (FamilyRow, error) {
	db = s.q(db)
	f, err := scanFamily(db.QueryRow(ctx,
		`SELECT `+familyCols+` FROM auth_session_families WHERE session_family_id = $1`, familyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return FamilyRow{}, ErrNotFound
	}
	return f, err
}

// TouchFamilyRefresh slides last_refreshed_at and the 30-day window cap.
func (s *Store) TouchFamilyRefresh(ctx context.Context, db DBTX, familyID id.UUID, refreshedAt, expiresAt time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE auth_session_families SET last_refreshed_at = $2, expires_at = $3
		 WHERE session_family_id = $1`, familyID, refreshedAt, expiresAt)
	return err
}

// RevokeFamily marks one family revoked.
func (s *Store) RevokeFamily(ctx context.Context, db DBTX, familyID id.UUID, reason string, at time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE auth_session_families SET revoked_at = $2, revoke_reason = $3
		 WHERE session_family_id = $1 AND revoked_at IS NULL`, familyID, at, reason)
	return err
}

// RevokeFamiliesByAccount revokes every live family of the account;
// keepFamilyID (zero = none) is spared (password-change keeps the new one).
func (s *Store) RevokeFamiliesByAccount(ctx context.Context, db DBTX, accountID id.UUID, keep id.UUID, reason string, at time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE auth_session_families SET revoked_at = $3, revoke_reason = $4
		 WHERE account_id = $1 AND revoked_at IS NULL AND session_family_id <> $2`,
		accountID, keep, at, reason)
	return err
}

// ---------------------------------------------------------------------------
// auth_refresh_credentials

// RefreshCredentialRow is one stored refresh generation.
type RefreshCredentialRow struct {
	CredentialHash   []byte
	SessionFamilyID  id.UUID
	Generation       int32
	IssuedAt         time.Time
	ExpiresAt        time.Time
	FirstPresentedAt *time.Time
	RotatedAt        *time.Time
}

func scanRefreshCred(r pgx.Row) (RefreshCredentialRow, error) {
	var c RefreshCredentialRow
	err := r.Scan(&c.CredentialHash, &c.SessionFamilyID, &c.Generation, &c.IssuedAt,
		&c.ExpiresAt, &c.FirstPresentedAt, &c.RotatedAt)
	return c, err
}

const refreshCredCols = `credential_hash, session_family_id, generation, issued_at, expires_at,
	first_presented_at, rotated_at`

// InsertRefreshCredential stores generation N.
func (s *Store) InsertRefreshCredential(ctx context.Context, db DBTX, c RefreshCredentialRow) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`INSERT INTO auth_refresh_credentials
		 (credential_hash, session_family_id, generation, issued_at, expires_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		c.CredentialHash, c.SessionFamilyID, c.Generation, c.IssuedAt, c.ExpiresAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// GetRefreshCredential loads a credential by hash.
func (s *Store) GetRefreshCredential(ctx context.Context, db DBTX, hash []byte) (RefreshCredentialRow, error) {
	db = s.q(db)
	c, err := scanRefreshCred(db.QueryRow(ctx,
		`SELECT `+refreshCredCols+` FROM auth_refresh_credentials WHERE credential_hash = $1`, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshCredentialRow{}, ErrNotFound
	}
	return c, err
}

// LatestRefreshCredential returns the newest generation of the family.
func (s *Store) LatestRefreshCredential(ctx context.Context, db DBTX, familyID id.UUID) (RefreshCredentialRow, error) {
	db = s.q(db)
	c, err := scanRefreshCred(db.QueryRow(ctx,
		`SELECT `+refreshCredCols+` FROM auth_refresh_credentials
		 WHERE session_family_id = $1 ORDER BY generation DESC LIMIT 1`, familyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshCredentialRow{}, ErrNotFound
	}
	return c, err
}

// MarkRefreshPresented stamps first_presented_at when unset.
func (s *Store) MarkRefreshPresented(ctx context.Context, db DBTX, hash []byte, at time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE auth_refresh_credentials SET first_presented_at = $2
		 WHERE credential_hash = $1 AND first_presented_at IS NULL`, hash, at)
	return err
}

// MarkRefreshRotated stamps rotated_at when unset.
func (s *Store) MarkRefreshRotated(ctx context.Context, db DBTX, hash []byte, at time.Time) error {
	db = s.q(db)
	_, err := db.Exec(ctx,
		`UPDATE auth_refresh_credentials SET rotated_at = $2
		 WHERE credential_hash = $1 AND rotated_at IS NULL`, hash, at)
	return err
}

// ---------------------------------------------------------------------------
// auth_revocations

// InsertRevocation appends one revocation record.
func (s *Store) InsertRevocation(ctx context.Context, db DBTX, revocationID id.UUID,
	scope string, accountID *id.UUID, familyID *id.UUID, providerID *string,
	notBefore, createdAt, expiresAt time.Time) error {
	_, err := db.Exec(ctx,
		`INSERT INTO auth_revocations
		 (revocation_id, scope, account_id, session_family_id, provider_id, not_before, created_at, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		revocationID, scope, accountID, familyID, providerID, notBefore, createdAt, expiresAt)
	return err
}

// RevokedSince reports whether an account-scope revocation exists whose
// not_before is at/after the credential issue time (i.e. tokens issued at
// `issuedAt` or earlier are dead).
func (s *Store) RevokedSince(ctx context.Context, db DBTX, accountID id.UUID, issuedAt, now time.Time) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM auth_revocations
		  WHERE scope = '`+RevocationScopeAccount+`' AND account_id = $1
		    AND not_before >= $2 AND expires_at > $3)`,
		accountID, issuedAt, now).Scan(&ok)
	return ok, err
}

// FamilyRevokedSince reports whether a SESSION_FAMILY revocation covers
// credentials issued at `issuedAt`.
func (s *Store) FamilyRevokedSince(ctx context.Context, db DBTX, familyID id.UUID, issuedAt, now time.Time) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM auth_revocations
		  WHERE scope = '`+RevocationScopeSessionFamily+`' AND session_family_id = $1
		    AND not_before >= $2 AND expires_at > $3)`,
		familyID, issuedAt, now).Scan(&ok)
	return ok, err
}

// ---------------------------------------------------------------------------
// rate_limit_counters / auth_failure_backoff (hash-keyed, no lockColumns)

// CounterRow is one L2 window pair.
type CounterRow struct {
	KeyHash       []byte
	WindowSeconds int32
	WindowStart   time.Time
	CurrentCount  int32
	PreviousCount int32
}

// GetCounter loads one counter row.
func (s *Store) GetCounter(ctx context.Context, db DBTX, keyHash []byte) (CounterRow, error) {
	db = s.q(db)
	var c CounterRow
	err := db.QueryRow(ctx,
		`SELECT key_hash, window_seconds, window_start, current_count, previous_count
		 FROM rate_limit_counters WHERE key_hash = $1`, keyHash).
		Scan(&c.KeyHash, &c.WindowSeconds, &c.WindowStart, &c.CurrentCount, &c.PreviousCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return CounterRow{}, ErrNotFound
	}
	return c, err
}

// BumpCounter atomically advances one sliding-window counter and returns
// the resulting effective count pair (current window discipline of
// external_integrations.md § 3): same window increments current; the next
// window rolls current into previous; farther out resets both.
func (s *Store) BumpCounter(ctx context.Context, keyHash []byte, windowSeconds int32, now time.Time) (current, previous int32, windowStart time.Time, err error) {
	err = s.pool.QueryRow(ctx,
		`INSERT INTO rate_limit_counters (key_hash, window_seconds, window_start, current_count, previous_count)
		 VALUES ($1, $2, $3, 1, 0)
		 ON CONFLICT (key_hash) DO UPDATE SET
		   previous_count = CASE
		     WHEN rate_limit_counters.window_start = $3 THEN rate_limit_counters.previous_count
		     WHEN rate_limit_counters.window_start + make_interval(secs => $2) >= $3 THEN rate_limit_counters.current_count
		     ELSE 0 END,
		   current_count = CASE
		     WHEN rate_limit_counters.window_start = $3 THEN rate_limit_counters.current_count + 1
		     ELSE 1 END,
		   window_start = CASE
		     WHEN rate_limit_counters.window_start = $3 THEN rate_limit_counters.window_start
		     ELSE $3 END
		 RETURNING current_count, previous_count, window_start`,
		keyHash, windowSeconds, now).Scan(&current, &previous, &windowStart)
	return current, previous, windowStart, err
}

// UnbumpCounter removes one unit — a rejected request consumes nothing
// (rate_limits.md: only admitted requests count toward the bucket).
func (s *Store) UnbumpCounter(ctx context.Context, keyHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE rate_limit_counters SET current_count = current_count - 1
		 WHERE key_hash = $1 AND current_count > 0`, keyHash)
	return err
}

// DeleteCounter clears a counter row (success clears USERNAME failure
// budget; capacity buckets are never cleared by success).
func (s *Store) DeleteCounter(ctx context.Context, keyHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM rate_limit_counters WHERE key_hash = $1`, keyHash)
	return err
}

// BackoffRow is one progressive-failure row.
type BackoffRow struct {
	KeyHash             []byte
	ConsecutiveFailures int32
	LockedUntil         *time.Time
	LastFailureAt       time.Time
}

// GetBackoff loads one backoff row.
func (s *Store) GetBackoff(ctx context.Context, db DBTX, keyHash []byte) (BackoffRow, error) {
	db = s.q(db)
	var b BackoffRow
	err := db.QueryRow(ctx,
		`SELECT key_hash, consecutive_failures, locked_until, last_failure_at
		 FROM auth_failure_backoff WHERE key_hash = $1`, keyHash).
		Scan(&b.KeyHash, &b.ConsecutiveFailures, &b.LockedUntil, &b.LastFailureAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BackoffRow{}, ErrNotFound
	}
	return b, err
}

// UpsertBackoff stores the failure counter/lock for one key.
func (s *Store) UpsertBackoff(ctx context.Context, b BackoffRow) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_failure_backoff (key_hash, consecutive_failures, locked_until, last_failure_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (key_hash) DO UPDATE SET
		   consecutive_failures = EXCLUDED.consecutive_failures,
		   locked_until = EXCLUDED.locked_until,
		   last_failure_at = EXCLUDED.last_failure_at`,
		b.KeyHash, b.ConsecutiveFailures, b.LockedUntil, b.LastFailureAt)
	return err
}

// DeleteBackoff clears one backoff row.
func (s *Store) DeleteBackoff(ctx context.Context, keyHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_failure_backoff WHERE key_hash = $1`, keyHash)
	return err
}

// ---------------------------------------------------------------------------
// characters (read surface for session attach + CHARACTER_LIST)

// CharacterSummaryRow feeds S2C_CHARACTER_LIST.
type CharacterSummaryRow struct {
	CharacterID   id.UUID
	AccountID     id.UUID
	Name          string
	ClassID       string
	Level         int32
	MapID         string
	SessionActive bool
	LastOnlineAt  *time.Time
}

// ListCharacters returns account's characters joined to activity state.
func (s *Store) ListCharacters(ctx context.Context, db DBTX, accountID id.UUID) ([]CharacterSummaryRow, error) {
	db = s.q(db)
	rows, err := db.Query(ctx,
		`SELECT c.character_id, c.account_id, c.name, c.class_id, c.level, c.map_id,
		        COALESCE(a.session_active, FALSE),
		        GREATEST(a.last_attached_at, a.last_detached_at, c.updated_at)
		 FROM characters c LEFT JOIN character_activity a ON a.character_id = c.character_id
		 WHERE c.account_id = $1 ORDER BY c.created_at, c.character_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CharacterSummaryRow
	for rows.Next() {
		var c CharacterSummaryRow
		if err := rows.Scan(&c.CharacterID, &c.AccountID, &c.Name, &c.ClassID,
			&c.Level, &c.MapID, &c.SessionActive, &c.LastOnlineAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCharacterOwner returns the owning account + name/map of one character.
func (s *Store) GetCharacterOwner(ctx context.Context, db DBTX, characterID id.UUID) (CharacterSummaryRow, error) {
	db = s.q(db)
	var c CharacterSummaryRow
	err := db.QueryRow(ctx,
		`SELECT c.character_id, c.account_id, c.name, c.class_id, c.level, c.map_id,
		        COALESCE(a.session_active, FALSE),
		        GREATEST(a.last_attached_at, a.last_detached_at, c.updated_at)
		 FROM characters c LEFT JOIN character_activity a ON a.character_id = c.character_id
		 WHERE c.character_id = $1`, characterID).
		Scan(&c.CharacterID, &c.AccountID, &c.Name, &c.ClassID, &c.Level, &c.MapID,
			&c.SessionActive, &c.LastOnlineAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CharacterSummaryRow{}, ErrNotFound
	}
	return c, err
}

// LiveCharacterForAccount returns the account's session_active character,
// if any (resume/superseding-HELLO re-attach target).
func (s *Store) LiveCharacterForAccount(ctx context.Context, db DBTX, accountID id.UUID) (CharacterSummaryRow, error) {
	db = s.q(db)
	var c CharacterSummaryRow
	err := db.QueryRow(ctx,
		`SELECT c.character_id, c.account_id, c.name, c.class_id, c.level, c.map_id,
		        TRUE, GREATEST(a.last_attached_at, a.last_detached_at, c.updated_at)
		 FROM characters c JOIN character_activity a ON a.character_id = c.character_id
		 WHERE c.account_id = $1 AND a.session_active
		 ORDER BY a.last_attached_at DESC NULLS LAST, c.character_id LIMIT 1`, accountID).
		Scan(&c.CharacterID, &c.AccountID, &c.Name, &c.ClassID, &c.Level, &c.MapID,
			&c.SessionActive, &c.LastOnlineAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CharacterSummaryRow{}, ErrNotFound
	}
	return c, err
}

// ---------------------------------------------------------------------------
// account_refund_consumed_events (derived score, data_model.md § accounts)

// RefundConsumedScore counts events in the 180-day window.
func (s *Store) RefundConsumedScore(ctx context.Context, db DBTX, accountID id.UUID, since time.Time) (int32, error) {
	db = s.q(db)
	var n int32
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM account_refund_consumed_events
		 WHERE account_id = $1 AND occurred_at >= $2`, accountID, since).Scan(&n)
	return n, err
}

// ---------------------------------------------------------------------------

func isUniqueViolation(err error) bool {
	var pge *pgconn.PgError
	return errors.As(err, &pge) && pge.Code == "23505"
}

// RecentNewOriginLogin reports whether an is_new_origin login-history row
// exists within `since` — the takeover-rule trigger (anti_cheat.md
// § Account takeover).
func (s *Store) RecentNewOriginLogin(ctx context.Context, db DBTX, accountID id.UUID, since time.Time) (bool, error) {
	db = s.q(db)
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account_login_history
		  WHERE account_id = $1 AND is_new_origin AND observed_at >= $2)`,
		accountID, since).Scan(&ok)
	return ok, err
}

// AuditEvent is one audit_events row (data_model.md § Audit).
type AuditEvent struct {
	EventID            id.UUID
	OccurredAt         time.Time
	ActorKind          string // PLAYER | OPERATOR | SYSTEM
	ActorID            *id.UUID
	SubjectAccountID   *id.UUID
	SubjectCharacterID *id.UUID
	Action             string
	Reason             *string
	TicketID           *string
	OperationID        *id.UUID
	Payload            []byte // JSONB
}

// InsertAuditEvent appends one audit row (no FK — subjects survive
// erasure).
func (s *Store) InsertAuditEvent(ctx context.Context, db DBTX, e AuditEvent) error {
	db = s.q(db)
	if len(e.Payload) == 0 {
		e.Payload = []byte("{}")
	}
	_, err := db.Exec(ctx,
		`INSERT INTO audit_events (audit_event_id, occurred_at, actor_kind, actor_id,
		 subject_account_id, subject_character_id, action, reason, ticket_id,
		 operation_id, payload)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.EventID, e.OccurredAt, e.ActorKind, e.ActorID, e.SubjectAccountID,
		e.SubjectCharacterID, e.Action, e.Reason, e.TicketID, e.OperationID,
		e.Payload)
	return err
}

// DeleteStaleRateRows drops limiter/backoff rows untouched for 24 h
// (external_integrations.md §3 cleanup).
func (s *Store) DeleteStaleRateRows(ctx context.Context, before time.Time) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM rate_limit_counters WHERE window_start < $1`, before); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`DELETE FROM auth_failure_backoff WHERE last_failure_at < $1`, before)
	return err
}

// FamilyByCredentialHash resolves the family owning a refresh
// credential hash (tests / token forensics).
func (s *Store) FamilyByCredentialHash(ctx context.Context, db DBTX, hash []byte) (FamilyRow, error) {
	db = s.q(db)
	var f FamilyRow
	err := db.QueryRow(ctx,
		`SELECT f.session_family_id, f.account_id, f.provider_id, f.client_platform,
		        f.app_version, f.device_model_class, f.device_id_hash,
		        f.absolute_expires_at, f.created_at, f.last_refreshed_at,
		        f.expires_at, f.revoked_at, f.revoke_reason
		 FROM auth_session_families f
		 JOIN auth_refresh_credentials c ON c.session_family_id = f.session_family_id
		 WHERE c.credential_hash = $1`, hash).
		Scan(&f.SessionFamilyID, &f.AccountID, &f.ProviderID, &f.ClientPlatform,
			&f.AppVersion, &f.DeviceModelClass, &f.DeviceIDHash,
			&f.AbsoluteExpiresAt, &f.CreatedAt, &f.LastRefreshedAt,
			&f.ExpiresAt, &f.RevokedAt, &f.RevokeReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return FamilyRow{}, ErrNotFound
	}
	return f, err
}
