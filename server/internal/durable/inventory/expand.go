package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// expandPrices is the capacity ladder: capacity 60→120 in +10 steps;
// the price charged at capacity c (c < 120) is expandPrices[(c-60)/10]
// (inventory.md § invariants + plan's ratified table).
var expandPrices = []int64{10000, 25000, 50000, 100000, 200000, 400000}

const (
	minCapacity  = 60
	maxCapacity  = 120
	capacityStep = 10
)

// expandExecutor applies one C2S_INVENTORY_EXPAND record inside
// TrustedReplay: expected_capacity optimistic check, CAPACITY_FULL at
// 120, ALL_OR_NOTHING currency.common debit, capacity += 10 and
// revision += 1 in the same commit.
func (d Deps) expandExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ExpandFamily {
		return idempotency.Outcome{}, fmt.Errorf("inventory: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SInventoryExpand()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("inventory: record without c2s_inventory_expand")
	}

	// Lock order canonical: characters (20) → character_currencies (30)
	// → character_inventories (40). The debit path re-takes the
	// currency row inside currency.Debit — same-tx relock, no new
	// inversion.
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", charID),
		lockorder.RowLock("character_currencies", charID),
		lockorder.RowLock("character_inventories", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	capacity, _, err := ensureInventoryRow(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if req.GetExpectedCapacity() != uint32(capacity) {
		return d.expandVerdict(opID, ErrConflict)
	}
	if capacity >= maxCapacity {
		return d.expandVerdict(opID, ErrCapacityFull)
	}
	price := expandPrices[(capacity-minCapacity)/capacityStep]

	_, err = currency.Debit(ctx, tx, currency.Mutation{
		CharacterID: charID,
		CurrencyID:  currency.Common,
		Delta:       price,
		OperationID: opID,
		ReasonCode:  "inventory.expand",
		SourceRef:   "character_inventories.capacity",
		Actor:       currency.ActorPlayer,
	})
	if err != nil {
		if errors.Is(err, currency.ErrInsufficientBalance) {
			return d.expandVerdict(opID, ErrInsufficientCurrency)
		}
		return idempotency.Outcome{}, err
	}

	after, err := tx.Query(ctx,
		`UPDATE character_inventories
		    SET capacity = capacity + $2, revision = revision + 1
		  WHERE character_id = $1
		  RETURNING capacity`, charID.String(), capacityStep)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var capacityAfter int32
	if after.Next() {
		if err := after.Scan(&capacityAfter); err != nil {
			after.Close()
			return idempotency.Outcome{}, err
		}
	}
	after.Close()

	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInventoryExpandResult{
			S2CInventoryExpandResult: &protocolv1.S2CInventoryExpandResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				CapacityAfter: uint32(capacityAfter),
				CurrencyDelta: []*protocolv1.CurrencyDelta{
					{CurrencyId: string(currency.Common), Amount: -price},
				},
			},
		},
	}
	return marshalOutcome(outcome)
}

// expandVerdict writes the committed 429 nonexecution; the in-set
// error list embeds S2CInventoryExpandResult as client_result.
func (d Deps) expandVerdict(opID id.UUID, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
	}
	if expandInSet(code) {
		outcome.ClientResult = &journalv1.JournalOutcome_S2CInventoryExpandResult{
			S2CInventoryExpandResult: &protocolv1.S2CInventoryExpandResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
			},
		}
	}
	return marshalOutcome(outcome)
}
