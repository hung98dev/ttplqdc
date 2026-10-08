package reward

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestRewardClaimMaterialization: a SINGLE claim delivers its full
// payload through 408 — item materializes on the lowest free slot, the
// claim commits CLAIMED with claim_operation_id, the 409 receipt
// carries the granted instance and claims_revision bumped exactly once.
func TestRewardClaimMaterialization(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	opID, claimOp := newOp(), newOp()

	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, opID, "QUEST",
			itemLine("item.potion.hp", 5, false)))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		claimID = c.ClaimID
	})
	if claimsRev(t, char) != 1 {
		t.Fatalf("claims_revision after create = %d, want 1", claimsRev(t, char))
	}

	ex := Executors(d)[ClaimFamily]
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, claimRecord(char, claimOp, claimID))
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		out := outcome(t, o)
		res := out.GetS2CRewardClaimResult()
		if res == nil || res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("409 result: %+v", res)
		}
		if len(res.GetGranted()) != 1 || res.GetGranted()[0].GetItemId() != "item.potion.hp" ||
			res.GetGranted()[0].GetQuantity() != 5 {
			t.Fatalf("granted: %+v", res.GetGranted())
		}
		if res.GetClaimState() != protocolv1.RewardClaimState_REWARD_CLAIM_STATE_CLAIMED {
			t.Fatalf("claim_state %v", res.GetClaimState())
		}
	})

	var qty int32
	if err := pool(t).QueryRow(context.Background(),
		`SELECT ii.quantity FROM item_instances ii
		 JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		 WHERE il.character_id=$1 AND il.slot='inv.0'`, char.String()).Scan(&qty); err != nil {
		t.Fatalf("slot: %v", err)
	}
	if qty != 5 {
		t.Fatalf("qty %d want 5", qty)
	}
	var state, cop string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT state, claim_operation_id::text FROM reward_claims WHERE reward_claim_id=$1`,
		claimID.String()).Scan(&state, &cop); err != nil {
		t.Fatalf("claim row: %v", err)
	}
	if state != "CLAIMED" || cop != claimOp.String() {
		t.Fatalf("state=%s op=%s", state, cop)
	}
	if claimsRev(t, char) != 2 {
		t.Fatalf("claims_revision after claim = %d, want 2", claimsRev(t, char))
	}
}

// TestPersistentOverflowClaims: a SINGLE claim that cannot fit commits
// an INVENTORY_FULL verdict, mutates nothing, stays PENDING, and a
// later op after making space delivers it (persistence across ops).
func TestPersistentOverflowClaims(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	for i := 0; i < 60; i++ { // fill all 60 slots
		mkItem(t, char, "item.potion.mp", 1, uint32(i))
	}
	opID, claimOp := newOp(), newOp()
	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, opID, "MONSTER",
			itemLine("item.potion.hp", 3, false)))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		claimID = c.ClaimID
	})

	ex := Executors(d)[ClaimFamily]
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, claimRecord(char, claimOp, claimID))
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcome(t, o).GetS2CRewardClaimResult()
		if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL {
			t.Fatalf("error %v want INVENTORY_FULL", res.GetResult().GetErrorCode())
		}
	})
	var st string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT state FROM reward_claims WHERE reward_claim_id=$1`, claimID.String()).Scan(&st); err != nil {
		t.Fatalf("state: %v", err)
	}
	if st != "PENDING" {
		t.Fatalf("state %s want PENDING", st)
	}
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM item_instances i
		 JOIN item_locations l ON l.item_instance_id = i.item_instance_id
		 WHERE i.item_id='item.potion.hp' AND l.character_id=$1`, char.String()).Scan(&n); err != nil {
		t.Fatalf("instances: %v", err)
	}
	if n != 0 {
		t.Fatalf("overflow created %d instances", n)
	}

	// Free one slot, retry with a new operation -> batch delivers.
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		var inst string
		if err := tx.QueryRow(ctx,
			`SELECT item_instance_id FROM item_locations
			 WHERE character_id=$1 AND slot='inv.0'`, char.String()).Scan(&inst); err != nil {
			t.Fatalf("pick: %v", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM item_locations WHERE item_instance_id=$1::uuid`, inst); err != nil {
			t.Fatalf("del loc: %v", err)
		}
	})
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, claimRecord(char, newOp(), claimID))
		if err != nil {
			t.Fatalf("exec2: %v", err)
		}
		res := outcome(t, o).GetS2CRewardClaimResult()
		if res.GetClaimState() != protocolv1.RewardClaimState_REWARD_CLAIM_STATE_CLAIMED {
			t.Fatalf("state %v after space", res.GetClaimState())
		}
	})
}

