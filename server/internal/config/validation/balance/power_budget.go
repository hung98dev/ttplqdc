package balance

import (
	"sort"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// Roll-magnitude budget rule (balance_validation.md § New-Stat
// Power-Budget Validation). Flat stats use power(x)=10000*x/ref with the
// pinned Lv60 reference denominators; utilities use power(x)=10000*x/cap.
var (
	flatRefDenom = map[string]int64{
		"ATTACK": 724, "DEFENSE": 426, "MAX_HP": 4794, "MAX_MP": 859,
	}
	utilCapDenom = map[string]config.Rat{
		"CRIT_CHANCE":        {Num: 60, Den: 100},
		"ATTACK_SPEED":       {Num: 50, Den: 100},
		"CAST_SPEED":         {Num: 50, Den: 100},
		"COOLDOWN_REDUCTION": {Num: 35, Den: 100},
		"LIFESTEAL":          {Num: 8, Den: 100},
		"REFLECT":            {Num: 15, Den: 100},
		"ABSORB":             {Num: 10, Den: 100},
		"HEAL_REDUCTION":     {Num: 30, Den: 100},
	}
)

// oldPoolStats is the pre-expansion 8-type pool (baseline); the current
// 12-type pool adds the four new stats.
var (
	oldPoolStats = []string{"ATTACK", "DEFENSE", "MAX_HP", "MAX_MP",
		"CRIT_CHANCE", "ATTACK_SPEED", "CAST_SPEED", "COOLDOWN_REDUCTION"}
	newPoolStats = []string{"LIFESTEAL", "REFLECT", "ABSORB", "HEAL_REDUCTION"}
)

// powerOf converts one roll magnitude to common power units.
func powerOf(roll *equipment.RollDef, m config.Rat) (config.Rat, bool) {
	if roll == nil || m.Den == 0 {
		return config.Rat{}, false
	}
	if d, ok := flatRefDenom[roll.Stat]; ok {
		return reduceRat(m.Num*10000, m.Den*d), true
	}
	if capv, ok := utilCapDenom[roll.Stat]; ok {
		return reduceRat(m.Num*10000*capv.Den, m.Den*capv.Num), true
	}
	return config.Rat{}, false
}

func reduceRat(n, d int64) config.Rat {
	if d == 0 {
		return config.Rat{}
	}
	if d < 0 {
		n, d = -n, -d
	}
	g := gcd(abs64(n), d)
	return config.Rat{Num: n / g, Den: d / g}
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func ratAdd(a, b config.Rat) config.Rat {
	return reduceRat(a.Num*b.Den+b.Num*a.Den, a.Den*b.Den)
}

// ratMid is the exact midpoint of an inclusive uniform [lo,hi] draw.
func ratMid(lo, hi config.Rat) config.Rat {
	return reduceRat(lo.Num*hi.Den+hi.Num*lo.Den, 2*lo.Den*hi.Den)
}

func cmpRat(a, b config.Rat) int {
	l := a.Num * b.Den
	r := b.Num * a.Den
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	}
	return 0
}

// poolExpectedPower computes E_item = K/N * sum(E_power(type)) over the
// pool restricted to the given stats at one tier.
func poolExpectedPower(cat *equipment.Catalog, tier string, stats []string) (config.Rat, bool) {
	sum := config.Rat{Num: 0, Den: 1}
	for _, s := range stats {
		var found *equipment.RollDef
		for _, r := range cat.Rolls {
			if r != nil && r.Stat == s {
				found = r
				break
			}
		}
		if found == nil {
			return config.Rat{}, false
		}
		lo, hi, ok := rollMagnitudeRange(cat, found, tier)
		if !ok {
			return config.Rat{}, false
		}
		p, ok := powerOf(found, ratMid(lo, hi))
		if !ok {
			return config.Rat{}, false
		}
		sum = ratAdd(sum, p)
	}
	var k int64 = 1
	if b := cat.Budgets[tier]; b != nil && b.SecondaryRolls > 0 {
		k = b.SecondaryRolls
	}
	return reduceRat(sum.Num*k, sum.Den*int64(len(stats))), true
}

// rollMagnitudeRange resolves one roll's legal [lo,hi] magnitude at a
// tier: FLAT rolls ride flat_roll_ranges coefficients scaled by the
// tier's authoring unit (floored); UTILITY rolls carry authored
// tier_ranges.
func rollMagnitudeRange(cat *equipment.Catalog, roll *equipment.RollDef, tier string) (lo, hi config.Rat, ok bool) {
	if rng, ok2 := roll.TierRanges[tier]; ok2 {
		return rng[0], rng[1], true
	}
	var fr *equipment.FlatRollRange
	for _, f := range cat.FlatRanges {
		if f.Stat == roll.Stat {
			fr = f
			break
		}
	}
	b := cat.Budgets[tier]
	if fr == nil || b == nil {
		return config.Rat{}, config.Rat{}, false
	}
	var unit int64
	switch fr.Unit {
	case "A":
		unit = b.UnitA
	case "D":
		unit = b.UnitD
	case "H":
		unit = b.UnitH
	case "M":
		unit = b.UnitM
	}
	if unit <= 0 || fr.Lo.Den == 0 || fr.Hi.Den == 0 {
		return config.Rat{}, config.Rat{}, false
	}
	return reduceRat(floorRat(mulRat(fr.Lo, unit)), 1),
		reduceRat(floorRat(mulRat(fr.Hi, unit)), 1), true
}

func floorRat(r config.Rat) int64 {
	if r.Den == 0 {
		return 0
	}
	v := r.Num / r.Den
	if r.Num < 0 && r.Num%r.Den != 0 {
		v--
	}
	return v
}

func mulRat(r config.Rat, k int64) config.Rat {
	return reduceRat(r.Num*k, r.Den)
}

// rollMagnitudes enumerates the legal inclusive magnitudes of one roll at
// one tier: integer steps for flats (unit denominators), the authored
// denominator grid for utility fractions.
func rollMagnitudes(cat *equipment.Catalog, roll *equipment.RollDef, tier string) []config.Rat {
	lo, hi, ok := rollMagnitudeRange(cat, roll, tier)
	if !ok {
		return nil
	}
	rng := [2]config.Rat{lo, hi}
	den := rng[0].Den / gcd(rng[0].Den, rng[1].Den) * rng[1].Den
	loN, hiN := rng[0].Num*den/rng[0].Den, rng[1].Num*den/rng[1].Den
	if hiN < loN || hiN-loN > 100000 {
		return nil
	}
	out := make([]config.Rat, 0, hiN-loN+1)
	for v := loN; v <= hiN; v++ {
		out = append(out, reduceRat(v, den))
	}
	return out
}

// expectedMagnitude reports the expected magnitude (rational midpoint) of
// one roll stat at a tier — the Validation Output field.
func expectedMagnitude(cat *equipment.Catalog, stat, tier string) (config.Rat, bool) {
	for _, r := range cat.Rolls {
		if r != nil && r.Stat == stat {
			if lo, hi, ok := rollMagnitudeRange(cat, r, tier); ok {
				return ratMid(lo, hi), true
			}
		}
	}
	return config.Rat{}, false
}

// powerDeltaPct is the secondary-roll pool expected power delta vs the
// pre-change baseline, in percent (negative = decrease).
func powerDeltaPct(cat *equipment.Catalog, tier string) (float64, bool) {
	cur, okC := poolExpectedPower(cat, tier, append(append([]string{}, oldPoolStats...), newPoolStats...))
	old, okO := poolExpectedPower(cat, tier, oldPoolStats)
	if !okC || !okO || old.Num == 0 {
		return 0, false
	}
	return 100 * (ratFloat(cur)/ratFloat(old) - 1), true
}

// checkPowerBudget enforces the roll-magnitude budget rule: the
// current-12 pool's expected per-item power must not exceed the old-8
// baseline by more than 1% absent an approved balance note.
func checkPowerBudget(c *config.CandidateSnapshot, cat *equipment.Catalog, d *config.Diagnostics) {
	if len(cat.Rolls) != 12 {
		d.Addf(config.DiagBalanceGuardrail, "equipment", 0,
			"balance.roll_magnitude: secondary pool has %d types, want 12", len(cat.Rolls))
		return
	}
	for _, ep := range endpoints {
		cur, okC := poolExpectedPower(cat, ep.Tier, append(append([]string{}, oldPoolStats...), newPoolStats...))
		old, okO := poolExpectedPower(cat, ep.Tier, oldPoolStats)
		if !okC || !okO {
			d.Addf(config.DiagIntegrationCheck, "equipment", 0,
				"balance.roll_magnitude: tier %s missing roll ranges for budget", ep.Tier)
			continue
		}
		if ratFloat(cur) > ratFloat(old)*1.01 {
			d.Addf(config.DiagBalanceGuardrail, "equipment", 0,
				"balance.roll_magnitude: tier %s expected power %s exceeds old8 baseline %s by >1%%",
				ep.Tier, cur.String(), old.String())
		}
	}
	checkWindowPreservation(c, cat, d)
}

// checkWindowPreservation runs fixture B of the TTK and Survivability
// Window Preservation Rule: every legal magnitude of every roll type
// added to each reference slot separately (one marginal slot per draw),
// plus fixed max-everywhere stress loadouts (each type at maximum on
// every slot eligible for it). Windows never widen.
func checkWindowPreservation(c *config.CandidateSnapshot, cat *equipment.Catalog, d *config.Diagnostics) {
	ids := make([]string, 0, len(cat.Rolls))
	for id := range cat.Rolls {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, ep := range endpoints {
		for _, cl := range classIDs {
			for _, rid := range ids {
				roll := cat.Rolls[rid]
				if roll == nil {
					continue
				}
				for _, m := range rollMagnitudes(cat, roll, ep.Tier) {
					windowCheck(c, cat, cl, ep.Level,
						map[string]float64{roll.Stat: ratFloat(m)},
						"B/"+rid, d)
				}
				// max-everywhere: hi magnitude on every eligible slot
				if _, hi, ok := rollMagnitudeRange(cat, roll, ep.Tier); ok {
					n := eligibleSlots(cat, ep.Tier, roll.Stat)
					if n == 0 {
						d.Addf(config.DiagIntegrationCheck, "equipment", 0,
							"balance.roll_magnitude: %s has no eligible slot at %s", rid, ep.Tier)
						continue
					}
					windowCheck(c, cat, cl, ep.Level,
						map[string]float64{roll.Stat: float64(n) * ratFloat(hi)},
						"B-max/"+rid, d)
				}
			}
		}
	}
}
