package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// auditPayload is the JSONB body recorded per committed mutation
// (economy.md § Audit).
type auditPayload struct {
	CurrencyID string `json:"currency_id"`
	Delta      int64  `json:"delta"`
	Before     int64  `json:"balance_before"`
	After      int64  `json:"balance_after"`
	SourceRef  string `json:"source_reference"`
}

// writeAudit appends one audit_events row per committed mutation.
// Append-only table — no lockorder entry.
func writeAudit(ctx context.Context, tx pgx.Tx, m Mutation, res Result) error {
	payload, err := json.Marshal(auditPayload{
		CurrencyID: string(m.CurrencyID),
		Delta:      m.Delta,
		Before:     res.Before,
		After:      res.After,
		SourceRef:  m.SourceRef,
	})
	if err != nil {
		return fmt.Errorf("currency: audit marshal: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_events
		 (audit_event_id, occurred_at, actor_kind, actor_id,
		  subject_account_id, subject_character_id, action, reason,
		  ticket_id, operation_id, payload)
		 VALUES ($1,$2,$3,NULL,NULL,$4,$5,NULL,NULL,$6,$7::jsonb)`,
		id.NewV4().String(), time.Now().UTC(), string(m.Actor),
		m.CharacterID.String(), m.ReasonCode, m.OperationID.String(), string(payload))
	if err != nil {
		return fmt.Errorf("currency: audit insert: %w", err)
	}
	return nil
}
