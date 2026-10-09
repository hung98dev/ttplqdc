package anti_rmt

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// audit actions appended on flag transitions (anti_cheat.md §
// ECONOMY_REVIEW, ADR-0065: exactly one audit append per transition).
const (
	ActionReviewSet     = "ECONOMY_REVIEW_SET"
	ActionReviewCleared = "ECONOMY_REVIEW_CLEARED"
)

// applyFlag sets the flag when any predicate qualified and clears it
// when all three are false; each transition appends exactly one audit
// row. While flagged, re-evaluations append nothing.
func (s *Store) applyFlag(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, at time.Time, preds []Predicate) error {
	// The accounts row lock was already taken by evaluate() (priority
	// 10 precedes EconomyRollups 200).
	var flagged bool
	var flaggedAt *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT economy_review_flagged_at FROM accounts
		 WHERE account_id=$1`, accountID.String()).Scan(&flaggedAt); err != nil {
		return fmt.Errorf("anti_rmt: flag read: %w", err)
	}
	flagged = flaggedAt != nil
	any := false
	for _, p := range preds {
		if p.Qualified {
			any = true
			break
		}
	}
	switch {
	case any && !flagged:
		res, err := tx.Exec(ctx,
			`UPDATE accounts SET economy_review_flagged_at=$2
			 WHERE account_id=$1 AND economy_review_flagged_at IS NULL`,
			accountID.String(), at)
		if err != nil {
			return fmt.Errorf("anti_rmt: flag set: %w", err)
		}
		if res.RowsAffected() == 1 {
			return appendAudit(ctx, tx, accountID, ActionReviewSet, at, preds)
		}
	case !any && flagged:
		res, err := tx.Exec(ctx,
			`UPDATE accounts SET economy_review_flagged_at=NULL
			 WHERE account_id=$1 AND economy_review_flagged_at IS NOT NULL`,
			accountID.String())
		if err != nil {
			return fmt.Errorf("anti_rmt: flag clear: %w", err)
		}
		if res.RowsAffected() == 1 {
			return appendAudit(ctx, tx, accountID, ActionReviewCleared, at, preds)
		}
	}
	return nil
}
