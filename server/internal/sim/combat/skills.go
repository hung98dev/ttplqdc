package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/progression"
)

// TargetBand selects the per-level target-cap table of skills.md §
// Target Count Limits and Scaling.
type TargetBand uint8

const (
	BandSingleTarget TargetBand = iota // single-target & dash actives: 1/1
	BandCleave                         // Lv8/14/22 actives: 2/1 -> 3/2
	BandWide                           // Lv32/45 actives: 2/1 -> 3/2 -> 4/3
	BandBasic1                         // 1/1
	BandBasic2                         // 1/1 -> 2/1
	BandBasic3                         // 1/1 -> 2/1 -> 3/1
	BandBasic4                         // 2/1 -> 3/2 -> 4/3
)

// TimingSpeed selects which speed stat scales STARTUP/RECOVERY
// (skills.md § Attack Speed & Cast Speed).
type TimingSpeed uint8

const (
	TimingNone TimingSpeed = iota
	TimingAttackSpeed
	TimingCastSpeed
)

// CostTiming selects when MP is committed (skills.md § Commit Rules).
type CostTiming uint8

const (
	CostOnStart CostTiming = iota
	CostOnCastEnd
)

// GeometryKind enumerates the typed hit geometries of skills.md §
// Geometry & Reach.
type GeometryKind uint8

const (
	GeomSelf              GeometryKind = iota // SELF: caster only
	GeomMeleeBox                              // origin=facing dir, len=Amm halfheight=Bmm
	GeomDirectionBox                          // same, projectile-less direction box
	GeomProjectile                            // range Amm, speed Bmm/tick? radius Cmm
	GeomAreaSelf                              // radius Amm around caster
	GeomAreaPosition                          // range Amm to cast point, radius Bmm
	GeomSingleTargetRange                     // locked target within Amm
	GeomDashLine                              // dash distance Amm, halfwidth Bmm
	GeomMoveLine                              // movement line Amm, halfwidth Bmm
	GeomMoveContact                           // contact while moving
	GeomBarrier                               // no hit geometry (placement)
)

// Geometry is the typed hit shape (all distances mm).
type Geometry struct {
	Kind       GeometryKind
	A, B, C    int64
	RangeMM    int64 // outer reach for grid prefilter / target intent
	RequiresLo bool  // requires locked target (SINGLE_TARGET forms)
}

// rangeMM is the outer reach used for the spatial prefilter.
func (g Geometry) rangeMM() int64 {
	r := g.RangeMM
	if g.A > r {
		r = g.A
	}
	return r
}

// hits reports whether the hurtbox intersects the hit geometry.
// ox,oy = skill origin (mm); areaX,areaY = cast point for AREA_POSITION.
func (g Geometry) hits(ox, oy int64, facing protocolv1.Facing, areaX, areaY int32, hb Hurtbox) bool {
	switch g.Kind {
	case GeomSelf:
		return false // caster never damages itself through SELF
	case GeomMeleeBox, GeomDirectionBox:
		var minX, maxX int64
		if facing == protocolv1.Facing_FACING_RIGHT {
			minX, maxX = ox, ox+g.A
		} else {
			minX, maxX = ox-g.A, ox
		}
		minY, maxY := oy-g.B, oy+g.B
		return hb.MaxX >= minX && hb.MinX <= maxX && hb.MaxY >= minY && hb.MinY <= maxY
	case GeomProjectile:
		return closestDistanceSq(ox, oy, hb) <= (g.A+g.C)*(g.A+g.C)
	case GeomAreaSelf:
		return closestDistanceSq(ox, oy, hb) <= g.A*g.A
	case GeomAreaPosition:
		ax, ay := int64(areaX), int64(areaY)
		return closestDistanceSq(ax, ay, hb) <= g.B*g.B
	case GeomSingleTargetRange:
		return closestDistanceSq(ox, oy, hb) <= g.A*g.A
	case GeomDashLine, GeomMoveLine, GeomMoveContact:
		var minX, maxX int64
		if facing == protocolv1.Facing_FACING_RIGHT {
			minX, maxX = ox, ox+g.A
		} else {
			minX, maxX = ox-g.A, ox
		}
		minY, maxY := oy-g.B, oy+g.B
		return hb.MaxX >= minX && hb.MinX <= maxX && hb.MaxY >= minY && hb.MinY <= maxY
	default:
		return false
	}
}

// SkillDef is the content projection combat consumes. Rows come from
// the compiled skill catalog (class_skill_catalog.md); combat treats
// them as read-only.
type SkillDef struct {
	SkillID     string
	Level       int32
	IsBasic     bool
	Band        TargetBand
	Timing      TimingSpeed
	CostTiming  CostTiming
	CostMP      int64
	CooldownMs  int64
	StartupMs   int64
	ActiveMs    int64
	RecoveryMs  int64
	MotionMs    int64 // authored forced-motion span; 0 = none
	Interrupts  uint16
	Geom        Geometry
	Coefficient float64
	FlatDamage  int64
	Element     protocolv1.Element
	NoCrit      bool
	NoReflect   bool
	NoLifesteal bool
	NoProc      bool
	Hostile     bool // damages targets (non-hostile skills skip JG etc.)
	RequiresTgt bool
}

// SkillSource resolves skill content at a level.
type SkillSource interface {
	Lookup(skillID string) (SkillDef, bool)
}

// StatSource resolves the authoritative stat projection for an entity.
type StatSource interface {
	StatsOf(entityID uint64) (progression.Stats, bool)
}
