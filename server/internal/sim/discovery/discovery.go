package discovery

import (
	"sync"

	"thinhthan/internal/core/id"
)

// Detector is the sim-side first-entry dedupe: one process emits at
// most one sim.discovery_settlement per (character, map) until restart.
// The durable once-only row remains the authoritative fence — a
// restarted process may re-emit and the executor still never re-grants.
// "First authoritative entry" is evaluated only after a successful
// admission: failed transfers never reach here, so they never commit.
type Detector struct {
	mu   sync.Mutex
	seen map[seenKey]struct{}
}

type seenKey struct {
	char  id.UUID
	mapID string
}

// NewDetector builds an empty detector.
func NewDetector() *Detector {
	return &Detector{seen: map[seenKey]struct{}{}}
}

// Observe records one successful authoritative entry and reports
// whether it is the first observation for (character, map) — the only
// observation that emits the settlement intent.
func (d *Detector) Observe(characterID id.UUID, mapID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	k := seenKey{char: characterID, mapID: mapID}
	if _, ok := d.seen[k]; ok {
		return false
	}
	d.seen[k] = struct{}{}
	return true
}
