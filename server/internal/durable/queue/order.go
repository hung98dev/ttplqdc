package queue

import (
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// AggKey is the per-aggregate ordering key {operation_family, owner_kind,
// owner_id}: commands sharing one key commit in submission order while
// independent keys may execute concurrently. The operation ID is not part
// of the key — it identifies the single operation for receipt/ack/dedup
// lookups, never for ordering.
type AggKey struct {
	Family    string
	OwnerKind idempotency.OwnerKind
	OwnerID   id.UUID
}

// recordKey is the identity of one admitted command
// (family, owner_id, operation_id) — the operations/receipts natural key.
type recordKey struct {
	family  string
	ownerID id.UUID
	opID    id.UUID
}

// Reference bits tracked per admitted record. Ack fires only after commit
// AND every reference is disposed.
const (
	refQueued   = 1 << iota // waiting in a lane or deferred list
	refInflight             // executing
	refJournal              // inside a FrozenInventory held by the writer
)

// recordState tracks one admitted record through its lifecycle.
type recordState struct {
	key      recordKey
	agg      AggKey
	kind     ProducerKind
	owner    idempotency.Owner
	frozen   *journalv1.DurableCommandRecord // immutable queue-owned clone
	refs     int
	resolved bool   // execution resolved (committed or terminally rejected)
	release  func() // admission-gate slot release (CLIENT), freed on dispatch
	slotHeld bool   // capacity semaphore held until full disposal
	// attempts counts consecutive transient retries for the exponential
	// backoff schedule; reset on resolution.
	attempts int
	// retryNotBefore holds redispatch until a transient retry delay elapses.
	retryNotBefore time.Time
}

// lane is one aggregate key's FIFO: only the head may be in-flight, so
// commits happen in submission order.
type lane struct {
	queue   []*recordState
	running bool // head dispatched to a worker
}
