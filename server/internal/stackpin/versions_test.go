package stackpin

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "00_context", "technology_versions.md")); err != nil {
		t.Fatalf("technology_versions.md not found under %s: %v", root, err)
	}
	return root
}

func readMatrix(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "00_context", "technology_versions.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Every pin carried by the typed registry must appear verbatim in the
// canonical version matrix; a pin absent from the matrix is unlisted.
func TestPinsMatchVersionMatrix(t *testing.T) {
	matrix := readMatrix(t)
	wants := []string{
		GoVersion, ProtocVersion, ProtocGenGoVersion,
		PostgresVersion,
		"postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722",
		StaticcheckVersion, StaticcheckDistLabel,
		UnityEditorVersion, UnityEditorChangeset,
		UnityWindowsInstallers["editor"].SHA256, UnityWindowsInstallers["il2cpp"].SHA256,
		EDBPostgresZip.SHA256, "343,808,005",
		GoogleProtobufNupkg.SHA256, GoogleProtobufNupkgVersion,
		CacertSHA256, GcloudVersion, GcloudPin.SHA256, GitForWindowsVersion,
	}
	for _, pin := range AndroidModules {
		wants = append(wants, pin.SHA256, filepath.Base(pin.URL))
	}
	for _, tool := range CliTools {
		wants = append(wants, tool.Version, tool.LinuxSHA256, tool.WindowsSHA256, tool.LinuxAsset, tool.WindowsAsset)
	}
	for _, sha := range Actions {
		wants = append(wants, sha)
	}
	for _, modVer := range GoModulePins {
		wants = append(wants, modVer)
	}
	for _, w := range wants {
		if w == "" {
			t.Fatal("empty pin value in registry")
		}
		if !strings.Contains(matrix, w) {
			t.Errorf("pin %q not found in technology_versions.md", w)
		}
	}
	for mod := range GoModulePins {
		if !strings.Contains(matrix, mod) {
			t.Errorf("module %q not found in technology_versions.md", mod)
		}
	}
}

func TestStaticcheckVersionAndInstall(t *testing.T) {
	matrix := readMatrix(t)
	for _, want := range []string{
		"honnef.co/go/tools/cmd/staticcheck@" + StaticcheckVersion,
		StaticcheckDistLabel,
	} {
		if !strings.Contains(matrix, want) {
			t.Errorf("matrix missing %q", want)
		}
	}
	// staticcheck is a CI-installed tool, never a module dependency.
	gomod, err := os.ReadFile(filepath.Join(repoRoot(t), "server", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gomod), "honnef.co") {
		t.Error("go.mod must not contain staticcheck")
	}
}

func TestRunnerLabelsPinned(t *testing.T) {
	matrix := readMatrix(t)
	if len(RunnerLabels) != 2 {
		t.Fatalf("RunnerLabels = %v, want exactly ubuntu-24.04 + windows-2022", RunnerLabels)
	}
	for _, label := range []string{"ubuntu-24.04", "windows-2022"} {
		if RunnerLabels[label] == "" {
			t.Errorf("missing runner label %q", label)
		}
		if !strings.Contains(matrix, label) {
			t.Errorf("matrix missing runner label %q", label)
		}
	}
	for label := range RunnerLabels {
		if strings.Contains(label, "latest") || strings.Contains(label, "self-hosted") {
			t.Errorf("forbidden runner label %q", label)
		}
	}
}

// TestCscRspScopedPerAsmdef: a csc.rsp sits beside every asmdef with the exact
// pinned contents and no root client/Assets/csc.rsp exists (CODE-001).
func TestCscRspScopedPerAsmdef(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "client", "Assets", "csc.rsp")); !os.IsNotExist(err) {
		t.Fatal("root client/Assets/csc.rsp must not exist")
	}
	want := "-warnaserror+\n-nullable:enable\n"
	found := 0
	err := filepath.WalkDir(filepath.Join(root, "client", "Assets"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".asmdef") {
			return nil
		}
		rsp := filepath.Join(filepath.Dir(path), "csc.rsp")
		b, err := os.ReadFile(rsp)
		if err != nil {
			t.Errorf("csc.rsp missing beside %s", path)
			return nil
		}
		if string(b) != want {
			t.Errorf("%s: got %q, want %q", rsp, string(b), want)
		}
		found++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found != 13 {
		t.Fatalf("found %d asmdef csc.rsp pairs, want 13", found)
	}
}

