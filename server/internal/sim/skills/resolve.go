package skills

import (
	"fmt"

	"thinhthan/internal/sim/combat"
	"thinhthan/internal/sim/spatial/geometry"
)

// Level bounds (skills.md § Skill Level).
const (
	MinLevel   = 1
	MaxLevel   = 12
	LevelSteps = 11
)

// EffectivePhaseMs is the timing-speed scaling of skills.md § Timing
// Speed Stat: ceil(base_ms / (1 + speed_stat)) applied to startup and
// recovery only. The arithmetic matches combat.effectivePhaseMs.
func EffectivePhaseMs(baseMs int64, speed float64) int64 {
	if speed < 0 {
		speed = 0
	}
	num := baseMs * 1000
	den := int64((1 + speed) * 1000)
	return (num + den - 1) / den
}

// CooldownMs resolves the per-level cooldown of skills.md / catalog §
// Skill-Level Scaling Model, integer milliseconds:
//
//	basic:  max(round_half_up(1000*max_cd), round_half_up(1000*(base_cd - cd_step*step)))
//	active: base_ms * (1 - 0.030*step)
func CooldownMs(d *Def, level int32) int64 {
	step := levelStep(level)
	if d.Kind == KindBasic {
		cand := geometry.RoundDiv(d.BaseCooldownMs*1000-d.CooldownStepUs*int64(step), 1000)
		if d.MaxCooldownMs > cand {
			return d.MaxCooldownMs
		}
		return cand
	}
	return geometry.RoundDiv(d.BaseCooldownMs*(100-3*int64(step)), 100)
}

// DamageScaleBP is damage_scale(S) in basis points: 1+0.040*step for
// basics, 1+0.035*step for actives.
func DamageScaleBP(d *Def, level int32) int64 {
	step := int64(levelStep(level))
	if d.Kind == KindBasic {
		return 10000 + 400*step
	}
	return 10000 + 350*step
}

// SupportScaleBP is support_scale(S) in basis points: 1+0.025*step.
func SupportScaleBP(_ *Def, level int32) int64 {
	return 10000 + 250*int64(levelStep(level))
}

// ProcBP is the basic proc chance in basis points at a level:
// round_half_up(10000*(base + (max-base)*step/11)).
func ProcBP(d *Def, level int32) int64 {
	step := int64(levelStep(level))
	return d.BaseProcBP + geometry.RoundDiv((d.MaxProcBP-d.BaseProcBP)*step, 11)
}

// BasicPhases returns the per-level scaled timing phases of a basic
// attack (skills.md § Authoritative basic interval):
//
//	phase_scale(S) = cooldown_seconds(S)/base_cooldown
//	startup(S) = ceil(base_startup * scale); active(S) = max(40, ceil(...));
//	recovery(S) = ceil(base_recovery * scale)
//
// Values carry millisecond precision (the scaled cooldown ratio is
// applied on micro-milliseconds to stay exact).
func BasicPhases(d *Def, level int32) (startup, active, recovery int64) {
	scaleNum := CooldownMs(d, level) // cd_ms(S) = cooldown_seconds(S)*1000
	scaleDen := d.BaseCooldownMs
	scale := func(v int64) int64 {
		return geometry.RoundDiv(v*scaleNum*1000, scaleDen*1000)
	}
	startup = scale(d.StartupMs)
	active = scale(d.ActiveMs)
	if active < 40 {
		active = 40
	}
	recovery = scale(d.RecoveryMs)
	return
}

// BasicIntervalMs is the authoritative basic interval (skills.md §
// Authoritative basic interval):
//
//	interval_ms = max(ceil(cooldown_ms(S)/(1+AS)),
//	                  effective_startup_ms + active_ms(S),
//	                  effective_startup_ms + authored_motion_ms)
func BasicIntervalMs(d *Def, level int32, attackSpeed float64) int64 {
	startup, active, _ := BasicPhases(d, level)
	effStartup := EffectivePhaseMs(startup, attackSpeed)
	interval := EffectivePhaseMs(CooldownMs(d, level), attackSpeed)
	if m := effStartup + active; interval < m {
		interval = m
	}
	if m := effStartup + motionMs(d); interval < m {
		interval = m
	}
	return interval
}

