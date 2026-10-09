package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// ReportRetention is the 3-year deadline from created_at for every
// player_reports row — including OPEN cases with no automatic
// extension (personal_data_register § 1 H / data_model.md §
// player_reports).
const ReportRetention = 3 * 365 * 24 * time.Hour

// Case is the full moderation-case row as persisted — every registered
// evidence field (PRIV-004): the report retains original reporter
// ownership with no account FK, and no chat/operator join or OPEN
// status changes its retention.
type Case struct {
	ReportID            id.UUID
	OperationID         id.UUID
	ReporterAccountID   id.UUID
	ReporterCharacterID id.UUID
	TargetCharacterID   id.UUID
	Reason              string
	ChatMessageID       *id.UUID
	ReporterNotes       *string
	CreatedAt           time.Time
	Status              string
	ResolvedAt          *time.Time
	HandledByOperatorID *id.UUID
	ResolutionCode      *string
}

// ReporterExport is the canonical reporter-own projection — the only
// shape a player data export may surface (data_protection.md § Access
// and Portability / personal_data_register § 1.1: reporter-own
// projection only; never target-owned, investigator or joined-evidence
// data). Operator attribution is deliberately absent.
type ReporterExport struct {
	ReportID      id.UUID
	Reason        string
	ChatMessageID *id.UUID
	ReporterNotes *string
	CreatedAt     time.Time
	Status        string
	ResolvedAt    *time.Time
}

// Reports reads case state from player_reports. Writes stay owned by
// durable/social (the C2S_REPORT_PLAYER durable exec) — this store adds
// only the retention/evidence queries the case rules need.
type Reports struct {
	pool *pgxpool.Pool
}

// NewReports constructs the case store.
func NewReports(pool *pgxpool.Pool) *Reports {
	return &Reports{pool: pool}
}

type qx interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *Reports) q(tx pgx.Tx) qx {
	if tx != nil {
		return tx
	}
	return r.pool
}

// OpenForCharacter lists OPEN cases whose reporter is `characterID` —
// the moderation queue view (no join to chat content or operator rows).
func (r *Reports) OpenForCharacter(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) ([]Case, error) {
	rows, err := r.q(tx).Query(ctx,
		`SELECT report_id, operation_id, reporter_account_id,
		        reporter_character_id, target_character_id, reason,
		        chat_message_id, reporter_notes, created_at, status,
		        resolved_at, handled_by_operator_id, resolution_code
		   FROM player_reports
		  WHERE reporter_character_id = $1 AND status = 'OPEN'
		  ORDER BY created_at`, characterID[:])
	if err != nil {
		return nil, fmt.Errorf("moderation: open cases: %w", err)
	}
	defer rows.Close()
	var out []Case
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CaseByID fetches the full evidence row — the row survives reporter
// erasure unchanged (PRIV-004), so the fields returned here are the
// retained canonical set.
func (r *Reports) CaseByID(ctx context.Context, tx pgx.Tx,
	reportID id.UUID) (Case, bool, error) {
	rows, err := r.q(tx).Query(ctx,
		`SELECT report_id, operation_id, reporter_account_id,
		        reporter_character_id, target_character_id, reason,
		        chat_message_id, reporter_notes, created_at, status,
		        resolved_at, handled_by_operator_id, resolution_code
		   FROM player_reports WHERE report_id = $1`, reportID[:])
	if err != nil {
		return Case{}, false, fmt.Errorf("moderation: case: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return Case{}, false, rows.Err()
	}
	c, err := scanCase(rows)
	return c, err == nil, err
}

// ExportForAccount is the player-access shape — only the reporter-own
// projection of that account's own reports.
func (r *Reports) ExportForAccount(ctx context.Context, tx pgx.Tx,
	accountID id.UUID) ([]ReporterExport, error) {
	rows, err := r.q(tx).Query(ctx,
		`SELECT report_id, reason, chat_message_id, reporter_notes,
		        created_at, status, resolved_at
		   FROM player_reports
		  WHERE reporter_account_id = $1
		  ORDER BY created_at`, accountID[:])
	if err != nil {
		return nil, fmt.Errorf("moderation: reporter export: %w", err)
	}
	defer rows.Close()
	var out []ReporterExport
	for rows.Next() {
		var e ReporterExport
		var chatRef []byte
		var notes *string
		var resolved *time.Time
		if err := rows.Scan(&e.ReportID, &e.Reason, &chatRef,
			&notes, &e.CreatedAt, &e.Status, &resolved); err != nil {
			return nil, err
		}
		e.ReporterNotes = notes
		e.ResolvedAt = resolved
		if len(chatRef) == 16 {
			var m id.UUID
			copy(m[:], chatRef)
			e.ChatMessageID = &m
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PurgeExpired deletes up to `bound` reports past the 3-year deadline —
// one bounded batch of the daily purge worker. OPEN cases get no
// extension: an OPEN row whose created_at crosses the deadline purges
// like any other (personal_data_register § 1 H).
func (r *Reports) PurgeExpired(ctx context.Context, tx pgx.Tx,
	now time.Time, bound int) (int64, error) {
	tag, err := r.q(tx).Exec(ctx,
		`DELETE FROM player_reports WHERE ctid IN (
		    SELECT ctid FROM player_reports
		     WHERE created_at <= $1
		     LIMIT $2)`,
		now.Add(-ReportRetention), bound)
	if err != nil {
		return 0, fmt.Errorf("moderation: purge reports: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanCase(rows pgx.Rows) (Case, error) {
	var c Case
	var opIDraw, chatRaw, opRaw []byte
	var notes, resCode *string
	var resolved *time.Time
	if err := rows.Scan(&c.ReportID, &opIDraw, &c.ReporterAccountID,
		&c.ReporterCharacterID, &c.TargetCharacterID, &c.Reason,
		&chatRaw, &notes, &c.CreatedAt, &c.Status, &resolved,
		&opRaw, &resCode); err != nil {
		return c, err
	}
	copy(c.OperationID[:], opIDraw)
	if len(chatRaw) == 16 {
		var m id.UUID
		copy(m[:], chatRaw)
		c.ChatMessageID = &m
	}
	c.ReporterNotes = notes
	c.ResolvedAt = resolved
	if len(opRaw) == 16 {
		var o id.UUID
		copy(o[:], opRaw)
		c.HandledByOperatorID = &o
	}
	c.ResolutionCode = resCode
	return c, nil
}
