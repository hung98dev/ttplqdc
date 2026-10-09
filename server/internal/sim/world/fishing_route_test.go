package world

import (
	"sync/atomic"
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// CAST/HOOK commands route to the channel's fishing delegates bound by
// SetFishingFactory at host construction; the command rides through
// verbatim (kind, target spot, character, preserved operation_id).
func TestInteractCastHookRouteToFishingDelegates(t *testing.T) {
	e := newTestEnv(t, testMaps())

	var gotMap, gotCh atomic.Value
	var castCalls, hookCalls atomic.Int32
	var castCmd, hookCmd atomic.Value
	e.w.SetFishingFactory(func(mapID string, ch uint32) *FishingHandlers {
		gotMap.Store(mapID)
		gotCh.Store(ch)
		return &FishingHandlers{
			Cast: func(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
				castCalls.Add(1)
				castCmd.Store(*cmd)
			},
			Hook: func(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
				hookCalls.Add(1)
				hookCmd.Store(*cmd)
			},
		}
	})

	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	if gotMap.Load() != "map.test.alpha" || gotCh.Load() != h.ch {
		t.Fatalf("factory args: got (%v,%v) want (%s,%d)",
			gotMap.Load(), gotCh.Load(), h.mapID, h.ch)
	}

	opID := [16]byte{9, 9, 9}
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid, OperationID: opID,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_CAST),
		TargetID:     77,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "cast delegate", func() bool { return castCalls.Load() == 1 })
	got := castCmd.Load().(Command)
	if got.CharacterID != cid || got.OperationID != opID || got.TargetID != 77 ||
		protocolv1.InteractKind(got.InteractKind) != protocolv1.InteractKind_INTERACT_KIND_CAST {
		t.Fatalf("cast cmd fields: %+v", got)
	}

	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid, OperationID: opID,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_HOOK),
		TargetID:     77,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "hook delegate", func() bool { return hookCalls.Load() == 1 })
	if got := hookCmd.Load().(Command); got.CharacterID != cid || got.OperationID != opID {
		t.Fatalf("hook cmd fields: %+v", got)
	}
}

// Without a bound factory CAST/HOOK are inert: the drain continues past
// them (a consult posted after still answers) and opens no session.
func TestInteractCastHookUnboundInert(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	npcEID := npcEntityID(t, h, "npc.test.alpha.guide")

	for _, kind := range []protocolv1.InteractKind{
		protocolv1.InteractKind_INTERACT_KIND_CAST,
		protocolv1.InteractKind_INTERACT_KIND_HOOK,
	} {
		if err := e.w.PostCommand(cid, &Command{
			Kind: CmdInteract, CharacterID: cid,
			InteractKind: uint32(kind),
			TargetID:     77,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// FIFO: the consult drains only after the two interacts.
	e.consultSync(t, Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     npcEID,
		ServiceID:    ServiceSetCheckpoint,
	})
	if h.sessions.Valid(npcEID, cid, h.part.TickN()) {
		t.Fatal("unbound CAST/HOOK must not open an NPC session")
	}
}
