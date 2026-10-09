package skills

// status_bridge.go — the typed request surface between skill
// resolution and the landed combat/status/shield pipelines. Skills
// produce ordered requests; the pipelines own the effects themselves.

// SpatialKind enumerates the secondary spatial effect families of
// class_skill_catalog.md § Secondary Spatial Effects.
type SpatialKind uint8

const (
	// SplashCircle: circle at the primary-hit hurtbox center; the
	// primary target is excluded and results share the triggering
	// action's cap.
	SplashCircle SpatialKind = iota
	// ContactSweep: MOVE_CONTACT_LINE contact box over the resolved
	// path; the first eligible enemy receives the authored contact
	// result; path truncation truncates the sweep.
	ContactSweep
	// PushAlong: push the connected target in the projectile/caster
	// facing direction.
	PushAlong
	// PullToCenter: pull each selected target toward the area center
	// by its remaining center distance, capped at MaxDistanceMM.
	PullToCenter
	// RadialPush: push selected enemies radially outward from the
	// area center.
	RadialPush
	// AirborneLift: canonical AIRBORNE status — vertical presentation
	// apex; the collision anchor stays on the world plane.
	AirborneLift
	// ExplosionCircle: circle at an expired-status target's hurtbox
	// center; the source target may be hit; hard ceiling caps.
	ExplosionCircle
	// AuraCircle: non-damaging aura around the caster skill origin.
	AuraCircle
	// ReactiveMelee: eligibility gate on the triggering hostile
	// component — the attacker must be within RangeMM; no area search.
	ReactiveMelee
)

// CapInteraction is the target-cap contract of a secondary spatial
// effect.
type CapInteraction uint8

const (
	// CapPrimaryOnly: applies only to the action's primary target.
	CapPrimaryOnly CapInteraction = iota
	// CapShared: shares the triggering action's resolved target cap.
	CapShared
	// CapExact: exactly one monster/player regardless of cap.
	CapExactOne
	// CapHard: independent hard ceiling (4 monsters / 3 players).
	CapHard
	// CapNone: non-damaging; no ADR-0018 cap applies.
	CapNone
)

// SpatialDef is a compiled Secondary Spatial Effects row.
type SpatialDef struct {
	ID             string
	Kind           SpatialKind
	RadiusMM       int64 // circle forms
	DistanceMM     int64 // push/pull magnitudes
	MaxDistanceMM  int64 // pull cap
	DurationMs     int64 // airborne lifetime
	ApexMM         int64 // airborne presentation apex
	TriggerRangeMM int64 // ReactiveMelee eligibility range
	Cap            CapInteraction
	ForcedPosition bool   // creates PULL/KNOCKBACK/AIRBORNE/displacement
	EffectID       string // status/template applied by the contact/splash
}

// spatialTable is the complete launch secondary-spatial set.
var spatialTable = map[string]SpatialDef{
	"spatial.effect.basic.area_splash_50": {
		ID: "spatial.effect.basic.area_splash_50", Kind: SplashCircle,
		RadiusMM: 1200, Cap: CapShared, EffectID: "effect.basic.area_splash_50",
	},
	"spatial.skill.thuy.active.luu_bo.contact": {
		ID: "spatial.skill.thuy.active.luu_bo.contact", Kind: ContactSweep,
		Cap: CapExactOne, EffectID: "effect.skill.thuy.chill_3s",
	},
	"spatial.skill.thuy.basic.am_luu.knockback": {
		ID: "spatial.skill.thuy.basic.am_luu.knockback", Kind: PushAlong,
		DistanceMM: 1000, Cap: CapPrimaryOnly, ForcedPosition: true,
	},
	"spatial.skill.thuy.active.trieu_quyen.pull": {
		ID: "spatial.skill.thuy.active.trieu_quyen.pull", Kind: PullToCenter,
		MaxDistanceMM: 2600, Cap: CapShared, ForcedPosition: true,
	},
	"spatial.skill.moc.active.van_moc_hoi_sinh.knockback": {
		ID: "spatial.skill.moc.active.van_moc_hoi_sinh.knockback", Kind: RadialPush,
		DistanceMM: 1500, Cap: CapShared, ForcedPosition: true,
	},
	"spatial.skill.tho.active.thach_kich.knockback": {
		ID: "spatial.skill.tho.active.thach_kich.knockback", Kind: PushAlong,
		DistanceMM: 3500, Cap: CapPrimaryOnly, ForcedPosition: true,
	},
	"spatial.skill.tho.active.dia_chan.airborne": {
		ID: "spatial.skill.tho.active.dia_chan.airborne", Kind: AirborneLift,
		DurationMs: 800, ApexMM: 1200, Cap: CapShared, ForcedPosition: true,
	},
	"spatial.skill.hoa.passive.du_hoa.explosion": {
		ID: "spatial.skill.hoa.passive.du_hoa.explosion", Kind: ExplosionCircle,
		RadiusMM: 2000, Cap: CapHard,
	},
	"spatial.skill.tho.passive.son_ha_ho_the.aura": {
		ID: "spatial.skill.tho.passive.son_ha_ho_the.aura", Kind: AuraCircle,
		RadiusMM: 4000, Cap: CapNone,
	},
	"spatial.reactive.melee_source": {
		ID: "spatial.reactive.melee_source", Kind: ReactiveMelee,
		TriggerRangeMM: 3000, Cap: CapPrimaryOnly,
	},
}

