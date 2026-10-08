package auction

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/reward"
)

// mkRevisedItem stamps the instance with a content revision, as every
// production item carries (created through claim delivery or stamped at
// authoring).
func mkRevisedItem(t *testing.T, charID id.UUID, itemID string, qty int32,
	slot int, binding string) id.UUID {
	t.Helper()
	inst := mkItem(t, charID, itemID, qty, slot, binding)
	if _, err := pool(t).Exec(ctx(t),
		`UPDATE item_instances SET content_revision=$1 WHERE item_instance_id=$2`,
		strings.Repeat("a", 64), inst.String()); err != nil {
		t.Fatalf("stamp revision: %v", err)
	}
	return inst
}

// backdate pushes ended_at past EscrowClaimDelay so the listing is
// sweep-eligible.
func backdate(t *testing.T, lid id.UUID, past time.Duration) {
	t.Helper()
	if _, err := pool(t).Exec(ctx(t),
		`UPDATE auction_listings SET ended_at = NOW() - $1::interval
		 WHERE listing_id=$2`, past.String(), lid.String()); err != nil {
		t.Fatalf("backdate: %v", err)
	}
}

// TestMoveToClaimSweep creates a claim through the real IMP-010
// claim-creation API for an unreclaimed escrow asset older than
// EscrowClaimDelay: the listing lands MOVED_TO_CLAIM, the escrow
// location row is removed, and the claim carries the immutable payload.
func TestMoveToClaimSweep(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)

	inst := mkRevisedItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Cancel(ctx(t), tx, CancelIn{OperationID: id.NewV4(),
			CharacterID: seller, AccountID: acct, ListingID: lid})
		return err
	}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	backdate(t, lid, EscrowClaimDelay+time.Hour)

	sink := RewardClaimSink{Store: reward.NewStore(pool(t))}
	if err := inTx(t, func(tx pgx.Tx) error {
		n, err := s.MoveToClaimSweep(ctx(t), tx, time.Now(), sink, nil)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("moved=%d", n)
		}
		return nil
	}); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var st string
	var et *time.Time
	if err := pool(t).QueryRow(ctx(t),
		`SELECT state, ended_at FROM auction_listings WHERE listing_id=$1`,
		lid.String()).Scan(&st, &et); err != nil {
		t.Fatalf("listing: %v", err)
	}
	if st != string(StateMovedToClaim) {
		t.Fatalf("state=%s", st)
	}
	if et == nil {
		t.Fatal("ended_at not overwritten on MOVED_TO_CLAIM")
	}
	var locs int
	if err := pool(t).QueryRow(ctx(t),
		`SELECT count(*) FROM item_locations WHERE item_instance_id=$1`,
		inst.String()).Scan(&locs); err != nil {
		t.Fatalf("locs: %v", err)
	}
	if locs != 0 {
		t.Fatalf("escrow location rows=%d", locs)
	}

	var kind, srcType, srcRef, slot string
	var state string
	if err := pool(t).QueryRow(ctx(t),
		`SELECT claim_kind, source_type, source_reference, reward_slot, state
		   FROM reward_claims WHERE owner_character_id=$1`,
		seller.String()).Scan(&kind, &srcType, &srcRef, &slot, &state); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if kind != reward.ClaimKindSingle || srcType != "AUCTION_ESCROW_EXPIRY" ||
		srcRef != lid.String() || slot != "escrow" || state != "PENDING" {
		t.Fatalf("claim %s %s %s %s %s", kind, srcType, srcRef, slot, state)
	}
	var rev int64
	if err := pool(t).QueryRow(ctx(t),
		`SELECT claims_revision FROM characters WHERE character_id=$1`,
		seller.String()).Scan(&rev); err != nil {
		t.Fatalf("claims_revision: %v", err)
	}
	if rev != 1 {
		t.Fatalf("claims_revision=%d", rev)
	}
	var lineItemID string
	var qty int64
	var crev string
	if err := pool(t).QueryRow(ctx(t),
		`SELECT item_id, quantity, content_revision
		   FROM reward_claim_lines WHERE reward_claim_id =
		     (SELECT reward_claim_id FROM reward_claims WHERE owner_character_id=$1)`,
		seller.String()).Scan(&lineItemID, &qty, &crev); err != nil {
		t.Fatalf("line: %v", err)
	}
	if lineItemID != "item.potion.hp" || qty != 1 || crev != strings.Repeat("a", 64) {
		t.Fatalf("line %s qty=%d rev=%s", lineItemID, qty, crev)
	}
}

