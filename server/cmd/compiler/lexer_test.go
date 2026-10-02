package main

import (
	"os"
	"path/filepath"
	"testing"

	"thinhthan/internal/config"
)

func loadSource(t *testing.T, name, body string) (*File, *config.Diagnostics) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d := config.Diagnostics{}
	f, err := LoadFile(p, &d)
	if err != nil {
		t.Fatalf("LoadFile: %v diags=%v", err, d)
	}
	return f, &d
}

func TestLexer_HeadingScope(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
text
## B
inner
## C
inner2
# D
`)
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	a := f.Root.SectionAt("A")
	if a == nil {
		t.Fatal("no A")
	}
	if len(a.Children) != 2 || a.Children[0].Title != "B" || a.Children[1].Title != "C" {
		t.Fatalf("children: %+v", a.Children)
	}
	if got := f.Root.SectionAt("A > B"); got == nil || got.Title != "B" {
		t.Fatal("ancestry resolve failed")
	}
	if f.Root.SectionAt("A > D") != nil {
		t.Fatal("D leaked into A scope")
	}
}

func TestLexer_FenceSuspendsHeadingsAndTables(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
`+"```text\n# not a heading\n| a | b |\n|---|---|\n```\n"+`## B
`)
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	a := f.Root.SectionAt("A")
	if len(a.Children) != 1 || a.Children[0].Title != "B" {
		t.Fatalf("fence content became heading: %+v", a.Children)
	}
	var fences int
	for _, b := range a.Content {
		if b.Kind == BlockFence {
			fences++
			if b.Lang != "text" || len(b.FLines) != 3 {
				t.Fatalf("fence: %+v", b)
			}
		}
	}
	if fences != 1 {
		t.Fatalf("fences=%d", fences)
	}
}

func TestLexer_TableParse(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
| h1 | h2 |
|:---|---:|
| a\|b | `+"`c|d`"+` |
| x | y |
`)
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	var tbl *Block
	for _, b := range f.Root.SectionAt("A").Content {
		if b.Kind == BlockTable {
			tbl = b
		}
	}
	if tbl == nil {
		t.Fatal("no table")
	}
	if len(tbl.Headers) != 2 || tbl.Headers[0] != "h1" {
		t.Fatalf("headers: %v", tbl.Headers)
	}
	if len(tbl.Cells) != 2 {
		t.Fatalf("rows: %d", len(tbl.Cells))
	}
	if tbl.Cells[0][0].Text != "a|b" {
		t.Fatalf("escaped pipe: %q", tbl.Cells[0][0].Text)
	}
	if tbl.Cells[0][1].Scalar() != "c|d" {
		t.Fatalf("code span: %q", tbl.Cells[0][1].Scalar())
	}
}

func TestLexer_TableCellCountError(t *testing.T) {
	_, d := loadSource(t, "x.md", `# A
| h1 | h2 |
|---|---|
| one | two | three |
`)
	if !d.HasErrors() {
		t.Fatal("expected cell-count diagnostic")
	}
	if (*d)[0].Code != config.DiagTableSyntaxError {
		t.Fatalf("code: %s", (*d)[0].Code)
	}
}

func TestLexer_BacktickStrip(t *testing.T) {
	c := Cell{Text: "`item.book.potential`"}
	if c.Scalar() != "item.book.potential" {
		t.Fatalf("scalar: %q", c.Scalar())
	}
	c2 := Cell{Text: "`a`b`"}
	if c2.Scalar() != "`a`b`" {
		t.Fatalf("embedded backtick deleted: %q", c2.Scalar())
	}
}

func TestLexer_Assignments(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
`+"```text\nhp = 10*L + 50\nname = wolf\n```\n")
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	var fb *Block
	for _, b := range f.Root.SectionAt("A").Content {
		if b.Kind == BlockFence {
			fb = b
		}
	}
	as := fb.Assignments(f)
	if as["hp"] != "10*L + 50" || as["name"] != "wolf" {
		t.Fatalf("assignments: %v", as)
	}
}

func TestLexer_AssignmentDuplicateRejects(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
`+"```text\nhp = 1\nhp = 2\n```\n")
	var fb *Block
	for _, b := range f.Root.SectionAt("A").Content {
		if b.Kind == BlockFence {
			fb = b
		}
	}
	fb.Assignments(f)
	if !d.HasErrors() || (*d)[0].Code != config.DiagAmbiguousSource {
		t.Fatalf("diags: %v", d)
	}
}

func TestLexer_CRLFNormalized(t *testing.T) {
	f, d := loadSource(t, "x.md", "# A\r\nline\r\n## B\r\n")
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	if f.Lines[0] != "# A" {
		t.Fatalf("CRLF not normalized: %q", f.Lines[0])
	}
}

func TestLexer_BOMRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.md")
	if err := os.WriteFile(p, append([]byte{0xEF, 0xBB, 0xBF}, []byte("# A\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	d := config.Diagnostics{}
	if _, err := LoadFile(p, &d); err == nil {
		t.Fatal("BOM accepted")
	}
	if d[0].Code != config.DiagTableSyntaxError {
		t.Fatalf("code: %s", d[0].Code)
	}
}

func TestLexer_StableIDHeading(t *testing.T) {
	f, _ := loadSource(t, "x.md", "# A\n## `item.book.potential`\n## plain\n")
	a := f.Root.SectionAt("A")
	if a.Children[0].StableID() != "item.book.potential" {
		t.Fatalf("stable id: %q", a.Children[0].StableID())
	}
	if a.Children[1].StableID() != "" {
		t.Fatal("plain heading gave stable id")
	}
}

func TestLexer_UnterminatedFenceDiagnosed(t *testing.T) {
	_, d := loadSource(t, "x.md", "# A\n```text\nnever closed\n")
	if !d.HasErrors() || (*d)[0].Code != config.DiagTableSyntaxError {
		t.Fatalf("diags: %v", d)
	}
}

func TestLexer_MissingFile(t *testing.T) {
	d := config.Diagnostics{}
	if _, err := LoadFile("/nonexistent/x.md", &d); err == nil {
		t.Fatal("missing file accepted")
	}
	if d[0].Code != config.DiagCatalogFileNotFound {
		t.Fatalf("code: %s", d[0].Code)
	}
}

func TestLexer_DividerAlignVariants(t *testing.T) {
	f, d := loadSource(t, "x.md", `# A
| a | b | c |
|---:|:---:|---|
| 1 | 2 | 3 |
`)
	if d.HasErrors() {
		t.Fatalf("diags: %v", d)
	}
	var tbl *Block
	for _, b := range f.Root.SectionAt("A").Content {
		if b.Kind == BlockTable {
			tbl = b
		}
	}
	if len(tbl.Cells) != 1 || tbl.Cells[0][2].Text != "3" {
		t.Fatalf("rows: %+v", tbl.Cells)
	}
}
