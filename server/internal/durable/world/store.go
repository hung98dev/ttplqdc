package world

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// DBTX is the query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store owns the world-domain characters fields (checkpoint_id, map_id)
// and the world_consequence_relics reads a starting partition needs.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// Option configures the store.
type Option func(*Store)

// WithClock overrides wall-clock reads (tests advance server time).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// NewStore builds the world durable store over the shared pool.
func NewStore(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{pool: pool, now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// q normalizes a nil DBTX to the pool for non-tx calls.
func (s *Store) q(db DBTX) DBTX {
	if db == nil {
		return s.pool
	}
	return db
}

// CheckpointRow is the characters placement state this domain reads.
type CheckpointRow struct {
	CharacterID  id.UUID
	CheckpointID string
	MapID        string
}

// Checkpoint loads the character's checkpoint_id and map_id.
func (s *Store) Checkpoint(ctx context.Context, db DBTX, characterID id.UUID) (CheckpointRow, error) {
	db = s.q(db)
	var r CheckpointRow
	err := db.QueryRow(ctx,
		`SELECT character_id, checkpoint_id, map_id FROM characters WHERE character_id = $1`,
		characterID).Scan(&r.CharacterID, &r.CheckpointID, &r.MapID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckpointRow{}, ErrNotFound
	}
	return r, err
}

// SetCheckpoint writes the active respawn checkpoint
// (world_route_catalog checkpoint.<region>.<anchor>).
func (s *Store) SetCheckpoint(ctx context.Context, tx pgx.Tx, characterID id.UUID, checkpointID string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE characters SET checkpoint_id = $2, updated_at = $3 WHERE character_id = $1`,
		characterID, checkpointID, s.now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMap writes the character's resident map on transfer completion /
// respawn placement / recovery.
func (s *Store) SetMap(ctx context.Context, tx pgx.Tx, characterID id.UUID, mapID string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE characters SET map_id = $2, updated_at = $3 WHERE character_id = $1`,
		characterID, mapID, s.now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsequenceRow is one plain world_consequence_relics row — durable/
// may not import sim/, so sim/world's loader adapter maps it to
// runtime.WorldConsequence itself.
type ConsequenceRow struct {
	RelicID     string
	SourceID    string
	Active      bool
	BuffEffectID string
	SpawnedAt   time.Time
	ExpiresAt   time.Time
}

// LoadConsequences returns every world_consequence_relics row for the
// (map_id, channel_id) partition key (ADR-0040 stable key); the caller
// filters Active && ExpiresAt > now per sharding.md.
func (s *Store) LoadConsequences(ctx context.Context, mapID string, channelID uint64) ([]ConsequenceRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT relic_id, source_id, relic_active, buff_effect_id, spawned_at, expires_at
		 FROM world_consequence_relics
		 WHERE map_id = $1 AND channel_id = $2
		 ORDER BY relic_id`, mapID, int16(channelID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConsequenceRow
	for rows.Next() {
		var r ConsequenceRow
		if err := rows.Scan(&r.RelicID, &r.SourceID, &r.Active, &r.BuffEffectID,
			&r.SpawnedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordAttachEvent appends one character_attach_events row for attach
// bookkeeping (save_rules.md § sim.checkpoint — transfer/instance
// membership boundaries).
func (s *Store) RecordAttachEvent(ctx context.Context, tx pgx.Tx, characterID id.UUID, sessionEpoch uint64, attachedAt time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO character_attach_events (character_id, session_epoch, attached_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (character_id, session_epoch) DO NOTHING`,
		characterID, int64(sessionEpoch), attachedAt)
	return err
}
