package anti_rmt

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// Evaluate runs all three predicates for accountID at instant at,
// inside tx. Callers hold the account's lock order; rollup reads ride
// the priority-200 EconomyRollups family.
func (s *Store) evaluate(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, at time.Time) ([]Predicate, error) {
	at = at.UTC()
	start7 := at.AddDate(0, 0, -WindowDays7)
	start30 := at.AddDate(0, 0, -WindowDays30)
	// Lock order: accounts(10) before EconomyRollups(200). The flag
	// write later in applyFlag rides the accounts lock taken here.
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("accounts", accountID)); err != nil {
		return nil, err
	}
	chars, err := accountCharacters(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	locks := []lockorder.Lock{
		lockorder.RowLock("economy_account_daily_rollups", accountID)}
	for _, cs := range chars {
		c, err := id.ParseUUID(cs)
		if err != nil {
			return nil, fmt.Errorf("anti_rmt: character id: %w", err)
		}
		locks = append(locks,
			lockorder.RowLock("economy_character_daily_rollups", c))
	}
	if err := lockorder.SortLocks(locks); err != nil {
		return nil, err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return nil, err
	}
	p1, err := s.evalNetOutflow(ctx, tx, accountID, start7, at)
	if err != nil {
		return nil, err
	}
	p2, err := s.evalPartnerConcentration(ctx, tx, accountID, chars, start30, at)
	if err != nil {
		return nil, err
	}
	p3, err := s.evalItemConcentration(ctx, tx, accountID, chars, start30, at)
	if err != nil {
		return nil, err
	}
	return []Predicate{p1, p2, p3}, nil
}

// evalNetOutflow implements predicate (a): account net outflow over
// [T-7d,T) > 20,000,000 common AND inside the top-1% kth-cutoff of the
// active non-tombstone population (k = ceil(N/100); all ties at the
// cutoff qualify).
func (s *Store) evalNetOutflow(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, start, at time.Time) (Predicate, error) {
	p := Predicate{Name: "net_outflow_7d_top_1pct"}
	nets, err := populationNets(ctx, tx, start, at)
	if err != nil {
		return p, err
	}
	// The caller's own net uses the same whole-day + raw-partial sum
	// (its events may not have produced rollup rows yet).
	outflow, inflow, err := acctFlow(ctx, tx, accountID, start, at)
	if err != nil {
		return p, err
	}
	tomb, err := tombstoneSet(ctx, tx, nets)
	if err != nil {
		return p, err
	}
	for a := range tomb {
		delete(nets, a)
	}
	sid := accountID.String()
	hadEvent := false
	if _, ok := nets[sid]; ok {
		hadEvent = true
	} else if outflow != 0 || inflow != 0 {
		hadEvent = true
	}
	if _, ok := nets[sid]; !ok && hadEvent {
		nets[sid] = outflow - inflow
	}
	n := len(nets)
	p.Evidence = map[string]any{
		"net_outflow": outflow - inflow,
		"population":  n,
	}
	if !hadEvent || n == 0 {
		return p, nil
	}
	net := nets[sid]
	k := (n + 99) / 100 // ceil(n/100)
	sorted := make([]int64, 0, n)
	for _, v := range nets {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] > sorted[j] })
	cutoff := sorted[k-1]
	p.Evidence["cutoff"] = cutoff
	p.Qualified = net > NetOutflowFlagCommon && net >= cutoff
	return p, nil
}

// accountCharacters lists the account's character ids (OR-of-
// predicates evaluation unit per character).
func accountCharacters(ctx context.Context, tx pgx.Tx,
	accountID id.UUID) ([]string, error) {
	rows, err := tx.Query(ctx,
		`SELECT character_id FROM characters WHERE account_id=$1`,
		accountID.String())
	if err != nil {
		return nil, fmt.Errorf("anti_rmt: characters: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// evalPartnerConcentration implements predicate (b): for any of the
// account's characters, total_outflow_30d >= 5,000,000 AND top-3
// partner share >= 0.80 on trade_partner_volumes (aggregated per
// partner before ranking, integer cross-multiplication).
func (s *Store) evalPartnerConcentration(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, chars []string, start, at time.Time) (Predicate, error) {
	p := Predicate{Name: "partner_concentration_30d"}
	var best int64
	for _, cs := range chars {
		c, err := id.ParseUUID(cs)
		if err != nil {
			return p, fmt.Errorf("anti_rmt: character id: %w", err)
		}
		vols, err := charPartnerVolumes(ctx, tx, c, start, at)
		if err != nil {
			return p, err
		}
		total := sumOf(vols)
		if total > best {
			best = total
		}
		if total >= PartnerTotalMinCommon && concentration(vols) {
			p.Qualified = true
			p.Evidence = map[string]any{
				"character_id":  cs,
				"total_outflow": total,
				"top3":          top3Sum(vols),
			}
			return p, nil
		}
	}
	p.Evidence = map[string]any{"max_total_outflow": best}
	return p, nil
}

// evalItemConcentration implements predicate (c): for any of the
// account's characters, received_items_30d >= 50 AND top-3 source
// share >= 0.80 on item_partner_counts.
func (s *Store) evalItemConcentration(ctx context.Context, tx pgx.Tx,
	accountID id.UUID, chars []string, start, at time.Time) (Predicate, error) {
	p := Predicate{Name: "item_concentration_30d"}
	var best int64
	for _, cs := range chars {
		c, err := id.ParseUUID(cs)
		if err != nil {
			return p, fmt.Errorf("anti_rmt: character id: %w", err)
		}
		counts, err := charItemCounts(ctx, tx, c, start, at)
		if err != nil {
			return p, err
		}
		total := sumOf(counts)
		if total > best {
			best = total
		}
		if total >= ReceivedItemsMin && concentration(counts) {
			p.Qualified = true
			p.Evidence = map[string]any{
				"character_id":   cs,
				"received_items": total,
				"top3":           top3Sum(counts),
			}
			return p, nil
		}
	}
	p.Evidence = map[string]any{"max_received_items": best}
	return p, nil
}
