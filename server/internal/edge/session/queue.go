// Package session owns the in-memory session state of session.md:
// session epochs, the gameplay-ticket / resume-credential machines, the
// login queue, live-character attach state and the listener adapter that
// binds connections to sessions. Everything here is volatile per
// process; restart clears reservations and sessions (clients refresh and
// re-ticket).
package session

import (
	"container/list"
	"time"

	"thinhthan/internal/core/id"
)

// Slot states of session.md § Login Queue: capacity is the number of
// distinct accounts holding a slot in any of these states.
type slotState int

const (
	slotTicketReserved slotState = iota
	slotCharacterSelect
	slotAttached
	slotReconnectGrace
)

// queueEntry is one FIFO login-queue entry for an account without a slot.
type queueEntry struct {
	accountID id.UUID
	firstAt   time.Time
	lastAt    time.Time
	el        *list.Element
}

// loginQueue is the FIFO of session.md § Login Queue. The single Edge
// admission owner (the registry mutex) serializes every transition.
type loginQueue struct {
	l        *list.List // of *queueEntry, front = oldest
	byAcct   map[id.UUID]*queueEntry
	cap      int
	used     map[id.UUID]slotState // accounts holding a slot
	admitTTL time.Duration         // head entry must re-request within 60s of position 1
}

func newLoginQueue(capacity int, admissionTTL time.Duration) *loginQueue {
	return &loginQueue{
		l:        list.New(),
		byAcct:   make(map[id.UUID]*queueEntry),
		cap:      capacity,
		used:     make(map[id.UUID]slotState),
		admitTTL: admissionTTL,
	}
}

// evictStaleHeads drops head entries whose admission window has lapsed
// (must request within 60s of reaching position 1 or lose their place).
func (q *loginQueue) evictStaleHeads(now time.Time) {
	for {
		e := q.l.Front()
		if e == nil {
			return
		}
		ent := e.Value.(*queueEntry)
		if now.Sub(ent.lastAt) <= q.admitTTL {
			return
		}
		q.l.Remove(e)
		delete(q.byAcct, ent.accountID)
	}
}

// admit tries to grant accountID a slot:
//
//   - already slotted → true (reissue replaces the unconsumed ticket).
//   - free capacity & queue empty or account is the admitted head → grant.
//   - otherwise enqueue (first request keeps FIFO order) → false + position.
//
// Returns (granted, position, isNewHead).
func (q *loginQueue) admit(accountID id.UUID, now time.Time) (granted bool, position int32) {
	if _, ok := q.used[accountID]; ok {
		return true, 0
	}
	q.evictStaleHeads(now)
	if len(q.used) < q.cap && (q.l.Len() == 0 || q.l.Front().Value.(*queueEntry).accountID == accountID) {
		if ent, ok := q.byAcct[accountID]; ok {
			q.l.Remove(ent.el)
			delete(q.byAcct, accountID)
		}
		q.used[accountID] = slotTicketReserved
		return true, 0
	}
	if ent, ok := q.byAcct[accountID]; ok {
		ent.lastAt = now
		return false, q.position(accountID)
	}
	ent := &queueEntry{accountID: accountID, firstAt: now, lastAt: now}
	ent.el = q.l.PushBack(ent)
	q.byAcct[accountID] = ent
	return false, q.position(accountID)
}

// bypass grants a slot regardless of capacity (reconnect: char live or in
// grace bypasses the queue on the ticket path too).
func (q *loginQueue) bypass(accountID id.UUID) {
	if ent, ok := q.byAcct[accountID]; ok {
		q.l.Remove(ent.el)
		delete(q.byAcct, accountID)
	}
	q.used[accountID] = slotTicketReserved
}

func (q *loginQueue) position(accountID id.UUID) int32 {
	i := int32(0)
	for e := q.l.Front(); e != nil; e = e.Next() {
		i++
		if e.Value.(*queueEntry).accountID == accountID {
			return i
		}
	}
	return 0
}

// setState updates one account's held-slot state.
func (q *loginQueue) setState(accountID id.UUID, st slotState) {
	if _, ok := q.used[accountID]; ok {
		q.used[accountID] = st
	}
}

// release frees the slot (detach-to-logout, disconnect without grace,
// logout) and drops any queued entry.
func (q *loginQueue) release(accountID id.UUID) {
	delete(q.used, accountID)
	if ent, ok := q.byAcct[accountID]; ok {
		q.l.Remove(ent.el)
		delete(q.byAcct, accountID)
	}
}

// RetryAfterMs implements the spec backoff: position ≤10 → 5000; else
// min(30000, 5000 + 1000·⌊position/10⌋).
func RetryAfterMs(position int32) int64 {
	if position <= 10 {
		return 5000
	}
	ms := 5000 + 1000*int64(position/10)
	if ms > 30000 {
		ms = 30000
	}
	return ms
}
