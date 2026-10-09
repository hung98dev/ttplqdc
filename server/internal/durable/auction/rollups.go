package auction

// rollups.go — the per-character and per-account daily economy rollups
// upserted inside the purchase settlement transaction (data_model.md §
// Economy Aggregation Fields: "At every settlement commit, the
// applicable rollup rows are upserted in the same transaction"). The
// settlement-level operation dedup covers the deltas; no independent
// rollup idempotency key exists.

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// rollups upserts the rollups one FIXED_PRICE purchase produced: the
// buyer sent the gross price and received the item units, the seller
// received the net proceeds (price - tax) and sent/received nothing
// else. day is the UTC calendar day of the settlement instant.
func (s *Store) rollups(ctx context.Context, tx pgx.Tx, l *Listing,
	buyer, buyerAccount id.UUID, out *BuyOut, day, now time.Time) error {
	if err := charRollup(ctx, tx, buyer, l.SellerCharacterID, 0,
		l.PriceCommon, l.Quantity, day, now); err != nil {
		return err
	}
	if err := charRollup(ctx, tx, l.SellerCharacterID, buyer, out.Amount,
		0, 0, day, now); err != nil {
		return err
	}
	if err := acctRollup(ctx, tx, buyerAccount, l.PriceCommon, 0, day, now); err != nil {
		return err
	}
	return acctRollup(ctx, tx, l.SellerAccountID, 0, out.Amount, day, now)
}

// charRollup upserts one character's daily row: outflow = common sent,
// inflow = common received; trade_partner_volumes accumulates common
// sent to the partner, item_partner_counts item units received from
// them (jsonb maps keyed by partner character_id).
func charRollup(ctx context.Context, tx pgx.Tx, self, partner id.UUID,
	received, sent, itemsReceived int64, day, now time.Time) error {
	pid := partner.String()
	_, err := tx.Exec(ctx,
		`INSERT INTO economy_character_daily_rollups
		 (character_id, utc_day, common_outflow, common_inflow,
		  trade_partner_volumes, item_partner_counts, updated_at)
		 VALUES ($1,$2,$3,$4, jsonb_build_object($5::text,$6::bigint),
		         jsonb_build_object($5::text,$7::bigint), $8)
		 ON CONFLICT (character_id, utc_day) DO UPDATE SET
		   common_outflow = economy_character_daily_rollups.common_outflow + EXCLUDED.common_outflow,
		   common_inflow  = economy_character_daily_rollups.common_inflow  + EXCLUDED.common_inflow,
		   trade_partner_volumes = jsonb_set(
		     COALESCE(economy_character_daily_rollups.trade_partner_volumes,'{}'::jsonb),
		     ARRAY[$5::text],
		     to_jsonb(COALESCE((economy_character_daily_rollups.trade_partner_volumes->>$5)::bigint,0) + $6::bigint)),
		   item_partner_counts = jsonb_set(
		     COALESCE(economy_character_daily_rollups.item_partner_counts,'{}'::jsonb),
		     ARRAY[$5::text],
		     to_jsonb(COALESCE((economy_character_daily_rollups.item_partner_counts->>$5)::bigint,0) + $7::bigint)),
		   updated_at = $8`,
		self.String(), day, sent, received, pid, sent, itemsReceived, now)
	if err != nil {
		return fmt.Errorf("auction: character rollup: %w", err)
	}
	return nil
}

// acctRollup upserts one account's daily common flow.
func acctRollup(ctx context.Context, tx pgx.Tx, accountID id.UUID,
	sent, received int64, day, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO economy_account_daily_rollups
		 (account_id, utc_day, common_outflow, common_inflow, updated_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (account_id, utc_day) DO UPDATE SET
		   common_outflow = economy_account_daily_rollups.common_outflow + EXCLUDED.common_outflow,
		   common_inflow  = economy_account_daily_rollups.common_inflow  + EXCLUDED.common_inflow,
		   updated_at = $5`,
		accountID.String(), day, sent, received, now)
	if err != nil {
		return fmt.Errorf("auction: account rollup: %w", err)
	}
	return nil
}
