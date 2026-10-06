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

func TestPlanUnityModes_ClientRuntimeDiffRunsPlayMode(t *testing.T) {
	// client/Assets/Scripts/** diff activates playmode even when IMP-065 is
	// not DONE anywhere — the suite must verify its own owner's fixes.
	ctx := RunContext{
		OSTarget:     "windows",
		UnityScope:   "full",
		ChangedPaths: []string{"client/Assets/Scripts/Core/Net/Envelope.cs"},
	}
	p := PlanUnityModes(ctx)
	if !p.PlayMode {
		t.Fatalf("client runtime diff must activate playmode: %+v", p)
	}
	if p.EditMode || p.VisualReview || p.Performance || p.GraphicalLoad {
		t.Fatalf("runtime trigger is playmode-only: %+v", p)
	}
	// .cs.meta under Scripts/ is still client-runtime surface.
	ctx.ChangedPaths = []string{"client/Assets/Scripts/Core/Net/Envelope.cs.meta"}
	if p = PlanUnityModes(ctx); !p.PlayMode {
		t.Fatalf("runtime .meta diff must activate playmode: %+v", p)
	}
	// PlayMode harness diffs self-verify too (e.g. a FakeServer-only fix).
	ctx.ChangedPaths = []string{"client/Assets/Tests/PlayMode/Net/FakeServer.cs"}
	if p = PlanUnityModes(ctx); !p.PlayMode {
		t.Fatalf("PlayMode harness diff must activate playmode: %+v", p)
	}
	// Non-runtime diffs keep playmode off.
	ctx.ChangedPaths = []string{"docs/10_implementation/task_queue.md", "server/internal/durable/queue/queue.go"}
	if p = PlanUnityModes(ctx); p.PlayMode {
		t.Fatalf("non-runtime diff must not activate playmode: %+v", p)
	}
	ctx.ChangedPaths = []string{"client/Assets/Art/UI/icon.png", "client/ProjectSettings/ProjectVersion.txt"}
	if p = PlanUnityModes(ctx); p.PlayMode {
		t.Fatalf("client non-Scripts diff must not activate playmode: %+v", p)
	}
	// EditMode-only harness diffs stay off — PlayMode cost is not justified.
	ctx.ChangedPaths = []string{"client/Assets/Tests/EditMode/AssemblyGraph/FooTests.cs"}
	if p = PlanUnityModes(ctx); p.PlayMode {
		t.Fatalf("EditMode-only diff must not activate playmode: %+v", p)
	}
}
