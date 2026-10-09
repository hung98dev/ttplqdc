package anti_rmt

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
)

// evalT is a fixed evaluation instant mid-UTC-day so both boundary
// partial ranges exist (oldest + current).
func evalT() time.Time {
	return time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
}

// flagTrigger seeds a partner-concentration qualifying state for acct:
// one whole-day rollup row sending 5M common to a single partner.
func flagTrigger(t *testing.T, acct id.UUID) id.UUID {
	t.Helper()
	char := mkCharacter(t, acct, 60, 72*time.Hour)
	partner := id.NewV4()
	seedCharRollup(t, char, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		5_000_000, 0, map[string]int64{partner.String(): 5_000_000}, nil)
	return char
}

// TestReviewFlagSetAndCleared asserts ECONOMY_REVIEW transitions: set
// once with a single audit append on false→true, no re-append while
// flagged, cleared when every predicate is false with its own audit
// append (anti_cheat.md § ECONOMY_REVIEW).
func TestReviewFlagSetAndCleared(t *testing.T) {
	s := New(pool(t))
	acct := mkAccount(t)
	flagTrigger(t, acct)
	at := evalT()

	record(t, s, acct, at)
	if !flagOf(t, acct) {
		t.Fatal("flag not set after qualifying predicate")
	}
	if n := auditCount(t, acct, ActionReviewSet); n != 1 {
		t.Fatalf("set audits = %d, want 1", n)
	}

	// Re-evaluation while flagged appends nothing.
	record(t, s, acct, at.Add(time.Hour))
	if n := auditCount(t, acct, ActionReviewSet); n != 1 {
		t.Fatalf("re-eval set audits = %d, want 1", n)
	}

	// Window slides past the seeded day: all predicates false -> clear.
	later := at.AddDate(0, 0, 40)
	record(t, s, acct, later)
	if flagOf(t, acct) {
		t.Fatal("flag not cleared when all predicates false")
	}
	if n := auditCount(t, acct, ActionReviewCleared); n != 1 {
		t.Fatalf("clear audits = %d, want 1", n)
	}
	if n := auditCount(t, acct, ActionReviewSet); n != 1 {
		t.Fatalf("total set audits = %d, want 1", n)
	}
}

// TestReviewFlagBlocksNothing asserts ADR-0065 semantics: the flag is a
// review queue entry only — a flagged account keeps emitting
// settlements without rejection and the flag never mutates status.
func TestReviewFlagBlocksNothing(t *testing.T) {
	s := New(pool(t))
	acct := mkAccount(t)
	char := flagTrigger(t, acct)
	at := evalT()
	record(t, s, acct, at)
	if !flagOf(t, acct) {
		t.Fatal("precondition: flag set")
	}

	// The flagged account still records further settlements normally.
	partner := mkAccount(t)
	partnerChar := mkCharacter(t, partner, 60, 72*time.Hour)
	seedTrade(t, char, acct, partnerChar, partner, at.Add(-time.Hour),
		1_000, 0, nil)
	record(t, s, acct, at.Add(2*time.Hour))
	if !flagOf(t, acct) {
		t.Fatal("flag lost while predicate still qualifies")
	}
	var status string
	if err := pool(t).QueryRow(ctx0(),
		`SELECT status FROM accounts WHERE account_id=$1`,
		acct.String()).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "ACTIVE" {
		t.Fatalf("flag mutated account status to %q", status)
	}
}
