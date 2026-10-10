package guild_storage

import (
	"errors"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Sentinel errors; wire codes map only through codeOf — executors
// convert domain errors into a 649 ERROR outcome instead of surfacing
// them, unless the failure is infrastructural.
var (
	// ErrMalformedRecord marks a journal record that fails the closed
	// identity/shape contract (missing oneof, short UUID, wrong family).
	ErrMalformedRecord = errors.New("guild_storage: malformed record")

	errPermissionDenied   = errors.New("guild_storage: permission denied")
	errNotMember          = errors.New("guild_storage: not a member")
	errGuildNotFound      = errors.New("guild_storage: guild not found")
	errItemNotFound       = errors.New("guild_storage: item not in storage")
	errCapacityFull       = errors.New("guild_storage: section capacity full")
	errStateConflict      = errors.New("guild_storage: state conflict")
	errSameAccount        = errors.New("guild_storage: same-account transfer prohibited")
	errMembershipTooNew   = errors.New("guild_storage: membership under 72h")
	errInventoryFull      = errors.New("guild_storage: inventory full")
	errQuotaExceeded      = errors.New("guild_storage: withdraw quota exceeded")
	errClaimNotFound      = errors.New("guild_storage: claim not found")
	errInvalidSection     = errors.New("guild_storage: invalid section")
	errSectionLocked      = errors.New("guild_storage: section locked")
	errInvalidQuantity    = errors.New("guild_storage: invalid quantity")
	errClaimNotDecidable  = errors.New("guild_storage: claim not in a decidable state")
	errRequesterNotMember = errors.New("guild_storage: claim requester no longer a member")
)

// codeOf maps domain errors onto wire codes (errors.md domain list).
func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, errSameAccount):
		return protocolv1.ErrorCode_ERROR_CODE_GUILD_STORAGE_SAME_ACCOUNT
	case errors.Is(err, errMembershipTooNew):
		return protocolv1.ErrorCode_ERROR_CODE_GUILD_MEMBERSHIP_TOO_NEW
	case errors.Is(err, errInventoryFull):
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL
	case errors.Is(err, errPermissionDenied), errors.Is(err, errNotMember),
		errors.Is(err, errSectionLocked), errors.Is(err, errRequesterNotMember):
		return protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED
	case errors.Is(err, errCapacityFull):
		return protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL
	case errors.Is(err, errItemNotFound), errors.Is(err, errClaimNotFound),
		errors.Is(err, errGuildNotFound):
		return protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}