func TestGoogleProtobufNupkgSha256(t *testing.T) {
	matrix := readMatrix(t)
	if !strings.Contains(GoogleProtobufNupkg.URL, "/"+GoogleProtobufNupkgVersion+"/") {
		t.Errorf("nupkg URL %q does not carry version %s", GoogleProtobufNupkg.URL, GoogleProtobufNupkgVersion)
	}
	if len(GoogleProtobufNupkg.SHA256) != 64 {
		t.Fatalf("nupkg SHA256 malformed: %q", GoogleProtobufNupkg.SHA256)
	}
	if !strings.Contains(matrix, GoogleProtobufNupkg.SHA256) {
		t.Error("nupkg SHA256 not in matrix")
	}
	// The committed dll under Assets/Plugins must come from this nupkg.
	dll := filepath.Join(repoRoot(t), "client", "Assets", "Plugins", "Google.Protobuf", "Google.Protobuf.dll")
	if _, err := os.Stat(dll); err != nil {
		t.Fatalf("Google.Protobuf.dll missing: %v", err)
	}
}

// TestProtocReleaseAssetSha256: both protoc 36.2 release-asset zips carry
// URL + SHA-256 pins and verify.yml SHA-verifies each download before the
// codegen gates may consume them.
func TestProtocReleaseAssetSha256(t *testing.T) {
	for _, plat := range []string{"linux", "windows"} {
		pin, ok := ProtocZips[plat]
		if !ok {
			t.Fatalf("ProtocZips missing %q", plat)
		}
		if len(pin.SHA256) != 64 {
			t.Fatalf("protoc %s SHA256 malformed: %q", plat, pin.SHA256)
		}
		if !strings.Contains(pin.URL, "/v"+ProtocVersion+"/") {
			t.Errorf("protoc %s URL %q does not carry version %s", plat, pin.URL, ProtocVersion)
		}
	}
	wf, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "verify.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(wf)
	for _, pin := range ProtocZips {
		if !strings.Contains(s, pin.SHA256) {
			t.Errorf("verify.yml does not SHA-verify %s", filepath.Base(pin.URL))
		}
		if !strings.Contains(s, filepath.Base(pin.URL)) {
			t.Errorf("verify.yml does not download %s", filepath.Base(pin.URL))
		}
	}
}

func TestEdbZipSha256(t *testing.T) {
	matrix := readMatrix(t)
	if len(EDBPostgresZip.SHA256) != 64 || !strings.Contains(matrix, EDBPostgresZip.SHA256) {
		t.Error("EDB zip SHA256 missing/mismatched vs matrix")
	}
	if EDBPostgresZip.Size != 343808005 {
		t.Errorf("EDB zip size = %d, want 343808005", EDBPostgresZip.Size)
	}
	if !strings.Contains(matrix, EDBPostgresZip.URL) {
		t.Error("EDB zip URL not in matrix")
	}
}

