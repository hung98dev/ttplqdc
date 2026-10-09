package fishing

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

const testSpot = "fishing_spot.map.lang_da.dinh_lang.01"

// execCast runs the CAST executor in a committed transaction and
// inserts its ADMITTED receipt (the durable sequence the hook derives).
func execCast(t *testing.T, d Deps, charID id.UUID, targetID string,
	at time.Time) (id.UUID, *protocolv1.S2CInteractResult) {
	t.Helper()
	opID := newOp()
	rec := interactRecord(t, FamilyCast, charID, opID,
		protocolv1.InteractKind_INTERACT_KIND_CAST, targetID)
	insertReceipt(t, FamilyCast, charID, opID, at)
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	o, err := d.castExec(ctx, txx, rec)
	if err != nil {
		t.Fatalf("castExec: %v", err)
	}
	if err := txx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return opID, outcomeOf(t, o).GetS2CInteractResult()
}

func execHook(t *testing.T, d Deps, charID id.UUID, targetID string,
	at time.Time) (id.UUID, *protocolv1.S2CInteractResult) {
	t.Helper()
	opID := newOp()
	rec := interactRecord(t, FamilyHook, charID, opID,
		protocolv1.InteractKind_INTERACT_KIND_HOOK, targetID)
	insertReceipt(t, FamilyHook, charID, opID, at)
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	o, err := d.hookExec(ctx, txx, rec)
	if err != nil {
		t.Fatalf("hookExec: %v", err)
	}
	if err := txx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return opID, outcomeOf(t, o).GetS2CInteractResult()
}

// TestCastConsumesBaitOnceAfterAllGates — rod + bait + cap + legal
// spot all pass, then exactly one bait unit leaves the lowest slot.
func TestCastConsumesBaitOnceAfterAllGates(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkItem(t, char, RodItemID, 1, 0)
	mkItem(t, char, BaitItemID, 3, 1)
	d := newDeps(t)
	_, res := execCast(t, d, char, testSpot, time.Now().UTC())
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("cast rejected: %v", res.GetResult().GetErrorCode())
	}
	if got := qty(t, char, BaitItemID); got != 2 {
		t.Fatalf("bait qty = %d, want 2", got)
	}
	if got := qty(t, char, RodItemID); got != 1 {
		t.Fatalf("rod consumed: qty %d", got)
	}
}

// TestCastGateOrder — a missing rod or bait or a reached cap rejects
// CAST with the bait untouched (consume only after every gate).
func TestCastGateOrder(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := newDeps(t)
	// No rod, no bait.
	_, res := execCast(t, d, char, testSpot, time.Now().UTC())
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND {
		t.Fatalf("no-rod code = %v", res.GetResult().GetErrorCode())
	}
	// Rod only, still no bait.
	mkItem(t, char, RodItemID, 1, 0)
	_, res = execCast(t, d, char, testSpot, time.Now().UTC())
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND {
		t.Fatalf("no-bait code = %v", res.GetResult().GetErrorCode())
	}
	// Illegal spot id.
	mkItem(t, char, BaitItemID, 2, 1)
	_, res = execCast(t, d, char, "fishing_spot", time.Now().UTC())
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("bad spot code = %v", res.GetResult().GetErrorCode())
	}
	if got := qty(t, char, BaitItemID); got != 2 {
		t.Fatalf("bait consumed on rejection: %d", got)
	}
	// Daily cap reached -> 51st rejected, bait preserved.
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE characters SET fishing_utc_date = $2, fishing_catch_count = 50
		 WHERE character_id = $1`,
		char.String(), time.Now().UTC().Format("2006-01-02")); err != nil {
		t.Fatalf("cap: %v", err)
	}
	_, res = execCast(t, d, char, testSpot, time.Now().UTC())
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED {
		t.Fatalf("cap code = %v", res.GetResult().GetErrorCode())
	}
	if got := qty(t, char, BaitItemID); got != 2 {
		t.Fatalf("bait consumed at cap: %d", got)
	}
}

// TestHookSettlesCatchCounterExp — one HOOK resolves the cast: one
// catch from the closed table, the daily counter commits in the same
// transaction, and the authored LIFE_SKILL EXP lands.
func TestHookSettlesCatchCounterExp(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkItem(t, char, RodItemID, 1, 0)
	mkItem(t, char, BaitItemID, 1, 1)
	d := newDeps(t)
	now := time.Now().UTC()
	execCast(t, d, char, testSpot, now)
	_, res := execHook(t, d, char, testSpot, now.Add(time.Second))
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("hook rejected: %v", res.GetResult().GetErrorCode())
	}
	if len(res.GetGranted()) != 1 || res.GetGranted()[0].GetQuantity() != 1 {
		t.Fatalf("granted = %v", res.GetGranted())
	}
	catchID := res.GetGranted()[0].GetItemId()
	legal := map[string]bool{}
	for _, r := range defaultTable {
		legal[r.ItemID] = true
	}
	if !legal[catchID] {
		t.Fatalf("catch %s outside the closed table", catchID)
	}
	if got := qty(t, char, catchID); got != 1 {
		t.Fatalf("catch qty = %d", got)
	}
	day, n := catchState(t, char)
	if day != now.Format("2006-01-02") || n != 1 {
		t.Fatalf("counter = (%s,%d)", day, n)
	}
	if expOf(t, char) != 6417 { // act I award
		t.Fatalf("exp = %d, want 6417", expOf(t, char))
	}
}

// TestHookWithoutCastFails — a HOOK with no preceding accepted cast
// resolves as failure (no catch, no counter mutation).
func TestHookWithoutCastFails(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	d := newDeps(t)
	_, res := execHook(t, d, char, testSpot, time.Now().UTC())
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR ||
		res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("hook w/o cast = %v", res.GetResult())
	}
	if _, n := catchState(t, char); n != 0 {
		t.Fatalf("counter mutated: %d", n)
	}
}

// TestHookCapRejectsFiftyFirst — the atomic counter check rejects the
// 51st settlement even when the cast was admitted below the cap.
func TestHookCapRejectsFiftyFirst(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkItem(t, char, RodItemID, 1, 0)
	mkItem(t, char, BaitItemID, 1, 1)
	d := newDeps(t)
	now := time.Now().UTC()
	execCast(t, d, char, testSpot, now)
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE characters SET fishing_utc_date = $2, fishing_catch_count = 50
		 WHERE character_id = $1`,
		char.String(), now.Format("2006-01-02")); err != nil {
		t.Fatalf("cap: %v", err)
	}
	_, res := execHook(t, d, char, testSpot, now.Add(time.Second))
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED {
		t.Fatalf("51st code = %v", res.GetResult().GetErrorCode())
	}
	if _, n := catchState(t, char); n != 50 {
		t.Fatalf("counter = %d, want 50", n)
	}
}

