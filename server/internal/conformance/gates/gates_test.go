package gates

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// twoPhaseTaskIDs is the canonical two-phase gate-task list
// (audit_gates.md § Two-Phase Gate Tasks).
var twoPhaseTaskIDs = []string{
	"IMP-000", "IMP-003", "IMP-004", "IMP-005", "IMP-061", "IMP-065", "IMP-068", "IMP-083",
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeRepo creates a committed git repo from a path->content map.
func makeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "test")
	git("config", "commit.gpgsign", "false")
	for rel, content := range files {
		writeFile(t, dir, rel, content)
	}
	git("add", "-A")
	git("commit", "-qm", "init")
	return dir
}

// miniQueue builds a PacketIndex with IMP-000 owning the given paths.
func miniQueue(owned ...string) PacketIndex {
	return PacketIndex{
		"IMP-000": &Packet{ID: "IMP-000", Status: "IN_PROGRESS", OwnedPaths: owned},
	}
}

func doneQueue(owned ...string) PacketIndex {
	return PacketIndex{
		"IMP-000": &Packet{ID: "IMP-000", Status: "DONE", OwnedPaths: owned},
	}
}

func TestGateRequiredWhenOwnerDoneOnMainOrHead(t *testing.T) {
	spec := GateSpec{ID: "Q3.unity.playmode", Owner: "IMP-065", OS: "windows", Phase: PhaseUnity}
	ctx := RunContext{OSTarget: "windows", UnityScope: "full"}
	if GateRequired(spec, ctx) {
		t.Fatal("gate must not be required before owner DONE")
	}
	ctx.MainIdx = PacketIndex{"IMP-065": &Packet{Status: "DONE"}}
	if !GateRequired(spec, ctx) {
		t.Fatal("owner DONE on main must activate the gate")
	}
	ctx.MainIdx = PacketIndex{}
	ctx.HeadIdx = PacketIndex{"IMP-065": &Packet{Status: "DONE"}}
	if !GateRequired(spec, ctx) {
		t.Fatal("owner DONE on head must activate the gate (ADR-0068)")
	}
}

func TestStatusOnlyPrFastPath(t *testing.T) {
	ctx := RunContext{
		OSTarget: "linux", StatusOnly: true,
		HeadIdx: doneQueue(), MainIdx: PacketIndex{},
	}
	spec := GateSpec{ID: "Q3.go.test", Owner: "IMP-000"}
	if got := gateVerdict(spec, ctx); got != SkipStatusOnly {
		t.Fatalf("non-Q0 gate on status-only PR: want SKIP(status-only), got %q", got)
	}
	q0 := GateSpec{ID: "Q0.absent_paths", Owner: "IMP-000"}
	if got := gateVerdict(q0, ctx); got != "" {
		t.Fatalf("Q0 gate must still evaluate on status-only PR, got %q", got)
	}
}

func TestDonePrRunsAllGates(t *testing.T) {
	root := fixtureRepo(t)
	ctx := RunContext{
		OSTarget: "linux", EventName: "pull_request", HeadBranch: "imp/IMP-000-done",
		UnityScope: "full",
		MainIdx:    miniQueue(),
		HeadIdx:    doneQueue("server/", "client/", "scripts/", ".github/", "docs/"),
	}
	r := &Runner{Root: root, Ctx: ctx}
	rep, err := r.Run(PhasePreUnity)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range rep.Gates {
		if g.Reason == SkipOwnerNotDone && g.Owner == "IMP-000" {
			t.Fatalf("IMP-000 gate %s skipped on its own -done PR", g.ID)
		}
	}
}

func TestPrRoleFromBranchPrefix(t *testing.T) {
	cases := map[string]Role{
		"spec/adr-0078":   RoleSpecOwner,
		"claim/IMP-003":   RoleCoordinator,
		"ops/fix-runner":  RoleCoordinator,
		"imp/IMP-000-x":   RoleImplementer,
		"block/IMP-001-2": RoleImplementer,
		"revert/abc":      RoleMergeGuard,
	}
	for b, want := range cases {
		got, err := RoleForBranch(b)
		if err != nil || got != want {
			t.Fatalf("RoleForBranch(%q) = %q,%v want %q", b, got, err, want)
		}
	}
	if _, err := RoleForBranch("feature/x"); err == nil {
		t.Fatal("unknown branch prefix must fail closed")
	}
}

