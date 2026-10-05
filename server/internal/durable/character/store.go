package character

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// MaxPerAccount is the live-character cap (character.md, messages.md
// character_slots = 3). No slot product exists.
const MaxPerAccount = 3

var (
	// ErrNotFound is returned when a lookup finds no characters row.
	ErrNotFound = errors.New("character: not found")
	// ErrNameTaken maps the UNIQUE name_key collision (23505).
	ErrNameTaken = errors.New("character: name taken")
)

// DBTX is the query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store owns row access for the characters table. Permanence is
// structural: no delete method exists and accounts FK RESTRICT forbids
// removing an account that owns rows.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// Option configures the store.
type Option func(*Store)

// WithClock overrides wall-clock reads (tests advance server tx time).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

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

// Row is one characters row.
type Row struct {
	CharacterID  id.UUID
	AccountID    id.UUID
	Name         string
	NameKey      string
	ClassID      string
	Level        int32
	MapID        string
	CheckpointID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const rowCols = `character_id, account_id, name, name_key, class_id, level,
	map_id, checkpoint_id, created_at, updated_at`

func scanRow(r pgx.Row) (Row, error) {
	var c Row
	err := r.Scan(&c.CharacterID, &c.AccountID, &c.Name, &c.NameKey, &c.ClassID,
		&c.Level, &c.MapID, &c.CheckpointID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// Get loads one characters row by primary key.
func (s *Store) Get(ctx context.Context, db DBTX, characterID id.UUID) (Row, error) {
	db = s.q(db)
	c, err := scanRow(db.QueryRow(ctx,
		`SELECT `+rowCols+` FROM characters WHERE character_id = $1`, characterID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	return c, err
}

// ListByAccount returns the account's characters in creation order.
func (s *Store) ListByAccount(ctx context.Context, db DBTX, accountID id.UUID) ([]Row, error) {
	db = s.q(db)
	rows, err := db.Query(ctx,
		`SELECT `+rowCols+` FROM characters WHERE account_id = $1
		 ORDER BY created_at, character_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		c, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountByAccount counts the account's characters. Callers hold the
// accounts row lock so the count serializes the <=3 check.
func (s *Store) CountByAccount(ctx context.Context, tx pgx.Tx, accountID id.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM characters WHERE account_id = $1`, accountID).Scan(&n)
	return n, err
}

// AccountStatus reads accounts.status under the account lock.
func (s *Store) AccountStatus(ctx context.Context, tx pgx.Tx, accountID id.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx,
		`SELECT status FROM accounts WHERE account_id = $1`, accountID).Scan(&status)
	return status, err
}

// Insert creates the characters row; creation defaults (level=1, exp=0,
// starter checkpoint/map, empty appearance) are DDL defaults. created_at
// and updated_at are set explicitly to the server transaction time
// (ADR-0048 — no trigger). A name_key collision maps to ErrNameTaken.
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, characterID, accountID id.UUID,
	display, nameKey, classID string) error {
	now := s.now()
	_, err := tx.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id,
		 created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$6)`,
		characterID.String(), accountID.String(), display, nameKey, classID, now)
	if isUniqueViolation(err) {
		return ErrNameTaken
	}
	return err
}

// UpdateProgression commits a characters-row mutation (level/exp/points)
// and stamps updated_at with the server transaction time (ADR-0048).
func (s *Store) UpdateProgression(ctx context.Context, tx pgx.Tx, characterID id.UUID,
	level, currentExp, skillPoints, potentialPoints int32) error {
	tag, err := tx.Exec(ctx,
		`UPDATE characters SET level = $2, current_exp = $3,
		 unspent_skill_points = $4, unspent_potential_points = $5, updated_at = $6
		 WHERE character_id = $1`,
		characterID, level, currentExp, skillPoints, potentialPoints, s.now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pge *pgconn.PgError
	return errors.As(err, &pge) && pge.Code == "23505"
}
