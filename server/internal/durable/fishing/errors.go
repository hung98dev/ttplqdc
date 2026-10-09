package fishing

import "errors"

// Sentinel domain errors mapped to wire codes at the executor boundary.
var (
	// ErrNoRod — the caster owns no `item.tool.can_cau_tre`.
	ErrNoRod = errors.New("fishing: bamboo fishing rod required")
	// ErrNoBait — no `item.consumable.moi_cau` stack can cover the cast.
	ErrNoBait = errors.New("fishing: earthworm bait required")
	// ErrDailyCap — the character's 50-success UTC-day cap is reached.
	ErrDailyCap = errors.New("fishing: daily catch cap reached")
	// ErrCastMissing — no accepted cast precedes the hook today.
	ErrCastMissing = errors.New("fishing: no cast to resolve")
	// ErrInsufficientItems — a frozen/committed consume came up short.
	ErrInsufficientItems = errors.New("fishing: insufficient items")
	// ErrInventoryFull — no stack or empty slot can take the catch.
	ErrInventoryFull = errors.New("fishing: inventory capacity full")
	// ErrNotFound — a committed consume instance is gone at commit.
	ErrNotFound = errors.New("fishing: item instance missing")
)
