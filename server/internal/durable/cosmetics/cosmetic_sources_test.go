package cosmetics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestRedeemDualRoute — material and special routes grant the same
// entitlement; an attempt consumes exactly one route; after
// acquisition the other route is a permanent no-op.
func TestRedeemDualRoute(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	_ = acct
	cid := "cosmetic.frame.nui_thieng" // 30 vai_hoa_van OR 20 special

	mkItem(t, char, "item.material.vai_hoa_van", 30)
	mkCurrency(t, char, string(currency.Special), 20)
	op := id.NewV4()
	txOf(t, func(tx pgx.Tx) error {
		return s.Redeem(ctx, tx, char, cid,
			protocolv1.CosmeticRoute_COSMETIC_ROUTE_MATERIAL, op,
			time.Now().UTC())
	})
	var stack int
	if err := sharedPool.QueryRow(ctx,
		`SELECT COALESCE(sum(ii.quantity),0)
		 FROM item_instances ii
		 JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		 WHERE il.character_id = $1 AND ii.item_id = 'item.material.vai_hoa_van'`,
		char[:]).Scan(&stack); err != nil {
		t.Fatal(err)
	}
	if stack != 0 {
		t.Fatalf("material left: %d", stack)
	}
	var bal int64
	if err := sharedPool.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		 WHERE character_id = $1 AND currency_id = $2`,
		char[:], string(currency.Special)).Scan(&bal); err != nil {
		t.Fatal(err)
	}
	if bal != 20 {
		t.Fatalf("special consumed: %d", bal)
	}
	// Already-owned: the other route is a no-op consuming nothing.
	txOf(t, func(tx pgx.Tx) error {
		return s.Redeem(ctx, tx, char, cid,
			protocolv1.CosmeticRoute_COSMETIC_ROUTE_CURRENCY_SPECIAL,
			id.NewV4(), time.Now().UTC())
	})
	if err := sharedPool.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		 WHERE character_id = $1 AND currency_id = $2`,
		char[:], string(currency.Special)).Scan(&bal); err != nil {
		t.Fatal(err)
	}
	if bal != 20 {
		t.Fatalf("double-consume on owned cosmetic: %d", bal)
	}
}

// TestRedeemInsufficient — a material attempt with a short stack
// fails INSUFFICIENT_ITEM and consumes nothing.
func TestRedeemInsufficient(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, char := mkCharacter(t)
	ctx := context.Background()
	cid := "cosmetic.frame.nui_thieng"
	mkItem(t, char, "item.material.vai_hoa_van", 5)

	err := txErr(func(tx pgx.Tx) error {
		return s.Redeem(ctx, tx, char, cid,
			protocolv1.CosmeticRoute_COSMETIC_ROUTE_MATERIAL,
			id.NewV4(), time.Now().UTC())
	})
	if !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("err: %v", err)
	}
	var stack int
	if err := sharedPool.QueryRow(ctx,
		`SELECT sum(quantity) FROM item_instances ii
		 JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		 WHERE il.character_id = $1`, char[:]).Scan(&stack); err != nil {
		t.Fatal(err)
	}
	if stack != 5 {
		t.Fatalf("stack consumed on failure: %d", stack)
	}
}

