package reward

import (
	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// DeltaPush builds S2C_REWARD_CLAIM_DELTA (441): the AUTHORITATIVE_EVENT
// emitted for every committed claims-projection change, carrying the
// post-change claims_revision (+1 per committed tx), total_count, and
// the added/removed claim ids so the client applies an ordered delta.
// A client observing a non-contiguous revision re-requests 439 @ 0
// (messages.md §439-441).
func DeltaPush(revision, totalCount uint64, added []*Claim, removed []id.UUID) *protocolv1.S2CRewardClaimDelta {
	out := &protocolv1.S2CRewardClaimDelta{
		ClaimsRevision: revision,
		TotalCount:     uint32(totalCount),
	}
	for _, c := range added {
		out.Added = append(out.Added, viewOf(c))
	}
	for _, r := range removed {
		b := r.Bytes()
		out.Removed = append(out.Removed, b[:])
	}
	return out
}
