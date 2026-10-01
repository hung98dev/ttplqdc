// Package db owns the PostgreSQL connection pool and the canonical lock
// ordering of database.md §5. It is the only package that constructs a
// pgxpool for gameplay persistence; sim and edge never touch the database.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Canonical lock priorities (database.md §5). Every transaction that touches
// more than one aggregate locks rows in ascending priority; inside a table
// rows lock in ascending ID order (physical_schema_contract.md §5).
const (
	// LockPrioritySeasonFinalization serializes season boundary work
	// (priority 0): match admission, cutoff transition, settlement drain
	// and lazy new-season reset.
	LockPrioritySeasonFinalization = 0
	// LockPriorityAccounts covers accounts and their auth rows
	// (priority 1).
	LockPriorityAccounts = 1
	// LockPriorityCharacters covers characters and their owned
	// projections (priority 2).
	LockPriorityCharacters = 2
	// LockPriorityReceipts covers durable_command_receipts
	// (priority 2.5): after account/character locks, before value
	// aggregate locks. Value 25 = 2.5 in tenths.
	LockPriorityReceipts = 25
	// LockPriorityValueAggregates starts the value-lock band
	// (priorities 3-20 in tenths: 30-200).
	LockPriorityValueAggregates = 30
	// LockPriorityRegionMarker is the shared region_di_tich_markers
	// lock (database.md priority 18, in tenths = 180), always before any
	// world_consequence_relics channel row.
	LockPriorityRegionMarker = 180
	// LockPriorityOperations is the trailing operations insert
	// (database.md: the operations row is inserted last).
	LockPriorityOperations = 1000
)

// Pool configures a pgxpool for the one-world single-process PostgreSQL
// (ADR-0052). database.md: bounded pool, parameterized queries only,
// READ COMMITTED default.
func Pool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: invalid DSN: %w", err)
	}
	cfg.MaxConns = 8
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}
	return pool, nil
}
