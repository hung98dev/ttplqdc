package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

// TestScopedKeys: rate-limit keys are HMAC-SHA-256 of
// action:scope_kind:scope_value under ACCOUNT_SIGNAL_SALT — distinct
// per tuple, never reversible.
func TestScopedKeys(t *testing.T) {
	k1 := testSalt.RateKey(ActionPasswordLogin, ScopeUsername, "alice")
	k2 := testSalt.RateKey(ActionPasswordLogin, ScopeUsername, "bob")
	k3 := testSalt.RateKey(ActionPasswordLogin, ScopeIP, "10.0.0.1")
	k4 := testSalt.RateKey(ActionPasswordChange, ScopeUsername, "alice")
	if k1 == k2 || k1 == k3 || k1 == k4 {
		t.Fatal("scope keys collide across tuples")
	}
	m := hmac.New(sha256.New, testSalt[:])
	m.Write([]byte("auth.password.login:USERNAME:alice"))
	var want [32]byte
	copy(want[:], m.Sum(nil))
	if k1 != want {
		t.Fatal("key is not HMAC-SHA-256(salt, action:scope:value)")
	}
}

// TestSlidingWindowApproximation: effective = current + previous·
// (1−elapsed/window) — the aligned-bucket approximation of §3.
func TestSlidingWindowApproximation(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Date(2030, 6, 1, 12, 0, 20, 0, time.UTC)}
	lim := NewLimiter(store, testSalt, clk.Now)
	ctx := context.Background()

	// Fill 10 in window [12:00,12:01).
	for i := 0; i < 10; i++ {
		if err := lim.Check(ctx, ActionRefresh, ScopeAccount, "acct-1", 10, time.Minute); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	// 11th in the same window rejects.
	var ae *APIError
	if err := lim.Check(ctx, ActionRefresh, ScopeAccount, "acct-1", 10, time.Minute); !errors.As(err, &ae) || ae.Code != "RATE_LIMITED" {
		t.Fatalf("overflow: %v", err)
	}
	// Midway through the next window: previous contributes 10·0.5 = 5
	// → 5 more fit.
	clk.now = clk.now.Add(70 * time.Second)
	for i := 0; i < 5; i++ {
		if err := lim.Check(ctx, ActionRefresh, ScopeAccount, "acct-1", 10, time.Minute); err != nil {
			t.Fatalf("post-window %d: %v", i, err)
		}
	}
	if err := lim.Check(ctx, ActionRefresh, ScopeAccount, "acct-1", 10, time.Minute); !errors.As(err, &ae) || ae.Code != "RATE_LIMITED" {
		t.Fatalf("approx overflow: %v", err)
	}
}

// TestProgressiveBackoffSchedule: from the 5th consecutive verified
// failure the lock is min(30s·2^(n−5), 900s).
func TestProgressiveBackoffSchedule(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	lim := NewLimiter(store, testSalt, clk.Now)
	ctx := context.Background()

	want := []time.Duration{0, 0, 0, 0, 30 * time.Second, 60 * time.Second, 120 * time.Second}
	for i, w := range want {
		if err := lim.RecordFailure(ctx, ActionPasswordLogin, ScopeUsername, "victim"); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		locked, retry, err := lim.BackoffLocked(ctx, ActionPasswordLogin, ScopeUsername, "victim")
		if err != nil {
			t.Fatalf("locked: %v", err)
		}
		if w == 0 {
			if locked {
				t.Fatalf("failure %d: unexpectedly locked", i+1)
			}
			continue
		}
		if !locked {
			t.Fatalf("failure %d: not locked", i+1)
		}
		if retry > w || retry < w-2*time.Second {
			t.Fatalf("failure %d: retry %v want ~%v", i+1, retry, w)
		}
	}
	// Cap: keep failing until the schedule hits 900 s.
	for i := 0; i < 30; i++ {
		_ = lim.RecordFailure(ctx, ActionPasswordLogin, ScopeUsername, "victim")
	}
	_, retry, err := lim.BackoffLocked(ctx, ActionPasswordLogin, ScopeUsername, "victim")
	if err != nil || retry <= 0 || retry > 900*time.Second {
		t.Fatalf("cap: retry=%v err=%v", retry, err)
	}
}

// TestLockedLoginSameShape: while the username lock holds, a wrong
// password returns RATE_LIMITED (never AUTH_INVALID) with retry_after —
// same outward shape as other lock delays.
func TestLockedLoginSameShape(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "victim", "v@x.co", "validpass1")

	// Five verified failures arm the lock.
	for i := 0; i < 5; i++ {
		_, err := svc.PasswordLogin(ctx, "victim", "wrongpass1", testMeta())
		if err == nil {
			t.Fatalf("failure %d unexpectedly logged in", i)
		}
	}
	// Wrong password while locked → RATE_LIMITED w/ retry_after.
	var ae *APIError
	_, err := svc.PasswordLogin(ctx, "victim", "wrongpass1", testMeta())
	if !errors.As(err, &ae) || ae.Code != "RATE_LIMITED" || ae.RetryAfterMs <= 0 {
		t.Fatalf("locked failure shape: %+v", err)
	}
}
