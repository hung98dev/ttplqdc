package cosmetics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// Increment applies one authoritative counter event for a cumulative
// feat (KILL_COUNT / BOSS_KILL_COUNT / GATHER_COUNT): the server-owned
// counter advances by n inside the caller's tx; reaching the authored
// threshold inserts the milestone row and grants the cosmetic in the
// same transaction. The milestone PK (character_id, feat_id,
// milestone_threshold) makes a retried grant a committed no-op
// (cosmetics.md § Folklore Feats Tracking). Returns whether the
// milestone committed on this call.
//
// Event-owning tasks call this from their own durable executors —
// counters never accept client input (cosmetics.md §109).
func (s *Store) Increment(ctx context.Context, tx pgx.Tx, char id.UUID,
	featID string, n int64, opID id.UUID, now time.Time) (bool, error) {
	f, ok := FeatByID(featID)
	if !ok || f.Flag {
		return false, ErrUnknownCosmetic
	}
	var counter int64
	err := tx.QueryRow(ctx,
		`INSERT INTO character_feats (character_id, feat_id, counter_value)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (character_id, feat_id)
		 DO UPDATE SET counter_value = character_feats.counter_value + $3
		 RETURNING counter_value`, char[:], featID, n).Scan(&counter)
	if err != nil {
		return false, err
	}
	if counter < f.Threshold {
		return false, nil
	}
	return s.completeMilestone(ctx, tx, char, f, opID, now)
}

// Flag latches a boolean feat (ENHANCEMENT_FLAG / PVP_RANK_FLAG) once
// the qualifying event commits: the counter goes to the flag value
// and the milestone + cosmetic grant commit atomically. Repeated
// qualifying events are no-ops — flags never clear
// (cosmetic_catalog.md § Feat Counter Rules).
func (s *Store) Flag(ctx context.Context, tx pgx.Tx, char id.UUID,
	featID string, reached int64, opID id.UUID, now time.Time) (bool, error) {
	f, ok := FeatByID(featID)
	if !ok || !f.Flag {
		return false, ErrUnknownCosmetic
	}
	if reached < f.Threshold {
		return false, nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO character_feats (character_id, feat_id, counter_value)
		 VALUES ($1,$2,1)
		 ON CONFLICT (character_id, feat_id) DO NOTHING`,
		char[:], featID); err != nil {
		return false, err
	}
	return s.completeMilestone(ctx, tx, char, f, opID, now)
}

// completeMilestone inserts the milestone row and the cosmetic grant
// in the same tx. ON CONFLICT DO NOTHING on both sides makes a replay
// of the same operation or a re-qualifying event a safe no-op that
// never double-grants (cosmetics.md §101-106).
func (s *Store) completeMilestone(ctx context.Context, tx pgx.Tx,
	char id.UUID, f FeatDef, opID id.UUID, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO character_feat_milestones
		    (character_id, feat_id, milestone_threshold,
		     completed_at, reward_operation_id)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (character_id, feat_id, milestone_threshold) DO NOTHING`,
		char[:], f.ID, f.Threshold, now, opID[:])
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil // already granted
	}
	if err := s.Grant(ctx, tx, char, f.Reward, SourcePlay,
		"feat."+f.ID, nil, opID, now); err != nil {
		return false, err
	}
	return true, nil
}
