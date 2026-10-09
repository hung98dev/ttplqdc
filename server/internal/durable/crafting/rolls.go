package crafting

import (
	"math/big"
	"sort"

	"thinhthan/internal/config/equipment"
	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// secondaryRolls picks `def.SecondaryRolls` closed rolls without
// replacement on the instance-keyed stream and rolls each value inside
// its authored tier range (equipment_catalog.md § Secondary Roll Pool:
// flat = uniform integer over floored bounds; utility = FLAT_ADD
// fraction stored at bp scale).
func secondaryRolls(cat *equipment.Catalog, def *equipment.ItemDef,
	instID id.UUID) []*journalv1.JournalStat {
	if cat == nil || def.SecondaryRolls <= 0 {
		return nil
	}
	pool := make([]*equipment.RollDef, 0, len(cat.Rolls))
	for _, r := range cat.Rolls {
		if _, ok := r.TierRanges[def.Tier]; ok {
			pool = append(pool, r)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	rng := streamFor(craftRollKey(id.UUID(instID).String()))
	n := int(def.SecondaryRolls)
	if n > len(pool) {
		n = len(pool)
	}
	// Deterministic partial shuffle; take the first n rolls.
	for i := 0; i < n; i++ {
		j := i + int(rng.Uint64()%uint64(len(pool)-i))
		pool[i], pool[j] = pool[j], pool[i]
	}
	out := make([]*journalv1.JournalStat, 0, n)
	for _, r := range pool[:n] {
		out = append(out, &journalv1.JournalStat{
			StatId: r.Stat,
			Value:  rollValue(cat, def.Tier, r, rng),
			Scale:  rollScale(r),
		})
	}
	return out
}

// rollScale encodes a roll value: flat rolls are scale-1 integers;
// utility (fraction) rolls serialize at bp scale 10000.
func rollScale(r *equipment.RollDef) uint32 {
	if r.Kind == "FLAT" {
		return 1
	}
	return 10000
}

// rollValue draws one value inside the roll's authored range for the
// item tier on the stream.
func rollValue(cat *equipment.Catalog, tier string, r *equipment.RollDef, rng randSource) int64 {
	rg, ok := r.TierRanges[tier]
	if !ok {
		return 0
	}
	lo, hi := rg[0], rg[1]
	if r.Kind == "FLAT" {
		unit := flatUnit(cat, r.Stat, tier)
		loI, hiI := ratAt(lo, unit), ratAt(hi, unit)
		if hiI <= loI {
			return loI
		}
		return loI + int64(rng.Uint64()%uint64(hiI-loI+1))
	}
	// Utility roll: uniform fraction in [lo, hi] at bp scale.
	t := new(big.Rat).SetFrac64(int64(rng.Uint64()%(1<<32)), 1<<32)
	loR := new(big.Rat).SetFrac64(lo.Num, lo.Den)
	hiR := new(big.Rat).SetFrac64(hi.Num, hi.Den)
	v := new(big.Rat).Add(loR, new(big.Rat).Mul(t, new(big.Rat).Sub(hiR, loR)))
	v.Mul(v, big.NewRat(10000, 1))
	return new(big.Int).Quo(v.Num(), v.Denom()).Int64()
}

type randSource interface{ Uint64() uint64 }

// flatUnit resolves a flat roll's tier-unit magnitude via the compiled
// flat_roll_ranges unit letter (A/D/H/M) on the tier budget.
func flatUnit(cat *equipment.Catalog, stat, tier string) int64 {
	letter := ""
	for _, fr := range cat.FlatRanges {
		if fr.Stat == stat {
			letter = fr.Unit
			break
		}
	}
	b := cat.Budgets[tier]
	if b == nil {
		return 1
	}
	switch letter {
	case "A":
		return b.UnitA
	case "D":
		return b.UnitD
	case "H":
		return b.UnitH
	case "M":
		return b.UnitM
	}
	return 1
}

// itemQty packs one wire ItemQuantity.
func itemQty(itemID string, qty int64) *protocolv1.ItemQuantity {
	return &protocolv1.ItemQuantity{ItemId: itemID, Quantity: uint32(qty)}
}
