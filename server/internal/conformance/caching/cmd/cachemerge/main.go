// Command cachemerge folds cache telemetry JSONL into a verify-report.json
// document as a top-level cached_steps[] array (.devin/scripts/cache-policy.md
// §Telemetry, CI-003). verify.ps1 invokes it best-effort after the canonical
// report is written; the evidence merge excludes cached_steps, so the fold
// never changes the CI-004 stable projection.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"thinhthan/internal/conformance/gates"
)

type reportDoc struct {
	gates.Report
	CachedSteps []gates.CacheRecord `json:"cached_steps,omitempty"`
}

func main() { os.Exit(run()) }

func run() int {
	reportPath := flag.String("report", "", "verify-report.json path to update in place (required)")
	telemetryPath := flag.String("telemetry", defaultTelemetryPath(), "cache telemetry JSONL (default $THINHTHAN_CACHE_TELEMETRY or $RUNNER_TEMP/cache-telemetry.jsonl)")
	outPath := flag.String("out", "", "output path (default: overwrite -report)")
	flag.Parse()
	if *reportPath == "" {
		fmt.Fprintln(os.Stderr, "cachemerge: -report is required")
		return 2
	}
	out := *outPath
	if out == "" {
		out = *reportPath
	}
	if err := fold(*reportPath, *telemetryPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "cachemerge: %v\n", err)
		return 1
	}
	return 0
}

func defaultTelemetryPath() string {
	if p := os.Getenv("THINHTHAN_CACHE_TELEMETRY"); p != "" {
		return p
	}
	if t := os.Getenv("RUNNER_TEMP"); t != "" {
		return filepath.Join(t, "cache-telemetry.jsonl")
	}
	return ""
}

func fold(reportPath, telemetryPath, outPath string) error {
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read report: %w", err)
	}
	var doc reportDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", reportPath, err)
	}
	if doc.Schema != gates.SchemaReport {
		return fmt.Errorf("%s: schema %q, want %q", reportPath, doc.Schema, gates.SchemaReport)
	}
	steps, err := readTelemetry(telemetryPath)
	if errors.Is(err, fs.ErrNotExist) {
		steps = nil // no telemetry recorded this run — report unchanged
	} else if err != nil {
		return err
	}
	if len(steps) > 0 {
		doc.CachedSteps = steps
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append(b, '\n'), 0o644)
}

func readTelemetry(path string) ([]gates.CacheRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []gates.CacheRecord
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		txt := strings.TrimSpace(sc.Text())
		if txt == "" {
			continue
		}
		var rec gates.CacheRecord
		if err := json.Unmarshal([]byte(txt), &rec); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if rec.Step == "" || (rec.Result != "hit" && rec.Result != "miss") {
			return nil, fmt.Errorf("%s:%d: invalid record %q", path, line, txt)
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}
