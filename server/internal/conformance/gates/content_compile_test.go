package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validCompileReportJSON() string {
	return `{"catalogs_evaluated":["atlas","souls"],"content_revision_hash":"` +
		strings.Repeat("a", 64) +
		`","entity_counts":{"definitions.items":168},"validation_diagnostics":[]}`
}

func TestValidateCompileReport(t *testing.T) {
	cases := []struct {
		name    string
		report  *compileReport
		wantErr bool
	}{
		{
			name: "valid report passes",
			report: &compileReport{
				ContentRevisionHash: strings.Repeat("a", 64),
				CatalogsEvaluated:   []string{"atlas"},
				EntityCounts:        map[string]int{"definitions.items": 1},
			},
		},
		{
			name: "non-hex revision fails",
			report: &compileReport{
				ContentRevisionHash: "deadbeef",
				CatalogsEvaluated:   []string{"atlas"},
				EntityCounts:        map[string]int{"x": 1},
			},
			wantErr: true,
		},
		{
			name: "uppercase revision fails",
			report: &compileReport{
				ContentRevisionHash: strings.Repeat("A", 64),
				CatalogsEvaluated:   []string{"atlas"},
				EntityCounts:        map[string]int{"x": 1},
			},
			wantErr: true,
		},
		{
			name: "empty catalogs fails",
			report: &compileReport{
				ContentRevisionHash: strings.Repeat("a", 64),
				EntityCounts:        map[string]int{"x": 1},
			},
			wantErr: true,
		},
		{
			name: "empty entity counts fails",
			report: &compileReport{
				ContentRevisionHash: strings.Repeat("a", 64),
				CatalogsEvaluated:   []string{"atlas"},
			},
			wantErr: true,
		},
		{
			name: "diagnostics fail",
			report: &compileReport{
				ContentRevisionHash: strings.Repeat("a", 64),
				CatalogsEvaluated:   []string{"atlas"},
				EntityCounts:        map[string]int{"x": 1},
				ValidationDiagnostics: []compileReportDiag{
					{Code: "CAT-ERR", File: "souls.content", Line: 3, Message: "bad ref"},
				},
			},
			wantErr: true,
		},
	}
	for _, c := range cases {
		errs := validateCompileReport(c.report)
		if c.wantErr && len(errs) == 0 {
			t.Fatalf("%s: want diagnostics, got pass", c.name)
		}
		if !c.wantErr && len(errs) != 0 {
			t.Fatalf("%s: want pass, got %v", c.name, errs)
		}
	}
}

func TestReadCompileReport(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, CompileReportName)
	if err := os.WriteFile(good, []byte(validCompileReportJSON()), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := readCompileReport(good)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if rep.ContentRevisionHash != strings.Repeat("a", 64) {
		t.Fatalf("revision decoded wrong: %q", rep.ContentRevisionHash)
	}
	if rep.EntityCounts["definitions.items"] != 168 {
		t.Fatalf("entity_counts decoded wrong: %v", rep.EntityCounts)
	}
	if _, err := readCompileReport(filepath.Join(dir, "absent.json")); err == nil {
		t.Fatal("missing report must error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompileReport(bad); err == nil {
		t.Fatal("invalid JSON must error")
	}
}
