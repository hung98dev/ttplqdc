package gates

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"thinhthan/internal/conformance/ratchet"
)

var gateIDRe = regexp.MustCompile(`\{ID:\s*"([A-Za-z0-9._]+)"`)

// ratchetGateIDs extracts the registered gate IDs from a runner.go source —
// the base side of the gate-list ratchet (audit_gates.md: the ratchet is
// derived on the base branch from the verifier gate list plus tests named in
// DONE packets).
func ratchetGateIDs(src string) []string {
	set := map[string]bool{}
	for _, m := range gateIDRe.FindAllStringSubmatch(src, -1) {
		set[m[1]] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// gateRatchet evaluates the permanent conformance ratchet on the tested tree:
// base-derived entries (DONE-packet test names + registered gate IDs) may
// never shrink without an ADR already on origin/main. The head side is read
// from the tested tree's own queue and runner.go — not from this verifier's
// compiled Registry — so a PR cannot edit its own judge.
func (r *Runner) gateRatchet() (details []string, missing bool) {
	baseQueue, err := GitShowMain(r.Root, "docs/10_implementation/task_queue.md", "")
	if err != nil || baseQueue == "" {
		return nil, true
	}
	baseRunner, err := GitShowMain(r.Root, "server/internal/conformance/gates/runner.go", "")
	if err != nil || baseRunner == "" {
		return nil, true
	}
	headQueue, err := os.ReadFile(filepath.Join(r.Root, "docs/10_implementation/task_queue.md"))
	if err != nil {
		return []string{"read head task_queue.md: " + err.Error()}, false
	}
	headRunner, err := os.ReadFile(filepath.Join(r.Root, "server/internal/conformance/gates/runner.go"))
	if err != nil {
		return []string{"read head runner.go: " + err.Error()}, false
	}
	before := append(ratchet.Derive(baseQueue), ratchetGateIDs(baseRunner)...)
	after := append(ratchet.Derive(string(headQueue)), ratchetGateIDs(string(headRunner))...)
	adr, err := ratchet.ADROnMain(context.Background(), r.Root)
	if err != nil {
		// Cannot prove the sanction — the shrink is treated as unsanctioned.
		adr = false
	}
	return ratchet.Check(before, after, adr), false
}
