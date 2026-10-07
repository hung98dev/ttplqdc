package inventory

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// mutateExecutor applies one C2S_INVENTORY_MUTATE record inside
// Store.TrustedReplay under the receipt lock (ADR-0081). Deterministic
// rejections commit as JournalOutcome nonexecutions carrying the
// declared 401 in-set error as client_result; out-of-set failures leave
// client_result absent so the edge answers S2C_ERROR.
//
// Admission evidence never travels through ctx: the ADR-0083
// PartitionTick consult result is frozen into the record's
// spatial_source at admission and read here for cooldown_ends_at_tick.
func (d Deps) mutateExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != MutateFamily {
		return idempotency.Outcome{}, fmt.Errorf("inventory: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SInventoryMutate()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("inventory: record without c2s_inventory_mutate")
	}
	kind, ok := opKindOf(req.GetOp())
	if !ok {
		return d.mutateVerdict(opID, req.GetOp(), ErrInvalidState)
	}
	mop := MutateOp{Op: kind, ToSlot: req.GetToSlot(), Quantity: req.GetQuantity()}
	if kind != OpSort {
		if len(req.GetItemInstanceId()) != 16 {
			return d.mutateVerdict(opID, req.GetOp(), ErrInvalidState)
		}
		copy(mop.InstanceID[:], req.GetItemInstanceId())
	}
	if err := mop.validate(); err != nil {
		return d.mutateVerdict(opID, req.GetOp(), err)
	}

	// Owner-row lock set in canonical order: characters (20) before
	// character_inventories (40); item_instances/item_locations (50)
	// rows join the same sorted batch for single-instance ops.
	locks := []lockorder.Lock{
		lockorder.RowLock("characters", charID),
		lockorder.RowLock("character_inventories", charID),
	}
	if kind != OpSort {
		locks = append(locks,
			lockorder.RowLock("item_instances", mop.InstanceID),
			lockorder.RowLock("item_locations", mop.InstanceID))
	}
	if err := lockorder.SortLocks(locks); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return idempotency.Outcome{}, err
	}
	capacity, _, err := ensureInventoryRow(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	slots, err := slotViews(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	for i := range slots {
		slots[i].Item.origQty = slots[i].Item.Quantity
	}
	byInstance := map[id.UUID]*SlotView{}
	bySlot := map[uint32]*SlotView{}
	for i := range slots {
		s := &slots[i]
		if s.Item != nil {
			byInstance[s.Item.InstanceID] = s
			bySlot[s.Slot] = s
		}
	}
	ledger := d.lockLedger(charID)

	var (
		changed []*protocolv1.InventoryChangedSlot
		effect  *protocolv1.InventoryUseEffect
	)
	switch kind {
	case OpMove:
		changed, err = d.opMove(ctx, tx, charID, capacity, mop, byInstance, bySlot, ledger)
	case OpSplit:
		changed, err = d.opSplit(ctx, tx, charID, capacity, mop, byInstance, bySlot, ledger)
	case OpMerge:
		changed, err = d.opMerge(ctx, tx, charID, mop, byInstance, bySlot, ledger)
	case OpSort:
		changed, err = d.opSort(ctx, tx, charID, slots)
	case OpDiscard:
		changed, err = d.opDiscard(ctx, tx, charID, mop, byInstance, ledger)
	case OpUse:
		changed, effect, err = d.opUse(ctx, tx, cmd, charID, mop, byInstance, ledger)
	}
	if err != nil {
		if isSentinel(err) {
			return d.mutateVerdict(opID, req.GetOp(), err)
		}
		return idempotency.Outcome{}, err
	}

	rev, err := bumpRevision(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return mutateCommit(opID, req.GetOp(), rev, changed, effect)
}

// isSentinel reports whether err is a deterministic verdict sentinel
// (as opposed to an infrastructure failure).
func isSentinel(err error) bool {
	for _, s := range []error{ErrItemNotFound, ErrItemLocked, ErrFull,
		ErrCooldown, ErrInvalidState, ErrConflict, ErrInsufficientCurrency,
		ErrCapacityFull} {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}

// lockLedger resolves the character's live trade-lock ledger via the
// injected lookup (nil resolver → no locks; nil ledger → no locks).
func (d Deps) lockLedger(charID id.UUID) *items.TradeLockLedger {
	if d.Locks == nil {
		return nil
	}
	return d.Locks(charID)
}

// assertTrade applies the items.md § Trade Lock rules and maps the
// violation to the 401 in-set ITEM_LOCKED.
func assertTrade(ledger *items.TradeLockLedger, instanceID id.UUID,
	stackQty int32, op items.TradeOp, qty int32) error {
	if err := ledger.AssertOpAllowed(instanceID, int(stackQty), op, int(qty)); err != nil {
		if errors.Is(err, items.ErrTradeLocked) {
			return ErrItemLocked
		}
		return err
	}
	return nil
}

// opMove moves the instance's whole stack to ToSlot; an occupied target
// swaps the two stacks (messages.md §400).
func (d Deps) opMove(ctx context.Context, tx pgx.Tx, charID id.UUID,
	capacity int32, mop MutateOp, byInstance map[id.UUID]*SlotView,
	bySlot map[uint32]*SlotView, ledger *items.TradeLockLedger) ([]*protocolv1.InventoryChangedSlot, error) {
	src, ok := byInstance[mop.InstanceID]
	if !ok {
		return nil, ErrItemNotFound
	}
	if mop.ToSlot >= uint32(capacity) {
		return nil, fmt.Errorf("%w: to_slot %d >= capacity %d", ErrInvalidState, mop.ToSlot, capacity)
	}
	if mop.ToSlot == src.Slot {
		return nil, fmt.Errorf("%w: move to same slot", ErrInvalidState)
	}
	if err := assertTrade(ledger, mop.InstanceID, src.Item.Quantity,
		items.OpCustodyMove, src.Item.Quantity); err != nil {
		return nil, err
	}
	dst := bySlot[mop.ToSlot]
	if dst == nil || dst.Item == nil {
		// Empty destination: plain slot update on the location row.
		if err := moveSlotRow(ctx, tx, charID, mop.InstanceID, InvSlot(mop.ToSlot)); err != nil {
			return nil, err
		}
		return []*protocolv1.InventoryChangedSlot{
			changedSlot(src.Slot, nil),
			changedSlot(mop.ToSlot, src.Item),
		}, nil
	}
	if err := assertTrade(ledger, dst.Item.InstanceID, dst.Item.Quantity,
		items.OpCustodyMove, dst.Item.Quantity); err != nil {
		return nil, err
	}
	// Lock the counterparty rows before the swap: the executor's opening
	// batch only covered the source instance.
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("item_instances", dst.Item.InstanceID),
		lockorder.RowLock("item_locations", dst.Item.InstanceID)); err != nil {
		return nil, err
	}
	// Swap via a parked slot: the (character_id, location_kind, slot)
	// unique index forbids a direct two-statement exchange.
	const parked = "inv.swap"
	if _, err := tx.Exec(ctx,
		`UPDATE item_locations SET slot=$3
		  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY' AND slot=$2`,
		charID.String(), InvSlot(src.Slot), parked); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE item_locations SET slot=$3
		  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY' AND slot=$2`,
		charID.String(), InvSlot(dst.Slot), InvSlot(src.Slot)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE item_locations SET slot=$2
		  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY' AND slot=$3`,
		charID.String(), InvSlot(dst.Slot), parked); err != nil {
		return nil, err
	}
	return []*protocolv1.InventoryChangedSlot{
		changedSlot(src.Slot, dst.Item),
		changedSlot(dst.Slot, src.Item),
	}, nil
}

// opSplit carves Quantity units into the empty ToSlot.
func (d Deps) opSplit(ctx context.Context, tx pgx.Tx, charID id.UUID,
	capacity int32, mop MutateOp, byInstance map[id.UUID]*SlotView,
	bySlot map[uint32]*SlotView, ledger *items.TradeLockLedger) ([]*protocolv1.InventoryChangedSlot, error) {
	src, ok := byInstance[mop.InstanceID]
	if !ok {
		return nil, ErrItemNotFound
	}
	if mop.ToSlot >= uint32(capacity) {
		return nil, fmt.Errorf("%w: to_slot %d >= capacity %d", ErrInvalidState, mop.ToSlot, capacity)
	}
	if dst := bySlot[mop.ToSlot]; dst != nil && dst.Item != nil {
		return nil, fmt.Errorf("%w: split into occupied slot", ErrInvalidState)
	}
	if mop.Quantity >= uint32(src.Item.Quantity) {
		return nil, fmt.Errorf("%w: split %d of %d leaves nothing", ErrInvalidState, mop.Quantity, src.Item.Quantity)
	}
	newID, err := d.Items.SplitStack(ctx, tx, mop.InstanceID, int(mop.Quantity),
		InvSlot(mop.ToSlot), ledger)
	if err != nil {
		return nil, mapItemsErr(err)
	}
	remain := src.Item.Quantity - int32(mop.Quantity)
	return []*protocolv1.InventoryChangedSlot{
		changedSlot(src.Slot, &ItemRow{InstanceID: src.Item.InstanceID,
			ItemID: src.Item.ItemID, Quantity: remain}),
		changedSlot(mop.ToSlot, &ItemRow{InstanceID: newID,
			ItemID: src.Item.ItemID, Quantity: int32(mop.Quantity)}),
	}, nil
}

// opMerge pours the source stack into the stack occupying ToSlot
// (min(source, remaining) — items.MergeStacks).
func (d Deps) opMerge(ctx context.Context, tx pgx.Tx, charID id.UUID,
	mop MutateOp, byInstance map[id.UUID]*SlotView,
	bySlot map[uint32]*SlotView, ledger *items.TradeLockLedger) ([]*protocolv1.InventoryChangedSlot, error) {
	src, ok := byInstance[mop.InstanceID]
	if !ok {
		return nil, ErrItemNotFound
	}
	dst := bySlot[mop.ToSlot]
	if dst == nil || dst.Item == nil {
		return nil, fmt.Errorf("%w: merge into empty slot", ErrInvalidState)
	}
	if dst.Item.InstanceID == src.Item.InstanceID {
		return nil, fmt.Errorf("%w: merge onto self", ErrInvalidState)
	}
	def, err := d.defOf(ctx, src.Item.ItemID)
	if err != nil {
		return nil, err
	}
	if err := d.Items.MergeStacks(ctx, tx, dst.Item.InstanceID, mop.InstanceID, def.Def, ledger); err != nil {
		return nil, mapItemsErr(err)
	}
	// Post-merge quantities: re-read both instances; the source may be
	// deleted (full merge) or reduced.
	dstQty, err := instanceQty(ctx, tx, dst.Item.InstanceID)
	if err != nil {
		return nil, err
	}
	var srcView *ItemRow
	srcQty, err := instanceQty(ctx, tx, mop.InstanceID)
	switch {
	case err == nil && srcQty > 0:
		srcView = &ItemRow{InstanceID: src.Item.InstanceID,
			ItemID: src.Item.ItemID, Quantity: srcQty}
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	return []*protocolv1.InventoryChangedSlot{
		changedSlot(src.Slot, srcView),
		changedSlot(dst.Slot, &ItemRow{InstanceID: dst.Item.InstanceID,
			ItemID: dst.Item.ItemID, Quantity: dstQty}),
	}, nil
}

// opSort compacts compatible stacks (items.md: earliest slots fill
// first) then orders slots by type, rarity descending, item_id,
// binding, instance id. Every slot whose occupant or quantity changed
// is emitted in changed_slots.
func (d Deps) opSort(ctx context.Context, tx pgx.Tx, charID id.UUID,
	slots []SlotView) ([]*protocolv1.InventoryChangedSlot, error) {
	if len(slots) == 0 {
		return nil, nil
	}
	// Resolve defs once per item_id and pin the pre-sort quantity so the
	// changed-set can tell a compaction from a pure move.
	defCache := map[string]ItemDef{}
	for i := range slots {
		itemID := slots[i].Item.ItemID
		def, ok := defCache[itemID]
		if !ok {
			var err error
			def, err = d.defOf(ctx, itemID)
			if err != nil {
				return nil, err
			}
			defCache[itemID] = def
		}
		slots[i].Item.def = def
	}

	// Compaction: group compatible stacks (same item_id, binding,
	// item_state, content_revision — items.stacksCompatible contract),
	// each group pours later stacks into the earliest ones until full.
	groups := map[string][]*SlotView{}
	for i := range slots {
		it := slots[i].Item
		k := it.ItemID + "|" + it.EffectiveBinding + "|" +
			string(it.ItemState) + "|" + it.ContentRevision
		groups[k] = append(groups[k], &slots[i])
	}
	var absorbed []id.UUID
	for _, g := range groups {
		if len(g) < 2 || !g[0].Item.def.Def.Stackable {
			continue
		}
		cap := int32(g[0].Item.def.Def.MaxStack)
		if cap > items.MaxStackCeiling {
			cap = items.MaxStackCeiling
		}
		for i, j := 0, 1; i < len(g) && j < len(g); {
			if g[i].Item.Quantity >= cap {
				i++
				continue
			}
			move := g[j].Item.Quantity
			if room := cap - g[i].Item.Quantity; move > room {
				move = room
			}
			g[i].Item.Quantity += move
			g[j].Item.Quantity -= move
			if g[j].Item.Quantity == 0 {
				absorbed = append(absorbed, g[j].Item.InstanceID)
				g[j].Item = nil
				j++
			}
		}
	}
	for _, iid := range absorbed {
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_locations WHERE item_instance_id=$1`, iid.String()); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_instances WHERE item_instance_id=$1`, iid.String()); err != nil {
			return nil, err
		}
	}

	// Order: type asc, rarity desc, item_id asc, binding asc, iid asc.
	live := make([]*SlotView, 0, len(slots))
	for i := range slots {
		if slots[i].Item != nil {
			live = append(live, &slots[i])
		}
	}
	sort.SliceStable(live, func(a, b int) bool {
		ia, ib := live[a].Item, live[b].Item
		if ia.def.Def.Kind != ib.def.Def.Kind {
			return ia.def.Def.Kind < ib.def.Def.Kind
		}
		if ia.def.Rarity != ib.def.Rarity {
			return ia.def.Rarity > ib.def.Rarity
		}
		if ia.ItemID != ib.ItemID {
			return ia.ItemID < ib.ItemID
		}
		if ia.EffectiveBinding != ib.EffectiveBinding {
			return ia.EffectiveBinding < ib.EffectiveBinding
		}
		return ia.InstanceID.String() < ib.InstanceID.String()
	})

	// Write the permutation: live[i] lands on slot i. Relocating slots
	// pairwise collides on the (character_id, location_kind, slot) unique
	// index, so moved rows first park on a per-instance temp slot, then
	// take their final slot in a second pass.
	targets := map[uint32]bool{}
	var movedIDs []id.UUID
	for i, s := range live {
		if s.Slot != uint32(i) {
			movedIDs = append(movedIDs, s.Item.InstanceID)
		}
	}
	for pos, iid := range movedIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE item_locations SET slot=$2, updated_at=NOW()
			 WHERE item_instance_id=$1`, iid.String(), parkedSlot(pos)); err != nil {
			return nil, err
		}
	}
	var changed []*protocolv1.InventoryChangedSlot
	for i, s := range live {
		newSlot := uint32(i)
		targets[newSlot] = true
		item := s.Item
		moved := s.Slot != newSlot
		qtyChanged := item.Quantity != item.origQty
		if !moved && !qtyChanged {
			continue
		}
		if moved {
			if err := moveSlotRow(ctx, tx, charID, item.InstanceID, InvSlot(newSlot)); err != nil {
				return nil, err
			}
		}
		if qtyChanged {
			if err := setQty(ctx, tx, item.InstanceID, item.Quantity); err != nil {
				return nil, err
			}
		}
		changed = append(changed, changedSlot(newSlot, item))
	}
	for _, s := range slots {
		if s.Item == nil && !targets[s.Slot] {
			changed = append(changed, changedSlot(s.Slot, nil))
		}
	}
	return changed, nil
}

// opDiscard deletes the whole stack (quantity 0) or decrements it by
// quantity units (messages.md: "0 = whole stack"; discard_allowed only).
func (d Deps) opDiscard(ctx context.Context, tx pgx.Tx, charID id.UUID,
	mop MutateOp, byInstance map[id.UUID]*SlotView,
	ledger *items.TradeLockLedger) ([]*protocolv1.InventoryChangedSlot, error) {
	src, ok := byInstance[mop.InstanceID]
	if !ok {
		return nil, ErrItemNotFound
	}
	def, err := d.defOf(ctx, src.Item.ItemID)
	if err != nil {
		return nil, err
	}
	if mop.Quantity == 0 || mop.Quantity >= uint32(src.Item.Quantity) {
		// Whole-stack discard: the items primitive carries the full guard
		// (discard_allowed, location, soul-contract, trade lock).
		if err := d.Items.Discard(ctx, tx, mop.InstanceID, def.Def, ledger); err != nil {
			return nil, mapItemsErr(err)
		}
		return []*protocolv1.InventoryChangedSlot{changedSlot(src.Slot, nil)}, nil
	}
	if !def.Def.Discardable() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, items.ErrNotDiscardable)
	}
	if err := assertTrade(ledger, mop.InstanceID, src.Item.Quantity,
		items.OpReduce, int32(mop.Quantity)); err != nil {
		return nil, err
	}
	if contracted, err := soulContracted(ctx, tx, mop.InstanceID); err != nil {
		return nil, err
	} else if contracted {
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, items.ErrSoulContracted)
	}
	remain := src.Item.Quantity - int32(mop.Quantity)
	if err := setQty(ctx, tx, mop.InstanceID, remain); err != nil {
		return nil, err
	}
	return []*protocolv1.InventoryChangedSlot{
		changedSlot(src.Slot, &ItemRow{InstanceID: src.Item.InstanceID,
			ItemID: src.Item.ItemID, Quantity: remain}),
	}, nil
}

// opUse consumes exactly one unit (quantity=1) of a usable item:
// cooldown check against the shared tracker, point grants for the two
// book item_ids, then the use_effect projection.
func (d Deps) opUse(ctx context.Context, tx pgx.Tx, cmd *journalv1.JournalClientCommand,
	charID id.UUID, mop MutateOp, byInstance map[id.UUID]*SlotView,
	ledger *items.TradeLockLedger) ([]*protocolv1.InventoryChangedSlot, *protocolv1.InventoryUseEffect, error) {
	src, ok := byInstance[mop.InstanceID]
	if !ok {
		return nil, nil, ErrItemNotFound
	}
	def, err := d.defOf(ctx, src.Item.ItemID)
	if err != nil {
		return nil, nil, err
	}
	if def.Use == UseNone {
		return nil, nil, fmt.Errorf("%w: %s is not usable", ErrInvalidState, src.Item.ItemID)
	}
	if _, allowed := d.Cooldowns.AssertAllowed(charID, def.Def.SharedCooldownGroup); !allowed {
		return nil, nil, ErrCooldown
	}
	if err := assertTrade(ledger, mop.InstanceID, src.Item.Quantity,
		items.OpReduce, 1); err != nil {
		return nil, nil, err
	}
	if contracted, err := soulContracted(ctx, tx, mop.InstanceID); err != nil {
		return nil, nil, err
	} else if contracted {
		return nil, nil, fmt.Errorf("%w: %s", ErrInvalidState, items.ErrSoulContracted)
	}
	var pot, skill int32
	switch def.Use {
	case UseBookPotential:
		pot = 10
	case UseBookSkill:
		skill = 1
	}
	if pot != 0 || skill != 0 {
		if err := grantBookPoints(ctx, tx, charID, pot, skill); err != nil {
			return nil, nil, err
		}
	}

	// Consume one unit: decrement or delete the instance+location rows
	// (the row locks were already acquired for this instance).
	if src.Item.Quantity > 1 {
		if err := setQty(ctx, tx, mop.InstanceID, src.Item.Quantity-1); err != nil {
			return nil, nil, err
		}
	} else {
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_locations WHERE item_instance_id=$1`, mop.InstanceID.String()); err != nil {
			return nil, nil, err
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_instances WHERE item_instance_id=$1`, mop.InstanceID.String()); err != nil {
			return nil, nil, err
		}
	}
	d.Cooldowns.RecordUse(charID, def.Def.SharedCooldownGroup, cooldownFor(def))

	var chg *protocolv1.InventoryChangedSlot
	if src.Item.Quantity > 1 {
		chg = changedSlot(src.Slot, &ItemRow{InstanceID: src.Item.InstanceID,
			ItemID: src.Item.ItemID, Quantity: src.Item.Quantity - 1})
	} else {
		chg = changedSlot(src.Slot, nil)
	}
	effect := &protocolv1.InventoryUseEffect{
		CooldownGroup:          string(def.Def.SharedCooldownGroup),
		PotentialPointsGranted: uint32(pot),
		SkillPointsGranted:     uint32(skill),
	}
	// cooldown_ends_at_tick via the ADR-0083 consult tick frozen into the
	// record's spatial_source — never fabricated.
	if def.Def.SharedCooldownGroup != "" && def.Def.SharedCooldownGroup != items.CooldownNone {
		effect.CooldownEndsAtTick = d.Cooldowns.EndsAtTick(charID, def.Def.SharedCooldownGroup,
			cmd.GetSpatialSource().GetTick())
	}
	return []*protocolv1.InventoryChangedSlot{chg}, effect, nil
}

// defOf resolves one item_id through the injected Defs binding;
// an unresolved def is ErrItemNotFound (never an oracle into the
// catalog).
func (d Deps) defOf(ctx context.Context, itemID string) (ItemDef, error) {
	if d.Defs == nil {
		return ItemDef{}, ErrItemNotFound
	}
	def, err := d.Defs(ctx, itemID)
	if err != nil {
		return ItemDef{}, fmt.Errorf("inventory: def %q: %w", itemID, err)
	}
	return def, nil
}

// moveSlotRow rewrites the slot of an already-locked
// CHARACTER_INVENTORY location row.
func moveSlotRow(ctx context.Context, tx pgx.Tx, charID, instanceID id.UUID, slot string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE item_locations SET slot=$3
		  WHERE item_instance_id=$1 AND character_id=$2 AND location_kind='CHARACTER_INVENTORY'`,
		instanceID.String(), charID.String(), slot)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: instance %s not in inventory", ErrItemNotFound, instanceID)
	}
	return nil
}

