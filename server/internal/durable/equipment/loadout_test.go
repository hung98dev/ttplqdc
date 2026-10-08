package equipment

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/character"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	protocolv1 "thinhthan/internal/protocol/v1"
)

func newDeps(t *testing.T) Deps {
	t.Helper()
	p := pool(t)
	return Deps{
		Store:      New(p),
		Items:      items.New(p),
		Characters: character.NewStore(p),
		Defs:       testDefs(),
	}
}

func newIdem(t *testing.T) *idempotency.Store {
	t.Helper()
	return idempotency.NewStore(pool(t))
}

// TestLoadoutChangeEquipUnequipSwitch — the ADR-0060 acceptance path:
// EQUIP displaces the occupant to inventory, UNEQUIP requires capacity
// and returns a contracted Soul to Collection atomically, and
// SWITCH_ACTIVE flips the role without items passing through inventory.
func TestLoadoutChangeEquipUnequipSwitch(t *testing.T) {
	acct := mkAccount(t)
	charID := mkCharacter(t, acct)
	d := newDeps(t)
	idem := newIdem(t)
	ctx := context.Background()

	// EQUIP: item moves to EQUIPPED "loadout.primary.weapon".
	weapon := mkItem(t, charID, "item.eq.weapon", 0)
	res := runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "weapon", ItemInstanceId: weapon[:],
			}}
		}))
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("equip result: %v", res.GetResult().GetErrorCode())
	}
	kind, _, slot := locationOf(t, weapon)
	if kind != "EQUIPPED" || slot != "loadout.primary.weapon" {
		t.Fatalf("weapon location = %s %s", kind, slot)
	}
	if got := len(res.GetChangedSlots()); got != 1 {
		t.Fatalf("changed_slots %d", got)
	}
	rev1 := res.GetLoadoutRevision()
	if rev1 != 1 {
		t.Fatalf("loadout_revision = %d want 1 (one row bumped)", rev1)
	}

	// EQUIP a second weapon over the occupied slot: the occupant
	// displaces to the first free inventory slot (weapon2 took inv.0,
	// so the first free slot is inv.1).
	weapon2 := mkItem(t, charID, "item.eq.weapon", 0)
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "weapon", ItemInstanceId: weapon2[:],
			}}
		}))
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("displace equip: %v", res.GetResult().GetErrorCode())
	}
	kind, _, slot = locationOf(t, weapon)
	if kind != "CHARACTER_INVENTORY" || slot != "inv.1" {
		t.Fatalf("displaced occupant -> %s %s", kind, slot)
	}
	kind, _, slot = locationOf(t, weapon2)
	if kind != "EQUIPPED" || slot != "loadout.primary.weapon" {
		t.Fatalf("new weapon -> %s %s", kind, slot)
	}
	if res.GetLoadoutRevision() != 2 {
		t.Fatalf("revision after 2 equips = %d", res.GetLoadoutRevision())
	}

	// UNEQUIP: contracts a soul onto the equipped item first — the soul
	// must return to Collection atomically and be reported on 403.
	soul := mkSoul(t, charID, weapon2)
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_UNEQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Unequip{Unequip: &protocolv1.LoadoutUnequip{
				LoadoutId: "loadout.primary", SlotId: "weapon",
			}}
		}))
	if ec := res.GetResult().GetErrorCode(); ec != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("unequip: %v", ec)
	}
	kind, _, _ = locationOf(t, weapon2)
	if kind != "CHARACTER_INVENTORY" {
		t.Fatalf("unequipped weapon2 -> %s", kind)
	}
	var contracted *string
	err := pool(t).QueryRow(ctx,
		`SELECT contracted_item_instance_id::text FROM character_souls WHERE soul_instance_id = $1`,
		soul.String()).Scan(&contracted)
	if err != nil {
		t.Fatalf("soul row: %v", err)
	}
	if contracted != nil {
		t.Fatalf("soul still contracted to %v", *contracted)
	}
	if len(res.GetSoulContracts()) != 1 {
		t.Fatalf("soul_contracts emission: %d", len(res.GetSoulContracts()))
	}
	if len(res.GetSoulContracts()[0].GetItemInstanceId()) != 0 {
		t.Fatal("unequipped soul must report empty item_instance_id (Collection)")
	}

	// SWITCH_ACTIVE: secondary_1 becomes ACTIVE without inventory moves.
	helm := mkItem(t, charID, "item.eq.head", 2)
	_ = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.secondary_1", SlotId: "head", ItemInstanceId: helm[:],
			}}
		}))
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SWITCH_ACTIVE, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_SwitchActive{SwitchActive: &protocolv1.LoadoutSwitchActive{
				LoadoutId: "loadout.secondary_1",
			}}
		}))
	if ec := res.GetResult().GetErrorCode(); ec != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("switch: %v", ec)
	}
	if res.GetActiveLoadoutId() != "loadout.secondary_1" {
		t.Fatalf("active = %s", res.GetActiveLoadoutId())
	}
	// Helm must still be EQUIPPED under secondary_1 (no inventory pass-through).
	kind, _, slot = locationOf(t, helm)
	if kind != "EQUIPPED" || slot != "loadout.secondary_1.head" {
		t.Fatalf("helm after switch -> %s %s", kind, slot)
	}
	// Exactly one ACTIVE row; both switched rows bumped.
	var actives int
	var revSum int64
	err = pool(t).QueryRow(ctx,
		`SELECT COUNT(*) FILTER (WHERE role='ACTIVE'), COALESCE(SUM(revision),0)
		 FROM character_loadouts WHERE character_id = $1`, charID.String()).Scan(&actives, &revSum)
	if err != nil {
		t.Fatalf("loadout roles: %v", err)
	}
	if actives != 1 {
		t.Fatalf("ACTIVE rows = %d", actives)
	}
	if int64(res.GetLoadoutRevision()) != revSum {
		t.Fatalf("403 revision %d != SUM %d", res.GetLoadoutRevision(), revSum)
	}
}

