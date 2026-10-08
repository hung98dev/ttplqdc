package discovery

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// familyNpcService mirrors durable/world's family constant; the wire id
// is the contract, the packages stay decoupled.
const familyNpcService = "interaction.npc_service"

// NpcServiceMux routes interaction.npc_service records to the executor
// registered for the record's service_id (messages.md §160: one
// dispatcher, per-service handlers registered by the owning packet,
// unregistered service → exactly one TARGET_INVALID, never silence).
// Composition merges the family entries — e.g.
// map[string]queue.Executor{"set_checkpoint": <IMP-018 exec>,
// "travel": discovery.TravelExecutor(deps)} — behind the family's
// single q.RegisterExecutor slot without either package importing the
// other.
type NpcServiceMux map[string]queue.Executor

// Exec resolves service_id and dispatches. Unknown services commit a
// terminal TARGET_INVALID verdict carrying the 116.
func (m NpcServiceMux) Exec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != familyNpcService {
		return idempotency.Outcome{}, fmt.Errorf("discovery: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SInteract()
	if cmd == nil || req == nil || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("discovery: malformed npc_service record")
	}
	if ex, ok := m[req.GetServiceId()]; ok {
		return ex(ctx, tx, rec)
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	return interactVerdict(opID, &protocolv1.S2CInteractResult{
		Result: &protocolv1.OperationResult{
			OperationId: opID[:],
			Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
			ErrorCode:   protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID,
		},
		InteractKind: req.GetInteractKind(),
		TargetId:     req.GetTargetId(),
	})
}
