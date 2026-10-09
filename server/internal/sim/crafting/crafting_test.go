package crafting

import (
	"testing"

	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

// stationDeps builds a consult whose only station sits at (0,0) and
// whose recipe catalog knows min levels 1 and 31.
func stationDeps() Deps {
	return Deps{
		Station: func(npcID, service string) (geometry.Anchor, bool) {
			if npcID != "npc.thon.tho_nghe" {
				return geometry.Anchor{}, false
			}
			return geometry.Anchor{X: 0, Y: 0}, service == "crafting" ||
				service == "enhancement"
		},
		RecipeMin: func(recipeID string) (int32, bool) {
			switch recipeID {
			case "recipe.eq.t1.dinh_lang.weapon":
				return 1, true
			case "recipe.eq.t6.nui_thieng.weapon":
				return 51, true
			}
			return 0, false
		},
		NeedSlots: func(recipeID string, batch uint32) int64 {
			return int64(batch)
		},
	}
}

func entity(x, y int32) *runtime.Entity {
	e := &runtime.Entity{}
	e.Snap.X = x
	e.Snap.Y = y
	return e
}

// D-4 admission-reject suite: every gate rejects before a durable
// record is built — no partial failure leaks to settlement.
func TestCraftAdmissionRejects(t *testing.T) {
	c := New(stationDeps())

	cases := []struct {
		name      string
		entity    *runtime.Entity
		npcID     string
		recipeID  string
		batch     uint32
		level     int32
		freeSlots int64
		wantCode  string
	}{
		{"out of range", entity(10000, 0), "npc.thon.tho_nghe",
			"recipe.eq.t1.dinh_lang.weapon", 1, 10, 10, "OUT_OF_RANGE"},
		{"in combat", func() *runtime.Entity {
			e := entity(0, 0)
			e.InCombatWith = 1
			return e
		}(), "npc.thon.tho_nghe", "recipe.eq.t1.dinh_lang.weapon",
			1, 10, 10, "STATE_CONFLICT"},
		{"dead", func() *runtime.Entity {
			e := entity(0, 0)
			e.Dead = true
			return e
		}(), "npc.thon.tho_nghe", "recipe.eq.t1.dinh_lang.weapon",
			1, 10, 10, "STATE_CONFLICT"},
		{"missing entity", nil, "npc.thon.tho_nghe",
			"recipe.eq.t1.dinh_lang.weapon", 1, 10, 10, "TARGET_INVALID"},
		{"unknown station", entity(0, 0), "npc.thon.ngu_khach",
			"recipe.eq.t1.dinh_lang.weapon", 1, 10, 10, "TARGET_INVALID"},
		{"batch zero", entity(0, 0), "npc.thon.tho_nghe",
			"recipe.eq.t1.dinh_lang.weapon", 0, 10, 10, "OUT_OF_RANGE"},
		{"batch over 99", entity(0, 0), "npc.thon.tho_nghe",
			"recipe.eq.t1.dinh_lang.weapon", 100, 10, 10, "OUT_OF_RANGE"},
		{"unknown recipe", entity(0, 0), "npc.thon.tho_nghe",
			"recipe.eq.t0.nope.weapon", 1, 10, 10, "TARGET_INVALID"},
		{"level too low", entity(0, 0), "npc.thon.tho_nghe",
			"recipe.eq.t6.nui_thieng.weapon", 1, 20, 10, "LEVEL_TOO_LOW"},
		{"capacity", entity(0, 0), "npc.thon.tho_nghe",
			"recipe.eq.t1.dinh_lang.weapon", 3, 10, 2, "INVENTORY_FULL"},
	}
	for _, tc := range cases {
		got := c.AdmitCraft(tc.entity, tc.npcID, tc.recipeID, tc.batch,
			tc.level, tc.freeSlots)
		if got.OK || got.Code != tc.wantCode {
			t.Fatalf("%s: %+v want %s", tc.name, got, tc.wantCode)
		}
	}

	// Happy path: every gate green.
	if got := c.AdmitCraft(entity(0, 0), "npc.thon.tho_nghe",
		"recipe.eq.t1.dinh_lang.weapon", 5, 10, 5); got != okVerdict {
		t.Fatalf("admit ok: %+v", got)
	}
}

// AdmitEnhance shares the live + enhancement-station gates.
func TestEnhanceAdmissionRejects(t *testing.T) {
	c := New(stationDeps())
	if got := c.AdmitEnhance(entity(10000, 0), "npc.thon.tho_nghe"); got.OK ||
		got.Code != "OUT_OF_RANGE" {
		t.Fatalf("range: %+v", got)
	}
	if got := c.AdmitEnhance(entity(0, 0), "npc.thon.tho_nghe"); !got.OK {
		t.Fatalf("enhance admit: %+v", got)
	}
}
