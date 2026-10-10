package effects

// apply.go — the typed request surface. Skills emit
// skills.EffectRequest values; the stage-7 stat→status producer
// (HEAL_REDUCTION) and secondary builders emit the same shape.
// Apply resolves each request against the template registry and the
// target's book, returning an ordered result list — callers forward
// damage/shield results to their owning pipelines (the F-16-1 seam:
// this package never edits sim/combat).

import "thinhthan/internal/sim/skills"

// StatsView is the resolved stat read of one actor: the fields the
// effect engine consumes. Stats order mirrors
// replication.EntitySnapshot.Stats
// [lifesteal, reflect, absorb, heal_reduction, healing_received].
type StatsView struct {
	EntityID uint64
	Attack   int64
	MaxHP    int64
	Defense  int64
	Stats    [5]uint32
}

const (
	StatLifesteal       = 0
	StatReflect         = 1
	StatAbsorb          = 2
	StatHealReduction   = 3
	StatHealingReceived = 4
)

// TargetView identifies the entity the request lands on. The engine
// resolves the book internally by entity id.
type TargetView struct {
	EntityID uint64
}

// Ctx carries per-application context: the current tick, the
// recursion guard and the emission sink.
type Ctx struct {
	Now   uint64 // current sim tick
	Guard *RecursionGuard
}

// ResultKind enumerates engine outcomes.
type ResultKind uint8

const (
	ResultApplied ResultKind = iota
	ResultRefreshed
	ResultStackChanged
	ResultIgnored
	ResultReplaced
	ResultRejected // immunity / unknown template / depth-cap
	ResultExpired
	ResultDispelled
	ResultConsumed // CHILL consumption / ward unlink
	ResultDotTick
	ResultShieldGranted
	ResultShieldConsumed
	ResultShieldBroken
	ResultShieldExpired
	ResultShieldRemoved
	ResultForcedPositionSuppressed
)

// Result is one ordered outcome; downstream pipelines act on the
// damage/shield kinds, emit.go replicates the 205 kinds.
type Result struct {
	Kind       ResultKind
	TargetID   uint64
	SourceID   uint64
	EffectID   string
	Stacks     int32
	ExpiresAt  uint64
	Amount     int64 // DoT tick damage / shield amounts
	Element    string
	RejectedBy TagSet // immunity tags that rejected the request
	Instance   *Instance
}

// Apply consumes one typed request produced by skills
// (skills.EffectRequest) or by the stat→status producer and returns
// the ordered results.
//
// src is the request's originating actor (carries ATTACK for DoT
// snapshots and shield coefficients). tgt is the entity the effect
// lands on. req.Shape distinguishes produced requests:
// skills.EffectRequest maps to Kind by skills.RequestKind.
func (s *System) Apply(src StatsView, tgt TargetView, req Request, ctx Ctx) []Result {
	t, ok := TemplateByID(req.EffectID)
	if !ok {
		return []Result{{
			Kind: ResultRejected, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: req.EffectID,
		}}
	}
	if ctx.Guard != nil && !ctx.Guard.Enter(req.EffectID, tgt.EntityID) {
		return []Result{{
			Kind: ResultRejected, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: req.EffectID,
		}}
	}
	var out []Result
	switch t.Kind {
	case KindShield:
		out = s.applyShield(src, tgt, t, req, ctx)
	default:
		out = s.applyStatus(src, tgt, t, req, ctx)
	}
	if s.Outbound != nil {
		s.emitAll(out, ctx.Now)
	}
	return out
}

// Request is the engine's input shape — compatible with the fields
// skills.EffectRequest carries plus producer overrides for
// duration/magnitude (han_khi freeze length, cuong_hoa magnitude,
// absorb grant capacity) and forced-position spatial results.
type Request struct {
	// EffectID is the canonical template id.
	EffectID string
	// Stacks is the authored stack grant (STACK reapply).
	Stacks int32
	// AttackSnapshot overrides the DoT ATTACK snapshot when the
	// caller already resolved it; 0 = use src.Attack.
	AttackSnapshot int64
	// DurationMs overrides the template lifetime (han_khi freeze).
	DurationMs int64
	// HealingReceivedMult overrides the template magnitude
	// (stat-driven HEAL_REDUCTION).
	HealingReceivedMult float64
	// AttackAddBP / DefenseAddBP override magnitudes for the
	// passive-scaled buffs (cuong_hoa, bang_giap_guard).
	AttackAddBP  int32
	DefenseAddBP int32
	// ShieldAmount grants an explicit shield capacity
	// (effect.absorb.self, ally-shared tho_giap).
	ShieldAmount int64
	// SupportScale multiplies authored shield coefficients
	// (support_scale(S) of class_skill_catalog).
	SupportScale float64
	// ForcedPosition marks a spatial forced-position result
	// (PULL/KNOCKBACK/AIRBORNE) — suppressed by
	// DISPLACEMENT_IMMUNE while the rest of the hit resolves.
	ForcedPosition bool
	// FirstHitOnly is carried through for ordering parity with
	// skills.Payload (the caller filters eligibility).
	FirstHitOnly bool
}

// RequestOf adapts a skills.EffectRequest into the engine request.
func RequestOf(r skills.EffectRequest) Request {
	return Request{
		EffectID:     r.RefID,
		Stacks:       int32(r.Stacks),
		FirstHitOnly: r.FirstHitOnly,
	}
}
