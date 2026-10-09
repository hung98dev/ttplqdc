package moderation

import (
	"sync"
	"time"

	"thinhthan/internal/core/id"
	sociald "thinhthan/internal/durable/social"
)

// repeatWindow bounds the spam memory — repeats older than this are
// forgotten, never aggregated into a sanction record.
const repeatWindow = 30 * time.Second

// repeatLimit is the identical-content count inside repeatWindow at
// which the automated filter starts rejecting.
const repeatLimit = 4

// Filter is the automated unsafe/spam classifier (social.md §
// Moderation: "automated filtering may reject unsafe/spam content, but
// punitive sanctions require explicit moderation policy/action"). It
// detects one deterministic spam signal — the same sender submitting
// the identical message text repeatLimit times inside repeatWindow —
// and rejects the send. It keeps only per-(sender,text) counters and
// NEVER writes restrictions, reports, or account state: a filter
// verdict is rejection, nothing more.
type Filter struct {
	mu   sync.Mutex
	seen map[spamKey]spamCount
}

type spamKey struct {
	sender id.UUID
	text   string
}

type spamCount struct {
	n     int
	first time.Time
}

// NewFilter constructs the empty classifier.
func NewFilter() *Filter {
	return &Filter{seen: map[spamKey]spamCount{}}
}

// Reject reports whether the send is rejected as automated spam. The
// text is canonicalized with the same 240-grapheme rule as the send
// path (an already-canonical input is unchanged); non-canonical input
// rejects outright.
func (f *Filter) Reject(senderID id.UUID, text string, now time.Time) bool {
	canonical, ok := sociald.CanonicalText(text, 240)
	if !ok {
		return true
	}
	k := spamKey{sender: senderID, text: canonical}
	f.mu.Lock()
	c := f.seen[k]
	if c.n == 0 || now.Sub(c.first) >= repeatWindow {
		c = spamCount{n: 0, first: now}
	}
	c.n++
	f.seen[k] = c
	needForget := len(f.seen) > spamMapSoftCap
	f.mu.Unlock()
	if needForget {
		f.forget(now)
	}
	return c.n >= repeatLimit
}

// spamMapSoftCap triggers a stale-counter prune; keeps memory bounded
// by active senders rather than total senders.
const spamMapSoftCap = 1024

// forget prunes stale counters so memory stays bounded by active
// senders rather than cumulative senders.
func (f *Filter) forget(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, c := range f.seen {
		if now.Sub(c.first) >= repeatWindow {
			delete(f.seen, k)
		}
	}
}
