package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"thinhthan/internal/durable/account"
)

// TestRefreshLostResponseGrace: when the client never received
// generation N (issued ≤60 s ago, never presented), presenting N−1 is
// accepted once — N is marked rotated and N+1 issued; presenting N−1 a
// second time is reuse.
func TestRefreshLostResponseGrace(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "grace", "gr@x.co", "validpass1")
	resp := loginPassword(t, svc, "grace", "validpass1") // gen 1

	// Rotate to gen 2 — pretend the response was lost.
	r2, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID)
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	// Present gen 1 within the grace window → accepted once → gen 3.
	r3, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID)
	if err != nil {
		t.Fatalf("grace rotate: %v", err)
	}
	if r3.RefreshToken == r2.RefreshToken || r3.RefreshToken == resp.RefreshToken {
		t.Fatal("grace did not issue N+1")
	}
	// Present gen 1 yet again → reuse → family revoked.
	var ae *APIError
	if _, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("post-grace replay: %v", err)
	}
	// A different device_id does not qualify for the grace rule: an N-1
	// retry from another device is reuse, not grace.
	registerPassword(t, svc, "grace2", "g2@x.co", "validpass1")
	resp2 := loginPassword(t, svc, "grace2", "validpass1")
	if _, err := svc.Refresh(ctx, resp2.RefreshToken, testMeta().DeviceID); err != nil {
		t.Fatalf("rotate2: %v", err)
	}
	var ae2 *APIError
	if _, err := svc.Refresh(ctx, resp2.RefreshToken, "other-device"); !errors.As(err, &ae2) || ae2.Code != "AUTH_INVALID" {
		t.Fatalf("cross-device grace: %v", err)
	}
}

// TestRefreshReuseRevokesFamily: any other replay of a rotated
// generation revokes the whole session family (REUSE_DETECTED).
func TestRefreshReuseRevokesFamily(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "reuse", "ru@x.co", "validpass1")
	resp := loginPassword(t, svc, "reuse", "validpass1")

	if _, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	// Advance past the 60 s grace so the replay can only be reuse.
	clk.Advance(61 * time.Second)
	var ae *APIError
	if _, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("reuse: %v", err)
	}
	hash, _ := CredentialHash(resp.RefreshToken)
	fam, err := store.FamilyByCredentialHash(ctx, nil, hash)
	if err != nil {
		t.Fatalf("family: %v", err)
	}
	if fam.RevokedAt == nil || fam.RevokeReason == nil || *fam.RevokeReason != "REUSE_DETECTED" {
		t.Fatalf("reuse revoke: %+v", fam)
	}
	// The family's newest credential is dead too.
	if _, err := svc.Refresh(ctx, "anything", testMeta().DeviceID); err == nil {
		t.Fatal("post-revoke refresh succeeded")
	}
}

// TestRefreshAbsoluteCap90Days: a family never refreshes past 90 days
// after initial login — AUTH_EXPIRED.
func TestRefreshAbsoluteCap90Days(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "oldtimer", "o@x.co", "validpass1")
	resp := loginPassword(t, svc, "oldtimer", "validpass1")

	// Stay inside the 30d refresh TTL while the family ages toward the cap.
	tok := resp.RefreshToken
	var r TokenResponse
	var err error
	for _, day := range []int{29, 58, 87} {
		clk.Advance(29 * 24 * time.Hour)
		r, err = svc.Refresh(ctx, tok, testMeta().DeviceID)
		if err != nil {
			t.Fatalf("day-%d refresh: %v", day, err)
		}
		tok = r.RefreshToken
	}
	clk.Advance(4 * 24 * time.Hour) // day 91
	var ae *APIError
	if _, err := svc.Refresh(ctx, tok, testMeta().DeviceID); !errors.As(err, &ae) || ae.Code != "AUTH_EXPIRED" {
		t.Fatalf("day-91 refresh: %v", err)
	}
}

// TestLockedUsernameCorrectPasswordSucceeds: the progressive lock never
// blocks a correct password — it bypasses and clears the counters.
func TestLockedUsernameCorrectPasswordSucceeds(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "right", "ri@x.co", "validpass1")
	for i := 0; i < 6; i++ {
		_, _ = svc.PasswordLogin(ctx, "right", "wrongpass1", testMeta())
	}
	if _, err := svc.PasswordLogin(ctx, "right", "validpass1", testMeta()); err != nil {
		t.Fatalf("correct password under lock: %v", err)
	}
	// Counters cleared: next failure is failure #1 again.
	var ae *APIError
	if _, err := svc.PasswordLogin(ctx, "right", "wrongpass1", testMeta()); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("post-clear failure shape: %v", err)
	}
}

// TestBackoffDecay: the failure count decays −1 per 10 minutes of
// quiet; a stale lock decays until it clears.
func TestBackoffDecay(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	lim := NewLimiter(store, testSalt, clk.Now)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = lim.RecordFailure(ctx, ActionPasswordLogin, ScopeUsername, "decay")
	}
	if locked, _, _ := lim.BackoffLocked(ctx, ActionPasswordLogin, ScopeUsername, "decay"); !locked {
		t.Fatal("expected lock after 5 failures")
	}
	// 10 min quiet → 4 effective failures → lock cleared on read.
	clk.Advance(10 * time.Minute)
	locked, _, err := lim.BackoffLocked(ctx, ActionPasswordLogin, ScopeUsername, "decay")
	if err != nil || locked {
		t.Fatalf("decay: locked=%v err=%v", locked, err)
	}
}

