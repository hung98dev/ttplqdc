package crafting

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// ItemDef is the runtime view of one item_id the executors need
// (mirrors cooking.ItemDef).
type ItemDef struct {
	Def items.Def
}

// Defs resolves an item_id to its runtime definition; nil is fail-closed.
type Defs func(ctx context.Context, itemID string) (ItemDef, error)

// Blessed reports whether the guild-blessing buff flag is active for
// the character at attempt time (crafting.md § Guild Blessing). nil
// means never blessed.
type Blessed func(ctx context.Context, characterID id.UUID) (bool, error)

// Locked reports whether an item instance is under a live transaction
// lock (trade/session). nil means never locked.
type Locked func(ctx context.Context, characterID, instanceID id.UUID) (bool, error)

// Deps are the constructor-time dependencies of the crafting executors.
type Deps struct {
	Items           *items.Store
	Defs            Defs
	Recipes         Recipes
	Equip           *equipment.Catalog
	Blessed         Blessed
	Locked          Locked
	ContentRevision *string
	Now             func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// PlanCraft resolves one C2S_CRAFT admission into the frozen
// JournalCraftSnapshot (crafting.md § Frozen Craft Snapshot): every
// consumed instance id/quantity, the signed currency delta, and every
// created item's UUID + rolls + binding + provenance + time are settled
// inside the read-only admission tx so the committed record replays
// verbatim and a retry never reselects inputs or rerolls outputs.
func (d Deps) PlanCraft(ctx context.Context, tx pgx.Tx, charID id.UUID,
	recipeID string, batch uint32, charLevel int32) (*journalv1.JournalCraftSnapshot, error) {
	if batch < 1 || batch > maxBatch {
		return nil, fmt.Errorf("%w: batch %d", ErrStateConflict, batch)
	}
	rec, err := d.Recipes(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	if charLevel < rec.MinimumLevel {
		return nil, fmt.Errorf("%w: recipe %s needs level %d", ErrLevelTooLow, recipeID, rec.MinimumLevel)
	}
	out, err := d.Defs(ctx, rec.OutputItemID)
	if err != nil {
		return nil, err
	}
	matQty := rec.InputQty * int64(batch)
	consumed, err := selectConsume(ctx, tx, charID, rec.InputItemID, matQty)
	if err != nil {
		return nil, err
	}
	commonCost := rec.CommonCost * int64(batch)
	if commonCost > 0 {
		ok, err := balanceCovers(ctx, tx, charID, commonCost)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInsufficientCurrency
		}
	}
	stackable := out.Def.Stackable && out.Def.MaxStack > 1
	var needSlots int64
	if stackable {
		need, err := stackableSlotsNeeded(ctx, tx, charID, rec.OutputItemID, int64(batch), int64(out.Def.MaxStack))
		if err != nil {
			return nil, err
		}
		needSlots = need
	} else {
		needSlots = int64(batch)
	}
	if err := d.checkCapacity(ctx, tx, charID, needSlots); err != nil {
		return nil, err
	}

	snap := &journalv1.JournalCraftSnapshot{
		RecipeId:      recipeID,
		BatchQuantity: batch,
		Consumed:      consumed,
	}
	if commonCost != 0 {
		snap.CurrencyDelta = append(snap.CurrencyDelta, &journalv1.JournalCurrency{
			CurrencyId: string(currency.Common),
			Amount:     -commonCost,
		})
	}
	nowMs := d.now().UnixMilli()
	binding := out.Def.DefaultBinding.String()
	rev := ""
	if d.ContentRevision != nil {
		rev = *d.ContentRevision
	}
	if stackable {
		instID := id.NewV4()
		snap.CreatedItems = append(snap.CreatedItems, &journalv1.JournalItem{
			ItemId:           rec.OutputItemID,
			Quantity:         uint64(batch),
			EffectiveBinding: binding,
			ContentRevision:  rev,
			ItemInstanceId:   instID[:],
			CreatedAtMs:      &nowMs,
		})
	} else {
		eqDef := d.equipDef(rec.OutputItemID)
		for i := uint32(0); i < batch; i++ {
			instID := id.NewV4()
			it := &journalv1.JournalItem{
				ItemId:           rec.OutputItemID,
				Quantity:         1,
				EffectiveBinding: binding,
				ContentRevision:  rev,
				Enhancement:      0,
				ItemInstanceId:   instID[:],
				CreatedAtMs:      &nowMs,
			}
			if eqDef != nil {
				it.BaseRolls, it.SecondaryRolls = rollStats(d.Equip, eqDef, instID)
			}
			snap.CreatedItems = append(snap.CreatedItems, it)
		}
	}
	return snap, nil
}

// PlanEnhance resolves one C2S_ENHANCE admission into the frozen
// JournalEnhanceResult stored on the client command's `rng_outputs`
// (protobuf_conventions.md §7): validated inputs/charm ids, computed
// final rate, the keyed roll outcome, applied pity count, consumed
// items and signed currency — replayed verbatim at settlement.
func (d Deps) PlanEnhance(ctx context.Context, tx pgx.Tx, charID id.UUID,
	req *enhanceReq) (*journalv1.JournalEnhanceResult, error) {
	inst, err := d.loadEquipment(ctx, tx, charID, req.ItemInstanceID)
	if err != nil {
		return nil, err
	}
	cur := inst.Enhancement
	target := int64(req.TargetLevel)
	if target != cur+1 || cur >= maxEnhanceLevel {
		return nil, fmt.Errorf("%w: target +%d from +%d", ErrStateConflict, target, cur)
	}
	if d.Locked != nil {
		locked, err := d.Locked(ctx, charID, req.ItemInstanceID)
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, ErrItemLocked
		}
	}
	eqDef := d.equipDef(inst.ItemID)
	if eqDef == nil {
		return nil, fmt.Errorf("%w: %s not enhanceable", ErrStateConflict, inst.ItemID)
	}
	base := d.Equip.EnhBase[eqDef.Tier]
	if base == nil {
		return nil, fmt.Errorf("%w: tier %s no enhancement base", ErrStateConflict, eqDef.Tier)
	}
	row := tierTable[eqDef.Tier]

	var lucky, insurance *charmSel
	if req.LuckyCharmID != (id.UUID{}) {
		lucky, err = loadCharm(ctx, tx, charID, req.LuckyCharmID, charmKindLucky, cur)
		if err != nil {
			return nil, err
		}
	}
	if req.InsuranceID != (id.UUID{}) {
		insurance, err = loadCharm(ctx, tx, charID, req.InsuranceID, charmKindInsure, cur)
		if err != nil {
			return nil, err
		}
		if lucky != nil && lucky.InstanceID == insurance.InstanceID {
			return nil, ErrCharmIneligible
		}
	}

	matUnits, commonCost := attemptCosts(base, cur)
	if matQty := matUnits; matQty > 0 {
		ok, err := itemsCover(ctx, tx, charID, row.MaterialID, matQty)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrInsufficientItems, row.MaterialID)
		}
	}
	if commonCost > 0 {
		ok, err := balanceCovers(ctx, tx, charID, commonCost)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInsufficientCurrency
		}
	}

	var fails int64
	if target >= pityStartTarget {
		fails, err = pityRead(ctx, tx, req.ItemInstanceID, target)
		if err != nil {
			return nil, err
		}
	}
	blessed := false
	if d.Blessed != nil {
		blessed, err = d.Blessed(ctx, charID)
		if err != nil {
			return nil, err
		}
	}
	rate := finalRateBP(cur, blessed, lucky, fails)
	roll := int64(streamFor(enhanceRollKey(req.ItemInstanceID.String(), target, req.OperationID.String())).Uint64() % 10000)
	// NOTE: roll key binds instance, target and the durable operation id —
	// a retry of the same record replays this outcome verbatim.
	success, after := resolveAttempt(cur, roll, rate, insurance != nil)
	var postFails uint32
	if target >= pityStartTarget {
		if success {
			postFails = 0
		} else {
			postFails = uint32(min(fails+1, pityMaxFails))
		}
	}

	res := &journalv1.JournalEnhanceResult{
		Success:       success,
		LevelBefore:   uint32(cur),
		LevelAfter:    uint32(after),
		FinalRateBp:   uint32(rate),
		PityFailCount: postFails,
	}
	if matUnits > 0 {
		res.Consumed = append(res.Consumed, itemQty(row.MaterialID, matUnits))
	}
	if lucky != nil {
		res.Consumed = append(res.Consumed, itemQty(lucky.ItemID, 1))
	}
	if insurance != nil {
		res.Consumed = append(res.Consumed, itemQty(insurance.ItemID, 1))
	}
	if commonCost != 0 {
		res.CurrencyDelta = append(res.CurrencyDelta, &journalv1.JournalCurrency{
			CurrencyId: string(currency.Common),
			Amount:     -commonCost,
		})
	}
	return res, nil
}