// TestDownloadArtifactAndGitLfsPins: download-artifact is pinned by SHA and
// git-lfs 3.8.0 release-asset SHAs match the matrix (ADR-0072 item 8).
func TestDownloadArtifactAndGitLfsPins(t *testing.T) {
	matrix := readMatrix(t)
	sha, ok := Actions["actions/download-artifact"]
	if !ok {
		t.Fatal("actions/download-artifact missing from Actions")
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{40}$`, sha); !matched {
		t.Fatalf("download-artifact pin %q is not a full commit SHA", sha)
	}
	if !strings.Contains(matrix, sha) {
		t.Error("download-artifact SHA not in matrix")
	}
	var lfs *CliTool
	for i := range CliTools {
		if CliTools[i].Name == "git-lfs" {
			lfs = &CliTools[i]
		}
	}
	if lfs == nil {
		t.Fatal("git-lfs missing from CliTools")
	}
	if lfs.Version != "3.8.0" {
		t.Errorf("git-lfs version %q, want 3.8.0", lfs.Version)
	}
}

// TestNoFloatingOrUnlistedDeps: go.mod direct requires must be GoModulePins at
// exact versions; `// indirect` requires must be declared in GoModulePins or
// the TransitiveModuleAllowlist. No floating tag, range, or prerelease — a
// commit pseudo-version is permitted only when recorded verbatim in the
// matrix's transitive-closure entries (floating-check amendment).
func TestNoFloatingOrUnlistedDeps(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "server", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	matrix := readMatrix(t)
	requires := parseGoModRequires(string(b))
	if len(requires) == 0 {
		t.Fatal("go.mod declares no requires")
	}
	indirect := parseGoModIndirect(string(b))
	for mod, ver := range requires {
		floating := ver == "latest" || strings.Contains(ver, "*") || strings.ContainsAny(ver, "<>^~")
		if strings.Contains(ver, "-") && !isApprovedCommitPseudoVersion(matrix, mod, ver) {
			floating = true
		}
		if floating {
			t.Errorf("floating/prerelease dep %s@%s", mod, ver)
		}
		if pin, ok := GoModulePins[mod]; ok {
			if pin != ver {
				t.Errorf("dependency %s@%s, want pinned %s", mod, ver, pin)
			}
			continue
		}
		if pins, ok := TransitiveModuleAllowlist[mod]; ok && indirect[mod] {
			if !slices.Contains(pins, ver) {
				t.Errorf("transitive %s@%s, want one of %s", mod, ver, strings.Join(pins, "/"))
			}
			continue
		}
		if indirect[mod] {
			t.Errorf("unlisted transitive dependency %s@%s", mod, ver)
		} else {
			t.Errorf("unlisted direct dependency %s@%s", mod, ver)
		}
	}
}

// commitPseudoRe matches an exact commit pseudo-version
// (`v0.0.0-<yyyymmddhhmmss>-<sha>`).
var commitPseudoRe = regexp.MustCompile(`^v0\.0\.0-[0-9]{14}-[0-9a-f]+$`)

// isApprovedCommitPseudoVersion reports whether ver is a commit pseudo-version
// recorded verbatim in the matrix for mod — the only approved form of
// pseudo-version pin (floating-check amendment).
func isApprovedCommitPseudoVersion(matrix, mod, ver string) bool {
	return commitPseudoRe.MatchString(ver) && strings.Contains(matrix, mod+" "+ver)
}

// TestGoModuleClosureDeclaredInMatrix: the approved transitive require-closure
// is declared verbatim in the matrix and pinned in TransitiveModuleAllowlist
// (BLK-001, spec-change #30); every commit pseudo-version in go.mod is a
// matrix-declared pin.
func TestGoModuleClosureDeclaredInMatrix(t *testing.T) {
	matrix := readMatrix(t)
	for mod, vers := range TransitiveModuleAllowlist {
		for _, ver := range vers {
			if !strings.Contains(matrix, mod+" "+ver) {
				t.Errorf("TransitiveModuleAllowlist %s %s not declared verbatim in matrix", mod, ver)
			}
		}
	}
	// BLK-001 declared closure of pgx/v5 v5.11.0 and migrate/v4 v4.20.1.
	closure := map[string]string{
		"github.com/jackc/pgerrcode":     "v0.0.0-20220416144525-469b46aa5efa",
		"github.com/jackc/pgpassfile":    "v1.0.0",
		"github.com/jackc/pgservicefile": "v0.0.0-20240606120523-5a60cdf6a761",
		"github.com/jackc/puddle/v2":     "v2.2.2",
		"golang.org/x/sync":              "v0.23.0",
	}
	for mod, ver := range closure {
		if !strings.Contains(matrix, mod+" "+ver) {
			t.Errorf("matrix missing transitive-closure entry %s %s", mod, ver)
		}
		if pins := TransitiveModuleAllowlist[mod]; !slices.Contains(pins, ver) {
			t.Errorf("TransitiveModuleAllowlist %s = %q, want %q in set", mod, pins, ver)
		}
	}
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "server", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for mod, ver := range parseGoModRequires(string(b)) {
		if commitPseudoRe.MatchString(ver) && !isApprovedCommitPseudoVersion(matrix, mod, ver) {
			t.Errorf("unapproved pseudo-version %s %s in go.mod", mod, ver)
		}
	}
}

// TestGoModuleAndToolchain: module name and `go` directive match the layout
// and matrix pins.
func TestGoModuleAndToolchain(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "server", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var mod, gover string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod = strings.TrimPrefix(line, "module ")
		}
		if strings.HasPrefix(line, "go ") {
			gover = strings.TrimPrefix(line, "go ")
		}
	}
	if mod != GoModuleName {
		t.Errorf("module %q, want %q", mod, GoModuleName)
	}
	if gover != GoVersion {
		t.Errorf("go directive %q, want %q", gover, GoVersion)
	}
	layout, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "10_implementation", "repository_layout.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(layout), "module "+GoModuleName) {
		t.Error("module name not declared in repository_layout.md")
	}
}

// parseGoModIndirect returns the set of modules marked `// indirect`.
func parseGoModIndirect(gomod string) map[string]bool {
	out := map[string]bool{}
	var inBlock bool
	for _, line := range strings.Split(gomod, "\n") {
		l := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(l, "require ("):
			inBlock = true
			continue
		case inBlock && l == ")":
			inBlock = false
			continue
		}
		if !strings.Contains(l, "// indirect") {
			continue
		}
		var spec string
		if strings.HasPrefix(l, "require ") && !inBlock {
			spec = strings.TrimPrefix(l, "require ")
		} else if inBlock {
			spec = l
		} else {
			continue
		}
		if i := strings.Index(spec, "//"); i >= 0 {
			spec = strings.TrimSpace(spec[:i])
		}
		if f := strings.Fields(spec); len(f) >= 1 {
			out[f[0]] = true
		}
	}
	return out
}

// parseGoModRequires returns module -> version for every require entry, both
// the single-line form and require-block form.
func parseGoModRequires(gomod string) map[string]string {
	out := map[string]string{}
	var inBlock bool
	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) >= 2 {
				out[fields[0]] = fields[1]
			}
		case inBlock:
			fields := strings.Fields(line)
			if len(fields) >= 2 && !strings.HasPrefix(fields[0], "//") {
				out[fields[0]] = fields[1]
			}
		}
	}
	return out
}
