package anti_rmt

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// feePermille mirrors the trade settlement's offering-side fee
// (durable/trade/records.go): a participant's inflow is the partner's
// gross sent amount minus floor(sent*0.05).
func feeOf(offered int64) int64 {
	if offered <= 0 {
		return 0
	}
	return offered * 50 / 1000
}

// dayBounds returns the whole-day [from,to] rollup range plus the two
// partial raw ranges (oldest, current) covering window [start, at).
func dayBounds(start, at time.Time) (dayFrom, dayTo time.Time,
	oldFrom, oldTo, curFrom, curTo time.Time) {
	dayFrom = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	if !start.Equal(dayFrom) {
		oldFrom, oldTo = start, dayFrom.AddDate(0, 0, 1)
		dayFrom = oldTo
	}
	// dayFrom is now the first UTC day starting at/after start.
	dayEnd := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	if !at.Equal(dayEnd) {
		curFrom, curTo = dayEnd, at
	}
	// Whole days must end at or before at.
	dayTo = dayEnd.AddDate(0, 0, -1)
	if dayFrom.After(dayTo) {
		dayTo = dayFrom.AddDate(0, 0, -1) // empty rollup range
	}
	return
}

// acctFlow sums account rollups over whole days [dayFrom,dayTo] and
// raw settlement records over the two partial ranges.
func acctFlow(ctx context.Context, tx pgx.Tx, accountID id.UUID,
	start, at time.Time) (outflow, inflow int64, err error) {
	dayFrom, dayTo, oF, oT, cF, cT := dayBounds(start, at)
	if !dayFrom.After(dayTo) {
		err = tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(common_outflow),0), COALESCE(SUM(common_inflow),0)
			 FROM economy_account_daily_rollups
			 WHERE account_id=$1 AND utc_day >= $2 AND utc_day <= $3`,
			accountID.String(), dayFrom, dayTo).Scan(&outflow, &inflow)
		if err != nil {
			return 0, 0, fmt.Errorf("anti_rmt: acct rollups: %w", err)
		}
	}
	for _, r := range [][2]time.Time{{oF, oT}, {cF, cT}} {
		if r[0].IsZero() || !r[0].Before(r[1]) {
			continue
		}
		o, i, e := acctRawFlow(ctx, tx, accountID, r[0], r[1])
		if e != nil {
			return 0, 0, e
		}
		outflow += o
		inflow += i
	}
	return outflow, inflow, nil
}

// acctRawFlow mirrors the rollup accumulation on raw rows: a trade
// participant sends gross and receives partner_gross-fee; an auction
// buyer sends the gross price, the seller receives net proceeds.
func acctRawFlow(ctx context.Context, tx pgx.Tx, accountID id.UUID,
	from, to time.Time) (outflow, inflow int64, err error) {
	// Direct trade: out = own side sent, in = partner sent - fee.
	rows, err := tx.Query(ctx,
		`SELECT initiator_account_id, counterpart_account_id,
		        common_sent_by_initiator, common_sent_by_counterpart
		 FROM trade_settlement_records
		 WHERE settled_at >= $1 AND settled_at < $2
		   AND (initiator_account_id=$3 OR counterpart_account_id=$3)`,
		from, to, accountID.String())
	if err != nil {
		return 0, 0, fmt.Errorf("anti_rmt: raw trade: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var init, cpart string
		var initSent, cpartSent int64
		if err := rows.Scan(&init, &cpart, &initSent, &cpartSent); err != nil {
			return 0, 0, fmt.Errorf("anti_rmt: raw trade scan: %w", err)
		}
		if init == accountID.String() {
			outflow += initSent
			inflow += cpartSent - feeOf(cpartSent)
		}
		if cpart == accountID.String() {
			outflow += cpartSent
			inflow += initSent - feeOf(initSent)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	// Auction: buyer outflow = gross price; seller inflow = net proceeds.
	prows, err := tx.Query(ctx,
		`SELECT p.buyer_account_id, p.seller_account_id,
		        l.price_common, p.proceeds_amount
		 FROM auction_proceeds p
		 JOIN auction_listings l ON l.listing_id = p.listing_id
		 WHERE p.settled_at >= $1 AND p.settled_at < $2
		   AND (p.buyer_account_id=$3 OR p.seller_account_id=$3)`,
		from, to, accountID.String())
	if err != nil {
		return 0, 0, fmt.Errorf("anti_rmt: raw auction: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var buyer, seller string
		var price, proceeds int64
		if err := prows.Scan(&buyer, &seller, &price, &proceeds); err != nil {
			return 0, 0, fmt.Errorf("anti_rmt: raw auction scan: %w", err)
		}
		if buyer == accountID.String() {
			outflow += price
		}
		if seller == accountID.String() {
			inflow += proceeds
		}
	}
	return outflow, inflow, prows.Err()
}
