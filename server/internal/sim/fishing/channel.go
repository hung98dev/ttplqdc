package fishing

import (
	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

// Timing constants (world_rules.md § Folk Fishing; simulation 20 Hz).
const (
	// InteractRangeMM is the standard 103 interact reach used for CAST
	// admission (same reach as NPC service and hearth consults).
	InteractRangeMM = 2500
	// CastToWindowTicks is the 2 s between an accepted cast and the
	// hook window opening.
	CastToWindowTicks uint64 = 40
	// HookAcceptMinTicks / HookAcceptMaxTicks bound the one accepted
	// server-received HOOK after window start: [0.40 s, 1.20 s].
	HookAcceptMinTicks uint64 = 8
	HookAcceptMaxTicks uint64 = 24
	// SpectatorRadiusMM is the rare-catch splash visibility radius —
	// spectators in the same map_instance within 15 m see it.
	SpectatorRadiusMM = 15000
)

// State is the per-character fishing FSM.
type State int

const (
	StateIdle       State = iota // IDLE
	StateCasting                 // CAST accepted; waiting for the window
	StateHookWindow              // HOOK_WINDOW open
	StateResolving               // resolution committed/committed-failure
)

// Verdict is one consult admission verdict for an interact kind
// (mirrors sim/cooking's consult reply shape).
type Verdict struct {
	OK   bool
	Code string // stable fail-closed reason for the consult reply
}

var okVerdict = Verdict{OK: true}

// session is one character's live cast.
type session struct {
	state        State
	castAtTick   uint64
	windowAtTick uint64
	spotID       string
}

// Channel carries the fishing runtime of one map channel.
type Channel struct {
	mapID string
	spots []geometry.Anchor
	ok    bool // at least one spot anchor resolved

	rest         map[id.UUID]*session
	memberEntity func(charID id.UUID) (uint64, bool)
}

// New builds the channel's fishing runtime from the map's committed
// `fishing_spot.<map_id>.*` anchors. A map without a fishing spot
// (`has_water = false`) constructs with ok=false so consult answers
// fail closed instead of panicking.
func New(mapID string, anchors []geometry.Anchor) *Channel {
	c := &Channel{mapID: mapID, rest: map[id.UUID]*session{}}
	for _, a := range anchors {
		if len(a.ID) > len("fishing_spot.") &&
			a.ID[:len("fishing_spot.")] == "fishing_spot." {
			c.spots = append(c.spots, a)
		}
	}
	c.ok = len(c.spots) > 0
	return c
}

// SetMemberEntity binds the ChannelHost's character->entity lookup
// (same seam sim/cooking uses for its rest sessions).
func (c *Channel) SetMemberEntity(fn func(charID id.UUID) (uint64, bool)) {
	c.memberEntity = fn
}

// withinSpot reports whether the entity stands within interact reach
// of any fishing spot on this map.
func (c *Channel) withinSpot(ex, ey int32) bool {
	for _, a := range c.spots {
		dx := int64(ex) - a.X
		dy := int64(ey) - a.Y
		if dx*dx+dy*dy <= InteractRangeMM*InteractRangeMM {
			return true
		}
	}
	return false
}

// StateOf reports the character's current FSM state.
func (c *Channel) StateOf(charID id.UUID) State {
	if s, ok := c.rest[charID]; ok {
		return s.state
	}
	return StateIdle
}

// WindowOpen reports whether the character's hook window is currently
// open (presentation aid — the client shows the timing UI only).
func (c *Channel) WindowOpen(charID id.UUID) bool {
	s, ok := c.rest[charID]
	return ok && s.state == StateHookWindow
}

// AdmitCast checks CAST admission: a spot anchor exists on the map, the
// character entity is a live member, out of combat and alive, inside
// interact reach of a spot, and no cast is already in flight
// (world_rules.md § Folk Fishing — CAST starts IDLE -> CASTING).
func (c *Channel) AdmitCast(charID id.UUID, e *runtime.Entity) Verdict {
	if !c.ok || e == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if e.InCombatWith != 0 || e.Dead {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if c.StateOf(charID) != StateIdle {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if !c.withinSpot(e.Snap.X, e.Snap.Y) {
		return Verdict{Code: "OUT_OF_RANGE"}
	}
	return okVerdict
}

// AdmitHook checks HOOK admission: the session must be inside its
// HOOK_WINDOW and the consult tick must land in the [0.40 s, 1.20 s]
// accept interval. A HOOK arriving inside the window but outside the
// interval resolves the cast as a failure — the session closes (miss;
// the bait stays consumed per world_rules.md). A HOOK outside any
// window is a plain rejection.
func (c *Channel) AdmitHook(charID id.UUID, e *runtime.Entity, now uint64) Verdict {
	s, ok := c.rest[charID]
	if !ok || e == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if s.state != StateHookWindow {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if now < s.windowAtTick+HookAcceptMinTicks ||
		now > s.windowAtTick+HookAcceptMaxTicks {
		// Mistimed hook inside the window: the cast resolves as a
		// miss — close the session so no second hook can retry.
		delete(c.rest, charID)
		return Verdict{Code: "STATE_CONFLICT"}
	}
	return okVerdict
}

// ApplyCast commits an admitted CAST at tick now: IDLE -> CASTING and
// the hook window is scheduled to open CastToWindowTicks later.
func (c *Channel) ApplyCast(charID id.UUID, spotID string, now uint64) {
	c.rest[charID] = &session{state: StateCasting, castAtTick: now,
		spotID: spotID}
}

// ApplyHook closes an admitted HOOK: the durable hook executor is
// settling the roll; the session leaves the FSM (RESOLVING is a
// transient state inside Tick — the session deletes here so the next
// CAST is admissible once the result lands).
func (c *Channel) ApplyHook(charID id.UUID) {
	delete(c.rest, charID)
}

// EndFishing force-closes the character's cast — disconnect or map
// transfer during CASTING/HOOK_WINDOW resolves as failure (bait stays
// consumed, no catch; world_rules.md § Folk Fishing).
func (c *Channel) EndFishing(charID id.UUID) {
	delete(c.rest, charID)
}

// Tick advances the FSM once per partition tick: CASTING opens its
// HOOK_WINDOW 2 s after the cast; a window that expires without a
// valid hook resolves as failure (miss — no catch, bait consumed).
func (c *Channel) Tick(p *runtime.Partition, tc *runtime.TickContext) {
	now := p.TickN()
	for charID, s := range c.rest {
		switch s.state {
		case StateCasting:
			if now >= s.castAtTick+CastToWindowTicks {
				s.state = StateHookWindow
				s.windowAtTick = s.castAtTick + CastToWindowTicks
			}
		case StateHookWindow:
			if now > s.windowAtTick+HookAcceptMaxTicks {
				delete(c.rest, charID) // expired window = miss
			}
		}
	}
}
