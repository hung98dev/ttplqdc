package atlas

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// PageRow is the persisted state of one character_atlas row.
type PageRow struct {
	PageID         string
	Counter        uint64
	ReachedTier    uint32
	CompletedAt    *time.Time
	RewardOpID     *id.UUID
	AcknowledgedAt *time.Time
}

// Store owns character_atlas / character_atlas_state rows.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore binds the store to the shared pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Page reads one row; a missing row reports zero counter/tier.
func (s *Store) Page(ctx context.Context, tx pgx.Tx, charID id.UUID, pageID string) (PageRow, error) {
	row := PageRow{PageID: pageID}
	var completedAt, ackAt *time.Time
	var rewardOp []byte
	var tier int16
	var counter int64
	err := tx.QueryRow(ctx,
		`SELECT tier, seen_count, completed_at, reward_operation_id, acknowledged_at
		 FROM character_atlas WHERE character_id=$1 AND atlas_page_id=$2`,
		charID, pageID).Scan(&tier, &counter, &completedAt, &rewardOp, &ackAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Counter = uint64(counter)
	row.ReachedTier = uint32(tier)
	row.CompletedAt = completedAt
	row.AcknowledgedAt = ackAt
	if rewardOp != nil {
		var u id.UUID
		copy(u[:], rewardOp)
		row.RewardOpID = &u
	}
	return row, nil
}

// ApplyDelta upserts the counter: ApplyCount adds delta, ApplyMax keeps
// max(counter, delta). Returns the new counter value.
func (s *Store) ApplyDelta(ctx context.Context, tx pgx.Tx, charID id.UUID, pageID string, mode ApplyMode, delta uint64) (uint64, error) {
	var next int64
	var err error
	if mode == ApplyMax {
		err = tx.QueryRow(ctx,
			`INSERT INTO character_atlas (character_id, atlas_page_id, seen_count)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (character_id, atlas_page_id)
			 DO UPDATE SET seen_count = GREATEST(character_atlas.seen_count, EXCLUDED.seen_count)
			 RETURNING seen_count`, charID, pageID, int64(delta)).Scan(&next)
	} else {
		err = tx.QueryRow(ctx,
			`INSERT INTO character_atlas (character_id, atlas_page_id, seen_count)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (character_id, atlas_page_id)
			 DO UPDATE SET seen_count = character_atlas.seen_count + EXCLUDED.seen_count
			 RETURNING seen_count`, charID, pageID, int64(delta)).Scan(&next)
	}
	return uint64(next), err
}

// Promote records tier N on the row when it exceeds the stored tier. The
// deterministic reward_operation_id of the newest promotion is kept.
func (s *Store) Promote(ctx context.Context, tx pgx.Tx, charID id.UUID, pageID string, tier uint32, opID id.UUID, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE character_atlas
		 SET tier=$3, reward_operation_id=$4, completed_at=$5
		 WHERE character_id=$1 AND atlas_page_id=$2 AND tier < $3`,
		charID, pageID, int16(tier), opID, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Acknowledge sets acknowledged_at once; returns whether it changed.
func (s *Store) Acknowledge(ctx context.Context, tx pgx.Tx, charID id.UUID, pageID string, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE character_atlas SET acknowledged_at=$3
		 WHERE character_id=$1 AND atlas_page_id=$2 AND acknowledged_at IS NULL`,
		charID, pageID, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// BumpRevision increments the character_atlas_state revision, creating the
// row on first use. Returns the new revision.
func (s *Store) BumpRevision(ctx context.Context, tx pgx.Tx, charID id.UUID) (uint64, error) {
	var rev int64
	err := tx.QueryRow(ctx,
		`INSERT INTO character_atlas_state (character_id, revision)
		 VALUES ($1,1)
		 ON CONFLICT (character_id)
		 DO UPDATE SET revision = character_atlas_state.revision + 1
		 RETURNING revision`, charID).Scan(&rev)
	return uint64(rev), err
}

// Revision reads the current revision (0 when the state row is missing).
func (s *Store) Revision(ctx context.Context, tx pgx.Tx, charID id.UUID) (uint64, error) {
	var rev int64
	err := tx.QueryRow(ctx,
		`SELECT revision FROM character_atlas_state WHERE character_id=$1`, charID).Scan(&rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return uint64(rev), err
}

// MasteredCount counts pages at their authored top tier; used for
// milestone checks.
func (s *Store) MasteredCount(ctx context.Context, tx pgx.Tx, charID id.UUID) (int, error) {
	rows, err := tx.Query(ctx,
		`SELECT atlas_page_id, tier FROM character_atlas WHERE character_id=$1`, charID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var pageID string
		var tier int16
		if err := rows.Scan(&pageID, &tier); err != nil {
			return 0, err
		}
		if _, ok := PageByID(pageID); ok && int(tier) >= len(Page{}.Thresholds) {
			count++
		}
	}
	return count, rows.Err()
}

// RecordMilestone inserts an atlas_milestones row once; returns whether it
// was newly inserted.
func (s *Store) RecordMilestone(ctx context.Context, tx pgx.Tx, charID id.UUID, milestoneID string, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO atlas_milestones (character_id, milestone_id, completed_at)
		 VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, charID, milestoneID, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// MilestonesDone returns the completed milestone ids.
func (s *Store) MilestonesDone(ctx context.Context, tx pgx.Tx, charID id.UUID) (map[string]time.Time, error) {
	rows, err := tx.Query(ctx,
		`SELECT milestone_id, completed_at FROM atlas_milestones WHERE character_id=$1`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var m string
		var at time.Time
		if err := rows.Scan(&m, &at); err != nil {
			return nil, err
		}
		out[m] = at
	}
	return out, rows.Err()
}

// GrantCosmetic inserts the presentation entitlement once per
// (character, cosmetic, source_ref); returns whether it was new.
func (s *Store) GrantCosmetic(ctx context.Context, tx pgx.Tx, charID id.UUID, cosmeticID, sourceRef string, opID id.UUID, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO character_cosmetic_entitlements
			(character_id, cosmetic_id, source_kind, source_ref, grant_operation_id, granted_at)
		 VALUES ($1,$2,'PLAY',$3,$4,$5)
		 ON CONFLICT DO NOTHING`, charID, cosmeticID, sourceRef, opID, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Snapshot reads every persisted page row for the projection.
func (s *Store) Snapshot(ctx context.Context, tx pgx.Tx, charID id.UUID) ([]PageRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT atlas_page_id, tier, seen_count, completed_at, reward_operation_id, acknowledged_at
		 FROM character_atlas WHERE character_id=$1`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageRow
	for rows.Next() {
		var r PageRow
		var completedAt, ackAt *time.Time
		var rewardOp []byte
		var tier int16
		var counter int64
		if err := rows.Scan(&r.PageID, &tier, &counter, &completedAt, &rewardOp, &ackAt); err != nil {
			return nil, err
		}
		r.Counter = uint64(counter)
		r.ReachedTier = uint32(tier)
		r.CompletedAt = completedAt
		r.AcknowledgedAt = ackAt
		if rewardOp != nil {
			var u id.UUID
			copy(u[:], rewardOp)
			r.RewardOpID = &u
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