// NextAcceptTick is the basic self-chain deadline:
// accepted_tick + ceil(interval_ms / 50).
func NextAcceptTick(d *Def, level int32, attackSpeed float64, acceptedTick uint64) uint64 {
	return acceptedTick + ceilTick(BasicIntervalMs(d, level, attackSpeed))
}

// Deadlines is the resolved action deadline set of skills.md § Action
// Timing: millisecond dues are summed first, then each converts to the
// first 50ms tick at or after the due time.
type Deadlines struct {
	EffectiveStartupMs  int64
	EffectiveActiveMs   int64 // unscaled authored active span
	EffectiveRecoveryMs int64
	ActiveStartTick     uint64
	ActiveEndTick       uint64
	RecoveryEndTick     uint64
	MotionEndTick       uint64 // 0 = no authored motion
	CooldownEndTick     uint64
}

// ResolveDeadlines computes the action deadlines for a def at a level
// with the caster's speed stat. Actives use authored phases scaled by
// the declared speed stat; basics use the level-scaled phases.
func ResolveDeadlines(d *Def, level int32, speed float64, acceptedTick uint64) Deadlines {
	var startup, active, recovery int64
	if d.Kind == KindBasic {
		startup, active, recovery = BasicPhases(d, level)
	} else {
		startup, active, recovery = d.StartupMs, d.ActiveMs, d.RecoveryMs
	}
	effStartup := EffectivePhaseMs(startup, speed)
	effRecovery := EffectivePhaseMs(recovery, speed)
	acceptedMs := int64(acceptedTick) * 50
	activeDue := acceptedMs + effStartup
	recoveryDue := activeDue + active
	completeDue := recoveryDue + effRecovery
	dl := Deadlines{
		EffectiveStartupMs:  effStartup,
		EffectiveActiveMs:   active,
		EffectiveRecoveryMs: effRecovery,
		ActiveStartTick:     ceilTick(activeDue),
		ActiveEndTick:       ceilTick(recoveryDue),
		RecoveryEndTick:     ceilTick(completeDue),
		CooldownEndTick:     acceptedTick + ceilTick(CooldownMs(d, level)),
	}
	if m := motionMs(d); m > 0 {
		dl.MotionEndTick = ceilTick(activeDue + m)
	}
	return dl
}

// motionMs is the authored forced-motion span (dash/move lines), 0 for
// everything else.
func motionMs(d *Def) int64 {
	switch d.Geom.Kind {
	case combat.GeomDashLine, combat.GeomMoveContact:
		return d.Geom.Line.DurationMs
	case combat.GeomMoveLine:
		return d.Geom.Move.DurationMs
	}
	return 0
}

// ceilTick converts a millisecond due to the first 50ms tick at or
// after it.
func ceilTick(dueMs int64) uint64 {
	if dueMs <= 0 {
		return 0
	}
	return uint64((dueMs + 49) / 50)
}

func levelStep(level int32) int32 {
	if level < MinLevel {
		return 0
	}
	if level > MaxLevel {
		return MaxLevel - 1
	}
	return level - 1
}

// CombatDef projects the compiled catalog row onto combat's action
// contract (combat.SkillDef). Timing fields carry the level-resolved
// cooldown; geometry projects to the combat.Geometry convention
// (A = primary extent, B = half-height/secondary, C = hit radius).
func CombatDef(d *Def, level int32) combat.SkillDef {
	cd := combat.SkillDef{
		SkillID:     d.ID,
		Level:       level,
		IsBasic:     d.IsBasic(),
		Band:        d.Band,
		Timing:      d.Speed,
		CostTiming:  combat.CostOnStart,
		CostMP:      d.CostMP,
		CooldownMs:  CooldownMs(d, level),
		Element:     d.Element,
		Hostile:     d.Hostile(),
		RequiresTgt: d.Targeting == TargetSingle,
	}
	if d.Kind == KindBasic {
		cd.StartupMs, cd.ActiveMs, cd.RecoveryMs = BasicPhases(d, level)
		cd.Coefficient = d.BaseCoefficient * float64(DamageScaleBP(d, level)) / 10000
	} else {
		cd.StartupMs, cd.ActiveMs, cd.RecoveryMs = d.StartupMs, d.ActiveMs, d.RecoveryMs
		if c := directCoefficient(d); c > 0 {
			cd.Coefficient = c * float64(DamageScaleBP(d, level)) / 10000
		}
	}
	cd.MotionMs = motionMs(d)
	cd.Geom = projectGeom(d.Geom)
	return cd
}