func TestBlockAndOpsPrFastPath(t *testing.T) {
	changes := []FileChange{{
		Path:    "docs/10_implementation/task_queue.md",
		Added:   []string{"status: BLOCKED", "claimed_by: devin-x"},
		Removed: []string{"status: IN_PROGRESS"},
	}}
	for _, branch := range []string{"ops/fix-runner", "block/IMP-001-2"} {
		d := ClassifyControlDiff(branch, changes)
		if !d.StatusOnly {
			t.Fatalf("branch %s status diff must be status-only: %s", branch, d.Reason)
		}
	}
	// Non-ops branches may not remove lines from known_blockers.md.
	d := ClassifyControlDiff("block/IMP-001-2", []FileChange{{
		Path:    "docs/10_implementation/known_blockers.md",
		Removed: []string{"- BLK-001: old blocker"},
	}})
	if d.StatusOnly {
		t.Fatal("blocker removals by non-ops branch must not be status-only")
	}
	// claim/ may not touch blocked_by or known_blockers.md.
	if d := ClassifyControlDiff("claim/IMP-001", []FileChange{{
		Path:  "docs/10_implementation/task_queue.md",
		Added: []string{"blocked_by: BLK-007"},
	}}); d.StatusOnly {
		t.Fatal("claim/ writing blocked_by must not be status-only")
	}
	if d := ClassifyControlDiff("claim/IMP-001", []FileChange{{
		Path:  "docs/10_implementation/known_blockers.md",
		Added: []string{"- BLK-007: new"},
	}}); d.StatusOnly {
		t.Fatal("claim/ writing known_blockers must not be status-only")
	}
}

func TestDoneWithoutManifestAllowedOnHead(t *testing.T) {
	// A head that sets DONE without adding a manifest passes Q6.evidence;
	// the merged head must contain it (checked post-merge).
	errs := CheckEvidenceGate(t.TempDir(), strings.Repeat("a", 40), "hash", nil, nil)
	if len(errs) != 0 {
		t.Fatalf("no added manifests must be clean: %v", errs)
	}
}

func TestMergedHeadRequiresManifest(t *testing.T) {
	root := t.TempDir()
	idx := PacketIndex{"IMP-000": &Packet{Status: "DONE"}}
	if errs := CheckMergedHeadManifests(root, idx); len(errs) == 0 {
		t.Fatal("DONE packet without manifest must fail")
	}
	writeFile(t, root, "docs/10_implementation/evidence/IMP-000/manifest.json", "{}")
	if errs := CheckMergedHeadManifests(root, idx); len(errs) != 0 {
		t.Fatalf("manifest present: %v", errs)
	}
}

func TestTwoPhaseListIncludesImp083(t *testing.T) {
	var found83, found000 bool
	for _, id := range twoPhaseTaskIDs {
		found83 = found83 || id == "IMP-083"
		found000 = found000 || id == "IMP-000"
	}
	if !found83 || !found000 {
		t.Fatal("two-phase task list must include IMP-000 and IMP-083")
	}
	// The registry must carry IMP-083-owned gates so they can activate later.
	var has083 bool
	for _, s := range Registry {
		if s.Owner == "IMP-083" {
			has083 = true
		}
	}
	if !has083 {
		t.Fatal("registry has no IMP-083-owned gate")
	}
}

