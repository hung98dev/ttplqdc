package cooking

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestCookReplayPreservesFoodKindlingAndLifeSkillExp: one COOK record
// carries the frozen JournalCraftSnapshot — the consumed set selected at
// admission, the assigned output instance UUIDs and the authored
// per-act LIFE_SKILL EXP. Executing the record mutates exactly that
// frozen state: inputs decremented at the selected instances, outputs
// created at the frozen UUIDs, EXP granted once under the cook
// identity (idempotent replay serves the stored outcome, never
// rerolls).
func TestCookReplayPreservesFoodKindlingAndLifeSkillExp(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	// Inventory: ca_bong x2 (slots 0,5), rau_ram x1 (slot 1).
	ca1 := mkItem(t, char, "item.material.ca_bong", 1, 0)
	_ = mkItem(t, char, "item.material.ca_bong", 1, 5)
	rau1 := mkItem(t, char, "item.material.rau_ram", 1, 1)

	var snap *journalv1.JournalCraftSnapshot
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		var err error
		snap, err = d.PlanCook(ctx, tx, char, "recipe.food.ca_bong_kho")
		if err != nil {
			t.Fatalf("plan: %v", err)
		}
	})
	if snap.GetCharacterExp() != 6417 { // act I at level 1
		t.Fatalf("exp %d want 6417", snap.GetCharacterExp())
	}
	if len(snap.GetConsumed()) != 2 || len(snap.GetCreatedItems()) != 2 {
		t.Fatalf("snapshot consumed=%d created=%d", len(snap.GetConsumed()), len(snap.GetCreatedItems()))
	}
	// Frozen consume set: lowest slot first — ca_bong at inv.0, rau_ram
	// at inv.1.
	gotInst := map[string]bool{}
	for _, c := range snap.GetConsumed() {
		gotInst[string(c.GetItemInstanceId())] = true
	}
	if !gotInst[string(ca1[:])] || !gotInst[string(rau1[:])] {
		t.Fatalf("frozen consume set %v", gotInst)
	}
	var dishID, extraID string
	for _, it := range snap.GetCreatedItems() {
		switch it.GetItemId() {
		case "item.consumable.food.ca_bong_kho":
			dishID = string(it.GetItemInstanceId())
		case "item.material.cui_lua_trai":
			extraID = string(it.GetItemInstanceId())
		default:
			t.Fatalf("unexpected created item %s", it.GetItemId())
		}
	}
	if dishID == "" || extraID == "" {
		t.Fatalf("missing frozen output ids dish=%q extra=%q", dishID, extraID)
	}

	// Commit the frozen snapshot through the executor.
	op := newOp()
	rec := interactRecord(t, FamilyCook, char, op,
		protocolv1.InteractKind_INTERACT_KIND_COOK, "cooking_hearth.map_x", snap)
	exs := Executors(d)
	expBefore := expOf(t, char)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := exs[FamilyCook](ctx, tx, rec)
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcomeOf(t, o).GetS2CInteractResult()
		if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("status %v", res.GetResult().GetStatus())
		}
		if len(res.GetGranted()) != 2 {
			t.Fatalf("granted %d want 2", len(res.GetGranted()))
		}
	})
	// Inputs consumed at the frozen instances; untouched spare stack.
	if got := qty(t, char, "item.material.ca_bong"); got != 1 {
		t.Fatalf("ca_bong qty %d want 1", got)
	}
	if qty(t, char, "item.material.rau_ram") != 0 {
		t.Fatalf("rau_ram still present")
	}
	// Outputs landed at the frozen instance ids.
	for _, want := range []struct{ inst, item string }{
		{dishID, "item.consumable.food.ca_bong_kho"},
		{extraID, "item.material.cui_lua_trai"},
	} {
		var gotItem string
		if err := pool(t).QueryRow(context.Background(),
			`SELECT item_id FROM item_instances WHERE item_instance_id=$1`,
			idUUID(want.inst)).Scan(&gotItem); err != nil {
			t.Fatalf("frozen instance %s: %v", want.inst, err)
		}
		if gotItem != want.item {
			t.Fatalf("frozen instance item %q want %q", gotItem, want.item)
		}
	}
	if got := expOf(t, char) - expBefore; got != 6417 {
		t.Fatalf("exp delta %d want 6417", got)
	}
}

func idUUID(b string) string { var u id.UUID; copy(u[:], b); return u.String() }

