package anti_rmt

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// Window sizes in elapsed 24-hour days (anti_cheat.md § Economy
// Behavioural Signals).
const (
	WindowDays7  = 7
	WindowDays30 = 30
)

// Predicate thresholds — integer-only comparisons per spec.
const (
	NetOutflowFlagCommon  int64 = 20_000_000
	PartnerTotalMinCommon int64 = 5_000_000
	ConcentrationNum            = int64(5) // 5*top3 >= 4*total
	ConcentrationDen            = int64(4)
	ReceivedItemsMin      int64 = 50
)

// Store reads the economy rollups/raw settlement tables and writes
// the ECONOMY_REVIEW flag + audit transitions.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New returns a Store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, now: func() time.Time { return time.Now().UTC() }}
}

// WithNow pins the evaluation clock (tests).
func (s *Store) WithNow(now func() time.Time) *Store {
	s.now = now
	return s
}

// SettlementEvent is one qualifying economy commit handed to
// RecordSettlement by its producer: direct trade (both parties), AH
// listing, AH proceeds settlement, NPC transfer (ADR-0041 export API —
// IMP-028 binds NPC producers later).
type SettlementEvent struct {
	AccountID id.UUID
	At        time.Time
}

// Predicate is one economy behavioural signal outcome.
type Predicate struct {
	Name      string
	Qualified bool
	Evidence  map[string]any
}
