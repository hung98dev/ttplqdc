package cooking

import (
	"fmt"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

// Timing constants (world_rules.md § Bonfire Rest; simulation 20 Hz).
const (
	// InteractRangeMM is the standard 103 interact reach used for
	// KINDLE and COOK admission (same reach as NPC service consults).
	InteractRangeMM = 2500
	// RestRadiusMM is the bonfire-rest radius — the session ends when
	// the character leaves it (world_rules.md § Bonfire Rest).
	RestRadiusMM = 6000
	// KindleExtendTicks is the 30-minute activation granted per
	// successful kindle (20 Hz × 1800 s).
	KindleExtendTicks uint64 = 36000
	// KindleCapTicks is the 120-minute expiry cap measured from server
	// time; a kindle that cannot extend by a full interval consumes no
	// item (world_rules.md § Kindling).
	KindleCapTicks uint64 = 144000
	// RestExpTickInterval is one completed rest tick — 10 s.
	RestExpTickInterval uint64 = 200
	// RestExpPerTick is the per-tick rest EXP award (rest_exp = 1,000).
	RestExpPerTick uint64 = 1000
	// RestExpDailyCap bounds completed rest ticks per character per
	// UTC day (180/day; enforced at settlement by durable/reward).
	RestExpDailyCap = 180
	// BondIntervalTicks is one completed bond interval — 300 s at
	// default bond, 150 s with the Bond-100 perk (spirit_beasts.md).
	BondIntervalTicks       uint64 = 6000
	BondIntervalPerkedTicks uint64 = 3000
)

// stationID returns the committed anchor id of the map's stations.
func bonfireAnchorID(mapID string) string { return "bonfire." + mapID }
func hearthAnchorID(mapID string) string  { return "cooking_hearth." + mapID }

// effect is the committed KINDLE outcome the apply side reports.
type Effect int

const (
	EffectActivated Effect = iota + 1 // inactive -> active, +30 min
	EffectExtended                    // active -> +30 min
	EffectCapped                      // cap blocks extension: no item consumed
)

// Channel carries the bonfire/hearth runtime of one map channel.
type Channel struct {
	mapID   string
	bonfire geometry.Anchor
	hearth  geometry.Anchor
	ok      bool // both anchors resolved

	active         bool
	expiresAtTick  uint64 // tick at which the bonfire goes inactive
	rest           map[id.UUID]*restSession
	emit           *Emission
	seq            uint64                      // deterministic settlement op-id sequence
	bondIntervalFn func(charID id.UUID) uint64 // bond-100 perk hook
	memberEntity   func(charID id.UUID) (uint64, bool)
}

// restSession is one live BONFIRE_REST session: counters tick up inside
// the partition tick; exp and bond settle independently.
type restSession struct {
	bonfireX, bonfireY int64
	lastX, lastY       int32 // last observed position (movement input check)
	ticksExp           uint64
	ticksBond          uint64
}

// New builds the channel's cooking runtime from the map's committed
// anchors. A map without both anchors is a hard catalog defect; the
// channel still constructs (ok=false) so consult answers fail closed
// instead of panicking.
func New(mapID string, anchors []geometry.Anchor, em *Emission) *Channel {
	c := &Channel{
		mapID: mapID,
		rest:  make(map[id.UUID]*restSession),
		emit:  em,
		bondIntervalFn: func(id.UUID) uint64 {
			return BondIntervalTicks
		},
	}
	for _, a := range anchors {
		switch a.ID {
		case "bonfire." + mapID:
			c.bonfire = a
		case "cooking_hearth." + mapID:
			c.hearth = a
		}
	}
	c.ok = c.bonfire.ID != "" && c.hearth.ID != ""
	return c
}

// SetMemberEntity binds the host's member lookup — a resting
// character's entity id inside this channel's partition — so Tick can
// resolve entities without importing sim/world (package boundary:
// sim/cooking is consumed by sim/world, never the reverse).
func (c *Channel) SetMemberEntity(fn func(charID id.UUID) (uint64, bool)) {
	c.memberEntity = fn
}

// SetBondInterval binds the perk-aware bond interval resolver — the
// Bond-100 perk halves the interval to 150 s (spirit_beasts.md);
// composition binds this to the companion's bond state once beast data
// flows (IMP-057). Default: the 300 s interval for every character.
func (c *Channel) SetBondInterval(fn func(charID id.UUID) uint64) {
	if fn != nil {
		c.bondIntervalFn = fn
	}
}

// Emission returns the channel's settlement ledger (payload store for
// the emit adapter).
func (c *Channel) Emission() *Emission { return c.emit }

// within reports whether the entity stands within radius mm of an
// anchor (squared compare — mm grid).
func within(ex, ey int32, a geometry.Anchor, radiusMM int64) bool {
	dx := int64(ex) - a.X
	dy := int64(ey) - a.Y
	return dx*dx+dy*dy <= radiusMM*radiusMM
}

// Verdict is one consult admission verdict for an interact kind.
type Verdict struct {
	OK   bool
	Code string // stable fail-closed reason for the consult reply
	// Consume=false admits the interaction but tells the edge path not
	// to submit a durable consume record (capped kindle: the cap
	// blocks the extension, so no cui_lua_trai is consumed —
	// world_rules.md § Kindling). The consult emits S2C_INTERACT_RESULT
	// SUCCESS directly and skips postWorld.
	Consume bool
}

var okVerdict = Verdict{OK: true, Consume: true}

// AdmitKindle checks KINDLE admission: bonfire anchor present, the
// character entity is a live channel member, out of combat and within
// interact reach. When the 120-minute server-time cap would block a
// full extension the verdict admits with Consume=false — the client
// sees a successful interact and nothing is consumed.
func (c *Channel) AdmitKindle(e *runtime.Entity, now uint64) Verdict {
	if !c.ok || e == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if e.InCombatWith != 0 || e.Dead {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if !within(e.Snap.X, e.Snap.Y, c.bonfire, InteractRangeMM) {
		return Verdict{Code: "OUT_OF_RANGE"}
	}
	if c.active && now < c.expiresAtTick &&
		c.expiresAtTick+KindleExtendTicks > now+KindleCapTicks {
		return Verdict{OK: true, Consume: false}
	}
	return okVerdict
}

// AdmitCook checks COOK admission: hearth anchor present, member in
// range, not in combat (hearth interaction rules world_rules.md).
func (c *Channel) AdmitCook(e *runtime.Entity) Verdict {
	if !c.ok || e == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if e.InCombatWith != 0 || e.Dead {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if !within(e.Snap.X, e.Snap.Y, c.hearth, InteractRangeMM) {
		return Verdict{Code: "OUT_OF_RANGE"}
	}
	return okVerdict
}

// AdmitRest checks BONFIRE_REST admission: bonfire active, member
// stationary within the 6 m rest radius and out of combat. A member
// already resting re-admits — the apply ends the session (the
// stand-up interaction).
func (c *Channel) AdmitRest(charID id.UUID, e *runtime.Entity, now uint64) Verdict {
	if !c.ok || e == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if _, resting := c.rest[charID]; resting {
		return okVerdict // stand-up toggle
	}
	if !c.active || now >= c.expiresAtTick {
		return Verdict{Code: "TARGET_INVALID"} // inactive bonfire
	}
	if e.InCombatWith != 0 || e.Dead {
		return Verdict{Code: "STATE_CONFLICT"}
	}
	if e.Snap.Vx != 0 || e.Snap.Vy != 0 {
		return Verdict{Code: "STATE_CONFLICT"} // must be stationary
	}
	if !within(e.Snap.X, e.Snap.Y, c.bonfire, RestRadiusMM) {
		return Verdict{Code: "OUT_OF_RANGE"}
	}
	return okVerdict
}

// ApplyKindle commits one admitted KINDLE at tick now: activates or
// extends the bonfire by 30 min, capped at 120 min after server time.
// EffectCapped means the cap blocks the full extension — no
// cui_lua_trai is consumed for it.
func (c *Channel) ApplyKindle(now uint64) Effect {
	capAt := now + KindleCapTicks
	if !c.active || now >= c.expiresAtTick {
		c.active = true
		c.expiresAtTick = now + KindleExtendTicks
		return EffectActivated
	}
	if c.expiresAtTick+KindleExtendTicks > capAt {
		return EffectCapped
	}
	c.expiresAtTick += KindleExtendTicks
	return EffectExtended
}

// ApplyRest toggles the rest session for an admitted BONFIRE_REST:
// start when absent, end (stand-up) when present.
func (c *Channel) ApplyRest(charID id.UUID, e *runtime.Entity) (started bool) {
	if _, ok := c.rest[charID]; ok {
		delete(c.rest, charID)
		return false
	}
	c.rest[charID] = &restSession{
		bonfireX: c.bonfire.X,
		bonfireY: c.bonfire.Y,
		lastX:    e.Snap.X,
		lastY:    e.Snap.Y,
	}
	return true
}

// IsResting reports whether the character currently rests at this
// channel's bonfire — the gating read for ruou_nep use
// (world_rules.md § Rượu Nếp: consume only while resting).
func (c *Channel) IsResting(charID id.UUID) bool {
	_, ok := c.rest[charID]
	return ok
}

// EndRest force-closes the character's rest session — used on map
// transfer, disconnect and combat/respawn paths (no settle tail).
func (c *Channel) EndRest(charID id.UUID) {
	c.endSession(charID)
}

// endSession removes the session inside Tick iteration (delete during
// range is safe — Go map semantics).
func (c *Channel) endSession(charID id.UUID) {
	delete(c.rest, charID)
}

// BonfireActive reports live bonfire activation for presentation.
func (c *Channel) BonfireActive(now uint64) bool {
	return c.active && now < c.expiresAtTick
}

// ExpiresAtTick exposes the active expiry for tests/presentation.
func (c *Channel) ExpiresAtTick() uint64 { return c.expiresAtTick }

// Tick advances every rest session once per partition tick. A session
// ends — settled counts already emitted stay emitted — on movement,
// combat, death, radius exit or bonfire expiry (world_rules.md § Rest
// ends). Each completed 200-tick rest emits one `sim.rest_settlement`
// intent; each completed bond interval (6000 or 3000 ticks) emits one
// `sim.beast_settlement` intent.
func (c *Channel) Tick(p *runtime.Partition, tc *runtime.TickContext) {
	now := p.TickN()
	for charID, s := range c.rest {
		e, err := c.restEntity(p, charID)
		if err != nil || e == nil {
			c.endSession(charID)
			continue // member gone (transfer/disconnect handled too)
		}
		if !c.BonfireActive(now) || e.Dead || e.InCombatWith != 0 ||
			e.Snap.X != s.lastX || e.Snap.Y != s.lastY ||
			!within(e.Snap.X, e.Snap.Y, c.bonfire, RestRadiusMM) {
			c.endSession(charID)
			continue
		}
		s.lastX, s.lastY = e.Snap.X, e.Snap.Y
		s.ticksExp++
		s.ticksBond++
		if s.ticksExp >= RestExpTickInterval {
			s.ticksExp = 0
			c.emitRest(p, charID, now)
		}
		if s.ticksBond >= c.bondIntervalFn(charID) {
			s.ticksBond = 0
			c.emitBond(p, charID, now)
		}
	}
}

// restEntity resolves the resting member's entity through the bound
// member lookup (SetMemberEntity).
func (c *Channel) restEntity(p *runtime.Partition, charID id.UUID) (*runtime.Entity, error) {
	if c.memberEntity == nil {
		return nil, fmt.Errorf("cooking: member entity lookup unbound")
	}
	eid, ok := c.memberEntity(charID)
	if !ok {
		return nil, fmt.Errorf("cooking: no member entity for %s", charID)
	}
	return p.Entity(eid)
}
