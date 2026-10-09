package atlas

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/progression"
)

// lifeSkillActs is the authored LIFE_SKILL per-act grant
// (progression_route.md § Seven-Channel EXP Source Portfolio), the same
// table the cooking settlement applies: act I..VI →
// 6417 / 8283 / 6332 / 7047 / 9448 / 10494.
var lifeSkillActs = [6]int64{6417, 8283, 6332, 7047, 9448, 10494}

// expCap is the progression.md level-60 ceiling on current_exp
// (mirrors durable/cooking and durable/reward — single canonical formula).
const expCap int64 = 702_100_000

// characterAct maps the character's persisted level to its 1..6 act
// (progression_route.md act bands: Lv 1-10 I ... Lv 51-60 VI).
func characterAct(level int32) int {
	act := int((level + 9) / 10)
	if act < 1 {
		act = 1
	}
	if act > 6 {
		act = 6
	}
	return act
}

// LifeSkillExpForAct returns the authored LIFE_SKILL grant for act (1..6).
func LifeSkillExpForAct(act int) int64 {
	if act < 1 || act > 6 {
		return 0
	}
	return lifeSkillActs[act-1]
}

// applyCharacterExp grants LIFE_SKILL EXP under the character lock,
// derives the new level (progression.md § Level Derivation:
// cum(k) = 10000·(k-1)² cumulative), applies per-level +1 skill /
// +4 potential gains and caps at the ceiling — the identical formula
// durable/cooking applies for cook EXP.
func applyCharacterExp(ctx context.Context, tx pgx.Tx, prog *progression.Store,
	charID id.UUID, exp int64) (applied int64, levelAfter int32, err error) {
	p, err := prog.Read(ctx, tx, charID)
	if err != nil {
		return 0, 0, err
	}
	newExp := int64(p.CurrentExp) + exp
	if newExp > expCap {
		newExp = expCap
	}
	newLevel := levelForExp(newExp)
	var skillGain, potGain int32
	if newLevel > p.Level {
		skillGain = newLevel - p.Level
		potGain = 4 * (newLevel - p.Level)
	}
	if err := prog.Write(ctx, tx, charID,
		newLevel, int32(newExp),
		p.UnspentSkillPoints+skillGain,
		p.UnspentPotentialPoints+potGain); err != nil {
		return 0, 0, err
	}
	return int64(newExp) - int64(p.CurrentExp), newLevel, nil
}

// levelForExp mirrors progression.md: L = max{k in [1..60] |
// cumulative_exp_to_reach(k) <= exp}, cumulative = sum(10000·i², i<k).
func levelForExp(exp int64) int32 {
	level := int32(1)
	cum := int64(0)
	for k := int32(2); k <= 60; k++ {
		cum += 10000 * int64(k-1) * int64(k-1)
		if exp < cum {
			break
		}
		level = k
	}
	return level
}