func setQty(ctx context.Context, tx pgx.Tx, instanceID id.UUID, qty int32) error {
	tag, err := tx.Exec(ctx,
		`UPDATE item_instances SET quantity=$2 WHERE item_instance_id=$1`,
		instanceID.String(), qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: instance %s", ErrItemNotFound, instanceID)
	}
	return nil
}

func instanceQty(ctx context.Context, tx pgx.Tx, instanceID id.UUID) (int32, error) {
	var q int32
	err := tx.QueryRow(ctx,
		`SELECT quantity FROM item_instances WHERE item_instance_id=$1`,
		instanceID.String()).Scan(&q)
	return q, err
}

func soulContracted(ctx context.Context, tx pgx.Tx, instanceID id.UUID) (bool, error) {
	var yes bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_souls
		  WHERE contracted_item_instance_id=$1)`,
		instanceID.String()).Scan(&yes)
	return yes, err
}

// grantBookPoints applies the book point grant and bumps
// characters.progression_revision once — the committed-mutation rule
// for the 515 projection (data_model.md). A single UPDATE keeps the
// write-set minimal.
func grantBookPoints(ctx context.Context, tx pgx.Tx, charID id.UUID,
	potential, skill int32) error {
	tag, err := tx.Exec(ctx,
		`UPDATE characters SET
		   unspent_potential_points = unspent_potential_points + $2,
		   unspent_skill_points = unspent_skill_points + $3,
		   progression_revision = progression_revision + 1
		 WHERE character_id=$1`, charID.String(), potential, skill)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: character %s", ErrItemNotFound, charID)
	}
	return nil
}

func changedSlot(slot uint32, item *ItemRow) *protocolv1.InventoryChangedSlot {
	c := &protocolv1.InventoryChangedSlot{Slot: slot}
	if item != nil {
		c.ItemInstanceId = item.InstanceID[:]
		c.ItemId = item.ItemID
		c.Quantity = uint32(item.Quantity)
	}
	return c
}

// mapItemsErr translates items.Store sentinels into inventory verdict
// sentinels.
func mapItemsErr(err error) error {
	switch {
	case errors.Is(err, items.ErrUnknownItem), errors.Is(err, items.ErrInvalidLocation),
		errors.Is(err, items.ErrLocationConflict), errors.Is(err, items.ErrForbiddenOwner):
		return ErrItemNotFound
	case errors.Is(err, items.ErrTradeLocked):
		return ErrItemLocked
	case errors.Is(err, items.ErrCapacity), errors.Is(err, items.ErrSlotOccupied):
		return ErrFull
	case errors.Is(err, items.ErrIncompatibleStack), errors.Is(err, items.ErrNotDiscardable),
		errors.Is(err, items.ErrQuantity), errors.Is(err, items.ErrSoulContracted),
		errors.Is(err, items.ErrBindingBlocksTransfer):
		return ErrInvalidState
	}
	return err
}

// mutateVerdict writes the committed 401 nonexecution: in-set codes
// embed S2CInventoryResult; out-of-set codes leave client_result
// absent (edge answers S2C_ERROR with the same code).
func (d Deps) mutateVerdict(opID id.UUID, op protocolv1.InventoryOp, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
	}
	if mutateInSet(code) {
		outcome.ClientResult = &journalv1.JournalOutcome_S2CInventoryResult{
			S2CInventoryResult: &protocolv1.S2CInventoryResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				Op: op,
			},
		}
	}
	return marshalOutcome(outcome)
}

// mutateCommit writes the successful 401 outcome.
func mutateCommit(opID id.UUID, op protocolv1.InventoryOp, rev int64,
	changed []*protocolv1.InventoryChangedSlot,
	effect *protocolv1.InventoryUseEffect) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInventoryResult{
			S2CInventoryResult: &protocolv1.S2CInventoryResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				Op:                op,
				InventoryRevision: uint64(rev),
				ChangedSlots:      changed,
				UseEffect:         effect,
			},
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the JournalOutcome as protojson — the
// retained receipts/operations outcome representation
// (durable/character precedent).
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
