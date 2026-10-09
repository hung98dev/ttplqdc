package atlas

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// SettlementFamily is the server producer family for Atlas reward
// settlements (save_rules.md §7: kind "atlas" → sim.atlas_settlement).
const SettlementFamily = "sim.atlas_settlement"

// AcknowledgeFamily is the client family carrying C2S_ATLAS_CLAIM (504).
const AcknowledgeFamily = "atlas.acknowledge"

// RewardKind is the JournalRewardCommand kind routed to this consumer.
const RewardKind = "atlas"

// Deps is the fail-closed dependency set for the Atlas executors.
type Deps struct {
	Store    *Store
	Progress *progression.Store
	Now      func() time.Time
}

// settledKey is the character-scoped presentation-entitlement source_ref
// (character is already the row's PK component; VARCHAR(64) bound).
func settledKey(pageID string, tier uint32) string {
	return "atlas.tier." + pageID + "." + strconv.Itoa(int(tier))
}

// SettlementExecutor settles sim.atlas_settlement records: every
// atlas_progress slot applies its counter delta once (the durable op
// ledger fences replay of the whole command), promotes every crossed
// tier and auto-settles each promotion — currency.special, the derived
// presentation entitlement and one LIFE_SKILL EXP per current act —
// atomically in the same transaction under the canonical lock order,
// then bumps the Atlas revision once and reports each settled tier in
// the outcome's reward_slots.
func SettlementExecutor(d Deps) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		if rec.GetOperationFamily() != SettlementFamily {
			return idempotency.Outcome{}, fmt.Errorf("atlas: unsupported family %q", rec.GetOperationFamily())
		}
		rwd := rec.GetReward()
		if rwd == nil || rwd.GetKind() != RewardKind ||
			len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
			return idempotency.Outcome{}, fmt.Errorf("atlas: malformed reward record")
		}
		var charID id.UUID
		copy(charID[:], rec.GetOwnerId())

		locks := []lockorder.Lock{
			lockorder.RowLock("characters", charID),
			lockorder.RowLock("character_currencies", charID),
			lockorder.RowLock("character_atlas", charID),
			lockorder.RowLock("character_atlas_state", charID),
		}
		if err := lockorder.SortLocks(locks); err != nil {
			return idempotency.Outcome{}, err
		}
		if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
			return idempotency.Outcome{}, err
		}

		now := d.Now()
		changed := false
		var outSlots []*journalv1.JournalRewardSlot
		for _, slot := range rwd.GetSlots() {
			for _, ap := range slot.GetAtlasProgress() {
				page, ok := PageByID(ap.GetPageId())
				if !ok {
					return idempotency.Outcome{}, fmt.Errorf("atlas: unauthored page %q", ap.GetPageId())
				}
				delta := ap.GetDelta()
				if delta == 0 {
					continue
				}
				before, err := d.Store.Page(ctx, tx, charID, page.ID)
				if err != nil {
					return idempotency.Outcome{}, err
				}
				counter, err := d.Store.ApplyDelta(ctx, tx, charID, page.ID, page.Mode, delta)
				if err != nil {
					return idempotency.Outcome{}, err
				}
				changed = changed || counter != before.Counter
				for tier := uint32(1); tier <= 3; tier++ {
					if before.ReachedTier >= tier || counter < page.Thresholds[tier-1] {
						continue
					}
					opID := TierOpID(charID, page.ID, tier)
					promotedRow, err := d.Store.Promote(ctx, tx, charID, page.ID, tier, opID, now)
					if err != nil {
						return idempotency.Outcome{}, err
					}
					if !promotedRow {
						continue // tier already settled — replay never re-grants
					}
					expApplied, levelAfter, err := d.settleTier(ctx, tx, charID, page, tier, opID, now)
					if err != nil {
						return idempotency.Outcome{}, err
					}
					outSlots = append(outSlots, &journalv1.JournalRewardSlot{
						RewardSlot:              settledKey(page.ID, tier),
						CharacterExp:            uint64(expApplied),
						SourceRewardOperationId: opID[:],
						SourceType:              "atlas.tier",
						SourceReference:         page.ID,
					})
					_ = levelAfter
					changed = true
				}
			}
		}

		milestones, err := d.settleMilestones(ctx, tx, charID, now)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		for _, m := range milestones {
			outSlots = append(outSlots, &journalv1.JournalRewardSlot{
				RewardSlot:      m.ID,
				SourceType:      "atlas.milestone",
				SourceReference: m.ID,
			})
			changed = true
		}

		if changed {
			if _, err := d.Store.BumpRevision(ctx, tx, charID); err != nil {
				return idempotency.Outcome{}, err
			}
		}

		outcome := outcomeResult(protocolv1.ResultStatus_RESULT_STATUS_SUCCESS, nil)
		outcome.RewardSlots = outSlots
		payload, err := marshalOutcome(outcome)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
	}
}

