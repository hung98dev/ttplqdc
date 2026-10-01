package id

import (
	"fmt"
	"log/slog"
)

// ValidateContentRevision enforces the canonical content revision form:
// exactly 64 lowercase hexadecimal SHA-256 characters matching
// ^[0-9a-f]{64}$ — never prefixed, numeric or truncated
// (content_authoring_contract.md § Semantic Content Revision).
func ValidateContentRevision(rev string) error {
	if len(rev) != 64 {
		return fmt.Errorf("id: content revision %q: want 64 lowercase hex chars", rev)
	}
	for i := 0; i < len(rev); i++ {
		c := rev[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return fmt.Errorf("id: content revision %q: want 64 lowercase hex chars", rev)
		}
	}
	return nil
}

// RevisionLogAttr returns the mandatory "revision" structured-logging
// attribute (engineering_conventions.md §1.2) carrying the content revision
// through compile/runtime diagnostic context.
func RevisionLogAttr(rev string) slog.Attr {
	return slog.String("revision", rev)
}
