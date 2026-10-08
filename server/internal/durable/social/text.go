package social

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	"golang.org/x/text/unicode/norm"
)

// CanonicalText applies the shared social text rules (social.md
// § Message Content, ADR-0013, ADR-0060): reject malformed UTF-8, trim,
// NFC, reject empty and Unicode control or separator characters, count
// UAX #29 graphemes against max. Returns the canonical value; ok=false
// means the text is rejected (CHAT_TEXT_INVALID / notes invalid).
func CanonicalText(raw string, maxGraphemes int) (string, bool) {
	if !utf8.ValidString(raw) {
		return "", false
	}
	text := norm.NFC.String(strings.TrimSpace(raw))
	if text == "" {
		return "", false
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return "", false
		}
	}
	clusters := 0
	it := graphemes.FromString(text)
	for it.Next() {
		clusters++
		if clusters > maxGraphemes {
			return "", false
		}
	}
	return text, true
}
