package world

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// SetCheckpointService is the wire service_id of the checkpoint write
// this package implements (npc_shop_catalog.md § Guide Service Contract).
const SetCheckpointService = "set_checkpoint"

// Deps are the constructor-time dependencies of the world executors.
type Deps struct {
	Store *Store
}

// Executors returns the world client-family executor exports for the
// composition ProducerClient family-mux:
//
//	interaction.npc_service -> S2C_INTERACT_RESULT (116)
//	placement.portal        -> S2C_INTERACT_RESULT (116, kind PORTAL)
//	placement.channel       -> S2C_CHANNEL_SWITCH_RESULT (110)
//
// Every executor commits its write inside Store.TrustedReplay under the
// receipt lock (ADR-0081) and produces the recorded client_result the
// edge delivers. Admission rejects (range/in_combat/capacity/cooldown)
// are consult-side: admitted records commit SUCCESS unless the record
// itself is malformed.
func Executors(deps Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		FamilyNpcService:       deps.npcServiceExec,
		FamilyPlacementPortal:  deps.portalExec,
		FamilyPlacementChannel: deps.channelExec,
	}
}

// CheckpointExecutors returns the ProducerCheckpoint executor for the
// server-driven sim.checkpoint family (transfer completion / respawn /
// recovery writes); composition binds it to ProducerCheckpoint.
func (deps Deps) CheckpointExecutors() map[string]queue.Executor {
	return map[string]queue.Executor{
		FamilySimCheckpoint: deps.checkpointExec,
	}
}

// npcServiceExec commits the CLIENT-variant interaction.npc_service
// write. The admission consult already resolved the service to the
// checkpoint payload on the command (field 12); the executor applies
// characters.checkpoint_id and returns the 116 client_result. An
// unregistered service or a malformed request commits TARGET_INVALID —
// exactly one 116 per 103, never silence.
func (d Deps) npcServiceExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyNpcService {
		return idempotency.Outcome{}, fmt.Errorf("world: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	if cmd == nil {
		return idempotency.Outcome{}, fmt.Errorf("world: npc_service record without client payload")
	}
	req := cmd.GetC2SInteract()
	if req == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("world: malformed npc_service record")
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
		req.GetServiceId() != SetCheckpointService {
		return verdictInteract(opID, result)
	}
	cp := cmd.GetCheckpoint()
	if cp == nil || cp.GetCheckpointId() == "" {
		return verdictInteract(opID, result)
	}

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	store := d.Store
	if err := store.SetCheckpoint(ctx, tx, charID, cp.GetCheckpointId()); err != nil {
		if err == ErrNotFound {
			return verdictInteract(opID, result)
		}
		return idempotency.Outcome{}, err
	}

	result.Result.Status = protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	result.Result.ErrorCode = protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		Checkpoint:  cp,
		ClientResult: &journalv1.JournalOutcome_S2CInteractResult{
			S2CInteractResult: result,
		},
	}
	return marshalOutcome(outcome)
}

// portalExec commits the placement.portal intent: the committed write
// is the durable receipt plus the recorded 116 (PORTAL) — the map write
// itself lands on transfer completion via sim.checkpoint.
func (d Deps) portalExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyPlacementPortal {
		return idempotency.Outcome{}, fmt.Errorf("world: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SPortalUse()
	if cmd == nil || req == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("world: malformed portal record")
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())

	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInteractResult{
			S2CInteractResult: &protocolv1.S2CInteractResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				InteractKind: protocolv1.InteractKind_INTERACT_KIND_PORTAL,
				TargetId:     req.GetPortalId(),
			},
		},
	}
	return marshalOutcome(outcome)
}

// channelExec commits the placement.channel intent: receipt plus the
// recorded 110 — characters has no channel column (R3-4).
func (d Deps) channelExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilyPlacementChannel {
		return idempotency.Outcome{}, fmt.Errorf("world: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SChannelSwitch()
	if cmd == nil || req == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("world: malformed channel record")
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())

	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CChannelSwitchResult{
			S2CChannelSwitchResult: &protocolv1.S2CChannelSwitchResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				TargetChannelIndex: req.GetTargetChannelIndex(),
			},
		},
	}
	return marshalOutcome(outcome)
}

// checkpointExec commits a server-driven CHECKPOINT record: the payload
// (JournalCheckpoint, field 25) is authoritative — characters.checkpoint_id
// and characters.map_id are written only when present in the payload.
func (d Deps) checkpointExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FamilySimCheckpoint {
		return idempotency.Outcome{}, fmt.Errorf("world: unsupported checkpoint family %q", rec.GetOperationFamily())
	}
	cp := rec.GetCheckpoint()
	if cp == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("world: malformed checkpoint record")
	}
	var charID, opID id.UUID
	copy(charID[:], rec.GetOwnerId())
	copy(opID[:], rec.GetOperationId())

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if cp.GetCheckpointId() != "" {
		if err := d.Store.SetCheckpoint(ctx, tx, charID, cp.GetCheckpointId()); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	if cp.GetSafeMapId() != "" {
		if err := d.Store.SetMap(ctx, tx, charID, cp.GetSafeMapId()); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		Checkpoint:  cp,
	}
	return marshalOutcome(outcome)
}

// verdictInteract commits a terminal nonexecution carrying the 116
// client_result the edge delivers.
func verdictInteract(opID id.UUID, result *protocolv1.S2CInteractResult) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   result.GetResult().GetErrorCode(),
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInteractResult{
			S2CInteractResult: result,
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the schema-v1 JournalOutcome as protojson —
// the retained durable_command_receipts.outcome representation.
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}

