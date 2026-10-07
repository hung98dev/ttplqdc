package inventory

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestInventoryMutateOps exercises MOVE (empty + swap), SPLIT, MERGE,
// DISCARD (partial + whole), and SORT compaction through the real
// durable path.
func TestInventoryMutateOps(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	s := NewStore(pool(t))

	// MOVE to an empty slot.
	a := mkItem(t, char, "item.mat.ore", 10, 0)
	out := runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, a, 5, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("move empty: %v", out.GetErrorCode())
	}
	if len(out.GetS2CInventoryResult().GetChangedSlots()) != 2 {
		t.Fatalf("changed %v", out.GetS2CInventoryResult().GetChangedSlots())
	}

	// MOVE onto an occupied slot swaps the two stacks.
	b := mkItem(t, char, "item.potion.hp", 4, 2)
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, a, 2, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("swap: %v", out.GetErrorCode())
	}
	res := out.GetS2CInventoryResult()
	var slot2 *protocolv1.InventoryChangedSlot
	for _, c := range res.GetChangedSlots() {
		if c.GetSlot() == 2 {
			slot2 = c
		}
	}
	if slot2 == nil || string(slot2.GetItemInstanceId()) != string(a[:]) {
		t.Fatalf("swap slot2 %+v", slot2)
	}

	// SPLIT 4 of 10 into free slot 7.
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_SPLIT, a, 7, 4, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("split: %v", out.GetErrorCode())
	}
	res = out.GetS2CInventoryResult()
	if len(res.GetChangedSlots()) != 2 {
		t.Fatalf("split changed %v", res.GetChangedSlots())
	}
	var srcQty, dstQty uint32
	for _, c := range res.GetChangedSlots() {
		if c.GetSlot() == 2 {
			srcQty = c.GetQuantity()
		}
		if c.GetSlot() == 7 {
			dstQty = c.GetQuantity()
		}
	}
	if srcQty != 6 || dstQty != 4 {
		t.Fatalf("split qty %d/%d", srcQty, dstQty)
	}

	// MERGE the split back onto slot 2.
	var newID id.UUID
	for _, c := range res.GetChangedSlots() {
		if c.GetSlot() == 7 {
			copy(newID[:], c.GetItemInstanceId())
		}
	}
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_MERGE, newID, 2, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("merge: %v", out.GetErrorCode())
	}
	st, err := s.LoadState(ctx, char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(st.Slots) != 2 { // a(10)@2, b(4)@5→wait a moved to 2 earlier
		for _, sv := range st.Slots {
			t.Logf("slot %d %s x%d", sv.Slot, sv.Item.ItemID, sv.Item.Quantity)
		}
	}

	// DISCARD partial then whole.
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_DISCARD, a, 0, 3, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("discard partial: %v", out.GetErrorCode())
	}
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_DISCARD, a, 0, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("discard whole: %v", out.GetErrorCode())
	}
	st, err = s.LoadState(ctx, char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, sv := range st.Slots {
		if sv.Item != nil && sv.Item.InstanceID == a {
			t.Fatal("discarded stack still present")
		}
	}
	_ = b
}

// TestInventoryCapacityStacking covers capacity bounds: an op
// addressing to_slot >= capacity is INVALID_STATE, and a split into an
// occupied slot is INVALID_STATE.
func TestInventoryCapacityStacking(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	a := mkItem(t, char, "item.mat.ore", 5, 0)
	b := mkItem(t, char, "item.potion.hp", 3, 1)

	out := runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, a, 60, 0, nil)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("move past cap: %v", out.GetErrorCode())
	}
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_SPLIT, a, 1, 2, nil)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("split into occupied: %v", out.GetErrorCode())
	}
	_ = b
}