func TestLocalDeferMissingNeverInCi(t *testing.T) {
	// The script rejects -LocalDeferMissing under CI.
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "verify.ps1"))
	if err != nil {
		t.Fatalf("verify.ps1: %v", err)
	}
	if !strings.Contains(string(b), "LocalDeferMissing") || !strings.Contains(string(b), "CI") {
		t.Fatal("verify.ps1 must reject -LocalDeferMissing under CI")
	}
	// A missing input on CI is FAIL, never DEFERRED.
	r := &Runner{
		Root:     t.TempDir(),
		Ctx:      RunContext{OSTarget: "windows", InCI: true},
		UnityDir: t.TempDir(), // empty -> checkUnityEditor reports missing
	}
	row := r.evaluate(GateSpec{ID: "Q1.unity.editor", OS: "windows", Owner: "IMP-000"})
	if row.Result != ResultFail {
		t.Fatalf("missing input on CI must FAIL, got %s", row.Result)
	}
	r2 := &Runner{
		Root:     t.TempDir(),
		Ctx:      RunContext{OSTarget: "windows", LocalDeferMissing: true},
		UnityDir: t.TempDir(),
	}
	row = r2.evaluate(GateSpec{ID: "Q1.unity.editor", OS: "windows", Owner: "IMP-000"})
	if row.Result != ResultDeferred || row.Reason != DeferLocalMissing {
		t.Fatalf("local missing input must DEFER, got %s/%s", row.Result, row.Reason)
	}
}

// --- wrapper-to-verifier wiring + fail-closed mutation fixtures ------------

// fixtureRepo builds a minimal valid tree the IMP-000 gates accept.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		".gitattributes":            "* text=auto eol=lf\n*.asset filter=lfs diff=lfs merge=lfs -text\n*.asset merge=unityyamlmerge\n",
		".editorconfig":             "root = true\nend_of_line = lf\ninsert_final_newline = true\ncsharp_new_line_before_open_brace = all\ncsharp_style_namespace_declarations = block_scoped\n",
		".gitignore":                "client/Library/\n",
		"server/go.mod":             "module thinhthan\n\ngo 1.27.1\n",
		"server/go.sum":             "",
		"server/internal/keep/x.go": "package keep\n\n// Keep is a placeholder package for gate fixtures.\ntype Keep struct{}\n",
	}
	return makeRepo(t, files)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestOwnerNotDoneSkipRows(t *testing.T) {
	root := fixtureRepo(t)
	ctx := RunContext{OSTarget: "linux", HeadIdx: miniQueue()}
	r := &Runner{Root: root, Ctx: ctx}
	rep, err := r.Run(PhasePreUnity)
	if err != nil {
		t.Fatal(err)
	}
	var skip083 bool
	for _, g := range rep.Gates {
		if g.Owner == "IMP-083" && g.Result == ResultSkip && g.Reason == SkipOwnerNotDone {
			skip083 = true
		}
	}
	if !skip083 {
		t.Fatal("IMP-083-owned gates must report SKIP(owner-not-done)")
	}
}

func TestQ0AbsentPathsMutationFails(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, root, "server/cmd/server/main.go", "package main\n")
	cmd := exec.Command("git", "-C", root, "add", "-A")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	tracked, err := gitLsFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	errs := CheckAbsentPaths(doneQueue("server/internal/", "scripts/", ".github/"), tracked)
	if len(errs) == 0 {
		t.Fatal("unowned server/cmd/server/main.go must violate Q0.absent_paths")
	}
	// Owned file must pass.
	tracked = []string{"server/internal/sim/x.go", ".github/workflows/verify.yml", "docs/README.md"}
	errs = CheckAbsentPaths(doneQueue("server/internal/", ".github/workflows/", "docs/"), tracked)
	if len(errs) != 0 {
		t.Fatalf("owned paths must pass: %v", errs)
	}
}

func TestQ1PinsMutationFails(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, root, "server/go.mod",
		"module thinhthan\n\ngo 1.27.1\n\nrequire github.com/gorilla/mux v1.8.0\n")
	if errs := CheckPins(root); len(errs) == 0 {
		t.Fatal("unlisted require must violate Q1.pins")
	}
	// Floating go directive mutation.
	writeFile(t, root, "server/go.mod", "module thinhthan\n\ngo 1.28\n")
	if errs := CheckPins(root); len(errs) == 0 {
		t.Fatal("wrong go directive must violate Q1.pins")
	}
}

func TestForbiddenDepsMutationFails(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, root, "server/go.sum",
		"gorm.io/gorm v1.25.0 h1:abc=\n")
	if errs := CheckForbiddenDeps(root); len(errs) == 0 {
		t.Fatal("gorm in go.sum must violate Q1.forbidden_deps")
	}
	writeFile(t, root, "server/internal/x.go",
		"package x\n\nimport _ \"github.com/gorilla/websocket\"\n")
	if errs := CheckForbiddenImports(root); len(errs) == 0 {
		t.Fatal("gorilla import must violate Q1.forbidden_deps")
	}
}

