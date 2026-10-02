package schema

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGoModuleClosureDeclaredInMatrix (BLK-001): every `// indirect` require
// row in server/go.mod — the require-closure Go >=1.21 records for the pinned
// pgx/migrate modules — is declared at its exact version in the version
// matrix (a pinned row or a transitive-closure entry), and the matrix carries
// the pinned-commit pseudo-version amendment: an exact
// `v0.0.0-<yyyymmddhhmmss>-<sha>` recorded verbatim in a transitive-closure
// entry is an approved pin, not a floating version.
func TestGoModuleClosureDeclaredInMatrix(t *testing.T) {
	root := testRepoRoot()
	matrixB, err := os.ReadFile(filepath.Join(
		root, "docs/00_context/technology_versions.md"))
	if err != nil {
		t.Fatalf("matrix: %v", err)
	}
	matrix := string(matrixB)

	// Every backticked `module version` pair in the matrix (top-level pin
	// rows and inline transitive-closure declarations alike) is declared.
	declRe := regexp.MustCompile("`([\\w./-]+) (v[\\w.-]+)`")
	declared := map[string]bool{}
	for _, m := range declRe.FindAllStringSubmatch(matrix, -1) {
		declared[m[1]+"@"+m[2]] = true
	}

	for _, want := range []string{
		"github.com/jackc/pgpassfile@v1.0.0",
		"github.com/jackc/pgservicefile@v0.0.0-20240606120523-5a60cdf6a761",
		"github.com/jackc/puddle/v2@v2.2.2",
		"github.com/jackc/pgerrcode@v0.0.0-20220416144525-469b46aa5efa",
		"golang.org/x/sync@v0.23.0",
		"golang.org/x/text@v0.42.0",
	} {
		if !declared[want] {
			t.Errorf("matrix does not declare pinned transitive %s", want)
		}
	}
	if !strings.Contains(matrix,
		"recorded verbatim in a transitive-closure entry of this matrix is an approved pin") {
		t.Error("matrix lacks the pinned-commit pseudo-version amendment")
	}

	gomodB, err := os.ReadFile(filepath.Join(root, "server/go.mod"))
	if err != nil {
		t.Fatalf("go.mod: %v", err)
	}
	for _, line := range strings.Split(string(gomodB), "\n") {
		l := strings.TrimSpace(line)
		if !strings.HasSuffix(l, "// indirect") {
			continue
		}
		f := strings.Fields(strings.TrimSuffix(l, "// indirect"))
		if len(f) != 2 {
			t.Fatalf("unparsable indirect require line %q", line)
		}
		if !declared[f[0]+"@"+f[1]] {
			t.Errorf("go.mod indirect require %s@%s not declared in the matrix", f[0], f[1])
		}
	}
}
