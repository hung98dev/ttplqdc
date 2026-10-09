package anti_rmt

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// populationNets returns the non-tombstone accounts with at least one
// settled direct-trade or Auction purchase/proceeds event in
// [start,at), each with net outflow (outflow-inflow, may be <= 0).
// Whole days come from account rollups; partial days from raw rows.
func populationNets(ctx context.Context, tx pgx.Tx,
	start, at time.Time) (map[string]int64, error) {
	nets := map[string]int64{}
	dayFrom, dayTo, oF, oT, cF, cT := dayBounds(start, at)
	if !dayFrom.After(dayTo) {
		rows, err := tx.Query(ctx,
			`SELECT r.account_id,
			        COALESCE(SUM(r.common_outflow),0), COALESCE(SUM(r.common_inflow),0)
			 FROM economy_account_daily_rollups r
			 JOIN accounts a ON a.account_id = r.account_id
			 WHERE r.utc_day >= $1 AND r.utc_day <= $2
			   AND a.status <> 'TOMBSTONE_ERASED'
			 GROUP BY r.account_id`, dayFrom, dayTo)
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: population rollups: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var acct string
			var o, i int64
			if err := rows.Scan(&acct, &o, &i); err != nil {
				return nil, err
			}
			nets[acct] += o - i
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, r := range [][2]time.Time{{oF, oT}, {cF, cT}} {
		if r[0].IsZero() || !r[0].Before(r[1]) {
			continue
		}
		if err := populationRaw(ctx, tx, r[0], r[1], nets); err != nil {
			return nil, err
		}
	}
	return nets, nil
}

// populationRaw folds one partial-day raw interval into nets, using
// the same flow semantics as the rollups. Every row marks its
// accounts active even when the net contribution is zero.
func populationRaw(ctx context.Context, tx pgx.Tx,
	from, to time.Time, nets map[string]int64) error {
	rows, err := tx.Query(ctx,
		`SELECT r.initiator_account_id, r.counterpart_account_id,
		        r.common_sent_by_initiator, r.common_sent_by_counterpart
		 FROM trade_settlement_records r
		 JOIN accounts ai ON ai.account_id = r.initiator_account_id
		 JOIN accounts ac ON ac.account_id = r.counterpart_account_id
		 WHERE r.settled_at >= $1 AND r.settled_at < $2`,
		from, to)
	if err != nil {
		return fmt.Errorf("anti_rmt: population trade: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var init, cpart string
		var initSent, cpartSent int64
		if err := rows.Scan(&init, &cpart, &initSent, &cpartSent); err != nil {
			return err
		}
		nets[init] += initSent - (cpartSent - feeOf(cpartSent))
		nets[cpart] += cpartSent - (initSent - feeOf(initSent))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	prows, err := tx.Query(ctx,
		`SELECT p.buyer_account_id, p.seller_account_id,
		        l.price_common, p.proceeds_amount
		 FROM auction_proceeds p
		 JOIN auction_listings l ON l.listing_id = p.listing_id
		 WHERE p.settled_at >= $1 AND p.settled_at < $2`,
		from, to)
	if err != nil {
		return fmt.Errorf("anti_rmt: population auction: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var buyer, seller string
		var price, proceeds int64
		if err := prows.Scan(&buyer, &seller, &price, &proceeds); err != nil {
			return err
		}
		nets[buyer] += price
		nets[seller] -= proceeds
	}
	return prows.Err()
}

// tombstoneSet returns account_ids present in nets that are tombstoned;
// callers subtract them (population must exclude tombstones even when
// raw rows still reference them).
func tombstoneSet(ctx context.Context, tx pgx.Tx,
	nets map[string]int64) (map[string]bool, error) {
	accts := make([]string, 0, len(nets))
	for a := range nets {
		accts = append(accts, a)
	}
	out := map[string]bool{}
	if len(accts) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx,
		`SELECT account_id FROM accounts
		 WHERE account_id = ANY($1) AND status = 'TOMBSTONE_ERASED'`, accts)
	if err != nil {
		return nil, fmt.Errorf("anti_rmt: tombstones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out[a] = true
	}
	return out, rows.Err()
}

// jsonbMapSums folds jsonb map columns across whole-day rollup rows:
// each partner's value is aggregated across days before any ranking.
func jsonbMapSums(ctx context.Context, tx pgx.Tx, charID id.UUID,
	column string, dayFrom, dayTo time.Time) (map[string]int64, error) {
	q := fmt.Sprintf(
		`SELECT k, COALESCE(SUM(v),0) FROM economy_character_daily_rollups r,
		 LATERAL (SELECT (e).key AS k, (e).value::bigint AS v
		          FROM jsonb_each(r.%s) e) m
		 WHERE r.character_id=$1 AND r.utc_day >= $2 AND r.utc_day <= $3
		 GROUP BY k`, column)
	rows, err := tx.Query(ctx, q, charID.String(), dayFrom, dayTo)
	if err != nil {
		return nil, fmt.Errorf("anti_rmt: %s rollup: %w", column, err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// charPartnerVolumes aggregates one character's partner common-outflow
// map over window [start,at): whole-day rollup rows plus partial-day
// raw rows (own side's gross sent per counterpart; auction gross price
// per seller).
func charPartnerVolumes(ctx context.Context, tx pgx.Tx, charID id.UUID,
	start, at time.Time) (map[string]int64, error) {
	dayFrom, dayTo, oF, oT, cF, cT := dayBounds(start, at)
	vols := map[string]int64{}
	if !dayFrom.After(dayTo) {
		m, err := jsonbMapSums(ctx, tx, charID, "trade_partner_volumes", dayFrom, dayTo)
		if err != nil {
			return nil, err
		}
		vols = m
	}
	for _, r := range [][2]time.Time{{oF, oT}, {cF, cT}} {
		if r[0].IsZero() || !r[0].Before(r[1]) {
			continue
		}
		rows, err := tx.Query(ctx,
			`SELECT initiator_character_id, counterpart_character_id,
			        common_sent_by_initiator, common_sent_by_counterpart
			 FROM trade_settlement_records
			 WHERE settled_at >= $1 AND settled_at < $2
			   AND (initiator_character_id=$3 OR counterpart_character_id=$3)`,
			r[0], r[1], charID.String())
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: raw partner vols: %w", err)
		}
		for rows.Next() {
			var init, cpart string
			var initSent, cpartSent int64
			if err := rows.Scan(&init, &cpart, &initSent, &cpartSent); err != nil {
				rows.Close()
				return nil, err
			}
			if init == charID.String() {
				vols[cpart] += initSent
			}
			if cpart == charID.String() {
				vols[init] += cpartSent
			}
		}
		rows.Close()
		prows, err := tx.Query(ctx,
			`SELECT p.seller_character_id, l.price_common
			 FROM auction_proceeds p
			 JOIN auction_listings l ON l.listing_id = p.listing_id
			 WHERE p.settled_at >= $1 AND p.settled_at < $2
			   AND p.buyer_character_id=$3`,
			r[0], r[1], charID.String())
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: raw partner vols auction: %w", err)
		}
		for prows.Next() {
			var seller string
			var price int64
			if err := prows.Scan(&seller, &price); err != nil {
				prows.Close()
				return nil, err
			}
			vols[seller] += price
		}
		prows.Close()
	}
	return vols, nil
}

// charItemCounts aggregates one character's received-item source map
// over window [start,at): whole-day item_partner_counts plus raw
// partial-day trade item_transfers, auction_proceeds quantities, and
// guild storage audits (depositor->receiver, distinct characters).
func charItemCounts(ctx context.Context, tx pgx.Tx, charID id.UUID,
	start, at time.Time) (map[string]int64, error) {
	dayFrom, dayTo, oF, oT, cF, cT := dayBounds(start, at)
	counts := map[string]int64{}
	if !dayFrom.After(dayTo) {
		m, err := jsonbMapSums(ctx, tx, charID, "item_partner_counts", dayFrom, dayTo)
		if err != nil {
			return nil, err
		}
		counts = m
	}
	for _, r := range [][2]time.Time{{oF, oT}, {cF, cT}} {
		if r[0].IsZero() || !r[0].Before(r[1]) {
			continue
		}
		// trade_settlement_records.item_transfers: [{from_character_id,
		// to_character_id, quantity}] — receiver-side source units.
		rows, err := tx.Query(ctx,
			`SELECT (t->>'from_character_id') AS src, SUM((t->>'quantity')::bigint)
			 FROM trade_settlement_records r,
			      LATERAL jsonb_array_elements(r.item_transfers) t
			 WHERE r.settled_at >= $1 AND r.settled_at < $2
			   AND (t->>'to_character_id') = $3
			 GROUP BY src`, r[0], r[1], charID.String())
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: raw item counts: %w", err)
		}
		for rows.Next() {
			var src string
			var qty int64
			if err := rows.Scan(&src, &qty); err != nil {
				rows.Close()
				return nil, err
			}
			counts[src] += qty
		}
		rows.Close()
		prows, err := tx.Query(ctx,
			`SELECT seller_character_id, SUM(quantity)
			 FROM auction_proceeds
			 WHERE settled_at >= $1 AND settled_at < $2
			   AND buyer_character_id=$3
			 GROUP BY seller_character_id`, r[0], r[1], charID.String())
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: raw item counts auction: %w", err)
		}
		for prows.Next() {
			var src string
			var qty int64
			if err := prows.Scan(&src, &qty); err != nil {
				prows.Close()
				return nil, err
			}
			counts[src] += qty
		}
		prows.Close()
		grows, err := tx.Query(ctx,
			`SELECT source_character_id, SUM(quantity)
			 FROM guild_storage_audit
			 WHERE occurred_at >= $1 AND occurred_at < $2
			   AND receiver_character_id=$3
			   AND action IN ('WITHDRAW','CLAIM_DELIVER')
			   AND source_character_id IS NOT NULL
			   AND source_character_id <> receiver_character_id
			 GROUP BY source_character_id`, r[0], r[1], charID.String())
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: raw item counts guild: %w", err)
		}
		for grows.Next() {
			var src string
			var qty int64
			if err := grows.Scan(&src, &qty); err != nil {
				grows.Close()
				return nil, err
			}
			counts[src] += qty
		}
		grows.Close()
	}
	return counts, nil
}

// sumOf returns the total of a partner/value map.
func sumOf(m map[string]int64) int64 {
	var t int64
	for _, v := range m {
		t += v
	}
	return t
}

// top3Sum returns the sum of the three largest values; equal values
// are tie-broken by ascending key for determinism.
func top3Sum(m map[string]int64) int64 {
	type kv struct {
		k string
		v int64
	}
	top := [3]kv{}
	for k, v := range m {
		e := kv{k, v}
		for i := range top {
			if e.v > top[i].v || (e.v == top[i].v && e.k < top[i].k) {
				copy(top[i+1:], top[i:2])
				top[i] = e
				break
			}
		}
	}
	return top[0].v + top[1].v + top[2].v
}

// concentration returns true when the top-3 partner share reaches 0.80
// via integer cross-multiplication 5*top3 >= 4*total.
func concentration(m map[string]int64) bool {
	total := sumOf(m)
	if total <= 0 {
		return false
	}
	return ConcentrationNum*top3Sum(m) >= ConcentrationDen*total
}
