package equipment

// Enhancement tables — canonical in crafting.md § Enhancement (the
// equipment spec defers to it). Success is exactly +1 per attempt;
// failure drops one level clamped at the milestone floor, or preserves
// the level with insurance.

// MaxEnhancement is the terminal enhancement level (+0..+16).
const MaxEnhancement = 16

// successBP[current level] is the base success rate of the attempt
// current -> current+1, in basis points (crafting.md § Base Success
// Rates).
var successBP = [MaxEnhancement]int{
	10000, 10000, 8500, 7000, 5500, 4500, 3500, 2500,
	2000, 1500, 1000, 800, 600, 400, 300, 200,
}

// SuccessRateBP returns the base success rate (bp) for the attempt
// current -> current+1. current is clamped to the legal attempt domain
// [0,15]; a level at MaxEnhancement has no attempt and reports 0.
func SuccessRateBP(current int) int {
	if current < 0 || current >= MaxEnhancement {
		return 0
	}
	return successBP[current]
}

// Floor returns the milestone floor for a level (crafting.md § Milestone
// Floors): 0..3 -> 0, 4..7 -> 4, 8..11 -> 8, 12..15 -> 12, 16 -> 16.
func Floor(level int) int {
	switch {
	case level >= MaxEnhancement:
		return MaxEnhancement
	case level >= 12:
		return 12
	case level >= 8:
		return 8
	case level >= 4:
		return 4
	default:
		return 0
	}
}

// FailLevel applies the failure rule: without insurance the item drops
// one level clamped at its milestone floor; with insurance the level is
// strictly preserved.
func FailLevel(current int, insured bool) int {
	if insured {
		return current
	}
	if d := current - 1; d > Floor(current) {
		return d
	}
	return Floor(current)
}

// FinalRateBP applies charm and guild-blessing bonuses then clamps at
// 9500 bp: min(base + blessing + charm, 9500).
func FinalRateBP(current, blessingBP, charmBP int) int {
	r := SuccessRateBP(current) + blessingBP + charmBP
	if r > 9500 {
		return 9500
	}
	return r
}

// Lucky charm catalog rows (crafting.md § Lucky Charm): bonus bp and the
// current-level eligibility ceiling (charm usable only below it).
type charmDef struct {
	bonusBP   int
	levelLtEl int // eligible while current < levelLtEl; 0 = all levels
}

var luckyCharms = map[string]charmDef{
	"item.consumable.bua_may.so_cap":    {bonusBP: 500, levelLtEl: 8},
	"item.consumable.bua_may.trung_cap": {bonusBP: 300, levelLtEl: 12},
	"item.consumable.bua_may.cao_cap":   {bonusBP: 100, levelLtEl: 0},
	"item.consumable.bua_may.sieu_cap":  {bonusBP: 300, levelLtEl: 0},
}

// CharmBonusBP returns the charm's bonus bp and whether it is eligible
// for an attempt at current level. Unknown charm ids report ok=false.
func CharmBonusBP(charmItemID string, current int) (bp int, eligible bool) {
	c, ok := luckyCharms[charmItemID]
	if !ok {
		return 0, false
	}
	if c.levelLtEl != 0 && current >= c.levelLtEl {
		return 0, false
	}
	return c.bonusBP, true
}

// insurance tiers (crafting.md § Insurance): eligibility only — the
// effect is FailLevel's insured branch.
var insuranceCharms = map[string]int{ // current-level ceiling, 0 = all
	"item.consumable.bua_giu_bac.so_cap":    8,
	"item.consumable.bua_giu_bac.trung_cap": 12,
	"item.consumable.bua_giu_bac.cao_cap":   0,
}

// InsuranceEligible reports whether the insurance charm may be used at
// the current level; unknown ids are ineligible.
func InsuranceEligible(insItemID string, current int) bool {
	lt, ok := insuranceCharms[insItemID]
	if !ok {
		return false
	}
	return lt == 0 || current < lt
}
