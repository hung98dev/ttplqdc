package anti_rmt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// appendAudit inserts one SYSTEM audit_events row documenting a flag
// transition — the single audit append spec requires on each
// false→true (and true→false) edge.
func appendAudit(ctx context.Context, tx pgx.Tx, accountID id.UUID,
	action string, at time.Time, preds []Predicate) error {
	payload, err := json.Marshal(map[string]any{
		"predicates":   preds,
		"evaluated_at": at.UTC(),
	})
	if err != nil {
		return fmt.Errorf("anti_rmt: audit marshal: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_events
		 (audit_event_id, occurred_at, actor_kind, actor_id,
		  subject_account_id, subject_character_id, action, reason,
		  ticket_id, operation_id, payload)
		 VALUES ($1,$2,'SYSTEM',NULL,$3,NULL,$4,$5,NULL,NULL,$6::jsonb)`,
		id.NewV4().String(), at.UTC(), accountID.String(), action,
		"economy_behavioural_signal", string(payload))
	if err != nil {
		return fmt.Errorf("anti_rmt: audit insert: %w", err)
	}
	return nil
}
