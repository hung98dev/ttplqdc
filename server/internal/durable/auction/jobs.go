package auction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// ExpireSweep transitions every ACTIVE listing whose 24 h window closed
// to EXPIRED, stamping ended_at (ADR-0070). Invoked by the `auction`
// JournalJob sweep; the JournalAuctionJob listing_id key narrows the run
// to one listing when present.
func (s *Store) ExpireSweep(ctx context.Context, tx pgx.Tx, at time.Time, only id.UUID) (int, error) {
	where := `state='ACTIVE' AND expires_at <= $1`
	args := []any{at}
	if !only.IsNil() {
		where += ` AND listing_id = $2`
		args = append(args, only.String())
	}
	tag, err := tx.Exec(ctx, `UPDATE auction_listings
	   SET state='EXPIRED', ended_at=$1, revision=revision+1
	 WHERE `+where, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// EscrowClaimSink is the claim-creation seam the 7-day escrow sweep
// consumes: IMP-010's durable/reward claim-creation API
// (reward_claims.md source_type AUCTION_ESCROW_EXPIRY). The sweep never
// writes claim rows itself — one owner per concept.
type EscrowClaimSink interface {
	// CreateEscrowExpiryClaim persists one SINGLE-kind Reward Claim for
	// the dematerialized escrow asset (immutable creation payload per
	// ADR-0012). Must be idempotent on operationID.
	CreateEscrowExpiryClaim(ctx context.Context, tx pgx.Tx, in EscrowExpiryClaim) (id.UUID, error)
}

// EscrowExpiryClaim is the typed hand-off to the claim owner.
type EscrowExpiryClaim struct {
	OperationID      id.UUID // deterministic per (listing, sweep): replay-safe
	OwnerCharacterID id.UUID
	SourceReference  string // listing_id — provenance for the claim
	ItemID           string
	Quantity         int64
	EffectiveBinding string
	ItemState        []byte
	ContentRevision  *string
}

// MoveToClaimSweep moves every cancelled/expired asset unreclaimed for
// EscrowClaimDelay into a persistent Reward Claim for the seller
// character and removes it from auction escrow (trading_auction.md §
// Cancel / Expire / Reclaim): the listing transitions to MOVED_TO_CLAIM
// (ended_at overwritten, ADR-0070), the item_locations row is deleted
// (the instance remains as the listing's provenance row — its
// auction_listings FK is RESTRICT), and the claim owner stores the
// immutable re-materialization payload.
func (s *Store) MoveToClaimSweep(ctx context.Context, tx pgx.Tx, at time.Time,
	sink EscrowClaimSink, operationFor func(listingID id.UUID) id.UUID) (int, error) {
	rows, err := tx.Query(ctx, `SELECT listing_id FROM auction_listings
	  WHERE state IN ('CANCELLED','EXPIRED') AND ended_at <= $1
	  ORDER BY listing_id FOR UPDATE`, at.Add(-EscrowClaimDelay))
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var lid string
		if err := rows.Scan(&lid); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, lid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	moved := 0
	for _, lidStr := range ids {
		lid, err := id.ParseUUID(lidStr)
		if err != nil {
			return moved, err
		}
		l, err := s.loadForUpdate(ctx, tx, lid)
		if err != nil {
			return moved, err
		}
		if l.State != StateCancelled && l.State != StateExpired {
			continue
		}
		var itemID string
		var qty int64
		var binding string
		var itemState []byte
		var contentRev *string
		err = tx.QueryRow(ctx, `SELECT i.item_id, i.quantity, i.effective_binding,
			i.item_state, i.content_revision
		  FROM item_instances i
		  JOIN item_locations l ON l.item_instance_id = i.item_instance_id
		 WHERE i.item_instance_id=$1 AND l.location_kind='AUCTION_ESCROW'
		   AND l.listing_id=$2 FOR UPDATE OF i`,
			l.ItemInstanceID.String(), lidStr).Scan(&itemID, &qty, &binding, &itemState, &contentRev)
		if err != nil {
			return moved, fmt.Errorf("auction: load escrow asset: %w", err)
		}
		opID := id.NewV4()
		if operationFor != nil {
			opID = operationFor(lid)
		}
		if _, err := sink.CreateEscrowExpiryClaim(ctx, tx, EscrowExpiryClaim{
			OperationID:      opID,
			OwnerCharacterID: l.SellerCharacterID,
			SourceReference:  lidStr,
			ItemID:           itemID,
			Quantity:         qty,
			EffectiveBinding: binding,
			ItemState:        itemState,
			ContentRevision:  contentRev,
		}); err != nil {
			return moved, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM item_locations
		  WHERE item_instance_id=$1 AND location_kind='AUCTION_ESCROW'
		    AND listing_id=$2`, l.ItemInstanceID.String(), lidStr); err != nil {
			return moved, err
		}
		if err := s.transition(ctx, tx, lid, StateMovedToClaim, nil, at); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// EscrowAsset is one 744 my-state escrow row: the cancelled/expired
// asset plus its auto-claim deadline.
type EscrowAsset struct {
	ListingID      id.UUID
	ItemInstanceID id.UUID
	ItemID         string
	Quantity       int64
	State          ListingState
	AutoClaimAt    time.Time // ended_at + EscrowClaimDelay
}

// MyState is the S2C_AUCTION_MY_STATE (744) projection: own listings,
// escrow assets with auto-claim deadlines, and PENDING proceeds.
func (s *Store) MyState(ctx context.Context, tx pgx.Tx, characterID id.UUID) (
	listings []*Listing, escrow []*EscrowAsset, pending []*Proceeds, err error) {
	if err := s.gate(ctx, tx, characterID, false); err != nil {
		return nil, nil, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT listing_id, seller_character_id,
		seller_account_id, item_instance_id, item_id, quantity, price_common,
		listing_fee_common, state, listed_at, expires_at, ended_at,
		buyer_character_id, revision
	  FROM auction_listings WHERE seller_character_id=$1
	  ORDER BY listed_at DESC LIMIT 100`, characterID.String())
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		l := &Listing{}
		var buyer *string
		if err := rows.Scan(&l.ListingID, &l.SellerCharacterID, &l.SellerAccountID,
			&l.ItemInstanceID, &l.ItemID, &l.Quantity, &l.PriceCommon,
			&l.ListingFeeCommon, &l.State, &l.ListedAt, &l.ExpiresAt,
			&l.EndedAt, &buyer, &l.Revision); err != nil {
			return nil, nil, nil, err
		}
		if buyer != nil {
			if b, perr := id.ParseUUID(*buyer); perr == nil {
				l.BuyerCharacterID = &b
			}
		}
		listings = append(listings, l)
		if l.State == StateCancelled || l.State == StateExpired {
			a := &EscrowAsset{
				ListingID:      l.ListingID,
				ItemInstanceID: l.ItemInstanceID,
				ItemID:         l.ItemID,
				Quantity:       l.Quantity,
				State:          l.State,
			}
			if l.EndedAt != nil {
				a.AutoClaimAt = l.EndedAt.Add(EscrowClaimDelay)
			}
			escrow = append(escrow, a)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	prows, err := tx.Query(ctx, `SELECT proceeds_id, settled_at, seller_character_id,
		seller_account_id, buyer_character_id, buyer_account_id, proceeds_amount,
		item_id, quantity, listing_id, state, claimed_at, claim_operation_id
	  FROM auction_proceeds WHERE seller_character_id=$1 AND state='PENDING'
	  ORDER BY settled_at`, characterID.String())
	if err != nil {
		return nil, nil, nil, err
	}
	defer prows.Close()
	for prows.Next() {
		p := &Proceeds{}
		if err := prows.Scan(&p.ProceedsID, &p.SettledAt, &p.SellerCharacterID,
			&p.SellerAccountID, &p.BuyerCharacterID, &p.BuyerAccountID,
			&p.ProceedsAmount, &p.ItemID, &p.Quantity, &p.ListingID, &p.State,
			&p.ClaimedAt, &p.ClaimOperationID); err != nil {
			return nil, nil, nil, err
		}
		pending = append(pending, p)
	}
	return listings, escrow, pending, prows.Err()
}
