package anti_rmt

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// RecordSettlement is the exported emission entrypoint producers call
// inside (or right after) their settlement commit once per qualifying
// economy event — direct trade (both parties), AH listing, AH proceeds
// settlement, and NPC transfers via IMP-028. It re-evaluates the
// account's three behavioural predicates at ev.At and applies the
// ECONOMY_REVIEW flag transition plus its single audit append.
//
// Callers that commit rollups in the same tx should invoke this inside
// that tx so the evaluation sees the just-written deltas; the caller
// always owns the transaction boundary.
func (s *Store) RecordSettlement(ctx context.Context, tx pgx.Tx,
	ev SettlementEvent) error {
	at := ev.At
	if at.IsZero() {
		at = s.now()
	}
	preds, err := s.evaluate(ctx, tx, ev.AccountID, at)
	if err != nil {
		return err
	}
	if err := s.applyFlag(ctx, tx, ev.AccountID, at, preds); err != nil {
		return fmt.Errorf("anti_rmt: %w", err)
	}
	return nil
}

// EvaluateAccount runs the evaluation without writing — used by tests
// and diagnostics to inspect predicate outcomes at instant at.
func (s *Store) EvaluateAccount(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, at time.Time) ([]Predicate, error) {
	return s.evaluate(ctx, tx, accountID, at)
}
