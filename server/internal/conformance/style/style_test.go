package style

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBraceLines(t *testing.T) {
	src := []byte("namespace X\n{\n    class Y { void M() { int a = 1; } }\n}\n")
	if errs := CheckBraceLines(src); len(errs) == 0 {
		t.Fatal("inline braces must be flagged (CODE-002)")
	}
	ok := []byte("namespace X\n{\n    class Y\n    {\n        void M()\n        {\n            int a = 1;\n        }\n    }\n}\n")
	if errs := CheckBraceLines(ok); len(errs) != 0 {
		t.Fatalf("Allman braces must pass: %v", errs)
	}
}

func TestIndentAndWhitespace(t *testing.T) {
	bad := "namespace X\n{\n\tclass Y\n\t{\n    }\n}\n"
	if errs := CheckIndentWhitespace([]byte(bad)); len(errs) == 0 {
		t.Fatal("tab indent must be flagged")
	}
	bad2 := "namespace X\n{\n    class Y   \n    {\n    }\n}\n"
	if errs := CheckIndentWhitespace([]byte(bad2)); len(errs) == 0 {
		t.Fatal("trailing whitespace must be flagged")
	}
	ok := "namespace X\n{\n    class Y\n    {\n    }\n}\n"
	if errs := CheckIndentWhitespace([]byte(ok)); len(errs) != 0 {
		t.Fatalf("4-space indent must pass: %v", errs)
	}
}

func TestLineEndingsBomFinalNewline(t *testing.T) {
	if errs := CheckLineEndingsBOM([]byte("namespace X\r\n{\r\n}\r\n")); len(errs) == 0 {
		t.Fatal("CRLF must be flagged")
	}
	if errs := CheckLineEndingsBOM([]byte("\xef\xbb\xbfnamespace X\n{\n}\n")); len(errs) == 0 {
		t.Fatal("BOM must be flagged")
	}
	if errs := CheckLineEndingsBOM([]byte("namespace X\n{\n}")); len(errs) == 0 {
		t.Fatal("missing final newline must be flagged")
	}
	if errs := CheckLineEndingsBOM([]byte("namespace X\n{\n}\n")); len(errs) != 0 {
		t.Fatalf("LF + final newline must pass: %v", errs)
	}
}

func TestPrivateFieldNaming(t *testing.T) {
	bad := "class Y\n{\n    private int count;\n}\n"
	if errs := CheckPrivateFieldNaming([]byte(bad)); len(errs) == 0 {
		t.Fatal("private field without _ prefix must be flagged")
	}
	ok := "class Y\n{\n    private int _count;\n    private static int shared;\n    private const int Max = 1;\n}\n"
	if errs := CheckPrivateFieldNaming([]byte(ok)); len(errs) != 0 {
		t.Fatalf("_-prefixed + const/static exempt must pass: %v", errs)
	}
}

func TestOneTypePerFileAndNamespace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "client", "Assets", "Scripts", "Core")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ThinhThan.Core.asmdef"),
		[]byte(`{"name":"ThinhThan.Core"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := []byte("namespace ThinhThan.Core\n{\n    public class Foo\n    {\n    }\n    public class Bar\n    {\n    }\n}\n")
	if errs := CheckOneTypePerFile("Foo.cs", src); len(errs) == 0 {
		t.Fatal("two top-level types must be flagged")
	}
	if errs := CheckNamespaceDecl(root,
		"client/Assets/Scripts/Core/Foo.cs",
		[]byte("namespace Other.Ns\n{\n    public class Foo\n    {\n    }\n}\n")); len(errs) == 0 {
		t.Fatal("namespace mismatch must be flagged")
	}
	if errs := CheckNamespaceDecl(root,
		"client/Assets/Scripts/Core/Foo.cs",
		[]byte("namespace ThinhThan.Core\n{\n    public class Foo\n    {\n    }\n}\n")); len(errs) != 0 {
		t.Fatalf("matching namespace must pass: %v", errs)
	}
}

func TestEditorconfigGitattributesKeys(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repo")
	}
	root := strings.TrimSpace(string(out))
	ec, err := os.ReadFile(filepath.Join(root, ".editorconfig"))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"root = true", "end_of_line = lf", "insert_final_newline = true"} {
		if !strings.Contains(string(ec), k) {
			t.Fatalf(".editorconfig missing %q", k)
		}
	}
	ga, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"text=auto", "lfs", "unityyamlmerge"} {
		if !strings.Contains(string(ga), k) {
			t.Fatalf(".gitattributes missing %q", k)
		}
	}
}

func TestGoFmtVetStaticcheckWired(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repo")
	}
	root := strings.TrimSpace(string(out))
	wf, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "verify.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wf), "staticcheck@v0.8.1") {
		t.Fatal("CI must install the pinned staticcheck")
	}
	runner, err := os.ReadFile(filepath.Join(root, "server", "internal", "conformance", "gates", "runner.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"gofmt"`, `"vet"`} {
		if !strings.Contains(string(runner), k) {
			t.Fatalf("verifier must run %s", k)
		}
	}
}

func TestLintFileIgnoreRejected(t *testing.T) {
	src := []byte("//lint:file-ignore reason\nnamespace X\n{\n    class Y\n    {\n    }\n}\n")
	if errs := CheckLintFileIgnore(src); len(errs) == 0 {
		t.Fatal("//lint:file-ignore must be rejected (CODE-003)")
	}
	root := t.TempDir()
	if errs := CheckCSharpFile(root, "client/Assets/Scripts/Core/Y.cs", src); len(errs) == 0 {
		t.Fatal("CheckCSharpFile must reject lint:file-ignore")
	}
}

func TestAllocPassAndBenchReportWired(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repo")
	}
	root := strings.TrimSpace(string(out))
	runner, err := os.ReadFile(filepath.Join(root, "server", "internal", "conformance", "gates", "runner.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"Q3.go.alloc", "Q3.go.bench", "-benchmem", "allocs/op"} {
		if !strings.Contains(string(runner), k) {
			t.Fatalf("verifier wiring missing %q", k)
		}
	}
}
