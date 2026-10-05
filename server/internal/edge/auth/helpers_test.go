package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/edge/session"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

// fakeClock is the injected Now for time-driven tests.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// fakeProvider answers Verify with a fixed subject.
type fakeProvider struct {
	subject  string
	err      error
	gotToken string
	gotNonce string
}

func (f *fakeProvider) Verify(_ context.Context, token, nonce string) (string, error) {
	f.gotToken, f.gotNonce = token, nonce
	if f.err != nil {
		return "", f.err
	}
	return f.subject, nil
}

// VerifyFresh implements FreshnessProvider with a fresh iat.
func (f *fakeProvider) VerifyFresh(ctx context.Context, token, nonce string) (string, time.Time, error) {
	sub, err := f.Verify(ctx, token, nonce)
	return sub, time.Now().UTC(), err
}

// testMeta returns a stable login context.
func testMeta() LoginMeta {
	return LoginMeta{
		IP:         net.ParseIP("10.44.0.7"),
		DeviceID:   "550e8400-e29b-41d4-a716-446655440000",
		Platform:   "WINDOWS",
		AppVersion: "1.0.0",
	}
}

var testSalt = func() account.Salt {
	var s account.Salt
	for i := range s {
		s[i] = byte(i)
	}
	return s
}()

// newTestStore migrates a fresh database.
func newTestStore(t *testing.T) *account.Store {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.FreshDB(t)
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir("../../../.."), "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return account.NewStore(pool)
}

// newService builds a Service over a fresh store with the fake clock and
// (optionally) the given providers.
func newService(t *testing.T, store *account.Store, clk *fakeClock, providers map[string]ProviderClient) *Service {
	t.Helper()
	q := queue.New(8, idempotency.NewStore(store.Pool()), queue.Deps{Pool: store.Pool(), Now: clk.Now})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Shutdown(ctx)
	})
	reg := session.New(session.Config{Capacity: 2, Now: clk.Now}, store, q)
	return New(Config{
		Salt: testSalt, Store: store, Providers: providers,
		Tickets: reg, Revoker: reg, WSSURL: "wss://test/ws",
		Now: clk.Now,
	})
}

// issueAccountAccess registers a password account and logs in once,
// returning the service claims the handlers use.
func loginPassword(t *testing.T, svc *Service, username, password string) TokenResponse {
	t.Helper()
	resp, err := svc.PasswordLogin(context.Background(), username, password, testMeta())
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return resp
}

// registerPassword registers one password account.
func registerPassword(t *testing.T, svc *Service, username, email, password string) TokenResponse {
	t.Helper()
	resp, err := svc.Register(context.Background(), username, email, password, testMeta())
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return resp
}

// vararg account ids for repeated seeding.
var _ = id.NewV7
var _ = protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

// ---------------------------------------------------------------------------
// JWKS / Steam stubs

// appleKeys is a test RS256 signer + JWKS endpoint.
type appleKeys struct {
	priv *rsa.PrivateKey
	kid  string
}

// newAppleJWKS serves a one-key JWKS document over httptest.
func newAppleJWKS(t *testing.T) (*appleKeys, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	k := &appleKeys{priv: priv, kid: "k1"}
	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"%s","use":"sig","alg":"RS256","n":"%s","e":"%s"}]}`,
			k.kid, n, e)
	}))
	t.Cleanup(srv.Close)
	return k, srv.URL
}

// nonceClaimValue mirrors Apple's contract: the nonce claim equals
// base64url(SHA-256(rawNonce)).
func nonceClaimValue(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// sign mints an RS256 JWT from claims with the test kid header.
func (k *appleKeys) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	hdr, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": k.kid, "typ": "JWT"})
	cl, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(hdr) + "." +
		base64.RawURLEncoding.EncodeToString(cl)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// newSteamStub serves the AuthenticateUserTicket endpoint; responder
// returns the JSON body.
func newSteamStub(t *testing.T, responder func(map[string]string) map[string]any) (endpoint string, closeFn func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form := map[string]string{}
		for k := range r.Form {
			form[k] = r.Form.Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(responder(form))
		_, _ = w.Write(b)
	}))
	return srv.URL, srv.Close
}
