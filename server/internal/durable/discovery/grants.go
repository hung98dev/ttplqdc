package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Family is the registry family for first-discovery settlements
// (save_rules.md § REWARD closed set: sim.discovery_settlement,
// OwnerCharacter).
const Family = "sim.discovery_settlement"

// RewardKind is the JournalRewardCommand.kind the settlement consumes.
const RewardKind = "discovery"

// Deps wires the discovery executors at composition. Catalog surfaces
// arrive injected: the packet's sim-side catalogs own the authored
// values; durable code never reaches into sim packages.
type Deps struct {
	Store    *Store
	Progress *progression.Store
	// ExpFor resolves a normal-world map_id to its authored
	// first_discovery_exp (world_route_catalog.md § Map Metadata).
	ExpFor func(mapID string) (uint64, bool)
	// AnchorFor resolves a travel-eligible safe-anchor map_id to its
	// entry spawn anchor. Missing = not travel-eligible.
	AnchorFor func(destMapID string) (spawnAnchorID string, ok bool)
	// FeeFor resolves the travel common-currency fee for a destination
	// anchor map (npc_shop_catalog.md § Travel tiers).
	FeeFor func(destMapID string) (int64, bool)
	Now    func() time.Time
}

// RewardExecutor settles sim.discovery_settlement records: the first
// authoritative entry grants the authored EXP exactly once per
// reward.discovery.<map_id>.<character_id>; replays and reconnect
// restores commit SUCCESS with no grant — the reward never re-grants.
func RewardExecutor(d Deps) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		if rec.GetOperationFamily() != Family {
			return idempotency.Outcome{}, fmt.Errorf("discovery: unsupported family %q", rec.GetOperationFamily())
		}
		rwd := rec.GetReward()
		if rwd == nil || len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
			return idempotency.Outcome{}, fmt.Errorf("discovery: malformed reward record")
		}
		var charID, opID id.UUID
		copy(charID[:], rec.GetOwnerId())
		copy(opID[:], rec.GetOperationId())

		mapID := rwd.GetDiscoveryId()
		exp, ok := uint64(0), false
		if rwd.GetKind() == RewardKind && mapID != "" && d.ExpFor != nil {
			exp, ok = d.ExpFor(mapID)
		}
		if !ok {
			return grantVerdict(opID, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		}

		if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
			return idempotency.Outcome{}, err
		}
		first, err := d.Store.Record(ctx, tx, charID, KindMap, mapID, opID)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if !first {
			// Already discovered — reconnect/restore/replay never re-grants.
			return grantOutcome(opID, nil)
		}

		p, err := d.Progress.Read(ctx, tx, charID)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		grant := int64(exp)
		if p.Level >= 60 {
			grant = 0 // discovery EXP is ignored at Level 60
		}
		newExp := int64(p.CurrentExp) + grant
		if newExp > maxExp {
			newExp = maxExp
		}
		newLevel := levelFor(newExp)
		gained := newLevel - p.Level
		if grant > 0 || gained > 0 {
			if err := d.Progress.Write(ctx, tx, charID, newLevel,
				int32(newExp), p.UnspentSkillPoints+gained,
				p.UnspentPotentialPoints+4*gained); err != nil {
				return idempotency.Outcome{}, err
			}
		}
		slot := &journalv1.JournalRewardSlot{
			RewardSlot:   RewardKey(mapID, charID),
			CharacterExp: uint64(grant),
		}
		return grantOutcome(opID, []*journalv1.JournalRewardSlot{slot})
	}
}

// RewardKey is the once-only grant identity
// reward.discovery.<map_id>.<character_id> (world_route_catalog.md
// § Map Metadata and First Discovery).
func RewardKey(mapID string, characterID id.UUID) string {
	return "reward.discovery." + mapID + "." + characterID.String()
}