// TestOverflowRoutesToRewardClaim — a full inventory routes the earned
// catch to Reward Claims with source_type FISHING and the authored
// (spot, character, utc_date, cast_sequence) source_ref.
func TestOverflowRoutesToRewardClaim(t *testing.T) {
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkItem(t, char, RodItemID, 1, 0)
	mkItem(t, char, BaitItemID, 1, 1)
	// Fill all 60 slots with non-stackable catches so nothing fits.
	for i := uint32(2); i < 60; i++ {
		mkItem(t, char, "item.material.ca_chep", 99, i)
	}
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE item_instances SET quantity = 99 WHERE item_id = 'item.material.ca_chep'`); err != nil {
		t.Fatalf("fill: %v", err)
	}
	// Free slot would be inv.60 only — capacity is 60, slots 0..59.
	// Slots used: 0,1 (rod/bait pre-cast), 2..59 -> after bait consumed
	// slot 1 frees, but stacks must merge: ca_chep at 99 = full.
	d := newDeps(t)
	now := time.Now().UTC()
	execCast(t, d, char, testSpot, now)
	// Refill slot 1 after the cast freed it.
	mkItem(t, char, "item.material.ca_chep", 99, 1)
	_, res := execHook(t, d, char, testSpot, now.Add(time.Second))
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("hook rejected: %v", res.GetResult().GetErrorCode())
	}
	var st, ref string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT source_type, source_reference FROM reward_claims
		 WHERE owner_character_id = $1 ORDER BY created_at DESC LIMIT 1`,
		char.String()).Scan(&st, &ref); err != nil {
		t.Fatalf("no claim row: %v", err)
	}
	if st != "FISHING" {
		t.Fatalf("source_type = %s", st)
	}
	want := testSpot + "." + char.String() + "." + now.Format("2006-01-02") + ".1"
	if ref != want {
		t.Fatalf("source_ref = %s, want %s", ref, want)
	}
}

// TestRollDeterminismAndReplay — the keyed PCG-64 stream is fixed per
// (character, utc_date, cast_sequence): re-deriving the same roll
// returns the same catch (replay never rerolls).
func TestRollDeterminismAndReplay(t *testing.T) {
	table, ok := TableForSpot(testSpot, -1, false)
	if !ok {
		t.Fatal("default table unresolved")
	}
	char := id.NewV4()
	key := rollKey(char, "2026-10-01", 7)
	if Roll(table, key) != Roll(table, key) {
		t.Fatal("same key rerolled")
	}
	// A different sequence rolls a different stream.
	seen := map[string]bool{}
	for seq := uint64(1); seq <= 32; seq++ {
		got := Roll(table, rollKey(char, "2026-10-01", seq))
		for _, r := range table {
			if r.ItemID == got {
				seen[got] = true
			}
		}
	}
	if len(seen) < 2 {
		t.Fatalf("rolls never left one row: %v", seen)
	}
}

// TestTableLegality — the default table is the closed COMMON∪RARE set
// summing 10000 bp; a seasonal table appears only for the featured
// season's region spot; Di Tích shifts rare to 110 and drops the
// largest common by 10.
func TestTableLegality(t *testing.T) {
	sum := 0
	for _, r := range defaultTable {
		sum += r.WeightBP
	}
	if sum != 10000 {
		t.Fatalf("default sum = %d", sum)
	}
	if _, ok := TableForSpot("fishing_spot.map.lang_da.dinh_lang.01", 0, false); !ok {
		t.Fatal("season 0 spot table missing")
	}
	rows, _ := TableForSpot("fishing_spot.map.lang_da.dinh_lang.01", 0, false)
	if len(rows) != 6 || rows[5].ItemID != "item.material.ca_linh_giang" {
		t.Fatalf("seasonal table: %v", rows)
	}
	// Season 3/4 -> default.
	rows, _ = TableForSpot("fishing_spot.map.lang_da.dinh_lang.01", 3, false)
	if len(rows) != len(defaultTable) {
		t.Fatal("season 3 took a seasonal table")
	}
	// Di Tích: rare 100->110, largest common -10.
	rows, _ = TableForSpot(testSpot, -1, true)
	if rows[4].WeightBP != 110 || rows[0].WeightBP != 3990 {
		t.Fatalf("di tich adjust: %+v", rows)
	}
}
