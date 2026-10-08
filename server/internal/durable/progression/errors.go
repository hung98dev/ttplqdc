package progression

import "errors"

// Package sentinels: infrastructure/malformed-record failures (the queue
// maps non-transient errors to a terminal REJECTED receipt). Rule
// verdicts are never Go errors — they are committed JournalOutcome
// nonexecutions carrying the wire ErrorCode (ADR-0081).
var (
	// ErrMalformedRecord is a DurableCommandRecord that cannot be
	// trusted: wrong family, missing command/request, or short identity
	// fields.
	ErrMalformedRecord = errors.New("progression: malformed record")
	// ErrNotFound maps a missing characters row (deleted or never
	// created — not reachable for an attached session, kept as the
	// defensive terminal).
	ErrNotFound = errors.New("progression: character not found")
)
