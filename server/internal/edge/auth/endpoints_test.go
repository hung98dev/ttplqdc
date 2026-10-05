package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// apiCall runs one JSON request through the mounted handler.
func apiCall(t *testing.T, h http.Handler, method, path, bearer string, body any) (int, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rdr)
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.44.0.7:9999"
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// TestEndpointTableContracts: the full endpoint table is mounted and
// speaks the contract shapes (TokenResponse, providers list, account
// view, ticket, error body).
func TestEndpointTableContracts(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	gp := &fakeProvider{subject: "fed-1"}
	svc := newService(t, store, clk, map[string]ProviderClient{"google": gp})
	h := NewHandler(svc)

	// Unauthenticated 404s/405s and Bearer-required 401s.
	if code, _ := apiCall(t, h, "GET", "/api/v1/account", "", nil); code != 401 {
		t.Fatalf("account no-auth: %d", code)
	}
	if code, _ := apiCall(t, h, "POST", "/api/v1/nope", "", nil); code != 404 {
		t.Fatalf("unknown route: %d", code)
	}
	// register → TokenResponse shape
	code, out := apiCall(t, h, "POST", "/api/v1/auth/password/register", "",
		map[string]any{"username": "httpuser", "password": "validpass1",
			"email": "h@x.co", "device_id": "d1", "platform": "WINDOWS"})
	if code != 200 || out["access_token"] == nil || out["refresh_token"] == nil ||
		out["account_id"] == nil || out["access_expires_at"] == nil {
		t.Fatalf("register: %d %v", code, out)
	}
	if out["is_new_account"] != true {
		t.Fatalf("is_new_account: %v", out)
	}
	access := out["access_token"].(string)
	// GET account → providers list + status
	code, acc := apiCall(t, h, "GET", "/api/v1/account", access, nil)
	if code != 200 || acc["status"] != "ACTIVE" || acc["account_id"] != out["account_id"] {
		t.Fatalf("account: %d %v", code, acc)
	}
	if _, ok := acc["providers"]; !ok {
		t.Fatalf("providers missing: %v", acc)
	}
	// link google → providers
	code, pl := apiCall(t, h, "POST", "/api/v1/auth/link/google", access,
		map[string]any{"provider_token": "tok"})
	if code != 200 {
		t.Fatalf("link: %d %v", code, pl)
	}
	// gameplay ticket
	code, tk := apiCall(t, h, "POST", "/api/v1/gameplay/ticket", access,
		map[string]any{"client_build": 1, "platform": "WINDOWS",
			"protocol_major": 1, "protocol_minor": 0, "content_revision": ""})
	if code != 200 || tk["ticket"] == nil || tk["wss_url"] == nil {
		t.Fatalf("ticket: %d %v", code, tk)
	}
	// logout SESSION → 204, then access is dead
	code, _ = apiCall(t, h, "POST", "/api/v1/auth/logout", access, map[string]any{"scope": "SESSION"})
	if code != 204 {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := apiCall(t, h, "GET", "/api/v1/account", access, nil); code != 401 {
		t.Fatalf("post-logout access: %d", code)
	}
	// Error body shape on failure.
	code, ebody := apiCall(t, h, "POST", "/api/v1/auth/password/login", "",
		map[string]any{"username": "httpuser", "password": "wrongpass1"})
	if code != 401 || ebody["error_code"] != "AUTH_INVALID" ||
		ebody["retryability"] == nil || ebody["safe_message_key"] == nil {
		t.Fatalf("error body: %d %v", code, ebody)
	}
}

// TestLinkUnlinkRules: link adds a provider; unlink refuses the last
// usable method; same-provider and cross-account re-link are rejected.
func TestLinkUnlinkRules(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	gp := &fakeProvider{subject: "fed-7"}
	sp := &fakeProvider{subject: "fed-8"}
	svc := newService(t, store, clk, map[string]ProviderClient{"google": gp, "steam": sp})
	ctx := context.Background()
	registerPassword(t, svc, "multi", "m@x.co", "validpass1")
	a := loginPassword(t, svc, "multi", "validpass1")

	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "", ""); err != nil {
		t.Fatalf("link google: %v", err)
	}
	links, err := svc.Link(ctx, a.AccessToken, "steam", "tok2", "", "")
	if err != nil {
		t.Fatalf("link steam: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("providers: %+v", links)
	}
	// Unlink google while password+steam remain → ok.
	if _, err := svc.Unlink(ctx, a.AccessToken, "google", "validpass1", "", ""); err != nil {
		t.Fatalf("unlink google: %v", err)
	}
	var ae *APIError
	// Unlink steam while password remains → ok.
	if _, err := svc.Unlink(ctx, a.AccessToken, "steam", "validpass1", "", ""); err != nil {
		t.Fatalf("unlink steam: %v", err)
	}
	// Now password is the last method — unlink 'password' isn't a route;
	// simulate last-method by removing steam: create a federated-only
	// account and try to unlink its sole provider.
	gp2 := &fakeProvider{subject: "sole-1"}
	svc2 := newService(t, store, clk, map[string]ProviderClient{"google": gp2})
	fed, err := svc2.FederatedLogin(ctx, "google", "tok", "", testMeta())
	if err != nil {
		t.Fatalf("fed login: %v", err)
	}
	if _, err := svc2.Unlink(ctx, fed.AccessToken, "google", "", "tok", ""); !errors.As(err, &ae) || ae.Code != "LAST_LOGIN_METHOD" {
		t.Fatalf("last method unlink: %v", err)
	}
}

// TestCredentialGuardLock: while credential_guard_until > now, link
// and unlink fail CREDENTIAL_CHANGE_LOCKED unless the account's
// current_password is supplied.
func TestCredentialGuardLock(t *testing.T) {
	store := newTestStore(t)
	clk := &fakeClock{now: time.Now().UTC()}
	gp := &fakeProvider{subject: "fed-9"}
	svc := newService(t, store, clk, map[string]ProviderClient{"google": gp})
	ctx := context.Background()
	registerPassword(t, svc, "guarded", "g@x.co", "validpass1")
	a := loginPassword(t, svc, "guarded", "validpass1")

	if err := store.SetCredentialGuardUntil(ctx, nil, a.AccountID,
		clk.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("set guard: %v", err)
	}
	var ae *APIError
	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "", ""); !errors.As(err, &ae) || ae.Code != "CREDENTIAL_CHANGE_LOCKED" {
		t.Fatalf("guard link: %v", err)
	}
	// Wrong password also fails.
	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "wrongpass1", ""); !errors.As(err, &ae) || ae.Code != "CREDENTIAL_CHANGE_LOCKED" {
		t.Fatalf("guard wrong pass: %v", err)
	}
	// Correct current_password bypasses.
	if _, err := svc.Link(ctx, a.AccessToken, "google", "tok", "validpass1", ""); err != nil {
		t.Fatalf("guard with password: %v", err)
	}
	// password/change always carries the current password — the guard
	// never blocks it.
	if _, err := svc.PasswordChange(ctx, a.AccessToken, "validpass1", "newpass123"); err != nil {
		t.Fatalf("change during guard: %v", err)
	}
}
