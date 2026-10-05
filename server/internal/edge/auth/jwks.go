package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// jwksSet is a fetched OIDC key set (external_integrations.md § 2: Apple /
// Google JWKS endpoints, RS256, fetched over HTTPS, cached).
type jwksSet struct {
	Keys []struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// jwksFetcher fetches and caches one JWKS document.
type jwksFetcher struct {
	url    string
	client *http.Client

	mu     sync.Mutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
	now    func() time.Time
}

func newJWKSFetcher(url string, client *http.Client, now func() time.Time) *jwksFetcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &jwksFetcher{url: url, client: client, now: now}
}

const jwksCacheTTL = time.Hour

func (f *jwksFetcher) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.keys == nil || f.now().After(f.expiry) {
		if err := f.refreshLocked(ctx); err != nil {
			return nil, err
		}
	}
	k, ok := f.keys[kid]
	if !ok {
		// Key rotation: refetch once before declaring unknown kid.
		if err := f.refreshLocked(ctx); err != nil {
			return nil, err
		}
		if k, ok = f.keys[kid]; !ok {
			return nil, errors.New("auth: unknown jwks kid")
		}
	}
	return k, nil
}

func (f *jwksFetcher) refreshLocked(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks fetch status %d", resp.StatusCode)
	}
	var set jwksSet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("auth: jwks decode: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, jwk := range set.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" || jwk.N == "" || jwk.E == "" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		var eInt int
		for _, b := range eBytes {
			eInt = eInt<<8 | int(b)
		}
		keys[jwk.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: eInt}
	}
	f.keys = keys
	f.expiry = f.now().Add(jwksCacheTTL)
	return nil
}

// oidcClaims is the parsed payload of a provider ID token.
type oidcClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience any    `json:"aud"` // string or []string
	Expiry   int64  `json:"exp"`
	Nonce    string `json:"nonce"`
}

// audContains checks membership for both aud shapes.
func (c oidcClaims) audContains(want string) bool {
	switch a := c.Audience.(type) {
	case string:
		return a == want
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

// verifyOIDCToken verifies an RS256 JWT: header.kid → JWKS key, signature
// over header.payload, iss exact, aud membership, exp in the future.
func verifyOIDCToken(ctx context.Context, fetcher *jwksFetcher, token, wantIss string, wantAud []string, now time.Time) (oidcClaims, error) {
	var zero oidcClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return zero, ErrAuthInvalid
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, ErrAuthInvalid
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "RS256" || header.Kid == "" {
		return zero, ErrAuthInvalid
	}
	key, err := fetcher.key(ctx, header.Kid)
	if err != nil {
		return zero, ErrTemporaryDependency
	}
	signed := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return zero, ErrAuthInvalid
	}
	digest := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return zero, ErrAuthInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, ErrAuthInvalid
	}
	var c oidcClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return zero, ErrAuthInvalid
	}
	if c.Issuer != wantIss || c.Subject == "" || c.Expiry <= now.Unix() {
		return zero, ErrAuthInvalid
	}
	okAud := false
	for _, a := range wantAud {
		if c.audContains(a) {
			okAud = true
			break
		}
	}
	if !okAud {
		return zero, ErrAuthInvalid
	}
	return c, nil
}
