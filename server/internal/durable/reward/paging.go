package reward

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ADR-0064 paging contract: the attach snapshot (434) and the read
// surface (439/440) share one ordering — oldest PENDING first by
// (created_at, reward_claim_id); pages are 50-row bounded so no frame
// exceeds the outbound limit at any claim count.
const (
	// PageMax is the hard 439 limit bound (1..50).
	PageMax = pageLimit
	// StateCap is the 434 `cap` field value (soft cap, not page size).
	StateCap = statePushCap
)

// View maps one claim row set to the wire RewardClaimView. Line
// quantities report the remaining undelivered amount (exact decimal
// truncated to the wire uint32 view only for display; the authoritative
// counters stay NUMERIC(38,0)).
func viewOf(c *Claim) *protocolv1.RewardClaimView {
	cid := c.ClaimID.Bytes()
	v := &protocolv1.RewardClaimView{
		RewardClaimId:   cid[:],
		SourceType:      c.SourceType,
		SourceReference: c.SourceReference,
		RewardSlot:      c.RewardSlot,
		CreatedAtMs:     c.CreatedAtMs,
		ExpiresAtMs:     c.ExpiresAtMs,
	}
	switch c.State {
	case StatePending, StateClaiming:
		v.State = protocolv1.RewardClaimState_REWARD_CLAIM_STATE_PENDING
	case StateClaimed:
		v.State = protocolv1.RewardClaimState_REWARD_CLAIM_STATE_CLAIMED
	}
	for _, l := range c.Lines {
		line := &protocolv1.RewardClaimLine{Quantity: uint32(remView(l))}
		switch l.Kind {
		case lineItem:
			line.Grant = &protocolv1.RewardClaimLine_ItemId{ItemId: l.ItemID}
		case lineCurrency:
			line.Grant = &protocolv1.RewardClaimLine_CurrencyId{CurrencyId: l.CurrencyID}
		}
		v.Lines = append(v.Lines, line)
	}
	return v
}

// remView clamps an exact remainder into the wire view (display only).
func remView(l *Line) int64 {
	r := l.Remaining()
	if !r.IsInt64() {
		return 1 << 31
	}
	return r.Int64()
}

// pagePendingLocked lists PENDING claims oldest-first inside the
// caller's tx (character lock held by the caller for a consistent read).
func (s *Store) pagePendingLocked(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, offset, limit uint32) ([]*Claim, error) {
	rows, err := tx.Query(ctx,
		`SELECT reward_claim_id FROM reward_claims
		 WHERE owner_character_id = $1 AND state = 'PENDING'
		 ORDER BY created_at, reward_claim_id
		 OFFSET $2 LIMIT $3`,
		characterID, offset, limit)
	if err != nil {
		return nil, err
	}
	var ids []id.UUID
	for rows.Next() {
		var c id.UUID
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*Claim, 0, len(ids))
	for _, cid := range ids {
		c, err := s.loadClaimForUpdate(ctx, tx, cid)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// StatePush builds S2C_REWARD_CLAIMS_STATE (434): total_count + the 50
// oldest PENDING claims + the current claims_revision. The edge calls
// it after attach; REPLACEABLE_STATE (latest wins on coalescing).
func (s *Store) StatePush(ctx context.Context, tx pgx.Tx,
	characterID id.UUID) (*protocolv1.S2CRewardClaimsState, error) {
	if err := lockCharacter(ctx, tx, characterID); err != nil {
		return nil, err
	}
	rev, err := s.ClaimsRevision(ctx, tx, characterID)
	if err != nil {
		return nil, err
	}
	total, err := s.totalPending(ctx, tx, characterID)
	if err != nil {
		return nil, err
	}
	page, err := s.pagePendingLocked(ctx, tx, characterID, 0, pageLimit)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CRewardClaimsState{
		ClaimsRevision: uint64(rev),
		TotalCount:     uint32(total),
		Cap:            statePushCap,
	}
	for _, c := range page {
		out.Claims = append(out.Claims, viewOf(c))
	}
	return out, nil
}

// ListPage serves C2S_REWARD_CLAIM_LIST_REQUEST (439): read-only, same
// oldest-first order as 434; limit bounds to 1..50 (ADR-0064).
func (s *Store) ListPage(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, offset, limit uint32) (*protocolv1.S2CRewardClaimListResult, error) {
	if limit == 0 || limit > pageLimit {
		limit = pageLimit
	}
	if err := lockCharacter(ctx, tx, characterID); err != nil {
		return nil, err
	}
	rev, err := s.ClaimsRevision(ctx, tx, characterID)
	if err != nil {
		return nil, err
	}
	total, err := s.totalPending(ctx, tx, characterID)
	if err != nil {
		return nil, err
	}
	page, err := s.pagePendingLocked(ctx, tx, characterID, offset, limit)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CRewardClaimListResult{
		ClaimsRevision: uint64(rev),
		TotalCount:     uint32(total),
		Offset:         offset,
	}
	for _, c := range page {
		out.Claims = append(out.Claims, viewOf(c))
	}
	return out, nil
}
