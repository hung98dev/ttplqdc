package replication

import (
	"bytes"
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// richSnapshot returns an EntitySnapshot with every field populated so the
// wire tests can verify complete field carriage.
func richSnapshot() EntitySnapshot {
	s := EntitySnapshot{
		ID:            77,
		Kind:          protocolv1.EntityKind_ENTITY_KIND_PLAYER,
		ContentID:     "class_swordsman",
		DisplayName:   "anh_hung",
		Level:         60,
		OwnerEntityID: 9,
		X:             12345,
		Y:             -6789,
		Vx:            5000,
		Vy:            -3200,
		Facing:        protocolv1.Facing_FACING_LEFT,
		MovementState: protocolv1.MovementState_MOVEMENT_STATE_RUN,
		HP:            999,
		MaxHP:         1200,
		Shield:        44,
		Flags:         0x3,
		EncounterID:   55,
		Stats:         [5]uint32{10, 20, 30, 40, 50},
	}
	s.CharacterID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	s.StatusN = 2
	s.Statuses[0] = StatusEntry{EffectID: "buff_haste", SourceEntityID: 5, Stacks: 3, ExpiresAtTick: 400}
	s.Statuses[1] = StatusEntry{EffectID: "dot_burn", SourceEntityID: 6, Stacks: 1, ExpiresAtTick: 500}
	s.CosmeticN = 1
	s.Cosmetics[0] = CosmeticEntry{Slot: protocolv1.CosmeticSlot_COSMETIC_SLOT_AURA, CosmeticID: "cos_aura_001"}
	return s
}

// richCheckpoint returns a fully populated checkpoint snapshot.
func richCheckpoint() CheckpointSnapshot {
	return CheckpointSnapshot{
		X:                    12345,
		Y:                    -6789,
		Vx:                   5000,
		Vy:                   -3200,
		Facing:               protocolv1.Facing_FACING_LEFT,
		MovementState:        protocolv1.MovementState_MOVEMENT_STATE_RUN,
		PlatformID:           9,
		IsGrounded:           true,
		JumpCount:            1,
		DropIgnorePlatformID: 3,
		DropIgnoreUntilTick:  77,
		HeldHorizontalIntent: protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT,
		RunSpeedMmS:          4000,
		FirstJumpMmS:         6000,
		SecondJumpMmS:        7000,
		GravityMmS2:          28000,
		MaxFallMmS:           12000,
		AirControlBp:         6000,
		MaxStepHeightMm:      400,
	}
}

// TestBaselineEntityStateFields verifies ADR-0064: the baseline carries all
// 25 EntityState fields exactly, plus self blocks and encounters.
func TestBaselineEntityStateFields(t *testing.T) {
	snap := richSnapshot()
	ck := richCheckpoint()
	mechs := [MaxMechanics]MechanicSnapshot{}
	mechs[0] = MechanicSnapshot{MechanicInstanceID: 11, StartsAtTick: 100, EndsAtTick: 200}
	mechs[1] = MechanicSnapshot{MechanicInstanceID: 22, StartsAtTick: 110, EndsAtTick: 210}
	v := &View{
		Tick:       400,
		BaselineID: 7,
		Self:       snap,
		SelfPrivate: PrivateSnapshot{
			CurrentMP:        88,
			MaxMP:            200,
			AcceptedTargetID: 77,
		},
		SelfCheckpoint:         ck,
		LastProcessedClientSeq: 41,
		Entities:               []EntitySnapshot{snap},
		Encounters: []EncounterSnapshot{{
			EncounterID: 55,
			ContentID:   "enc_boss_1",
			PhaseNumber: 2,
			MechanicN:   2,
			Mechanics:   mechs,
		}},
	}
	inst := [16]byte{0xaa, 0xbb, 0xcc, 0xdd}
	out := NewBuilder().NewBaseline(v, &BaselineParams{
		BaselineID:      7,
		ServerTick:      400,
		MapID:           "map_hue_citadel",
		ChannelIndex:    3,
		InstanceID:      inst[:],
		ContentRevision: "rev0123",
	})

	if out.BaselineId != 7 || out.ServerTick != 400 || out.MapId != "map_hue_citadel" ||
		out.ChannelIndex != 3 || out.ContentRevision != "rev0123" {
		t.Fatalf("baseline envelope wrong: %+v", out)
	}
	if !bytes.Equal(out.InstanceId, inst[:]) {
		t.Fatalf("instance_id = %x, want %x", out.InstanceId, inst)
	}
	if out.Self == nil || out.Self.EntityId != 77 {
		t.Fatal("self entity state missing")
	}
	if out.SelfPrivate == nil || out.SelfPrivate.CurrentMp != 88 ||
		out.SelfPrivate.MaxMp != 200 || out.SelfPrivate.AcceptedTargetEntityId != 77 {
		t.Fatalf("self_private wrong: %+v", out.SelfPrivate)
	}
	cp := out.SelfCheckpoint
	if cp == nil {
		t.Fatal("self_checkpoint missing")
	}
	if cp.XMm != 12345 || cp.YMm != -6789 || cp.VxMmS != 5000 || cp.VyMmS != -3200 ||
		cp.Facing != protocolv1.Facing_FACING_LEFT ||
		cp.MovementState != protocolv1.MovementState_MOVEMENT_STATE_RUN ||
		cp.PlatformId != 9 || !cp.IsGrounded || cp.JumpCount != 1 ||
		cp.DropIgnorePlatformId != 3 || cp.DropIgnoreUntilTick != 77 ||
		cp.HeldHorizontalIntent != protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT {
		t.Fatalf("self_checkpoint wrong: %+v", cp)
	}
	ep := cp.EffectiveParameters
	if ep == nil || ep.RunSpeedMmS != 4000 || ep.FirstJumpMmS != 6000 ||
		ep.SecondJumpMmS != 7000 || ep.GravityMmS2 != 28000 ||
		ep.MaxFallMmS != 12000 || ep.AirControlBp != 6000 || ep.MaxStepHeightMm != 400 {
		t.Fatalf("effective_parameters wrong: %+v", ep)
	}

	if len(out.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(out.Entities))
	}
	e := out.Entities[0]
	if e.EntityId != 77 ||
		e.EntityKind != protocolv1.EntityKind_ENTITY_KIND_PLAYER ||
		e.ContentId != "class_swordsman" ||
		!bytes.Equal(e.CharacterId, snap.CharacterID[:]) ||
		e.DisplayName != "anh_hung" ||
		e.Level != 60 ||
		e.OwnerEntityId != 9 ||
		e.XMm != 12345 || e.YMm != -6789 ||
		e.VxMmS != 5000 || e.VyMmS != -3200 ||
		e.Facing != protocolv1.Facing_FACING_LEFT ||
		e.MovementState != protocolv1.MovementState_MOVEMENT_STATE_RUN ||
		e.Hp != 999 || e.MaxHp != 1200 || e.Shield != 44 ||
		e.Flags != 0x3 ||
		e.EncounterId != 55 ||
		e.StatLifesteal != 10 || e.StatReflect != 20 || e.StatAbsorb != 30 ||
		e.StatHealReduction != 40 || e.StatHealingReceived != 50 {
		t.Fatalf("entity state drift: %+v", e)
	}
	if len(e.Statuses) != 2 ||
		e.Statuses[0].EffectId != "buff_haste" || e.Statuses[0].SourceEntityId != 5 ||
		e.Statuses[0].Stacks != 3 || e.Statuses[0].ExpiresAtTick != 400 ||
		e.Statuses[1].EffectId != "dot_burn" {
		t.Fatalf("statuses wrong: %+v", e.Statuses)
	}
	if len(e.EquippedCosmetics) != 1 ||
		e.EquippedCosmetics[0].Slot != protocolv1.CosmeticSlot_COSMETIC_SLOT_AURA ||
		e.EquippedCosmetics[0].CosmeticId != "cos_aura_001" {
		t.Fatalf("cosmetics wrong: %+v", e.EquippedCosmetics)
	}

	if len(out.Encounters) != 1 {
		t.Fatalf("encounters = %d, want 1", len(out.Encounters))
	}
	en := out.Encounters[0]
	if en.EncounterId != 55 || en.EncounterContentId != "enc_boss_1" || en.PhaseNumber != 2 {
		t.Fatalf("encounter wrong: %+v", en)
	}
	if len(en.ActiveMechanics) != 2 ||
		en.ActiveMechanics[0].MechanicInstanceId != 11 ||
		en.ActiveMechanics[0].StartsAtTick != 100 ||
		en.ActiveMechanics[1].EndsAtTick != 210 {
		t.Fatalf("mechanics wrong: %+v", en.ActiveMechanics)
	}
}

