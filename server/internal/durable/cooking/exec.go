package cooking

import (
	"context"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/reward"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ItemDef is the runtime view of one item_id the executors need
// (mirrors inventory.ItemDef: Def custody fields bound at composition).
type ItemDef struct {
	Def items.Def
}

// Defs resolves an item_id to its runtime definition; nil is
// fail-closed.
type Defs func(ctx context.Context, itemID string) (ItemDef, error)

// Deps are the constructor-time dependencies of the cooking executors.
type Deps struct {
	Items           *items.Store
	Rewards         *reward.Store
	Prog            *progression.Store
	Defs            Defs
	Recipes         Recipes
	ContentRevision *string
}

// Executors returns the cooking client-family executor exports for the
// composition ProducerClient family-mux (mirrors durable/world):
//
//	interaction.kindle       -> S2C_INTERACT_RESULT (116)
//	interaction.cook         -> S2C_INTERACT_RESULT (116)
//	interaction.bonfire_rest -> S2C_INTERACT_RESULT (116)
func Executors(deps Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		FamilyKindle: deps.kindleExec,
		FamilyCook:   deps.cookExec,
		FamilyRest:   deps.restExec,
	}
}

// kindleExec commits the KINDLE consume: exactly one
// `item.material.cui_lua_trai` unit from the lowest-slot owned stack.
// The consult admission already proved the bonfire can accept kindling;
// the consume is the entire durable effect (world_rules.md § Kindling).
func (d Deps) kindleExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyKindle {
		return idempotency.Outcome{}, fmt.Errorf("cooking: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := consumeLowest(ctx, tx, charID, extraKindlingID, 1); err != nil {
		return idempotency.Outcome{}, err
	}
	opID := opUUID(rec)
	return verdictInteract(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_KINDLE,
		TargetId:     rec.GetClient().GetC2SInteract().GetTargetId(),
	})
}

