package fishing

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

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

// dailyCap is the 50-success UTC-day ceiling (world_rules.md § Folk
// Fishing: "maximum is 50").
const dailyCap = 50

// ItemDef is the runtime view of one item_id the executors need
// (mirrors durable/cooking: Def custody fields bound at composition).
type ItemDef struct {
	Def items.Def
}

// Defs resolves an item_id to its runtime definition; nil is
// fail-closed.
type Defs func(ctx context.Context, itemID string) (ItemDef, error)

// Deps are the constructor-time dependencies of the fishing executors.
type Deps struct {
	Items           *items.Store
	Rewards         *reward.Store
	Prog            *progression.Store
	Defs            Defs
	ContentRevision *string
	// SeasonIndex resolves the featured season number at admission
	// (seasons 3/4 have no seasonal catch table -> default).
	SeasonIndex func() int
	// DiTich reports whether the map's `buff.di_tich.*` rare-fish buff
	// is active for the spot (bosses.md).
	DiTich func(spotID string) bool
	// Now supplies the UTC clock; bound to wall time at composition.
	Now func() time.Time
}

// Executors returns the fishing client-family executor exports for the
// composition ProducerClient family-mux (mirrors durable/cooking):
//
//	interaction.cast -> S2C_INTERACT_RESULT (116)
//	interaction.hook -> S2C_INTERACT_RESULT (116)
func Executors(deps Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		FamilyCast: deps.castExec,
		FamilyHook: deps.hookExec,
	}
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now().UTC()
}

