package fishing

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/progression"
)

// expCap is the progression.md level-60 ceiling on current_exp
// (mirrors durable/reward + durable/cooking's constant — single
// canonical formula per package precedent).
const expCap = 702100000

// applyCharacterExp grants the authored LIFE_SKILL EXP under the
// character lock, derives the new level (progression.md § Level
// Derivation: cum(k) = 10000·(k-1)²), applies per-level +1 skill /
// +4 potential gains and caps at the ceiling — the identical formula
// durable/cooking applies for cook EXP.
func applyCharacterExp(ctx context.Context, tx pgx.Tx, prog *progression.Store,
	charID id.UUID, exp uint64) error {
	if exp == 0 {
		return nil
	}
	p, err := prog.Read(ctx, tx, charID)
	if err != nil {
		return err
	}
	newExp := int64(p.CurrentExp) + int64(exp)
	if newExp > expCap {
		newExp = expCap
	}
	newLevel := levelForExp(newExp)
	var skillGain, potGain int32
	if newLevel > p.Level {
		skillGain = newLevel - p.Level
		potGain = 4 * (newLevel - p.Level)
	}
	return prog.Write(ctx, tx, charID,
		newLevel, int32(newExp),
		p.UnspentSkillPoints+skillGain,
		p.UnspentPotentialPoints+potGain)
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

// characterAct maps the character's persisted level to its 1..6 act
// band (progression_route.md: I=1..10 ... VI=51..60).
func characterAct(ctx context.Context, tx pgx.Tx, charID id.UUID) (int, error) {
	var level int32
	if err := tx.QueryRow(ctx,
		`SELECT level FROM characters WHERE character_id = $1`,
		charID.String()).Scan(&level); err != nil {
		return 0, err
	}
	act := int((level + 9) / 10)
	if act < 1 {
		act = 1
	}
	if act > 6 {
		act = 6
	}
	return act, nil
}
