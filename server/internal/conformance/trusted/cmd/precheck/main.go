// Command precheck is the Gate A blocker check: it reads known_blockers.md
// and fails while any BLK-xxx entry is open (audit_gates.md: an open BLK
// blocks merge). Run against the verifier (base) checkout's canonical
// register.
//
// Usage: precheck -doc docs/10_implementation/known_blockers.md
package main

import (
	"flag"
	"fmt"
	"os"

	"thinhthan/internal/conformance/trusted"
)

func main() {
	doc := flag.String("doc", "docs/10_implementation/known_blockers.md",
		"path to known_blockers.md (canonical register on base)")
	flag.Parse()
	b, err := os.ReadFile(*doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "precheck: read %s: %v\n", *doc, err)
		os.Exit(2)
	}
	open := trusted.OpenBlockers(string(b))
	for _, id := range open {
		fmt.Println("precheck FAIL: open blocker", id)
	}
	if len(open) > 0 {
		fmt.Fprintf(os.Stderr, "precheck: Gate A fails — %d open BLK entries\n", len(open))
		os.Exit(1)
	}
	fmt.Println("precheck: no open BLK entries")
}
