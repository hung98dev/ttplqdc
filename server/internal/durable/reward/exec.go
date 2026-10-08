package reward

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ClaimRecord builds the schema-v1 durable command record for one
// C2S_REWARD_CLAIM intent (the ProducerClient envelope the router's
// "reward.claim" family submits).
func ClaimRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SRewardClaim,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("reward: claim request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	issuedMs, _ := id.OperationIssuedAt(opID)
	_ = issuedMs
	rec := &journalv1.DurableCommandRecord{
		SchemaVersion:   1,
		OperationFamily: ClaimFamily,
		OwnerKind:       journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:         characterID[:],
		OperationId:     opID[:],
		CommandType:     journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:    admittedAt.UnixMilli(),
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:      accountID[:],
				CharacterId:    characterID[:],
				SessionEpoch:   sessionEpoch,
				OwnershipEpoch: &ownershipEpoch,
				AdmittedAtMs:   admittedAt.UnixMilli(),
				IssuedAtMs:     issuedMs,
				Request: &journalv1.JournalClientCommand_C2SRewardClaim{
					C2SRewardClaim: req,
				},
			},
		},
	}
	return rec, nil
}

// KindMux is the durable reward-command executor surface: one
// ProducerReward executor dispatching each committed
// JournalRewardCommand to the consumer registered for its `kind`
// (save_rules.md §7). Composition registers kind consumers
// (e.g. "rest" via RestExecutor); a record whose kind has no consumer
// resolves terminally — fail-closed, never re-executed silently.
func KindMux(consumers map[string]queue.Executor) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx, rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		rw := rec.GetReward()
		if rw == nil {
			return idempotency.Outcome{}, fmt.Errorf("reward: record without reward payload")
		}
		ex, ok := consumers[rw.GetKind()]
		if !ok {
			return idempotency.Outcome{}, fmt.Errorf("reward: no consumer for kind %q", rw.GetKind())
		}
		return ex(ctx, tx, rec)
	}
}