// enhanceReq is the validated enhancement admission input (parsed from
// C2S_ENHANCE by the sim admission seam / record builder).
type enhanceReq struct {
	OperationID    id.UUID
	ItemInstanceID id.UUID
	TargetLevel    uint32
	LuckyCharmID   id.UUID
	InsuranceID    id.UUID
}

// ownedInst is one loaded owned item instance.
type ownedInst struct {
	ItemID      string
	Enhancement int64
	Equipped    bool
}

// loadEquipment loads one owned equipment instance for enhancement —
// crafting.md § Equipped Enhancement permits inventory OR equipped
// items (never crafting-consumed while equipped).
func (d Deps) loadEquipment(ctx context.Context, tx pgx.Tx, charID, instID id.UUID) (*ownedInst, error) {
	var itemID string
	var enh int64
	var owner id.UUID
	var kind string
	err := tx.QueryRow(ctx, `
		SELECT ii.item_id, ii.enhancement_level, il.character_id, il.location_kind
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE ii.item_instance_id = $1
		  AND il.location_kind IN ('CHARACTER_INVENTORY', 'EQUIPPED')`,
		instID).Scan(&itemID, &enh, &owner, &kind)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, instID)
	}
	if owner != charID {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, instID)
	}
	return &ownedInst{ItemID: itemID, Enhancement: enh, Equipped: kind == "EQUIPPED"}, nil
}

