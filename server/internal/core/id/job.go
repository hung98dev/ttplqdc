package id

import (
	"encoding/hex"
	"strings"
)

// ServerJobOperationID derives the deterministic operation ID of a
// server-initiated job: UUIDv5 over ServerJobNamespaceUUID and
// "<family>:<job_key>" where job_key is the ':'-joined typed components
// (ids.md § Operation IDs / § Closed Server Producer Families, ADR-0070).
// The family and every component must be non-empty canonical ASCII without
// ':'; an invalid input yields the nil UUID, which every admission path
// rejects as malformed. A crash retry recomputes the identical ID.
func ServerJobOperationID(family string, components ...string) UUID {
	if !validJobComponent(family) || len(components) == 0 {
		return UUID{}
	}
	var b strings.Builder
	b.WriteString(family)
	for _, c := range components {
		if !validJobComponent(c) {
			return UUID{}
		}
		b.WriteByte(':')
		b.WriteString(c)
	}
	return V5(ServerJobNamespaceUUID, b.String())
}

func validJobComponent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ':' || s[i] > 0x7f {
			return false
		}
	}
	return true
}

// MaintenanceCursorKey returns the maintenance cursor key: the lowercase
// hex encoding of the canonical typed primary-key bytes, never unescaped
// free text (ids.md § Closed Server Producer Families).
func MaintenanceCursorKey(pkBytes []byte) string {
	return hex.EncodeToString(pkBytes)
}
