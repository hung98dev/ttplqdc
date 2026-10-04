package lockorder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// specAliases expands spec-text shorthand into canonical table names.
// database.md § Lock Order uses prose for the rollup line ("economy
// daily rollups"); every other segment is already a table token or a
// parenthetical the token regex extracts.
var specAliases = map[string][]string{
	"economy daily rollups": {"economy_account_daily_rollups", "economy_character_daily_rollups"},
}

var (
	priorityLineRe = regexp.MustCompile(`^(\d+)(?:\.(\d+))?\s+(.+)$`)
	snakeTokenRe   = regexp.MustCompile(`[a-z][a-z0-9]*(?:_[a-z0-9]+)+`)
	singleWordRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	parenRe        = regexp.MustCompile(`\(([^)]*)\)`)
	fenceStartRe   = regexp.MustCompile("```")
)

// ParseDatabaseMd re-derives the canonical priority table from
// database.md § Lock Order's fenced text block. It exists so
// TestLockOrderMatchesDatabaseMd compares CanonicalOrder against the
// spec itself (ADR-0060: helper priorities equal the spec table).
func ParseDatabaseMd(text string) ([]TableEntry, error) {
	block := lockOrderBlock(text)
	if block == "" {
		return nil, fmt.Errorf("lockorder: no fenced priority table under ## Lock Order")
	}
	var entries []TableEntry
	var cur *TableEntry
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := priorityLineRe.FindStringSubmatch(line); m != nil {
			flush()
			major, _ := strconv.Atoi(m[1])
			p := Priority(major * 10)
			if m[2] != "" {
				minor, _ := strconv.Atoi(m[2])
				p += Priority(minor)
			}
			cur = &TableEntry{Priority: p}
			cur.Tables = append(cur.Tables, extractTables(m[3])...)
			continue
		}
		if cur == nil {
			continue
		}
		cur.Tables = append(cur.Tables, extractTables(line)...)
	}
	flush()
	return entries, nil
}

// lockOrderBlock returns the fenced text block following the
// "## Lock Order" heading.
func lockOrderBlock(text string) string {
	idx := strings.Index(text, "## Lock Order")
	if idx < 0 {
		return ""
	}
	rest := text[idx:]
	start := fenceStartRe.FindStringIndex(rest)
	if start == nil {
		return ""
	}
	after := rest[start[1]:]
	end := fenceStartRe.FindStringIndex(after)
	if end == nil {
		return ""
	}
	return after[:end[0]]
}

// extractTables pulls canonical table tokens out of one spec segment.
// Segments split on ',', '+' and '/'; a lone lowercase word is a table
// name ("accounts", "friends"); snake_case tokens inside parentheses
// ("(item_locations GUILD_STORAGE)") count while uppercase qualifiers
// are dropped; pure-prose segments resolve through specAliases.
func extractTables(segment string) []string {
	var tables []string
	for _, g := range parenRe.FindAllStringSubmatch(segment, -1) {
		tables = append(tables, snakeTokenRe.FindAllString(g[1], -1)...)
	}
	rest := parenRe.ReplaceAllString(segment, " ")
	for _, part := range strings.FieldsFunc(rest, func(r rune) bool {
		return r == ',' || r == '+' || r == '/'
	}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		words := strings.Fields(part)
		if len(words) == 1 && singleWordRe.MatchString(words[0]) {
			tables = append(tables, words[0])
			continue
		}
		tables = append(tables, snakeTokenRe.FindAllString(part, -1)...)
	}
	if len(tables) == 0 {
		norm := strings.Join(strings.Fields(rest), " ")
		if alias, ok := specAliases[norm]; ok {
			tables = alias
		}
	}
	return tables
}
