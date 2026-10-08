package travel

import (
	"testing"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/world"
)

// anchors mirrors the six authored safe anchors (world_route_catalog.md
// § Checkpoints) for handler injection.
func testAnchorFor(dest string) (string, bool) {
	anchor := map[string]string{
		"map.lang_da.dinh_lang":    "checkpoint.lang_da.dinh_lang",
		"map.rung_u_minh.xom_rung": "checkpoint.rung_u_minh.xom_rung",
		"map.ben_nuoc_den.cho_ben": "checkpoint.ben_nuoc_den.cho_ben",
		"map.deo_may.ban_chan_deo": "checkpoint.deo_may.ban_chan_deo",
		"map.thanh_co.cong_ngoai":  "checkpoint.thanh_co.cong_ngoai",
		"map.nui_thieng.chan_nui":  "checkpoint.nui_thieng.chan_nui",
	}[dest]
	return anchor, anchor != ""
}

func TestTravelRequiresDiscovery(t *testing.T) {
	d := world.NewInteractDispatcher()
	Register(d)
	if !d.ServiceRegistered(ServiceID) {
		t.Fatal("travel must register on the dispatcher")
	}

	char := id.NewV4()
	discovered := map[string]bool{"map.ben_nuoc_den.cho_ben": true}
	h := Handler{
		Anchors: testAnchorFor,
		Discovered: func(c id.UUID, dest string) bool {
			return c == char && discovered[dest]
		},
	}

	if _, code := h.Destination(char, "map.nui_thieng.chan_nui"); code != protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED {
		t.Fatalf("undiscovered anchor: code = %v, want NOT_DISCOVERED", code)
	}
	res, code := h.Destination(char, "map.ben_nuoc_den.cho_ben")
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("discovered anchor: code = %v", code)
	}
	if res.MapID != "map.ben_nuoc_den.cho_ben" ||
		res.SpawnAnchorID != "checkpoint.ben_nuoc_den.cho_ben" {
		t.Fatalf("placement = %+v", res)
	}
	if _, code := h.Destination(char, "map.lang_da.bo_ruong"); code != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("non-anchor destination: code = %v, want TARGET_INVALID", code)
	}
}

func TestTravelFeeOncePerOperation(t *testing.T) {
	want := map[string]int64{
		"map.lang_da.dinh_lang":    0,
		"map.rung_u_minh.xom_rung": 100,
		"map.ben_nuoc_den.cho_ben": 200,
		"map.deo_may.ban_chan_deo": 350,
		"map.thanh_co.cong_ngoai":  550,
		"map.nui_thieng.chan_nui":  800,
	}
	if len(want) != len(anchorTier) {
		t.Fatalf("fee table covers %d anchors, want %d", len(anchorTier), len(want))
	}
	for dest, fee := range want {
		got, ok := FeeFor(dest)
		if !ok || got != fee {
			t.Fatalf("FeeFor(%s) = %d,%v; want %d,true", dest, got, ok, fee)
		}
	}
	// Deterministic per destination: the same operation resolves the
	// same fee on every evaluation (one charge per operation_id).
	for i := 0; i < 3; i++ {
		if fee, _ := FeeFor("map.deo_may.ban_chan_deo"); fee != 350 {
			t.Fatalf("reeval %d changed the fee to %d", i, fee)
		}
	}
	// Returning to map.lang_da.dinh_lang always costs 0.
	if fee, _ := FeeFor("map.lang_da.dinh_lang"); fee != 0 {
		t.Fatalf("return home fee = %d, want 0", fee)
	}
	if _, ok := FeeFor("map.lang_da.bo_ruong"); ok {
		t.Fatal("FIELD map must not resolve a fee")
	}
}

func TestTravelStartsTransfer(t *testing.T) {
	char := id.NewV4()
	h := Handler{
		Anchors:    testAnchorFor,
		Discovered: func(id.UUID, string) bool { return true },
	}
	res, code := h.Destination(char, "map.thanh_co.cong_ngoai")
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatalf("admission: code = %v", code)
	}
	// The resolved placement is what the post-commit 105 TRAVEL
	// transfer start consumes: destination map, auto order, entry
	// spawn pinned to the safe anchor.
	if res.Kind != world.PlaceAuto || res.MapID != "map.thanh_co.cong_ngoai" {
		t.Fatalf("placement = %+v", res)
	}
	if res.SpawnAnchorID == "" {
		t.Fatal("travel placement must pin the destination entry spawn")
	}
	// No admission when discovery is missing: the transfer never
	// starts for an undiscovered anchor.
	h.Discovered = func(id.UUID, string) bool { return false }
	if _, code := h.Destination(char, "map.thanh_co.cong_ngoai"); code != protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED {
		t.Fatalf("blocked admission: code = %v, want NOT_DISCOVERED", code)
	}
}
