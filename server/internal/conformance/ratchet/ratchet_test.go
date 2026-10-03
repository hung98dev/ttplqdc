package ratchet

import (
	"strings"
	"testing"
)

const queueFixture = `## ` + "`IMP-001`" + ` — First task
id: IMP-001
status: DONE
owned_paths: [` + "`server/internal/x/`" + `]

## Tests
- ` + "`server/internal/x/x_test.go`" + `: TestAlpha, TestBeta
- ` + "`server/internal/x/y_test.go`" + `: TestGamma

## ` + "`IMP-002`" + ` — Second task
id: IMP-002
status: IN_PROGRESS
owned_paths: [` + "`server/internal/y/`" + `]

## Tests
- ` + "`server/internal/y/y_test.go`" + `: TestDelta

## ` + "`IMP-003`" + ` — Third task
id: IMP-003
status: DONE
owned_paths: [` + "`server/internal/z/`" + `]

## Tests
- ` + "`server/internal/z/z_test.go`" + `: TestEpsilon
`

func TestRatchetDerivedFromDonePackets(t *testing.T) {
	got := Derive(queueFixture)
	want := []string{"TestAlpha", "TestBeta", "TestEpsilon", "TestGamma"}
	if len(got) != len(want) {
		t.Fatalf("Derive = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Derive[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRatchetDecreaseNeedsAdrOnMain(t *testing.T) {
	before := Derive(queueFixture)
	// Head drops IMP-003 entirely (packet un-DONE / tests removed) plus renames
	// TestBeta: both names leave the ratchet set, sanctioned only by an ADR on
	// main.
	headQueue := strings.Replace(queueFixture, "TestBeta", "TestBetaV2", 1)
	cut := strings.Index(headQueue, "## `IMP-003`")
	head := headQueue[:cut]
	after := Derive(head)
	if d := Check(before, after, true); len(d) != 0 {
		t.Fatalf("shrink with ADR on main should pass, got %v", d)
	}
	if d := Check(before, after, false); len(d) != 2 {
		t.Fatalf("shrink without ADR should fail on 2 names, got %v", d)
	}
}

func TestSkipWithoutAdrFails(t *testing.T) {
	before := Derive(queueFixture)
	// A [SKIP] mark removes the name from the ratchet set: same failure class
	// as deletion, requiring an ADR on main.
	head := strings.Replace(queueFixture, "TestGamma", "TestGamma [SKIP]", 1)
	after := Derive(head)
	d := Check(before, after, false)
	if len(d) != 1 || !strings.Contains(d[0], "TestGamma") {
		t.Fatalf("skip-marked ratchet entry without ADR should fail, got %v", d)
	}
	if d := Check(before, after, true); len(d) != 0 {
		t.Fatalf("skip-marked entry with ADR on main should pass, got %v", d)
	}
}
