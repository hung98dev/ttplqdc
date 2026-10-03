// Package ratchet derives the permanent conformance-gate ratchet set from the
// task queue (audit_gates.md § Gates: every gate-test name in a DONE packet's
// "## Tests" becomes a permanent conformance gate) and checks a head state
// against it. The set can never shrink: removing a ratchet entry, or marking
// one skipped, requires a spec-change ADR already on main.
package ratchet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"thinhthan/internal/conformance/taskgraph"
)

const queueRelPath = "docs/10_implementation/task_queue.md"
const adrRelDir = "docs/11_decisions"

// testNameRe matches a Go test identifier in a `## Tests` entry.
var testNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// skipRe marks a ratchet entry as skipped in a `## Tests` name list.
var skipRe = regexp.MustCompile(`(?i)\[(skip|skipped)\]`)

// Derive parses queue text and returns the sorted, unique ratchet set: the
// gate-test names declared by packets whose status is DONE. Names explicitly
// marked [SKIP] are not ratcheted — a marked entry is a shrink.
func Derive(queueText string) []string {
	q, err := taskgraph.ParseQueue(queueText)
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, p := range q.Packets {
		if p == nil || p.Status != "DONE" {
			continue
		}
		for _, name := range testNames(p.Tests) {
			set[name] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// DeriveFile reads <root>/docs/10_implementation/task_queue.md and derives.
func DeriveFile(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, queueRelPath))
	if err != nil {
		return nil, err
	}
	return Derive(string(b)), nil
}

// testNames extracts the identifiers after the ":" in each `## Tests` entry,
// e.g. "- `server/x/y_test.go`: TestA, TestB" -> {"TestA","TestB"}.
func testNames(tests string) []string {
	var out []string
	for _, line := range strings.Split(tests, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "-") {
			continue
		}
		i := strings.Index(t, ":")
		if i < 0 {
			continue
		}
		for _, tok := range strings.Split(t[i+1:], ",") {
			tok = strings.TrimSpace(tok)
			if skipRe.MatchString(tok) {
				continue
			}
			if testNameRe.MatchString(tok) {
				out = append(out, tok)
			}
		}
	}
	return out
}

// Removed returns the sorted names present in before but absent from after.
func Removed(before, after []string) []string {
	have := map[string]bool{}
	for _, n := range after {
		have[n] = true
	}
	var out []string
	for _, n := range before {
		if !have[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// Check compares the base-derived ratchet set with the head-derived set.
// Any removed entry is a failure unless adrOnMain is true (a spec-change PR
// added an ADR file on main sanctioning the shrink).
func Check(before, after []string, adrOnMain bool) []string {
	removed := Removed(before, after)
	if len(removed) == 0 {
		return nil
	}
	if adrOnMain {
		return nil
	}
	var out []string
	for _, n := range removed {
		out = append(out, fmt.Sprintf("ratchet entry %q removed or skipped without an ADR on main", n))
	}
	return out
}

// ADROnMain reports whether any docs/11_decisions file was added on origin/main
// since the merge-base of HEAD and origin/main in repoDir — the only sanction
// for shrinking the ratchet set.
func ADROnMain(ctx context.Context, repoDir string) (bool, error) {
	mb, err := git(ctx, repoDir, "merge-base", "HEAD", "origin/main")
	if err != nil {
		return false, err
	}
	diff, err := git(ctx, repoDir, "diff", "--name-only", "--diff-filter=A",
		strings.TrimSpace(mb)+".."+"origin/main", "--", adrRelDir+"/")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(diff) != "", nil
}
