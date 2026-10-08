package travel

// fee.go — the destination-tier common-currency fee table
// (npc_shop_catalog.md § Travel): T1 0, T2 100, T3 200, T4 350, T5 550,
// T6 800; returning to map.lang_da.dinh_lang always costs 0 — it is the
// tier-1 anchor, so the table encodes that directly.

// tierCost indexes cost by region tier 1..6.
var tierCost = [7]int64{0, 0, 100, 200, 350, 550, 800}

// anchorTier binds each travel-eligible safe/social anchor map to its
// region tier, in authored progression order.
var anchorTier = map[string]int{
	"map.lang_da.dinh_lang":    1,
	"map.rung_u_minh.xom_rung": 2,
	"map.ben_nuoc_den.cho_ben": 3,
	"map.deo_may.ban_chan_deo": 4,
	"map.thanh_co.cong_ngoai":  5,
	"map.nui_thieng.chan_nui":  6,
}

// TierFor resolves a destination map's region tier (1..6). Maps outside
// the six safe anchors are not travel-eligible.
func TierFor(destMapID string) (int, bool) {
	t, ok := anchorTier[destMapID]
	return t, ok
}

// FeeFor resolves the common-currency fee for one travel operation to
// destMapID, per the tier table. The fee is charged once per
// operation_id inside the durable commit — a replay commits nothing
// extra.
func FeeFor(destMapID string) (int64, bool) {
	t, ok := anchorTier[destMapID]
	if !ok {
		return 0, false
	}
	return tierCost[t], true
}
