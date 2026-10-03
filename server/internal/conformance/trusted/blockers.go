package trusted

import (
	"strings"

	"thinhthan/internal/conformance/taskgraph"
)

// OpenBlockers returns the IDs of every BLK-xxx entry filed under the open
// section of known_blockers.md. Gate A fails while any entry is open; OPS-xxx
// entries gate through Gate D (environment), not Gate A.
func OpenBlockers(doc string) []string {
	var out []string
	for id, e := range taskgraph.ParseBlockers(doc) {
		if e.Section == "open" && strings.HasPrefix(id, "BLK-") {
			out = append(out, id)
		}
	}
	return out
}
