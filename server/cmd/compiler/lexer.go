package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// BlockKind classifies one lexical block (§1 lexing rules).
type BlockKind int

const (
	BlockTable BlockKind = iota
	BlockFence
	BlockProse
)

// Cell is one table cell: raw text plus its source line.
type Cell struct {
	Text string
	Line int
}

// Scalar returns the cell trimmed with a single matching outer backtick
// pair stripped (§1: never delete embedded backticks).
func (c Cell) Scalar() string {
	s := strings.TrimSpace(c.Text)
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		inner := s[1 : len(s)-1]
		if !strings.Contains(inner, "`") {
			return inner
		}
	}
	return s
}

// Block is a table, fence or prose run inside a section.
type Block struct {
	Kind    BlockKind
	Line    int      // 1-based start line
	Headers []string // table header names
	Cells   [][]Cell // table rows (aligned to Headers)
	Lang    string   // fence language tag
	FLines  []string // fence body lines
	Prose   []string // prose lines
}

// Section is one heading scope in the section tree.
type Section struct {
	Title    string
	Level    int
	Line     int
	Children []*Section
	Content  []*Block
	Parent   *Section
	File     *File
}

// StableID returns the backticked stable ID when the heading is exactly one
// backticked token (`item.book.potential`), else "".
func (s *Section) StableID() string {
	t := strings.TrimSpace(s.Title)
	if len(t) >= 2 && t[0] == '`' && t[len(t)-1] == '`' {
		inner := t[1 : len(t)-1]
		if inner != "" && !strings.ContainsAny(inner, "` ") {
			return inner
		}
	}
	return ""
}

var (
	headingRe  = regexp.MustCompile(`^#{1,6} `)
	fenceRe    = regexp.MustCompile("^```([a-zA-Z0-9_-]*)\\s*$")
	tableDivRe = regexp.MustCompile(`^:?-{3,}:?$`)
)

// lex scans normalized lines into the section tree. Fences suspend
// heading/table recognition; a next same-or-higher heading ends scope.
func lex(f *File) *Section {
	root := &Section{Level: 0, File: f}
	cur := root
	var pendingBlock *Block // accumulating prose
	i := 0
	n := len(f.Lines)
	for i < n {
		line := f.Lines[i]
		ln := i + 1

		if m := fenceRe.FindStringSubmatch(strings.TrimRight(line, " \t")); m != nil {
			// fence open: consume to closing fence
			blk := &Block{Kind: BlockFence, Line: ln, Lang: m[1]}
			i++
			for i < n && !fenceRe.MatchString(strings.TrimRight(f.Lines[i], " \t")) {
				blk.FLines = append(blk.FLines, f.Lines[i])
				i++
			}
			if i >= n {
				f.diags.Addf(config.DiagTableSyntaxError, f.Path, ln, "unterminated %s fence", blk.Lang)
			} else {
				i++ // consume closing fence
			}
			cur.Content = append(cur.Content, blk)
			pendingBlock = nil
			continue
		}

		if h := headingRe.FindString(line); h != "" {
			level := len(h) - 1
			sec := &Section{Title: strings.TrimSpace(line[level+1:]), Level: level, Line: ln, File: f}
			// attach to nearest ancestor with lower level
			p := cur
			for p.Level >= level && p.Parent != nil {
				p = p.Parent
			}
			sec.Parent = p
			p.Children = append(p.Children, sec)
			cur = sec
			pendingBlock = nil
			i++
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "|") && isTableStart(f.Lines, i) {
			blk, next := parseTable(f, i)
			cur.Content = append(cur.Content, blk)
			pendingBlock = nil
			i = next
			continue
		}

		if strings.TrimSpace(line) == "" {
			pendingBlock = nil
			i++
			continue
		}
		if pendingBlock == nil {
			pendingBlock = &Block{Kind: BlockProse, Line: ln}
			cur.Content = append(cur.Content, pendingBlock)
		}
		pendingBlock.Prose = append(pendingBlock.Prose, line)
		i++
	}
	return root
}

// isTableStart reports whether line i begins a table: a pipe row followed by
// a divider row of `:?-{3,}:?` cells.
func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
		return false
	}
	div := strings.TrimSpace(lines[i+1])
	if !strings.HasPrefix(div, "|") {
		return false
	}
	cells, ok := splitRow(div)
	if !ok || len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !tableDivRe.MatchString(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

// parseTable consumes a table starting at line i: header, divider, rows.
func parseTable(f *File, i int) (*Block, int) {
	ln := i + 1
	blk := &Block{Kind: BlockTable, Line: ln}
	headerCells, _ := splitRow(strings.TrimSpace(f.Lines[i]))
	seen := map[string]int{}
	for _, h := range headerCells {
		name := strings.TrimSpace(h)
		if prev, dup := seen[name]; dup {
			f.diags.Addf(config.DiagTableSyntaxError, f.Path, ln,
				"duplicate header %q (also line %d)", name, prev)
		}
		seen[name] = ln
		blk.Headers = append(blk.Headers, name)
	}
	i += 2 // skip header + divider
	for i < len(f.Lines) {
		line := strings.TrimSpace(f.Lines[i])
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells, ok := splitRow(line)
		if !ok {
			f.diags.Addf(config.DiagTableSyntaxError, f.Path, i+1, "unbalanced table row")
			i++
			continue
		}
		row := make([]Cell, len(cells))
		for j, c := range cells {
			row[j] = Cell{Text: strings.TrimSpace(c), Line: i + 1}
		}
		if len(row) != len(blk.Headers) {
			f.diags.Addf(config.DiagTableSyntaxError, f.Path, i+1,
				"row has %d cells, want %d columns", len(row), len(blk.Headers))
		}
		blk.Cells = append(blk.Cells, row)
		i++
	}
	return blk, i
}

// splitRow splits one `|`-row on unescaped pipes outside inline-code spans.
// Returns cell raw strings (outer pipes dropped) and false on imbalance.
func splitRow(line string) ([]string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "|") {
		return nil, false
	}
	var cells []string
	var b strings.Builder
	inCode := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && s[i+1] == '|' {
			b.WriteByte('|')
			i++
			continue
		}
		if c == '`' {
			inCode = !inCode
			b.WriteByte(c)
			continue
		}
		if c == '|' && !inCode {
			cells = append(cells, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	// trailing cell before final pipe is dropped when the line ends with '|'
	if !strings.HasSuffix(s, "|") {
		cells = append(cells, b.String())
	}
	return cells, true
}

// Assignments parses a `text` fence's `field = value` lines (§1): one
// assignment per nonblank line, first-`=` split, duplicates reject.
func (b *Block) Assignments(f *File) map[string]string {
	out := map[string]string{}
	for j, l := range b.FLines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		idx := strings.IndexByte(l, '=')
		if idx < 0 {
			f.diags.Addf(config.DiagTableSyntaxError, f.Path, b.Line+1+j,
				"text fence line is not `field = value`: %q", strings.TrimSpace(l))
			continue
		}
		k := strings.TrimSpace(l[:idx])
		v := strings.TrimSpace(l[idx+1:])
		if _, dup := out[k]; dup {
			f.diags.Addf(config.DiagAmbiguousSource, f.Path, b.Line+1+j,
				"duplicate explicit field %q in one record", k)
			continue
		}
		out[k] = v
	}
	return out
}
