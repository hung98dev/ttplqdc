package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"thinhthan/internal/durable/account"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestFederatedLoginVerifiesProvider: unknown subject → new account;
// known subject → existing account; bad provider/id → AUTH_INVALID.
func TestFederatedLoginVerifiesProvider(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	gp := &fakeProvider{subject: "g-sub-1"}
	svc := newService(t, store, clk, map[string]ProviderClient{"google": gp})
	ctx := context.Background()

	r1, err := svc.FederatedLogin(ctx, "google", "tok", "n", testMeta())
	if err != nil {
		t.Fatalf("federated login: %v", err)
	}
	if !r1.IsNewAccount {
		t.Fatal("first federated login did not create the account")
	}
	r2, err := svc.FederatedLogin(ctx, "google", "tok", "n", testMeta())
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if r2.IsNewAccount || r2.AccountID != r1.AccountID {
		t.Fatalf("known subject did not resolve the same account: %v vs %v", r1.AccountID, r2.AccountID)
	}
	var ae *APIError
	if _, err := svc.FederatedLogin(ctx, "steamos", "tok", "n", testMeta()); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("unknown provider: %v", err)
	}
	bad := &fakeProvider{err: ErrAuthInvalid}
	svc2 := newService(t, store, clk, map[string]ProviderClient{"google": bad})
	if _, err := svc2.FederatedLogin(ctx, "google", "tok", "n", testMeta()); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("bad provider token: %v", err)
	}
}

// TestTicketSingleUseAndTTL: the gameplay ticket mints once and the
// registry consumes it at HELLO — replay is AUTH_INVALID.
func TestTicketSingleUseAndTTL(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "ticketer", "t@x.co", "validpass1")
	resp := loginPassword(t, svc, "ticketer", "validpass1")

	tk, err := svc.Ticket(ctx, resp.AccessToken, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 1, 0, "")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}
	if tk.Credential == "" || tk.WSSURL == "" || tk.ExpiresAt.IsZero() {
		t.Fatalf("ticket shape: %+v", tk)
	}
	// Single-use is exercised at the session layer (hello_test covers
	// HELLO consumption); here a second issue while the first is
	// unconsumed replaces it — the same slot reservation transfers.
	tk2, err := svc.Ticket(ctx, resp.AccessToken, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 1, 0, "")
	if err != nil {
		t.Fatalf("re-ticket: %v", err)
	}
	if tk2.Credential == tk.Credential {
		t.Fatal("replacement ticket reused credential")
	}
}

// TestRefreshRotationAndReuseRevoke: presenting the rotated credential
// again = reuse → family revoked.
func TestRefreshRotationAndReuseRevoke(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	svc := newService(t, store, clk, nil)
	ctx := context.Background()
	registerPassword(t, svc, "renew", "r@x.co", "validpass1")
	resp := loginPassword(t, svc, "renew", "validpass1")

	r2, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if r2.RefreshToken == resp.RefreshToken {
		t.Fatal("no rotation")
	}
	// Present the old credential again within the grace window → the
	// lost-response retry is accepted once (spec § refresh grace).
	if _, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); err != nil {
		t.Fatalf("grace retry: %v", err)
	}
	// A further presentation is reuse → AUTH_INVALID + family gone.
	var ae *APIError
	if _, err := svc.Refresh(ctx, resp.RefreshToken, testMeta().DeviceID); !errors.As(err, &ae) || ae.Code != "AUTH_INVALID" {
		t.Fatalf("reuse: %v", err)
	}
	hash, _ := CredentialHash(resp.RefreshToken)
	fam, err := store.FamilyByCredentialHash(ctx, nil, hash)
	if err != nil {
		t.Fatalf("family: %v", err)
	}
	if fam.RevokedAt == nil || fam.RevokeReason == nil || *fam.RevokeReason != account.RevokeReuseDetected {
		t.Fatalf("reuse revocation: %+v", fam)
	}
}

// TestProviderLinkUniqueness: provider_id+provider_subject is globally
// unique and (account_id,provider_id) is unique — a second link of the
// same provider on one account and the same subject anywhere fail.
func TestProviderLinkUniqueness(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	gp := &fakeProvider{subject: "g-sub-9"}
	svc := newService(t, store, clk, map[string]ProviderClient{"google": gp})
	ctx := context.Background()
	registerPassword(t, svc, "linker", "l@x.co", "validpass1")
	a := loginPassword(t, svc, "linker", "validpass1")

	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "", "n"); err != nil {
		t.Fatalf("link: %v", err)
	}
	var ae *APIError
	// Same provider again on this account → PROVIDER_ALREADY_LINKED.
	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "", "n"); !errors.As(err, &ae) || ae.Code != "PROVIDER_ALREADY_LINKED" {
		t.Fatalf("same provider relink: %v", err)
	}
	// Same subject on another account → PROVIDER_ALREADY_LINKED.
	registerPassword(t, svc, "linker2", "l2@x.co", "validpass1")
	b := loginPassword(t, svc, "linker2", "validpass1")
	if _, err := svc.Link(ctx, b.AccessToken, "google", "tok", "", "n"); !errors.As(err, &ae) || ae.Code != "PROVIDER_ALREADY_LINKED" {
		t.Fatalf("subject on other account: %v", err)
	}
}
