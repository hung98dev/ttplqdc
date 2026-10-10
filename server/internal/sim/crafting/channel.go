package crafting

import (
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

// InteractRangeMM is the standard NPC service reach — the same 2.5 m
// consult range used by every NPC interaction (ADR-0083).
const InteractRangeMM = 2500

// Deps are the composition-bound resolvers a station channel needs:
// NPC service positions, the compiled recipe minimum level, and the
// frozen capacity requirement of one batch. nil is fail-closed.
type Deps struct {
	// Station resolves a `*.tho_nghe` service NPC's anchor + whether it
	// offers the asked service ("crafting" or "enhancement").
	Station func(npcID, service string) (geometry.Anchor, bool)
	// RecipeMin resolves a recipe_id to its minimum character level.
	RecipeMin func(recipeID string) (minLevel int32, exists bool)
	// NeedSlots resolves the free-slot requirement of one batch
	// (stackable outputs collapse; equipment needs batch slots).
	NeedSlots func(recipeID string, batch uint32) int64
}

// Channel carries station admission checks for one bound consult
// context — the runtime gates mirrored on sim/cooking.Channel: entity
// live, out of combat, in reach, level/batch/capacity pre-checks.
// Frozen payload building and settlement live in durable/crafting.
type Channel struct {
	deps Deps
}

// New binds the consult deps.
func New(deps Deps) *Channel { return &Channel{deps: deps} }

// Verdict is one admission verdict (mirrors sim/cooking.Verdict).
type Verdict struct {
	OK      bool
	Code    string // stable fail-closed reason for the consult reply
	Consume bool
}

var okVerdict = Verdict{OK: true, Consume: true}

// within reports whether the entity stands within radius mm of an
// anchor (squared compare — mm grid).
func within(ex, ey int32, a geometry.Anchor, radiusMM int64) bool {
	dx := int64(ex) - a.X
	dy := int64(ey) - a.Y
	return dx*dx+dy*dy <= radiusMM*radiusMM
}

// liveGate is the shared entity precondition: present, alive, out of
// combat (crafting.md § Enhancement: not in combat).
func liveGate(e *runtime.Entity) (Verdict, bool) {
	if e == nil {
		return Verdict{Code: "TARGET_INVALID"}, false
	}
	if e.InCombatWith != 0 || e.Dead {
		return Verdict{Code: "STATE_CONFLICT"}, false
	}
	return Verdict{}, true
}

// stationGate resolves the station NPC and checks interact reach.
func (c *Channel) stationGate(e *runtime.Entity, npcID, service string) (Verdict, bool) {
	if c.deps.Station == nil {
		return Verdict{Code: "TARGET_INVALID"}, false
	}
	a, ok := c.deps.Station(npcID, service)
	if !ok {
		return Verdict{Code: "TARGET_INVALID"}, false
	}
	if !within(e.Snap.X, e.Snap.Y, a, InteractRangeMM) {
		return Verdict{Code: "OUT_OF_RANGE"}, false
	}
	return Verdict{}, true
}

// AdmitCraft checks C2S_CRAFT admission: live entity, SERVICE(crafting)
// station in range, batch inside 1..99, recipe exists in the compiled
// catalog, tier minimum level, and the capacity pre-check — every gate
// fails before any durable record is built (crafting.md § Atomic
// Craft: validate all before consume).
func (c *Channel) AdmitCraft(e *runtime.Entity, npcID, recipeID string,
	batch uint32, level int32, freeSlots int64) Verdict {
	if v, ok := liveGate(e); !ok {
		return v
	}
	if v, ok := c.stationGate(e, npcID, "crafting"); !ok {
		return v
	}
	if batch < 1 || batch > 99 {
		return Verdict{Code: "OUT_OF_RANGE"}
	}
	if c.deps.RecipeMin == nil {
		return Verdict{Code: "TARGET_INVALID"}
	}
	minLevel, exists := c.deps.RecipeMin(recipeID)
	if !exists {
		return Verdict{Code: "TARGET_INVALID"}
	}
	if level < minLevel {
		return Verdict{Code: "LEVEL_TOO_LOW"}
	}
	if c.deps.NeedSlots != nil && freeSlots < c.deps.NeedSlots(recipeID, batch) {
		return Verdict{Code: "INVENTORY_FULL"}
	}
	return okVerdict
}

// AdmitEnhance checks C2S_ENHANCE admission: live entity,
// SERVICE(enhancement) station in range. Item/target/lock/charm
// validation is part of the frozen durable plan (state may move
// between consult and commit).
func (c *Channel) AdmitEnhance(e *runtime.Entity, npcID string) Verdict {
	if v, ok := liveGate(e); !ok {
		return v
	}
	if v, ok := c.stationGate(e, npcID, "enhancement"); !ok {
		return v
	}
	return okVerdict
}
