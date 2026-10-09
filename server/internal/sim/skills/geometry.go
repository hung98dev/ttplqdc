package skills

import (
	"errors"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/combat"
	"thinhthan/internal/sim/spatial/collision"
)

// Measurement constants (skills.md § Range Measurement and Collider
// Intersection; physics_geometry_contract.md § quantization).
const (
	// SkillOriginYOffsetMM: SKILL_ORIGIN_Y = caster_anchor_y + 0.9m.
	SkillOriginYOffsetMM = int64(900)
	// ContactEpsilonMM: gap <= 0.001m counts as contact; >= 0.002m does not.
	ContactEpsilonMM = int64(1)
	// MaxPositionedOuterReachMM: cast + radius cap for positioned circles.
	MaxPositionedOuterReachMM = int64(11000)
	// MaxProjectileEnvelopeMM: max_range + hit_radius cap.
	MaxProjectileEnvelopeMM = int64(8800)
)

// Origin is the authoritative skill origin: the caster anchor from the
// accepting/resolving tick lifted by SKILL_ORIGIN_Y. Never an animation
// socket.
type Origin struct {
	X, Y int64
}

// SkillOrigin derives the authoritative origin from the caster anchor
// (authoritative foot-center of the collider profile).
func SkillOrigin(anchorX, anchorY int64) Origin {
	return Origin{X: anchorX, Y: anchorY + SkillOriginYOffsetMM}
}

// ResolveBox returns the world-space AABB of a forward box, swept line
// or barrier form at the given origin/facing. For line forms this is
// the authored maximal span (actual sweeps truncate on collision; see
// sweep.go).
func ResolveBox(g GeometrySpec, o Origin, facing protocolv1.Facing) collision.AABB {
	var reach, hh int64
	switch g.Kind {
	case combat.GeomMeleeBox, combat.GeomDirectionBox:
		reach, hh = g.Box.ReachMM, g.Box.HalfHeightMM
	case combat.GeomDashLine, combat.GeomMoveContact:
		reach, hh = g.Line.DistanceMM, g.Line.HitHalfHeightMM
	case combat.GeomMoveLine:
		reach, hh = g.Move.DistanceMM, 0
	default:
		return collision.AABB{}
	}
	b := collision.AABB{MinY: o.Y - hh, MaxY: o.Y + hh}
	if facing == protocolv1.Facing_FACING_LEFT {
		b.MinX, b.MaxX = o.X-reach, o.X
	} else {
		b.MinX, b.MaxX = o.X, o.X+reach
	}
	return b
}

// ResolveCircle returns the world-space circle (cx, cy, r) of AREA_SELF
// (centered on the skill origin) or AREA_POSITION (centered on the
// validated cast point).
func ResolveCircle(g GeometrySpec, o Origin, castX, castY int64) (cx, cy, r int64) {
	switch g.Kind {
	case combat.GeomAreaSelf:
		return o.X, o.Y, g.Circle.RadiusMM
	case combat.GeomAreaPosition:
		return castX, castY, g.Area.RadiusMM
	}
	return 0, 0, 0
}

// ClosestPointDistanceSq is the squared distance (mm²) from (px,py) to
// the closest point of the hurtbox.
func ClosestPointDistanceSq(px, py int64, hb combat.Hurtbox) int64 {
	var dx, dy int64
	if px < hb.MinX {
		dx = hb.MinX - px
	} else if px > hb.MaxX {
		dx = px - hb.MaxX
	}
	if py < hb.MinY {
		dy = hb.MinY - py
	} else if py > hb.MaxY {
		dy = py - hb.MaxY
	}
	return dx*dx + dy*dy
}

// Hits reports whether the authoritative hurtbox intersects the
// resolved shape under the 1mm contact epsilon: a hurtbox at most 1mm
// outside the authored boundary still connects. SELF shapes never hit
// hurtboxes (the caster is selected by targeting, not geometry);
// projectiles resolve per-flight in projectile.go.
func Hits(g GeometrySpec, o Origin, facing protocolv1.Facing, castX, castY int64, hb combat.Hurtbox) bool {
	eps := ContactEpsilonMM
	switch g.Kind {
	case combat.GeomMeleeBox, combat.GeomDirectionBox, combat.GeomDashLine,
		combat.GeomMoveContact, combat.GeomMoveLine:
		b := ResolveBox(g, o, facing)
		return hb.MaxX >= b.MinX-eps && hb.MinX <= b.MaxX+eps &&
			hb.MaxY >= b.MinY-eps && hb.MinY <= b.MaxY+eps
	case combat.GeomAreaSelf, combat.GeomAreaPosition, combat.GeomSingleTargetRange:
		var cx, cy, r int64
		if g.Kind == combat.GeomSingleTargetRange {
			cx, cy, r = o.X, o.Y, g.Range.RangeMM
		} else {
			cx, cy, r = ResolveCircle(g, o, castX, castY)
		}
		reach := r + eps
		return ClosestPointDistanceSq(cx, cy, hb) <= reach*reach
	}
	return false
}

// OuterReachMM returns the authored outer reach used for the launch
// reach band audit and the spatial prefilter.
func OuterReachMM(g GeometrySpec) int64 {
	switch g.Kind {
	case combat.GeomMeleeBox, combat.GeomDirectionBox:
		return g.Box.ReachMM
	case combat.GeomProjectile:
		return g.Projectile.RangeMM + g.Projectile.RadiusMM
	case combat.GeomAreaSelf:
		return g.Circle.RadiusMM
	case combat.GeomAreaPosition:
		return g.Area.CastMM + g.Area.RadiusMM
	case combat.GeomSingleTargetRange:
		return g.Range.RangeMM
	case combat.GeomDashLine, combat.GeomMoveContact:
		return g.Line.DistanceMM
	case combat.GeomMoveLine:
		return g.Move.DistanceMM
	case combat.GeomBarrier:
		return g.Barrier.CastMM
	}
	return 0
}

// ValidateEnvelope applies the compile-time envelope caps of
// skills.md § Validation: projectile max_range + hit_radius <= 8.8m;
// positioned circle cast_range + radius <= 11.0m.
func ValidateEnvelope(g GeometrySpec) error {
	switch g.Kind {
	case combat.GeomProjectile:
		if g.Projectile.RangeMM+g.Projectile.RadiusMM > MaxProjectileEnvelopeMM {
			return errors.New("skills: projectile range+radius exceeds 8.8m envelope")
		}
	case combat.GeomAreaPosition:
		if g.Area.CastMM+g.Area.RadiusMM > MaxPositionedOuterReachMM {
			return errors.New("skills: positioned cast+radius exceeds 11.0m outer reach")
		}
	}
	return nil
}