// TestCompatibleCurrencyAggregation: currency-only contributions sharing
// owner+currency_id+source_family merge into one aggregate claim; the
// contribution ledger keeps each key; delivery credits the wallet and
// leaves the aggregate CLAIMED.
func TestCompatibleCurrencyAggregation(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkWallet(t, char, "currency.common", 10)

	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c1, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 100)))
		if err != nil {
			t.Fatalf("c1: %v", err)
		}
		c2, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 50)))
		if err != nil {
			t.Fatalf("c2: %v", err)
		}
		if c1.ClaimID != c2.ClaimID {
			t.Fatalf("aggregate keys differ: %s vs %s", c1.ClaimID, c2.ClaimID)
		}
		claimID = c1.ClaimID
	})
	var rows int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claims WHERE owner_character_id=$1 AND state='PENDING'`,
		char.String()).Scan(&rows); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("aggregate rows %d want 1", rows)
	}
	var contrib int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claim_contributions WHERE owner_character_id=$1`,
		char.String()).Scan(&contrib); err != nil {
		t.Fatalf("contrib: %v", err)
	}
	if contrib != 2 {
		t.Fatalf("contributions %d want 2", contrib)
	}
	var amt string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT amount::text FROM reward_claim_lines WHERE reward_claim_id=$1 AND line_no=0`,
		claimID.String()).Scan(&amt); err != nil {
		t.Fatalf("amount: %v", err)
	}
	if amt != "150" {
		t.Fatalf("merged amount %s want 150", amt)
	}

	ex := Executors(d)[ClaimFamily]
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, claimRecord(char, newOp(), claimID))
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		res := outcome(t, o).GetS2CRewardClaimResult()
		if len(res.GetCurrencyDelta()) != 1 || res.GetCurrencyDelta()[0].GetAmount() != 150 {
			t.Fatalf("delta %+v", res.GetCurrencyDelta())
		}
	})
	var bal int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT balance FROM character_currencies WHERE character_id=$1 AND currency_id='currency.common'`,
		char.String()).Scan(&bal); err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal != 160 {
		t.Fatalf("balance %d want 160", bal)
	}
}

// TestRewardClaimSourceTypeEnum: unknown source_type fails closed at
// validation (ADR-0060 closed enum).
func TestRewardClaimSourceTypeEnum(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		_, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MAIL",
			currencyLine("currency.common", 1)))
		if !errors.Is(err, ErrUnknownSourceType) {
			t.Fatalf("err %v want ErrUnknownSourceType", err)
		}
	})
	for _, st := range []string{"MONSTER", "BOSS", "BOSS_CHEST", "DUNGEON", "QUEST",
		"WORLD_EVENT", "ATLAS", "FEAT", "LEVEL_MILESTONE", "PVP", "GUILD_WAR",
		"GUILD", "FISHING", "HIDDEN_CHEST", "AUCTION_ESCROW_EXPIRY", "ADMIN_COMPENSATION"} {
		in := makeInput(char, newOp(), st, currencyLine("currency.common", 1))
		if err := in.validate(); err != nil {
			t.Fatalf("enum %s rejected: %v", st, err)
		}
	}
}

// TestRewardClaimsStatePush: the attach surface carries revision,
// total_count and the first page in the same order the list endpoint
// serves.
func TestRewardClaimsStatePush(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		for i := 0; i < 3; i++ {
			if _, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
				itemLine("item.potion.hp", 1, true))); err != nil {
				t.Fatalf("create %d: %v", i, err)
			}
		}
		push, err := d.Store.StatePush(ctx, tx, char)
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		if push.GetTotalCount() != 3 || len(push.GetClaims()) != 3 || push.GetCap() != 100 {
			t.Fatalf("push %+v", push)
		}
		if push.GetClaimsRevision() != 3 {
			t.Fatalf("revision %d want 3", push.GetClaimsRevision())
		}
	})
}
