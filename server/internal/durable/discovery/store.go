package discovery

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// KindMap is the character_discoveries kind slot for a normal-world map
// first entry (world_route_catalog.md § Checkpoints/Map Metadata).
const KindMap = "MAP"

// Store owns reads and writes of character_discoveries. Rows are
// once-only by the (character_id, kind, content_key) primary key: the
// first authoritative insert commits, every later attempt for the same
// key reports already-present and never duplicates the grant.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New builds the store. now defaults to time.Now (tests pin it).
func New(pool *pgxpool.Pool, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, now: now}
}

// Record inserts one discovery row inside the caller's transaction and
// reports whether this commit is the first insert for the key (true) or a
// replay/already-discovered commit (false). The caller decides what a
// first insert grants; replay inserts commit nothing extra.
func (s *Store) Record(ctx context.Context, tx pgx.Tx, characterID id.UUID,
	kind, contentKey string, operationID id.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO character_discoveries
		 (character_id, kind, content_key, discovered_at, operation_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT DO NOTHING`,
		characterID.String(), kind, contentKey, s.now(), operationID.String())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Discovered reports whether the (character, kind, key) row already
// exists — the travel admission's discovery check.
func (s *Store) Discovered(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, kind, contentKey string) (bool, error) {
	var ok bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM character_discoveries
			WHERE character_id = $1 AND kind = $2 AND content_key = $3)`,
		characterID.String(), kind, contentKey).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