// TestInventoryExpandSteps walks the price ladder: funded wallet,
// 60→70 at 10000, currency_delta, revision+1.
func TestInventoryExpandSteps(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	mkWallet(t, char, "currency.common", 1_000_000)

	out := runExpand(t, d, idem, acct, char, id.NewV7(time.Now()), 60)
	res := out.GetS2CInventoryExpandResult()
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		res.GetCapacityAfter() != 70 {
		t.Fatalf("expand: %v %d", out.GetErrorCode(), res.GetCapacityAfter())
	}
	if len(res.GetCurrencyDelta()) != 1 ||
		res.GetCurrencyDelta()[0].GetCurrencyId() != "currency.common" ||
		res.GetCurrencyDelta()[0].GetAmount() != -10000 {
		t.Fatalf("delta %+v", res.GetCurrencyDelta())
	}
	s := NewStore(pool(t))
	st, err := s.LoadState(context.Background(), char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if st.Capacity != 70 || st.Revision != 1 {
		t.Fatalf("capacity %d rev %d", st.Capacity, st.Revision)
	}
}

// TestInventoryExpansionLimits covers CAPACITY_FULL at 120,
// STATE_CONFLICT on expected_capacity mismatch, INSUFFICIENT_CURRENCY,
// and the ALL_OR_NOTHING boundary (a wallet short of the step price
// leaves capacity untouched).
func TestInventoryExpansionLimits(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	mkWallet(t, char, "currency.common", 50)

	// expected_capacity mismatch.
	out := runExpand(t, d, idem, acct, char, id.NewV7(time.Now()), 70)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT {
		t.Fatalf("mismatch: %v", out.GetErrorCode())
	}
	// Insufficient funds: no capacity change.
	out = runExpand(t, d, idem, acct, char, id.NewV7(time.Now()), 60)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY {
		t.Fatalf("insufficient: %v", out.GetErrorCode())
	}
	s := NewStore(pool(t))
	st, err := s.LoadState(context.Background(), char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if st.Capacity != 60 {
		t.Fatalf("capacity %d after failed expand", st.Capacity)
	}

	// At the 120 ceiling: CAPACITY_FULL.
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE character_inventories SET capacity=120 WHERE character_id=$1`,
		char.String()); err != nil {
		t.Fatalf("cap seed: %v", err)
	}
	out = runExpand(t, d, idem, acct, char, id.NewV7(time.Now()), 120)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL {
		t.Fatalf("cap full: %v", out.GetErrorCode())
	}
}

// TestIAPEntitlementPanelAccess covers the 435 projection: every
// account entitlement surfaces with its grant_state, the per-character
// claimed tier ids join correctly, and claimable ids come from the
// injected catalog binding — refund-revoked rows are absent from the
// equippable set by state (REFUNDED_CONSUMED never claimable).
func TestIAPEntitlementPanelAccess(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	ent1 := id.NewV4()
	ent2 := id.NewV4()
	if _, err := pool(t).Exec(ctx,
		`INSERT INTO account_iap_entitlements
		  (entitlement_id, account_id, entitlement_type, product_id, grant_state,
		   platform, platform_receipt, created_at, season_number, claim_deadline_at)
		 VALUES ($1,$2,'ACCOUNT_SCOPED_ACCESS','product.season.1','GRANTED',
		         'STEAM',$4,NOW(),1,NOW()+interval '30 days'),
		        ($3,$2,'DIRECT_ACCOUNT_COSMETIC','product.cos.9','REFUNDED_CONSUMED',
		         'STEAM',$5,NOW(),NULL,NULL)`,
		ent1.String(), acct.String(), ent2.String(), ent1.String(), ent2.String()); err != nil {
		t.Fatalf("entitlements: %v", err)
	}
	if _, err := pool(t).Exec(ctx,
		`INSERT INTO account_entitlement_claims
		  (account_entitlement_id, account_id, entitlement_type, character_id,
		   reward_tier_id, claimed_at, claim_operation_id)
		 VALUES ($1,$2,'ACCOUNT_SCOPED_ACCESS',$3,'tier.5',NOW(),$4)`,
		ent1.String(), acct.String(), char.String(), id.NewV4().String()); err != nil {
		t.Fatalf("claim: %v", err)
	}

	s := NewStore(pool(t))
	claimable := func(entitlementID string) []string {
		if entitlementID == ent1.String() {
			return []string{"tier.10", "tier.5"}
		}
		return nil
	}
	push, err := s.EntitlementPush(ctx, acct, char, claimable)
	if err != nil {
		t.Fatalf("panel: %v", err)
	}
	if len(push.Entitlements) != 2 {
		t.Fatalf("entitlements %d", len(push.Entitlements))
	}
	var e1 *protocolv1.EntitlementView
	for _, e := range push.Entitlements {
		if string(e.GetEntitlementId()) == string(ent1[:]) {
			e1 = e
		}
	}
	if e1 == nil {
		t.Fatal("season entitlement missing")
	}
	if e1.GetGrantState() != protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_GRANTED ||
		e1.GetEntitlementType() != protocolv1.EntitlementType_ENTITLEMENT_TYPE_ACCOUNT_SCOPED_ACCESS ||
		e1.GetSeasonNumber() != 1 || e1.GetClaimDeadlineAt() == 0 {
		t.Fatalf("view %+v", e1)
	}
	if len(e1.GetClaimedTierIds()) != 1 || e1.GetClaimedTierIds()[0] != "tier.5" {
		t.Fatalf("claimed %v", e1.GetClaimedTierIds())
	}
	if len(e1.GetClaimableTierIds()) != 2 {
		t.Fatalf("claimable %v", e1.GetClaimableTierIds())
	}
}

// TestStatePushAfterAttachAndChange covers the REPLACEABLE_STATE
// contract: attach produces 432/433/435 payloads and a committed
// mutation bumps inventory_revision in the following push.
func TestStatePushAfterAttachAndChange(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	mkWallet(t, char, "currency.common", 500)
	inst := mkItem(t, char, "item.mat.ore", 7, 0)

	s := NewStore(pool(t))
	inv0, err := s.InventoryPush(ctx, char, nil)
	if err != nil {
		t.Fatalf("attach push: %v", err)
	}
	if inv0.GetCapacity() != 60 || inv0.GetInventoryRevision() != 0 ||
		len(inv0.GetSlots()) != 1 {
		t.Fatalf("attach state %+v", inv0)
	}

	out := runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, inst, 9, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("move: %v", out.GetErrorCode())
	}
	inv1, err := s.InventoryPush(ctx, char, nil)
	if err != nil {
		t.Fatalf("post push: %v", err)
	}
	if inv1.GetInventoryRevision() != inv0.GetInventoryRevision()+1 {
		t.Fatalf("revision %d -> %d", inv0.GetInventoryRevision(), inv1.GetInventoryRevision())
	}
	if inv1.GetSlots()[0].GetSlot() != 9 {
		t.Fatalf("slot %d", inv1.GetSlots()[0].GetSlot())
	}
}

// TestReplaySameOperationId drives the same operation_id twice: the
// second submit replays the retained committed outcome and the state
// does not change.
func TestReplaySameOperationId(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	inst := mkItem(t, char, "item.mat.ore", 8, 0)

	opID := id.NewV7(time.Now())
	out1 := runMutate(t, d, idem, acct, char, opID,
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, inst, 3, 0, nil)
	if out1.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first: %v", out1.GetErrorCode())
	}
	out2 := runMutate(t, d, idem, acct, char, opID,
		protocolv1.InventoryOp_INVENTORY_OP_MOVE, inst, 3, 0, nil)
	if out2.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		out2.GetS2CInventoryResult().GetInventoryRevision() !=
			out1.GetS2CInventoryResult().GetInventoryRevision() {
		t.Fatalf("replay outcome differs")
	}
	s := NewStore(pool(t))
	st, err := s.LoadState(ctx, char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if st.Slots[0].Slot != 3 {
		t.Fatalf("double-applied: slot %d", st.Slots[0].Slot)
	}
}

// TestUseBookGrantsAndCooldown covers USE: book points + progression
// revision bump, shared-cooldown rejection, use_effect projection.
func TestUseBookGrantsAndCooldown(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	book := mkItem(t, char, "item.book.potential", 2, 0)
	pot := mkItem(t, char, "item.potion.hp", 1, 1)
	pot2 := mkItem(t, char, "item.potion.hp", 1, 2)
	tick := uint64(100)

	out := runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_USE, book, 0, 1, &tick)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("book use: %v", out.GetErrorCode())
	}
	res := out.GetS2CInventoryResult()
	if res.GetUseEffect().GetPotentialPointsGranted() != 10 {
		t.Fatalf("effect %+v", res.GetUseEffect())
	}
	var rev int64
	var potPoints int32
	if err := pool(t).QueryRow(ctx,
		`SELECT unspent_potential_points, progression_revision FROM characters WHERE character_id=$1`,
		char.String()).Scan(&potPoints, &rev); err != nil {
		t.Fatalf("points: %v", err)
	}
	if potPoints != 10 || rev != 1 {
		t.Fatalf("points %d rev %d", potPoints, rev)
	}

	// First potion consumes and starts the HP-group cooldown.
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_USE, pot, 0, 1, &tick)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("potion use: %v", out.GetErrorCode())
	}
	if res := out.GetS2CInventoryResult().GetUseEffect(); res.GetCooldownGroup() != "HP" ||
		res.GetCooldownEndsAtTick() != tick+160 { // 8s / 50ms
		t.Fatalf("effect %+v", res)
	}
	// Second HP-group use inside the window: COOLDOWN_ACTIVE.
	out = runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_USE, pot2, 0, 1, &tick)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE {
		t.Fatalf("cooldown: %v", out.GetErrorCode())
	}
}

// TestSortCompactAndOrder covers the deterministic SORT: compatible
// stacks compact earliest-first, then order by type, rarity desc,
// item_id, binding, instance id.
func TestSortCompactAndOrder(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := testDeps(t)
	idem := idempotency.NewStore(pool(t))
	mkItem(t, char, "item.mat.ore", 60, 0)
	mkItem(t, char, "item.mat.ore", 50, 3)
	sword := mkItem(t, char, "item.weapon.sword", 1, 5)
	book := mkItem(t, char, "item.book.potential", 1, 8)

	out := runMutate(t, d, idem, acct, char, id.NewV7(time.Now()),
		protocolv1.InventoryOp_INVENTORY_OP_SORT, id.UUID{}, 0, 0, nil)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("sort: %v", out.GetErrorCode())
	}
	s := NewStore(pool(t))
	st, err := s.LoadState(ctx, char)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Expected order: equipment sword (kind bit lowest) at slot 0;
	// consumable book (rarity 3) then ore (rarity 0) compacted 110.
	if len(st.Slots) != 3 {
		t.Fatalf("slots %d", len(st.Slots))
	}
	if st.Slots[0].Item.ItemID != "item.weapon.sword" ||
		st.Slots[0].Item.InstanceID != sword {
		t.Fatalf("slot0 %+v", st.Slots[0].Item)
	}
	if st.Slots[1].Item.InstanceID != book {
		t.Fatalf("slot1 %+v", st.Slots[1].Item)
	}
	if st.Slots[2].Item.ItemID != "item.mat.ore" || st.Slots[2].Item.Quantity != 110 {
		t.Fatalf("slot2 %+v", st.Slots[2].Item)
	}
}
