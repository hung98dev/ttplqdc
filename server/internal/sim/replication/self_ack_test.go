package replication

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestEveryDeltaCarriesSelfAck pins ADR-0069: even a delta with no entity
// or private changes still carries the complete self_ack — the client's
// prediction checkpoint.
func TestEveryDeltaCarriesSelfAck(t *testing.T) {
	b := NewBuilder()
	ck := richCheckpoint()
	cur := &View{
		Tick:                   3,
		BaselineID:             9,
		LastProcessedClientSeq: 17,
		SelfCheckpoint:         ck,
	}
	d := b.NewDelta(&View{BaselineID: 9}, cur)
	if d.SelfAck == nil {
		t.Fatal("self_ack missing on empty delta")
	}
	if d.SelfAck.LastProcessedClientSeq != 17 {
		t.Fatalf("last_processed_client_seq = %d, want 17",
			d.SelfAck.LastProcessedClientSeq)
	}
	cp := d.SelfAck.Checkpoint
	if cp == nil {
		t.Fatal("self_ack checkpoint missing")
	}
	if cp.XMm != ck.X || cp.YMm != ck.Y || cp.VxMmS != ck.Vx || cp.VyMmS != ck.Vy ||
		cp.Facing != ck.Facing || cp.MovementState != ck.MovementState ||
		cp.PlatformId != ck.PlatformID || cp.IsGrounded != ck.IsGrounded ||
		cp.JumpCount != ck.JumpCount ||
		cp.DropIgnorePlatformId != ck.DropIgnorePlatformID ||
		cp.DropIgnoreUntilTick != ck.DropIgnoreUntilTick ||
		cp.HeldHorizontalIntent != ck.HeldHorizontalIntent {
		t.Fatalf("self_ack checkpoint drift: %+v", cp)
	}
	ep := cp.EffectiveParameters
	if ep == nil || ep.RunSpeedMmS != ck.RunSpeedMmS ||
		ep.FirstJumpMmS != ck.FirstJumpMmS || ep.SecondJumpMmS != ck.SecondJumpMmS ||
		ep.GravityMmS2 != ck.GravityMmS2 || ep.MaxFallMmS != ck.MaxFallMmS ||
		ep.AirControlBp != ck.AirControlBp || ep.MaxStepHeightMm != ck.MaxStepHeightMm {
		t.Fatalf("self_ack effective parameters drift: %+v", ep)
	}
	if d.SelfPrivate != nil {
		t.Fatal("unchanged self_private present")
	}
	if len(d.Entities) != 0 {
		t.Fatalf("empty delta carried %d entities", len(d.Entities))
	}
}

// TestSelfAckSeqMonotonic verifies every delta echoes the view's
// last_processed_client_seq exactly — the runtime never regresses it, so a
// non-decreasing client seq stream produces a non-decreasing ack stream.
func TestSelfAckSeqMonotonic(t *testing.T) {
	b := NewBuilder()
	var prev uint64
	for _, seq := range []uint64{5, 9, 9, 40} {
		v := &View{Tick: 1, BaselineID: 1, LastProcessedClientSeq: seq}
		d := b.NewDelta(&View{BaselineID: 1}, v)
		got := d.SelfAck.GetLastProcessedClientSeq()
		if got != seq {
			t.Fatalf("ack seq = %d, want %d", got, seq)
		}
		if got < prev {
			t.Fatalf("ack seq regressed: %d < %d", got, prev)
		}
		prev = got
		b.Reset()
	}
}

// TestListWrapperFullReplacement pins ADR-0064: statuses and
// equipped_cosmetics are wrapped lists that carry the full current list
// whenever present — never a partial diff.
func TestListWrapperFullReplacement(t *testing.T) {
	b := NewBuilder()

	// One status drops: the wrapper carries the complete remaining list.
	prev := richSnapshot()
	cur := prev
	cur.StatusN = 1
	d := b.NewDelta(
		&View{Entities: []EntitySnapshot{prev}},
		&View{Entities: []EntitySnapshot{cur}})
	de := d.Entities[0]
	if de.Statuses == nil || len(de.Statuses.Entries) != 1 ||
		de.Statuses.Entries[0].EffectId != "buff_haste" {
		t.Fatalf("statuses not full replacement: %+v", de.Statuses)
	}
	if de.EquippedCosmetics != nil {
		t.Fatal("unchanged cosmetics present")
	}

	// Clearing the list entirely still carries the wrapper, empty.
	b.Reset()
	cur2 := prev
	cur2.StatusN = 0
	d = b.NewDelta(
		&View{Entities: []EntitySnapshot{prev}},
		&View{Entities: []EntitySnapshot{cur2}})
	if d.Entities[0].Statuses == nil || len(d.Entities[0].Statuses.Entries) != 0 {
		t.Fatalf("cleared statuses not sent as empty list: %+v", d.Entities[0].Statuses)
	}

	// A new cosmetic entry replaces the whole equipped_cosmetics list.
	b.Reset()
	cur3 := prev
	cur3.CosmeticN = 2
	cur3.Cosmetics[1] = CosmeticEntry{
		Slot:       protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE,
		CosmeticID: "cos_title_009",
	}
	d = b.NewDelta(
		&View{Entities: []EntitySnapshot{prev}},
		&View{Entities: []EntitySnapshot{cur3}})
	ce := d.Entities[0].EquippedCosmetics
	if ce == nil || len(ce.Entries) != 2 ||
		ce.Entries[1].CosmeticId != "cos_title_009" {
		t.Fatalf("cosmetics not full replacement: %+v", ce)
	}
}
