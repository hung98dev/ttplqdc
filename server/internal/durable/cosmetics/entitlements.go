package cosmetics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// Source kinds mirror the schema CHECK on
// character_cosmetic_entitlements.source_kind.
const (
	SourcePlay        = "PLAY"
	SourceRedemption  = "REDEMPTION"
	SourceSeasonTrack = "SEASON_TRACK"
)

// Grant inserts one grant-source row. The durable primary key
// (character_id, cosmetic_id, source_ref) makes a re-submitted source
// a committed no-op — ownership requires any row, never all rows
// (cosmetics.md § Identity / Ownership). Season-track grants carry the
// source entitlement id so a track revoke can scope exactly.
func (s *Store) Grant(ctx context.Context, tx pgx.Tx, char id.UUID,
	cosmeticID, sourceKind, sourceRef string, sourceEntitlement *id.UUID,
	opID id.UUID, now time.Time) error {
	var src []byte
	if sourceEntitlement != nil {
		src = sourceEntitlement[:]
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO character_cosmetic_entitlements
		    (character_id, cosmetic_id, source_kind,
		     source_entitlement_id, source_ref, grant_operation_id,
		     granted_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (character_id, cosmetic_id, source_ref) DO NOTHING`,
		char[:], cosmeticID, sourceKind, src, sourceRef, opID[:], now)
	return err
}

// RevokeBySourceEntitlement removes every character-scoped grant tied
// to one account entitlement (a refunded/rejected season track).
// Rows granted by other sources survive — ownership persists while
// any row exists (cosmetics.md §15 + data_model.md).
func (s *Store) RevokeBySourceEntitlement(ctx context.Context, tx pgx.Tx,
	sourceEntitlement id.UUID) (int64, error) {
	tag, err := s.db(tx).Exec(ctx,
		`DELETE FROM character_cosmetic_entitlements
		 WHERE source_entitlement_id = $1`, sourceEntitlement[:])
	if err != nil {
		return 0, fmt.Errorf("cosmetics: revoke: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GrantSourceExists reports whether a concrete grant-source row
// already exists (idempotent re-delivery of a source grant).
func (s *Store) GrantSourceExists(ctx context.Context, tx pgx.Tx,
	char id.UUID, cosmeticID, sourceRef string) (bool, error) {
	var n int
	if err := s.db(tx).QueryRow(ctx,
		`SELECT count(*) FROM character_cosmetic_entitlements
		 WHERE character_id = $1 AND cosmetic_id = $2 AND source_ref = $3`,
		char[:], cosmeticID, sourceRef).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
