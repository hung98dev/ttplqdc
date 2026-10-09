package cosmetics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// Store is the cosmetics row access (pool-backed outside transactions,
// tx-bound inside executors).
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// db resolves the tx when bound, else the pool (durable/chat pattern).
func (s *Store) db(tx pgx.Tx) interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
} {
	if tx != nil {
		return tx
	}
	return s.pool
}

// EntitlementRow is one persisted grant-source row.
type EntitlementRow struct {
	CosmeticID string
	SourceKind string
	SourceRef  string
}

// CharacterEntitlements returns every character-scoped grant row.
func (s *Store) CharacterEntitlements(ctx context.Context, tx pgx.Tx,
	char id.UUID) ([]EntitlementRow, error) {
	rows, err := s.db(tx).Query(ctx,
		`SELECT cosmetic_id, source_kind, source_ref
		 FROM character_cosmetic_entitlements
		 WHERE character_id = $1`, char[:])
	if err != nil {
		return nil, fmt.Errorf("cosmetics: char entitlements: %w", err)
	}
	defer rows.Close()
	var out []EntitlementRow
	for rows.Next() {
		var r EntitlementRow
		if err := rows.Scan(&r.CosmeticID, &r.SourceKind, &r.SourceRef); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AccountEntitlements returns every account-entitled cosmetic id
// (account_cosmetic_entitlements joins account_iap_entitlements to
// exclude rejected/refunded grants).
func (s *Store) AccountEntitlements(ctx context.Context, tx pgx.Tx,
	acct id.UUID) ([]string, error) {
	rows, err := s.db(tx).Query(ctx,
		`SELECT ace.cosmetic_id
		 FROM account_cosmetic_entitlements ace
		 JOIN account_iap_entitlements e
		   ON e.entitlement_id = ace.entitlement_id
		 WHERE ace.account_id = $1 AND e.grant_state = 'GRANTED'`, acct[:])
	if err != nil {
		return nil, fmt.Errorf("cosmetics: account entitlements: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Owned reports whether the character owns the cosmetic through any
// grant row — character-scoped rows first, then the account set
// (ADR-0060: ownership while any row exists).
func (s *Store) Owned(ctx context.Context, tx pgx.Tx, acct, char id.UUID,
	cosmeticID string) (bool, error) {
	var n int
	if err := s.db(tx).QueryRow(ctx,
		`SELECT (
		    SELECT count(*) FROM character_cosmetic_entitlements
		     WHERE character_id = $1 AND cosmetic_id = $2
		 ) + (
		    SELECT count(*) FROM account_cosmetic_entitlements ace
		    JOIN account_iap_entitlements e
		      ON e.entitlement_id = ace.entitlement_id
		     WHERE ace.account_id = $3 AND ace.cosmetic_id = $2
		       AND e.grant_state = 'GRANTED'
		 )`, char[:], cosmeticID, acct[:]).Scan(&n); err != nil {
		return false, fmt.Errorf("cosmetics: owned: %w", err)
	}
	return n > 0, nil
}

// accountOf resolves the character's owning account.
func (s *Store) accountOf(ctx context.Context, tx pgx.Tx,
	char id.UUID) (id.UUID, error) {
	var acct id.UUID
	err := s.db(tx).QueryRow(ctx,
		`SELECT account_id FROM characters WHERE character_id = $1`,
		char[:]).Scan(&acct)
	return acct, err
}

// Equips returns the persisted slot → cosmetic_id map.
func (s *Store) Equips(ctx context.Context, tx pgx.Tx,
	char id.UUID) (map[string]string, error) {
	rows, err := s.db(tx).Query(ctx,
		`SELECT slot, cosmetic_id FROM character_cosmetic_equips
		 WHERE character_id = $1`, char[:])
	if err != nil {
		return nil, fmt.Errorf("cosmetics: equips: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var slot, c string
		if err := rows.Scan(&slot, &c); err != nil {
			return nil, err
		}
		out[slot] = c
	}
	return out, rows.Err()
}
