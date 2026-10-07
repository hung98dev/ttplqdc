package inventory

import (
	"context"

	"thinhthan/internal/durable/items"
)

// UseKind classifies the consume effect of an item_id.
type UseKind int

const (
	// UseNone — not usable (non-consumable kinds).
	UseNone UseKind = iota
	// UseConsumable — shared-cooldown-group consumable (HP/MP/BUFF/FOOD);
	// consume + cooldown only, the stat effect is sim-side.
	UseConsumable
	// UseBookPotential — item.book.potential: +10 unspent potential.
	UseBookPotential
	// UseBookSkill — item.book.skill: +1 unspent skill point.
	UseBookSkill
)

// ItemDef is the runtime view of one item_id the executors need. It
// embeds the items.Def custody fields plus the inventory-domain fields
// (rarity for SORT, use effect + cooldown for USE) that the caller's
// catalog binding supplies. Composition binds Defs from the activated
// content snapshot; a nil Defs resolver is fail-closed (ITEM_NOT_FOUND).
type ItemDef struct {
	Def       items.Def
	Rarity    int32
	Use       UseKind
	CooldownS float64 // item-specific cooldown seconds (0 = group default)
}

// Defs resolves an item_id to its runtime definition.
type Defs func(ctx context.Context, itemID string) (ItemDef, error)

// cooldownFor returns the effective cooldown seconds for def: the
// shared-group duration from items.CooldownSeconds unless a longer
// item-specific cooldown overrides (items.md: longer wins).
func cooldownFor(def ItemDef) float64 {
	base := float64(items.CooldownSeconds(def.Def.SharedCooldownGroup))
	if def.CooldownS > base {
		return def.CooldownS
	}
	return base
}