// directCoefficient returns the direct DAMAGE coefficient (r of the
// first DAMAGE payload), 0 when the action has none.
func directCoefficient(d *Def) float64 {
	for _, p := range d.Payloads {
		if p.Kind == PayDamage {
			return p.Coefficient
		}
	}
	return 0
}

// projectGeom maps the typed GeometrySpec to combat's flattened
// Geometry: A = reach/range/cast, B = half-height or area radius,
// C = projectile hit radius. RangeMM is the outer reach.
func projectGeom(g GeometrySpec) combat.Geometry {
	out := combat.Geometry{Kind: g.Kind}
	switch g.Kind {
	case combat.GeomMeleeBox, combat.GeomDirectionBox:
		out.A, out.B = g.Box.ReachMM, g.Box.HalfHeightMM
		out.RangeMM = g.Box.ReachMM
	case combat.GeomProjectile:
		out.A, out.B, out.C = g.Projectile.RangeMM, g.Projectile.SpeedMMPerS, g.Projectile.RadiusMM
		out.RangeMM = g.Projectile.RangeMM + g.Projectile.RadiusMM
	case combat.GeomAreaSelf:
		out.A = g.Circle.RadiusMM
		out.RangeMM = g.Circle.RadiusMM
	case combat.GeomAreaPosition:
		out.A, out.B = g.Area.CastMM, g.Area.RadiusMM
		out.RangeMM = g.Area.CastMM + g.Area.RadiusMM
	case combat.GeomSingleTargetRange:
		out.A = g.Range.RangeMM
		out.RangeMM = g.Range.RangeMM
		out.RequiresLo = true
	case combat.GeomDashLine, combat.GeomMoveContact:
		out.A, out.B = g.Line.DistanceMM, g.Line.HitHalfHeightMM
		out.RangeMM = g.Line.DistanceMM
	case combat.GeomMoveLine:
		out.A = g.Move.DistanceMM
		out.RangeMM = g.Move.DistanceMM
	case combat.GeomBarrier:
		out.A = g.Barrier.CastMM
		out.RangeMM = g.Barrier.CastMM
	}
	return out
}