// TestDeltaOptionalPresence verifies ADR-0064 presence semantics: changed
// fields carry their new value, unchanged fields are absent, and identity
// fields never ride the delta.
func TestDeltaOptionalPresence(t *testing.T) {
	b := NewBuilder()
	prev := richSnapshot()
	cur := prev
	cur.X = prev.X + 500
	cur.HP = prev.HP - 100
	cur.StatusN = 1

	p := &View{Tick: 1, BaselineID: 9, Entities: []EntitySnapshot{prev}}
	c := &View{Tick: 2, BaselineID: 9, Entities: []EntitySnapshot{cur}}
	d := b.NewDelta(p, c)
	if len(d.Entities) != 1 {
		t.Fatalf("delta entities = %d, want 1", len(d.Entities))
	}
	de := d.Entities[0]
	if de.EntityId != cur.ID {
		t.Fatalf("entity_id = %d, want %d", de.EntityId, cur.ID)
	}
	if de.XMm == nil || *de.XMm != cur.X {
		t.Fatalf("x_mm not present with %d", cur.X)
	}
	if de.Hp == nil || *de.Hp != cur.HP {
		t.Fatalf("hp not present with %d", cur.HP)
	}
	if de.Statuses == nil || len(de.Statuses.Entries) != 1 ||
		de.Statuses.Entries[0].EffectId != "buff_haste" {
		t.Fatal("statuses did not carry full replacement")
	}
	if de.YMm != nil || de.VxMmS != nil || de.VyMmS != nil ||
		de.Facing != nil || de.MovementState != nil ||
		de.MaxHp != nil || de.Shield != nil || de.Flags != nil ||
		de.DisplayName != nil || de.Level != nil || de.OwnerEntityId != nil ||
		de.EncounterId != nil || de.StatLifesteal != nil || de.StatReflect != nil ||
		de.StatAbsorb != nil || de.StatHealReduction != nil ||
		de.StatHealingReceived != nil || de.EquippedCosmetics != nil {
		t.Fatalf("unchanged fields present in delta: %+v", de)
	}
}
