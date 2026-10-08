package discovery

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TravelServiceID is the NPC_SERVICE service_id for travel
// (npc_shop_catalog.md § Shared Service Shape; messages.md §160).
const TravelServiceID = "travel"

// reasonTravelFee is the currency audit reason for the destination-tier
// fee charged once per operation_id inside the commit.
const reasonTravelFee = "travel.fee"

// TravelExecutor commits interaction.npc_service records whose
// service_id is "travel": destination must be a travel-eligible safe
// anchor, the destination must already be discovered (NOT_DISCOVERED
// otherwise — travel itself never creates discovery), and the tier fee
// is debited once per operation_id inside the commit. SUCCESS carries
// the 116 with the currency_delta; the post-commit 105 TRAVEL transfer
// is the runtime effect wired at composition.
func TravelExecutor(d Deps) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		if rec.GetOperationFamily() != familyNpcService {
			return idempotency.Outcome{}, fmt.Errorf("discovery: unsupported client family %q", rec.GetOperationFamily())
		}
		cmd := rec.GetClient()
		req := cmd.GetC2SInteract()
		if cmd == nil || req == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
			return idempotency.Outcome{}, fmt.Errorf("discovery: malformed npc_service record")
		}
		var charID, opID id.UUID
		copy(charID[:], rec.GetOwnerId())
		copy(opID[:], rec.GetOperationId())

		result := &protocolv1.S2CInteractResult{
			Result: &protocolv1.OperationResult{
				OperationId: opID[:],
				Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
				ErrorCode:   protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID,
			},
			InteractKind: req.GetInteractKind(),
			TargetId:     req.GetTargetId(),
		}

		if req.GetInteractKind() != protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE ||
			req.GetServiceId() != TravelServiceID {
			return interactVerdict(opID, result)
		}
		dest := req.GetServiceParam()
		if _, ok := d.AnchorFor(dest); !ok {
			return interactVerdict(opID, result)
		}
		fee, ok := d.FeeFor(dest)
		if !ok {
			return interactVerdict(opID, result)
		}

		if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
			return idempotency.Outcome{}, err
		}
		discovered, err := d.Store.Discovered(ctx, tx, charID, KindMap, dest)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if !discovered {
			result.Result.ErrorCode = protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED
			return interactVerdict(opID, result)
		}

		if fee > 0 {
			if _, err := currency.Debit(ctx, tx, currency.Mutation{
				CharacterID: charID,
				CurrencyID:  currency.Common,
				Delta:       fee,
				OperationID: opID,
				ReasonCode:  reasonTravelFee,
				SourceRef:   "travel." + dest,
				Actor:       currency.ActorPlayer,
			}); err != nil {
				if err == currency.ErrInsufficientBalance {
					result.Result.ErrorCode = protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY
					return interactVerdict(opID, result)
				}
				return idempotency.Outcome{}, err
			}
			result.CurrencyDelta = []*protocolv1.CurrencyDelta{
				{CurrencyId: string(currency.Common), Amount: -fee},
			}
		}

		result.Result.Status = protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
		result.Result.ErrorCode = protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
		return interactOK(opID, result)
	}
}
