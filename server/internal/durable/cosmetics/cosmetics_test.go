package cosmetics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestCharacterCosmeticOwnershipAnyRow — ownership while ANY grant
// row exists for the character or the account (ADR-0060).
func TestCharacterCosmeticOwnershipAnyRow(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	cid := "cosmetic.title.lang_da"

	owned, err := s.Owned(ctx, nil, acct, char, cid)
	if err != nil || owned {
		t.Fatalf("pre-state: owned=%v err=%v", owned, err)
	}
	grant(t, char, cid, "feat.x")
	owned, err = s.Owned(ctx, nil, acct, char, cid)
	if err != nil || !owned {
		t.Fatalf("one row confers ownership: owned=%v err=%v", owned, err)
	}
	grant(t, char, cid, "other.src") // second source row
	txOf(t, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM character_cosmetic_entitlements
			 WHERE character_id = $1 AND cosmetic_id = $2 AND source_ref = 'feat.x'`,
			char[:], cid)
		return err
	})
	owned, err = s.Owned(ctx, nil, acct, char, cid)
	if err != nil || !owned {
		t.Fatalf("surviving row keeps ownership: owned=%v err=%v", owned, err)
	}
}

// TestPlayEarnedCharacterCosmetics — play grants are CHARACTER scoped
// and populate the 438 view.
func TestPlayEarnedCharacterCosmetics(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	grant(t, char, "cosmetic.title.lang_da", "feat.combat.slay_ma_da")

	st, err := s.StateFor(ctx, nil, char)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetOwned()) != 1 ||
		st.GetOwned()[0].GetScope() != protocolv1.CosmeticScope_COSMETIC_SCOPE_CHARACTER {
		t.Fatalf("owned: %+v", st.GetOwned())
	}
	// A different character on the same account must NOT see it.
	_, other := mkCharacter(t)
	owned, err := s.Owned(ctx, nil, acct, other, "cosmetic.title.lang_da")
	if err != nil || owned {
		t.Fatalf("cross-character leak: %v", owned)
	}
}

// TestIAPAccountEntitledWardrobe — account-entitled cosmetics equip
// on any account character; the first equip stamps first_equipped_at
// exactly once.
func TestIAPAccountEntitledWardrobe(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	mkIAPGrant(t, acct, "cosmetic.iap.frame.thien_long")

	st, err := s.StateFor(ctx, nil, char)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetOwned()) != 1 ||
		st.GetOwned()[0].GetScope() != protocolv1.CosmeticScope_COSMETIC_SCOPE_ACCOUNT {
		t.Fatalf("owned: %+v", st.GetOwned())
	}
	txOf(t, func(tx pgx.Tx) error {
		return s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME,
			"cosmetic.iap.frame.thien_long")
	})
	var first bool
	if err := sharedPool.QueryRow(ctx,
		`SELECT first_equipped_at IS NOT NULL
		 FROM account_cosmetic_entitlements
		 WHERE account_id = $1 AND cosmetic_id = $2`,
		acct[:], "cosmetic.iap.frame.thien_long").Scan(&first); err != nil {
		t.Fatal(err)
	}
	if !first {
		t.Fatal("first_equipped_at not set")
	}
	// Unequip + re-equip: write-once stays a single stamp.
	txOf(t, func(tx pgx.Tx) error {
		if err := s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME, ""); err != nil {
			return err
		}
		return s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME,
			"cosmetic.iap.frame.thien_long")
	})
	var n int
	if err := sharedPool.QueryRow(ctx,
		`SELECT count(*) FROM account_cosmetic_entitlements
		 WHERE account_id = $1 AND cosmetic_id = $2
		   AND first_equipped_at IS NOT NULL`,
		acct[:], "cosmetic.iap.frame.thien_long").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("first_equipped_at rows: %d", n)
	}
}

// TestCosmeticEquipValidation — slot/category mismatch, unowned,
// unknown id; valid equip round-trips through the 438 view.
func TestCosmeticEquipValidation(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, char := mkCharacter(t)
	ctx := context.Background()
	grant(t, char, "cosmetic.title.lang_da", "src.a")

	try := func(slot protocolv1.CosmeticSlot, cid string) error {
		return txErr(func(tx pgx.Tx) error {
			return s.Equip(ctx, tx, char, slot, cid)
		})
	}
	if err := try(protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME,
		"cosmetic.title.lang_da"); err == nil {
		t.Fatal("title equipped into frame slot accepted")
	}
	if err := try(protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE,
		"cosmetic.title.u_minh"); err == nil {
		t.Fatal("unowned cosmetic accepted")
	}
	if err := try(protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE,
		"cosmetic.title.none"); err == nil {
		t.Fatal("unknown cosmetic accepted")
	}
	txOf(t, func(tx pgx.Tx) error {
		if err := s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE,
			"cosmetic.title.lang_da"); err != nil {
			return err
		}
		return nil
	})
	st, err := s.StateFor(ctx, nil, char)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetEquipped()) != 1 ||
		st.GetEquipped()[0].GetCosmeticId() != "cosmetic.title.lang_da" {
		t.Fatalf("equipped: %+v", st.GetEquipped())
	}
}

// TestFirstEquippedAtSetOnce — dedicated write-once assertion (IAP
// cosmetic equipped twice keeps a single non-null stamp).
func TestFirstEquippedAtSetOnce(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	mkIAPGrant(t, acct, "cosmetic.iap.frame.thien_long")

	txOf(t, func(tx pgx.Tx) error {
		return s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME,
			"cosmetic.iap.frame.thien_long")
	})
	var v1 interface{}
	if err := sharedPool.QueryRow(ctx,
		`SELECT first_equipped_at FROM account_cosmetic_entitlements
		 WHERE account_id = $1 AND cosmetic_id = $2`,
		acct[:], "cosmetic.iap.frame.thien_long").Scan(&v1); err != nil {
		t.Fatal(err)
	}
	if v1 == nil {
		t.Fatal("not stamped")
	}
	// Second character + second equip attempt: the row already
	// carries the stamp and must not move.
	txOf(t, func(tx pgx.Tx) error {
		return s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME,
			"cosmetic.iap.frame.thien_long")
	})
	var v2 interface{}
	if err := sharedPool.QueryRow(ctx,
		`SELECT first_equipped_at FROM account_cosmetic_entitlements
		 WHERE account_id = $1 AND cosmetic_id = $2`,
		acct[:], "cosmetic.iap.frame.thien_long").Scan(&v2); err != nil {
		t.Fatal(err)
	}
	if v2 != v1 {
		t.Fatalf("first_equipped_at moved: %v -> %v", v1, v2)
	}
}

// TestGuildStoneInscriptionSlot — the guild_stone_inscription slot
// equips a character-scoped shrine inscription cosmetic and falls
// back to none when the entitlement is removed.
func TestGuildStoneInscriptionSlot(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	_, char := mkCharacter(t)
	ctx := context.Background()
	cid := "cosmetic.guild_stone.inscription.trung_nguyen"
	grant(t, char, cid, "src.shrine")

	txOf(t, func(tx pgx.Tx) error {
		return s.Equip(ctx, tx, char,
			protocolv1.CosmeticSlot_COSMETIC_SLOT_GUILD_STONE_INSCRIPTION,
			cid)
	})
	st, err := s.StateFor(ctx, nil, char)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetEquipped()) != 1 {
		t.Fatalf("equipped: %+v", st.GetEquipped())
	}
	// Remove the entitlement → equip row falls back to none.
	txOf(t, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM character_cosmetic_entitlements
			 WHERE character_id = $1 AND cosmetic_id = $2`, char[:], cid)
		return err
	})
	st, err = s.StateFor(ctx, nil, char)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetEquipped()) != 0 {
		t.Fatalf("fallback should hide unowned equip: %+v", st.GetEquipped())
	}
}