// TestAllFoodRecipesEmitKindlingExtra: every `recipe.food.*` row grants
// `extra_output = 1 item.material.cui_lua_trai` — cooking is the
// kindling faucet (crafting_catalog.md § Hearth Cooking).
func TestAllFoodRecipesEmitKindlingExtra(t *testing.T) {
	want := []string{
		"recipe.food.ca_bong_kho", "recipe.food.ca_chep_nuong",
		"recipe.food.tom_nuong", "recipe.food.ca_ro_kho",
		"recipe.food.ruou_nep",
	}
	if len(staticRecipeTable) != len(want) {
		t.Fatalf("recipe table %d want %d", len(staticRecipeTable), len(want))
	}
	for _, rid := range want {
		r, ok := staticRecipeTable[rid]
		if !ok {
			t.Fatalf("recipe %s missing", rid)
		}
		if r.ExtraItemID != "item.material.cui_lua_trai" {
			t.Fatalf("%s extra %q", rid, r.ExtraItemID)
		}
		if len(r.Inputs) == 0 || r.OutputItemID == "" {
			t.Fatalf("%s incomplete row", rid)
		}
	}
}

// TestKindleConsumesLowestSlotStack: one KINDLE consume takes exactly
// one cui_lua_trai unit from the lowest-slot owned stack.
func TestKindleConsumesLowestSlotStack(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkItem(t, char, "item.material.cui_lua_trai", 1, 2)
	mkItem(t, char, "item.material.cui_lua_trai", 3, 7)

	op := newOp()
	rec := interactRecord(t, FamilyKindle, char, op,
		protocolv1.InteractKind_INTERACT_KIND_KINDLE, "bonfire.map_x", nil)
	exs := Executors(d)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := exs[FamilyKindle](ctx, tx, rec)
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcomeOf(t, o).GetS2CInteractResult()
		if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("status %v", res.GetResult().GetStatus())
		}
	})
	if got := qty(t, char, "item.material.cui_lua_trai"); got != 3 {
		t.Fatalf("cui qty %d want 3", got)
	}
	// The inv.2 single-unit stack was consumed entirely.
	var slot string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT il.slot FROM item_instances ii
		 JOIN item_locations il ON il.item_instance_id=ii.item_instance_id
		 WHERE il.character_id=$1 AND ii.item_id='item.material.cui_lua_trai'`,
		char.String()).Scan(&slot); err != nil || slot != "inv.7" {
		t.Fatalf("remaining stack slot %q err %v", slot, err)
	}
}

// TestCookOverflowExtraRoutesToClaim: with a full inventory the earned
// extra_output routes to a Reward Claims row carrying the immutable
// creation payload — the dish itself must still be placeable or the
// commit fails (all-or-nothing inputs/primary output; the extra is the
// earned overflow).
func TestCookOverflowExtraRoutesToClaim(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	// Partial-consume input stacks keep their slots: consume decrements
	// but frees nothing, so no output can place.
	mkItem(t, char, "item.material.ca_bong", 5, 0)
	mkItem(t, char, "item.material.rau_ram", 3, 1)
	for n := uint32(2); n < 60; n++ {
		mkItem(t, char, fmt.Sprintf("item.material.filler%d", n), 1, n)
	}
	if invCount(t, char) != 60 {
		t.Fatalf("inventory %d want 60", invCount(t, char))
	}

	var snap *journalv1.JournalCraftSnapshot
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		var err error
		snap, err = d.PlanCook(ctx, tx, char, "recipe.food.ca_bong_kho")
		if err != nil {
			t.Fatalf("plan: %v", err)
		}
	})
	op := newOp()
	rec := interactRecord(t, FamilyCook, char, op,
		protocolv1.InteractKind_INTERACT_KIND_COOK, "cooking_hearth.map_x", snap)
	exs := Executors(d)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := exs[FamilyCook](ctx, tx, rec)
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcomeOf(t, o).GetS2CInteractResult()
		if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("status %v", res.GetResult().GetStatus())
		}
	})
	// Both outputs overflow → one claim each (distinct reward slots).
	var claims int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reward_claims WHERE owner_character_id=$1`,
		char.String()).Scan(&claims); err != nil {
		t.Fatalf("claims: %v", err)
	}
	if claims != 2 {
		t.Fatalf("reward claims %d want 2", claims)
	}
	// Inputs still consumed (partial): 5→4 and 3→2.
	if qty(t, char, "item.material.ca_bong") != 4 || qty(t, char, "item.material.rau_ram") != 2 {
		t.Fatalf("partial consume wrong")
	}
}

// TestRestVerdictSuccess: BONFIRE_REST is a recorded admission — its
// only durable effect is the exactly-once 116 SUCCESS (the session and
// settlement intents live in sim/cooking).
func TestRestVerdictSuccess(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	op := newOp()
	rec := interactRecord(t, FamilyRest, char, op,
		protocolv1.InteractKind_INTERACT_KIND_BONFIRE_REST, "bonfire.map_x", nil)
	exs := Executors(d)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := exs[FamilyRest](ctx, tx, rec)
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcomeOf(t, o).GetS2CInteractResult()
		if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("status %v", res.GetResult().GetStatus())
		}
		if res.GetInteractKind() != protocolv1.InteractKind_INTERACT_KIND_BONFIRE_REST {
			t.Fatalf("kind %v", res.GetInteractKind())
		}
	})
}
