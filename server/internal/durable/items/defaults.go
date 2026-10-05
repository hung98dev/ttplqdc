package items

import "strings"

// CooldownGroup is the items.md § Consumables shared_cooldown_group enum.
type CooldownGroup string

const (
	CooldownNone CooldownGroup = "NONE"
	CooldownHP   CooldownGroup = "HP"
	CooldownMP   CooldownGroup = "MP"
	CooldownBuff CooldownGroup = "BUFF"
	CooldownFood CooldownGroup = "FOOD"
)

// CooldownSeconds returns the shared-group cooldown in seconds
// (items.md: HP 8s, MP 8s, BUFF 5s, FOOD 1s; NONE has none).
func CooldownSeconds(g CooldownGroup) int {
	switch g {
	case CooldownHP, CooldownMP:
		return 8
	case CooldownBuff:
		return 5
	case CooldownFood:
		return 1
	default:
		return 0
	}
}

// ApplyDefinitionDefaults resolves items.md § Definition Defaults onto a
// catalog row: a row overrides a default only by stating the field
// explicitly. discard_allowed defaults false for two independent classes —
// QUEST items and CHARACTER_BOUND progression items (item.book.*) — and
// true otherwise. shared_cooldown_group defaults NONE. Unstackable items
// default max_stack 1; stackable items may declare up to the 9999 ceiling.
func ApplyDefinitionDefaults(d Def) Def {
	if d.MaxStack <= 0 {
		if d.Stackable {
			d.MaxStack = MaxStackCeiling
		} else {
			d.MaxStack = 1
		}
	}
	if d.DiscardAllowed == nil {
		quest := d.Kind&KindQuest != 0
		boundBook := d.DefaultBinding == BindingCharacterBound &&
			strings.HasPrefix(d.ItemID, "item.book.")
		discard := !(quest || boundBook)
		d.DiscardAllowed = &discard
	}
	if d.SharedCooldownGroup == "" {
		d.SharedCooldownGroup = CooldownNone
	}
	return d
}
