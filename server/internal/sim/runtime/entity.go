package runtime

import (
	"errors"

	"thinhthan/internal/sim/replication"
)

// EntityClass selects which per-class budget of the MAX_ENTITIES_PER_CHANNEL
// capacity an admission draws on (docs/04_architecture/realtime_loop.md,
// ADR-0039, ADR-0070).
type EntityClass int

const (
	ClassPlayer     EntityClass = iota // reserved slots: placement never refused by the entity cap
	ClassSpawnGroup                    // spawn-group managed monsters
	ClassEvent                         // world-event entities
	ClassBoss                          // boss entities; PUBLIC adds are never refused
	ClassTransient                     // transient entities; dropped first under pressure
)

// Per-class budgets. The slot layout partitions the fixed entity array into
// per-class ranges so an Admit of one class can never starve another.
const (
	PlayersCap    = 22
	SpawnGroupCap = 42
	EventCap      = 12
	BossCap       = 8
	TransientCap  = 16

	// EntityCap is MAX_ENTITIES_PER_CHANNEL.
	EntityCap = PlayersCap + SpawnGroupCap + EventCap + BossCap + TransientCap
)

// classBase is the first slot index of each class range.
var classBase = [...]int{0, PlayersCap, PlayersCap + SpawnGroupCap, PlayersCap + SpawnGroupCap + EventCap, PlayersCap + SpawnGroupCap + EventCap + BossCap}

func classCapOf(c EntityClass) int {
	switch c {
	case ClassPlayer:
		return PlayersCap
	case ClassSpawnGroup:
		return SpawnGroupCap
	case ClassEvent:
		return EventCap
	case ClassBoss:
		return BossCap
	case ClassTransient:
		return TransientCap
	}
	return 0
}

// Admission errors. OverloadDegrade reasons are typed so callers and tests
// can assert the exact capacity that failed.
var (
	// ErrEntityCap reports a failed admission into a full class budget
	// (a spawn-group, event, boss or transient add, or a player add when
	// the reserved player range itself is full — the only cap a player
	// admission is subject to).
	ErrEntityCap = errors.New("runtime: entity class capacity exhausted")
	// ErrUnknownEntity reports a mutation or lookup for an entity id that
	// is not live.
	ErrUnknownEntity = errors.New("runtime: unknown entity")
	// ErrNotPlayer reports an operation valid only for player entities.
	ErrNotPlayer = errors.New("runtime: entity is not a player")
)

// Entity is one live thing inside the partition. Its replicated wire state
// rides in Snap so the replication view needs no copying.
type Entity struct {
	ID    uint64
	Class EntityClass
	Slot  int

	// Snap carries the EntityState wire fields (position, vitals, statuses,
	// cosmetics, encounter and stat block).
	Snap replication.EntitySnapshot
	// Private carries SelfPrivateState fields replicated only to the owner.
	Private replication.PrivateSnapshot
	// Checkpoint carries the complete MovementCheckpoint fields replicated
	// inside every self_ack.
	Checkpoint replication.CheckpointSnapshot

	// PartyID links same-party entities for the never-shed AOI class
	// (0 = no party).
	PartyID uint64
	// Objective marks an active encounter/boss objective entity
	// (never-shed class).
	Objective bool
	// InCombatWith holds the viewer entity id this entity is in combat
	// with (0 = none); the AOI in-combat class is viewer-relative.
	InCombatWith uint64
	// Hostile marks hostile entities (shed class 4).
	Hostile bool

	// ExpiresAtTick removes a transient entity at this tick
	// (CLEANUP_EXPIRED_TRANSIENT_ENTITIES); 0 = never expires.
	ExpiresAtTick uint64
	// Dead selects DESPAWN_REASON_DIED over REMOVED when the entity is
	// removed.
	Dead bool

	// Input bookkeeping (player entities). Edge events are a FIFO —
	// ADR-0038 forbids coalescing them; the movement system drains the
	// queue in receive order each tick.
	lastClientSeq uint64
	pendingEdges  [DiscreteCap]uint8
	pendingEdgeN  int
	hasHeld       bool
}

// PendingEdges returns queued movement-edge events in receive order.
func (e *Entity) PendingEdges() []uint8 { return e.pendingEdges[:e.pendingEdgeN] }

func slotOf(id uint64) int { return int(id >> 20) }

// entityByID resolves a runtime id to the live entity or nil.
func (p *Partition) entityByID(id uint64) *Entity {
	slot := slotOf(id)
	if slot < 0 || slot >= EntityCap || !p.used[slot] {
		return nil
	}
	e := &p.ent[slot]
	if e.ID != id {
		return nil
	}
	return e
}

// Entity returns the live entity for id, or ErrUnknownEntity.
func (p *Partition) Entity(id uint64) (*Entity, error) {
	e := p.entityByID(id)
	if e == nil {
		return nil, ErrUnknownEntity
	}
	return e, nil
}

// Admit places one entity of the given class and returns its runtime id.
// The id encodes the slot and a per-slot incarnation counter, so it is
// unique for the partition lifetime and resolves without a lookup table.
// Player admission draws only on the reserved player range; a spawn-group
// admission that fails the entity cap is retryable; a boss add is refused
// (not created) when the boss range is full; a transient add is dropped
// first under pressure — all reported as ErrEntityCap.
func (p *Partition) Admit(class EntityClass) (uint64, error) {
	base := classBase[class]
	budget := classCapOf(class)
	for i := 0; i < budget; i++ {
		slot := base + i
		if p.used[slot] {
			continue
		}
		p.nextSeq[slot]++
		id := uint64(slot)<<20 | (p.nextSeq[slot] & 0xfffff)
		e := &p.ent[slot]
		*e = Entity{ID: id, Class: class, Slot: slot}
		e.Snap.ID = id
		p.used[slot] = true
		p.classN[class]++
		if class == ClassPlayer {
			p.attachClient(id)
		}
		return id, nil
	}
	return 0, ErrEntityCap
}

func (p *Partition) remove(e *Entity) {
	p.used[e.Slot] = false
	p.classN[e.Class]--
	*e = Entity{ID: e.ID, Slot: e.Slot}
}

// Remove despawns an entity, recording the despawn reason for the
// replication phase (DIED when e.Dead, else REMOVED).
func (p *Partition) Remove(id uint64) error {
	e := p.entityByID(id)
	if e == nil {
		return ErrUnknownEntity
	}
	reason := uint8(removedRemoved)
	if e.Dead {
		reason = removedDied
	}
	if p.removedN < len(p.removed) {
		p.removed[p.removedN] = removedLog{ID: id, Reason: reason}
		p.removedN++
	}
	if e.Class == ClassPlayer {
		p.detachClient(e.Slot)
	}
	p.grid.Remove(id)
	p.remove(e)
	return nil
}