// settleTier grants one tier bundle atomically: currency.special credit,
// the presentation entitlement row, and one LIFE_SKILL EXP grant for the
// character's current act. The tier triple key is the single idempotency
// identity — Promote already fenced re-entry, so a settled tier can never
// re-grant on replay.
func (d Deps) settleTier(ctx context.Context, tx pgx.Tx, charID id.UUID,
	p Page, tier uint32, opID id.UUID, now time.Time) (int64, int32, error) {
	if amt := p.Special[tier-1]; amt > 0 {
		if _, err := currency.Credit(ctx, tx, currency.Mutation{
			CharacterID: charID,
			CurrencyID:  currency.Special,
			Delta:       amt,
			OperationID: opID,
			ReasonCode:  "atlas.tier",
			SourceRef:   settledKey(p.ID, tier),
			Actor:       currency.ActorSystem,
		}); err != nil {
			return 0, 0, err
		}
	}
	if cosmetic := p.PresentationEntitlement(int(tier)); cosmetic != "" {
		if _, err := d.Store.GrantCosmetic(ctx, tx, charID, cosmetic,
			settledKey(p.ID, tier), opID, now); err != nil {
			return 0, 0, err
		}
	}
	prog, err := d.Progress.Read(ctx, tx, charID)
	if err != nil {
		return 0, 0, err
	}
	exp, levelAfter, err := applyCharacterExp(ctx, tx, d.Progress, charID,
		LifeSkillExpForAct(characterAct(prog.Level)))
	if err != nil {
		return 0, 0, err
	}
	return exp, levelAfter, nil
}

// settleMilestones records newly crossed completion milestones and grants
// each milestone's glowing title.
func (d Deps) settleMilestones(ctx context.Context, tx pgx.Tx, charID id.UUID, now time.Time) ([]Milestone, error) {
	mastered, err := d.Store.MasteredCount(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	done, err := d.Store.MilestonesDone(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	var out []Milestone
	for _, m := range Milestones() {
		if mastered < m.Needed {
			continue
		}
		if _, ok := done[m.ID]; ok {
			continue
		}
		inserted, err := d.Store.RecordMilestone(ctx, tx, charID, m.ID, now)
		if err != nil {
			return nil, err
		}
		if !inserted {
			continue
		}
		opID := id.ServerJobOperationID("atlas.milestone", charID.String(), m.ID)
		if _, err := d.Store.GrantCosmetic(ctx, tx, charID, m.Title, m.ID, opID, now); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// AcknowledgeExecutor handles C2S_ATLAS_CLAIM (504) on the
// atlas.acknowledge family: it acknowledges only — writes acknowledged_at
// once per page, grants nothing, and emits S2C_ATLAS_CLAIM_RESULT (505)
// with the tier's settled grant view. Unreached tiers resolve
// ATLAS_TIER_NOT_REACHED with no state change.
func AcknowledgeExecutor(d Deps) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		if rec.GetOperationFamily() != AcknowledgeFamily {
			return idempotency.Outcome{}, fmt.Errorf("atlas: unsupported family %q", rec.GetOperationFamily())
		}
		req := rec.GetClient().GetC2SAtlasClaim()
		if req == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
			return idempotency.Outcome{}, fmt.Errorf("atlas: malformed acknowledge record")
		}
		var charID, opID id.UUID
		copy(charID[:], rec.GetOwnerId())
		copy(opID[:], rec.GetOperationId())

		page, ok := PageByID(req.GetAtlasPageId())
		if !ok {
			return claimVerdict(opID, req, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID, nil)
		}
		locks := []lockorder.Lock{
			lockorder.RowLock("character_atlas", charID),
			lockorder.RowLock("character_atlas_state", charID),
		}
		if err := lockorder.SortLocks(locks); err != nil {
			return idempotency.Outcome{}, err
		}
		if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
			return idempotency.Outcome{}, err
		}
		row, err := d.Store.Page(ctx, tx, charID, page.ID)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		tier := req.GetTier()
		if tier == 0 || tier > row.ReachedTier {
			return claimVerdict(opID, req,
				protocolv1.ErrorCode_ERROR_CODE_ATLAS_TIER_NOT_REACHED, nil)
		}
		changed, err := d.Store.Acknowledge(ctx, tx, charID, page.ID, d.Now())
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if changed {
			if _, err := d.Store.BumpRevision(ctx, tx, charID); err != nil {
				return idempotency.Outcome{}, err
			}
		}
		granted := &protocolv1.AtlasGrantView{}
		if amt := page.Special[tier-1]; amt > 0 {
			granted.CurrencyDelta = append(granted.CurrencyDelta,
				&protocolv1.CurrencyDelta{CurrencyId: string(currency.Special), Amount: amt})
		}
		return claimVerdict(opID, req, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, granted)
	}
}

// claimVerdict emits the 505 result outcome: SUCCESS on nil error code,
// ERROR otherwise; granted carries the settled bundle echo on success.
func claimVerdict(opID id.UUID, req *protocolv1.C2SAtlasClaim,
	code protocolv1.ErrorCode, granted *protocolv1.AtlasGrantView) (idempotency.Outcome, error) {
	status := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	result := &protocolv1.S2CAtlasClaimResult{
		Result:      operationResult(opID, status, code),
		AtlasPageId: req.GetAtlasPageId(),
		Tier:        req.GetTier(),
		Granted:     granted,
	}
	payload, err := marshalOutcome(outcomeResult(status, result))
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}
