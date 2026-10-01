package id

import (
	"fmt"
	"strings"
)

// ValidateStaticContentID enforces the canonical static content ID grammar
// [a-z0-9]+(\.[a-z0-9_]+)+ (ids.md): ASCII lowercase segments separated by
// '.', words inside one segment joined by '_' (never in the first segment),
// ':' forbidden everywhere, wire bound of 128 bytes
// (protobuf_conventions.md §6).
func ValidateStaticContentID(s string) error {
	if len(s) == 0 || len(s) > 128 {
		return fmt.Errorf("id: static content ID %q: byte length must be in 1..128", s)
	}
	segments := strings.Split(s, ".")
	if len(segments) < 2 {
		return fmt.Errorf("id: static content ID %q: need at least two dot-separated segments", s)
	}
	for i, seg := range segments {
		if seg == "" {
			return fmt.Errorf("id: static content ID %q: empty segment", s)
		}
		for j := 0; j < len(seg); j++ {
			c := seg[j]
			if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '_' && i > 0 {
				continue
			}
			return fmt.Errorf("id: static content ID %q: invalid character %q", s, c)
		}
	}
	return nil
}
