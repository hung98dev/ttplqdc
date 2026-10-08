package reward

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestContributionKeyDeduplicates: the canonical idempotency key
// (source_reward_operation_id + owner + reward_slot) dedupes a replayed
// contribution — same claim returned, nothing mutated, zero additional
// amount (ADR-0065).
func TestContributionKeyDeduplicates(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	opID := newOp()

	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, opID, "MONSTER",
			currencyLine("currency.common", 40)))
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		claimID = c.ClaimID
		if !c.Inserted {
			t.Fatalf("first contribution must insert")
		}
	})
	if r := claimsRev(t, char); r != 1 {
		t.Fatalf("rev %d want 1", r)
	}
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, opID, "MONSTER",
			currencyLine("currency.common", 40)))
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if c.Inserted {
			t.Fatalf("replayed contribution must not mutate")
		}
		if c.ClaimID != claimID {
			t.Fatalf("dedup returned %s not %s", c.ClaimID, claimID)
		}
	})
	var amt string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT amount::text FROM reward_claim_lines WHERE reward_claim_id=$1`,
		claimID.String()).Scan(&amt); err != nil {
		t.Fatalf("amount: %v", err)
	}
	if amt != "40" {
		t.Fatalf("dedup added amount: %s want 40", amt)
	}
	if r := claimsRev(t, char); r != 1 {
		t.Fatalf("dedup bumped revision: %d want 1", r)
	}
	var contrib int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claim_contributions WHERE owner_character_id=$1`,
		char.String()).Scan(&contrib); err != nil {
		t.Fatalf("contrib: %v", err)
	}
	if contrib != 1 {
		t.Fatalf("contributions %d want 1", contrib)
	}
}

// TestConsolidationUniquePendingKey: two compatible aggregates share
// the partial-unique (owner, claim_kind, consolidation_key) while
// PENDING — sequential merges funnel into one row; the second
// contribution never races a second aggregate.
func TestConsolidationUniquePendingKey(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	var c1, c2 *Created
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		var err error
		c1, err = d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 10)))
		if err != nil {
			t.Fatalf("c1: %v", err)
		}
		c2, err = d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 10)))
		if err != nil {
			t.Fatalf("c2: %v", err)
		}
	})
	if c1.ClaimID != c2.ClaimID {
		t.Fatalf("compatible aggregates split: %s vs %s", c1.ClaimID, c2.ClaimID)
	}
	var key string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT consolidation_key FROM reward_claims WHERE reward_claim_id=$1`,
		c1.ClaimID.String()).Scan(&key); err != nil {
		t.Fatalf("key: %v", err)
	}
	if key != "currency.common:MONSTER" {
		t.Fatalf("key %q want currency.common:MONSTER", key)
	}
	var rows int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claims
		 WHERE owner_character_id=$1 AND claim_kind='CURRENCY_AGGREGATE'`,
		char.String()).Scan(&rows); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("aggregate rows %d want 1 (partial unique)", rows)
	}
}

// TestTypedLinesRoundTrip: the typed-table model round-trips every
// authored field — ITEM lines carry item_id + quantity + binding +
// item_state + content_revision with currency columns NULL; CURRENCY
// lines carry currency_id + amount with item columns NULL
// (ADR-0065 typed tables).
func TestTypedLinesRoundTrip(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	in := makeInput(char, newOp(), "QUEST",
		itemLine("item.potion.hp", 5, false),
		currencyLine("currency.bound", 9))
	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, in)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if c.Kind != ClaimKindSingle {
			t.Fatalf("mixed bundle kind %s want SINGLE", c.Kind)
		}
		claimID = c.ClaimID
	})
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		got, err := d.Store.loadClaimForUpdate(ctx, tx, claimID)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(got.Lines) != 2 {
			t.Fatalf("lines %d want 2", len(got.Lines))
		}
		it, cu := got.Lines[0], got.Lines[1]
		if it.Kind != lineItem || it.ItemID != "item.potion.hp" ||
			it.Quantity.Int64() != 5 || it.EffectiveBinding != "UNBOUND" ||
			it.CurrencyID != "" || it.Amount == nil || it.Amount.Sign() != 0 {
			t.Fatalf("item line %+v", it)
		}
		if cu.Kind != lineCurrency || cu.CurrencyID != "currency.bound" ||
			cu.Amount.Int64() != 9 || cu.ItemID != "" {
			t.Fatalf("currency line %+v", cu)
		}
	})
	var itemState []byte
	if err := pool(t).QueryRow(context.Background(),
		`SELECT item_state FROM reward_claim_lines
		 WHERE reward_claim_id=$1 AND line_no=0`, claimID.String()).Scan(&itemState); err != nil {
		t.Fatalf("item_state: %v", err)
	}
	if string(itemState) != "{}" {
		t.Fatalf("item_state %s want {}", itemState)
	}
}