// TestSuccessClearsIpRow: a successful login clears the USERNAME
// counter + both backoff rows (USERNAME and source-IP) but never the
// IP request-capacity counter.
func TestSuccessClearsIpRow(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "clear", "cl@x.co", "validpass1")

	for i := 0; i < 3; i++ {
		_, _ = svc.PasswordLogin(ctx, "clear", "wrongpass1", testMeta())
	}
	ipRow := testSalt.RateKey(ActionPasswordLogin, ScopeIP, ipScope(testMeta().IP))
	unRow := testSalt.RateKey(ActionPasswordLogin, ScopeUsername, "clear")
	if _, err := store.GetBackoff(ctx, nil, ipRow[:]); err != nil {
		t.Fatalf("ip backoff missing: %v", err)
	}
	if _, err := store.GetBackoff(ctx, nil, unRow[:]); err != nil {
		t.Fatalf("username backoff missing: %v", err)
	}
	if _, err := svc.PasswordLogin(ctx, "clear", "validpass1", testMeta()); err != nil {
		t.Fatalf("success: %v", err)
	}
	for _, k := range [][32]byte{ipRow, unRow} {
		if _, err := store.GetBackoff(ctx, nil, k[:]); !errors.Is(err, account.ErrNotFound) {
			t.Fatalf("backoff not cleared: %v", err)
		}
	}
	// The IP capacity counter stays.
	ipCap := testSalt.RateKey(ActionPasswordLogin, ScopeIP, ipScope(testMeta().IP))
	if _, err := store.GetCounter(ctx, nil, ipCap[:]); err != nil {
		t.Fatalf("ip capacity counter deleted: %v", err)
	}
}

// TestRateLimitKeysHmacSalted: a request wrote its counter row under
// the exact HMAC key (already covered in TestScopedKeys; here verified
// end-to-end against rate_limit_counters).
func TestRateLimitKeysHmacSalted(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	lim := NewLimiter(store, testSalt, clk.Now)
	ctx := context.Background()
	if err := lim.Check(ctx, ActionRefresh, ScopeAccount, "acct-key", 10, time.Minute); err != nil {
		t.Fatalf("check: %v", err)
	}
	key := testSalt.RateKey(ActionRefresh, ScopeAccount, "acct-key")
	row, err := store.GetCounter(ctx, nil, key[:])
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	if row.CurrentCount != 1 {
		t.Fatalf("count: %d", row.CurrentCount)
	}
}

// TestAppleNonceRequired: the Apple verifier rejects a token whose
// nonce claim does not match the presented raw nonce (RS256 over the
// fetcher's keys).
func TestAppleNonceRequired(t *testing.T) {
	ks, jwksURL := newAppleJWKS(t)
	apple := NewAppleProvider(jwksURL, []string{"aud-ok"}, nil, nil)
	ctx := context.Background()

	good := ks.sign(t, map[string]any{
		"iss": "https://appleid.apple.com", "aud": "aud-ok",
		"sub": "apple-sub", "exp": time.Now().Add(time.Hour).Unix(),
		"nonce": nonceClaimValue("raw-nonce"),
	})
	if _, err := apple.Verify(ctx, good, "raw-nonce"); err != nil {
		t.Fatalf("valid apple token: %v", err)
	}
	// Token without the nonce claim fails when a nonce is presented.
	non := ks.sign(t, map[string]any{
		"iss": "https://appleid.apple.com", "aud": "aud-ok",
		"sub": "apple-sub", "exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := apple.Verify(ctx, non, "raw-nonce"); !errors.Is(err, ErrAuthInvalid) {
		t.Fatalf("missing nonce claim: %v", err)
	}
	// Mismatched nonce fails too.
	bad := ks.sign(t, map[string]any{
		"iss": "https://appleid.apple.com", "aud": "aud-ok",
		"sub": "apple-sub", "exp": time.Now().Add(time.Hour).Unix(),
		"nonce": nonceClaimValue("other"),
	})
	if _, err := apple.Verify(ctx, bad, "raw-nonce"); !errors.Is(err, ErrAuthInvalid) {
		t.Fatalf("nonce mismatch: %v", err)
	}
}

// TestSteamIdentityString: the Steam verifier posts identity=
// "thinhthan-login" with key/appid/ticket and accepts only a clean
// params block (result=OK, steamid set, no bans).
func TestSteamIdentityString(t *testing.T) {
	var gotForm map[string]string
	ep, closeFn := newSteamStub(t, func(form map[string]string) map[string]any {
		gotForm = form
		return map[string]any{"response": map[string]any{"params": map[string]any{
			"result": "OK", "steamid": "76561198000000000",
			"vacbanned": false, "publisherbanned": false,
		}}}
	})
	defer closeFn()
	steam := NewSteamProvider(ep, "pub-key", "440", nil)
	sub, err := steam.Verify(context.Background(), "ticket-hex", "")
	if err != nil {
		t.Fatalf("steam verify: %v", err)
	}
	if sub != "76561198000000000" {
		t.Fatalf("subject: %s", sub)
	}
	if gotForm["identity"] != "thinhthan-login" || gotForm["key"] != "pub-key" ||
		gotForm["appid"] != "440" || gotForm["ticket"] != "ticket-hex" {
		t.Fatalf("steam form: %v", gotForm)
	}
	// A banned/bad result rejects.
	ep2, close2 := newSteamStub(t, func(form map[string]string) map[string]any {
		return map[string]any{"response": map[string]any{"params": map[string]any{
			"result": "OK", "steamid": "76561198000000000",
			"vacbanned": true, "publisherbanned": false,
		}}}
	})
	defer close2()
	steam2 := NewSteamProvider(ep2, "pub-key", "440", nil)
	if _, err := steam2.Verify(context.Background(), "ticket-hex", ""); !errors.Is(err, ErrAuthInvalid) {
		t.Fatalf("vacbanned accept: %v", err)
	}
}
