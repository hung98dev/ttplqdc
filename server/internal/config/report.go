package config

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

// CompileReport is the `content_compile_report.json` contract
// (test_and_release_evidence.md §4): content_revision_hash,
// catalogs_evaluated, entity_counts, validation_diagnostics.
// Field names match the JSON contract exactly.
type CompileReport struct {
	ContentRevisionHash   string
	CatalogsEvaluated     []string
	EntityCounts          map[string]int
	ValidationDiagnostics []Diagnostic
}

// MarshalJSON emits the report in canonical key order on the §5 writer.
func (r *CompileReport) Bytes() ([]byte, error) {
	cats := append([]string(nil), r.CatalogsEvaluated...)
	sort.Strings(cats)
	catVals := make([]Value, len(cats))
	for i, c := range cats {
		catVals[i] = VStr(c)
	}
	counts := map[string]Value{}
	for k, v := range r.EntityCounts {
		counts[k] = VInt(int64(v))
	}
	diagVals := make([]Value, len(r.ValidationDiagnostics))
	for i, d := range r.ValidationDiagnostics {
		diagVals[i] = VRec(map[string]Value{
			"code":    VStr(string(d.Code)),
			"file":    VStr(d.File),
			"line":    VInt(int64(d.Line)),
			"message": VStr(d.Message),
		})
	}
	return CanonicalBytes(VRec(map[string]Value{
		"catalogs_evaluated":     VList(catVals...),
		"content_revision_hash":  VStr(r.ContentRevisionHash),
		"entity_counts":          VRec(counts),
		"validation_diagnostics": VList(diagVals...),
	}))
}

// Write writes the report to path (0644). It is always written, including on
// error diagnostics, per the contract.
func (r *CompileReport) Write(path string) error {
	b, err := r.Bytes()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("config: write compile report %s: %w", path, err)
	}
	return nil
}

// String renders the report bytes for tests.
func (r *CompileReport) String() string {
	b, err := r.Bytes()
	if err != nil {
		return "<error>"
	}
	return string(bytes.TrimRight(b, "\n"))
}
