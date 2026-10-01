package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fxRoot = "_testdata"

func findDetails(details []string, substrs ...string) []string {
	var out []string
	for _, d := range details {
		for _, s := range substrs {
			if strings.Contains(d, s) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

// badImportFiles is the in-code mutation fixture for import-graph rules:
// forbidden modules and fenced directions must each fail.
var badImportFiles = []goFile{
	{rel: "server/internal/edge/bad_sql.go", pkg: "edge", imports: []string{"database/sql"}},
	{rel: "server/internal/sim/bad.go", pkg: "sim", imports: []string{"github.com/jackc/pgx/v5", "thinhthan/internal/edge"}},
	{rel: "server/internal/durable/bad.go", pkg: "durable", imports: []string{"thinhthan/internal/sim"}},
	{rel: "server/internal/protocol/bad.go", pkg: "protocol", imports: []string{"thinhthan/internal/global"}},
	{rel: "server/internal/observability/bad.go", pkg: "observability", imports: []string{"thinhthan/internal/edge"}},
	{rel: "server/internal/badpkg/dep.go", pkg: "badpkg", imports: []string{"github.com/gin-gonic/gin", "math/rand"}},
	{rel: "server/cmd/server/main.go", pkg: "main", isMain: true},
	{rel: "server/cmd/foo/main.go", pkg: "main", isMain: true},
}

func TestImportDirection(t *testing.T) {
	got := checkImports(badImportFiles)
	for _, w := range []struct{ file, via string }{
		{"edge/bad_sql.go", "database/sql"}, // rule 2: edge not an SQL owner
		{"sim/bad.go", "pgx"},               // rule 2: sim must not touch SQL
		{"sim/bad.go", "edge"},              // rule 3: sim no edge import
		{"durable/bad.go", "sim"},           // rule 4: durable no sim
		{"protocol/bad.go", "global"},       // rule 5: protocol no domain
		{"observability/bad.go", "edge"},    // rule 10: no imports back
	} {
		if len(findDetails(got, w.file, w.via)) == 0 {
			t.Errorf("expected %s via %s violation, got %v", w.file, w.via, got)
		}
	}
}

func TestOneProductionMain(t *testing.T) {
	got := checkMains(badImportFiles)
	if len(findDetails(got, "cmd/foo/main.go")) == 0 {
		t.Fatalf("expected cmd/foo violation, got %v", got)
	}
	if len(findDetails(got, "cmd/server/main.go")) != 0 {
		t.Errorf("allowed cmd/server flagged: %v", got)
	}
}

func TestForbiddenDependencies(t *testing.T) {
	got := checkForbiddenImports(badImportFiles)
	if len(findDetails(got, "badpkg/dep.go", "gin")) == 0 {
		t.Errorf("expected gin violation, got %v", got)
	}
	if len(findDetails(got, "badpkg/dep.go", "math/rand")) == 0 {
		t.Errorf("expected math/rand violation, got %v", got)
	}
	got = checkSchema(fxRoot)
	for _, w := range []string{"durability", "global_leader_lease"} {
		if len(findDetails(got, w)) == 0 {
			t.Errorf("expected schema %s violation, got %v", w, got)
		}
	}
}

func TestGeneratedBoundary(t *testing.T) {
	// Rule 7: .pb.go source references.
	got := checkPbGo(fxRoot)
	if len(findDetails(got, "bad.pb.go")) == 0 {
		t.Errorf("expected bad.pb.go missing-source violation, got %v", got)
	}
	if len(findDetails(got, "good.pb.go")) != 0 {
		t.Errorf("good.pb.go flagged: %v", got)
	}
	// Rule 9: asmdef set conformance.
	got = checkAsmdefs(fxRoot)
	for _, w := range []string{
		"ThinhThan.Protocol references project assembly",
		"references ThinhThan.App",
		"unknown project assembly",
		"not in § Mandatory Assemblies table",
		"mandatory assembly ThinhThan.Missing",
		"invalid asmdef JSON",
	} {
		if len(findDetails(got, w)) == 0 {
			t.Errorf("expected asmdef violation %q, got %v", w, got)
		}
	}
	// Acyclic mandatory set + PlayMode-only App reference on a clean defs set.
	clean := []asmdef{
		{Name: "ThinhThan.Protocol", file: "p.asmdef"},
		{Name: "ThinhThan.Core", file: "c.asmdef"},
		{Name: "ThinhThan.App", References: []string{"ThinhThan.Core", "ThinhThan.Protocol"}, file: "a.asmdef"},
		{Name: "ThinhThan.Tests.PlayMode", References: []string{"ThinhThan.App"}, file: "pm.asmdef"},
	}
	root := t.TempDir()
	lb, err := os.ReadFile(filepath.Join(fxRoot, layoutFile))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, layoutFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, lb, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := checkAsmdefSet(clean, root); len(findDetails(got, "cycle", "ThinhThan.App")) != 0 {
		t.Errorf("clean asmdef set flagged: %v", got)
	}
	// Cycle detection via in-memory defs.
	cyc := []asmdef{
		{Name: "ThinhThan.Core", References: []string{"ThinhThan.A"}, file: "c.asmdef"},
		{Name: "ThinhThan.A", References: []string{"ThinhThan.Core"}, file: "a.asmdef"},
	}
	if got := checkAsmdefSet(cyc, root); len(findDetails(got, "cycle")) == 0 {
		t.Errorf("asmdef cycle not flagged: %v", got)
	}
	// Rule 11: test placement.
	got = checkTestPlacement(fxRoot)
	if len(findDetails(got, "misplaced/x_test.go")) == 0 {
		t.Errorf("expected misplaced Go test violation, got %v", got)
	}
	if len(findDetails(got, "HudTests.cs")) == 0 {
		t.Errorf("expected misplaced C# test violation, got %v", got)
	}
}
