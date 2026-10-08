package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/progression"
	"thinhthan/internal/sim/runtime"
)

// Wire ids (messages.md).
const (
	idS2CActionStarted  = 203
	idS2CActionRejected = 204
	idS2CDeath          = 206
	idS2CCombatEvent    = 304
)

const (
	inCombatLockMs  = 6000
	hitReactionMs   = 120
	respawnDelayTks = 60 // RESPAWN_DELAY = 3 s (death_respawn.md)
)

// RequestKind enumerates the inbound combat intents.
type RequestKind uint8

const (
	ReqSkillUse RequestKind = iota + 1
	ReqBasicAttack
	ReqTargetIntent
	ReqRespawn
)

// Request is one decoded inbound combat message.
type Request struct {
	Kind          RequestKind
	Source        uint64
	SkillID       string
	Facing        protocolv1.Facing
	TargetID      uint64
	AreaX, AreaY  int32
	ClientSeq     uint64
	OperationID   []byte // 208 only
	ClientMonoMs  int64
	ReceiveMs     int64
	RequestWireID uint32 // 200/201/202/208 echoed on 204
}

// Config wires combat's injected ports.
type Config struct {
	Outbound runtime.OutboundPort
	Skills   SkillSource
	Stats    StatSource
	// Learned reports whether the entity has the skill learned
	// (SKILL_NOT_LEARNED otherwise). nil → always true.
	Learned func(e *runtime.Entity, skillID string) bool
	// Loadout reports whether the skill is in the current loadout
	// (SKILL_LOADOUT_INVALID otherwise). nil → always true.
	Loadout func(e *runtime.Entity, skillID string) bool
	// Hurtbox returns (halfWidthMM, heightMM) for an entity's
	// authoritative hurtbox.
	Hurtbox func(entityID uint64) (int64, int64)
	// HardControl reports STUN/FREEZE/ROOT active on the entity
	// (AIRBORNE does not count).
	HardControl func(e *runtime.Entity) bool
	Hint        HintPort
	Rand        func() float64 // rand_float(0,1); required
}

// System is the per-partition combat runtime.
type System struct {
	cfg          Config
	actors       map[uint64]*actorState
	enumBuf      []uint64
	nextActionID uint64
	nextEventID  uint64
}

// New wires the system.
func New(cfg Config) *System {
	return &System{cfg: cfg, actors: make(map[uint64]*actorState), nextActionID: 1, nextEventID: 1}
}

func (s *System) hardLocked(e *runtime.Entity) bool {
	return s.cfg.HardControl != nil && s.cfg.HardControl(e)
}

func (s *System) learned(e *runtime.Entity, skillID string) bool {
	return s.cfg.Learned == nil || s.cfg.Learned(e, skillID)
}

func (s *System) loadout(e *runtime.Entity, skillID string) bool {
	return s.cfg.Loadout == nil || s.cfg.Loadout(e, skillID)
}

