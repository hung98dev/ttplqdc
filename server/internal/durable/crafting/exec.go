package crafting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Client families (protobuf_conventions.md §7 / save_rules.md registry).
const (
	FamilyCraft   = "craft.create"
	FamilyEnhance = "enhance.apply"
)

// Executors returns the crafting client-family executor exports for the
// composition ProducerClient family-mux (mirrors durable/cooking):
//
//	craft.create   -> S2C_CRAFT_RESULT (405)
//	enhance.apply  -> S2C_ENHANCE_RESULT (407)
func Executors(deps Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		FamilyCraft:   deps.craftExec,
		FamilyEnhance: deps.enhanceExec,
	}
}

// craftExec commits the frozen JournalCraftSnapshot: revalidates every
// frozen consumed row (owner + quantity), debits the signed currency,
// then places every frozen created item at deterministic lowest free
// slots. Any stale selection fails the whole commit atomically — zero
// consumption, zero creation, never a synthetic CRAFT claim.
func (d Deps) craftExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyCraft {
		return idempotency.Outcome{}, fmt.Errorf("crafting: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	if cmd == nil || cmd.GetC2SCraft() == nil {
		return idempotency.Outcome{}, fmt.Errorf("crafting: %s record without C2SCraft", FamilyCraft)
	}
	snap := cmd.GetCraft()
	if snap == nil {
		return idempotency.Outcome{}, fmt.Errorf("crafting: %s record without craft snapshot", FamilyCraft)
	}
	req := cmd.GetC2SCraft()
	opID := opUUID(rec)

	fail := func(code protocolv1.ErrorCode) (idempotency.Outcome, error) {
		return d.craftVerdict(opID, req, snap, code, nil, nil)
	}

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := consumeFrozen(ctx, tx, charID, snap.GetConsumed()); err != nil {
		return fail(consumeCode(err))
	}
	// Signed currency deltas: negative rows debit, positive credit.
	for _, dc := range snap.GetCurrencyDelta() {
		if dc.GetAmount() < 0 {
			m := currency.Mutation{
				CharacterID: charID,
				CurrencyID:  currency.ID(dc.GetCurrencyId()),
				Delta:       -dc.GetAmount(),
				OperationID: opID,
				ReasonCode:  "CRAFT_COST",
				SourceRef:   fmt.Sprintf("craft.%s.%s", snap.GetRecipeId(), opID.String()),
				Actor:       currency.ActorSystem,
			}
			if _, err := currency.Debit(ctx, tx, m); err != nil {
				return fail(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY)
			}
		}
	}
	var granted []*protocolv1.ItemGrant
	var createdIDs []*journalv1.JournalCreatedId
	for _, it := range snap.GetCreatedItems() {
		def, err := d.Defs(ctx, it.GetItemId())
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if err := d.createItem(ctx, tx, charID, it, def); err != nil {
			if err == ErrInventoryFull {
				return fail(protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL)
			}
			return idempotency.Outcome{}, err
		}
		granted = append(granted, &protocolv1.ItemGrant{
			ItemInstanceId: it.GetItemInstanceId(),
			ItemId:         it.GetItemId(),
			Quantity:       uint32(it.GetQuantity()),
		})
		var cid string
		if it.GetContentRevision() != "" {
			s := it.GetContentRevision()
			cid = s
		}
		createdIDs = append(createdIDs, &journalv1.JournalCreatedId{
			Kind:      "item_instance",
			Id:        it.GetItemInstanceId(),
			ContentId: &cid,
		})
	}
	return d.craftVerdict(opID, req, snap, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, granted, createdIDs)
}

