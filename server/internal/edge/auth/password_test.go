package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"thinhthan/internal/durable/account"
)

// TestRegisterValidation covers the 400-class validation of
// auth.md § Password Provider.
func TestRegisterValidation(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()

	cases := []struct {
		name     string
		user     string
		pass     string
		email    string
		wantCode string
	}{
		{"short username", "ab", "validpass1", "a@example.com", "USERNAME_INVALID"},
		{"bad char", "a b c d", "validpass1", "a@example.com", "USERNAME_INVALID"},
		{"digit start", "1abc", "validpass1", "a@example.com", "USERNAME_INVALID"},
		{"long username", strings.Repeat("a", 21), "validpass1", "a@example.com", "USERNAME_INVALID"},
		{"no at", "validuser", "validpass1", "not-an-email", "EMAIL_INVALID"},
		{"empty local", "validuser", "validpass1", "@x.co", "EMAIL_INVALID"},
		{"short password", "validuser", "abc123", "a@example.com", "PASSWORD_INVALID"},
		{"password=username", "validuser", "ValidUser", "a@example.com", "PASSWORD_INVALID"},
	}
	for i, tc := range cases {
		// Vary the device: the IP_DEVICE enumeration bucket (5/h) is checked
		// before validation by design — a shared device exhausts it first.
		meta := testMeta()
		meta.DeviceID = fmt.Sprintf("dev-%d", i)
		_, err := svc.Register(ctx, tc.user, tc.email, tc.pass, meta)
		var ae *APIError
		if !errors.As(err, &ae) || ae.Code != tc.wantCode {
			t.Fatalf("%s: want %s got %v", tc.name, tc.wantCode, err)
		}
	}
	if _, err := svc.Register(ctx, "validuser", "a@example.com", "validpass1", testMeta()); err != nil {
		t.Fatalf("valid register: %v", err)
	}
}

// TestUsernameEmailKeyUniqueness: canonical keys are UNIQUE and
// case-folded — ALICE vs alice collide; register surfaces
// USERNAME_TAKEN / EMAIL_TAKEN (409).
func TestUsernameEmailKeyUniqueness(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "alice_p", "alice@x.co", "validpass1")

	var ae *APIError
	_, err := svc.Register(ctx, "Alice_P", "other@x.co", "validpass1", testMeta())
	if !errors.As(err, &ae) || ae.Code != "USERNAME_TAKEN" || ae.Status != 409 {
		t.Fatalf("dup username: %v", err)
	}
	_, err = svc.Register(ctx, "bobx", " Alice@x.co ", "validpass1", testMeta())
	if !errors.As(err, &ae) || ae.Code != "EMAIL_TAKEN" {
		t.Fatalf("dup email: %v", err)
	}
}

// TestLoginGenericFailure: unknown username and wrong password return
// the identical AUTH_INVALID shape (dummy-hash verify keeps latency
// uniform).
func TestLoginGenericFailure(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "carol", "c@x.co", "validpass1")

	var ae *APIError
	_, err1 := svc.PasswordLogin(ctx, "carol", "wrongpass1", testMeta())
	_, err2 := svc.PasswordLogin(ctx, "nosuch", "wrongpass1", testMeta())
	if !errors.As(err1, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("wrong pass: %v", err1)
	}
	if !errors.As(err2, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("unknown user: %v", err2)
	}
	if err1.(*APIError).Status != err2.(*APIError).Status ||
		err1.(*APIError).SafeMessageKey != err2.(*APIError).SafeMessageKey {
		t.Fatalf("failure shapes differ: %v vs %v", err1, err2)
	}
}

// TestArgon2idParamsAndRehash: PHC params match spec and a login on a
// stale params_version row rehashes to the current version.
func TestArgon2idParamsAndRehash(t *testing.T) {
	hash, err := HashPassword("validpass1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("phc params: %s", hash)
	}
	ok, err := VerifyPassword("validpass1", hash)
	if err != nil || !ok {
		t.Fatalf("verify: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword("validpass2", hash); ok {
		t.Fatal("wrong password verified")
	}

	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	resp := registerPassword(t, svc, "dora", "d@x.co", "validpass1")
	// Simulate a legacy params_version row.
	if _, err := store.Pool().Exec(ctx,
		`UPDATE account_password_credentials SET params_version = 0 WHERE account_id = $1`,
		resp.AccountID); err != nil {
		t.Fatalf("downgrade version: %v", err)
	}
	loginPassword(t, svc, "dora", "validpass1")
	row, err := store.GetPasswordByAccount(ctx, nil, resp.AccountID)
	if err != nil {
		t.Fatalf("get cred: %v", err)
	}
	if row.ParamsVersion != ArgonParamsVersion {
		t.Fatalf("rehash on login: version=%d", row.ParamsVersion)
	}
}

// TestPasswordChangeRevokesOtherSessions: password/change revokes all
// OTHER session families (PASSWORD_CHANGE) and returns fresh creds on
// the surviving one.
func TestPasswordChangeRevokesOtherSessions(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "erin", "e@x.co", "validpass1")
	a := loginPassword(t, svc, "erin", "validpass1")
	b := loginPassword(t, svc, "erin", "validpass1")

	out, err := svc.PasswordChange(ctx, a.AccessToken, "validpass1", "newpass123")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if out.RefreshToken == a.RefreshToken {
		t.Fatal("refresh not rotated on change")
	}
	// Family B's refresh is dead.
	var ae *APIError
	if _, err := svc.Refresh(ctx, b.RefreshToken, testMeta().DeviceID); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("revoked family refresh: %v", err)
	}
	bHash, ok := CredentialHash(b.RefreshToken)
	if !ok {
		t.Fatal("credential hash")
	}
	fam, err := store.FamilyByCredentialHash(ctx, nil, bHash)
	if err != nil {
		t.Fatalf("family: %v", err)
	}
	// Either reason proves the revocation: the 1h takeover window (register
	// was a new-origin login) revokes first, else the password change does.
	if fam.RevokedAt == nil || fam.RevokeReason == nil ||
		(*fam.RevokeReason != account.RevokePasswordChange &&
			*fam.RevokeReason != account.RevokeTakeoverRule) {
		t.Fatalf("revocation: %+v", fam.RevokedAt)
	}
	// The surviving family still refreshes.
	if _, err := svc.Refresh(ctx, out.RefreshToken, testMeta().DeviceID); err != nil {
		t.Fatalf("surviving family refresh: %v", err)
	}
}
