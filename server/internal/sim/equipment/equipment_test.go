package equipment

import (
	"errors"
	"testing"
	"time"

	"thinhthan/internal/core/id"
)

// TestFourteenEquipmentSlots: the canonical roster is exactly the 14
// ordered slots — 8 BASIC + 6 ADVANCED — and LoadoutIndex/LoadoutID map
// the closed 3-loadout roster both ways (equipment.md invariants).
func TestFourteenEquipmentSlots(t *testing.T) {
	want := [14]string{"weapon", "head", "body", "hands", "legs", "feet",
		"necklace", "ring", "costume", "talisman", "jade", "seal", "relic", "charm"}
	if Slots != want {
		t.Fatalf("slots %v", Slots)
	}
	for i, s := range Slots {
		if !IsSlot(s) {
			t.Fatalf("slot %d %q not registered", i, s)
		}
	}
	for _, bad := range []string{"", "weapon2", "Weapon", "mount", "loadout.primary.weapon"} {
		if IsSlot(bad) {
			t.Fatalf("bogus slot %q accepted", bad)
		}
	}
}

// TestThreeLoadoutSwitching: exactly 3 loadouts, closed wire ids, and
// SWITCH_ACTIVE gates — 3 s success cooldown, in-combat/dead/
// transferring rejects — per equipment.md § Loadout Switch.
func TestThreeLoadoutSwitching(t *testing.T) {
	if LoadoutCount != 3 {
		t.Fatalf("LoadoutCount %d", LoadoutCount)
	}
	for i, want := range []string{"loadout.primary", "loadout.secondary_1", "loadout.secondary_2"} {
		got, ok := LoadoutID(i + 1)
		if !ok || got != want {
			t.Fatalf("index %d -> %q,%v", i+1, got, ok)
		}
		back, ok := LoadoutIndex(want)
		if !ok || back != i+1 {
			t.Fatalf("id %q -> %d,%v", want, back, ok)
		}
	}
	if _, ok := LoadoutID(0); ok {
		t.Fatal("index 0 must not resolve")
	}
	if _, ok := LoadoutID(4); ok {
		t.Fatal("index 4 must not resolve")
	}
	if _, ok := LoadoutIndex("loadout.tertiary"); ok {
		t.Fatal("unknown loadout id must not resolve")
	}

	// Gates: combat/dead/transferring rejects; cooldown 3 s.
	if err := CheckSwitch(GateState{InCombat: true, Alive: true}); !errors.Is(err, RejectInCombat) {
		t.Fatalf("in-combat switch: %v", err)
	}
	if err := CheckSwitch(GateState{Alive: false}); !errors.Is(err, RejectStateConflict) {
		t.Fatalf("dead switch: %v", err)
	}
	if err := CheckSwitch(GateState{Alive: true, Transferring: true}); !errors.Is(err, RejectStateConflict) {
		t.Fatalf("transferring switch: %v", err)
	}
	if err := CheckMutation(GateState{InCombat: true}); !errors.Is(err, RejectInCombat) {
		t.Fatalf("in-combat equip: %v", err)
	}

	now := time.Unix(1_700_000_000, 0)
	clk := now
	tr := NewSwitchTracker(func() time.Time { return clk })
	c := id.NewV4()
	if !tr.Ready(c) {
		t.Fatal("first switch must be ready")
	}
	tr.RecordSuccess(c)
	if tr.Ready(c) {
		t.Fatal("cooldown must hold after success")
	}
	clk = clk.Add(3 * time.Second)
	if !tr.Ready(c) || tr.ReadyIn(c) != 0 {
		t.Fatal("cooldown must expire at exactly 3 s")
	}
	// Other characters are independent.
	if !tr.Ready(id.NewV4()) {
		t.Fatal("cooldown leaked across characters")
	}
}

// TestEnhancementSuccessCurve: the crafting.md base-rate table, the
// milestone floors, the failure rule and the 9500 bp clamp.
func TestEnhancementSuccessCurve(t *testing.T) {
	wantBP := [16]int{10000, 10000, 8500, 7000, 5500, 4500, 3500, 2500,
		2000, 1500, 1000, 800, 600, 400, 300, 200}
	for cur, want := range wantBP {
		if got := SuccessRateBP(cur); got != want {
			t.Fatalf("+%d->+%d = %d want %d", cur, cur+1, got, want)
		}
	}
	if SuccessRateBP(16) != 0 {
		t.Fatal("+16 has no further attempt")
	}
	for lvl, floor := range map[int]int{0: 0, 3: 0, 4: 4, 7: 4, 8: 8, 11: 8, 12: 12, 15: 12, 16: 16} {
		if Floor(lvl) != floor {
			t.Fatalf("floor(%d) = %d want %d", lvl, Floor(lvl), floor)
		}
	}
	// Failure: drop 1 clamped at floor.
	if FailLevel(13, false) != 12 || FailLevel(12, false) != 12 ||
		FailLevel(5, false) != 4 || FailLevel(1, false) != 0 {
		t.Fatal("uninsured failure rule violated")
	}
	if FailLevel(13, true) != 13 {
		t.Fatal("insurance must preserve level")
	}
	// Clamp: 0->1 base 10000 + bonus still caps at 9500.
	if FinalRateBP(0, 500, 500) != 9500 {
		t.Fatal("rate clamp 9500 violated")
	}
	if FinalRateBP(15, 0, 100) != 300 {
		t.Fatalf("+15->+16 with charm = %d want 300", FinalRateBP(15, 0, 100))
	}
}

// TestLuckyCharmProtection: exactly the 4 catalog charms, their bonuses
// and level-eligibility ceilings; ineligible = CHARM_INELIGIBLE upstream.
func TestLuckyCharmProtection(t *testing.T) {
	cases := []struct {
		id     string
		bonus  int
		maxCur int // highest current level still eligible (-1 = all)
	}{
		{"item.consumable.bua_may.so_cap", 500, 7},
		{"item.consumable.bua_may.trung_cap", 300, 11},
		{"item.consumable.bua_may.cao_cap", 100, -1},
		{"item.consumable.bua_may.sieu_cap", 300, -1},
	}
	for _, c := range cases {
		bp, ok := CharmBonusBP(c.id, 0)
		if !ok || bp != c.bonus {
			t.Fatalf("%s at +0: bp=%d ok=%v", c.id, bp, ok)
		}
		if c.maxCur >= 0 {
			if _, ok := CharmBonusBP(c.id, c.maxCur); !ok {
				t.Fatalf("%s must be eligible at +%d", c.id, c.maxCur)
			}
			if _, ok := CharmBonusBP(c.id, c.maxCur+1); ok {
				t.Fatalf("%s must be ineligible at +%d", c.id, c.maxCur+1)
			}
		} else if _, ok := CharmBonusBP(c.id, 15); !ok {
			t.Fatalf("%s must be eligible at +15", c.id)
		}
	}
	if _, ok := CharmBonusBP("item.consumable.bua_may.nonexistent", 0); ok {
		t.Fatal("unknown charm must be ineligible")
	}
	for _, c := range []struct {
		id     string
		maxCur int
	}{
		{"item.consumable.bua_giu_bac.so_cap", 7},
		{"item.consumable.bua_giu_bac.trung_cap", 11},
		{"item.consumable.bua_giu_bac.cao_cap", -1},
	} {
		if !InsuranceEligible(c.id, 0) {
			t.Fatalf("%s must be eligible at +0", c.id)
		}
		if c.maxCur >= 0 && InsuranceEligible(c.id, c.maxCur+1) {
			t.Fatalf("%s must be ineligible at +%d", c.id, c.maxCur+1)
		}
	}
}
