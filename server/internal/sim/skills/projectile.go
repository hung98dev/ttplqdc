package skills

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/combat"
	"thinhthan/internal/sim/spatial/collision"
	"thinhthan/internal/sim/spatial/geometry"
)

// Projectile is a server-owned projectile flight state: spawns at the
// resolved skill origin, travels the authored max range along the
// facing axis, and terminates at max range, the first configured
// collision (hurtbox or blocking geometry/barrier), or an explicit
// effect rule.
type Projectile struct {
	X, Y        int64 // current center (mm)
	startX      int64 // launch x
	DirX        int64 // +1 right / -1 left (launch axis)
	RangeMM     int64
	SpeedMMPerS int64
	RadiusMM    int64
	ElapsedMs   int64
	Done        bool
	Hit         bool // terminated by a hurtbox contact
	Blocked     bool // terminated by blocking geometry/barrier
}

// SpawnProjectile emits a projectile at the resolved skill origin along
// the facing axis.
func SpawnProjectile(g GeometrySpec, o Origin, facing protocolv1.Facing) Projectile {
	dir := int64(1)
	if facing == protocolv1.Facing_FACING_LEFT {
		dir = -1
	}
	return Projectile{
		X: o.X, Y: o.Y, startX: o.X, DirX: dir,
		RangeMM:     g.Projectile.RangeMM,
		SpeedMMPerS: g.Projectile.SpeedMMPerS,
		RadiusMM:    g.Projectile.RadiusMM,
	}
}

// Traveled returns the horizontal distance travelled (mm).
func (p *Projectile) Traveled() int64 {
	d := p.X - p.startX
	if d < 0 {
		return -d
	}
	return d
}

// Advance moves the flight forward dtMs, terminating at the authored
// max range, a hurtbox contact (radius + 1mm contact epsilon), or
// blocking geometry / barrier AABB. It mutates the flight and returns
// whether it is still in flight.
func (p *Projectile) Advance(dtMs int64, w *collision.World, barriers *BarrierSet, tick uint64, hurtboxes []combat.Hurtbox) bool {
	if p.Done {
		return false
	}
	p.ElapsedMs += dtMs
	travel := geometry.RoundDiv(p.SpeedMMPerS*p.ElapsedMs, 1000)
	if travel >= p.RangeMM {
		travel = p.RangeMM
		p.Done = true
	}
	nextX := p.startX + p.DirX*travel
	// Blocking geometry over the swept span (per-tick motion is small,
	// so a span-box test is exact): first configured collision wins.
	span := hitBox(p.X, p.Y, p.RadiusMM)
	next := hitBox(nextX, p.Y, p.RadiusMM)
	swept := collision.AABB{
		MinX: min64(span.MinX, next.MinX), MaxX: max64(span.MaxX, next.MaxX),
		MinY: span.MinY, MaxY: span.MaxY,
	}
	if w != nil && overlapsSolid(w, swept) {
		p.Done, p.Blocked = true, true
		return false
	}
	if barriers != nil && barriers.BlockedBy(swept, tick) {
		p.Done, p.Blocked = true, true
		return false
	}
	p.X = nextX
	// Hurtbox contact: center expanded by radius + epsilon.
	for _, hb := range hurtboxes {
		if hitBox(p.X, p.Y, p.RadiusMM+ContactEpsilonMM).Intersects(
			collision.AABB{MinX: hb.MinX, MaxX: hb.MaxX, MinY: hb.MinY, MaxY: hb.MaxY}) {
			p.Done, p.Hit = true, true
			return false
		}
	}
	return !p.Done
}

func hitBox(cx, cy, r int64) collision.AABB {
	return collision.AABB{MinX: cx - r, MaxX: cx + r, MinY: cy - r, MaxY: cy + r}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