// Accept validates one inbound intent and either starts the action
// (emitting 203) or rejects it (emitting 204 with the echoed seq /
// request_message_id / error_code).
func (s *System) Accept(p *runtime.Partition, req Request, tc *runtime.TickContext) {
	src, err := p.Entity(req.Source)
	if err != nil {
		return
	}
	st := s.stateFor(src)

	switch req.Kind {
	case ReqTargetIntent:
		def, ok := s.cfg.Skills.Lookup(req.SkillID)
		if !ok || !s.acquireTarget(p, src, req.TargetID, &def) {
			s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
			return
		}
		return
	case ReqRespawn:
		// Respawn execution is IMP-084's domain; combat only validates
		// the death state precondition.
		if st.state != StateDead {
			s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		}
		return
	}

	def, ok := s.cfg.Skills.Lookup(req.SkillID)
	if !ok || !s.learned(src, req.SkillID) {
		s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED)
		return
	}
	if !s.loadout(src, req.SkillID) {
		s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_SKILL_LOADOUT_INVALID)
		return
	}
	if until, ok := st.cooldownUntil[def.SkillID]; ok && tc.Tick < until {
		s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE)
		return
	}
	if !s.canStart(src, st, &def, tc) && !s.selfChainOK(st, &def, tc) {
		s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	if def.CostMP > 0 && src.Private.CurrentMP < def.CostMP && def.CostTiming == CostOnStart {
		s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_MP)
		return
	}

	// Target validation: a hostile action needs a resolvable target —
	// locked intent target first, else nearest in-cone candidate.
	target := req.TargetID
	if target == 0 {
		target = src.Private.AcceptedTargetID
	}
	if def.RequiresTgt {
		if target == 0 {
			if cand := s.gatherCandidates(p, src, nil, &def); len(cand) > 0 {
				target = selectTargets(cand, 0, TargetCap{1, 1})[0]
			} else {
				s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
				return
			}
		}
		t, err := p.Entity(target)
		if err != nil || t.Snap.HP <= 0 || t.Dead {
			s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
			return
		}
		ox, oy := s.originOf(src)
		reach := int64(def.Geom.rangeMM())
		if closestDistanceSq(ox, oy, s.hurtboxOf(t)) > reach*reach {
			s.reject(src, req, protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE)
			return
		}
	}

	// Commit: costs + cooldown ON_START, phases converted to deadline
	// ticks (skills.md § Action Deadline Conversion).
	stats, _ := s.cfg.Stats.StatsOf(src.ID)
	var speed float64
	switch def.Timing {
	case TimingAttackSpeed:
		speed = stats[progression.StatAttackSpeed]
	case TimingCastSpeed:
		speed = stats[progression.StatCastSpeed]
	}
	startup := effectivePhaseMs(def.StartupMs, speed)
	recovery := effectivePhaseMs(def.RecoveryMs, speed)
	act := &Action{
		ID:           s.nextActionID,
		Source:       src.ID,
		SkillID:      def.SkillID,
		Facing:       req.Facing,
		Target:       target,
		AreaX:        req.AreaX,
		AreaY:        req.AreaY,
		ClientSeq:    req.ClientSeq,
		AcceptedTick: tc.Tick,
		Phase:        PhaseStartup,
		ActiveStart:  tc.Tick + ceilTick(startup),
		Interrupts:   def.Interrupts,
	}
	act.ActiveEnd = tc.Tick + ceilTick(startup+def.ActiveMs)
	act.RecoveryEnd = tc.Tick + ceilTick(startup+def.ActiveMs+recovery)
	if def.MotionMs > 0 {
		act.MotionEnd = tc.Tick + ceilTick(def.MotionMs)
	}
	act.CooldownEnd = tc.Tick + ceilTick(def.CooldownMs)
	if def.IsBasic {
		interval := def.CooldownMs
		if stats[progression.StatAttackSpeed] > 0 {
			scaled := effectivePhaseMs(def.CooldownMs, stats[progression.StatAttackSpeed])
			interval = scaled
		}
		if min := startup + def.ActiveMs; interval < min {
			interval = min
		}
		if min := startup + def.MotionMs; interval < min {
			interval = min
		}
		act.NextAcceptTick = tc.Tick + ceilTick(interval)
	}
	if st.cooldownUntil == nil {
		st.cooldownUntil = make(map[string]uint64)
	}
	st.cooldownUntil[def.SkillID] = act.CooldownEnd
	if def.CostTiming == CostOnStart && def.CostMP > 0 {
		src.Private.CurrentMP -= def.CostMP
	}
	s.nextActionID++
	st.action = act
	if def.Timing == TimingCastSpeed {
		st.state = StateCasting
	} else {
		st.state = StateAttacking
	}
	if def.Hostile && target != 0 {
		s.enterCombat(src, st, target, tc)
	}
	s.emitStarted(src, act, tc)
}

// Step advances every tracked action and the in_combat bookkeeping.
func (s *System) Step(p *runtime.Partition, tc *runtime.TickContext) {
	nowMs := tickMs(tc.Tick)
	for id, st := range s.actors {
		e, err := p.Entity(id)
		if err != nil {
			delete(s.actors, id)
			continue
		}
		// hit-reaction presentation marker expiry.
		if st.state == StateHitReaction && tc.Tick >= st.hitReactEnd {
			st.state = StateIdle
		}
		// action phase advancement.
		if st.action != nil {
			act := st.action
			if tc.Tick >= act.ActiveStart && !act.Resolved {
				act.Phase = PhaseActive
				st.state = StateRecovery
				act.Resolved = true
				s.resolveAction(p, e, act, tc)
			}
			if tc.Tick >= act.RecoveryEnd && tc.Tick >= act.MotionEnd {
				st.action = nil
				if st.state != StateDead && st.state != StateHitReaction {
					st.state = StateIdle
				}
			}
		}
		// in_combat expiry: 6 s since last hostile refresh AND no
		// unresolved hostile action.
		if e.InCombatWith != 0 && nowMs-st.lastHostileMs >= inCombatLockMs && st.hostileOpen == 0 {
			e.InCombatWith = 0
			st.opponent = 0
		}
	}
}

// NoteRtt folds a heartbeat RTT sample into the entity's latency model.
func (s *System) NoteRtt(e *runtime.Entity, rttMs int64) {
	s.stateFor(e).latency.NoteRtt(rttMs)
}

// NoteEdge records one C2S_MOVEMENT_EDGE for Just Guard evaluation:
// the latency model stamps its effective time and staleness verdict.
func (s *System) NoteEdge(e *runtime.Entity, edgeType protocolv1.MovementEdgeType, dir protocolv1.Facing, clientMonoMs, receiveMs int64) {
	st := s.stateFor(e)
	eff, stale := st.latency.Evaluate(clientMonoMs, receiveMs)
	st.stamps.note(edgeStamp{effectiveMs: eff, stale: stale, horizontal: horizontalEdge(edgeType, dir)})
}

// Interrupt applies an external interruption cause to the entity's
// unresolved action (combat.md § Interrupt and Cancel).
func (s *System) Interrupt(e *runtime.Entity, cause InterruptCause) {
	st := s.stateFor(e)
	s.interrupt(st, cause)
}

// LatencyOf exposes the entity's latency model for tests and the
// edge-draining lane.
func (s *System) LatencyOf(e *runtime.Entity) *LatencyModel {
	return s.stateFor(e).latency
}

// StateOf reports the entity's public action state.
func (s *System) StateOf(e *runtime.Entity) ActionState {
	return s.stateOf(e)
}
