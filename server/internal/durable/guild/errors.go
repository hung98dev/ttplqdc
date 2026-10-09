package guild

import "errors"

// Sentinel errors. Wire codes map only through the verdict helpers —
// executors convert domain errors into a 649 error outcome instead of
// surfacing them, unless the failure is infrastructural.
var (
	// ErrMalformedRecord marks a journal record that fails the closed
	// identity/shape contract (missing oneof, short UUID, wrong family).
	ErrMalformedRecord = errors.New("guild: malformed record")

	// ErrGuildNameInvalid rejects a name failing the canonical text
	// pipeline (text.md § Name Limits, guild row: 1..24 graphemes,
	// <=96 UTF-8 bytes, non-whitespace).
	ErrGuildNameInvalid = errors.New("guild: invalid name")

	errGuildNameTaken      = errors.New("guild: name taken")
	errGuildNotFound       = errors.New("guild: not found")
	errNotMember           = errors.New("guild: not a member")
	errAlreadyInGuild      = errors.New("guild: character already in a guild")
	errLevelTooLow         = errors.New("guild: level too low")
	errPermissionDenied    = errors.New("guild: permission denied")
	errCapacityFull        = errors.New("guild: capacity full")
	errViceCapacityFull    = errors.New("guild: vice-leader capacity full")
	errInviteNotFound      = errors.New("guild: no pending invite")
	errApplicationNotFound = errors.New("guild: no pending application")
	errApplicationCap      = errors.New("guild: pending application cap reached")
	errStateConflict       = errors.New("guild: state conflict")
	errDisbandBlocked      = errors.New("guild: disband blocked")
	errAlreadyResolved     = errors.New("guild: already resolved")
	errInsufficientFunds   = errors.New("guild: insufficient currency")
	errLeaderOfflineWindow = errors.New("guild: leader not inactive or claimant ineligible")
	errVoteClosed          = errors.New("guild: vote closed or ineligible")
	errNotCandidate        = errors.New("guild: not a blessing candidate")
)
