package atlas

import (
	"context"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TierOpID is the deterministic UUIDv5 reward_operation_id for a tier
// triple — the single idempotency key for promotion, EXP, currency and
// entitlement of that tier (atlas.md: the key never re-rolls).
func TierOpID(charID id.UUID, pageID string, tier uint32) id.UUID {
	return id.ServerJobOperationID("atlas.tier",
		charID.String(), pageID, strconv.Itoa(int(tier)))
}

// TierKey is the canonical tier-triple key string
// atlas.tier.<character_id>.<atlas_page_id>.<tier>.
func TierKey(charID id.UUID, pageID string, tier uint32) string {
	return "atlas.tier." + charID.String() + "." + pageID + "." + strconv.Itoa(int(tier))
}

// outcomeResult wraps an S2C client result in a JournalOutcome payload.
func outcomeResult(status protocolv1.ResultStatus, result *protocolv1.S2CAtlasClaimResult) *journalv1.JournalOutcome {
	out := &journalv1.JournalOutcome{Status: status}
	if result != nil {
		out.ClientResult = &journalv1.JournalOutcome_S2CAtlasClaimResult{S2CAtlasClaimResult: result}
	}
	return out
}

// operationResult builds the wire OperationResult for an op id.
func operationResult(opID id.UUID, status protocolv1.ResultStatus, code protocolv1.ErrorCode) *protocolv1.OperationResult {
	return &protocolv1.OperationResult{
		OperationId: opID[:],
		Status:      status,
		ErrorCode:   code,
	}
}

// SnapshotView builds the sorted S2C_ATLAS_STATE projection over the full
// authored roster: locked pages report counter/tier zero with no tier
// rows; reached tiers emit their deterministic reward_operation_id, the
// settled grant view and the page-level acknowledgement timestamp.
// Pages are sorted by page_id, tier rows ascending (messages.md §518).
func SnapshotView(charID id.UUID, rows []PageRow, revision uint64,
	settled map[string]map[uint32]*protocolv1.AtlasGrantView) *protocolv1.S2CAtlasState {
	byID := map[string]PageRow{}
	for _, r := range rows {
		byID[r.PageID] = r
	}
	roster := Roster()
	sort.Slice(roster, func(i, j int) bool { return roster[i].ID < roster[j].ID })
	out := &protocolv1.S2CAtlasState{AtlasRevision: revision}
	for _, p := range roster {
		r, ok := byID[p.ID]
		pv := &protocolv1.AtlasPageView{AtlasPageId: p.ID}
		if ok {
			pv.Counter = r.Counter
			pv.ReachedTier = r.ReachedTier
		}
		var ackMs int64
		if r.AcknowledgedAt != nil {
			ackMs = r.AcknowledgedAt.UnixMilli()
		}
		for t := uint32(1); t <= r.ReachedTier && t <= 3; t++ {
			op := TierOpID(charID, p.ID, t)
			tv := &protocolv1.AtlasTierView{
				Tier:              t,
				RewardOperationId: op[:],
				AcknowledgedAtMs:  ackMs,
			}
			if settled != nil {
				tv.SettledGrant = settled[p.ID][t]
			}
			pv.Tiers = append(pv.Tiers, tv)
		}
		out.Pages = append(out.Pages, pv)
	}
	return out
}

// marshalOutcome serializes an outcome payload for the durable op ledger.
func marshalOutcome(o *journalv1.JournalOutcome) ([]byte, error) {
	return protojson.Marshal(o)
}

// NewSnapshot builds the projection used by the emission path; reads
// through tx so it stays consistent with the commit.
func NewSnapshot(ctx context.Context, tx pgx.Tx, s *Store, charID id.UUID,
	settled map[string]map[uint32]*protocolv1.AtlasGrantView) (*protocolv1.S2CAtlasState, error) {
	rows, err := s.Snapshot(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	rev, err := s.Revision(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	return SnapshotView(charID, rows, rev, settled), nil
}
