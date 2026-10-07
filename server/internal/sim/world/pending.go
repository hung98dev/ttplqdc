package world

import (
	"time"

	"thinhthan/internal/core/id"
)

// PendingReason is the S2C_PLACEMENT_PENDING reason code
// (PLACEMENT_REASON_* wire values).
type PendingReason uint8

// Placement-pending reason values (session.proto PlacementReason).
const (
	PendingRespawn        PendingReason = 1
	PendingInstanceReturn PendingReason = 2
	PendingReconnect      PendingReason = 3 // server-initiated re-placement
	PendingFirstLogin     PendingReason = 4
)

// PendingWait is one queued forced placement: the 15 is emitted on entry
// and every PendingRetryMS (5s) until the placement resolves or the
// request cancels. RequestMessageID is the wire id of the trigger
// (6 attach / 208 respawn / 0 server-driven).
type PendingWait struct {
	CharacterID       id.UUID
	RequestMessageID  uint32
	Reason            PendingReason
	Request           PlacementRequest
	RetryAfterMs      uint32
	// Resolve runs when the retry placement succeeds — nil keeps the
	// character pending until the caller acts on PlacementResult.
	Resolve func(*Runtime, PlacementResult)
	nextAt            time.Time
}

// NewPendingWait constructs the wait with its first emit already implied:
// the caller emits the 15 once on creation; nextAt schedules the retry.
func NewPendingWait(now time.Time, characterID id.UUID, requestMessageID uint32,
	reason PendingReason, req PlacementRequest) *PendingWait {
	return &PendingWait{
		CharacterID:      characterID,
		RequestMessageID: requestMessageID,
		Reason:           reason,
		Request:          req,
		RetryAfterMs:     PendingRetryMS,
		nextAt:           now.Add(PendingRetryMS * time.Millisecond),
	}
}
