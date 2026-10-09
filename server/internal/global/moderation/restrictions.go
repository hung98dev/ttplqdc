package moderation

import (
	"sync"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Restriction is the canonical communication restriction verdict on one
// character (social.md § Moderation): only NONE and MUTED exist — a
// MUTED entry carries the channel subset it applies to (an empty set
// mutes every player-authored channel) and a server-authoritative
// expiry after which it no longer restricts.
type Restriction struct {
	// Channels is the muted subset; empty = all channels.
	Channels map[protocolv1.ChatChannel]bool
	// Until is the server-authoritative expiry.
	Until time.Time
}

// covers reports whether this restriction mutes `channel` at `now`.
func (r Restriction) covers(channel protocolv1.ChatChannel, now time.Time) bool {
	if !r.Until.IsZero() && !now.Before(r.Until) {
		return false
	}
	if len(r.Channels) == 0 {
		return true
	}
	return r.Channels[channel]
}

// Restrictions is the runtime store of operator-issued mutes, consulted
// at send admission. It keeps no history — an expired entry is a NONE.
type Restrictions struct {
	mu   sync.Mutex
	rows map[id.UUID]Restriction
}

// NewRestrictions constructs the empty store.
func NewRestrictions() *Restrictions {
	return &Restrictions{rows: map[id.UUID]Restriction{}}
}

// Mute issues a MUTED restriction on `characterID` covering `channels`
// (empty = all player-authored channels) until `until`. A zero until is
// indefinite. This is the only write path — reports and the automated
// filter never call it (social.md: submitting a report never
// automatically mutes; sanctions require explicit moderation action).
func (r *Restrictions) Mute(characterID id.UUID,
	channels []protocolv1.ChatChannel, until time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := map[protocolv1.ChatChannel]bool{}
	for _, c := range channels {
		set[c] = true
	}
	r.rows[characterID] = Restriction{Channels: set, Until: until}
}

// Lift clears any restriction (operator unmute / verdict reversal).
func (r *Restrictions) Lift(characterID id.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, characterID)
}

// Restricted reports whether `characterID` is muted on `channel` at
// `now`. Expired entries are dropped lazily.
func (r *Restrictions) Restricted(characterID id.UUID,
	channel protocolv1.ChatChannel, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[characterID]
	if !ok {
		return false
	}
	if !row.Until.IsZero() && !now.Before(row.Until) {
		delete(r.rows, characterID)
		return false
	}
	return row.covers(channel, now)
}
