package reward

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestAttachSnapshotOldest50: 434 carries total_count + the 50 oldest
// PENDING claims in (created_at, reward_claim_id) order — the attach
// frame is bounded regardless of how many claims exist (ADR-0064).
func TestAttachSnapshotOldest50(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	var want []id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		for i := 0; i < 70; i++ {
			in := makeInput(char, newOp(), "MONSTER",
				itemLine("item.potion.hp", 1, true))
			in.CreatedAtUnixMs = 1000 + int64(i)
			c, err := d.Store.Create(ctx, tx, in)
			if err != nil {
				t.Fatalf("create %d: %v", i, err)
			}
			if i < 50 {
				want = append(want, c.ClaimID)
			}
		}
	})
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		push, err := d.Store.StatePush(ctx, tx, char)
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		if push.GetTotalCount() != 70 {
			t.Fatalf("total_count %d want 70", push.GetTotalCount())
		}
		if len(push.GetClaims()) != 50 {
			t.Fatalf("page %d want 50", len(push.GetClaims()))
		}
		for i, got := range push.GetClaims() {
			var w [16]byte
			copy(w[:], got.GetRewardClaimId())
			if id.UUID(w) != want[i] {
				t.Fatalf("order mismatch at %d", i)
			}
		}
	})
}

// TestListPagingOrder: 439 serves the same oldest-first order in
// offset/limit pages; the echoed offset + shared claims_revision match
// the snapshot revision.
func TestListPagingOrder(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	var first, last *Claim
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		for i := 0; i < 60; i++ {
			in := makeInput(char, newOp(), "QUEST", itemLine("item.potion.hp", 1, true))
			in.CreatedAtUnixMs = 5000 + int64(i)
			c, err := d.Store.Create(ctx, tx, in)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if i == 0 {
				first, _ = d.Store.loadClaimForUpdate(ctx, tx, c.ClaimID)
			}
			if i == 59 {
				last, _ = d.Store.loadClaimForUpdate(ctx, tx, c.ClaimID)
			}
		}
	})
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		p0, err := d.Store.ListPage(ctx, tx, char, 0, 50)
		if err != nil {
			t.Fatalf("page0: %v", err)
		}
		p1, err := d.Store.ListPage(ctx, tx, char, 50, 50)
		if err != nil {
			t.Fatalf("page1: %v", err)
		}
		if p0.GetTotalCount() != 60 || p1.GetTotalCount() != 60 {
			t.Fatalf("totals %d/%d", p0.GetTotalCount(), p1.GetTotalCount())
		}
		if len(p0.GetClaims()) != 50 || len(p1.GetClaims()) != 10 {
			t.Fatalf("page lens %d/%d", len(p0.GetClaims()), len(p1.GetClaims()))
		}
		var f, l [16]byte
		copy(f[:], p0.GetClaims()[0].GetRewardClaimId())
		copy(l[:], p1.GetClaims()[9].GetRewardClaimId())
		if id.UUID(f) != first.ClaimID || id.UUID(l) != last.ClaimID {
			t.Fatalf("order: first %x last %x", f, l)
		}
		if p0.GetClaimsRevision() != p1.GetClaimsRevision() {
			t.Fatalf("revisions differ %d vs %d", p0.GetClaimsRevision(), p1.GetClaimsRevision())
		}
		if p1.GetOffset() != 50 {
			t.Fatalf("offset echo %d", p1.GetOffset())
		}
	})
}

// TestDeltaRevisionIncrements: every committed claims-projection change
// bumps characters.claims_revision by exactly one — create (+1), merge
// contribution (+1), claim delivery (+1).
func TestDeltaRevisionIncrements(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkWallet(t, char, "currency.common", 0)

	var claimID id.UUID
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		c, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 5)))
		if err != nil {
			t.Fatalf("c1: %v", err)
		}
		claimID = c.ClaimID
	})
	if r := claimsRev(t, char); r != 1 {
		t.Fatalf("rev after create %d want 1", r)
	}
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := d.Store.Create(ctx, tx, makeInput(char, newOp(), "MONSTER",
			currencyLine("currency.common", 5))); err != nil {
			t.Fatalf("c2: %v", err)
		}
	})
	if r := claimsRev(t, char); r != 2 {
		t.Fatalf("rev after merge %d want 2", r)
	}
	ex := Executors(d)[ClaimFamily]
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := ex(ctx, tx, claimRecord(char, newOp(), claimID)); err != nil {
			t.Fatalf("exec: %v", err)
		}
	})
	if r := claimsRev(t, char); r != 3 {
		t.Fatalf("rev after claim %d want 3", r)
	}

	// The 441 delta carries the post-change revision + totals for the
	// client-side ordered delta apply.
	var cnt int64
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		push, err := d.Store.StatePush(ctx, tx, char)
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		delta := DeltaPush(push.GetClaimsRevision(), uint64(push.GetTotalCount()),
			nil, []id.UUID{claimID})
		if delta.GetClaimsRevision() != 3 {
			t.Fatalf("delta rev %d", delta.GetClaimsRevision())
		}
		cnt = int64(delta.GetTotalCount())
	})
	_ = cnt
	_ = protocolv1.S2CRewardClaimDelta{}
}

// TestLargeClaimCountFrameBounded: at several hundred pending claims
// the attach frame still carries at most one bounded page — no frame
// exceeds the outbound limit at any claim count (ADR-0064).
func TestLargeClaimCountFrameBounded(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		for i := 0; i < 220; i++ {
			in := makeInput(char, newOp(), "MONSTER",
				itemLine("item.potion.hp", 1, true))
			if _, err := d.Store.Create(ctx, tx, in); err != nil {
				t.Fatalf("create: %v", err)
			}
		}
		push, err := d.Store.StatePush(ctx, tx, char)
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		if push.GetTotalCount() != 220 {
			t.Fatalf("total %d", push.GetTotalCount())
		}
		if len(push.GetClaims()) != 50 {
			t.Fatalf("attach page %d exceeds bound", len(push.GetClaims()))
		}
	})
}