func (d Deps) equipDef(itemID string) *equipment.ItemDef {
	if d.Equip == nil {
		return nil
	}
	return d.Equip.Items[itemID]
}

// checkCapacity asserts the free-slot pre-check for needSlots.
func (d Deps) checkCapacity(ctx context.Context, tx pgx.Tx, charID id.UUID, needSlots int64) error {
	if needSlots <= 0 {
		return nil
	}
	var capacity, used int64
	err := tx.QueryRow(ctx, `
		SELECT ci.capacity, COALESCE(cnt.n, 0)
		FROM character_inventories ci
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS n FROM item_locations il
			WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		) cnt ON true
		WHERE ci.character_id = $1`, charID).Scan(&capacity, &used)
	if err != nil {
		return fmt.Errorf("%w: capacity %s", ErrNotFound, charID)
	}
	if capacity-used < needSlots {
		return ErrInventoryFull
	}
	return nil
}

// balanceCovers reads the common-currency balance (no lock — the plan
// read; the debit at commit revalidates).
func balanceCovers(ctx context.Context, tx pgx.Tx, charID id.UUID, amount int64) (bool, error) {
	var bal int64
	err := tx.QueryRow(ctx, `
		SELECT balance FROM character_currencies
		WHERE character_id = $1 AND currency_id = $2`, charID, string(currency.Common)).Scan(&bal)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bal >= amount, nil
}

// itemsCover asserts the owned stackable quantity of itemID is ≥ qty.
func itemsCover(ctx context.Context, tx pgx.Tx, charID id.UUID, itemID string, qty int64) (bool, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(ii.quantity), 0)
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2`, charID, itemID).Scan(&total)
	if err != nil {
		return false, err
	}
	return total >= qty, nil
}

// stackableSlotsNeeded returns the new free slots a stackable grant of
// qty needs: 0 when an existing stack has room, else the ceil stacks
// beyond current room.
func stackableSlotsNeeded(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string, qty, maxStack int64) (int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT ii.quantity FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2`, charID, itemID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var room int64
	for rows.Next() {
		var q int64
		if err := rows.Scan(&q); err != nil {
			return 0, err
		}
		if maxStack > q {
			room += maxStack - q
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if room >= qty {
		return 0, nil
	}
	rem := qty - room
	return (rem + maxStack - 1) / maxStack, nil
}

// selectConsume freezes the lowest-slot consume selection across owned
// CHARACTER_INVENTORY stacks of itemID (mirrors cooking.selectConsume):
// deterministic order by slot; every picked instance id + quantity is
// recorded so the commit revalidates the exact rows.
func selectConsume(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string, qty int64) ([]*journalv1.JournalConsumedItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT ii.item_instance_id, ii.quantity
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2
		ORDER BY il.slot`, charID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*journalv1.JournalConsumedItem
	var need = qty
	for rows.Next() && need > 0 {
		var instID id.UUID
		var have int64
		if err := rows.Scan(&instID, &have); err != nil {
			return nil, err
		}
		take := min(have, need)
		out = append(out, &journalv1.JournalConsumedItem{
			ItemInstanceId: instID[:],
			ItemId:         itemID,
			Quantity:       uint64(take),
		})
		need -= take
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if need > 0 {
		return nil, fmt.Errorf("%w: %s", ErrInsufficientItems, itemID)
	}
	return out, nil
}

// rollStats freezes one equipment instance's persistent rolls: base_rolls
// carry the fixed stats verbatim; secondary_rolls are the authored pool
// picks without replacement on the instance-keyed PCG-64 stream.
func rollStats(cat *equipment.Catalog, def *equipment.ItemDef, instID id.UUID) (base, secondary []*journalv1.JournalStat) {
	for _, st := range def.FixedStats {
		if st.Flat != 0 {
			base = append(base, &journalv1.JournalStat{StatId: st.Stat, Value: st.Flat, Scale: 1})
		} else {
			base = append(base, &journalv1.JournalStat{
				StatId: st.Stat, Value: st.Fraction.Num, Scale: uint32(st.Fraction.Den),
			})
		}
	}
	return base, secondaryRolls(cat, def, instID)
}

// ratAt scales a rational bound to an integer floor: floor(r * unit).
func ratAt(r config.Rat, unit int64) int64 {
	v := new(big.Rat).Mul(new(big.Rat).SetFrac64(r.Num, r.Den), big.NewRat(unit, 1))
	return new(big.Int).Quo(v.Num(), v.Denom()).Int64()
}