// TestMoveToClaimSweepIdempotent replays the sweep on the same
// (listing, operation) key: the contribution ledger dedupes and no
// second claim is created — the listing is already MOVED_TO_CLAIM.
func TestMoveToClaimSweepIdempotent(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)

	inst := mkRevisedItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Cancel(ctx(t), tx, CancelIn{OperationID: id.NewV4(),
			CharacterID: seller, AccountID: acct, ListingID: lid})
		return err
	}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	backdate(t, lid, EscrowClaimDelay+time.Hour)

	sink := RewardClaimSink{Store: reward.NewStore(pool(t))}
	opID := id.NewV4()
	opFor := func(id.UUID) id.UUID { return opID }
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.MoveToClaimSweep(ctx(t), tx, time.Now(), sink, opFor)
		return err
	}); err != nil {
		t.Fatalf("sweep1: %v", err)
	}

	// Force a second sweep against the same deterministic operation:
	// replay the claim creation inside one tx.
	if err := inTx(t, func(tx pgx.Tx) error {
		var itemID string
		var qty int64
		var binding string
		var istate []byte
		var crev *string
		if err := tx.QueryRow(ctx(t),
			`SELECT item_id, quantity, effective_binding, item_state, content_revision
			   FROM item_instances WHERE item_instance_id=$1`,
			inst.String()).Scan(&itemID, &qty, &binding, &istate, &crev); err != nil {
			return err
		}
		claimID, err := sink.CreateEscrowExpiryClaim(ctx(t), tx, EscrowExpiryClaim{
			OperationID: opID, OwnerCharacterID: seller,
			SourceReference: lid.String(), ItemID: itemID, Quantity: qty,
			EffectiveBinding: binding, ItemState: istate, ContentRevision: crev,
		})
		if err != nil {
			return err
		}
		if claimID == (id.UUID{}) {
			t.Fatal("empty replay claim id")
		}
		return nil
	}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	var claims int
	if err := pool(t).QueryRow(ctx(t),
		`SELECT count(*) FROM reward_claims WHERE owner_character_id=$1`,
		seller.String()).Scan(&claims); err != nil {
		t.Fatalf("claims: %v", err)
	}
	if claims != 1 {
		t.Fatalf("claims=%d", claims)
	}
	var rev int64
	if err := pool(t).QueryRow(ctx(t),
		`SELECT claims_revision FROM characters WHERE character_id=$1`,
		seller.String()).Scan(&rev); err != nil {
		t.Fatalf("claims_revision: %v", err)
	}
	if rev != 1 {
		t.Fatalf("claims_revision=%d after replay", rev)
	}
}

// TestMoveToClaimSweepMissingRevision refuses to fabricate a content
// revision for an unstamped escrow asset: the claim payload must carry
// the recorded provenance revision (ADR-0012).
func TestMoveToClaimSweepMissingRevision(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)

	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Cancel(ctx(t), tx, CancelIn{OperationID: id.NewV4(),
			CharacterID: seller, AccountID: acct, ListingID: lid})
		return err
	}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	backdate(t, lid, EscrowClaimDelay+time.Hour)

	sink := RewardClaimSink{Store: reward.NewStore(pool(t))}
	err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.MoveToClaimSweep(ctx(t), tx, time.Now(), sink, nil)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "content revision") {
		t.Fatalf("sweep err=%v", err)
	}
}
