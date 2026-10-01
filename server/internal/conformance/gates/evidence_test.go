package gates

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSourceTreeHashExclusions(t *testing.T) {
	root := makeRepo(t, map[string]string{
		"server/main.go":                                        "package main\nfunc main(){}\n",
		"docs/10_implementation/task_queue.md":                  "queue\n",
		"docs/10_implementation/known_blockers.md":              "blockers\n",
		"docs/10_implementation/evidence/IMP-000/manifest.json": "{}\n",
	})
	h1, err := SourceTreeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	// Excluded files must not affect the hash.
	writeFile(t, root, "docs/10_implementation/evidence/IMP-000/manifest.json", "{\"x\":1}\n")
	writeFile(t, root, "docs/10_implementation/task_queue.md", "queue v2\n")
	if out, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	h2, err := SourceTreeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("excluded files (evidence/, task_queue.md) must not change source_tree_hash")
	}
	writeFile(t, root, "server/main.go", "package main\n")
	h3repo := makeRepo(t, map[string]string{
		"server/main.go":                           "package main\n",
		"docs/10_implementation/task_queue.md":     "queue v3\n",
		"docs/10_implementation/known_blockers.md": "b\n",
	})
	h3, err := SourceTreeHash(h3repo)
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 {
		t.Fatal("tracked source change must change source_tree_hash")
	}
}

