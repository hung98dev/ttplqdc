package crafting

import (
	"math/big"
)

// Expected-cost reference machinery (crafting.md § Expected-Cost
// Reference): exact rational arithmetic over the renewal recurrence —
// never rounded intermediate steps; round the final multiplier once to
// two decimals half-up.
//
//	d_L(f) = cost to first reach L+1 from current L with this target's
//	         pity counter at f and all lower-target counters reset by
//	         their latest successful restoration.
//	r_L    = 0 at a milestone floor (L in {4,8,12}) or an insured L
//	         (reference model insures L=8..11); else r_L = d_(L-1)(0).
//	L < 12 : d_L = (cost_L + (1-p_L) * r_L) / p_L, p_L = base_bp[L]/10000
//	L >= 12: p_L(f) = min(base_bp[L] + max(0,f-4)*100, 9500)/10000
//	         d_L(9) = (cost_L + (1-p_L(9)) * r_L) / p_L(9)
//	         d_L(f) = cost_L + (1-p_L(f)) * (r_L + d_L(f+1)), f=8..0
//
// Expected cumulative units to +T = sum_{L=0}^{T-1} d_L(0).

// insuredLevel reports whether the reference model assumes Insurance at
// this current level (L=8..11 — crafting.md cost-model notes).
func insuredLevel(l int) bool { return l >= 8 && l <= 11 }

// expectedUnits returns the exact expected multiplier units to reach
// target level from +0 for one per-level cost function.
func expectedUnits(target int, costOf func(l int) *big.Rat) *big.Rat {
	type key struct{ l, f int }
	memo := map[key]*big.Rat{}
	var d func(l, f int) *big.Rat
	d = func(l, f int) *big.Rat {
		if v, ok := memo[key{l, f}]; ok {
			return v
		}
		p := rateRat(l, f)
		r := big.NewRat(0, 1)
		if !insuredLevel(l) && floorOf(int64(l)) != int64(l) {
			r = d(l-1, 0)
		}
		cost := costOf(l)
		one := big.NewRat(1, 1)
		fail := new(big.Rat).Sub(one, p)
		var out *big.Rat
		if l < 12 {
			// d = (cost + (1-p)*r) / p
			out = new(big.Rat).Quo(
				new(big.Rat).Add(cost, new(big.Rat).Mul(fail, r)), p)
		} else if f >= pityMaxFails {
			// f=9 cap: self-loop => d = (cost + (1-p)*r)/p
			out = new(big.Rat).Quo(
				new(big.Rat).Add(cost, new(big.Rat).Mul(fail, r)), p)
		} else {
			// d(f) = cost + (1-p) * (r + d(f+1))
			out = new(big.Rat).Add(cost,
				new(big.Rat).Mul(fail, new(big.Rat).Add(r, d(l, f+1))))
		}
		memo[key{l, f}] = out
		return out
	}
	total := big.NewRat(0, 1)
	for l := 0; l < target; l++ {
		total.Add(total, d(l, 0))
	}
	return total
}

// rateRat returns p_L(f) as an exact rational: min(base_bp[L] +
// max(0,f-4)*100, 9500)/10000 (pity term only for L>=12 targets).
func rateRat(l, f int) *big.Rat {
	bp := baseRateBP[l]
	if l >= 12 {
		bp += pityBonusBP(int64(f))
	}
	if bp > rateClampBP {
		bp = rateClampBP
	}
	return big.NewRat(bp, 10000)
}

// ExpectedMaterialUnits returns the exact expected material multiplier
// units to reach `target` from +0 (crafting.md validation references).
func ExpectedMaterialUnits(target int) *big.Rat {
	return expectedUnits(target, func(l int) *big.Rat {
		return big.NewRat(materialMultiplier[l], 1)
	})
}

// ExpectedCommonUnits returns the exact expected common-base multiplier
// units to reach `target` from +0.
func ExpectedCommonUnits(target int) *big.Rat {
	return expectedUnits(target, func(l int) *big.Rat {
		return big.NewRat(commonMultiplier[l], 1)
	})
}

// RoundUnits rounds one exact-unit total to two decimals half-up —
// the only rounding step in the reference (never intermediate).
func RoundUnits(r *big.Rat) *big.Rat {
	v := new(big.Rat).Mul(r, big.NewRat(100, 1))
	// Half-up: floor(v + 1/2) / 100.
	half := big.NewRat(1, 2)
	v.Add(v, half)
	floored := new(big.Int).Quo(v.Num(), v.Denom())
	return new(big.Rat).SetFrac(floored, big.NewInt(100))
}
