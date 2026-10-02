package main

import (
	"fmt"
	"strings"

	"thinhthan/internal/config"
)

// SourceBinding is one parsed row of a catalog's `Compiler Source Schema`
// registry table: `source_section | output / key | typed inputs |
// defaults / finite rule` (contract §1).
type SourceBinding struct {
	Raw          string              // raw source_section cell
	SectionPaths []string            // every backticked heading ancestry in the cell
	Target       string              // non-backticked remainder (table sig, `text` fence, bullet list, ...)
	Output       string              // output family + key text
	KeyCols      []string            // key column names parsed from `(a, b)` in Output
	InputSpecs   map[string]TypeSpec // name -> parsed column spec where machine-readable
	InputsText   string              // raw typed inputs cell
	DefaultsText string              // raw defaults/finite rule cell
	Line         int                 // registry row line
	consumed     bool                // set by Ctx.consumed once any driver consumes it
}

// Registry is one catalog's parsed Compiler Source Schema.
type Registry struct {
	Catalog  string
	Bindings []*SourceBinding
	ByOutput map[string]*SourceBinding // keyed by output-family token
}

// LoadRegistry parses the catalog's `Compiler Source Schema` section: finds
// its first registry table (headers contain source_section / output / typed
// inputs / defaults) and returns bindings. Missing section or table =
// SOURCE_SCHEMA_MISSING (CAT-004).
func LoadRegistry(f *File) (*Registry, error) {
	sec := f.Root.SectionAt("Compiler Source Schema")
	if sec == nil {
		f.diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1,
			"catalog declares no `Compiler Source Schema` section")
		return nil, fmt.Errorf("%s: no schema section", f.Name)
	}
	var reg *Block
	for _, b := range sec.Content {
		if b.Kind == BlockTable && hasHeaders(b, "source_section") {
			reg = b
			break
		}
	}
	if reg == nil {
		f.diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
			"Compiler Source Schema section has no registry table")
		return nil, fmt.Errorf("%s: no registry table", f.Name)
	}
	ci := colIndex(reg)
	r := &Registry{Catalog: f.Name, ByOutput: map[string]*SourceBinding{}}
	for _, row := range reg.Cells {
		b := &SourceBinding{
			Raw:        cellAt(row, ci["source_section"]).Text,
			Output:     cellAt(row, ci["output / key"]).Text,
			InputsText: cellAt(row, ci["typed inputs"]).Text,
			Line:       row[0].Line,
		}
		if dc, ok := ci["defaults / finite rule"]; ok {
			b.DefaultsText = cellAt(row, dc).Text
		}
		b.parseSourceSection(f)
		b.parseKeyCols()
		b.parseInputs()
		r.Bindings = append(r.Bindings, b)
		// first whitespace token of Output names the family loosely
		fam := strings.TrimSpace(strings.Split(b.Output, "/")[0])
		fam = strings.Trim(fam, "` ")
		if _, dup := r.ByOutput[fam]; dup {
			// repeated family output is legal (several tables per family) —
			// keep first for index purposes
			continue
		}
		r.ByOutput[fam] = b
	}
	return r, nil
}

// parseSourceSection extracts the leading backticked heading paths from
// the cell. A binding may join several sections with `+` (“ `A` + `B` “);
// the first non-separator text (typically `/ table ...`) ends the path
// list and becomes the target descriptor.
func (b *SourceBinding) parseSourceSection(f *File) {
	rest := b.Raw
	for {
		rest = strings.TrimLeft(rest, " \t+,")
		if !strings.HasPrefix(rest, "`") {
			break
		}
		end := strings.IndexByte(rest[1:], '`')
		if end < 0 {
			break
		}
		b.SectionPaths = append(b.SectionPaths, rest[1:1+end])
		rest = rest[end+2:]
	}
	t := strings.TrimSpace(rest)
	t = strings.TrimPrefix(t, "/")
	b.Target = strings.TrimSpace(t)
}

