package discovery

import (
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// interactVerdict commits a terminal nonexecution carrying the 116
// client_result the edge delivers — same shape as the checkpoint
// verdict in durable/world, one 116 per 103, never silence.
func interactVerdict(opID id.UUID, result *protocolv1.S2CInteractResult) (idempotency.Outcome, error) {
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

// interactOK commits SUCCESS carrying the 116 client_result.
func interactOK(opID id.UUID, result *protocolv1.S2CInteractResult) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CInteractResult{
			S2CInteractResult: result,
		},
	}
	return marshalOutcome(outcome)
}

// grantOutcome commits a REWARD outcome; reward_slots echo the committed
// grant surface (empty on a replay that granted nothing).
func grantOutcome(opID id.UUID, slots []*journalv1.JournalRewardSlot) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		RewardSlots: slots,
	}
	return marshalOutcome(outcome)
}

// grantVerdict commits a terminal rejected REWARD outcome.
func grantVerdict(opID id.UUID, code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
	}
	return marshalOutcome(outcome)
}

func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
