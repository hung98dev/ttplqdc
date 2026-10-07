package inventory

import (
	"errors"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Sentinels. Verdict mapping is per family: the declared in-set error
// lists for 401 (mutate) and 429 (expand) differ (messages.md).
var (
	// ErrItemNotFound covers missing instances, foreign ownership and
	// instances not in the owner's CHARACTER_INVENTORY — never an
	// existence oracle (ITEM_NOT_FOUND).
	ErrItemNotFound = errors.New("inventory: item not found")
	// ErrItemLocked is a trade/session lock violation (ITEM_LOCKED).
	ErrItemLocked = errors.New("inventory: item locked")
	// ErrFull is used-slot exhaustion or a merge/split that cannot fit
	// (INVENTORY_FULL).
	ErrFull = errors.New("inventory: full")
	// ErrCooldown is the shared-cooldown-group violation on USE
	// (COOLDOWN_ACTIVE).
	ErrCooldown = errors.New("inventory: cooldown active")
	// ErrInvalidState is a request/shape invariant failure
	// (INVALID_STATE).
	ErrInvalidState = errors.New("inventory: invalid state")
	// ErrConflict is an optimistic state mismatch such as
	// expected_capacity (STATE_CONFLICT).
	ErrConflict = errors.New("inventory: state conflict")
	// ErrInsufficientCurrency maps wallet debit failure
	// (INSUFFICIENT_CURRENCY).
	ErrInsufficientCurrency = errors.New("inventory: insufficient currency")
	// ErrCapacityFull is the 120-slot ceiling on expand (CAPACITY_FULL).
	ErrCapacityFull = errors.New("inventory: capacity full")
)

// codeOf maps a sentinel to its wire error code.
func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, ErrItemNotFound):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	case errors.Is(err, ErrItemLocked):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED
	case errors.Is(err, ErrFull):
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL
	case errors.Is(err, ErrCooldown):
		return protocolv1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE
	case errors.Is(err, ErrInsufficientCurrency):
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY
	case errors.Is(err, ErrCapacityFull):
		return protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL
	case errors.Is(err, ErrConflict):
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	default:
		return protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE
	}
}

// mutateInSet is the declared 401 in-set error list (messages.md §400):
// verdicts in it carry S2C_INVENTORY_RESULT as client_result; anything
// else degrades to the out-of-set path (edge emits S2C_ERROR).
func mutateInSet(c protocolv1.ErrorCode) bool {
	switch c {
	case protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND,
		protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED,
		protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL,
		protocolv1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE,
		protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE,
		protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT:
		return true
	}
	return false
}

// expandInSet is the declared 429 in-set error list.
func expandInSet(c protocolv1.ErrorCode) bool {
	switch c {
	case protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY,
		protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL,
		protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE,
		protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT:
		return true
	}
	return false
}
