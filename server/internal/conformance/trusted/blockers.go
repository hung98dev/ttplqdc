package trusted

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"thinhthan/internal/conformance/taskgraph"
)

var impBranchRe = regexp.MustCompile(`^imp/(IMP-\d+)`)

// ImpTaskRef returns the IMP-NNN task a ref implements. Non-implementation
// lanes (block/, spec/, ops/, revert/, fix/ and detached heads) return "" —
// they are the lanes that declare and resolve blockers, so Gate A never
// closes on them (audit_gates.md § Gate A + §6 resolution flow).
func ImpTaskRef(ref string) string {
	if m := impBranchRe.FindStringSubmatch(strings.TrimSpace(ref)); m != nil {
		return m[1]
	}
	return ""
}

// OpenBlockers returns the IDs of every BLK-xxx entry filed under the open
// section of known_blockers.md. OPS-xxx entries gate through Gate D
// (environment), not Gate A.
func OpenBlockers(doc string) []string {
	var out []string
	for id, e := range taskgraph.ParseBlockers(doc) {
		if e.Section == "open" && strings.HasPrefix(id, "BLK-") {
			out = append(out, id)
		}
	}
	return out
}

// GateAFailures returns detail lines for every open BLK whose blocks: scope
// covers the ref's task — an explicit `blocks: <IMP-NNN>` match, `blocks: ALL`,
// or an entry with no blocks: field at all (fail closed). Non-implementation
// refs are never gated: block/ declares entries, spec/ resolves them, ops/
// clears operations blockers.
func GateAFailures(doc, ref string) []string {
	task := ImpTaskRef(ref)
	if task == "" {
		return nil
	}
	var out []string
	for id, e := range taskgraph.ParseBlockers(doc) {
		if e.Section != "open" || !strings.HasPrefix(id, "BLK-") {
			continue
		}
		if blocks(e, task) {
			out = append(out, fmt.Sprintf("open blocker %s blocks %s", id, task))
		}
	}
	sort.Strings(out)
	return out
}

func blocks(e taskgraph.BlockerEntry, task string) bool {
	if len(e.Blocks) == 0 {
		return true
	}
	for _, b := range e.Blocks {
		if b == "ALL" || b == task {
			return true
		}
	}
	return false
}
