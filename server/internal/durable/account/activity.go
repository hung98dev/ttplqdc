package account

import (
	"context"
	"fmt"
	"time"

	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
)

// Activity transitions of JournalActivity.transition for the
// character.activity family (save_rules.md producer row: first-attach /
// session activity / detach lifecycle).
const (
	ActivityAttached = "ATTACHED"
	ActivityDetached = "DETACHED"
)

// ClearStaleSessionActive resets session_active on every character row.
// Invoked from cmd/server before the public listener reports ready: a
// process restart leaves rows marked live although all slots and sessions
// were cleared (data_model.md § character_activity; session.md § Login
// Queue "restart clears both reservations and runtime sessions").
func ClearStaleSessionActive(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx,
		`UPDATE character_activity SET session_active = FALSE WHERE session_active`)
	return err
}

// ActivityRecord builds one character.activity command record for queue
// admission. Operation identity is deterministic per
// (character, epoch, transition) so a resubmitted transition dedupes.
func ActivityRecord(characterID id.UUID, epoch uint64, transition string, occurredAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID := id.ServerJobOperationID("character.activity",
		characterID.String(), fmt.Sprintf("%d", epoch), transition)
	if opID.IsNil() {
		return nil, fmt.Errorf("account: invalid activity job identity %s %d %s",
			characterID, epoch, transition)
	}
	fpr := id.OperationFingerprint("character.activity", characterID, opID, nil)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    "character.activity",
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_ACTIVITY,
		EnqueuedAtMs:       occurredAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Activity{
			Activity: &journalv1.JournalActivity{
				CharacterId:  characterID[:],
				SessionEpoch: epoch,
				Transition:   transition,
				OccurredAtMs: occurredAt.UnixMilli(),
			},
		},
	}, nil
}

// SubmitActivity enqueues one character.activity record.
func SubmitActivity(ctx context.Context, q *queue.Queue, characterID id.UUID,
	epoch uint64, transition string, occurredAt time.Time) error {
	rec, err := ActivityRecord(characterID, epoch, transition, occurredAt)
	if err != nil {
		return err
	}
	return q.Submit(ctx, rec)
}

// ActivityExecutor is the queue executor for ProducerActivity: it writes
// the immutable attach event and the character_activity projection inside
// the committing transaction (one queue-executor per save_rules.md row).
func (s *Store) ActivityExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	a := rec.GetActivity()
	if a == nil {
		return idempotency.Outcome{}, fmt.Errorf("account: activity record without payload")
	}
	if len(a.GetCharacterId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("account: activity bad character id")
	}
	var charID id.UUID
	copy(charID[:], a.GetCharacterId())
	if err := LockCharacter(ctx, tx, charID); err != nil {
		return idempotency.Outcome{}, err
	}
	at := time.UnixMilli(a.GetOccurredAtMs()).UTC()
	switch a.GetTransition() {
	case ActivityAttached:
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_attach_events (character_id, session_epoch, attached_at)
			 VALUES ($1,$2,$3)`,
			charID, int64(a.GetSessionEpoch()), at); err != nil {
			return idempotency.Outcome{}, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_activity (character_id, last_attached_at, session_active)
			 VALUES ($1,$2,TRUE)
			 ON CONFLICT (character_id) DO UPDATE SET
			   last_attached_at = EXCLUDED.last_attached_at,
			   session_active = TRUE`,
			charID, at); err != nil {
			return idempotency.Outcome{}, err
		}
	case ActivityDetached:
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_activity (character_id, last_detached_at, session_active)
			 VALUES ($1,$2,FALSE)
			 ON CONFLICT (character_id) DO UPDATE SET
			   last_detached_at = EXCLUDED.last_detached_at,
			   session_active = FALSE`,
			charID, at); err != nil {
			return idempotency.Outcome{}, err
		}
	default:
		return idempotency.Outcome{}, fmt.Errorf("account: unknown activity transition %q",
			a.GetTransition())
	}
	out, err := json.Marshal(map[string]string{"applied": a.GetTransition()})
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: out}, nil
}