// castExec commits the CAST consume: after rod, bait and the 50/day
// cap revalidate under the character lock, exactly one
// `item.consumable.moi_cau` unit leaves the lowest-slot owned stack —
// never before every gate passes (world_rules.md § Folk Fishing). The
// durable cast sequence is the receipt ordinal this transaction
// reserves for the hook's roll key.
func (d Deps) castExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyCast {
		return idempotency.Outcome{}, fmt.Errorf("fishing: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SInteract()
	opID := opUUID(rec)
	fail := func(code protocolv1.ErrorCode) (idempotency.Outcome, error) {
		return verdictInteract(opID, interactErr(opID, code,
			protocolv1.InteractKind_INTERACT_KIND_CAST, req.GetTargetId()))
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	day := d.utcDay()
	// Spot legality and the daily cap are durable-state gates.
	if _, ok := TableForSpot(req.GetTargetId(), d.season(), d.diTich(req.GetTargetId())); !ok {
		return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	count, err := d.catchCount(ctx, tx, charID, day)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if count >= dailyCap {
		return fail(protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}
	if has, err := d.hasItem(ctx, tx, charID, RodItemID); err != nil {
		return idempotency.Outcome{}, err
	} else if !has {
		return fail(protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND)
	}
	if err := d.consumeLowest(ctx, tx, charID, BaitItemID, 1); err != nil {
		return fail(consumeCode(err))
	}
	return verdictInteract(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_CAST,
		TargetId:     req.GetTargetId(),
	})
}

// hookExec commits the HOOK settlement: derives the accepted cast's
// durable sequence (the receipt ordinal of the latest cast admitted
// before this hook on its UTC day), rolls the spot's catch table once
// on the keyed PCG-64 stream, increments the daily counter atomically
// with the grant, grants the catch (overflow -> Reward Claims) and
// applies the authored per-act LIFE_SKILL EXP. Replay returns the
// recorded outcome — never allocates another sequence or rerolls.
func (d Deps) hookExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyHook {
		return idempotency.Outcome{}, fmt.Errorf("fishing: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, err := ownerCharacter(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SInteract()
	opID := opUUID(rec)
	fail := func(code protocolv1.ErrorCode) (idempotency.Outcome, error) {
		return verdictInteract(opID, interactErr(opID, code,
			protocolv1.InteractKind_INTERACT_KIND_HOOK, req.GetTargetId()))
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	spotID := req.GetTargetId()
	table, ok := TableForSpot(spotID, d.season(), d.diTich(spotID))
	if !ok {
		return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	// The accepted cast's durable sequence: the ordinal of the latest
	// interaction.cast receipt admitted before this hook, counted on
	// the cast's own UTC day (reward_claims.md § Fishing identity).
	castDate, seq, err := d.castSequence(ctx, tx, charID, rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if seq == 0 {
		return fail(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
	}
	// Daily counter increments atomically with settlement; a caught
	// 51st is rejected inside the transaction (never granted).
	ok2, err := d.bumpCatchCount(ctx, tx, charID, castDate)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !ok2 {
		return fail(protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}
	// One PCG-64 roll on the earned identity.
	catchID := Roll(table, rollKey(charID, castDate, seq))
	def, err := d.Defs(ctx, catchID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	inst := id.NewV4()
	placed, err := d.grantItem(ctx, tx, charID, &journalv1ItemDef{
		itemID:     catchID,
		quantity:   1,
		binding:    def.Def.DefaultBinding.String(),
		instanceID: inst[:],
		maxStack:   uint64(def.Def.MaxStack),
		capacity:   60,
		def:        def.Def,
	})
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if placed < 1 {
		if err := d.claimOverflow(ctx, tx, charID, rec, catchID, def,
			spotID, castDate, seq); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	// Authored LIFE_SKILL EXP for the character's captured act.
	act, err := characterAct(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := applyCharacterExp(ctx, tx, d.Prog, charID, expForAct(act)); err != nil {
		return idempotency.Outcome{}, err
	}
	return verdictInteract(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_HOOK,
		TargetId:     spotID,
		Granted: []*protocolv1.ItemQuantity{
			{ItemId: catchID, Quantity: 1},
		},
	})
}

// utcDay renders the settlement day (UTC) for the counter and keys.
func (d Deps) utcDay() string {
	return d.now().UTC().Format("2006-01-02")
}

func (d Deps) season() int {
	if d.SeasonIndex != nil {
		return d.SeasonIndex()
	}
	return -1 // no featured season -> default table
}

func (d Deps) diTich(spotID string) bool {
	return d.DiTich != nil && d.DiTich(spotID)
}

// castSequence resolves the latest interaction.cast receipt admitted
// at-or-before this hook's own admission and its per-day ordinal.
// ok=false / seq=0 when no cast precedes the hook today.
func (d Deps) castSequence(ctx context.Context, tx pgx.Tx, charID id.UUID,
	rec *journalv1.DurableCommandRecord) (string, uint64, error) {
	var castAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT r.admitted_at FROM durable_command_receipts r
		WHERE r.operation_family = $1 AND r.owner_id = $2
		  AND r.admitted_at <= (
		    SELECT admitted_at FROM durable_command_receipts
		    WHERE operation_family = $3 AND owner_id = $2
		      AND operation_id = $4)
		ORDER BY r.admitted_at DESC LIMIT 1`,
		FamilyCast, charID.String(), FamilyHook, opUUID(rec).String(),
	).Scan(&castAt)
	if err != nil {
		return "", 0, nil // no cast row -> seq 0
	}
	day := castAt.UTC().Format("2006-01-02")
	var seq uint64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM durable_command_receipts
		WHERE operation_family = $1 AND owner_id = $2
		  AND (admitted_at AT TIME ZONE 'UTC')::date = $3
		  AND admitted_at <= $4`,
		FamilyCast, charID.String(), day, castAt).Scan(&seq); err != nil {
		return "", 0, err
	}
	return day, seq, nil
}

// catchCount reads today's success counter (0 when the stored date is
// stale — the day rolls at settlement).
func (d Deps) catchCount(ctx context.Context, tx pgx.Tx, charID id.UUID,
	day string) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(fishing_catch_count, 0) FROM characters
		WHERE character_id = $1 AND fishing_utc_date = $2`,
		charID.String(), day).Scan(&count); err != nil {
		return 0, nil // no row for today -> 0
	}
	return count, nil
}

// bumpCatchCount increments the day's success counter atomically under
// the cap: rolls the date forward when stale, rejects the 51st.
func (d Deps) bumpCatchCount(ctx context.Context, tx pgx.Tx, charID id.UUID,
	day string) (bool, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE characters SET
		  fishing_utc_date = $2,
		  fishing_catch_count = CASE
		    WHEN fishing_utc_date IS NULL OR fishing_utc_date <> $2 THEN 1
		    ELSE fishing_catch_count + 1 END
		WHERE character_id = $1
		  AND (fishing_utc_date IS NULL OR fishing_utc_date <> $2
		       OR fishing_catch_count < $3)`,
		charID.String(), day, dailyCap)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// hasItem reports whether the character owns at least one unit of
// itemID in CHARACTER_INVENTORY.
func (d Deps) hasItem(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string) (bool, error) {
	var n int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2`, charID.String(), itemID).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// consumeLowest decrements qty from the lowest-slot owned stack of
// itemID (deterministic; mirrors durable/cooking's consume).
func (d Deps) consumeLowest(ctx context.Context, tx pgx.Tx, charID id.UUID,
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
		FOR UPDATE OF ii`, charID.String(), itemID).Scan(&instID, &have)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNoBait, itemID)
	}
	if have < int64(qty) {
		return fmt.Errorf("%w: %s", ErrNoBait, itemID)
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

// claimOverflow routes the unplaceable earned catch to Reward Claims:
// source_type/source_family FISHING, source_ref =
// (fishing_spot_id, character_id, utc_date, cast_sequence)
// (world_rules.md § Folk Fishing / reward_claims.md).
func (d Deps) claimOverflow(ctx context.Context, tx pgx.Tx, charID id.UUID,
	rec *journalv1.DurableCommandRecord, catchID string, def ItemDef,
	spotID, castDate string, seq uint64) error {
	opID := opUUID(rec)
	rev := ""
	if d.ContentRevision != nil {
		rev = *d.ContentRevision
	}
	in := &reward.Input{
		OwnerCharacterID:        charID,
		SourceType:              "FISHING",
		SourceReference:         fmt.Sprintf("%s.%s.%s.%d", spotID, charID.String(), castDate, seq),
		RewardSlot:              fmt.Sprintf("hook.%x.0", opID[:]),
		SourceRewardOperationID: opID,
		Lines: []reward.LineInput{{
			Kind:             "ITEM",
			ItemID:           catchID,
			Quantity:         big.NewInt(1),
			EffectiveBinding: def.Def.DefaultBinding.String(),
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
		return id.UUID{}, fmt.Errorf("fishing: malformed record owner")
	}
	var c id.UUID
	copy(c[:], rec.GetOwnerId())
	return c, nil
}

func consumeCode(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, ErrNoBait), errors.Is(err, ErrInsufficientItems):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	case errors.Is(err, ErrNoRod), errors.Is(err, ErrNotFound):
		return protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}

func interactErr(opID id.UUID, code protocolv1.ErrorCode,
	kind protocolv1.InteractKind, target string) *protocolv1.S2CInteractResult {
	return &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
			ErrorCode:   code,
		},
		InteractKind: kind,
		TargetId:     target,
	}
}

// verdictInteract packs one S2C_INTERACT_RESULT as the record's
// client_result outcome (mirrors durable/cooking.verdictInteract).
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