// consumeLowest decrements qty from the lowest-slot owned stack of
// itemID (deterministic; the unit removed is the whole stack when
// quantities match).
func consumeLowest(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string, qty uint64) error {
	var instID id.UUID
	var have int64
	err := tx.QueryRow(ctx, `
		SELECT ii.item_instance_id, ii.quantity
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2
		ORDER BY il.slot
		LIMIT 1
		FOR UPDATE OF ii`, charID, itemID).Scan(&instID, &have)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInsufficientItems, itemID)
	}
	if have < int64(qty) {
		return fmt.Errorf("%w: %s", ErrInsufficientItems, itemID)
	}
	if have == int64(qty) {
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_locations WHERE item_instance_id = $1`, instID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`DELETE FROM item_instances WHERE item_instance_id = $1`, instID)
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
		instID, qty)
	return err
}

// cookExec commits the frozen COOK snapshot: revalidates the consumed
// instances under the transaction, applies the dish + extra_output
// grant (overflow routes to Reward Claims per inventory.md §24), and
// grants the authored LIFE_SKILL EXP under
// `life_skill.cook.<recipe_id>.<character_id>.<operation_id>` — never a
// CRAFT claim source (protobuf_conventions.md §7).
func (d Deps) cookExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyCook {
		return idempotency.Outcome{}, fmt.Errorf("cooking: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	snap := cmd.GetCraft()
	req := cmd.GetC2SInteract()
	if snap == nil || req == nil {
		return idempotency.Outcome{}, fmt.Errorf("cooking: COOK record without craft snapshot or request")
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	// Commit-side revalidation of the frozen consumed set.
	var consumed []*journalConsumed
	for _, c := range snap.GetConsumed() {
		consumed = append(consumed, &journalConsumed{
			itemInstanceID: c.GetItemInstanceId(),
			itemID:         c.GetItemId(),
			quantity:       c.GetQuantity(),
		})
	}
	if err := consumeFrozen(ctx, tx, charID, consumed); err != nil {
		opID := opUUID(rec)
		return verdictInteract(opID, &protocolv1.S2CInteractResult{
			Result: &protocolv1.OperationResult{
				OperationId: opID[:],
				Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
				ErrorCode:   consumeCode(err),
			},
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_COOK,
			TargetId:     req.GetTargetId(),
		})
	}
	// Grant every frozen output; unplaceable remainder routes to a
	// Reward Claims row (earned overflow — inventory.md §24).
	var granted []*protocolv1.ItemQuantity
	for _, it := range snap.GetCreatedItems() {
		def, err := d.Defs(ctx, it.GetItemId())
		if err != nil {
			return idempotency.Outcome{}, err
		}
		instID := it.GetItemInstanceId()
		if len(instID) != 16 {
			instID = nil
		}
		placed, err := d.grantItem(ctx, tx, charID, &journalv1ItemDef{
			itemID:     it.GetItemId(),
			quantity:   it.GetQuantity(),
			binding:    it.GetEffectiveBinding(),
			instanceID: instID,
			maxStack:   uint64(def.Def.MaxStack),
			capacity:   60,
			def:        def.Def,
		})
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if placed < it.GetQuantity() {
			if err := d.claimOverflow(ctx, tx, charID, rec, it, it.GetQuantity()-placed, len(granted)); err != nil {
				return idempotency.Outcome{}, err
			}
		}
		granted = append(granted, &protocolv1.ItemQuantity{
			ItemId:   it.GetItemId(),
			Quantity: uint32(it.GetQuantity()),
		})
	}
	// Authored LIFE_SKILL EXP under the cook identity, applied once per
	// operation (idempotency: record replay returns the stored outcome
	// before this executor reruns).
	if err := applyCharacterExp(ctx, tx, d.Prog, charID, snap.GetCharacterExp()); err != nil {
		return idempotency.Outcome{}, err
	}
	opID := opUUID(rec)
	return verdictInteract(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_COOK,
		TargetId:     req.GetTargetId(),
		Granted:      granted,
	})
}

// restExec records the admitted BONFIRE_REST interact — its only
// durable effect is the exactly-once 116 verdict; the rest session and
// tick/interval settlement intents live in sim/cooking.
func (d Deps) restExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyRest {
		return idempotency.Outcome{}, fmt.Errorf("cooking: unsupported client family %q", rec.GetOperationFamily())
	}
	opID := opUUID(rec)
	return verdictInteract(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_BONFIRE_REST,
		TargetId:     rec.GetClient().GetC2SInteract().GetTargetId(),
	})
}

// claimOverflow routes an unplaceable earned output to Reward Claims
// with the complete immutable item-creation payload (reward_claims.md
// § Delivery; ADR-0012). The source is the shared folk-gathering loop
// source FISHING (reward_claims.md enum — ADR-0024 covers the
// fishing/hearth loop; no cooking source exists in the enum).
func (d Deps) claimOverflow(ctx context.Context, tx pgx.Tx, charID id.UUID,
	rec *journalv1.DurableCommandRecord, it *journalv1.JournalItem,
	qty uint64, slotIndex int) error {
	opID := opUUID(rec)
	recipeID := rec.GetClient().GetC2SInteract().GetRecipeId()
	rev := it.GetContentRevision()
	if rev == "" && d.ContentRevision != nil {
		rev = *d.ContentRevision
	}
	in := &reward.Input{
		OwnerCharacterID:        charID,
		SourceType:              "FISHING",
		SourceReference:         "cooking_hearth." + recipeID + "." + it.GetItemId(),
		RewardSlot:              fmt.Sprintf("cook.%x.%d", opID[:], slotIndex),
		SourceRewardOperationID: opID,
		Lines: []reward.LineInput{{
			Kind:             "ITEM",
			ItemID:           it.GetItemId(),
			Quantity:         big.NewInt(int64(qty)),
			EffectiveBinding: it.GetEffectiveBinding(),
			ContentRevision:  rev,
		}},
	}
	if _, err := d.Rewards.Create(ctx, tx, in); err != nil {
		return err
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
		return id.UUID{}, fmt.Errorf("cooking: malformed record owner")
	}
	var c id.UUID
	copy(c[:], rec.GetOwnerId())
	return c, nil
}

func consumeCode(err error) protocolv1.ErrorCode {
	switch {
	case err == ErrInsufficientItems:
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	case err == ErrNotFound:
		return protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}

// verdictInteract packs one S2C_INTERACT_RESULT as the record's
// client_result outcome (mirrors durable/world.verdictInteract).
func verdictInteract(opID id.UUID, result *protocolv1.S2CInteractResult) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInteractResult{
			S2CInteractResult: result,
		},
	}
	raw, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{Payload: raw}, nil
}
