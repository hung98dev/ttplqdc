package trade

import (
	"math"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

const (
	// MinLevel is the direct-trade admission floor (trading_auction.md §
	// Eligibility).
	MinLevel = 10
	// MinAge is the minimum character age for admission.
	MinAge = 24 * time.Hour
	// RangeMeters is the same-map proximity bound for invites.
	RangeMeters = 4.0
)

// ParticipantView is the runtime view of one character the manager
// consults for admission: identity gates plus the map-instance
// proximity/combat/death state the invite path checks.
type ParticipantView struct {
	AccountID id.UUID
	Level     int
	Age       time.Duration
	MapID     string
	X, Y      float64
	InCombat  bool
	Dead      bool
	Online    bool
}

// Distance returns the planar distance between two participants.
func Distance(a, b ParticipantView) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

// CheckEligibility maps the static gates to their error code; ok=false
// when admission must reject.
func CheckEligibility(v ParticipantView) (protocolv1.ErrorCode, bool) {
	if v.Level < MinLevel {
		return protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_LEVEL_REQUIRED, false
	}
	if v.Age < MinAge {
		return protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_AGE_REQUIRED, false
	}
	return protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, true
}

// CheckProximity maps the spatial/combat gates (invite admission and
// the finalise re-check performed session-side before committing).
func CheckProximity(a, b ParticipantView) (protocolv1.ErrorCode, bool) {
	if a.MapID == "" || a.MapID != b.MapID || Distance(a, b) > RangeMeters {
		return protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE, false
	}
	if a.InCombat || b.InCombat {
		return protocolv1.ErrorCode_ERROR_CODE_IN_COMBAT, false
	}
	if a.Dead || b.Dead {
		return protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE, false
	}
	return protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, true
}

// Presence is the map-instance seam the manager consults: participant
// state and the IMP-034 can_direct_interact gate (wired by the
// composition root once IMP-034 lands; nil means no block consult).
type Presence interface {
	// Participant resolves a character's runtime view; ok=false when the
	// character is not present on this map instance.
	Participant(charID id.UUID) (ParticipantView, bool)
	// CanDirectInteract applies the shared direct-interaction gate;
	// nil-safe — a nil implementation admits.
	CanDirectInteract(a, b id.UUID) error
}

// PresenceFunc adapts a lookup function (tests).
type PresenceFunc func(charID id.UUID) (ParticipantView, bool)

// Participant implements Presence.
func (f PresenceFunc) Participant(charID id.UUID) (ParticipantView, bool) { return f(charID) }

// CanDirectInteract implements Presence.
func (f PresenceFunc) CanDirectInteract(a, b id.UUID) error { return nil }