func TestCscRspMutationFails(t *testing.T) {
	root := fixtureRepo(t)
	writeFile(t, root, "client/Assets/Scripts/Core/ThinhThan.Core.asmdef",
		`{"name":"ThinhThan.Core","rootNamespace":"ThinhThan.Core"}`)
	if errs := checkCscRsp(root); len(errs) == 0 {
		t.Fatal("asmdef without sibling csc.rsp must violate Q4.cscrsp")
	}
	writeFile(t, root, "client/Assets/Scripts/Core/csc.rsp", "-warnaserror+\n-nullable:enable\n")
	for _, e := range checkCscRsp(root) {
		if strings.Contains(e, "csc.rsp") && !strings.Contains(e, "asmdefs") {
			t.Fatalf("correct csc.rsp must not flag content errors: %v", e)
		}
	}
	// Mutation: missing nullable flag.
	writeFile(t, root, "client/Assets/Scripts/Core/csc.rsp", "-warnaserror+\n")
	if errs := checkCscRsp(root); len(errs) == 0 {
		t.Fatal("csc.rsp without -nullable:enable must fail")
	}
}

func TestWindowsOnlySkipOnLinux(t *testing.T) {
	root := fixtureRepo(t)
	ctx := RunContext{
		OSTarget: "linux", EventName: "pull_request", HeadBranch: "imp/x",
		UnityScope: "full", HeadIdx: doneQueue(),
	}
	r := &Runner{Root: root, Ctx: ctx}
	rep, err := r.Run(PhaseUnity)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range rep.Gates {
		if g.ID == "Q3.unity.editmode" && (g.Result != ResultSkip || g.Reason != SkipWindowsOnly) {
			t.Fatalf("unity gate on linux: %s/%s", g.Result, g.Reason)
		}
	}
	// Linux-scoped gates are evaluated on linux, absent on windows.
	ctx2 := RunContext{
		OSTarget: "windows", EventName: "pull_request", HeadBranch: "imp/x",
		UnityScope: "full", HeadIdx: doneQueue(),
	}
	r2 := &Runner{Root: root, Ctx: ctx2}
	rep2, err := r2.Run(PhasePreUnity)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range rep2.Gates {
		if g.ID == "Q3.go.race" {
			t.Fatal("linux-scoped gate must not appear in a windows report")
		}
	}
}

func TestSkipReasonEnumOnly(t *testing.T) {
	// Only the canonical skip/defer reasons may appear in reports.
	for _, s := range []string{SkipOwnerNotDone, SkipStatusOnly, SkipNoClientChange, SkipWindowsOnly, DeferLocalMissing} {
		if !allowedReasons[s] {
			t.Fatalf("canonical reason %q not allowlisted", s)
		}
	}
	row := GateRow{ID: "Q3.x", Result: ResultSkip, Reason: "flaky"}
	if !AnyFail([]GateRow{row}) {
		t.Fatal("non-canonical skip reason must be a failure")
	}
}

func TestParseQueueFieldForms(t *testing.T) {
	src := "## `IMP-001`\nstatus: DONE\nblocked_by: IMP-002, IMP-003\nowned_paths: [server/internal/sim/, docs/]\n\n## `IMP-002`\nstatus: NOT_STARTED\nBlocks: IMP-001\n"
	idx, err := ParseQueue(src)
	if err != nil {
		t.Fatal(err)
	}
	p := idx["IMP-001"]
	if p == nil || p.Status != "DONE" {
		t.Fatal("IMP-001 not parsed")
	}
	if p.BlockedBy != "IMP-002, IMP-003" {
		t.Fatalf("blocked_by: %q", p.BlockedBy)
	}
	if len(p.OwnedPaths) != 2 || p.OwnedPaths[0] != "server/internal/sim/" {
		t.Fatalf("owned_paths: %v", p.OwnedPaths)
	}
	if idx["IMP-002"].BlockedBy != "IMP-001" { // Blocks: case-insensitive
		t.Fatal("blocks: field must parse case-insensitively")
	}
}
