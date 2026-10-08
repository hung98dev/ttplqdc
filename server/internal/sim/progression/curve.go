// Package progression owns the pure character-progression domain rules of
// progression.md and stats.md: the EXP curve and level budget, skill/potential
// point grants, allocation and respec semantics, and the final-stat pipeline.
// No SQL, pgx or edge imports — every value is an in-memory domain value.
package progression

// EXP scale: current_exp and required EXP are stored and sent in the ×100
// display scale of progression.md / ADR-0031 (spec values are the scaled
// integers used below).
const (
	// StartLevel is the creation level.
	StartLevel int32 = 1
	// MaxLevel is the level cap; EXP stops accumulating there.
	MaxLevel int32 = 60
	// ExpCap is the absolute cumulative EXP at level 60
	// (CumulativeExp(MaxLevel) = 702_100_000; fits int32).
	ExpCap int64 = 702_100_000
)

// ExpRequired returns the EXP needed to advance from level to level+1
// (`exp_required(L) = 10000·L²`, scaled). At the cap the requirement is 0 —
// level 60 is the stop.
func ExpRequired(level int32) int64 {
	if level < StartLevel || level >= MaxLevel {
		return 0
	}
	l := int64(level)
	return 10000 * l * l
}

// CumulativeExp returns the absolute cumulative EXP required to reach level:
// cum(1) = 0, cum(k) = Σ 10000·i² for i = 1..k-1. Input is clamped to
// [StartLevel, MaxLevel].
func CumulativeExp(level int32) int64 {
	if level <= StartLevel {
		return 0
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	l := int64(level)
	// Σ i² for i = 1..l-1 = (l-1)·l·(2l-1)/6.
	return 10000 * (l - 1) * l * (2*l - 1) / 6
}

// LevelFor resolves the level for an absolute cumulative EXP value: the
// highest k in [1..60] with CumulativeExp(k) ≤ exp. Negative input clamps
// to StartLevel; values above the cap resolve to MaxLevel.
func LevelFor(exp int64) int32 {
	if exp <= 0 {
		return StartLevel
	}
	if exp >= ExpCap {
		return MaxLevel
	}
	lo, hi := StartLevel, MaxLevel
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if CumulativeExp(mid) <= exp {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// LevelUp is the grant produced by advancing into one level: +1 skill point
// and +4 potential points per level (progression.md § Level Rewards), plus
// the +2 bonus skill points at levels 55 and 60. Unlocks carries the
// catalog skills whose unlock level was just reached, in catalog document
// order (learned automatically at the milestone).
type LevelUp struct {
	Level           int32
	SkillPoints     int32
	PotentialPoints int32
	Unlocks         []string
}

// ApplyExp adds delta EXP to cur and returns the resulting value plus the
// per-level grants for every level gained (cur.Level+1 .. new level). EXP
// clamps at ExpCap; a character already at MaxLevel gains nothing (EXP does
// not accumulate past the cap). Newly unlocked catalog skills are appended
// to Unlocks; the caller persists them as learned rows at level 1.
func ApplyExp(cur PlayerProgress, delta int64) (PlayerProgress, []LevelUp) {
	out := cur
	exp := int64(out.Exp) + delta
	if exp < 0 {
		exp = 0
	}
	if exp > ExpCap {
		exp = ExpCap
	}
	out.Exp = int32(exp)
	next := LevelFor(exp)
	if next <= out.Level {
		out.Level = max(out.Level, next)
		return out, nil
	}
	var ups []LevelUp
	for lvl := out.Level + 1; lvl <= next; lvl++ {
		up := LevelUp{
			Level:           lvl,
			SkillPoints:     1 + BonusSkillPoints(lvl),
			PotentialPoints: PotentialPerLevel,
			Unlocks:         skillsUnlockedAt(out.ClassID, lvl),
		}
		out.UnspentSkillPoints += up.SkillPoints
		out.UnspentPotentialPoints += up.PotentialPoints
		if len(up.Unlocks) > 0 {
			out.Skills = cloneSkills(out.Skills)
			for _, id := range up.Unlocks {
				out.Skills[id] = 1
			}
		}
		ups = append(ups, up)
	}
	out.Level = next
	return out, ups
}
