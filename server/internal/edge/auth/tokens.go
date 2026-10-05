package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// Opaque credential helpers (auth.md § Credential Types): every
// client-facing credential is 32 bytes of crypto/rand; only hashes of the
// refresh credential persist.

// NewOpaqueCredential returns a 256-bit URL-safe opaque credential and its
// SHA-256 storage hash.
func NewOpaqueCredential() (cred string, hash []byte, err error) {
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", nil, err
	}
	cred = base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256(raw[:])
	return cred, sum[:], nil
}

// CredentialHash computes the lookup hash for a presented credential —
// SHA-256 over the decoded bytes, matching NewOpaqueCredential.
func CredentialHash(cred string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cred)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}