// TestUnregisteredKindsFailClosed: SKILL_SET and SOUL_CONTRACT are
// IMP-017/IMP-031 kinds — this packet registers only the three core
// kinds, so they reject deterministically from the 402 error set and
// commit no state.
func TestUnregisteredKindsFailClosed(t *testing.T) {
	acct := mkAccount(t)
	charID := mkCharacter(t, acct)
	d := newDeps(t)
	idem := newIdem(t)

	for _, tc := range []struct {
		kind protocolv1.LoadoutChangeKind
		want protocolv1.ErrorCode
	}{
		{protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SKILL_SET,
			protocolv1.ErrorCode_ERROR_CODE_SKILL_LOADOUT_INVALID},
		{protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SOUL_CONTRACT,
			protocolv1.ErrorCode_ERROR_CODE_SOUL_CONTRACT_LIMIT_REACHED},
	} {
		op := newOp()
		res := runChange(t, d, idem, acct, charID,
			&protocolv1.C2SLoadoutChange{OperationId: op[:], Kind: tc.kind})
		if got := res.GetResult().GetErrorCode(); got != tc.want {
			t.Fatalf("kind %v -> %v want %v", tc.kind, got, tc.want)
		}
	}
	var cnt int
	err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*) FROM character_loadouts WHERE character_id = $1`,
		charID.String()).Scan(&cnt)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// First-touch seeds the roster; rejects after that commit no revision.
	var revSum int64
	_ = pool(t).QueryRow(context.Background(),
		`SELECT COALESCE(SUM(revision),0) FROM character_loadouts WHERE character_id = $1`,
		charID.String()).Scan(&revSum)
	if revSum != 0 {
		t.Fatalf("rejections bumped revision: %d", revSum)
	}
}

// TestUnequipRequiresCapacity: UNEQUIP into a full inventory rejects
// INVENTORY_FULL and keeps the item equipped (all-or-nothing).
func TestUnequipRequiresCapacity(t *testing.T) {
	acct := mkAccount(t)
	charID := mkCharacter(t, acct)
	d := newDeps(t)
	idem := newIdem(t)

	// Fill inventory to capacity (60) minus none; equip one item.
	helm := mkItem(t, charID, "item.eq.head", 0)
	_ = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "head", ItemInstanceId: helm[:],
			}}
		}))
	// Occupy all 60 inventory slots.
	for i := 0; i < 60; i++ {
		mkItem(t, charID, "item.consumable.potion", i)
	}
	res := runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_UNEQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Unequip{Unequip: &protocolv1.LoadoutUnequip{
				LoadoutId: "loadout.primary", SlotId: "head",
			}}
		}))
	if got := res.GetResult().GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL {
		t.Fatalf("unequip full -> %v want INVENTORY_FULL", got)
	}
	kind, _, slot := locationOf(t, helm)
	if kind != "EQUIPPED" || slot != "loadout.primary.head" {
		t.Fatalf("failed unequip must leave item equipped, got %s %s", kind, slot)
	}
}

// TestEquipValidationRejects covers the durable-side deterministic
// rejects: slot mismatch, level gate, unknown item, unknown loadout.
func TestEquipValidationRejects(t *testing.T) {
	acct := mkAccount(t)
	charID := mkCharacter(t, acct)
	d := newDeps(t)
	idem := newIdem(t)

	// item.eq.head into slot weapon -> SLOT_MISMATCH.
	head := mkItem(t, charID, "item.eq.head", 0)
	res := runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "weapon", ItemInstanceId: head[:],
			}}
		}))
	if got := res.GetResult().GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_SLOT_MISMATCH {
		t.Fatalf("head->weapon = %v", got)
	}
	// level_min 20 head on level-10 char -> LEVEL_TOO_LOW.
	head20 := mkItem(t, charID, "item.eq.head.l20", 1)
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "head", ItemInstanceId: head20[:],
			}}
		}))
	if got := res.GetResult().GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_LEVEL_TOO_LOW {
		t.Fatalf("l20 head = %v", got)
	}
	// Unknown item instance -> ITEM_NOT_FOUND.
	ghost := id.NewV4()
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.primary", SlotId: "weapon", ItemInstanceId: ghost[:],
			}}
		}))
	if got := res.GetResult().GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND {
		t.Fatalf("ghost = %v", got)
	}
	// Unknown loadout -> deterministic reject (INVALID_STATE).
	res = runChange(t, d, idem, acct, charID,
		equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
			r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
				LoadoutId: "loadout.bogus", SlotId: "weapon", ItemInstanceId: head[:],
			}}
		}))
	if got := res.GetResult().GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT &&
		got != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("bogus loadout = %v", got)
	}
}

// TestReplayIdempotent: the same operation_id replays the recorded
// outcome without re-applying the mutation.
func TestReplayIdempotent(t *testing.T) {
	acct := mkAccount(t)
	charID := mkCharacter(t, acct)
	d := newDeps(t)
	idem := newIdem(t)

	weapon := mkItem(t, charID, "item.eq.weapon", 0)
	req := equipReq(protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP, func(r *protocolv1.C2SLoadoutChange) {
		r.Change = &protocolv1.C2SLoadoutChange_Equip{Equip: &protocolv1.LoadoutEquip{
			LoadoutId: "loadout.primary", SlotId: "weapon", ItemInstanceId: weapon[:],
		}}
	})
	res1 := runChange(t, d, idem, acct, charID, req)
	res2 := runChange(t, d, idem, acct, charID, req)
	if res1.GetLoadoutRevision() != res2.GetLoadoutRevision() {
		t.Fatalf("replay revision %d -> %d", res1.GetLoadoutRevision(), res2.GetLoadoutRevision())
	}
	kind, _, _ := locationOf(t, weapon)
	if kind != "EQUIPPED" {
		t.Fatalf("replay moved item: %s", kind)
	}
}

var _ = pgx.Tx(nil)
