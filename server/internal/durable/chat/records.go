package chat

import (
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// ChatLogFamily is the closed queue family of the chat delivery
// subsystem's asynchronous moderation-log producer (save_rules.md §
// Closed Durable Queue Producer Registry: `chat.log` / CHAT_LOG).
const ChatLogFamily = "chat.log"

// Entry is one accepted chat message as delivered — the authoritative
// message UUID, sender identity, channel scope and text exactly as
// fanned out. It maps 1:1 onto JournalChatLog (protobuf_conventions §7).
type Entry struct {
	MessageID         id.UUID
	SenderAccountID   id.UUID
	SenderCharacterID id.UUID
	Channel           string
	ScopeID           *string
	Content           string
	CreatedAt         time.Time
}

// NewRecord builds the queue record for one accepted message. The
// record's operation_id is the message UUID itself — the same accepted
// message re-submitted after a crash resolves as an idempotent replay
// instead of a second receipt.
func NewRecord(e Entry) (*journalv1.DurableCommandRecord, error) {
	if e.MessageID == (id.UUID{}) {
		return nil, errField("message_id")
	}
	if e.SenderAccountID == (id.UUID{}) || e.SenderCharacterID == (id.UUID{}) {
		return nil, errField("sender")
	}
	if e.Content == "" {
		return nil, errField("content")
	}
	j := &journalv1.JournalChatLog{
		MessageId:         e.MessageID[:],
		SenderAccountId:   e.SenderAccountID[:],
		SenderCharacterId: e.SenderCharacterID[:],
		Channel:           e.Channel,
		Content:           e.Content,
		CreatedAtMs:       e.CreatedAt.UnixMilli(),
	}
	if e.ScopeID != nil && *e.ScopeID != "" {
		j.ScopeId = e.ScopeID
	}
	reqBytes, err := proto.Marshal(j)
	if err != nil {
		return nil, err
	}
	fpr := id.OperationFingerprint(ChatLogFamily, e.SenderCharacterID,
		e.MessageID, reqBytes)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    ChatLogFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            e.SenderCharacterID[:],
		OperationId:        e.MessageID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG,
		EnqueuedAtMs:       time.Now().UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_ChatLog{
			ChatLog: j,
		},
	}, nil
}
