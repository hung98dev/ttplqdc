package cosmetics

import (
	"errors"

	protocolv1 "thinhthan/internal/protocol/v1"
)

var (
	ErrMalformedRecord      = errors.New("cosmetics: malformed record")
	ErrUnknownCosmetic      = errors.New("cosmetics: unknown cosmetic id")
	ErrNotOwned             = errors.New("cosmetics: entitlement not owned")
	ErrNoRoute              = errors.New("cosmetics: cosmetic exposes no such redemption route")
	ErrInsufficientCurrency = errors.New("cosmetics: insufficient currency balance")
	ErrInsufficientItem     = errors.New("cosmetics: insufficient material quantity")
	ErrInvalidSlot          = errors.New("cosmetics: slot does not accept this cosmetic")
	ErrNotGuildMember       = errors.New("cosmetics: actor not a guild member")
	ErrPermission           = errors.New("cosmetics: role lacks guild cosmetic permission")
	ErrRevisionConflict     = errors.New("cosmetics: expected_revision mismatch")
	ErrGuildCosmeticLocked  = errors.New("cosmetics: guild cosmetic not unlocked")
	ErrWrongCategory        = errors.New("cosmetics: cosmetic category does not fit the slot")
)

// codeOf maps domain errors onto the wire error enum. Codes outside
// the family result's declared set leave client_result absent — the
// edge answers S2C_ERROR instead (durable/inventory precedent).
func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, ErrNotOwned):
		return protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER
	case errors.Is(err, ErrInsufficientCurrency):
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY
	case errors.Is(err, ErrInsufficientItem):
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_ITEM
	case errors.Is(err, ErrUnknownCosmetic), errors.Is(err, ErrNoRoute):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	case errors.Is(err, ErrNotGuildMember), errors.Is(err, ErrGuildCosmeticLocked):
		return protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	case errors.Is(err, ErrPermission):
		return protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED
	case errors.Is(err, ErrRevisionConflict):
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	default:
		return protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	}
}
