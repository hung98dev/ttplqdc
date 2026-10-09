package crafting

import (
	"errors"
)

// Sentinel failures the executors translate to wire error codes
// (messages.md § 404/406 error sets).
var (
	// ErrInsufficientItems: material/input stacks cannot cover the
	// frozen selection -> ERROR_CODE_INSUFFICIENT_ITEM.
	ErrInsufficientItems = errors.New("crafting: insufficient items")
	// ErrInsufficientCurrency: common balance cannot cover the
	// signed cost -> ERROR_CODE_INSUFFICIENT_CURRENCY.
	ErrInsufficientCurrency = errors.New("crafting: insufficient currency")
	// ErrInventoryFull: capacity pre-check/revalidation cannot place
	// every frozen output -> ERROR_CODE_INVENTORY_FULL.
	ErrInventoryFull = errors.New("crafting: inventory full")
	// ErrLevelTooLow: character level below the recipe minimum ->
	// ERROR_CODE_LEVEL_TOO_LOW.
	ErrLevelTooLow = errors.New("crafting: level too low")
	// ErrNotFound: recipe, item instance, or character missing.
	ErrNotFound = errors.New("crafting: not found")
	// ErrItemLocked: instance under a live transaction lock ->
	// ERROR_CODE_ITEM_LOCKED.
	ErrItemLocked = errors.New("crafting: item locked")
	// ErrCharmIneligible: stacked/ineligible charm selection ->
	// ERROR_CODE_CHARM_INELIGIBLE (consumed nothing).
	ErrCharmIneligible = errors.New("crafting: charm ineligible")
	// ErrStateConflict: target_level != current+1, item not
	// enhanceable, or out-of-window state -> ERROR_CODE_STATE_CONFLICT.
	ErrStateConflict = errors.New("crafting: state conflict")
)
