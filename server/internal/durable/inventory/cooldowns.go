package inventory

import (
	"sync"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
)

// tickMillis is the partition-tick duration (simulation 20 Hz).
const tickMillis = 50

// CooldownTracker is the in-memory shared-cooldown ledger (items.md §
// Consumables: no persistence requirement) keyed (character_id, group).
// One instance is shared between the edge admission check and the
// durable executors inside one process (composition injects the same
// tracker into both).
type CooldownTracker struct {
	mu      sync.Mutex
	now     func() time.Time
	wallEnd map[id.UUID]map[items.CooldownGroup]time.Time
}

// NewCooldownTracker builds a tracker; now defaults to wall clock.
func NewCooldownTracker(now func() time.Time) *CooldownTracker {
	if now == nil {
		now = func() time.Time { return time.Now() }
	}
	return &CooldownTracker{now: now, wallEnd: map[id.UUID]map[items.CooldownGroup]time.Time{}}
}

// AssertAllowed reports whether (charID, group) may consume now.
// endsAt is the wall-time the group unblocks when the check fails —
// callers map it to COOLDOWN_ACTIVE.
func (t *CooldownTracker) AssertAllowed(charID id.UUID, group items.CooldownGroup) (endsAt time.Time, ok bool) {
	if group == "" || group == items.CooldownNone {
		return time.Time{}, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	end, set := t.wallEnd[charID][group]
	if set && t.now().Before(end) {
		return end, false
	}
	return time.Time{}, true
}

// RecordUse commits a consumed use: the group's wall-end is now +
// duration when it extends the current end (a later shorter use never
// shortens an in-flight longer cooldown).
func (t *CooldownTracker) RecordUse(charID id.UUID, group items.CooldownGroup, durationS float64) {
	if group == "" || group == items.CooldownNone || durationS <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.wallEnd[charID]
	if m == nil {
		m = map[items.CooldownGroup]time.Time{}
		t.wallEnd[charID] = m
	}
	end := t.now().Add(time.Duration(durationS * float64(time.Second)))
	if cur, set := m[group]; set && end.Before(cur) {
		return
	}
	m[group] = end
}

// EndsAtTick converts the remaining wall duration of (charID, group)
// to a destination partition tick: tick + ceil(remaining/50ms)
// (messages.md remaining-duration → destination-tick rule). The tick
// comes from the ADR-0083 PartitionTick consult frozen into the record
// at admission — never fabricated.
func (t *CooldownTracker) EndsAtTick(charID id.UUID, group items.CooldownGroup, tick uint64) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	end, set := t.wallEnd[charID][group]
	if !set {
		return tick
	}
	rem := end.Sub(t.now())
	if rem <= 0 {
		return tick
	}
	return tick + uint64(rem.Milliseconds()+tickMillis-1)/tickMillis
}