// validateDef enforces the static content validation rules of
// skills.md § Validation that are checkable on the compiled row.
func validateDef(d *Def) error {
	if d.ID == "" || d.ClassID == "" {
		return fmt.Errorf("missing identity fields")
	}
	if d.StartupMs < 0 || d.ActiveMs < 0 || d.RecoveryMs < 0 {
		return fmt.Errorf("negative timing")
	}
	if err := ValidateEnvelope(d.Geom); err != nil {
		return err
	}
	switch d.Geom.Kind {
	case combat.GeomMeleeBox, combat.GeomDirectionBox:
		if d.Geom.Box == nil || d.Geom.Box.ReachMM <= 0 || d.Geom.Box.HalfHeightMM <= 0 {
			return fmt.Errorf("invalid box geometry")
		}
	case combat.GeomProjectile:
		if d.Geom.Projectile == nil || d.Geom.Projectile.RangeMM <= 0 ||
			d.Geom.Projectile.SpeedMMPerS <= 0 || d.Geom.Projectile.RadiusMM <= 0 {
			return fmt.Errorf("invalid projectile geometry")
		}
	case combat.GeomAreaSelf:
		if d.Geom.Circle == nil || d.Geom.Circle.RadiusMM <= 0 {
			return fmt.Errorf("invalid area-self geometry")
		}
	case combat.GeomAreaPosition:
		if d.Geom.Area == nil || d.Geom.Area.CastMM <= 0 || d.Geom.Area.RadiusMM <= 0 {
			return fmt.Errorf("invalid area-position geometry")
		}
	case combat.GeomSingleTargetRange:
		if d.Geom.Range == nil || d.Geom.Range.RangeMM <= 0 {
			return fmt.Errorf("invalid single-target geometry")
		}
	case combat.GeomDashLine, combat.GeomMoveContact:
		if d.Geom.Line == nil || d.Geom.Line.DistanceMM <= 0 || d.Geom.Line.DurationMs <= 0 ||
			d.Geom.Line.HitHalfHeightMM <= 0 {
			return fmt.Errorf("invalid line geometry")
		}
	case combat.GeomMoveLine:
		if d.Geom.Move == nil || d.Geom.Move.DistanceMM <= 0 || d.Geom.Move.DurationMs <= 0 {
			return fmt.Errorf("invalid move geometry")
		}
	case combat.GeomBarrier:
		b := d.Geom.Barrier
		if b == nil || b.CastMM <= 0 || b.ThicknessMM <= 0 || b.HeightMM <= 0 || b.DurationMs <= 0 {
			return fmt.Errorf("invalid barrier geometry")
		}
		if !hasPayloadKind(d, PayBarrier) {
			return fmt.Errorf("BARRIER_POSITION without BARRIER payload")
		}
	case combat.GeomSelf:
		// no spatial parameters
	default:
		return fmt.Errorf("unknown geometry kind %d", d.Geom.Kind)
	}
	if d.Geom.Kind != combat.GeomBarrier && hasPayloadKind(d, PayBarrier) {
		return fmt.Errorf("BARRIER payload requires BARRIER_POSITION geometry")
	}
	if d.Tags.Has(TagDisplacement) && !hasForcedPositionEffect(d) {
		return fmt.Errorf("DISPLACEMENT tag without forced-position effect")
	}
	if !d.Tags.Has(TagDisplacement) && hasForcedPositionEffect(d) {
		return fmt.Errorf("forced-position effect without DISPLACEMENT tag")
	}
	if d.Tags.Has(TagShield) && !hasPayloadKind(d, PayShield) {
		return fmt.Errorf("SHIELD tag without shield effect")
	}
	if d.Tags.Has(TagHeal) && !hasHeal(d) {
		return fmt.Errorf("HEAL tag without positive heal effect")
	}
	if d.Tags.Has(TagProjectile) && d.Geom.Kind != combat.GeomProjectile {
		return fmt.Errorf("PROJECTILE tag without projectile geometry")
	}
	if d.Tags.Has(TagBasicAttack) && d.Kind != KindBasic {
		return fmt.Errorf("BASIC_ATTACK tag on non-basic")
	}
	if d.Kind == KindBasic {
		if d.BaseCoefficient <= 0 {
			return fmt.Errorf("basic missing base_coefficient")
		}
		if len(d.ProcEffects) == 0 {
			return fmt.Errorf("basic missing proc effects")
		}
	}
	return nil
}

func hasPayloadKind(d *Def, k PayloadKind) bool {
	for _, p := range d.Payloads {
		if p.Kind == k {
			return true
		}
	}
	return false
}

func hasHeal(d *Def) bool {
	for _, p := range d.Payloads {
		if p.Kind == PayHeal && (p.Ratio > 0 || p.Coefficient > 0) {
			return true
		}
		if p.Kind == PayZone {
			if z, ok := zoneTable[p.RefID]; ok {
				for _, zp := range z.Payloads {
					if zp.Kind == PayHeal && (zp.Ratio > 0 || zp.Coefficient > 0) {
						return true
					}
				}
			}
		}
	}
	return false
}

// hasForcedPositionEffect reports whether the action can create a
// forced-position result (KNOCKBACK/PULL/TELEPORT spatial or a
// canonical AIRBORNE status).
func hasForcedPositionEffect(d *Def) bool {
	for _, p := range d.Payloads {
		switch p.Kind {
		case PaySpatial:
			if s, ok := spatialTable[p.RefID]; ok && s.ForcedPosition {
				return true
			}
		case PayStatus:
			if forcedPositionStatus[p.RefID] {
				return true
			}
		}
	}
	if d.ProcSpatial != "" {
		if s, ok := spatialTable[d.ProcSpatial]; ok && s.ForcedPosition {
			return true
		}
	}
	for _, id := range d.ProcEffects {
		if forcedPositionStatus[id] {
			return true
		}
	}
	return false
}
