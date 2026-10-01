package gates

import "testing"

func TestPlanUnityModes(t *testing.T) {
	// scope=full + owner DONE -> mode on.
	ctx := RunContext{
		OSTarget:   "windows",
		UnityScope: "full",
		MainIdx: PacketIndex{
			"IMP-000": &Packet{Status: "DONE"},
			"IMP-065": &Packet{Status: "DONE"},
		},
	}
	p := PlanUnityModes(ctx)
	if !p.EditMode || !p.PlayMode {
		t.Fatalf("editmode+playmode expected on: %+v", p)
	}
	if p.VisualReview || p.Performance || p.GraphicalLoad {
		t.Fatalf("inactive owners must be off: %+v", p)
	}
	// scope=none -> every mode off.
	ctx.UnityScope = "none"
	p = PlanUnityModes(ctx)
	if p.EditMode || p.PlayMode || p.VisualReview || p.Performance || p.GraphicalLoad {
		t.Fatalf("scope=none must disable all modes: %+v", p)
	}
	// status-only -> all off even when scope=full.
	ctx.UnityScope = "full"
	ctx.StatusOnly = true
	p = PlanUnityModes(ctx)
	if p.EditMode || p.PlayMode || p.VisualReview || p.Performance || p.GraphicalLoad {
		t.Fatalf("status-only must disable all modes: %+v", p)
	}
	// owner DONE on head only also activates.
	ctx = RunContext{
		OSTarget:   "windows",
		UnityScope: "full",
		HeadIdx:    PacketIndex{"IMP-070": &Packet{Status: "DONE"}},
	}
	p = PlanUnityModes(ctx)
	if !p.VisualReview {
		t.Fatal("head-DONE owner must activate its mode")
	}
	if p.EditMode {
		t.Fatal("IMP-000 not DONE -> editmode off")
	}
}
