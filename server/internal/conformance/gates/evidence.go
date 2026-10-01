package gates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"thinhthan/internal/stackpin"
)

// sourceTreeExclusions are the three paths excluded from source_tree_hash so
// a -done status PR's hash equals its tested implementation tree (ADR-0057/0068).
var sourceTreeExclusions = []string{
	"docs/10_implementation/task_queue.md",
	"docs/10_implementation/known_blockers.md",
	"docs/10_implementation/evidence/",
}

// SourceTreeHash = SHA-256 over sorted lines "path NUL git-blob-sha LF" from
// `git ls-files`, minus the three exclusions (test_and_release_evidence.md §2).
func SourceTreeHash(root string) (string, error) {
	out, err := ExecTimed(root, "git", "ls-files", "-s", "-z")
	if err != nil {
		return "", fmt.Errorf("git ls-files: %w", err)
	}
	type ent struct{ path, blob string }
	var ents []ent
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		// "<mode> <blob-sha> <stage>\t<path>"
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		path := rec[tab+1:]
		f := strings.Fields(rec[:tab])
		if len(f) < 2 {
			continue
		}
		excluded := false
		for _, ex := range sourceTreeExclusions {
			if path == ex || strings.HasPrefix(path, ex) {
				excluded = true
			}
		}
		if !excluded {
			ents = append(ents, ent{path, f[1]})
		}
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].path < ents[j].path })
	h := sha256.New()
	for _, e := range ents {
		fmt.Fprintf(h, "%s\x00%s\n", e.path, e.blob)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Toolchain pins echoed into every evidence manifest.
type Toolchain struct {
	Go          string `json:"go"`
	Unity       string `json:"unity"`
	Postgres    string `json:"postgresql"`
	Protoc      string `json:"protoc"`
	ProtocGenGo string `json:"protoc_gen_go"`
}

// JobRow is one jobs[] tuple (name, os, result).
type JobRow struct {
	Name   string `json:"name"`
	OS     string `json:"os"`
	Result string `json:"result"`
}

// DriftRow is one codegen_drift[] tuple (path, result).
type DriftRow struct {
	Path   string `json:"path"`
	Result string `json:"result"` // "IDENTICAL" when Q2 active
}

// Manifest is the schema-v2 evidence manifest committed byte-for-byte under
// docs/10_implementation/evidence/<ID>/manifest.json.
type Manifest struct {
	SchemaVersion   int             `json:"schema_version"`
	TaskID          string          `json:"task_id"`
	SourceTreeHash  string          `json:"source_tree_hash"`
	Toolchain       Toolchain       `json:"toolchain"`
	Commands        []CommandRecord `json:"commands"`
	TestSummary     TestSummary     `json:"test_summary"`
	SkippedReasons  []SkippedReason `json:"skipped_reasons"`
	ContentRevision string          `json:"content_revision"`
	CiRunID         string          `json:"ci_run_id"`
	RunAttempt      string          `json:"run_attempt"`
	Jobs            []JobRow        `json:"jobs"`
	Gates           []GateRow       `json:"gates"`
	CodegenDrift    []DriftRow      `json:"codegen_drift"`
	WorktreeClean   bool            `json:"worktree_clean"`
	Result          string          `json:"result"` // PASSED | FAILED
}

// WriteManifest serializes the evidence manifest.
func WriteManifest(path string, m *Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadManifest parses a schema-v2 evidence manifest.
func ReadManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	if m.SchemaVersion != SchemaEvidenceVersion {
		return nil, fmt.Errorf("%s: schema_version %d, want %d", path, m.SchemaVersion, SchemaEvidenceVersion)
	}
	return &m, nil
}

// ValidateManifestSchema checks schema-level validity only (used for older
// manifests not touched by the diff).
func ValidateManifestSchema(path string) []string {
	m, err := ReadManifest(path)
	if err != nil {
		return []string{err.Error()}
	}
	var errs []string
	if m.Result != "PASSED" {
		errs = append(errs, "manifest result is not PASSED")
	}
	if m.SourceTreeHash == "" || m.TaskID == "" {
		errs = append(errs, "manifest missing task_id/source_tree_hash")
	}
	for _, g := range m.Gates {
		if g.Result == ResultFail {
			errs = append(errs, "manifest gate row FAIL: "+g.ID)
		}
	}
	return errs
}

// MergeReports combines the linux + windows verify-reports into one
// schema-v2 evidence manifest (test_and_release_evidence.md §2). Merged Unity
// verdicts use the Windows rows only; Linux's SKIP(windows-only) rows for
// those gates are dropped.
func MergeReports(linuxPath, windowsPath string) (*Manifest, []string) {
	var errs []string
	m := &Manifest{
		SchemaVersion:   SchemaEvidenceVersion,
		ContentRevision: "none",
		CodegenDrift:    []DriftRow{},
		Toolchain: Toolchain{
			Go:          stackpin.GoVersion,
			Unity:       stackpin.UnityEditorVersion,
			Postgres:    stackpin.PostgresVersion,
			Protoc:      stackpin.ProtocVersion,
			ProtocGenGo: stackpin.ProtocGenGoVersion,
		},
		CiRunID:    os.Getenv("GITHUB_RUN_ID"),
		RunAttempt: os.Getenv("GITHUB_RUN_ATTEMPT"),
		Result:     "PASSED",
	}
	reports := map[string]*Report{}
	for _, spec := range []struct{ osName, path string }{
		{"linux", linuxPath}, {"windows", windowsPath},
	} {
		r, err := ReadReport(spec.path, SchemaReport)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s report: %v", spec.osName, err))
			continue
		}
		reports[spec.osName] = r
	}
	if reports["linux"] == nil || reports["windows"] == nil {
		errs = append(errs, "merged manifest needs both linux and windows reports")
		m.Result = "FAILED"
		return m, errs
	}
	lr, wr := reports["linux"], reports["windows"]
	if lr.HeadSHA != wr.HeadSHA {
		errs = append(errs, "head_sha differs between reports")
	}
	if lr.SourceTreeHash != wr.SourceTreeHash {
		errs = append(errs, "source_tree_hash differs between reports (CI-004)")
	}
	m.SourceTreeHash = wr.SourceTreeHash
	m.WorktreeClean = lr.Conclusion == "PASSED" && wr.Conclusion == "PASSED"
	m.Jobs = []JobRow{
		{Name: "Q0-Q6 verify (Linux)", OS: "linux", Result: lr.Conclusion},
		{Name: "Q0-Q6 verify (Windows)", OS: "windows", Result: wr.Conclusion},
	}
	for _, g := range wr.Gates {
		g.OS = "windows"
		m.Gates = append(m.Gates, g)
	}
	for _, g := range lr.Gates {
		if g.Reason == SkipWindowsOnly {
			continue // Windows rows carry the real verdict
		}
		g.OS = "linux"
		m.Gates = append(m.Gates, g)
	}
	sort.Slice(m.Gates, func(i, j int) bool {
		if m.Gates[i].OS != m.Gates[j].OS {
			return m.Gates[i].OS < m.Gates[j].OS
		}
		return m.Gates[i].ID < m.Gates[j].ID
	})
	for _, r := range []*Report{lr, wr} {
		m.Commands = append(m.Commands, r.Commands...)
		m.TestSummary.Total += r.TestSummary.Total
		m.TestSummary.Passed += r.TestSummary.Passed
		m.TestSummary.Failed += r.TestSummary.Failed
		m.TestSummary.Skipped += r.TestSummary.Skipped
	}
	m.SkippedReasons = skippedReasonsOf(m.Gates)
	if AnyFail(m.Gates) || lr.Conclusion != "PASSED" || wr.Conclusion != "PASSED" {
		m.Result = "FAILED"
	}
	if len(errs) > 0 {
		m.Result = "FAILED"
	}
	return m, errs
}

