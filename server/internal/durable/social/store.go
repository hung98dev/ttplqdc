package social

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// FriendLimit, OutgoingCap and BlockCap mirror social.md § Friends /
// Blocks; RequestTTL is the fixed 7-day pending window the schema's
// expires_at CHECK enforces.
const (
	FriendLimit  = 100
	OutgoingCap  = 100
	BlockCap     = 500
	RequestTTL   = 7 * 24 * time.Hour
	ReportWindow = 24 * time.Hour
	ReportLimit  = 10
)

// Store is the sole SQL surface for the social tables. Methods take a
// pgx.Tx inside the committing transaction, or nil to read on the pool.
type Store struct {
	pool *pgxpool.Pool
}

// New builds the store over the durable pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// q returns the tx when non-nil, else the pool.
func (s *Store) q(tx pgx.Tx) interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
} {
	if tx != nil {
		return tx
	}
	return s.pool
}

// PendingRequest is one unresolved friend_requests row.
type PendingRequest struct {
	RequestID id.UUID
	Requester id.UUID
	Target    id.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

// FriendRow is one friends pair member for the reading character.
type FriendRow struct {
	FriendID    id.UUID
	DisplayName string
	CreatedAt   time.Time
}

// BlockRow is one blocks entry for the blocking character.
type BlockRow struct {
	BlockedID   id.UUID
	DisplayName string
	CreatedAt   time.Time
}

// CharacterInfo carries the character fields social gates read.
type CharacterInfo struct {
	DisplayName   string
	Level         int32
	SessionActive bool
}

// lowHigh orders a pair for the unordered friends key.
func lowHigh(a, b id.UUID) (lo, hi id.UUID) {
	for i := 0; i < 16; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return a, b
			}
			return b, a
		}
	}
	return a, b
}

// Character reads display_name/level/session_active for one character.
func (s *Store) Character(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) (CharacterInfo, bool, error) {
	var info CharacterInfo
	err := s.q(tx).QueryRow(ctx,
		`SELECT c.name, c.level, COALESCE(a.session_active, FALSE)
		   FROM characters c
		   LEFT JOIN character_activity a ON a.character_id = c.character_id
		   WHERE c.character_id = $1`, characterID[:]).
		Scan(&info.DisplayName, &info.Level, &info.SessionActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return info, false, nil
	}
	if err != nil {
		return info, false, err
	}
	return info, true, nil
}

// AreFriends reports whether the unordered pair exists in friends.
func (s *Store) AreFriends(ctx context.Context, tx pgx.Tx,
	a, b id.UUID) (bool, error) {
	lo, hi := lowHigh(a, b)
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT 1 FROM friends
		  WHERE character_low_id = $1 AND character_high_id = $2`,
		lo[:], hi[:]).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// BlockedEither reports a block in either direction — the shared
// can_direct_interact gate for whispers and social invites.
func (s *Store) BlockedEither(ctx context.Context, tx pgx.Tx,
	a, b id.UUID) (bool, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT 1 FROM blocks
		  WHERE (blocker_character_id = $1 AND blocked_character_id = $2)
		     OR (blocker_character_id = $2 AND blocked_character_id = $1)`,
		a[:], b[:]).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// IsBlocked reports the directional blocker→blocked row only.
func (s *Store) IsBlocked(ctx context.Context, tx pgx.Tx,
	blocker, blocked id.UUID) (bool, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT 1 FROM blocks
		  WHERE blocker_character_id = $1 AND blocked_character_id = $2`,
		blocker[:], blocked[:]).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// PendingBetween returns the live PENDING request for the unordered
// pair (any direction), treating expires_at <= now as expired — the
// schema keeps such rows until a resolver touches them (social.md
// § Requests).
func (s *Store) PendingBetween(ctx context.Context, tx pgx.Tx,
	a, b id.UUID, now time.Time) (*PendingRequest, bool, error) {
	p := &PendingRequest{}
	err := s.q(tx).QueryRow(ctx,
		`SELECT friend_request_id, requester_character_id,
		        target_character_id, created_at, expires_at
		   FROM friend_requests
		  WHERE state = 'PENDING'
		    AND ((requester_character_id = $1 AND target_character_id = $2)
		      OR (requester_character_id = $2 AND target_character_id = $1))`,
		a[:], b[:]).
		Scan(&p.RequestID, &p.Requester, &p.Target, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !p.ExpiresAt.After(now) {
		return nil, false, nil
	}
	return p, true, nil
}

// PendingFrom returns the live PENDING request with the given
// requester→target direction (for accept/decline).
func (s *Store) PendingFrom(ctx context.Context, tx pgx.Tx,
	requester, target id.UUID, now time.Time) (*PendingRequest, bool, error) {
	p := &PendingRequest{}
	err := s.q(tx).QueryRow(ctx,
		`SELECT friend_request_id, requester_character_id,
		        target_character_id, created_at, expires_at
		   FROM friend_requests
		  WHERE state = 'PENDING'
		    AND requester_character_id = $1
		    AND target_character_id = $2`,
		requester[:], target[:]).
		Scan(&p.RequestID, &p.Requester, &p.Target, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !p.ExpiresAt.After(now) {
		return nil, false, nil
	}
	return p, true, nil
}

// CountFriends counts both halves of the character's pairs.
func (s *Store) CountFriends(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) (int, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT COUNT(*) FROM friends
		  WHERE character_low_id = $1 OR character_high_id = $1`,
		characterID[:]).Scan(&n)
	return n, err
}