// TestLaunchCosmeticCounts294 — compiled catalog assertions.
func TestLaunchCosmeticCounts294(t *testing.T) {
	roster := Roster()
	if len(roster) != 294 {
		t.Fatalf("total: %d", len(roster))
	}
	byCat := map[Category]int{}
	byScope := map[Scope]int{}
	for _, d := range roster {
		byCat[d.Category]++
		byScope[d.Scope]++
	}
	if byScope[ScopeGuild] != 5 {
		t.Fatalf("guild: %d", byScope[ScopeGuild])
	}
	if byScope[ScopeAccount] != 13 {
		t.Fatalf("iap: %d", byScope[ScopeAccount])
	}
	play, seasonal := 0, 0
	for _, d := range roster {
		if d.Category != CategoryTitle {
			continue
		}
		if strings.Contains(d.ID, ".season.") {
			seasonal++
		} else {
			play++
		}
	}
	if play != 127 {
		t.Fatalf("play titles: %d", play)
	}
	if seasonal != 72 { // 60 atlas-page + 6 free + 6 paid season titles
		t.Fatalf("seasonal titles: %d", seasonal)
	}
	// ProfileFrame is the frame category across play, sinks, seasons
	// and IAP — assert the play-earned subset (no ".season.", no
	// "cosmetic.iap.", no special-sink route-only defs) is 9.
	playFrames := 0
	for _, d := range roster {
		if d.Category != CategoryProfileFrame ||
			strings.Contains(d.ID, ".season.") ||
			strings.HasPrefix(d.ID, "cosmetic.iap.") {
			continue
		}
		r, hasRoute := RoutesFor(d.ID)
		if hasRoute && r.SpecialAmount > 0 && r.MaterialItem == "" &&
			d.ID != "cosmetic.frame.nui_thieng" {
			continue // special-currency sink frame
		}
		playFrames++
	}
	if playFrames != 9 {
		t.Fatalf("play frames: %d", playFrames)
	}
	// Dual-route cosmetics must expose both material and special.
	r, ok := RoutesFor("cosmetic.frame.nui_thieng")
	if !ok || r.MaterialQty != 30 || r.SpecialAmount != 20 {
		t.Fatalf("nui_thieng routes: %+v ok=%v", r, ok)
	}
}

