package skills

import "thinhthan/internal/sim/combat"

// Hard server target caps (skills.md § Target Count Limits, ADR-0018).
const (
	MaxMonsterTargets = 4
	MaxPlayerTargets  = 3
)

// Cap is the per-resolution victim budget.
type Cap struct {
	Monsters int
	Players  int
}

// CapFor resolves the authored band's cap at a skill level — the exact
// tables of skills.md § Target Count Limits and Scaling and
// class_skill_catalog.md § Target Scaling Tables:
//
//	single-target / dash actives: 1/1
//	cleave actives (Lv8/14/22):   2/1 -> 3/2 at Lv6
//	wide actives (Lv32/45):       2/1 -> 3/2 -> 4/3 at Lv5/Lv9
//	basic_1, basic_4:             1/1
//	basic_2:                      1/1 -> 2/1 at Lv6
//	basic_3:                      1/1 -> 2/1 -> 4/3 at Lv4/Lv8
func CapFor(d *Def, level int32) Cap {
	switch d.Band {
	case combat.BandCleave:
		if level >= 6 {
			return Cap{3, 2}
		}
		return Cap{2, 1}
	case combat.BandWide:
		if level >= 9 {
			return Cap{4, 3}
		}
		if level >= 5 {
			return Cap{3, 2}
		}
		return Cap{2, 1}
	case combat.BandBasic2:
		if level >= 6 {
			return Cap{2, 1}
		}
		return Cap{1, 1}
	case combat.BandBasic3:
		if level >= 8 {
			return Cap{4, 3}
		}
		if level >= 4 {
			return Cap{2, 1}
		}
		return Cap{1, 1}
	default:
		// BandSingleTarget, BandBasic1, BandBasic4: flat 1/1.
		return Cap{1, 1}
	}
}

// Candidate pairs an eligible entity with its resolution distance for
// deterministic selection.
type Candidate struct {
	ID       uint64
	DistSq   int64
	IsPlayer bool
}

// Select applies the canonical over-cap order: the authoritative
// primary target first (when eligible), then nearest distance, then
// entity id ascending — each class bounded by the cap.
func Select(cands []Candidate, primary uint64, cap Cap) []uint64 {
	// insertion sort by (distSq, id) — stable and allocation-free for
	// the small candidate sets this domain produces.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0; j-- {
			a, b := cands[j-1], cands[j]
			if a.DistSq < b.DistSq || (a.DistSq == b.DistSq && a.ID < b.ID) {
				break
			}
			cands[j-1], cands[j] = cands[j], cands[j-1]
		}
	}
	out := make([]uint64, 0, len(cands))
	monsters, players := 0, 0
	add := func(c Candidate) {
		if c.IsPlayer {
			if players < cap.Players {
				players++
				out = append(out, c.ID)
			}
			return
		}
		if monsters < cap.Monsters {
			monsters++
			out = append(out, c.ID)
		}
	}
	for _, c := range cands {
		if c.ID == primary {
			add(c)
			break
		}
	}
	for _, c := range cands {
		if c.ID == primary {
			continue
		}
		add(c)
	}
	return out
}