// CountOutgoing counts live PENDING requests the character sent —
// includes unexpired rows only per social.md.
func (s *Store) CountOutgoing(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, now time.Time) (int, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT COUNT(*) FROM friend_requests
		  WHERE requester_character_id = $1
		    AND state = 'PENDING' AND expires_at > $2`,
		characterID[:], now).Scan(&n)
	return n, err
}

// CountBlocks counts the character's directional blocks.
func (s *Store) CountBlocks(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) (int, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT COUNT(*) FROM blocks WHERE blocker_character_id = $1`,
		characterID[:]).Scan(&n)
	return n, err
}

// InsertPending creates one live PENDING request. The caller holds the
// pair lock; a concurrent insert trips the partial unique index —
// callers convert the violation into PENDING_REQUEST_EXISTS.
func (s *Store) InsertPending(ctx context.Context, tx pgx.Tx,
	requestID, requester, target, opID id.UUID, now time.Time) error {
	_, err := s.q(tx).Exec(ctx,
		`INSERT INTO friend_requests
		   (friend_request_id, requester_character_id, target_character_id,
		    state, created_at, expires_at, create_operation_id)
		 VALUES ($1, $2, $3, 'PENDING', $4, $5, $6)`,
		requestID[:], requester[:], target[:], now,
		now.Add(RequestTTL), opID[:])
	return err
}