// TestMultiSourceOwnershipSurvivesOneRevoke — removing one grant
// source leaves ownership intact while another row exists.
func TestMultiSourceOwnershipSurvivesOneRevoke(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	cid := "cosmetic.title.lang_da"
	grant(t, char, cid, "src.a")
	grant(t, char, cid, "src.b")

	txOf(t, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM character_cosmetic_entitlements
			 WHERE character_id = $1 AND cosmetic_id = $2 AND source_ref = 'src.a'`,
			char[:], cid)
		return err
	})
	owned, err := s.Owned(ctx, nil, acct, char, cid)
	if err != nil || !owned {
		t.Fatalf("one revoke killed ownership: %v", owned)
	}
}

// TestSeasonTrackRevokeBySource — revoking one season-track
// entitlement deletes only that entitlement's grant rows.
func TestSeasonTrackRevokeBySource(t *testing.T) {
	wipe(t)
	s := NewStore(requirePool(t))
	acct, char := mkCharacter(t)
	ctx := context.Background()
	trk := id.NewV4()
	mkIAPRow(t, acct, trk)

	txOf(t, func(tx pgx.Tx) error {
		if err := s.Grant(ctx, tx, char, "cosmetic.title.lang_da",
			SourceSeasonTrack, "season.track.a", &trk, id.NewV4(),
			time.Now().UTC()); err != nil {
			return err
		}
		if err := s.Grant(ctx, tx, char, "cosmetic.title.u_minh",
			SourceSeasonTrack, "season.track.b", &trk, id.NewV4(),
			time.Now().UTC()); err != nil {
			return err
		}
		return s.Grant(ctx, tx, char, "cosmetic.title.lang_da",
			SourcePlay, "feat.x", nil, id.NewV4(), time.Now().UTC())
	})
	txOf(t, func(tx pgx.Tx) error {
		n, err := s.RevokeBySourceEntitlement(ctx, tx, trk)
		if err != nil {
			return err
		}
		if n != 2 {
			t.Fatalf("revoked %d rows", n)
		}
		return nil
	})
	// dao_cam survives via the unrelated play row; bo_kieu is gone.
	owned, _ := s.Owned(ctx, nil, acct, char, "cosmetic.title.lang_da")
	if !owned {
		t.Fatal("play row revoked by season track")
	}
	owned, _ = s.Owned(ctx, nil, acct, char, "cosmetic.title.u_minh")
	if owned {
		t.Fatal("track-granted cosmetic survived")
	}
}
