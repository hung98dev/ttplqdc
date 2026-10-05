package auth

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"thinhthan/internal/durable/account"
)

// TestRefreshFamilyPersistsAcrossRestart: refresh credentials and
// session families live in PostgreSQL — a new Service over the same
// store (a restart) still rotates them.
func TestRefreshFamilyPersistsAcrossRestart(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "keep", "k@x.co", "validpass1")
	resp := loginPassword(t, svc, "keep", "validpass1")

	// "Restart": fresh in-memory state, same store.
	svc2 := newService(t, store, clk, nil)
	if _, err := svc2.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); err != nil {
		t.Fatalf("post-restart refresh: %v", err)
	}
}

// TestAccessTokenInvalidAfterRestart: access tokens are in-memory
// only — a restart forces a refresh.
func TestAccessTokenInvalidAfterRestart(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	registerPassword(t, svc, "gone", "g2@x.co", "validpass1")
	resp := loginPassword(t, svc, "gone", "validpass1")

	svc2 := newService(t, store, clk, nil)
	if _, err := svc2.ValidateAccess(resp.AccessToken); !errors.Is(err, ErrAuthInvalid) {
		t.Fatalf("access after restart: %v", err)
	}
}

// TestLoginHistoryNewOrigin: one row per successful login; the
// (device, ip-prefix) pair unseen in 90 days is is_new_origin.
func TestLoginHistoryNewOrigin(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "hist", "hi@x.co", "validpass1")

	loginPassword(t, svc, "hist", "validpass1") // origin row #1 (new)
	loginPassword(t, svc, "hist", "validpass1") // same pair → not new

	var count, newCount int
	if err := store.Pool().QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE is_new_origin)
		 FROM account_login_history`).Scan(&count, &newCount); err != nil {
		t.Fatalf("history count: %v", err)
	}
	// register login (new) + login1 (same pair, also is_new_origin? no —
	// same device+ip as register → not new) + login2 same → total 3
	// rows with exactly 1 is_new_origin.
	if count != 3 || newCount != 1 {
		t.Fatalf("history rows: count=%d new=%d", count, newCount)
	}
	// A new device+IP is a new origin.
	m2 := testMeta()
	m2.DeviceID = "550e8400-e29b-41d4-a716-446655440099"
	m2.IP = net.ParseIP("192.168.0.1")
	if _, err := svc.PasswordLogin(ctx, "hist", "validpass1", m2); err != nil {
		t.Fatalf("new-origin login: %v", err)
	}
	if err := store.Pool().QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE is_new_origin) FROM account_login_history`).
		Scan(&newCount); err != nil {
		t.Fatalf("recount: %v", err)
	}
	if newCount != 2 {
		t.Fatalf("new-origin count=%d", newCount)
	}
}

// TestTakeoverRuleSetsCredentialGuard: a password change within 1 h of
// an is_new_origin login revokes all OTHER session families, sets
// credential_guard_until = now+24 h, and emits ACCOUNT_SECURITY_REVIEW.
func TestTakeoverRuleSetsCredentialGuard(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "victim2", "v2@x.co", "validpass1")

	// New-origin login (fresh device+ip).
	mNew := testMeta()
	mNew.DeviceID = "550e8400-e29b-41d4-a716-4466554400ff"
	mNew.IP = net.ParseIP("172.20.1.9")
	resp, err := svc.PasswordLogin(ctx, "victim2", "validpass1", mNew)
	if err != nil {
		t.Fatalf("new-origin login: %v", err)
	}
	// A second family to be revoked by the takeover rule.
	other := loginPassword(t, svc, "victim2", "validpass1")

	out, err := svc.PasswordChange(ctx, resp.AccessToken, "validpass1", "newpass123")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	_ = out
	arow, err := store.GetAccount(ctx, nil, resp.AccountID)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	if arow.CredentialGuardUntil == nil {
		t.Fatal("credential_guard_until not set")
	}
	if d := arow.CredentialGuardUntil.Sub(clk.Now()); d < 23*time.Hour || d > 24*time.Hour {
		t.Fatalf("guard window: %v", d)
	}
	// Other family revoked under TAKEOVER_RULE.
	hash, _ := CredentialHash(other.RefreshToken)
	fam, err := store.FamilyByCredentialHash(ctx, nil, hash)
	if err != nil {
		t.Fatalf("family: %v", err)
	}
	if fam.RevokedAt == nil || fam.RevokeReason == nil || *fam.RevokeReason != account.RevokeTakeoverRule {
		t.Fatalf("takeover revoke: %+v", fam)
	}
	// Audit row emitted.
	var auditCount int
	if err := store.Pool().QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action = 'ACCOUNT_SECURITY_REVIEW'`).
		Scan(&auditCount); err != nil {
		t.Fatalf("audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count=%d", auditCount)
	}
}
