// Command ratchet is the Gate C ratchet check: it derives the permanent
// gate-test set from the base checkout's task queue and fails when the head
// checkout's queue removed or skipped a ratcheted name without a new ADR on
// origin/main (audit_gates.md: the ratchet set can never shrink).
//
// Usage: ratchet -base <verifier checkout> -head <target checkout>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"thinhthan/internal/conformance/ratchet"
)

func main() {
	base := flag.String("base", "", "base checkout root (verifier source, e.g. main)")
	head := flag.String("head", "", "head checkout root (target under test)")
	flag.Parse()
	if *base == "" || *head == "" {
		fmt.Fprintln(os.Stderr, "ratchet: -base and -head are required")
		os.Exit(2)
	}
	ctx := context.Background()

	baseSet, err := ratchet.DeriveFile(*base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ratchet: derive base queue: %v\n", err)
		os.Exit(2)
	}
	headSet, err := ratchet.DeriveFile(*head)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ratchet: derive head queue: %v\n", err)
		os.Exit(2)
	}
	adr, err := ratchet.ADROnMain(ctx, *head)
	if err != nil {
		// A head checkout without origin/main cannot prove the ADR sanction;
		// fail closed: the shrink is treated as unsanctioned.
		adr = false
	}
	for _, d := range ratchet.Check(baseSet, headSet, adr) {
		fmt.Println("ratchet FAIL:", d)
	}
	if len(ratchet.Check(baseSet, headSet, adr)) > 0 {
		os.Exit(1)
	}
	abs, _ := filepath.Abs(*head)
	fmt.Printf("ratchet: %d entries derived on base, head set OK (adr_on_main=%t) [%s]\n",
		len(baseSet), adr, abs)
}
