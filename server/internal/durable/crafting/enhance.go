package crafting

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/config/equipment"
	"thinhthan/internal/core/id"
)

// pityRead returns the stored consecutive-failure count for one
// `item_instance_id + target_level` pity record (0 absent).
func pityRead(ctx context.Context, tx pgx.Tx, instID id.UUID, target int64) (int64, error) {
	var n int64
	err := tx.QueryRow(ctx, `
		SELECT pity_fail_count FROM enhancement_pity
		WHERE item_instance_id = $1 AND target_level = $2
		FOR UPDATE`, instID, target).Scan(&n)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

// pityWrite upserts the post-attempt counter: success resets the record
// (delete), failure stores the frozen count.
func pityWrite(ctx context.Context, tx pgx.Tx, instID id.UUID, target, count int64) error {
	if target < pityStartTarget {
		return nil
	}
	if count <= 0 {
		_, err := tx.Exec(ctx, `
			DELETE FROM enhancement_pity
			WHERE item_instance_id = $1 AND target_level = $2`, instID, target)
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO enhancement_pity (item_instance_id, target_level, pity_fail_count, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (item_instance_id, target_level)
		DO UPDATE SET pity_fail_count = $3, updated_at = now()`, instID, target, count)
	return err
}

// pityBonusBP derives the +bp pity bonus for a stored fail count
// (+1% per fail from the 6th consecutive failure, cap +5% at 9).
func pityBonusBP(fails int64) int64 {
	if fails < pityRampAfter {
		return 0
	}
	bp := (fails - pityRampAfter + 1) * pityPerFailBP
	if bp > pityCapBP {
		return pityCapBP
	}
	return bp
}

// charmSel is one validated charm choice frozen at admission.
type charmSel struct {
	InstanceID id.UUID
	ItemID     string
	Grade      string
}

// loadCharm validates one optional charm instance id: owned, correct
// kind, eligible at the current level. Ineligible/stacked rejects
// CHARM_INELIGIBLE pre-consume (ADR-0022).
func loadCharm(ctx context.Context, tx pgx.Tx, charID, instID id.UUID,
	kind string, curLevel int64) (*charmSel, error) {
	var itemID string
	var owner id.UUID
	err := tx.QueryRow(ctx, `
		SELECT ii.item_id, il.character_id
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE ii.item_instance_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'`,
		instID).Scan(&itemID, &owner)
	if err != nil {
		return nil, fmt.Errorf("%w: charm %s", ErrCharmIneligible, instID)
	}
	if owner != charID {
		return nil, fmt.Errorf("%w: charm %s", ErrCharmIneligible, instID)
	}
	grade, ok := charmGrade(itemID, kind)
	if !ok {
		return nil, fmt.Errorf("%w: charm %s", ErrCharmIneligible, instID)
	}
	var maxCur int64
	if kind == charmKindLucky {
		var bp int64
		bp, maxCur, ok = luckyBonusBP(grade)
		_ = bp
	} else {
		maxCur, ok = insuranceEligibility(grade)
	}
	if !ok || curLevel >= maxCur {
		return nil, fmt.Errorf("%w: charm %s grade %s at +%d", ErrCharmIneligible, instID, grade, curLevel)
	}
	return &charmSel{InstanceID: instID, ItemID: itemID, Grade: grade}, nil
}

// finalRateBP applies the canonical clamp order:
// min(base + blessing + charm + pity, 9500) — pity only for +13..+16.
func finalRateBP(cur int64, blessed bool, lucky *charmSel, pityFails int64) int64 {
	rate := baseRateBP[cur]
	if blessed {
		rate += blessingBonusBP
	}
	if lucky != nil {
		if bp, _, ok := luckyBonusBP(lucky.Grade); ok {
			rate += bp
		}
	}
	if cur+1 >= pityStartTarget {
		rate += pityBonusBP(pityFails)
	}
	if rate > rateClampBP {
		return rateClampBP
	}
	return rate
}

// resolveAttempt maps one frozen roll to the outcome: success +1,
// failure max(cur-1, floor) or insured preserve (crafting.md § Failure).
func resolveAttempt(cur, roll, rateBP int64, insured bool) (success bool, after int64) {
	if roll < rateBP {
		return true, cur + 1
	}
	if insured {
		return false, cur
	}
	return false, max(cur-1, floorOf(cur))
}

// attemptCosts returns the material units and common cost for one
// attempt at current level on a tier's bases.
func attemptCosts(base *equipment.EnhancementBase, cur int64) (matUnits, common int64) {
	return base.MaterialUnits * materialMultiplier[cur],
		base.CommonCurrency * commonMultiplier[cur]
}
