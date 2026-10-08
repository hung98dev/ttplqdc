package auction

import (
	"context"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/reward"
)

// RewardClaimSink adapts IMP-010's claim-creation API to the sweep's
// EscrowClaimSink seam: one SINGLE-kind claim per dematerialized escrow
// asset (reward_claims.md source_type AUCTION_ESCROW_EXPIRY, ADR-0062
// alwaysSettles). The sweep supplies the deterministic operation_id, so
// replays dedupe on the contribution ledger.
type RewardClaimSink struct {
	Store *reward.Store
}

// CreateEscrowExpiryClaim maps the escrow asset onto one reward claim
// input. The item's recorded content revision is required claim payload
// (ADR-0012); a NULL revision means the escrowed instance predates
// revision stamping and is surfaced as an integrity error rather than
// stamping a fabricated revision into the immutable payload.
func (s RewardClaimSink) CreateEscrowExpiryClaim(ctx context.Context, tx pgx.Tx,
	in EscrowExpiryClaim) (id.UUID, error) {
	if in.ContentRevision == nil {
		return id.UUID{}, fmt.Errorf("auction: escrow asset %s missing content revision",
			in.SourceReference)
	}
	out, err := s.Store.Create(ctx, tx, &reward.Input{
		OwnerCharacterID:        in.OwnerCharacterID,
		SourceType:              "AUCTION_ESCROW_EXPIRY",
		SourceReference:         in.SourceReference,
		RewardSlot:              "escrow",
		SourceRewardOperationID: in.OperationID,
		Lines: []reward.LineInput{{
			Kind:             "ITEM",
			ItemID:           in.ItemID,
			Quantity:         big.NewInt(in.Quantity),
			EffectiveBinding: in.EffectiveBinding,
			ItemState:        in.ItemState,
			ContentRevision:  *in.ContentRevision,
			PerInstance:      true,
		}},
	})
	if err != nil {
		return id.UUID{}, fmt.Errorf("auction: escrow claim %s: %w", in.SourceReference, err)
	}
	return out.ClaimID, nil
}
