package discovery

// curve.go mirrors the canonical EXP curve (progression.md:
// exp_required(L) = 10000·L², ×100 scale, cumulative_cap 702,100,000).
// sim/progression.LevelFor encodes the same formula; durable/ never
// imports sim/, so the trivial arithmetic is restated here rather than
// cross-imported.

// maxExp is the Level-60 absolute cumulative cap.
const maxExp = 702_100_000

// levelFor resolves the level for an absolute cumulative EXP value:
// the largest k in [1..60] with cumulative_exp_to_reach(k) <= exp.
func levelFor(exp int64) int32 {
	cum := int64(0) // cumulative EXP required to reach lvl
	for lvl := int64(1); lvl < 60; lvl++ {
		next := cum + 10000*lvl*lvl // cumulative required to reach lvl+1
		if exp < next {
			return int32(lvl)
		}
		cum = next
	}
	return 60
}
