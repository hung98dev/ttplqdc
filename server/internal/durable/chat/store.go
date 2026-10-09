package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// Store applies chat_messages writes inside the queue's committing
// transaction. It is the only writer of the moderation log.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs the store against the live pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// q runs statements on the in-flight transaction or falls back to the
// pool for read helpers.
func (s *Store) q(tx pgx.Tx) db {
	if tx != nil {
		return tx
	}
	return s.pool
}

type db interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// InsertRow appends one chat_messages row keyed message_id. The insert
// is idempotent: replay of an already-logged message is a no-op, and a
// message older than the 90-day retention horizon is skipped rather
// than reinserted (personal_data_register § 1 F — replay never
// reinserts an expired subject's log).
func (s *Store) InsertRow(ctx context.Context, tx pgx.Tx, e Entry) error {
	tag, err := s.q(tx).Exec(ctx,
		`INSERT INTO chat_messages
		    (message_id, sender_account_id, sender_character_id,
		     channel, scope_id, content, created_at)
		 SELECT $1, $2, $3, $4, $5, $6, $7::timestamptz
		 WHERE $7::timestamptz > $8
		 ON CONFLICT (message_id) DO NOTHING`,
		e.MessageID[:], e.SenderAccountID[:], e.SenderCharacterID[:],
		e.Channel, e.ScopeID, e.Content, e.CreatedAt,
		time.Now().Add(-RetentionWindow))
	if err != nil {
		return fmt.Errorf("chat: insert message: %w", err)
	}
	_ = tag
	return nil
}

// InsertBatch appends up to one bounded batch of rows in one statement.
// The executor uses it when a commit carries several log entries.
func (s *Store) InsertBatch(ctx context.Context, tx pgx.Tx, entries []Entry) error {
	for _, e := range entries {
		if err := s.InsertRow(ctx, tx, e); err != nil {
			return err
		}
	}
	return nil
}

// PurgeExpired deletes up to `bound` rows older than the 90-day rolling
// retention deadline — one bounded batch of the daily purge job
// (personal_data_register § 1; IMP-056 schedules the job).
func (s *Store) PurgeExpired(ctx context.Context, tx pgx.Tx, now time.Time, bound int) (int64, error) {
	tag, err := s.q(tx).Exec(ctx,
		`DELETE FROM chat_messages WHERE ctid IN (
		    SELECT ctid FROM chat_messages
		     WHERE created_at <= $1
		     LIMIT $2)`,
		now.Add(-RetentionWindow), bound)
	if err != nil {
		return 0, fmt.Errorf("chat: purge expired: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteByAccount removes every moderation-log row of one account —
// the subject-erasure action (within 15 days of the erasure request,
// data_protection.md / data_model.md § chat_messages). IMP-056 drives
// it from the erasure path.
func (s *Store) DeleteByAccount(ctx context.Context, tx pgx.Tx, accountID id.UUID) (int64, error) {
	tag, err := s.q(tx).Exec(ctx,
		`DELETE FROM chat_messages WHERE sender_account_id = $1`,
		accountID[:])
	if err != nil {
		return 0, fmt.Errorf("chat: erase account rows: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Count is the test-facing row counter.
func (s *Store) Count(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT COUNT(*) FROM chat_messages`).Scan(&n)
	return n, err
}

// Exists reports whether a message row is present (report evidence
// lookup mirrors durable/social).
func (s *Store) Exists(ctx context.Context, tx pgx.Tx, messageID id.UUID) (bool, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT 1 FROM chat_messages WHERE message_id = $1`,
		messageID[:]).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
