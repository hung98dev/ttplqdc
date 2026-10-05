package runtime

import (
	"context"
	"errors"
)

// Row is one query result row on the durable read surface.
type Row interface {
	Scan(dest ...any) error
}

// Rows iterates a query result on the durable read surface.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Tx is a read-only transaction handle hosts read Durable through during
// Restore — typed so global/ never imports pgx (architecture rule 2: only
// durable/stackpin/conformance/testing/migrate own SQL).
type Tx interface {
	QueryRow(ctx context.Context, query string, args ...any) Row
	Query(ctx context.Context, query string, args ...any) (Rows, error)
}

// Reader is the durable-side tx-scoped read surface, injected at
// composition (implemented inside durable/ or the test harness — the only
// places pgx may live). global/ defines the interface; it never names
// pgx itself.
type Reader interface {
	ReadTx(ctx context.Context, fn func(tx Tx) error) error
}

// DurableView is the minimal read surface hosts get at Restore: the
// bridge for submissions and a tx-scoped read helper for durable state.
// Per-family readers live in host packets, not here (plan §8.10.2).
type DurableView struct {
	bridge *DurableBridge
	reader Reader
}

// Bridge returns the submission surface (nil when no queue is wired).
func (v *DurableView) Bridge() *DurableBridge { return v.bridge }

// ReadTx runs fn inside a read-only transaction over the durable read
// surface injected at composition.
func (v *DurableView) ReadTx(ctx context.Context, fn func(tx Tx) error) error {
	if v.reader == nil {
		return errors.New("global: no durable reader configured")
	}
	return v.reader.ReadTx(ctx, fn)
}