// Spatial returns the compiled secondary spatial row.
func Spatial(id string) (SpatialDef, bool) {
	s, ok := spatialTable[id]
	return s, ok
}

// forcedPositionStatus marks canonical statuses that are themselves
// forced-position effects (AIRBORNE; control statuses are not).
var forcedPositionStatus = map[string]bool{
	"effect.skill.tho.airborne_800ms": true,
}

// RequestKind is the downstream consumer of an emitted effect request.
type RequestKind uint8

const (
	// ReqStatus applies a canonical status template (status pipeline).
	ReqStatus RequestKind = iota
	// ReqShield grants a shield template (shield pipeline).
	ReqShield
	// ReqZone spawns a scheduled zone (zone scheduler).
	ReqZone
	// ReqSpatial executes a secondary spatial effect.
	ReqSpatial
	// ReqResourceMP restores MP on the caster (basic_1 +2).
	ReqResourceMP
	// ReqDamage is a direct damage component request.
	ReqDamage
	// ReqBarrier places the CAT-002 barrier entity.
	ReqBarrier
)

// EffectRequest is one typed request produced by resolving a payload
// or proc. Downstream pipelines own the semantics; the request carries
// the resolved ids, scaling and target role.
type EffectRequest struct {
	Kind RequestKind
	// RefID is the effect_id / spatial_effect_id / zone_id.
	RefID string
	// Coefficient is the scaled DAMAGE coefficient or HEAL attack
	// coefficient (already multiplied by the level scale).
	Coefficient float64
	// Ratio is the HEAL target MAX_HP ratio or EXECUTE threshold.
	Ratio float64
	// Bonus is the EXECUTE source additive multiplier.
	Bonus float64
	// AmountMP is the flat MP restoration for RESOURCE_CHANGE.
	AmountMP int64
	// FirstHitOnly restricts the request to the first connected
	// target of the action.
	FirstHitOnly bool
	// Stacks is the authored stack grant (e.g. guaranteed CHILL).
	Stacks int
}

// HitRequests resolves the basic-attack effect requests for one
// connected hit: the proc roll's status set, any proc-rolled spatial,
// the guaranteed-cadence stack, and basic_1's +2 MP restore.
//
// procRolled reports whether the level's proc chance won (the caller
// supplies the roll verdict); hitOrdinal is the 1-based connected-hit
// counter for the guaranteed cadence.
func HitRequests(d *Def, level int32, procRolled bool, hitOrdinal int) []EffectRequest {
	var out []EffectRequest
	if procRolled {
		for _, id := range d.ProcEffects {
			out = append(out, EffectRequest{Kind: ReqStatus, RefID: id})
		}
		if d.ProcSpatial != "" {
			out = append(out, EffectRequest{Kind: ReqSpatial, RefID: d.ProcSpatial})
		}
	}
	if d.GuaranteedEvery > 0 && hitOrdinal > 0 && hitOrdinal%d.GuaranteedEvery == 0 {
		for _, id := range d.ProcEffects {
			if id == "effect.skill.thuy.chill_3s" {
				out = append(out, EffectRequest{Kind: ReqStatus, RefID: id, Stacks: 1})
				break
			}
		}
	}
	if d.RestoresMP > 0 {
		out = append(out, EffectRequest{Kind: ReqResourceMP, AmountMP: d.RestoresMP})
	}
	return out
}

// ActionRequests resolves an active action's payload cells into
// ordered effect requests at the given level.
func ActionRequests(d *Def, level int32) []EffectRequest {
	dmgBP := float64(DamageScaleBP(d, level)) / 10000
	supBP := float64(SupportScaleBP(d, level)) / 10000
	var out []EffectRequest
	for _, p := range d.Payloads {
		r := EffectRequest{Kind: payloadRequestKind(p.Kind), RefID: p.RefID, FirstHitOnly: p.FirstHitOnly}
		switch p.Kind {
		case PayDamage:
			r.Coefficient = p.Coefficient * dmgBP
		case PayHeal:
			r.Ratio = p.Ratio * supBP
			r.Coefficient = p.Coefficient * supBP
		case PayExecute:
			r.Ratio = p.Ratio
			r.Bonus = p.Bonus
		}
		out = append(out, r)
	}
	return out
}

func payloadRequestKind(k PayloadKind) RequestKind {
	switch k {
	case PayDamage, PayExecute:
		return ReqDamage
	case PayHeal:
		return ReqDamage // heal components resolve through the same value pipeline
	case PayStatus:
		return ReqStatus
	case PayShield:
		return ReqShield
	case PaySpatial:
		return ReqSpatial
	case PayZone:
		return ReqZone
	case PayBarrier:
		return ReqBarrier
	}
	return ReqDamage
}
