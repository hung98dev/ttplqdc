package auction

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestPurchaseRollupsUpsertedInSettlementTx asserts a FIXED_PRICE buy
// upserts the per-character and per-account daily rollups in the same
// transaction as the proceeds record (data_model.md § Economy
// Aggregation Fields): buyer outflow = gross price, seller inflow = net,
// partner volume/count maps accumulate.
func TestPurchaseRollupsUpsertedInSettlementTx(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 100_000)
	lid := listOK(t, s, sellerAcct, seller, inst, 1_000)
	var out *BuyOut
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		out, err = s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(),
			CharacterID: buyer, AccountID: buyerAcct, ListingID: lid,
			ExpectedPriceCommon: 1_000})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if out.Tax != 50 || out.Amount != 950 {
		t.Fatalf("settle %+v", out)
	}
	var outflow, inflow int64
	var volumes, counts string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT common_outflow, common_inflow,
		        trade_partner_volumes::text, item_partner_counts::text
		   FROM economy_character_daily_rollups
		  WHERE character_id=$1`, buyer.String()).
		Scan(&outflow, &inflow, &volumes, &counts); err != nil {
		t.Fatalf("buyer rollup: %v", err)
	}
	if outflow != 1_000 || inflow != 0 {
		t.Fatalf("buyer flow %d/%d", outflow, inflow)
	}
	if volumes != `{"`+seller.String()+`": 1000}` {
		t.Fatalf("buyer volumes %s", volumes)
	}
	if counts != `{"`+seller.String()+`": 1}` {
		t.Fatalf("buyer counts %s", counts)
	}
	if err := pool(t).QueryRow(context.Background(),
		`SELECT common_outflow, common_inflow
		   FROM economy_character_daily_rollups
		  WHERE character_id=$1`, seller.String()).
		Scan(&outflow, &inflow); err != nil {
		t.Fatalf("seller rollup: %v", err)
	}
	if outflow != 0 || inflow != 950 {
		t.Fatalf("seller flow %d/%d", outflow, inflow)
	}
	if err := pool(t).QueryRow(context.Background(),
		`SELECT common_outflow, common_inflow
		   FROM economy_account_daily_rollups
		  WHERE account_id=$1`, buyerAcct.String()).
		Scan(&outflow, &inflow); err != nil {
		t.Fatalf("buyer acct rollup: %v", err)
	}
	if outflow != 1_000 || inflow != 0 {
		t.Fatalf("buyer acct flow %d/%d", outflow, inflow)
	}
	if err := pool(t).QueryRow(context.Background(),
		`SELECT common_outflow, common_inflow
		   FROM economy_account_daily_rollups
		  WHERE account_id=$1`, sellerAcct.String()).
		Scan(&outflow, &inflow); err != nil {
		t.Fatalf("seller acct rollup: %v", err)
	}
	if outflow != 0 || inflow != 950 {
		t.Fatalf("seller acct flow %d/%d", outflow, inflow)
	}
}
