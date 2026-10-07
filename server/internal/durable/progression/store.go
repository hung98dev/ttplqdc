// Package progression owns the durable side of character progression
// (IMP-011): the characters progression columns, character_skill_levels,
// character_potential_allocations and character_progression_flags rows,
// and the executor/record surface for the 511–513 durable intents whose
// committed client_result is S2C_PROGRESSION_MUTATE_RESULT (514).
//
// Domain rule constants here deliberately mirror sim/progression — the
// architecture fence forbids durable importing sim, so the operation
// rules are re-expressed over row values; the sim package's tests are
// the authoritative formula check.
package progression

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// PotentialIds is the fixed row key order for
// character_potential_allocations.potential_id.
var PotentialIds = [4]string{"STR", "VIT", "INT", "AGI"}

// Progression is the row-level progression aggregate: the characters
// progression columns plus the learned-skill and allocation ledgers. The
// 515 projection renders from it.
type Progression struct {
	CharacterID            id.UUID
	AccountID              id.UUID
	ClassID                string
	MapID                  string
	Level                  int32
	CurrentExp             int32
	UnspentSkillPoints     int32
	UnspentPotentialPoints int32
	// Allocated carries allocated_points per PotentialIds index.
	Allocated     [4]int32
	Skills        map[string]int32
	Flags         map[string]struct{}
	Revision      uint64
	CommonBalance int64
}

// EarnedPotentialTotal is the derivable lifetime potential total:
// unspent plus every allocated point.
func (p Progression) EarnedPotentialTotal() int32 {
	var spent int32
	for _, v := range p.Allocated {
		spent += v
	}
	return p.UnspentPotentialPoints + spent
}

// SpentSkillPoints is Σ(level-1) over learned skills — the skill-respec
// refund basis.
func (p Progression) SpentSkillPoints() int32 {
	var spent int32
	for _, lvl := range p.Skills {
		if lvl > 1 {
			spent += lvl - 1
		}
	}
	return spent
}

// Store owns row access for the progression columns and ledger tables.
// All mutation methods run inside the caller's committing transaction —
// the executor supplies it under Store.TrustedReplay's receipt lock.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// Option configures the store.
type Option func(*Store)

// WithClock overrides wall-clock reads (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// NewStore builds the progression row store.
func NewStore(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{pool: pool, now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Read loads the full progression aggregate for one character. tx may be
// nil for read-only calls outside a transaction; callers inside the
// committing tx pass it so the read runs under the row locks.
func (s *Store) Read(ctx context.Context, tx pgx.Tx, characterID id.UUID) (Progression, error) {
	var db interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
		QueryRow(context.Context, string, ...any) pgx.Row
	}
	if tx == nil {
		db = s.pool
	} else {
		db = tx
	}
	p := Progression{
		CharacterID: characterID,
		Skills:      map[string]int32{},
		Flags:       map[string]struct{}{},
	}
	err := db.QueryRow(ctx,
		`SELECT account_id, class_id, map_id, level, current_exp,
		 unspent_skill_points, unspent_potential_points, progression_revision
		 FROM characters WHERE character_id = $1`, characterID.String()).
		Scan(&p.AccountID, &p.ClassID, &p.MapID, &p.Level, &p.CurrentExp,
			&p.UnspentSkillPoints, &p.UnspentPotentialPoints, &p.Revision)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Progression{}, ErrNotFound
	case err != nil:
		return Progression{}, err
	}

	rows, err := db.Query(ctx,
		`SELECT skill_id, level FROM character_skill_levels
		 WHERE character_id = $1 ORDER BY skill_id`, characterID.String())
	if err != nil {
		return Progression{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var skillID string
		var lvl int32
		if err := rows.Scan(&skillID, &lvl); err != nil {
			return Progression{}, err
		}
		p.Skills[skillID] = lvl
	}
	if err := rows.Err(); err != nil {
		return Progression{}, err
	}
	rows.Close()

	rows, err = db.Query(ctx,
		`SELECT potential_id, allocated_points FROM character_potential_allocations
		 WHERE character_id = $1`, characterID.String())
	if err != nil {
		return Progression{}, err
	}
	for rows.Next() {
		var pid string
		var pts int32
		if err := rows.Scan(&pid, &pts); err != nil {
			rows.Close()
			return Progression{}, err
		}
		for i, k := range PotentialIds {
			if pid == k {
				p.Allocated[i] = pts
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Progression{}, err
	}
	rows.Close()

	rows, err = db.Query(ctx,
		`SELECT flag_key FROM character_progression_flags
		 WHERE character_id = $1`, characterID.String())
	if err != nil {
		return Progression{}, err
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return Progression{}, err
		}
		p.Flags[k] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Progression{}, err
	}
	rows.Close()

	err = db.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		 WHERE character_id = $1 AND currency_id = 'currency.common'`,
		characterID.String()).Scan(&p.CommonBalance)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		p.CommonBalance = 0
	case err != nil:
		return Progression{}, err
	}
	return p, nil
}

// Write commits the characters progression columns and bumps
// progression_revision by exactly one per committed tx (data_model.md:
// one bump per mutating transaction, not per push).
func (s *Store) Write(ctx context.Context, tx pgx.Tx, characterID id.UUID,
	level, currentExp, skillPoints, potentialPoints int32) error {
	tag, err := tx.Exec(ctx,
		`UPDATE characters SET level = $2, current_exp = $3,
		 unspent_skill_points = $4, unspent_potential_points = $5,
		 progression_revision = progression_revision + 1, updated_at = $6
		 WHERE character_id = $1`,
		characterID.String(), level, currentExp, skillPoints, potentialPoints, s.now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetSkillLevel upserts one character_skill_levels row.
func (s *Store) SetSkillLevel(ctx context.Context, tx pgx.Tx, characterID id.UUID,
	skillID string, level int32) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO character_skill_levels (character_id, skill_id, level)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (character_id, skill_id) DO UPDATE SET level = $3`,
		characterID.String(), skillID, level)
	return err
}

// SetAllocation upserts one character_potential_allocations row; points
// 0 deletes the row so respec leaves no zero residue.
func (s *Store) SetAllocation(ctx context.Context, tx pgx.Tx, characterID id.UUID,
	potentialID string, points int32) error {
	if points <= 0 {
		_, err := tx.Exec(ctx,
			`DELETE FROM character_potential_allocations
			 WHERE character_id = $1 AND potential_id = $2`,
			characterID.String(), potentialID)
		return err
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO character_potential_allocations
		 (character_id, potential_id, allocated_points) VALUES ($1,$2,$3)
		 ON CONFLICT (character_id, potential_id)
		 DO UPDATE SET allocated_points = $3`,
		characterID.String(), potentialID, points)
	return err
}

// MapID reads the character's current map_id — the spatial evidence the
// edge freezes into JournalSource for respec admission.
func (s *Store) MapID(ctx context.Context, characterID id.UUID) (string, error) {
	var mapID string
	err := s.pool.QueryRow(ctx,
		`SELECT map_id FROM characters WHERE character_id = $1`,
		characterID.String()).Scan(&mapID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return mapID, err
}
