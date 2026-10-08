package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// Targeting rules (combat.md § Targeting, § Target Limits; skills.md §
// Range Measurement; ADR-0018, ADR-0047):
//
//   - all ranges measure from the authoritative skill origin
//     (caster anchor, SKILL_ORIGIN_Y = anchor + 0.9 m) to the closest
//     point of the target's authoritative hurtbox — never sprite
//     pivot/alpha/animation socket or client distance.
//   - target acquisition (202): same map instance + inside a 90° facing
//     cone within the skill's range → current_target_id stored.
//   - hard caps: at most 4 monster / 3 player victims per resolution;
//     over-cap selection is deterministic: primary target first, then
//     nearest distance, then entity id ascending.
const (
	skillOriginYOffsetMM = 900

	MaxMonsterTargets = 4
	MaxPlayerTargets  = 3
)

// Hurtbox is an authoritative AABB (mm): [minX,maxX] x [minY,maxY]
// anchored at the entity's foot-center position.
type Hurtbox struct {
	MinX, MaxX int64
	MinY, MaxY int64
}

// hurtboxOf derives the entity's authoritative hurtbox via the injected
// collider profile port.
func (s *System) hurtboxOf(e *runtime.Entity) Hurtbox {
	hw, h := s.cfg.Hurtbox(e.ID)
	x, y := int64(e.Snap.X), int64(e.Snap.Y)
	return Hurtbox{MinX: x - hw, MaxX: x + hw, MinY: y, MaxY: y + h}
}

// closestDistanceSq is the squared distance (mm²) from a point to the
// closest point of the hurtbox.
func closestDistanceSq(px, py int64, hb Hurtbox) int64 {
	var dx int64
	if px < hb.MinX {
		dx = hb.MinX - px
	} else if px > hb.MaxX {
		dx = px - hb.MaxX
	}
	var dy int64
	if py < hb.MinY {
		dy = hb.MinY - py
	} else if py > hb.MaxY {
		dy = py - hb.MaxY
	}
	return dx*dx + dy*dy
}

// inCone reports whether the hurtbox lies inside the 90° facing cone
// centered on the caster's facing direction (horizontal half-plane ±45°).
func inCone(ox, oy int64, hb Hurtbox, facing protocolv1.Facing) bool {
	// dx = horizontal gap in the facing direction; a hurtbox entirely
	// behind the caster is outside the cone.
	var dx int64
	if facing == protocolv1.Facing_FACING_RIGHT {
		if hb.MaxX < ox {
			return false
		}
		if hb.MinX > ox {
			dx = hb.MinX - ox
		}
	} else {
		if hb.MinX > ox {
			return false
		}
		if hb.MaxX < ox {
			dx = ox - hb.MaxX
		}
	}
	// |dy| <= |dx| for ±45° half-cone; compare closest-point y to origin.
	var dy int64
	if hb.MaxY < oy {
		dy = oy - hb.MaxY
	} else if hb.MinY <= oy {
		dy = 0
	} else {
		dy = hb.MinY - oy
	}
	return dy <= dx
}

// originOf returns the authoritative skill origin for a caster.
func (s *System) originOf(e *runtime.Entity) (int64, int64) {
	return int64(e.Snap.X), int64(e.Snap.Y) + skillOriginYOffsetMM
}

// TargetCap is the per-resolution victim budget of one skill level.
type TargetCap struct {
	Monsters int
	Players  int
}

// capFor resolves a skill's target cap at a level from its authored
// band table (skills.md § Target Count Limits and Scaling).
func (d *SkillDef) capFor(level int32) TargetCap {
	switch d.Band {
	case BandSingleTarget:
		return TargetCap{1, 1}
	case BandCleave:
		if level >= 6 {
			return TargetCap{3, 2}
		}
		return TargetCap{2, 1}
	case BandWide:
		if level >= 9 {
			return TargetCap{4, 3}
		}
		if level >= 5 {
			return TargetCap{3, 2}
		}
		return TargetCap{2, 1}
	case BandBasic2:
		if level >= 6 {
			return TargetCap{2, 1}
		}
		return TargetCap{1, 1}
	case BandBasic3:
		if level >= 8 {
			return TargetCap{3, 1}
		}
		if level >= 4 {
			return TargetCap{2, 1}
		}
		return TargetCap{1, 1}
	case BandBasic4:
		if level >= 8 {
			return TargetCap{4, 3}
		}
		if level >= 4 {
			return TargetCap{3, 2}
		}
		return TargetCap{2, 1}
	default:
		return TargetCap{1, 1}
	}
}

