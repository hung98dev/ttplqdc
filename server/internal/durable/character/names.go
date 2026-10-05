package character

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Canonical character-name limits (text.md § Name Limits, ADR-0065):
// 1..16 UAX #29 grapheme clusters and at most 64 UTF-8 bytes after trim
// + NFC; name_key column is VARCHAR(256).
const (
	maxNameGraphemes = 16
	maxNameBytes     = 64
	maxNameKeyRunes  = 256
	// reservedNamePrefix begins server-generated erasure placeholders
	// (data_model.md § Account Erasure); player keys may not claim it.
	reservedNamePrefix = "anonymized_"
)

// ErrNameInvalid rejects any name failing the canonical pipeline; every
// violation maps to ERROR_CODE_CHARACTER_NAME_INVALID at the edge.
var ErrNameInvalid = errors.New("character: invalid name")

var caseFold = cases.Fold()

func invalidName(reason string) error {
	return fmt.Errorf("%w: %s", ErrNameInvalid, reason)
}

// NormalizeName canonicalizes a player-authored display name to its
// persisted (display, name_key) pair (text.md § Canonical Display Value
// + § Name Limits): reject malformed UTF-8, trim, NFC, reject empty,
// reject Unicode control and line/paragraph separator characters,
// count UAX #29 graphemes against the limits, then key on
// NFC(case-fold(display)) with the reserved anonymized_ prefix closed.
// Nothing is truncated; every violation returns ErrNameInvalid.
func NormalizeName(raw string) (display, nameKey string, err error) {
	if !utf8.ValidString(raw) {
		return "", "", invalidName("malformed UTF-8")
	}
	display = norm.NFC.String(strings.TrimSpace(raw))
	if display == "" {
		return "", "", invalidName("empty after trim")
	}
	for _, r := range display {
		if unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return "", "", invalidName("control or separator character")
		}
	}
	clusters := 0
	it := graphemes.FromString(display)
	for it.Next() {
		clusters++
		if clusters > maxNameGraphemes {
			break
		}
	}
	if clusters == 0 || clusters > maxNameGraphemes {
		return "", "", invalidName("grapheme count outside 1..16")
	}
	if len(display) > maxNameBytes {
		return "", "", invalidName("over 64 UTF-8 bytes")
	}
	nameKey = norm.NFC.String(caseFold.String(display))
	if strings.HasPrefix(nameKey, reservedNamePrefix) {
		return "", "", invalidName("reserved name_key prefix")
	}
	if utf8.RuneCountInString(nameKey) > maxNameKeyRunes {
		return "", "", invalidName("name_key over 256 characters")
	}
	return display, nameKey, nil
}
