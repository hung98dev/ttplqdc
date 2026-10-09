package crafting

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// planCraft runs PlanCraft inside one rolled-back tx (read-only).
func planCraft(t *testing.T, d Deps, charID id.UUID, recipeID string,
	batch uint32, level int32) (*journalv1.JournalCraftSnapshot, error) {
	t.Helper()
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	return d.PlanCraft(ctx, txx, charID, recipeID, batch, level)
}

// planEnhance runs PlanEnhance inside one rolled-back tx.
func planEnhance(t *testing.T, d Deps, charID id.UUID,
	req enhanceReq) (*journalv1.JournalEnhanceResult, error) {
	t.Helper()
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	return d.PlanEnhance(ctx, txx, charID, &req)
}

// execRecord runs one executor inside a committed tx and returns the
// outcome.
func execRecord(t *testing.T,
	ex func(context.Context, pgx.Tx, *journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	var oc idempotency.Outcome
	var err error
	tx(t, func(ctx context.Context, txx pgx.Tx) {
		oc, err = ex(ctx, txx, rec)
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	return outcomeOf(t, oc)
}

// rollFor mirrors the admission-time deterministic roll for an op —
// tests pick operation ids whose roll forces success or failure.
func rollFor(itemID id.UUID, target int64, opID id.UUID) int64 {
	return int64(streamFor(enhanceRollKey(itemID.String(), target, opID.String())).Uint64() % 10000)
}

// pickOp returns a V7 op id whose keyed roll satisfies want (true =
// success roll below rate, false = failure roll at/above rate).
func pickOp(t *testing.T, itemID id.UUID, target int64, rateBP int64, want bool) id.UUID {
	t.Helper()
	for i := 0; i < 100000; i++ {
		op := id.NewV7(time.Now().Add(time.Duration(i) * time.Millisecond))
		r := rollFor(itemID, target, op)
		if (r < rateBP) == want {
			return op
		}
	}
	t.Fatalf("no op id for roll want=%v rate=%d", want, rateBP)
	return id.UUID{}
}

func enhanceReqFor(t *testing.T, opID, itemID id.UUID, target uint32,
	lucky, insurance id.UUID) enhanceReq {
	t.Helper()
	return enhanceReq{
		OperationID:    opID,
		ItemInstanceID: itemID,
		TargetLevel:    target,
		LuckyCharmID:   lucky,
		InsuranceID:    insurance,
	}
}

// TestFrozenCraftSnapshotRetainsRollsIdsMaterialsAndCosts asserts the
// admission freeze keeps recipe/batch, exact material instance ids +
// quantities, the signed currency delta, and every created item's
// UUID + full stat rolls + binding + revision + created time.
func TestFrozenCraftSnapshotRetainsRollsIdsMaterialsAndCosts(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t6"].MaterialID
	stk1 := mkItem(t, char, mat, 60, 0)
	stk2 := mkItem(t, char, mat, 80, 1)
	credit(t, char, 1_000_000)

	recipeID := "recipe.eq.t6.nui_thieng.weapon" // weight 5 -> 50 mat / 17500 common each
	snap, err := planCraft(t, d, char, recipeID, 2, 51)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if snap.GetRecipeId() != recipeID || snap.GetBatchQuantity() != 2 {
		t.Fatalf("snapshot recipe/batch: %v %v", snap.GetRecipeId(), snap.GetBatchQuantity())
	}
	// 2 x weapon = 100 material: frozen lowest-slot-first 60 + 40.
	if len(snap.GetConsumed()) != 2 {
		t.Fatalf("consumed rows: %v", snap.GetConsumed())
	}
	var gotID [16]byte
	copy(gotID[:], snap.GetConsumed()[0].GetItemInstanceId())
	if id.UUID(gotID) != stk1 || snap.GetConsumed()[0].GetQuantity() != 60 {
		t.Fatalf("frozen row0: %+v", snap.GetConsumed()[0])
	}
	copy(gotID[:], snap.GetConsumed()[1].GetItemInstanceId())
	if id.UUID(gotID) != stk2 || snap.GetConsumed()[1].GetQuantity() != 40 {
		t.Fatalf("frozen row1: %+v", snap.GetConsumed()[1])
	}
	if len(snap.GetCurrencyDelta()) != 1 ||
		snap.GetCurrencyDelta()[0].GetCurrencyId() != "currency.common" ||
		snap.GetCurrencyDelta()[0].GetAmount() != -35000 {
		t.Fatalf("currency delta: %v", snap.GetCurrencyDelta())
	}
	if len(snap.GetCreatedItems()) != 2 {
		t.Fatalf("created: %v", snap.GetCreatedItems())
	}
	seen := map[string]bool{}
	for _, it := range snap.GetCreatedItems() {
		if len(it.GetItemInstanceId()) != 16 || seen[string(it.GetItemInstanceId())] {
			t.Fatalf("created item id missing/dup: %v", it.GetItemInstanceId())
		}
		seen[string(it.GetItemInstanceId())] = true
		if it.GetItemId() != "item.eq.t6.nui_thieng.weapon" || it.GetQuantity() != 1 {
			t.Fatalf("created item shape: %+v", it)
		}
		if it.GetEffectiveBinding() != "UNBOUND" || it.GetEnhancement() != 0 {
			t.Fatalf("created binding/enh: %v +%d", it.GetEffectiveBinding(), it.GetEnhancement())
		}
		if it.GetContentRevision() != *d.ContentRevision {
			t.Fatalf("created revision: %q", it.GetContentRevision())
		}
		if it.CreatedAtMs == nil || it.GetCreatedAtMs() == 0 {
			t.Fatalf("created time missing")
		}
		// FixedStats ATTACK(flat) + CRIT_CHANCE(fraction); 2 secondary.
		if len(it.GetBaseRolls()) != 2 || len(it.GetSecondaryRolls()) != 2 {
			t.Fatalf("rolls: base=%v sec=%v", it.GetBaseRolls(), it.GetSecondaryRolls())
		}
	}
}

// TestStaleCraftSnapshotConsumesAndCreatesNothing: a frozen selection
// that went stale at commit rejects atomically — zero consumption,
// zero creation.
func TestStaleCraftSnapshotConsumesAndCreatesNothing(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	stk := mkItem(t, char, mat, 30, 0)
	credit(t, char, 100_000)
	snap, err := planCraft(t, d, char, "recipe.eq.t1.dinh_lang.body", 1, 10)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// Go stale: shrink the frozen stack below the frozen take.
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE item_instances SET quantity = 5 WHERE item_instance_id = $1`,
		stk.String()); err != nil {
		t.Fatalf("stale: %v", err)
	}
	rec := craftRecord(t, char, newOp(), "recipe.eq.t1.dinh_lang.body", 1, snap)
	out := execRecord(t, d.craftExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR {
		t.Fatalf("stale outcome: %+v", out)
	}
	if invCount(t, char) != 1 || qty(t, char, mat) != 5 {
		t.Fatalf("stale consumed something: inv=%d qty=%d", invCount(t, char), qty(t, char, mat))
	}
	if n := balance(t, char); n != 100_000 {
		t.Fatalf("stale debited: %d", n)
	}
}

// TestCraftCreatesInventoryNotSyntheticRewardClaim: a settled craft
// creates real inventory items and never writes a CRAFT-sourced
// Reward Claim.
func TestCraftCreatesInventoryNotSyntheticRewardClaim(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	mkItem(t, char, mat, 100, 0)
	credit(t, char, 100_000)
	snap, err := planCraft(t, d, char, "recipe.eq.t1.ben_da.head", 2, 10)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	rec := craftRecord(t, char, newOp(), "recipe.eq.t1.ben_da.head", 2, snap)
	out := execRecord(t, d.craftExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("craft outcome: %+v", out)
	}
	s2c := out.GetS2CCraftResult()
	if s2c == nil || len(s2c.GetGranted()) != 2 {
		t.Fatalf("405 granted: %+v", out.GetClientResult())
	}
	// Two real equipment instances landed in inventory at +0.
	if n := invCount(t, char); n != 3 {
		t.Fatalf("inv count after craft: %d", n)
	}
	var eqCount, eqZero int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*), COALESCE(SUM((enhancement_level=0)::int),0)
		 FROM item_instances ii JOIN item_locations il USING (item_instance_id)
		 WHERE il.character_id=$1 AND ii.item_id='item.eq.t1.ben_da.head'`,
		char.String()).Scan(&eqCount, &eqZero); err != nil {
		t.Fatalf("equip count: %v", err)
	}
	if eqCount != 2 || eqZero != 2 {
		t.Fatalf("equip rows: %d zero-enh %d", eqCount, eqZero)
	}
	// No reward claim was synthesized for the craft.
	var claims int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reward_claims WHERE owner_character_id=$1`,
		char.String()).Scan(&claims); err != nil {
		t.Fatalf("claims query: %v", err)
	}
	if claims != 0 {
		t.Fatalf("synthetic claims: %d", claims)
	}
}

// TestOneHundredSixtyEightRecipes asserts the compiled expansion
// resolves every recipe.eq.<tier>.<set_key>.<slot> plus the 12 utility
// recipes, with canonical tier materials and slot-weighted costs.
func TestOneHundredSixtyEightRecipes(t *testing.T) {
	ctx := context.Background()
	resolve := RecipesFromCatalog(testCatalog())
	cat := testCatalog()
	seen := 0
	for _, itemID := range cat.ItemIDs {
		it := cat.Items[itemID]
		rid := "recipe.eq." + it.Tier + "." + it.SetKey + "." + it.Slot
		rec, err := resolve(ctx, rid)
		if err != nil {
			t.Fatalf("resolve %s: %v", rid, err)
		}
		row := tierTable[it.Tier]
		if rec.OutputItemID != itemID || rec.InputItemID != row.MaterialID {
			t.Fatalf("%s recipe: %+v", rid, rec)
		}
		if rec.InputQty != row.Material*slotWeights[it.Slot] ||
			rec.CommonCost != row.Common*slotWeights[it.Slot] ||
			rec.MinimumLevel != row.MinLevel {
			t.Fatalf("%s costs: %+v", rid, rec)
		}
		seen++
	}
	if seen != EquipmentRecipeCount {
		t.Fatalf("equipment recipes: %d != %d", seen, EquipmentRecipeCount)
	}
	// 6 lucky + 6 insurance utility recipes.
	for i := 1; i <= 6; i++ {
		for _, kind := range []string{"bua_may", "bua_giu_bac"} {
			rid := "recipe.utility." + kind + ".t" + string(rune('0'+i))
			if _, err := resolve(ctx, rid); err != nil {
				t.Fatalf("utility %s: %v", rid, err)
			}
		}
	}
	if _, err := resolve(ctx, "recipe.eq.t0.nope.weapon"); err == nil {
		t.Fatalf("unknown recipe resolved")
	}
}

// TestGuaranteedCraftingSettlement: end-to-end all-or-nothing commit —
// exact frozen consume, signed debit, inventory creation, 405 fields;
// plus the canonical expected-cost reference vectors.
func TestGuaranteedCraftingSettlement(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	mkItem(t, char, mat, 100, 0)
	credit(t, char, 100_000)
	snap, err := planCraft(t, d, char, "recipe.eq.t1.dinh_lang.weapon", 1, 10)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	rec := craftRecord(t, char, newOp(), "recipe.eq.t1.dinh_lang.weapon", 1, snap)
	out := execRecord(t, d.craftExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("settle outcome: %+v", out)
	}
	if out.GetCraft() == nil || out.GetCraft().GetRecipeId() != "recipe.eq.t1.dinh_lang.weapon" {
		t.Fatalf("outcome craft: %+v", out.GetCraft())
	}
	if qty(t, char, mat) != 100-15 { // weapon weight 5 x base 3
		t.Fatalf("material left: %d", qty(t, char, mat))
	}
	if n := balance(t, char); n != 100_000-500 { // weight 5 x base 100
		t.Fatalf("balance left: %d", n)
	}
	s2c := out.GetS2CCraftResult()
	if len(s2c.GetConsumed()) != 1 || s2c.GetConsumed()[0].GetQuantity() != 15 {
		t.Fatalf("405 consumed: %+v", s2c.GetConsumed())
	}
	if len(s2c.GetCurrencyDelta()) != 1 || s2c.GetCurrencyDelta()[0].GetAmount() != -500 {
		t.Fatalf("405 currency: %+v", s2c.GetCurrencyDelta())
	}
	if len(s2c.GetGranted()) != 1 || len(s2c.GetGranted()[0].GetItemInstanceId()) != 16 {
		t.Fatalf("405 granted: %+v", s2c.GetGranted())
	}
	if len(out.GetCreatedIds()) != 1 {
		t.Fatalf("outcome created ids: %v", out.GetCreatedIds())
	}

	// Expected-cost reference vectors (crafting.md § Expected-Cost
	// Reference) — exact rational arithmetic, round once half-up.
	type vec struct {
		target   int
		material string
		common   string
	}
	for _, v := range []vec{
		{6, "27.45", "56.53"},
		{8, "192.21", "488.79"},
		{10, "275.54", "798.79"},
		{11, "375.54", "1198.79"},
		{12, "525.54", "1886.29"},
		{13, "701.43", "2765.73"},
		{16, "785782.04", "3979515.40"},
	} {
		m := RoundUnits(ExpectedMaterialUnits(v.target))
		c := RoundUnits(ExpectedCommonUnits(v.target))
		if m.RatString() != ratString(v.material) || c.RatString() != ratString(v.common) {
			t.Fatalf("+%d vectors: mat=%s want %s, common=%s want %s",
				v.target, m.RatString(), v.material, c.RatString(), v.common)
		}
	}
	// T6 +16 terminal destination: 3,979,515.40... x 250 -> 994,878,850.
	t6 := ExpectedCommonUnits(16)
	t6.Mul(t6, big.NewRat(250, 1))
	rounded := new(big.Rat).Add(t6, big.NewRat(1, 2))
	whole := new(big.Int).Quo(rounded.Num(), rounded.Denom())
	if whole.String() != "994878850" {
		t.Fatalf("T6 +16 common: %s", whole.String())
	}
}

func ratString(s string) string {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic(s)
	}
	return r.RatString()
}

// TestEnhancementPlusZeroToSixteen: success gains exactly +1 at every
// level; failure without insurance drops to max(cur-1, floor);
// insurance preserves; pity accrues only at +13..+16 targets.
func TestEnhancementPlusZeroToSixteen(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	mkItem(t, char, mat, 999_999, 0)
	credit(t, char, 1_000_000_000)
	itemID := "item.eq.t1.dinh_lang.weapon"

	// +0 -> +16 path: force success at every level.
	eq := mkEquip(t, char, itemID, 0, 1)
	for cur := int64(0); cur < 16; cur++ {
		target := cur + 1
		op := pickOp(t, eq, target, baseRateBP[cur]+pityBonusBP(pityOf(t, eq, target)), true)
		req := enhanceReqFor(t, op, eq, uint32(target), id.UUID{}, id.UUID{})
		frozen, err := planEnhance(t, d, char, req)
		if err != nil {
			t.Fatalf("plan +%d->+%d: %v", cur, target, err)
		}
		if !frozen.GetSuccess() || frozen.GetLevelBefore() != uint32(cur) ||
			frozen.GetLevelAfter() != uint32(target) {
			t.Fatalf("frozen +%d: %+v", cur, frozen)
		}
		out := execRecord(t, d.enhanceExec,
			enhanceRecord(t, char, op, eq, uint32(target), id.UUID{}, id.UUID{}, frozen))
		if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
			!out.GetS2CEnhanceResult().GetSuccess() {
			t.Fatalf("enhance +%d outcome: %+v", cur, out)
		}
		if got := enhOf(t, eq); got != int16(target) {
			t.Fatalf("level after +%d->+%d: %d", cur, target, got)
		}
	}
	if got := enhOf(t, eq); got != 16 {
		t.Fatalf("final level: +%d", got)
	}

	// Failure paths: uninsured drop clamps at the milestone floor;
	// insured preserves the level.
	for cur, floor := range map[int64]int64{3: 0, 5: 4, 9: 8, 13: 12} {
		it := mkEquip(t, char, itemID, int16(cur), uint32(cur+2))
		op := pickOp(t, it, cur+1, baseRateBP[cur], false)
		req := enhanceReqFor(t, op, it, uint32(cur+1), id.UUID{}, id.UUID{})
		frozen, err := planEnhance(t, d, char, req)
		if err != nil {
			t.Fatalf("plan fail +%d: %v", cur, err)
		}
		wantAfter := cur - 1
		if wantAfter < floor {
			wantAfter = floor
		}
		if frozen.GetSuccess() || frozen.GetLevelAfter() != uint32(wantAfter) {
			t.Fatalf("fail +%d frozen: %+v want after %d", cur, frozen, wantAfter)
		}
		out := execRecord(t, d.enhanceExec,
			enhanceRecord(t, char, op, it, uint32(cur+1), id.UUID{}, id.UUID{}, frozen))
		if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
			out.GetS2CEnhanceResult().GetSuccess() {
			t.Fatalf("fail +%d outcome: %+v", cur, out)
		}
		if got := enhOf(t, it); got != int16(wantAfter) {
			t.Fatalf("fail +%d level: %d want %d", cur, got, wantAfter)
		}
	}
	// Insurance preserves the current level on failure.
	{
		it := mkEquip(t, char, itemID, 9, 20)
		ins := mkItem(t, char, charmKindInsure+"."+luckyGradeMid, 5, 21)
		op := pickOp(t, it, 10, baseRateBP[9], false)
		req := enhanceReqFor(t, op, it, 10, id.UUID{}, ins)
		frozen, err := planEnhance(t, d, char, req)
		if err != nil {
			t.Fatalf("plan insured: %v", err)
		}
		if frozen.GetSuccess() || frozen.GetLevelAfter() != 9 {
			t.Fatalf("insured frozen: %+v", frozen)
		}
		out := execRecord(t, d.enhanceExec,
			enhanceRecord(t, char, op, it, 10, id.UUID{}, ins, frozen))
		if got := enhOf(t, it); got != 9 {
			t.Fatalf("insured level: %d (out %+v)", got, out)
		}
		if qty(t, char, charmKindInsure+"."+luckyGradeMid) != 4 {
			t.Fatalf("insurance not consumed: %d",
				qty(t, char, charmKindInsure+"."+luckyGradeMid))
		}
	}
	// Pity: failures at +13 accrue; rate gains +100bp per fail >=5.
	{
		it := mkEquip(t, char, itemID, 12, 30)
		for f := int64(1); f <= 6; f++ {
			wantRate := baseRateBP[12] + pityBonusBP(f-1)
			op := pickOp(t, it, 13, wantRate, false)
			req := enhanceReqFor(t, op, it, 13, id.UUID{}, id.UUID{})
			frozen, err := planEnhance(t, d, char, req)
			if err != nil {
				t.Fatalf("pity plan f%d: %v", f, err)
			}
			if frozen.GetSuccess() || frozen.GetPityFailCount() != uint32(f) {
				t.Fatalf("pity f%d frozen: %+v", f, frozen)
			}
			if int64(frozen.GetFinalRateBp()) != wantRate {
				t.Fatalf("pity f%d rate: %d want %d", f, frozen.GetFinalRateBp(), wantRate)
			}
			execRecord(t, d.enhanceExec,
				enhanceRecord(t, char, op, it, 13, id.UUID{}, id.UUID{}, frozen))
			if got := pityOf(t, it, 13); got != f {
				t.Fatalf("pity stored f%d: %d", f, got)
			}
			if got := enhOf(t, it); got != 12 {
				t.Fatalf("pity floor broken: +%d", got)
			}
		}
		// Success at +13 resets the pity counter.
		rate := baseRateBP[12] + pityBonusBP(6)
		op := pickOp(t, it, 13, rate, true)
		req := enhanceReqFor(t, op, it, 13, id.UUID{}, id.UUID{})
		frozen, err := planEnhance(t, d, char, req)
		if err != nil {
			t.Fatalf("pity success plan: %v", err)
		}
		if !frozen.GetSuccess() || frozen.GetPityFailCount() != 0 ||
			int64(frozen.GetFinalRateBp()) != rate {
			t.Fatalf("pity success frozen: %+v", frozen)
		}
		execRecord(t, d.enhanceExec,
			enhanceRecord(t, char, op, it, 13, id.UUID{}, id.UUID{}, frozen))
		if got := pityOf(t, it, 13); got != 0 {
			t.Fatalf("pity not reset: %d", got)
		}
	}
}

// TestEnhanceCharmIneligible: stacked or level-ineligible charm
// selections reject CHARM_INELIGIBLE consuming nothing (ADR-0060).
func TestEnhanceCharmIneligible(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	mkItem(t, char, mat, 999_999, 0)
	credit(t, char, 1_000_000_000)
	itemID := "item.eq.t1.dinh_lang.weapon"

	// so_cap lucky at current +9 (needs <8) -> CHARM_INELIGIBLE.
	it := mkEquip(t, char, itemID, 9, 1)
	charm := mkItem(t, char, charmKindLucky+"."+luckyGradeLow, 3, 2)
	op := newOp()
	if _, err := planEnhance(t, d, char,
		enhanceReqFor(t, op, it, 10, charm, id.UUID{})); !errors.Is(err, ErrCharmIneligible) {
		t.Fatalf("ineligible lucky: %v", err)
	}
	// trung_cap insurance at current +13 (needs <12) -> CHARM_INELIGIBLE.
	it2 := mkEquip(t, char, itemID, 13, 3)
	ins := mkItem(t, char, charmKindInsure+"."+luckyGradeMid, 3, 4)
	if _, err := planEnhance(t, d, char,
		enhanceReqFor(t, op, it2, 14, id.UUID{}, ins)); !errors.Is(err, ErrCharmIneligible) {
		t.Fatalf("ineligible insurance: %v", err)
	}
	// Same instance offered as both charms -> stacked -> ineligible.
	it3 := mkEquip(t, char, itemID, 2, 5)
	dual := mkItem(t, char, charmKindLucky+"."+luckyGradeLow, 3, 6)
	if _, err := planEnhance(t, d, char,
		enhanceReqFor(t, op, it3, 3, dual, dual)); !errors.Is(err, ErrCharmIneligible) {
		t.Fatalf("stacked charm: %v", err)
	}
	// A non-charm item in a charm slot -> CHARM_INELIGIBLE.
	notCharm := mkItem(t, char, mat, 1, 7)
	if _, err := planEnhance(t, d, char,
		enhanceReqFor(t, op, it3, 3, notCharm, id.UUID{})); !errors.Is(err, ErrCharmIneligible) {
		t.Fatalf("non-charm item: %v", err)
	}
	// Nothing was consumed (rejected before any settlement).
	if qty(t, char, mat) != 1_000_000 {
		t.Fatalf("consumed on ineligible: %d", qty(t, char, mat))
	}
}

// TestEnhanceTargetLevelMismatch: target_level != current+1 rejects
// STATE_CONFLICT before any consumption (messages.md § 406).
func TestEnhanceTargetLevelMismatch(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 51)
	d := newDeps(t)
	mat := tierTable["t1"].MaterialID
	mkItem(t, char, mat, 999_999, 0)
	credit(t, char, 1_000_000_000)
	it := mkEquip(t, char, "item.eq.t1.dinh_lang.weapon", 3, 1)
	op := newOp()

	for _, target := range []uint32{3, 5, 16} {
		if _, err := planEnhance(t, d, char,
			enhanceReqFor(t, op, it, target, id.UUID{}, id.UUID{})); err == nil ||
			!errors.Is(err, ErrStateConflict) {
			t.Fatalf("target %d at +3: %v", target, err)
		}
	}
	// +16 item has no target.
	cap := mkEquip(t, char, "item.eq.t1.dinh_lang.weapon", 16, 2)
	if _, err := planEnhance(t, d, char,
		enhanceReqFor(t, op, cap, 17, id.UUID{}, id.UUID{})); err == nil ||
		!errors.Is(err, ErrStateConflict) {
		t.Fatalf("target 17 at +16: %v", err)
	}
}
