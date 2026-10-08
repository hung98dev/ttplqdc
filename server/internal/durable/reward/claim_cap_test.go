package reward

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestClaimCapConsolidatesSameItemBinding: at pending_count >= 100 an
// item roll consolidates into the pending claim sharing
// owner+item_id+effective_binding — the second compatible contribution
// merges (quantity added) instead of creating a row (ADR-0063).
func TestClaimCapConsolidatesSameItemBinding(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 100)

	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			itemLine("item.potion.hp", 3, false)))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if c.Kind != ClaimKindItemConsolidated {
			t.Fatalf("kind %s want ITEM_CONSOLIDATED", c.Kind)
		}
		claimID = c.ClaimID
	})
	// ITEM_CONSOLIDATED rows do not count toward pending_count.
	if n := pendingOf(t, char); n != 100 {
		t.Fatalf("pending %d want 100", n)
	}
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			itemLine("item.potion.hp", 7, false)))
		if err != nil {
			t.Fatalf("merge: %v", err)
		}
		if c.ClaimID != claimID {
			t.Fatalf("merged into %s not %s", c.ClaimID, claimID)
		}
	})
	var qty string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT quantity::text FROM reward_claim_lines WHERE reward_claim_id=$1 AND line_no=0`,
		claimID.String()).Scan(&qty); err != nil {
		t.Fatalf("qty: %v", err)
	}
	if qty != "10" {
		t.Fatalf("consolidated quantity %s want 10", qty)
	}
}

// TestClaimCapNeverConsolidatesInstances: equipment/Soul per-instance
// payloads stay SINGLE at any pending count (ADR-0063).
func TestClaimCapNeverConsolidatesInstances(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 120)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		for i := 0; i < 2; i++ {
			c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "BOSS",
				itemLine("item.sword.rare", 1, true)))
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if c.Kind != ClaimKindSingle {
				t.Fatalf("per-instance kind %s want SINGLE", c.Kind)
			}
		}
	})
	if n := pendingOf(t, char); n != 122 {
		t.Fatalf("pending %d want 122 (instances never consolidate)", n)
	}
}

// TestClaimCapRejectsPreventableSource: a preventable source rejected
// at the soft cap consumes nothing — no claim row, no contribution.
func TestClaimCapRejectsPreventableSource(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 150)
	in := makeInput(char, newOp(), "QUEST", itemLine("item.potion.hp", 1, false))
	in.Preventable = true
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := d.Store.Create(ctx, tx, in); !errors.Is(err, ErrClaimCapReached) {
			t.Fatalf("err %v want CLAIM_CAP_REACHED", err)
		}
	})
	var contrib int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claim_contributions WHERE owner_character_id=$1`,
		char.String()).Scan(&contrib); err != nil {
		t.Fatalf("contrib: %v", err)
	}
	if contrib != 0 {
		t.Fatalf("rejected source consumed %d contributions", contrib)
	}
}

// TestPreventableSourceGateAt100: the gate trips at exactly 100; at 99
// the same preventable source creates its claim.
func TestPreventableSourceGateAt100(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 99)
	in := makeInput(char, newOp(), "QUEST", currencyLine("currency.common", 5))
	in.Preventable = true
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := d.Store.Create(ctx, tx, in); err != nil {
			t.Fatalf("at 99 create must pass: %v", err)
		}
	})
	in2 := makeInput(char, newOp(), "QUEST", itemLine("item.potion.hp", 1, true))
	in2.Preventable = true
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := d.Store.Create(ctx, tx, in2); !errors.Is(err, ErrClaimCapReached) {
			t.Fatalf("at 100 err %v want CLAIM_CAP_REACHED", err)
		}
	})
}

// TestClaimCapSoftForNonPreventableLoot: non-preventable combat loot
// creates claims beyond the 100 soft cap (never fails, never deletes).
func TestClaimCapSoftForNonPreventableLoot(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 100)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "BOSS",
			itemLine("item.sword.rare", 1, true)))
		if err != nil {
			t.Fatalf("loot at soft cap: %v", err)
		}
		if c.Kind != ClaimKindSingle {
			t.Fatalf("kind %s", c.Kind)
		}
	})
	if n := pendingOf(t, char); n != 101 {
		t.Fatalf("pending %d want 101", n)
	}
}

// TestSoftCapConsolidationBetween100And500: compatible item
// contributions consolidate across the whole 100..499 window.
func TestSoftCapConsolidationBetween100And500(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 250)
	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "WORLD_EVENT",
			itemLine("item.potion.hp", 2, false)))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		claimID = c.ClaimID
		if c.Kind != ClaimKindItemConsolidated {
			t.Fatalf("kind %s want ITEM_CONSOLIDATED at 250", c.Kind)
		}
	})
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "DUNGEON",
			itemLine("item.potion.hp", 2, false)))
		if err != nil {
			t.Fatalf("merge: %v", err)
		}
		if c.ClaimID != claimID {
			t.Fatalf("cross-source consolidation missed: key is item+binding only")
		}
	})
}

// TestHardCeiling500SkipsItemRolls: at pending_count >= 500 a
// non-preventable item roll is not performed — no claim, no
// contribution, nothing earned is lost (ADR-0062).
func TestHardCeiling500SkipsItemRolls(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 500)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			itemLine("item.potion.hp", 1, false)))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !c.Skipped {
			t.Fatalf("item roll at 500 must skip")
		}
	})
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claim_contributions WHERE owner_character_id=$1`,
		char.String()).Scan(&n); err != nil {
		t.Fatalf("contrib: %v", err)
	}
	if n != 0 {
		t.Fatalf("skipped roll recorded %d contributions", n)
	}
}

// TestHardCeilingStillSettlesCurrencyAndEscrowExpiry: at the 500 hard
// ceiling EXP/currency still settle through the currency aggregate and
// the always-settles sources (escrow expiry, PvP/Guild, compensation)
// still create item claims (ADR-0062).
func TestHardCeilingStillSettlesCurrencyAndEscrowExpiry(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	padPending(t, char, 520)

	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 10)))
		if err != nil {
			t.Fatalf("currency at ceiling: %v", err)
		}
		if c.Kind != ClaimKindCurrencyAggregate || !c.Inserted {
			t.Fatalf("currency must still settle: %+v", c)
		}
	})
	for _, src := range []string{"AUCTION_ESCROW_EXPIRY", "PVP", "GUILD_WAR", "GUILD", "ADMIN_COMPENSATION"} {
		tx(t, func(ctx context.Context, tx pgx.Tx) {
			c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), src,
				itemLine("item.sword.rare", 1, true)))
			if err != nil {
				t.Fatalf("%s at ceiling: %v", src, err)
			}
			if c.Skipped || c.Kind != ClaimKindSingle {
				t.Fatalf("%s must still create claims at 500", src)
			}
		})
	}
}