// ResolvePending marks a PENDING row ACCEPTED/DECLINED/CANCELLED with
// the resolving operation id.
func (s *Store) ResolvePending(ctx context.Context, tx pgx.Tx,
	requestID id.UUID, state string, opID id.UUID, now time.Time) error {
	tag, err := s.q(tx).Exec(ctx,
		`UPDATE friend_requests
		    SET state = $2, resolved_at = $3, resolve_operation_id = $4
		  WHERE friend_request_id = $1 AND state = 'PENDING'`,
		requestID[:], state, now, opID[:])
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

// CancelPendingPair cancels every live PENDING row for the pair in
// either direction (block sever per data_model.md § Social).
func (s *Store) CancelPendingPair(ctx context.Context, tx pgx.Tx,
	a, b id.UUID, opID id.UUID, now time.Time) error {
	_, err := s.q(tx).Exec(ctx,
		`UPDATE friend_requests
		    SET state = 'CANCELLED', resolved_at = $3, resolve_operation_id = $4
		  WHERE state = 'PENDING'
		    AND ((requester_character_id = $1 AND target_character_id = $2)
		      OR (requester_character_id = $2 AND target_character_id = $1))`,
		a[:], b[:], now, opID[:])
	return err
}

// InsertFriend creates the unordered friends pair.
func (s *Store) InsertFriend(ctx context.Context, tx pgx.Tx,
	a, b, opID id.UUID, now time.Time) error {
	lo, hi := lowHigh(a, b)
	_, err := s.q(tx).Exec(ctx,
		`INSERT INTO friends
		   (character_low_id, character_high_id, created_at, created_operation_id)
		 VALUES ($1, $2, $3, $4)`, lo[:], hi[:], now, opID[:])
	return err
}

// DeleteFriend removes the unordered pair; reports whether a row went.
func (s *Store) DeleteFriend(ctx context.Context, tx pgx.Tx,
	a, b id.UUID) (bool, error) {
	lo, hi := lowHigh(a, b)
	tag, err := s.q(tx).Exec(ctx,
		`DELETE FROM friends
		  WHERE character_low_id = $1 AND character_high_id = $2`,
		lo[:], hi[:])
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// InsertBlock adds the directional block row.
func (s *Store) InsertBlock(ctx context.Context, tx pgx.Tx,
	blocker, blocked, opID id.UUID, now time.Time) error {
	_, err := s.q(tx).Exec(ctx,
		`INSERT INTO blocks
		   (blocker_character_id, blocked_character_id, created_at, operation_id)
		 VALUES ($1, $2, $3, $4)`, blocker[:], blocked[:], now, opID[:])
	return err
}

// DeleteBlock removes the directional block row.
func (s *Store) DeleteBlock(ctx context.Context, tx pgx.Tx,
	blocker, blocked id.UUID) (bool, error) {
	tag, err := s.q(tx).Exec(ctx,
		`DELETE FROM blocks
		  WHERE blocker_character_id = $1 AND blocked_character_id = $2`,
		blocker[:], blocked[:])
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ReportCount24h counts OPEN+closed reports the account submitted in
// the trailing 24h window (social.md § Reports limit).
func (s *Store) ReportCount24h(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, now time.Time) (int, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT COUNT(*) FROM player_reports
		  WHERE reporter_account_id = $1
		    AND created_at > $2`,
		accountID[:], now.Add(-ReportWindow)).Scan(&n)
	return n, err
}

// ChatMessageExists reports whether the referenced chat_messages row
// exists (632 evidence reference → ITEM_NOT_FOUND when absent).
func (s *Store) ChatMessageExists(ctx context.Context, tx pgx.Tx,
	messageID id.UUID) (bool, error) {
	var n int
	err := s.q(tx).QueryRow(ctx,
		`SELECT 1 FROM chat_messages WHERE message_id = $1`,
		messageID[:]).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// InsertReport writes the OPEN player_reports row.
func (s *Store) InsertReport(ctx context.Context, tx pgx.Tx,
	reportID, opID, accountID, reporter, target id.UUID, reason string,
	chatMessageID *id.UUID, notes *string, now time.Time) error {
	var chatRef []byte
	if chatMessageID != nil {
		chatRef = chatMessageID[:]
	}
	var notesVal any
	if notes != nil {
		notesVal = *notes
	}
	_, err := s.q(tx).Exec(ctx,
		`INSERT INTO player_reports
		   (report_id, operation_id, reporter_account_id,
		    reporter_character_id, target_character_id, reason,
		    chat_message_id, reporter_notes, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		reportID[:], opID[:], accountID[:], reporter[:], target[:],
		reason, chatRef, notesVal, now)
	return err
}

// ListFriends returns every pair member for the snapshot push.
func (s *Store) ListFriends(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) ([]FriendRow, error) {
	rows, err := s.q(tx).Query(ctx,
		`SELECT CASE WHEN f.character_low_id = $1
		              THEN f.character_high_id ELSE f.character_low_id END,
		        c.name, f.created_at
		   FROM friends f
		   JOIN characters c ON c.character_id =
		     CASE WHEN f.character_low_id = $1
		          THEN f.character_high_id ELSE f.character_low_id END
		  WHERE f.character_low_id = $1 OR f.character_high_id = $1
		  ORDER BY c.name, 1`, characterID[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FriendRow
	for rows.Next() {
		var r FriendRow
		var fid []byte
		if err := rows.Scan(&fid, &r.DisplayName, &r.CreatedAt); err != nil {
			return nil, err
		}
		copy(r.FriendID[:], fid)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListIncoming returns live PENDING requests targeting the character.
func (s *Store) ListIncoming(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, now time.Time) ([]PendingRequest, map[id.UUID]string, error) {
	rows, err := s.q(tx).Query(ctx,
		`SELECT r.friend_request_id, r.requester_character_id,
		        r.target_character_id, r.created_at, r.expires_at,
		        c.name
		   FROM friend_requests r
		   JOIN characters c ON c.character_id = r.requester_character_id
		  WHERE r.target_character_id = $1
		    AND r.state = 'PENDING' AND r.expires_at > $2
		  ORDER BY r.created_at`, characterID[:], now)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []PendingRequest
	names := map[id.UUID]string{}
	for rows.Next() {
		var p PendingRequest
		var name string
		if err := rows.Scan(&p.RequestID, &p.Requester, &p.Target,
			&p.CreatedAt, &p.ExpiresAt, &name); err != nil {
			return nil, nil, err
		}
		names[p.Requester] = name
		out = append(out, p)
	}
	return out, names, rows.Err()
}

// ListOutgoing returns live PENDING requests the character sent.
func (s *Store) ListOutgoing(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, now time.Time) ([]PendingRequest, map[id.UUID]string, error) {
	rows, err := s.q(tx).Query(ctx,
		`SELECT r.friend_request_id, r.requester_character_id,
		        r.target_character_id, r.created_at, r.expires_at,
		        c.name
		   FROM friend_requests r
		   JOIN characters c ON c.character_id = r.target_character_id
		  WHERE r.requester_character_id = $1
		    AND r.state = 'PENDING' AND r.expires_at > $2
		  ORDER BY r.created_at`, characterID[:], now)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []PendingRequest
	names := map[id.UUID]string{}
	for rows.Next() {
		var p PendingRequest
		var name string
		if err := rows.Scan(&p.RequestID, &p.Requester, &p.Target,
			&p.CreatedAt, &p.ExpiresAt, &name); err != nil {
			return nil, nil, err
		}
		names[p.Target] = name
		out = append(out, p)
	}
	return out, names, rows.Err()
}

// ListBlocks returns the character's directional block list.
func (s *Store) ListBlocks(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) ([]BlockRow, error) {
	rows, err := s.q(tx).Query(ctx,
		`SELECT b.blocked_character_id, c.name, b.created_at
		   FROM blocks b
		   JOIN characters c ON c.character_id = b.blocked_character_id
		  WHERE b.blocker_character_id = $1
		  ORDER BY b.created_at, 1`, characterID[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlockRow
	for rows.Next() {
		var r BlockRow
		var bid []byte
		if err := rows.Scan(&bid, &r.DisplayName, &r.CreatedAt); err != nil {
			return nil, err
		}
		copy(r.BlockedID[:], bid)
		out = append(out, r)
	}
	return out, rows.Err()
}
