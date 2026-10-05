package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ProviderClient verifies one federated credential and returns the
// provider subject (external_integrations.md § 1-2).
type ProviderClient interface {
	// Verify resolves provider_token → stable subject id.
	Verify(ctx context.Context, token, nonce string) (subject string, err error)
}

// AppleProvider verifies Apple Sign-In ID tokens against
// https://appleid.apple.com/auth/keys (iss https://appleid.apple.com,
// aud ∈ client IDs; the nonce claim must equal SHA-256 of the raw nonce
// when one is supplied).
type AppleProvider struct {
	jwks      *jwksFetcher
	clientIDs []string
	now       func() time.Time
}

// NewAppleProvider binds the verifier; jwksURL is injectable for tests.
func NewAppleProvider(jwksURL string, clientIDs []string, client *http.Client, now func() time.Time) *AppleProvider {
	if jwksURL == "" {
		jwksURL = "https://appleid.apple.com/auth/keys"
	}
	return &AppleProvider{jwks: newJWKSFetcher(jwksURL, client, now), clientIDs: clientIDs, now: now}
}

// Verify implements ProviderClient for Apple.
func (p *AppleProvider) Verify(ctx context.Context, token, nonce string) (string, error) {
	now := time.Now().UTC()
	if p.now != nil {
		now = p.now()
	}
	c, err := verifyOIDCToken(ctx, p.jwks, token, "https://appleid.apple.com", p.clientIDs, now)
	if err != nil {
		return "", err
	}
	if nonce != "" {
		sum := sha256.Sum256([]byte(nonce))
		if c.Nonce == "" || c.Nonce != base64.RawURLEncoding.EncodeToString(sum[:]) {
			return "", ErrAuthInvalid
		}
	}
	return c.Subject, nil
}

// GoogleProvider verifies Google OIDC ID tokens (iss accounts.google.com,
// aud ∈ GOOGLE_OIDC_CLIENT_IDS).
type GoogleProvider struct {
	jwks      *jwksFetcher
	clientIDs []string
	now       func() time.Time
}

// NewGoogleProvider binds the verifier; jwksURL is injectable for tests.
func NewGoogleProvider(jwksURL string, clientIDs []string, client *http.Client, now func() time.Time) *GoogleProvider {
	if jwksURL == "" {
		jwksURL = "https://www.googleapis.com/oauth2/v3/certs"
	}
	return &GoogleProvider{jwks: newJWKSFetcher(jwksURL, client, now), clientIDs: clientIDs, now: now}
}

// Verify implements ProviderClient for Google.
func (p *GoogleProvider) Verify(ctx context.Context, token, _ string) (string, error) {
	now := time.Now().UTC()
	if p.now != nil {
		now = p.now()
	}
	c, err := verifyOIDCToken(ctx, p.jwks, token, "accounts.google.com", p.clientIDs, now)
	if err != nil {
		return "", err
	}
	return c.Subject, nil
}

// SteamProvider verifies a Steam session ticket through
// ISteamUserAuth/AuthenticateUserTicket/v1 (external_integrations.md § 2:
// accept iff result=OK, steamid non-empty, vacbanned=false,
// publisherbanned=false; subject = steamid).
type SteamProvider struct {
	endpoint string
	key      string
	appID    string
	identity string
	client   *http.Client
}

// NewSteamProvider binds the verifier; endpoint is injectable for tests.
func NewSteamProvider(endpoint, publisherKey, appID string, client *http.Client) *SteamProvider {
	if endpoint == "" {
		endpoint = "https://partner.steam-api.com/ISteamUserAuth/AuthenticateUserTicket/v1"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &SteamProvider{endpoint: endpoint, key: publisherKey, appID: appID,
		identity: "thinhthan-login", client: client}
}

// Verify implements ProviderClient for Steam.
func (p *SteamProvider) Verify(ctx context.Context, ticket, _ string) (string, error) {
	form := url.Values{
		"key":      {p.key},
		"appid":    {p.appID},
		"ticket":   {ticket},
		"identity": {p.identity},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", ErrTemporaryDependency
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", ErrTemporaryDependency
	}
	if resp.StatusCode != http.StatusOK {
		return "", ErrTemporaryDependency
	}
	var out struct {
		Response struct {
			Params struct {
				Result          string `json:"result"`
				SteamID         string `json:"steamid"`
				OwnerSteamID    string `json:"ownersteamid"`
				VACBanned       bool   `json:"vacbanned"`
				PublisherBanned bool   `json:"publisherbanned"`
			} `json:"params"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("auth: steam response decode: %w", err)
	}
	p2 := out.Response.Params
	if p2.Result != "OK" || p2.SteamID == "" || p2.VACBanned || p2.PublisherBanned {
		return "", ErrAuthInvalid
	}
	return p2.SteamID, nil
}
