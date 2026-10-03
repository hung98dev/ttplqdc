package gates

import (
	"testing"

	"thinhthan/internal/conformance/ratchet"
)

func TestRatchetGateIDsParse(t *testing.T) {
	src := `var Registry = []GateSpec{
	{ID: "Q0.absent_paths", Owner: "IMP-000"},
	{ID: "Q6.evidence", Owner: "IMP-068", Phase: PhasePreUnity},
}`
	got := ratchetGateIDs(src)
	if len(got) != 2 || got[0] != "Q0.absent_paths" || got[1] != "Q6.evidence" {
		t.Fatalf("ratchetGateIDs = %v", got)
	}
}

// TestGateRatchetRealRepo runs the evaluator on this checkout: nothing the
// base ratchet derived may be missing from the head — green on main.
func TestGateRatchetRealRepo(t *testing.T) {
	root := repoRoot(t)
	r := &Runner{Root: root}
	details, missing := r.gateRatchet()
	if missing {
		t.Skip("no origin/main in this environment")
	}
	if len(details) != 0 {
		t.Fatalf("ratchet must be green on the real tree: %v", details)
	}
}

// TestGateRatchetShrinkFails proves the Check composition: a name present on
// base but absent on head is a failure without an ADR.
func TestGateRatchetShrinkFails(t *testing.T) {
	before := []string{"TestA", "TestB"}
	after := []string{"TestA"}
	if d := ratchet.Check(before, after, false); len(d) == 0 {
		t.Fatal("shrinking the ratchet without ADR must fail")
	}
	if d := ratchet.Check(before, after, true); len(d) != 0 {
		t.Fatalf("ADR on main sanctions the shrink: %v", d)
	}
}
