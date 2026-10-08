package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// ActionState is the runtime combat action state of combat.md § Combat
// Action Phases. Movement state is independent.
type ActionState uint8

const (
	StateIdle ActionState = iota
	StateAttacking
	StateCasting
	StateRecovery
	StateHitReaction // presentation-only 120 ms; never locks actions
	StateDead
)

// ActionPhase is the canonical STARTUP -> ACTIVE -> RECOVERY timing of
// skills.md § Action Timing.
type ActionPhase uint8

const (
	PhaseStartup ActionPhase = iota
	PhaseActive
	PhaseRecovery
	PhaseDone
)

// InterruptCause is the authored interruption set of skills.md §
// Interrupt and Cancel. Death always cancels unresolved execution.
type InterruptCause uint8

const (
	InterruptNone InterruptCause = iota
	InterruptStun
	InterruptFreeze
	InterruptKnockback
	InterruptDamage
	InterruptMovement
	InterruptAnotherSkill
	InterruptMapTransfer
	InterruptDeath
)

// Action is one accepted authoritative action. Deadlines are exact
// cumulative millisecond dues converted to the first 50 ms tick at or
// after the due time (skills.md § Action Deadline Conversion).
type Action struct {
	ID             uint64
	Source         uint64
	SkillID        string
	Facing         protocolv1.Facing
	Target         uint64
	AreaX, AreaY   int32
	ClientSeq      uint64
	AcceptedTick   uint64
	Phase          ActionPhase
	CastMs         uint32
	ActiveStart    uint64 // tick of executed ACTIVE transition
	ActiveEnd      uint64
	RecoveryEnd    uint64
	MotionEnd      uint64 // 0 = no authored motion
	CooldownEnd    uint64
	NextAcceptTick uint64 // basic self-chain floor
	Interrupts     uint16 // bitmask of InterruptCause the skill defines
	Resolved       bool   // primary resolution committed
}

// ceilTick rounds a millisecond due up to the first 50 ms tick.
func ceilTick(dueMs int64) uint64 {
	if dueMs <= 0 {
		return 0
	}
	return uint64((dueMs + 49) / 50)
}

// tickMs converts a tick back to milliseconds.
func tickMs(t uint64) int64 { return int64(t) * 50 }

// effectivePhaseMs scales one timing phase by the action's declared
// timing speed stat: ceil(base / (1 + speed)).
func effectivePhaseMs(baseMs int64, speed float64) int64 {
	if speed < 0 {
		speed = 0
	}
	num := baseMs * 1000
	den := int64((1 + speed) * 1000)
	return (num + den - 1) / den
}

// actorState is the per-entity combat bookkeeping.
type actorState struct {
	state       ActionState
	action      *Action
	hitReactEnd uint64 // presentation marker deadline tick

	// in_combat: refresh on every hostile event; exit 6 s after the
	// last refresh with no unresolved hostile action.
	lastHostileMs int64
	opponent      uint64
	hostileOpen   int

	// cooldowns committed ON_START, keyed by skill_id.
	cooldownUntil map[string]uint64

	jg      justGuardState
	latency *LatencyModel
	stamps  edgeHistory

	shields []Shield
	shieldN uint64 // creation sequence
}

// stateFor returns the actor state for an entity, resetting on
// incarnation change.
func (s *System) stateFor(e *runtime.Entity) *actorState {
	st := s.actors[e.ID]
	if st == nil {
		st = &actorState{latency: NewLatencyModel()}
		s.actors[e.ID] = st
	}
	return st
}

// stateOf reports the public action state for an entity.
func (s *System) stateOf(e *runtime.Entity) ActionState {
	st := s.stateFor(e)
	if e.Snap.HP <= 0 || e.Dead {
		return StateDead
	}
	return st.state
}

// canStart reports whether a fresh action may be accepted: not dead,
// no unresolved action (RECOVERY blocks new actions except the basic
// self-chain after its deadlines), no alive forced motion, and no
// action-locking status.
func (s *System) canStart(e *runtime.Entity, st *actorState, skill *SkillDef, tc *runtime.TickContext) bool {
	if e.Snap.HP <= 0 || e.Dead {
		return false
	}
	if st.action != nil {
		if tc.Tick < st.action.RecoveryEnd {
			return false
		}
		if tc.Tick < st.action.MotionEnd {
			return false
		}
	}
	if s.hardLocked(e) {
		return false
	}
	return true
}

// selfChainOK implements the sole cancel_out exception: an equipped
// BASIC_ATTACK may cancel its own RECOVERY into another accept of the
// same basic after the effective next-accept deadline and forced-motion
// deadline.
func (s *System) selfChainOK(st *actorState, skill *SkillDef, tc *runtime.TickContext) bool {
	if st.action == nil || st.action.SkillID != skill.SkillID {
		return false
	}
	if !skill.IsBasic {
		return false
	}
	if tc.Tick < st.action.NextAcceptTick || tc.Tick < st.action.MotionEnd {
		return false
	}
	return true
}

// interrupt cancels the unresolved action when the cause is death
// (always) or is listed in the action's interrupt set.
func (s *System) interrupt(st *actorState, cause InterruptCause) {
	if st.action == nil {
		return
	}
	if cause == InterruptDeath || st.action.Interrupts&(1<<cause) != 0 {
		st.action = nil
	}
}
