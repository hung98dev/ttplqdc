package character

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestNameGraphemeAndByteLimits covers the ADR-0065 limits on canonical
// names: 1..16 UAX #29 grapheme clusters and <=64 UTF-8 bytes after trim
// + NFC (text.md § Name Limits).
func TestNameGraphemeAndByteLimits(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantOK  bool
		display string // expected canonical display when wantOK
	}{
		{name: "ascii", raw: "Alice", wantOK: true, display: "Alice"},
		{name: "vietnamese precomposed", raw: "Nguyễn Thị Lăng", wantOK: true, display: "Nguyễn Thị Lăng"},
		{name: "vietnamese decomposed", raw: "Nguye\u0302\u0303n", wantOK: true, display: "Nguyễn"},
		{name: "leading trailing ws trimmed", raw: "  エッジ　", wantOK: true, display: "エッジ"},
		{name: "combining mark grapheme", raw: "e\u0301xpert", wantOK: true, display: "éxpert"},
		{name: "emoji zwj one cluster", raw: "A👨‍👩‍👧B", wantOK: true, display: "A👨‍👩‍👧B"},
		{name: "exactly sixteen", raw: strings.Repeat("k", 16), wantOK: true, display: strings.Repeat("k", 16)},
		{name: "exactly sixty four bytes", raw: strings.Repeat("🐉", 16), wantOK: true, display: strings.Repeat("🐉", 16)},
		{name: "empty", raw: "", wantOK: false},
		{name: "whitespace only", raw: "   \t\n  ", wantOK: false},
		{name: "seventeen graphemes", raw: strings.Repeat("k", 17), wantOK: false},
		{name: "over sixty four bytes", raw: strings.Repeat("đ", 33), wantOK: false},
		{name: "bytes over with few graphemes", raw: strings.Repeat("👨‍👩‍👧‍👦", 3), wantOK: false},
		{name: "control character", raw: "ab\x07c", wantOK: false},
		{name: "line separator", raw: "ab c", wantOK: false},
		{name: "paragraph separator", raw: "ab c", wantOK: false},
		{name: "tab interior", raw: "a\tb", wantOK: false},
		{name: "invalid utf8", raw: string([]byte{0xff, 0xfe}), wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			display, key, err := NormalizeName(tc.raw)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("NormalizeName(%q): %v", tc.raw, err)
				}
				if display != tc.display {
					t.Fatalf("display %q, want %q", display, tc.display)
				}
				if key == "" {
					t.Fatal("empty name_key")
				}
				if !utf8.ValidString(key) {
					t.Fatal("name_key not valid UTF-8")
				}
				return
			}
			if !errors.Is(err, ErrNameInvalid) {
				t.Fatalf("NormalizeName(%q) err %v, want ErrNameInvalid", tc.raw, err)
			}
		})
	}
}

// TestAnonymizedPrefixReserved rejects player names whose canonical
// name_key starts with the erasure-placeholder prefix (text.md § Name
// Limits; data_model.md § Account Erasure).
func TestAnonymizedPrefixReserved(t *testing.T) {
	for _, raw := range []string{
		"anonymized_ghost",
		"Anonymized_9f2",
		"ANONYMIZED_x",
		" anonymized_7 ",
	} {
		if _, _, err := NormalizeName(raw); !errors.Is(err, ErrNameInvalid) {
			t.Fatalf("NormalizeName(%q) err %v, want ErrNameInvalid", raw, err)
		}
	}
	// The bare word without the underscore is not the reserved prefix.
	if _, key, err := NormalizeName("anonymized"); err != nil {
		t.Fatalf("NormalizeName bare word: %v", err)
	} else if !strings.HasPrefix(key, "anonymized") {
		t.Fatalf("unexpected key %q", key)
	}
}

// TestLongCaseFoldKeyStored verifies a name whose case-folded key
// expands far beyond its display length is stored intact (name_key
// VARCHAR(256): folding may expand code points — text.md).
func TestLongCaseFoldKeyStored(t *testing.T) {
	acct := mkAccount(t)
	store := NewStore(pool(t))
	// 'ﬃ' (U+FB03) folds to 'ffi': 16 graphemes -> 48-char key.
	display, key, err := NormalizeName(strings.Repeat("ﬃ", 16))
	if err != nil {
		t.Fatalf("NormalizeName: %v", err)
	}
	if utf8.RuneCountInString(key) != 48 {
		t.Fatalf("key length %d, want 48", utf8.RuneCountInString(key))
	}
	if key != strings.Repeat("ffi", 16) {
		t.Fatalf("key %q, want ffi x16", key)
	}
	charID := id.NewV4()
	err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return store.Insert(ctx, tx, charID, acct, display, key, "class.kim")
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	row, err := store.Get(context.Background(), nil, charID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row.NameKey != key || row.Name != display {
		t.Fatalf("stored (%q,%q), want (%q,%q)", row.Name, row.NameKey, display, key)
	}
}