// candidate pairs one eligible entity with its resolution distance.
type candidate struct {
	id     uint64
	distSq int64
	player bool
}

// selectTargets applies the deterministic over-cap order: authoritative
// primary target first (when inside the hit geometry), then nearest
// distance, then entity id ascending — each class bounded by its cap.
func selectTargets(cands []candidate, primary uint64, cap TargetCap) []uint64 {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0; j-- {
			a, b := cands[j-1], cands[j]
			if a.distSq < b.distSq || (a.distSq == b.distSq && a.id < b.id) {
				break
			}
			cands[j-1], cands[j] = cands[j], cands[j-1]
		}
	}
	var out []uint64
	monsters, players := 0, 0
	add := func(c candidate) {
		if c.player {
			if players < cap.Players {
				players++
				out = append(out, c.id)
			}
			return
		}
		if monsters < cap.Monsters {
			monsters++
			out = append(out, c.id)
		}
	}
	for i := range cands {
		if cands[i].id == primary {
			add(cands[i])
			break
		}
	}
	for i := range cands {
		if cands[i].id == primary {
			continue
		}
		add(cands[i])
	}
	return out
}

// gatherCandidates enumerates hostile-eligible entities whose hurtbox
// intersects the action's hit geometry.
func (s *System) gatherCandidates(p *runtime.Partition, src *runtime.Entity, act *Action, def *SkillDef) []candidate {
	var areaX, areaY int32
	if act != nil {
		areaX, areaY = act.AreaX, act.AreaY
	}
	ox, oy := s.originOf(src)
	s.enumBuf = s.enumBuf[:0]
	reach := def.Geom.rangeMM() + 4000 // + hurtbox margin
	p.Grid().Within(int32(ox), int32(oy), reach, &s.enumBuf)
	var out []candidate
	for _, id := range s.enumBuf {
		if id == src.ID {
			continue
		}
		t, err := p.Entity(id)
		if err != nil || t.Snap.HP <= 0 || t.Dead {
			continue
		}
		// No friendly fire: players never damage players outside
		// explicit PvP team contexts (out of scope).
		if src.Class == runtime.ClassPlayer && t.Class == runtime.ClassPlayer {
			continue
		}
		hb := s.hurtboxOf(t)
		if !def.Geom.hits(ox, oy, src.Snap.Facing, areaX, areaY, hb) {
			continue
		}
		out = append(out, candidate{
			id:     id,
			distSq: closestDistanceSq(ox, oy, hb),
			player: t.Class == runtime.ClassPlayer,
		})
	}
	return out
}

// acquireTarget implements C2S_TARGET_INTENT: the target stays current
// only while same-instance, inside the 90° facing cone, and within the
// given skill range; otherwise clears.
func (s *System) acquireTarget(p *runtime.Partition, src *runtime.Entity, targetID uint64, def *SkillDef) bool {
	if targetID == 0 {
		src.Private.AcceptedTargetID = 0
		return true
	}
	t, err := p.Entity(targetID)
	if err != nil || t.Snap.HP <= 0 || t.Dead {
		src.Private.AcceptedTargetID = 0
		return false
	}
	ox, oy := s.originOf(src)
	hb := s.hurtboxOf(t)
	reach := int64(def.Geom.rangeMM())
	if closestDistanceSq(ox, oy, hb) > reach*reach || !inCone(ox, oy, hb, src.Snap.Facing) {
		src.Private.AcceptedTargetID = 0
		return false
	}
	src.Private.AcceptedTargetID = targetID
	return true
}
