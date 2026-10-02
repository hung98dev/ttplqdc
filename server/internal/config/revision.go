package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"thinhthan/internal/core/id"
)

// ComputeContentRevision hashes canonical payload bytes into the semantic
// content revision: lowercase_hex(SHA256) matching ^[0-9a-f]{64}$
// (content_authoring_contract.md §5).
func ComputeContentRevision(payload []byte) (string, error) {
	sum := sha256.Sum256(payload)
	rev := hex.EncodeToString(sum[:])
	if err := id.ValidateContentRevision(rev); err != nil {
		return "", fmt.Errorf("config: computed revision invalid: %w", err)
	}
	return rev, nil
}
