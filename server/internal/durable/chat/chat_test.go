package chat

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
)

// TestChatPersistedToChatMessages: an accepted message flowing through
// the chat.log executor lands in chat_messages with the authoritative
// message UUID/sender/scope/text/time — and replay of the same record
// stays a no-op (append-once moderation log).
func TestChatPersistedToChatMessages(t *testing.T) {
	wipe(t)
	st := New(requirePool(t))
	acct, char := mkSender(t)
	now := time.Now().UTC()
	scope := "map.lang_son"
	e := Entry{
		MessageID:         id.NewV4(),
		SenderAccountID:   acct,
		SenderCharacterID: char,
		Channel:           "LOCAL",
		ScopeID:           &scope,
		Content:           "chao mung",
		CreatedAt:         now,
	}
	rec, err := NewRecord(e)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	apply(t, st, rec)
	apply(t, st, rec) // replay — one row, never double-logged

	n, err := st.Count(context.Background(), nil)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	var gotAcct, gotChar, gotMsg []byte
	var channel, content string
	var scopeID *string
	var created time.Time
	err = sharedPool.QueryRow(context.Background(),
		`SELECT message_id, sender_account_id, sender_character_id,
		        channel, scope_id, content, created_at
		   FROM chat_messages`).Scan(&gotMsg, &gotAcct, &gotChar,
		&channel, &scopeID, &content, &created)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(gotMsg) != string(e.MessageID[:]) ||
		string(gotAcct) != string(acct[:]) ||
		string(gotChar) != string(char[:]) {
		t.Fatal("row identity mismatch")
	}
	if channel != "LOCAL" || content != "chao mung" {
		t.Fatal("row payload mismatch")
	}
	if scopeID == nil || *scopeID != scope {
		t.Fatal("scope_id mismatch")
	}
	if created.UnixMilli() != now.UnixMilli() {
		t.Fatal("created_at marker mismatch")
	}
}

// TestNinetyDayRetentionDeadline: the retention deadline is the
// created_at marker — a row inserted with created_at older than 90 days
// is purge-eligible, a fresh row is not; and the executor refuses to
// reinsert an already-expired message.
func TestNinetyDayRetentionDeadline(t *testing.T) {
	wipe(t)
	st := New(requirePool(t))
	acct, char := mkSender(t)
	now := time.Now().UTC()

	old := Entry{MessageID: id.NewV4(), SenderAccountID: acct,
		SenderCharacterID: char, Channel: "WORLD", Content: "old",
		CreatedAt: now.Add(-RetentionWindow - time.Hour)}
	fresh := Entry{MessageID: id.NewV4(), SenderAccountID: acct,
		SenderCharacterID: char, Channel: "WORLD", Content: "fresh",
		CreatedAt: now}

	// Executor path refuses to (re)insert an already-expired row.
	rec, err := NewRecord(old)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	apply(t, st, rec)
	if n, _ := st.Count(context.Background(), nil); n != 0 {
		t.Fatalf("expired message reinserted (rows=%d)", n)
	}

	// Direct rows: purge batch removes only the expired one. Seed past
	// the executor guard via raw SQL — production rows are only ever
	// inserted fresh.
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO chat_messages
		    (message_id, sender_account_id, sender_character_id,
		     channel, scope_id, content, created_at)
		 VALUES ($1,$2,$3,$4,NULL,$5,$6), ($7,$2,$3,$4,NULL,$8,$9)`,
		old.MessageID[:], acct[:], char[:], "WORLD", old.Content,
		old.CreatedAt, fresh.MessageID[:], fresh.Content,
		fresh.CreatedAt); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tx, err := sharedPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	deleted, err := st.PurgeExpired(context.Background(), tx, now, 100)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("purged = %d, want 1", deleted)
	}
	ok, err := st.Exists(context.Background(), nil, fresh.MessageID)
	if err != nil || !ok {
		t.Fatal("fresh row purged")
	}
	ok, err = st.Exists(context.Background(), nil, old.MessageID)
	if err != nil || ok {
		t.Fatal("expired row survived purge")
	}
}
