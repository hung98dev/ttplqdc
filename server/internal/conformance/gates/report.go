package gates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Report schemas emitted by the verifier.
const (
	SchemaReport    = "verify-report-v1"
	SchemaPreReport = "verify-pre-unity-v1"
	// Evidence manifest is schema_version=2 (test_and_release_evidence.md §2).
	SchemaEvidenceVersion = 2
)

// GateRow is one row of gates[] in a report/manifest.
type GateRow struct {
	ID     string `json:"id"`
	OS     string `json:"os"`
	Owner  string `json:"owner"`
	Result Result `json:"result"`
	Reason string `json:"reason,omitempty"`
}

// CommandRecord logs one subprocess invocation for the report commands[].
type CommandRecord struct {
	Argv        []string `json:"argv"`
	ExitCode    int      `json:"exit_code"`
	WallSeconds float64  `json:"wall_seconds"`
}

// BenchRecord is one -benchmem result row (report-only, ns/op + allocs/op).
type BenchRecord struct {
	Package  string  `json:"package"`
	Name     string  `json:"name"`
	NsPerOp  float64 `json:"ns_per_op"`
	AllocB   int64   `json:"alloc_b_per_op"`
	AllocOps int64   `json:"allocs_per_op"`
}

// CacheRecord is one CI-003 cache telemetry row: key, hit|miss, wall seconds.
type CacheRecord struct {
	Step        string  `json:"step"`
	Result      string  `json:"result"` // "hit" | "miss"
	WallSeconds float64 `json:"wall_seconds"`
}

// TestSummary counts individual test outcomes across go test -v + Unity XML.
type TestSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// SkippedReason is one manifest skipped_reasons[] row.
type SkippedReason struct {
	ID      string `json:"id"`
	Reason  string `json:"reason"`
	Allowed bool   `json:"allowed"`
}

// Report is a verify-report-v1 / verify-pre-unity-v1 document.
type Report struct {
	Schema         string          `json:"schema"`
	HeadSHA        string          `json:"head_sha"`
	SourceTreeHash string          `json:"source_tree_hash"`
	OS             string          `json:"os"`
	Phase          string          `json:"phase,omitempty"`
	Conclusion     string          `json:"conclusion"` // PASSED | FAILED
	Gates          []GateRow       `json:"gates"`
	SkippedReasons []SkippedReason `json:"skipped_reasons"`
	TestSummary    TestSummary     `json:"test_summary"`
	Commands       []CommandRecord `json:"commands"`
	Bench          []BenchRecord   `json:"bench,omitempty"`
	Cache          []CacheRecord   `json:"cache,omitempty"`
}

// allowedReasons is the canonical skip/defer reason set.
var allowedReasons = map[string]bool{
	SkipOwnerNotDone:   true,
	SkipStatusOnly:     true,
	SkipNoClientChange: true,
	SkipWindowsOnly:    true,
	DeferLocalMissing:  true,
}

// skippedReasonsOf derives skipped_reasons[] from gate rows.
func skippedReasonsOf(rows []GateRow) []SkippedReason {
	var out []SkippedReason
	for _, g := range rows {
		if g.Result == ResultSkip || g.Result == ResultDeferred {
			out = append(out, SkippedReason{ID: g.ID, Reason: g.Reason, Allowed: allowedReasons[g.Reason]})
		}
	}
	return out
}

// WriteReport serializes a report byte-stably (sorted keys via struct order,
// trailing newline, LF).
func WriteReport(path string, r *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	r.SkippedReasons = skippedReasonsOf(r.Gates)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadReport parses and schema-checks a report file.
func ReadReport(path, wantSchema string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	if r.Schema != wantSchema {
		return nil, fmt.Errorf("%s: schema %q, want %q", path, r.Schema, wantSchema)
	}
	return &r, nil
}

// AnyFail reports whether any gate row is FAIL or carries an illegal reason.
func AnyFail(rows []GateRow) bool {
	for _, g := range rows {
		if g.Result == ResultFail {
			return true
		}
		if (g.Result == ResultSkip || g.Result == ResultDeferred) && !allowedReasons[g.Reason] {
			return true
		}
	}
	return false
}
