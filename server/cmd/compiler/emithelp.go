package main

import (
	"fmt"
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// emithelp.go — shared table/fence emit helpers used by the 24 drivers.

var snakeRE = regexp.MustCompile(`[^a-z0-9]+`)

// fieldName normalizes a table column header to a snake_case field name.
func fieldName(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.ReplaceAll(h, "%", " pct ")
	h = strings.ReplaceAll(h, "/", " ")
	h = snakeRE.ReplaceAllString(h, "_")
	return strings.Trim(h, "_")
}

// bindingSections resolves every declared section path of a binding;
// each path must resolve to exactly one section.
func bindingSections(c *Ctx, f *File, b *SourceBinding) []*Section {
	var out []*Section
	for _, p := range b.SectionPaths {
		secs := resolveSections(f, p)
		if len(secs) == 0 {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
				"registry section %q not found", p)
			continue
		}
		if len(secs) > 1 {
			c.Diags.Addf(config.DiagAmbiguousSource, f.Path, secs[1].Line,
				"registry section %q matches %d headings", p, len(secs))
		}
		out = append(out, secs[0])
	}
	return out
}

// consumed marks a binding satisfied and records the consumed coverage
// entry (declared-vs-consumed enforcement, CAT-004). Called once per
// binding regardless of how many sections/tables fed it.
func (c *Ctx) consumed(f *File, b *SourceBinding) {
	if b.consumed {
		return
	}
	b.consumed = true
	if c.Cov != nil {
		c.Cov.Add(f.Name, b.Raw, b.Output, "",
			config.CoverageConsumed)
	}
}

// inputTypes returns the binding's declared column types keyed by
// normalized field name.
func inputTypes(b *SourceBinding) map[string]TypeSpec {
	out := map[string]TypeSpec{}
	for name, ts := range b.InputSpecs {
		out[fieldName(name)] = ts
	}
	return out
}

// parseCell parses one table cell with a TypeSpec; `—`/empty/`none`/`NONE`
// produce a typed Null, never a parse error. `**bold**` emphasis is
// Markdown decoration, not value (§5 excludes decoration).
func parseCell(ts TypeSpec, raw string) (config.Value, error) {
	r := strings.TrimSpace(raw)
	if r == "" || r == "—" || r == "-" || strings.EqualFold(r, "none") {
		return config.VNull(), nil
	}
	for strings.HasPrefix(r, "**") && strings.HasSuffix(r, "**") && len(r) > 4 {
		r = strings.TrimSpace(r[2 : len(r)-2])
	}
	return ts.ParseValue(r)
}

// rowValues converts one table row into field-name → Value using the
// declared column types (undeclared columns parse as `string`).
func rowValues(c *Ctx, f *File, tbl *Block, ri int, types map[string]TypeSpec) map[string]config.Value {
	row := tbl.Cells[ri]
	out := map[string]config.Value{}
	for i, h := range tbl.Headers {
		fn := fieldName(h)
		raw := cellAt(row, i).Scalar()
		ts, ok := types[fn]
		if !ok {
			ts = TypeSpec{Name: "string"}
		}
		v, err := parseCell(ts, raw)
		if err != nil {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
				"column %q row %d: %v", h, ri+1, err)
			v = config.VStr(raw)
		}
		out[fn] = v
	}
	return out
}

// emitTable is the generic table binding driver: resolve the binding's
// sections, find the table matching the header signature under them, emit
// one record per data row keyed by the declared key fields.
func emitTable(c *Ctx, f *File, b *SourceBinding, sig string,
	family string, keyFields []string, extra map[string]config.Value) int {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, sig); t != nil {
			if tbl != nil {
				c.Diags.Addf(config.DiagAmbiguousSource, f.Path, t.Line,
					"multiple tables match %q for binding %q", sig, b.Raw)
				continue
			}
			tbl = t
		}
	}
	if tbl == nil {
		if len(b.SectionPaths) > 0 {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
				"no table with signature %q under %q", sig, b.Raw)
		}
		return 0
	}
	types := inputTypes(b)
	n := 0
	for ri := range tbl.Cells {
		vals := rowValues(c, f, tbl, ri, types)
		for k, v := range extra {
			vals[k] = v
		}
		key, ok := keyFrom(vals, keyFields)
		if !ok {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, tbl.Cells[ri][0].Line,
				"%s row %d lacks key fields %v", family, ri+1, keyFields)
			continue
		}
		c.Emit(f.Name, b.Raw, family, key, vals, tbl.Cells[ri][0].Line)
		n++
	}
	c.consumed(f, b)
	return n
}

// findTable returns the table under sec matching the header signature,
// or nil. Ambiguity between multiple matches reports the first.
func findTable(sec *Section, sig string) *Block {
	var hits []*Block
	for _, t := range allTables(sec) {
		if headerSig(t) == sig {
			hits = append(hits, t)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return hits[0]
}

// keyFrom extracts the composite key values from a parsed row.
func keyFrom(vals map[string]config.Value, fields []string) ([]config.Value, bool) {
	key := make([]config.Value, 0, len(fields))
	for _, kf := range fields {
		v, ok := vals[kf]
		if !ok || v.Kind == config.KindNull {
			return nil, false
		}
		key = append(key, v)
	}
	return key, true
}

// emitAssignRows parses `field = value` lines inside the binding's
// sections' `text` fences into `family` records keyed by the field name.
func emitAssignRows(c *Ctx, f *File, b *SourceBinding, family string) int {
	n := 0
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for name, raw := range fb.Assignments(f) {
				v, err := (TypeSpec{Name: "string"}).ParseValue(raw)
				if err != nil {
					v = config.VStr(raw)
				}
				c.Emit(f.Name, b.Raw, family,
					[]config.Value{config.VStr(name)},
					map[string]config.Value{"value": v}, fb.Line+1)
				n++
			}
		}
	}
	c.consumed(f, b)
	return n
}

// emitRule emits one rule record per declared section for prose bindings
// (zone contract, guardrails, invariants): section prose joins `rule`.
func emitRule(c *Ctx, f *File, b *SourceBinding, family string) int {
	n := 0
	for _, sec := range bindingSections(c, f, b) {
		var prose []string
		for _, bl := range allBlocks(sec) {
			if bl.Kind == BlockProse {
				prose = append(prose, bl.Prose...)
			}
		}
		c.Emit(f.Name, b.Raw, family, []config.Value{config.VStr(sec.Title)},
			map[string]config.Value{
				"rule":     config.VStr(strings.Join(prose, "\n")),
				"inputs":   config.VStr(b.InputsText),
				"defaults": config.VStr(b.DefaultsText),
			}, sec.Line)
		n++
	}
	c.consumed(f, b)
	return n
}

// allBlocks returns every block inside sec (descendants included).
func allBlocks(sec *Section) []*Block {
	var out []*Block
	var walk func(s *Section)
	walk = func(s *Section) {
		out = append(out, s.Content...)
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	walk(sec)
	return out
}

// enumSpec builds an enum TypeSpec from allowed tokens.
func enumSpec(tokens ...string) TypeSpec {
	return TypeSpec{Name: "enum", Enum: tokens}
}

// romanActs maps act enum tokens to ordinals 1..6.
var romanActs = map[string]int64{"I": 1, "II": 2, "III": 3, "IV": 4, "V": 5, "VI": 6}

func actOrdinal(v config.Value) (int64, error) {
	a, ok := romanActs[v.Str]
	if !ok {
		return 0, fmt.Errorf("act %q", v.Str)
	}
	return a, nil
}