func TestSourceTreeHashIdenticalAcrossOs(t *testing.T) {
	files := map[string]string{
		"a/b.go":                               "package b\n",
		"docs/10_implementation/task_queue.md": "q\n",
	}
	r1 := makeRepo(t, files)
	r2 := makeRepo(t, files)
	h1, err := SourceTreeHash(r1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := SourceTreeHash(r2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("source_tree_hash must be identical across filesystems/OS (blob SHAs)")
	}
}

func TestManifestSchemaV2(t *testing.T) {
	m := &Manifest{
		SchemaVersion:   SchemaEvidenceVersion,
		TaskID:          "IMP-000",
		SourceTreeHash:  "abc",
		Result:          "PASSED",
		ContentRevision: "none",
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var sv int
	if err := json.Unmarshal(raw["schema_version"], &sv); err != nil || sv != 2 {
		t.Fatalf("schema_version must be 2, got %s", raw["schema_version"])
	}
	for _, k := range []string{"task_id", "source_tree_hash", "toolchain", "commands", "test_summary",
		"skipped_reasons", "content_revision", "ci_run_id", "run_attempt", "jobs", "gates",
		"codegen_drift", "worktree_clean", "result"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("manifest missing field %q", k)
		}
	}
	// v1 file must be rejected.
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(p, []byte(`{"schema_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("schema_version 1 must be rejected")
	}
}

func reportFile(t *testing.T, dir, name string, rep *Report) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := WriteReport(p, rep); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEvidenceManifestJobMergesBothReports(t *testing.T) {
	dir := t.TempDir()
	lr := &Report{
		Schema: SchemaReport, OS: "linux", HeadSHA: "abc", SourceTreeHash: "h",
		Conclusion: "PASSED",
		Gates: []GateRow{
			{ID: "Q3.go.test", OS: "linux", Owner: "IMP-000", Result: ResultPass},
			{ID: "Q3.unity.editmode", OS: "linux", Owner: "IMP-000", Result: ResultSkip, Reason: SkipWindowsOnly},
		},
	}
	wr := &Report{
		Schema: SchemaReport, OS: "windows", HeadSHA: "abc", SourceTreeHash: "h",
		Conclusion: "PASSED",
		Gates: []GateRow{
			{ID: "Q3.go.test", OS: "windows", Owner: "IMP-000", Result: ResultPass},
			{ID: "Q3.unity.editmode", OS: "windows", Owner: "IMP-000", Result: ResultPass},
		},
	}
	m, errs := MergeReports(reportFile(t, dir, "linux.json", lr), reportFile(t, dir, "windows.json", wr))
	if len(errs) != 0 {
		t.Fatalf("merge: %v", errs)
	}
	if m.Result != "PASSED" || len(m.Jobs) != 2 {
		t.Fatalf("merged manifest wrong: %v", m)
	}
	var linuxEditmode bool
	for _, g := range m.Gates {
		if g.ID == "Q3.unity.editmode" && g.OS == "linux" {
			linuxEditmode = true
		}
	}
	if linuxEditmode {
		t.Fatal("linux SKIP(windows-only) rows must be dropped from merged gates")
	}
	if m.SchemaVersion != 2 || m.SourceTreeHash != "h" {
		t.Fatal("manifest identity fields wrong")
	}
	// Hash mismatch must fail.
	lr.SourceTreeHash = "h-other"
	m, errs = MergeReports(reportFile(t, dir, "linux2.json", lr), reportFile(t, dir, "windows.json", wr))
	if len(errs) == 0 || m.Result != "FAILED" {
		t.Fatal("CI-004 hash mismatch must fail the merge")
	}
}

func TestRunIdAttemptApiCheck(t *testing.T) {
	root := t.TempDir()
	hash := "deadbeef"
	m := &Manifest{
		SchemaVersion:   SchemaEvidenceVersion,
		TaskID:          "IMP-000",
		SourceTreeHash:  hash,
		Result:          "PASSED",
		ContentRevision: "none",
		CiRunID:         "123",
		RunAttempt:      "2",
		Gates:           []GateRow{},
	}
	p := filepath.Join(root, "docs", "10_implementation", "evidence", "IMP-000")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(filepath.Join(p, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	added := []string{"docs/10_implementation/evidence/IMP-000/manifest.json"}
	var gotRun, gotAttempt string
	bad := func(runID, attempt string) (string, string, error) {
		gotRun, gotAttempt = runID, attempt
		return "other.yml", "success", nil
	}
	errs := CheckEvidenceGate(root, "sha", hash, added, bad)
	if len(errs) == 0 {
		t.Fatal("manifest pointing at another workflow must fail")
	}
	if gotRun != "123" || gotAttempt != "2" {
		t.Fatalf("api check got %s/%s", gotRun, gotAttempt)
	}
	ok := func(runID, attempt string) (string, string, error) {
		return "verify.yml", "success", nil
	}
	if errs := CheckEvidenceGate(root, "sha", hash, added, ok); len(errs) != 0 {
		t.Fatalf("verify.yml/success must pass: %v", errs)
	}
	errFn := func(runID, attempt string) (string, string, error) {
		return "", "", fmt.Errorf("api down")
	}
	if errs := CheckEvidenceGate(root, "sha", hash, added, errFn); len(errs) == 0 {
		t.Fatal("API failure must fail the gate")
	}
}

func TestTwoPhaseStatusPrOwnRunEvidence(t *testing.T) {
	// On a -done PR the added manifest is validated in the same run: it must
	// match the head tree hash and the API-confirmed run identity.
	dir := t.TempDir()
	root := dir
	hash, err := SourceTreeHash(makeRepo(t, map[string]string{"x.txt": "x\n"}))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "docs", "10_implementation", "evidence", "IMP-000")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		SchemaVersion:   SchemaEvidenceVersion,
		TaskID:          "IMP-000",
		SourceTreeHash:  hash,
		Result:          "PASSED",
		ContentRevision: "none",
		Gates:           []GateRow{},
	}
	if err := WriteManifest(filepath.Join(p, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	// Hash not matching head must fail in the same run.
	errs := CheckEvidenceGate(root, "sha", "different-hash",
		[]string{"docs/10_implementation/evidence/IMP-000/manifest.json"}, nil)
	if len(errs) == 0 {
		t.Fatal("own-run manifest must match head tree hash")
	}
	if errs := CheckEvidenceGate(root, "sha", hash,
		[]string{"docs/10_implementation/evidence/IMP-000/manifest.json"}, nil); len(errs) != 0 {
		t.Fatalf("own-run manifest matching head must pass: %v", errs)
	}
}
