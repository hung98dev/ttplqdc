// Command precheck is Gate A: it fails when the tested tree's
// known_blockers.md contains an open BLK entry whose blocks: scope covers the
// ref under test (explicit IMP id, ALL, or no scope field — fail closed).
// Non-implementation refs (block/, spec/, ops/, revert/) are the lanes that
// declare and resolve blockers and are never gated.
package main

import (
	"flag"
	"fmt"
	"os"

	"thinhthan/internal/conformance/trusted"
)

func main() {
	doc := flag.String("doc", "docs/10_implementation/known_blockers.md", "path to known_blockers.md")
	ref := flag.String("ref", "", "branch/ref under test (falls back to GITHUB_HEAD_REF then GITHUB_REF_NAME)")
	flag.Parse()

	r := *ref
	if r == "" {
		r = os.Getenv("GITHUB_HEAD_REF")
	}
	if r == "" {
		r = os.Getenv("GITHUB_REF_NAME")
	}

	data, err := os.ReadFile(*doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "precheck: read %s: %v\n", *doc, err)
		os.Exit(1)
	}
	if d := trusted.GateAFailures(string(data), r); len(d) != 0 {
		for _, line := range d {
			fmt.Fprintf(os.Stderr, "precheck FAIL: %s\n", line)
		}
		os.Exit(1)
	}
}
