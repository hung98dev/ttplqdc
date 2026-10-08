package equipment

import (
	"time"

	"thinhthan/internal/core/id"
)

// SwitchCooldownS is the successful-loadout-switch cooldown
// (equipment.md § Loadout Switch): 3 s wall-clock.
const SwitchCooldownS = 3

// SwitchTracker is the in-memory last-successful-switch ledger keyed by
// character. Runtime gate state, never persisted (equipment.md §
// Persistence persists loadout state, not cooldowns). Not goroutine
// safe: callers run under the character's single-writer admission.
type SwitchTracker struct {
	last map[id.UUID]time.Time
	now  func() time.Time
}

// NewSwitchTracker builds a tracker on the injected clock (tests stub
// it; production uses time.Now).
func NewSwitchTracker(now func() time.Time) *SwitchTracker {
	return &SwitchTracker{last: map[id.UUID]time.Time{}, now: now}
}

// Ready reports whether a SWITCH_ACTIVE may commit now. The first
// switch after character load is always ready (no prior success).
func (t *SwitchTracker) Ready(charID id.UUID) bool {
	prev, ok := t.last[charID]
	return !ok || t.now().Sub(prev) >= SwitchCooldownS*time.Second
}

// ReadyIn returns the remaining cooldown; 0 when ready.
func (t *SwitchTracker) ReadyIn(charID id.UUID) time.Duration {
	prev, ok := t.last[charID]
	if !ok {
		return 0
	}
	if d := SwitchCooldownS*time.Second - t.now().Sub(prev); d > 0 {
		return d
	}
	return 0
}

// RecordSuccess stamps a committed switch. Admission calls this only
// after the durable commit reports success.
func (t *SwitchTracker) RecordSuccess(charID id.UUID) {
	t.last[charID] = t.now()
}
