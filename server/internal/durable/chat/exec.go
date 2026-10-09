package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Executor applies one `chat.log` record inside the committing
// transaction: the frozen JournalChatLog becomes exactly one
// chat_messages row. CHAT_LOG carries no client result — the retained
// outcome is a server-only SUCCESS marker.
func (s *Store) Executor() queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		if rec.GetOperationFamily() != ChatLogFamily {
			return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
		}
		j := rec.GetChatLog()
		if j == nil {
			return idempotency.Outcome{}, ErrMalformedRecord
		}
		e, err := entry(j)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if err := s.InsertRow(ctx, tx, e); err != nil {
			return idempotency.Outcome{}, err
		}
		return marshalOutcome(&journalv1.JournalOutcome{
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
			OperationId: rec.GetOperationId(),
		})
	}
}

// entry decodes the frozen journal row back into a log entry. All
// identity fields must be intact — the queue record froze the
// authoritative values at send time.
func entry(j *journalv1.JournalChatLog) (Entry, error) {
	var e Entry
	if len(j.GetMessageId()) != 16 || len(j.GetSenderAccountId()) != 16 ||
		len(j.GetSenderCharacterId()) != 16 {
		return e, errField("uuid")
	}
	if j.GetContent() == "" {
		return e, errField("content")
	}
	copy(e.MessageID[:], j.GetMessageId())
	copy(e.SenderAccountID[:], j.GetSenderAccountId())
	copy(e.SenderCharacterID[:], j.GetSenderCharacterId())
	e.Channel = j.GetChannel()
	e.Content = j.GetContent()
	if j.ScopeId != nil {
		s := j.GetScopeId()
		e.ScopeID = &s
	}
	e.CreatedAt = time.UnixMilli(j.GetCreatedAtMs()).UTC()
	return e, nil
}

// marshalOutcome serializes the schema-v1 JournalOutcome as protojson —
// the retained representation AwaitClientOutcome decodes.
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