// TestFeatCountersAndMilestone — cumulative counters reach the
// threshold and grant once; a repeat qualifying event is a no-op.
func TestFeatCountersAndMilestone(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	const f = "feat.combat.slay_ma_da" // KILL_COUNT 1000 → title dao_cam? catalog grant
	cos := FeatDefs[0].Reward

	done := false
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Increment(ctx, tx, char, f, 999, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if done {
		t.Fatal("milestone before threshold")
	}
	owned, _ := s.Owned(ctx, nil, acct, char, cos)
	if owned {
		t.Fatal("granted before threshold")
	}
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Increment(ctx, tx, char, f, 1, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if !done {
		t.Fatal("milestone did not grant at threshold")
	}
	owned, _ = s.Owned(ctx, nil, acct, char, cos)
	if !owned {
		t.Fatal("milestone cosmetic missing")
	}
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Increment(ctx, tx, char, f, 100, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if done {
		t.Fatal("second milestone grant")
	}
	var n int
	if err := sharedPool.QueryRow(ctx,
		`SELECT count(*) FROM character_feat_milestones
		 WHERE character_id = $1 AND feat_id = $2`, char[:], f).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("milestones: %d", n)
	}
}

// TestFlagFeat — boolean flag feats latch once the qualifying level
// is reached and stay a no-op after.
func TestFlagFeat(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, char := mkCharacter(t)
	ctx := context.Background()
	f := FeatDefs[3] // first flag def
	if !f.Flag {
		t.Fatalf("def not a flag: %+v", f)
	}
	done := false
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Flag(ctx, tx, char, f.ID, f.Threshold-1, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if done {
		t.Fatal("flag below threshold")
	}
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Flag(ctx, tx, char, f.ID, f.Threshold, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if !done {
		t.Fatal("flag milestone")
	}
	txOf(t, func(tx pgx.Tx) error {
		var e error
		done, e = s.Flag(ctx, tx, char, f.ID, f.Threshold, id.NewV4(),
			time.Now().UTC())
		return e
	})
	if done {
		t.Fatal("flag re-latched")
	}
}

// TestGuildGrantAndEquip — 656: role + expected_revision gates,
// category match, revision bump on real change only; repeated unlock
// retains the original grant audit.
func TestGuildGrantAndEquip(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, leader := mkCharacter(t)
	_, member := mkCharacter(t)
	gid := mkGuild(t, leader, "LEADER")
	mid := id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO guild_memberships
		    (character_id, guild_id, role, joined_at, membership_id)
		 VALUES ($1,$2,'MEMBER',now(),$3)`,
		member[:], gid[:], mid[:]); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cid := "cosmetic.guild.banner.guild_war_champion"
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildGrant(ctx, tx, gid, cid, "guild_war.win",
			id.NewV4(), time.Now().UTC())
	})
	// Repeat unlock: no-op, single row retained.
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildGrant(ctx, tx, gid, cid, "guild_war.win2",
			id.NewV4(), time.Now().UTC())
	})
	var n int
	if err := sharedPool.QueryRow(ctx,
		`SELECT count(*) FROM guild_cosmetic_entitlements
		 WHERE guild_id = $1 AND cosmetic_id = $2`, gid[:], cid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("duplicate unlock rows: %d", n)
	}

	equip := func(char id.UUID, rev uint64, cosmeticID string) error {
		return txErr(func(tx pgx.Tx) error {
			return s.GuildEquip(ctx, tx, char, gid, "BANNER",
				cosmeticID, rev)
		})
	}
	equipOf := func(char id.UUID, rev uint64, cosmeticID string) error {
		return txErr(func(tx pgx.Tx) error {
			return s.GuildEquip(ctx, tx, char, gid, "BANNER",
				cosmeticID, rev)
		})
	}
	if err := equip(member, 0, cid); !errors.Is(err, ErrPermission) {
		t.Fatalf("member equip: %v", err)
	}
	if err := equip(leader, 7, cid); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildEquip(ctx, tx, leader, gid, "BANNER", cid, 0)
	})
	_ = equipOf
	v, err := s.GuildCosmeticState(ctx, nil, gid)
	if err != nil {
		t.Fatal(err)
	}
	if v.Revision != 1 || v.Banner != cid || len(v.Owned) != 1 {
		t.Fatalf("view: %+v", v)
	}
	// Same selection → no revision bump.
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildEquip(ctx, tx, leader, gid, "BANNER", cid, 1)
	})
	v, _ = s.GuildCosmeticState(ctx, nil, gid)
	if v.Revision != 1 {
		t.Fatalf("revision moved on no-op: %d", v.Revision)
	}
	// Unequip bumps again.
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildEquip(ctx, tx, leader, gid, "BANNER", "", 1)
	})
	v, _ = s.GuildCosmeticState(ctx, nil, gid)
	if v.Revision != 2 || v.Banner != "" {
		t.Fatalf("view after unequip: %+v", v)
	}
}

// TestGuildEquipWrongCategoryAndLocked — locked id and wrong-category
// id both reject before writing.
func TestGuildEquipWrongCategoryAndLocked(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, leader := mkCharacter(t)
	gid := mkGuild(t, leader, "LEADER")
	ctx := context.Background()
	equip := func(cosmeticID string) error {
		return txErr(func(tx pgx.Tx) error {
			return s.GuildEquip(ctx, tx, leader, gid, "SHRINE",
				cosmeticID, 0)
		})
	}
	if err := equip("cosmetic.guild.banner.guild_war_champion"); !errors.Is(err, ErrGuildCosmeticLocked) && !errors.Is(err, ErrWrongCategory) {
		t.Fatalf("locked/wrong cat: %v", err)
	}
	txOf(t, func(tx pgx.Tx) error {
		return s.GuildGrant(ctx, tx, gid,
			"cosmetic.guild.banner.guild_war_champion", "src", id.NewV4(),
			time.Now().UTC())
	})
	if err := equip("cosmetic.guild.banner.guild_war_champion"); !errors.Is(err, ErrWrongCategory) {
		t.Fatalf("banner into SHRINE: %v", err)
	}
}
