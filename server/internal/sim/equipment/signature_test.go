package equipment

import "testing"

// Support signature selection: deterministic priority/id order,
// threshold gating, and the per-loadout/per-character caps.
func TestSupportSignatureSelection(t *testing.T) {
	defs := []SetSignature{
		{SetKey: "kim_t1", SupportID: "support.set.kim_t1", Priority: 20, Threshold: 2},
		{SetKey: "moc_t1", SupportID: "support.set.moc_t1", Priority: 20, Threshold: 2},
	}
	facts := BuildFacts{SetPieces: map[string]int{"kim_t1": 2, "moc_t1": 3}}
	got, ok := SelectSignature(facts, defs)
	if !ok {
		t.Fatal("expected a signature at threshold")
	}
	if got.SupportID != "support.set.kim_t1" { // same priority -> lexical
		t.Fatalf("deterministic pick = %s", got.SupportID)
	}
	// Under threshold -> none.
	if _, ok := SelectSignature(BuildFacts{SetPieces: map[string]int{"kim_t1": 1}}, defs); ok {
		t.Fatal("below threshold must not match")
	}
	// >4pc requirement is a spec violation -> never eligible.
	bad := []SetSignature{{SetKey: "x", SupportID: "s", Priority: 99, Threshold: 5}}
	if _, ok := SelectSignature(BuildFacts{SetPieces: map[string]int{"x": 14}}, bad); ok {
		t.Fatal("5pc signature must be rejected by the guardrail")
	}
	// At most one per SUPPORT loadout, two per character.
	sigs := Signatures([]BuildFacts{facts, facts}, defs)
	if len(sigs) != 2 {
		t.Fatalf("two supports -> %d signatures", len(sigs))
	}
	sigs = Signatures([]BuildFacts{facts, facts, facts}, defs)
	if len(sigs) != MaxSupportSignatures {
		t.Fatalf("cap violated: %d", len(sigs))
	}
}