// enhanceExec commits the frozen JournalEnhanceResult: consume the
// frozen items (materials lowest-slot + charm instances), debit the
// frozen currency, mutate the instance's enhancement_level and write
// the pity record — atomically. The roll is never re-derived here: the
// admitted result replays verbatim.
func (d Deps) enhanceExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyEnhance {
		return idempotency.Outcome{}, fmt.Errorf("crafting: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	if cmd == nil || cmd.GetC2SEnhance() == nil {
		return idempotency.Outcome{}, fmt.Errorf("crafting: %s record without C2SEnhance", FamilyEnhance)
	}
	frozen := cmd.GetRngOutputs()
	if frozen == nil {
		return idempotency.Outcome{}, fmt.Errorf("crafting: %s record without rng_outputs", FamilyEnhance)
	}
	req := cmd.GetC2SEnhance()
	opID := opUUID(rec)
	var itemID id.UUID
	if len(req.GetItemInstanceId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("crafting: item_instance_id %d bytes", len(req.GetItemInstanceId()))
	}
	copy(itemID[:], req.GetItemInstanceId())

	fail := func(code protocolv1.ErrorCode) (idempotency.Outcome, error) {
		return d.enhanceVerdict(opID, req, frozen, code)
	}

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	// Revalidate the target instance is still owned at the frozen level.
	var curEnh int64
	var itemItemID string
	var owner id.UUID
	err = tx.QueryRow(ctx, `
		SELECT ii.enhancement_level, ii.item_id, il.character_id
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE ii.item_instance_id = $1
		  AND il.location_kind IN ('CHARACTER_INVENTORY', 'EQUIPPED')
		FOR UPDATE OF ii`, itemID).Scan(&curEnh, &itemItemID, &owner)
	if err != nil {
		return fail(protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND)
	}
	if owner != charID || curEnh != int64(frozen.GetLevelBefore()) {
		return fail(protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}
	// Consume the frozen quantities: material/charm items re-select
	// lowest-slot inside the locked inventory (the journal records the
	// item_id+qty deltas; instance selection is deterministic).
	for _, c := range frozen.GetConsumed() {
		if err := consumeLowest(ctx, tx, charID, c.GetItemId(), int64(c.GetQuantity())); err != nil {
			return fail(consumeCode(err))
		}
	}
	for _, dc := range frozen.GetCurrencyDelta() {
		if dc.GetAmount() < 0 {
			m := currency.Mutation{
				CharacterID: charID,
				CurrencyID:  currency.ID(dc.GetCurrencyId()),
				Delta:       -dc.GetAmount(),
				OperationID: opID,
				ReasonCode:  "ENHANCE_COST",
				SourceRef:   fmt.Sprintf("enhance.%s.%s", itemID.String(), opID.String()),
				Actor:       currency.ActorSystem,
			}
			if _, err := currency.Debit(ctx, tx, m); err != nil {
				return fail(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY)
			}
		}
	}
	// Apply the frozen level + pity record.
	if _, err := tx.Exec(ctx,
		`UPDATE item_instances SET enhancement_level = $2 WHERE item_instance_id = $1`,
		itemID, frozen.GetLevelAfter()); err != nil {
		return idempotency.Outcome{}, err
	}
	if int64(req.GetTargetLevel()) >= pityStartTarget {
		if err := pityWrite(ctx, tx, itemID, int64(req.GetTargetLevel()),
			int64(frozen.GetPityFailCount())); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	return d.enhanceVerdict(opID, req, frozen, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED)
}

// createItem places one frozen created item at the lowest free
// inventory slot; capacity was pre-validated at admission and is
// re-derived here — overflow rejects INVENTORY_FULL (craft never emits
// a CRAFT Reward Claim: crafting.md § Atomic Craft).
func (d Deps) createItem(ctx context.Context, tx pgx.Tx, charID id.UUID,
	it *journalv1.JournalItem, def ItemDef) error {
	var instID id.UUID
	copy(instID[:], it.GetItemInstanceId())
	state, err := itemStateJSON(it)
	if err != nil {
		return err
	}
	if def.Def.Stackable && def.Def.MaxStack > 1 {
		// Top up compatible stacks first, then a fresh stack.
		need := int64(it.GetQuantity())
		rows, err := tx.Query(ctx, `
			SELECT ii.item_instance_id, ii.quantity
			FROM item_instances ii
			JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
			WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
			  AND ii.item_id = $2 AND ii.effective_binding = $3 AND ii.quantity < $4
			ORDER BY il.slot`, charID, it.GetItemId(), it.GetEffectiveBinding(), def.Def.MaxStack)
		if err != nil {
			return err
		}
		var tops []struct {
			instID id.UUID
			qty    int64
		}
		for rows.Next() {
			var iid id.UUID
			var q int64
			if err := rows.Scan(&iid, &q); err != nil {
				rows.Close()
				return err
			}
			tops = append(tops, struct {
				instID id.UUID
				qty    int64
			}{iid, q})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, tp := range tops {
			if need == 0 {
				break
			}
			room := int64(def.Def.MaxStack) - tp.qty
			add := min(need, room)
			if _, err := tx.Exec(ctx,
				`UPDATE item_instances SET quantity = quantity + $2 WHERE item_instance_id = $1`,
				tp.instID, add); err != nil {
				return err
			}
			need -= add
		}
		if need == 0 {
			return nil
		}
		qty := need
		slot, err := lowestFreeSlot(ctx, tx, charID)
		if err != nil {
			return err
		}
		if slot < 0 {
			return ErrInventoryFull
		}
		fields := items.CreateFields{
			InstanceID: instID,
			Quantity:   int(qty),
			ItemState:  state,
		}
		if it.GetCreatedAtMs() != 0 {
			fields.CreatedAt = time.UnixMilli(it.GetCreatedAtMs())
		}
		if it.GetContentRevision() != "" {
			rev := it.GetContentRevision()
			fields.ContentRevision = &rev
		}
		if _, err := d.Items.Create(ctx, tx, def.Def, fields, items.Location{
			Kind:        items.LocCharacterInventory,
			CharacterID: charID,
			Slot:        invSlot(slot),
		}); err != nil {
			return err
		}
		return nil
	}
	slot, err := lowestFreeSlot(ctx, tx, charID)
	if err != nil {
		return err
	}
	if slot < 0 {
		return ErrInventoryFull
	}
	fields := items.CreateFields{
		InstanceID:       instID,
		Quantity:         int(it.GetQuantity()),
		EnhancementLevel: int16(it.GetEnhancement()),
		ItemState:        state,
	}
	if it.GetCreatedAtMs() != 0 {
		fields.CreatedAt = time.UnixMilli(it.GetCreatedAtMs())
	}
	if it.GetContentRevision() != "" {
		rev := it.GetContentRevision()
		fields.ContentRevision = &rev
	}
	if _, err := d.Items.Create(ctx, tx, def.Def, fields, items.Location{
		Kind:        items.LocCharacterInventory,
		CharacterID: charID,
		Slot:        invSlot(slot),
	}); err != nil {
		return err
	}
	return nil
}

// itemStateJSON serializes the frozen rolls into the schema-versioned
// item_state JSONB (data_model.md § item_instances: immutable finalized
// rolls/enhancement/provenance, ADR-0012).
func itemStateJSON(it *journalv1.JournalItem) ([]byte, error) {
	if len(it.GetBaseRolls()) == 0 && len(it.GetSecondaryRolls()) == 0 {
		return nil, nil
	}
	type statJSON struct {
		StatID string `json:"stat_id"`
		Value  int64  `json:"value"`
		Scale  uint32 `json:"scale"`
	}
	pack := func(in []*journalv1.JournalStat) []statJSON {
		out := make([]statJSON, 0, len(in))
		for _, s := range in {
			out = append(out, statJSON{StatID: s.GetStatId(), Value: s.GetValue(), Scale: s.GetScale()})
		}
		return out
	}
	return json.Marshal(map[string]any{
		"v":               1,
		"base_rolls":      pack(it.GetBaseRolls()),
		"secondary_rolls": pack(it.GetSecondaryRolls()),
	})
}

// lowestFreeSlot returns the smallest unused inv.<N> index within the
// character's inventory capacity, or -1 when full.
func lowestFreeSlot(ctx context.Context, tx pgx.Tx, charID id.UUID) (int64, error) {
	var cap2 int64
	if err := tx.QueryRow(ctx,
		`SELECT capacity FROM character_inventories WHERE character_id = $1`,
		charID).Scan(&cap2); err != nil {
		return -1, err
	}
	rows, err := tx.Query(ctx, `
		SELECT slot FROM item_locations
		WHERE character_id = $1 AND location_kind = 'CHARACTER_INVENTORY'`, charID)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	taken := map[int64]struct{}{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return -1, err
		}
		var n int64
		if _, err := fmt.Sscanf(s, "inv.%d", &n); err == nil {
			taken[n] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return -1, err
	}
	for n := int64(0); n < cap2; n++ {
		if _, ok := taken[n]; !ok {
			return n, nil
		}
	}
	return -1, nil
}

func invSlot(n int64) string { return fmt.Sprintf("inv.%d", n) }

// consumeFrozen decrements the exact frozen instance selection
// (mirrors cooking.grant.go): each row loses its recorded quantity and
// is deleted at zero; a missing/short/differently-owned instance fails
// the commit atomically.
func consumeFrozen(ctx context.Context, tx pgx.Tx, charID id.UUID,
	sel []*journalv1.JournalConsumedItem) error {
	for _, c := range sel {
		var instID id.UUID
		copy(instID[:], c.GetItemInstanceId())
		var qty int64
		var owner id.UUID
		err := tx.QueryRow(ctx, `
			SELECT ii.quantity, il.character_id
			FROM item_instances ii
			JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
			WHERE ii.item_instance_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
			FOR UPDATE OF ii`, instID).Scan(&qty, &owner)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotFound, instID)
		}
		if owner != charID || qty < int64(c.GetQuantity()) {
			return fmt.Errorf("%w: %s", ErrInsufficientItems, instID)
		}
		if qty == int64(c.GetQuantity()) {
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_locations WHERE item_instance_id = $1`, instID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_instances WHERE item_instance_id = $1`, instID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
			instID, c.GetQuantity()); err != nil {
			return err
		}
	}
	return nil
}

// consumeLowest decrements qty across the lowest-slot owned stacks of
// itemID (deterministic ORDER BY slot); insufficient fails the commit.
func consumeLowest(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string, qty int64) error {
	rows, err := tx.Query(ctx, `
		SELECT ii.item_instance_id, ii.quantity
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2
		ORDER BY il.slot
		FOR UPDATE OF ii`, charID, itemID)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInsufficientItems, itemID)
	}
	var need = qty
	var picks []struct {
		instID id.UUID
		have   int64
	}
	for rows.Next() && need > 0 {
		var instID id.UUID
		var have int64
		if err := rows.Scan(&instID, &have); err != nil {
			rows.Close()
			return err
		}
		picks = append(picks, struct {
			instID id.UUID
			have   int64
		}{instID, have})
		need -= min(have, need)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if need > 0 {
		return fmt.Errorf("%w: %s", ErrInsufficientItems, itemID)
	}
	need = qty
	for _, pk := range picks {
		take := min(pk.have, need)
		if take == pk.have {
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_locations WHERE item_instance_id = $1`, pk.instID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_instances WHERE item_instance_id = $1`, pk.instID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx,
				`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
				pk.instID, take); err != nil {
				return err
			}
		}
		need -= take
	}
	return nil
}

func opUUID(rec *journalv1.DurableCommandRecord) id.UUID {
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	return opID
}

func ownerCharacter(rec *journalv1.DurableCommandRecord) (id.UUID, error) {
	if len(rec.GetOwnerId()) != 16 ||
		rec.GetOwnerKind() != journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER {
		return id.UUID{}, fmt.Errorf("crafting: malformed record owner")
	}
	var c id.UUID
	copy(c[:], rec.GetOwnerId())
	return c, nil
}

// consumeCode maps a settlement consume failure to its wire code.
func consumeCode(err error) protocolv1.ErrorCode {
	switch {
	case err == ErrInsufficientItems:
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_ITEM
	case err == ErrInventoryFull:
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL
	case err == ErrNotFound:
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}

// craftVerdict packs one S2C_CRAFT_RESULT as the record outcome; the
// craft snapshot lands on the outcome's `craft` field on success.
func (d Deps) craftVerdict(opID id.UUID, req *protocolv1.C2SCraft,
	snap *journalv1.JournalCraftSnapshot, code protocolv1.ErrorCode,
	granted []*protocolv1.ItemGrant, createdIDs []*journalv1.JournalCreatedId) (idempotency.Outcome, error) {
	status := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	var consumed []*protocolv1.ItemQuantity
	for _, c := range snap.GetConsumed() {
		consumed = append(consumed, itemQty(c.GetItemId(), int64(c.GetQuantity())))
	}
	var deltas []*protocolv1.CurrencyDelta
	for _, dc := range snap.GetCurrencyDelta() {
		deltas = append(deltas, &protocolv1.CurrencyDelta{
			CurrencyId: dc.GetCurrencyId(),
			Amount:     dc.GetAmount(),
		})
	}
	outcome := &journalv1.JournalOutcome{
		Status:      status,
		ErrorCode:   code,
		OperationId: opID[:],
		CreatedIds:  createdIDs,
		ClientResult: &journalv1.JournalOutcome_S2CCraftResult{
			S2CCraftResult: &protocolv1.S2CCraftResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      status,
					ErrorCode:   code,
				},
				RecipeId:      req.GetRecipeId(),
				BatchQuantity: req.GetBatchQuantity(),
				Consumed:      consumed,
				CurrencyDelta: deltas,
				Granted:       granted,
			},
		},
	}
	if status == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		outcome.Craft = snap
	}
	raw, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{Payload: raw}, nil
}

// enhanceVerdict packs one S2C_ENHANCE_RESULT as the record outcome;
// the enhancement result lands on the outcome's `enhancement` field.
func (d Deps) enhanceVerdict(opID id.UUID, req *protocolv1.C2SEnhance,
	frozen *journalv1.JournalEnhanceResult, code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	status := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	var deltas []*protocolv1.CurrencyDelta
	for _, dc := range frozen.GetCurrencyDelta() {
		deltas = append(deltas, &protocolv1.CurrencyDelta{
			CurrencyId: dc.GetCurrencyId(),
			Amount:     dc.GetAmount(),
		})
	}
	outcome := &journalv1.JournalOutcome{
		Status:      status,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CEnhanceResult{
			S2CEnhanceResult: &protocolv1.S2CEnhanceResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      status,
					ErrorCode:   code,
				},
				ItemInstanceId: req.GetItemInstanceId(),
				Success:        frozen.GetSuccess() && status == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				LevelBefore:    frozen.GetLevelBefore(),
				LevelAfter:     frozen.GetLevelAfter(),
				FinalRateBp:    frozen.GetFinalRateBp(),
				PityFailCount:  frozen.GetPityFailCount(),
				Consumed:       frozen.GetConsumed(),
				CurrencyDelta:  deltas,
			},
		},
	}
	if status == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		outcome.Enhancement = frozen
	}
	raw, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{Payload: raw}, nil
}
