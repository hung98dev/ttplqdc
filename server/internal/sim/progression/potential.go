package progression

// Potential allocation rules of stats.md § Potential Stats and
// progression.md § Potential Points: four stats (STR, VIT, INT, AGI), a
// per-stat cap of floor(0.6·earned_total), and all-or-nothing allocation.

// PotentialID names the four potential stats; its String is the
// persistence key (`character_potential_allocations.potential_id`).
type PotentialID uint8

const (
	PotentialStr PotentialID = iota
	PotentialVit
	PotentialInt
	PotentialAgi
)

var potentialNames = [4]string{"STR", "VIT", "INT", "AGI"}

// PotentialIDs is the canonical stat order.
var PotentialIDs = [4]PotentialID{PotentialStr, PotentialVit, PotentialInt, PotentialAgi}

func (p PotentialID) String() string { return potentialNames[p] }

// PotentialDelta is a per-stat point set — the C2S deltas shape and the
// stored allocation shape.
type PotentialDelta struct {
	Str int32
	Vit int32
	Int int32
	Agi int32
}

// Sum returns the total points in the set.
func (d PotentialDelta) Sum() int32 { return d.Str + d.Vit + d.Int + d.Agi }

// Plus returns componentwise a+b.
func (d PotentialDelta) Plus(o PotentialDelta) PotentialDelta {
	return PotentialDelta{d.Str + o.Str, d.Vit + o.Vit, d.Int + o.Int, d.Agi + o.Agi}
}

// At returns the value for one stat id.
func (d PotentialDelta) At(id PotentialID) int32 {
	switch id {
	case PotentialStr:
		return d.Str
	case PotentialVit:
		return d.Vit
	case PotentialInt:
		return d.Int
	default:
		return d.Agi
	}
}

// PerStatCap is the per-stat allocation cap floor(0.6·earned) — 213 at the
// level-60 earned total of 356.
func PerStatCap(earned int32) int32 {
	if earned < 0 {
		return 0
	}
	return earned * 6 / 10
}

// EarnedTotal derives the lifetime earned potential total: unspent plus
// every allocated point (`potential_earned_total` is derivable — no
// separate column).
func EarnedTotal(unspent int32, alloc PotentialDelta) int32 {
	return unspent + alloc.Sum()
}
