package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// CompileReportName is where `go -C server run ./cmd/compiler` writes the
// §4 content_compile_report.json (repo-root-relative). MergeReports reads it
// for the evidence manifest's content_revision; .gitignore excludes it.
const CompileReportName = "content_compile_report.json"

// compileRevisionHex matches the §4 contract: a full 64-lowercase-hex
// content_revision (content_authoring_contract.md), not a subset hash.
var compileRevisionHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// compileReport mirrors the emitted content_compile_report.json fields. It is
// declared here (rather than decoding config.CompileReport) because the
// canonical writer, not struct tags, defines the JSON keys.
type compileReport struct {
	ContentRevisionHash   string              `json:"content_revision_hash"`
	CatalogsEvaluated     []string            `json:"catalogs_evaluated"`
	EntityCounts          map[string]int      `json:"entity_counts"`
	ValidationDiagnostics []compileReportDiag `json:"validation_diagnostics"`
}

type compileReportDiag struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func readCompileReport(path string) (*compileReport, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("compile report: %w", err)
	}
	var rep compileReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("compile report: invalid JSON: %w", err)
	}
	return &rep, nil
}

// validateCompileReport applies the §4 contract to a parsed report:
// 0 validation diagnostics, a 64-lowercase-hex revision, catalogs evaluated,
// entity counts present.
func validateCompileReport(rep *compileReport) []string {
	var errs []string
	if !compileRevisionHex.MatchString(rep.ContentRevisionHash) {
		errs = append(errs, "content_revision_hash is not a 64-lowercase-hex revision")
	}
	if len(rep.CatalogsEvaluated) == 0 {
		errs = append(errs, "catalogs_evaluated is empty")
	}
	if len(rep.EntityCounts) == 0 {
		errs = append(errs, "entity_counts is empty")
	}
	for i, d := range rep.ValidationDiagnostics {
		if i >= 20 {
			errs = append(errs, fmt.Sprintf("...and %d more diagnostics", len(rep.ValidationDiagnostics)-20))
			break
		}
		errs = append(errs, fmt.Sprintf("%s %s:%d %s", d.Code, d.File, d.Line, d.Message))
	}
	return errs
}

// contentCompile runs the IMP-003 compiler against docs/07_content and
// validates the emitted report (test_and_release_evidence.md §4). The report
// file persists at the repo root so the evidence merge can wire
// content_revision. A nonzero compiler exit with a parseable report reports
// the diagnostics; otherwise the run output tail is the finding.
func (r *Runner) contentCompile() (details []string, missing bool) {
	if _, err := exec.LookPath("go"); err != nil {
		return nil, true
	}
	reportPath := filepath.Join(r.Root, CompileReportName)
	out, code := r.runCmd(context.Background(), filepath.Join(r.Root, "server"),
		"go", "run", "./cmd/compiler", "-report", reportPath)
	if code != 0 {
		if rep, err := readCompileReport(reportPath); err == nil && len(rep.ValidationDiagnostics) > 0 {
			return validateCompileReport(rep), false
		}
		return []string{"compiler exit " + fmt.Sprint(code) + ": " + tail(out)}, false
	}
	rep, err := readCompileReport(reportPath)
	if err != nil {
		return []string{err.Error()}, false
	}
	return validateCompileReport(rep), false
}