func (b *SourceBinding) parseKeyCols() {
	// output like `attacks / (monster_id, attack_id)` or `monster / monster_id`
	if i := strings.Index(b.Output, "("); i >= 0 {
		if j := strings.Index(b.Output[i:], ")"); j >= 0 {
			for _, k := range strings.Split(b.Output[i+1:i+j], ",") {
				b.KeyCols = append(b.KeyCols, strings.TrimSpace(k))
			}
			return
		}
	}
	// single key: last token after '/'
	parts := strings.Split(b.Output, "/")
	k := strings.TrimSpace(strings.Trim(parts[len(parts)-1], "` "))
	if k != "" {
		b.KeyCols = []string{k}
	}
}

// parseInputs parses `name:type(args); ...` tokens into TypeSpecs when the
// cell uses machine-readable grammar; unparseable tails stay in InputsText
// for the driver's custom grammar.
func (b *SourceBinding) parseInputs() {
	b.InputSpecs = map[string]TypeSpec{}
	for _, tok := range splitTop(b.InputsText, ';') {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		name, spec, ok := parseInputToken(tok)
		if ok {
			b.InputSpecs[name] = spec
		}
	}
}

// parseInputToken reads `name:type` where type may carry enum(args),
// adapters (`int ms`), or trailing prose annotations.
func parseInputToken(tok string) (string, TypeSpec, bool) {
	ci := strings.Index(tok, ":")
	if ci <= 0 {
		return "", TypeSpec{}, false
	}
	name := strings.TrimSpace(tok[:ci])
	rest := strings.TrimSpace(tok[ci+1:])
	// field names may be dotted ids or plain words; reject prose tokens
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.') {
			return "", TypeSpec{}, false
		}
	}
	spec := TypeSpec{}
	if strings.HasPrefix(rest, "enum(") && strings.Contains(rest, ")") {
		end := strings.Index(rest, ")")
		spec.Name = "enum"
		for _, e := range strings.Split(rest[5:end], ",") {
			spec.Enum = append(spec.Enum, strings.TrimSpace(e))
		}
		spec.Nullable = strings.Contains(rest[end:], "NONE") || strings.Contains(rest[end:], "nullable")
		return name, spec, true
	}
	head := strings.Fields(rest)
	if len(head) == 0 {
		return "", TypeSpec{}, false
	}
	spec.Name = strings.TrimRight(head[0], ",. ")
	switch spec.Name {
	case "id", "int", "decimal", "ratio", "bp", "bool", "string", "expr", "grouped_int", "percent":
		// adapter suffixes: "int ms", "int meters", "int degrees", "int count"
		if len(head) > 1 {
			switch head[1] {
			case "ms":
				spec.IntSuffix = "ms"
				spec.IntScale = 1
			case "meters":
				spec.IntSuffix = "m"
				spec.IntScale = 1000
			case "seconds":
				spec.IntSuffix = "s"
				spec.IntScale = 1000
			}
		}
		return name, spec, true
	case "set", "ordered", "pair", "range":
		// e.g. set(id), ordered(id)
		if len(head[0]) > len(spec.Name) || strings.HasPrefix(rest, spec.Name+"(") {
			inner := rest[len(spec.Name):]
			if strings.HasPrefix(inner, "(") && strings.Contains(inner, ")") {
				elem := strings.TrimSpace(inner[1:strings.Index(inner, ")")])
				es := TypeSpec{Name: elem}
				spec.Elem = &es
			}
		}
		return name, spec, true
	}
	return "", TypeSpec{}, false
}

// splitTop splits on sep but not inside parens or backticks.
func splitTop(s string, sep byte) []string {
	var out []string
	depth := 0
	inCode := false
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '`':
			inCode = !inCode
		case c == '(' && !inCode:
			depth++
		case c == ')' && !inCode:
			depth--
		case c == sep && depth == 0 && !inCode:
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	out = append(out, b.String())
	return out
}

func hasHeaders(b *Block, names ...string) bool {
	for _, n := range names {
		found := false
		for _, h := range b.Headers {
			if strings.TrimSpace(h) == n {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func colIndex(b *Block) map[string]int {
	m := map[string]int{}
	for i, h := range b.Headers {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

func cellAt(row []Cell, i int) Cell {
	if i < 0 || i >= len(row) {
		return Cell{}
	}
	return row[i]
}
