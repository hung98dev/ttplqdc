package main

import (
	"fmt"
	"os"

	"thinhthan/internal/config"
)

// run executes the compile pipeline: catalogs → CandidateSnapshot →
// canonical payload + revision → coverage → content_compile_report.json.
// Exit 0 only when diagnostics carry zero errors and zero warnings.
func run(dir, report, payload, coverage string) int {
	c := &Ctx{
		Defs:     &FamilyStore{},
		Params:   &FamilyStore{},
		Geom:     &FamilyStore{},
		Diags:    &config.Diagnostics{},
		Warnings: &config.Diagnostics{},
		Cov:      &config.CoverageReport{},
		Refs:     NewNamespaceIndex(),
		Rules:    map[string]int{},
		Catalogs: map[string]*File{},
		Dir:      dir,
	}
	err := runPipeline(c)

	var snap *config.CandidateSnapshot
	var revErr error
	if err == nil && !c.Diags.HasErrors() {
		snap, revErr = emitSnapshot(c)
		if revErr != nil {
			c.Diags.Addf(config.DiagTableSyntaxError, dir, 0,
				"snapshot emit: %v", revErr)
		}
	}
	rep := buildReport(c, snap)
	if werr := rep.Write(report); werr != nil {
		fmt.Fprintf(os.Stderr, "compiler: write report: %v\n", werr)
		return 2
	}
	if payload != "" && snap != nil {
		b, cerr := config.CanonicalBytes(config.CanonicalPayload(snap))
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "compiler: payload: %v\n", cerr)
			return 2
		}
		if payload == "-" {
			os.Stdout.Write(b)
		} else if werr := os.WriteFile(payload, b, 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "compiler: write payload: %v\n", werr)
			return 2
		}
	}
	if coverage != "" && c.Cov != nil {
		b := c.Cov.Bytes()
		if werr := os.WriteFile(coverage, b, 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "compiler: write coverage: %v\n", werr)
			return 2
		}
	}

	for _, d := range *c.Diags {
		fmt.Fprintln(os.Stderr, d.String())
	}
	for _, d := range *c.Warnings {
		fmt.Fprintln(os.Stderr, "warning: "+d.String())
	}
	if snap != nil {
		fmt.Fprintf(os.Stderr, "content_revision: %s\n", snap.ContentRevision)
	}
	if c.Diags.HasErrors() || len(*c.Warnings) > 0 {
		return 1
	}
	return 0
}