// CheckEvidenceGate (Q6.evidence): manifest files *added in the PR diff* get
// full validation (schema + source_tree_hash equals the head tree hash +
// ci_run_id/run_attempt confirmed via the GitHub API for workflow verify.yml
// with conclusion success). Manifests older than the diff get schema checks
// only. A DONE transition without an added manifest passes here — the merged
// head must contain it (post-merge guard).
func CheckEvidenceGate(root, headSHA, headTreeHash string, addedPaths []string, apiCheck func(runID, attempt string) (workflow, conclusion string, err error)) []string {
	var errs []string
	for _, p := range addedPaths {
		if !strings.HasPrefix(p, "docs/10_implementation/evidence/") || !strings.HasSuffix(p, "/manifest.json") {
			continue
		}
		mp := filepath.Join(root, filepath.FromSlash(p))
		m, err := ReadManifest(mp)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		errs = append(errs, ValidateManifestSchema(mp)...)
		if m.SourceTreeHash != headTreeHash {
			errs = append(errs, fmt.Sprintf("%s: source_tree_hash != head tree hash", p))
		}
		if apiCheck != nil {
			wf, conc, err := apiCheck(m.CiRunID, m.RunAttempt)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: api check: %v", p, err))
			} else if wf != "verify.yml" || conc != "success" {
				errs = append(errs, fmt.Sprintf("%s: ci_run_id=%s attempt=%s -> %s/%s (want verify.yml/success)", p, m.CiRunID, m.RunAttempt, wf, conc))
			}
		}
	}
	return errs
}

// CheckMergedHeadManifests asserts every packet marked DONE in headIdx has a
// manifest under docs/10_implementation/evidence/<ID>/ in the tree — the
// merged-head requirement enforced by the post-merge guard.
func CheckMergedHeadManifests(root string, headIdx PacketIndex) []string {
	var errs []string
	for id, p := range headIdx {
		if p.Status != "DONE" {
			continue
		}
		mp := filepath.Join(root, "docs", "10_implementation", "evidence", id, "manifest.json")
		if _, err := os.Stat(mp); err != nil {
			errs = append(errs, fmt.Sprintf("DONE packet %s has no evidence manifest", id))
		}
	}
	return errs
}
