// Package atlas persists Atlas journal progress: character_atlas page
// counters, tier promotion with atomic reward settlement (currency.special,
// presentation entitlement, LIFE_SKILL EXP), acknowledgement rows and the
// character_atlas_state revision feeding S2C_ATLAS_STATE (518).
//
// The authoritative launch roster is the compiled table below, mirrored from
// docs/07_content/atlas_catalog.md (104 launch pages + authored seasonal ids).
// Pages whose authored source verb is not one of the six unlock event kinds
// (obtain / consume / kindle) stay in the roster but resolve to EventNone;
// they appear in snapshots with zero progress.
//
// Reward settlement is idempotent on the tier triple
// atlas.tier.<character_id>.<atlas_page_id>.<tier> recorded as
// reward_operation_id; replays never re-grant.
package atlas

import "strings"

// EventKind enumerates the authoritative source events that feed Atlas pages.
// CHEST_SPOTTED and QUEST_CLUE never feed pages.
type EventKind int

const (
	EventNone EventKind = iota
	EventMonsterKilled
	EventSoulAcquired
	EventBossWitness
	EventChestOpened
	EventFishCaught
	EventDishCooked
)

// ApplyMode selects how an event value mutates the page counter.
type ApplyMode int

const (
	// ApplyCount adds the event count to the counter.
	ApplyCount ApplyMode = iota
	// ApplyMax keeps the counter at max(counter, value) (soul levels).
	ApplyMax
)

// Page is one authored Atlas page.
type Page struct {
	ID         string    // atlas.page.<family>.*  (VARCHAR(64) bound)
	Source     string    // authored source id; a ".*" suffix matches by prefix
	Kind       EventKind // resolved unlock event kind; EventNone = roster-only
	Mode       ApplyMode
	Thresholds [3]uint64 // counters at which tiers 1..3 promote
	Special    [3]int64  // currency.special granted per tier (0 for seasonal)
	Title      string    // cosmetic id granted at tier 3 ("" = none)
}

// Family returns the page family segment (quai_dam, hon_giam, di_tich,
// co_vat, season).
func (p Page) Family() string {
	parts := strings.Split(p.ID, ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

// SourcePrefix reports whether Source is a wildcard prefix (ends in ".*").
func (p Page) SourcePrefix() bool { return strings.HasSuffix(p.Source, ".*") }

// PresentationEntitlement derives the cosmetic id a tier grants: tier 3 uses
// the authored title id; tiers 1 and 2 derive illustration/frame ids from the
// page key (the catalog authors only titles; the derivation is deterministic).
func (p Page) PresentationEntitlement(tier int) string {
	key := strings.TrimPrefix(p.ID, "atlas.page.")
	key = strings.ReplaceAll(key, ".", "_")
	switch tier {
	case 1:
		return "cosmetic.illustration.atlas." + key
	case 2:
		return "cosmetic.frame.atlas." + key
	default:
		return p.Title
	}
}

// PagesForEvent returns the roster pages matching (kind, sourceID). A
// wildcard source (e.g. chest.hidden.*) matches any id with its prefix.
func PagesForEvent(kind EventKind, sourceID string) []Page {
	var out []Page
	for _, p := range catalog {
		if p.Kind != kind {
			continue
		}
		if p.SourcePrefix() {
			if strings.HasPrefix(sourceID, strings.TrimSuffix(p.Source, "*")) {
				out = append(out, p)
			}
			continue
		}
		if p.Source == sourceID {
			out = append(out, p)
		}
	}
	return out
}

// PageByID returns the authored page, ok=false when the id is not authored.
func PageByID(id string) (Page, bool) {
	for _, p := range catalog {
		if p.ID == id {
			return p, true
		}
	}
	return Page{}, false
}

// Roster returns the full authored page list (launch + seasonal template
// pages) in authored order.
func Roster() []Page { return append([]Page(nil), catalog...) }

// Milestone is a mastered-pages threshold granting a cosmetic title.
type Milestone struct {
	ID     string
	Needed int
	Title  string
}

// Milestones returns the authored completion milestones.
func Milestones() []Milestone {
	return []Milestone{
		{ID: "atlas.milestone.20", Needed: 20, Title: "cosmetic.title.atlas.nha_suu_tam"},
		{ID: "atlas.milestone.50", Needed: 50, Title: "cosmetic.title.atlas.hoc_gia_dan_gian"},
		{ID: "atlas.milestone.104", Needed: 104, Title: "cosmetic.title.atlas.bach_khoa_dan_gian"},
	}
}

// catalog is the compiled roster mirrored from atlas_catalog.md.
var catalog = []Page{
	{ID: "atlas.page.quai_dam.lang_da.dom_dom_ma", Source: "monster.lang_da.dom_dom_ma", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.dom_dom_ma"},
	{ID: "atlas.page.quai_dam.lang_da.bu_nhin_rom", Source: "monster.lang_da.bu_nhin_rom", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bu_nhin_rom"},
	{ID: "atlas.page.quai_dam.lang_da.coc_thanh_tinh", Source: "monster.lang_da.coc_thanh_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.coc_thanh_tinh"},
	{ID: "atlas.page.quai_dam.lang_da.hon_xo_non", Source: "monster.lang_da.hon_xo_non", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_xo_non"},
	{ID: "atlas.page.quai_dam.lang_da.quy_nhap_trang", Source: "monster.lang_da.quy_nhap_trang", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.quy_nhap_trang_dong"},
	{ID: "atlas.page.quai_dam.lang_da.vong_hon", Source: "monster.lang_da.vong_hon", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_hon"},
	{ID: "atlas.page.quai_dam.lang_da.hon_ma_co_thu", Source: "monster.lang_da.hon_ma_co_thu", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_co_thu"},
	{ID: "atlas.page.quai_dam.lang_da.hon_do_trang", Source: "monster.lang_da.hon_do_trang", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_do_trang"},
	{ID: "atlas.page.quai_dam.lang_da.ma_xo", Source: "monster.lang_da.ma_xo", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_xo"},
	{ID: "atlas.page.quai_dam.lang_da.vong_hon_gia", Source: "monster.lang_da.vong_hon_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_hon_gia"},
	{ID: "atlas.page.quai_dam.rung_u_minh.ma_rung", Source: "monster.rung_u_minh.ma_rung", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_rung"},
	{ID: "atlas.page.quai_dam.rung_u_minh.dom_lua", Source: "monster.rung_u_minh.dom_lua", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.dom_lua"},
	{ID: "atlas.page.quai_dam.rung_u_minh.bong_nguoi", Source: "monster.rung_u_minh.bong_nguoi", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bong_nguoi"},
	{ID: "atlas.page.quai_dam.rung_u_minh.ma_tranh", Source: "monster.rung_u_minh.ma_tranh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_tranh"},
	{ID: "atlas.page.quai_dam.rung_u_minh.tinh_cay", Source: "monster.rung_u_minh.tinh_cay", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.tinh_cay"},
	{ID: "atlas.page.quai_dam.rung_u_minh.dai_tinh_cay", Source: "monster.rung_u_minh.dai_tinh_cay", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.dai_tinh_cay"},
	{ID: "atlas.page.quai_dam.rung_u_minh.moc_tinh", Source: "monster.rung_u_minh.moc_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.moc_tinh"},
	{ID: "atlas.page.quai_dam.rung_u_minh.vong_rung_sau", Source: "monster.rung_u_minh.vong_rung_sau", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_rung_sau"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.ma_da", Source: "monster.ben_nuoc_den.ma_da", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_da"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.ca_tinh", Source: "monster.ben_nuoc_den.ca_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ca_tinh"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.quy_song_dem", Source: "monster.ben_nuoc_den.quy_song_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.quy_song_dem"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.thuong_luong", Source: "monster.ben_nuoc_den.thuong_luong", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.thuong_luong_song"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.bong_nuoc_ma", Source: "monster.ben_nuoc_den.bong_nuoc_ma", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bong_nuoc_ma"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.ma_da_gia", Source: "monster.ben_nuoc_den.ma_da_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_da_gia"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.hon_chet_duoi", Source: "monster.ben_nuoc_den.hon_chet_duoi", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_chet_duoi"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.ca_tinh_gia", Source: "monster.ben_nuoc_den.ca_tinh_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ca_tinh_gia"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.thuy_quai", Source: "monster.ben_nuoc_den.thuy_quai", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.thuy_quai"},
	{ID: "atlas.page.quai_dam.ben_nuoc_den.nguoi_song_co", Source: "monster.ben_nuoc_den.nguoi_song_co", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.nguoi_song_co"},
	{ID: "atlas.page.quai_dam.deo_may.ma_tranh", Source: "monster.deo_may.ma_tranh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_tranh_deo"},
	{ID: "atlas.page.quai_dam.deo_may.khi_nui", Source: "monster.deo_may.khi_nui", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.khi_nui"},
	{ID: "atlas.page.quai_dam.deo_may.ma_van_dem", Source: "monster.deo_may.ma_van_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_van_dem"},
	{ID: "atlas.page.quai_dam.deo_may.ho_tinh", Source: "monster.deo_may.ho_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ho_tinh_dong"},
	{ID: "atlas.page.quai_dam.deo_may.ho_con_tinh", Source: "monster.deo_may.ho_con_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ho_con_tinh"},
	{ID: "atlas.page.quai_dam.deo_may.ma_tranh_gia", Source: "monster.deo_may.ma_tranh_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_tranh_gia"},
	{ID: "atlas.page.quai_dam.deo_may.vong_rung", Source: "monster.deo_may.vong_rung", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_rung"},
	{ID: "atlas.page.quai_dam.deo_may.ho_tinh_lon", Source: "monster.deo_may.ho_tinh_lon", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ho_tinh_lon"},
	{ID: "atlas.page.quai_dam.deo_may.ho_tinh_ve", Source: "monster.deo_may.ho_tinh_ve", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ho_tinh_ve"},
	{ID: "atlas.page.quai_dam.deo_may.vong_nui_gia", Source: "monster.deo_may.vong_nui_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_nui_gia"},
	{ID: "atlas.page.quai_dam.thanh_co.tuong_da", Source: "monster.thanh_co.tuong_da", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.tuong_da"},
	{ID: "atlas.page.quai_dam.thanh_co.hon_binh", Source: "monster.thanh_co.hon_binh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_binh"},
	{ID: "atlas.page.quai_dam.thanh_co.ma_co", Source: "monster.thanh_co.ma_co", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_co"},
	{ID: "atlas.page.quai_dam.thanh_co.oan_hon_dem", Source: "monster.thanh_co.oan_hon_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.oan_hon_dem"},
	{ID: "atlas.page.quai_dam.thanh_co.thach_ve", Source: "monster.thanh_co.thach_ve", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.thach_ve"},
	{ID: "atlas.page.quai_dam.thanh_co.qua_tinh", Source: "monster.thanh_co.qua_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.qua_tinh"},
	{ID: "atlas.page.quai_dam.thanh_co.hon_tran_linh", Source: "monster.thanh_co.hon_tran_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_tran_linh"},
	{ID: "atlas.page.quai_dam.thanh_co.hon_tuong", Source: "monster.thanh_co.hon_tuong", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_tuong"},
	{ID: "atlas.page.quai_dam.thanh_co.qua_tinh_lon", Source: "monster.thanh_co.qua_tinh_lon", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.qua_tinh_lon"},
	{ID: "atlas.page.quai_dam.nui_thieng.vong_linh", Source: "monster.nui_thieng.vong_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.vong_linh"},
	{ID: "atlas.page.quai_dam.nui_thieng.tinh_thu", Source: "monster.nui_thieng.tinh_thu", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.tinh_thu"},
	{ID: "atlas.page.quai_dam.nui_thieng.than_rung_dem", Source: "monster.nui_thieng.than_rung_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.than_rung_dem"},
	{ID: "atlas.page.quai_dam.nui_thieng.ngu_tinh", Source: "monster.nui_thieng.ngu_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ngu_tinh_song"},
	{ID: "atlas.page.quai_dam.nui_thieng.ma_nui", Source: "monster.nui_thieng.ma_nui", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ma_nui"},
	{ID: "atlas.page.quai_dam.nui_thieng.than_trung", Source: "monster.nui_thieng.than_trung", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.than_trung_dong"},
	{ID: "atlas.page.quai_dam.nui_thieng.linh_ve", Source: "monster.nui_thieng.linh_ve", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.linh_ve"},
	{ID: "atlas.page.quai_dam.nui_thieng.hon_binh_co", Source: "monster.nui_thieng.hon_binh_co", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_binh_co"},
	{ID: "atlas.page.quai_dam.nui_thieng.dai_vong_linh", Source: "monster.nui_thieng.dai_vong_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.dai_vong_linh"},
	{ID: "atlas.page.quai_dam.nui_thieng.tinh_nui_gia", Source: "monster.nui_thieng.tinh_nui_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.tinh_nui_gia"},
	{ID: "atlas.page.quai_dam.nui_thieng.bong_vong", Source: "monster.nui_thieng.bong_vong", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bong_vong"},
	{ID: "atlas.page.hon_giam.coc_thanh_tinh", Source: "soul.normal.coc_thanh_tinh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_coc"},
	{ID: "atlas.page.hon_giam.ho_con_tinh", Source: "soul.normal.ho_con_tinh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ho_con"},
	{ID: "atlas.page.hon_giam.hon_binh", Source: "soul.normal.hon_binh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_binh_suu"},
	{ID: "atlas.page.hon_giam.tinh_cay", Source: "soul.normal.tinh_cay", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_tinh_cay"},
	{ID: "atlas.page.hon_giam.ma_rung", Source: "soul.normal.ma_rung", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_rung"},
	{ID: "atlas.page.hon_giam.khi_nui", Source: "soul.normal.khi_nui", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_khi_nui"},
	{ID: "atlas.page.hon_giam.ma_da", Source: "soul.normal.ma_da", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_da2"},
	{ID: "atlas.page.hon_giam.ca_tinh", Source: "soul.normal.ca_tinh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ca_tinh"},
	{ID: "atlas.page.hon_giam.hon_chet_duoi", Source: "soul.normal.hon_chet_duoi", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_chet_duoi2"},
	{ID: "atlas.page.hon_giam.dom_dom_ma", Source: "soul.normal.dom_dom_ma", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_dom_dom"},
	{ID: "atlas.page.hon_giam.dom_lua", Source: "soul.normal.dom_lua", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_dom_lua"},
	{ID: "atlas.page.hon_giam.qua_tinh", Source: "soul.normal.qua_tinh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_qua_tinh"},
	{ID: "atlas.page.hon_giam.bu_nhin_rom", Source: "soul.normal.bu_nhin_rom", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_bu_nhin"},
	{ID: "atlas.page.hon_giam.vong_hon", Source: "soul.normal.vong_hon", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_vong_hon2"},
	{ID: "atlas.page.hon_giam.ma_co", Source: "soul.normal.ma_co", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_co2"},
	{ID: "atlas.page.hon_giam.ho_tinh_ve", Source: "soul.elite.ho_tinh_ve", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ho_tinh_ve"},
	{ID: "atlas.page.hon_giam.thach_ve", Source: "soul.elite.thach_ve", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_thach_ve"},
	{ID: "atlas.page.hon_giam.moc_tinh", Source: "soul.elite.moc_tinh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_moc_tinh"},
	{ID: "atlas.page.hon_giam.ma_tranh", Source: "soul.elite.ma_tranh", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_tranh2"},
	{ID: "atlas.page.hon_giam.ma_da_gia", Source: "soul.elite.ma_da_gia", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_da_gia2"},
	{ID: "atlas.page.hon_giam.ma_xo", Source: "soul.elite.ma_xo", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_xo2"},
	{ID: "atlas.page.hon_giam.ma_tranh_gia", Source: "soul.elite.ma_tranh_gia", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ma_tranh_gia2"},
	{ID: "atlas.page.hon_giam.thuong_luong", Source: "soul.boss.thuong_luong", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_thuong_luong2"},
	{ID: "atlas.page.hon_giam.ho_tinh_chin_duoi", Source: "soul.boss.ho_tinh_chin_duoi", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_ho_tinh_chin"},
	{ID: "atlas.page.hon_giam.than_trung", Source: "soul.boss.than_trung", Kind: EventSoulAcquired, Mode: ApplyMax, Thresholds: [3]uint64{1, 3, 5}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.hon_than_trung2"},
	{ID: "atlas.page.di_tich.quy_nhap_trang", Source: "boss.quy_nhap_trang", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_quy"},
	{ID: "atlas.page.di_tich.moc_tinh_da", Source: "boss.moc_tinh_da", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_moc"},
	{ID: "atlas.page.di_tich.thuong_luong", Source: "boss.thuong_luong", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_thuong"},
	{ID: "atlas.page.di_tich.ma_da_chua", Source: "boss.ma_da_chua", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_ma_da_chua"},
	{ID: "atlas.page.di_tich.ho_tinh", Source: "boss.ho_tinh", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_ho_tinh"},
	{ID: "atlas.page.di_tich.ho_tinh_chin_duoi", Source: "boss.ho_tinh_chin_duoi", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_ho_chin"},
	{ID: "atlas.page.di_tich.ngu_tinh", Source: "boss.ngu_tinh", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_ngu_tinh"},
	{ID: "atlas.page.di_tich.than_trung", Source: "boss.than_trung", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.di_than_trung"},
	{ID: "atlas.page.co_vat.ruong_co_01", Source: "chest.hidden.*", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 36}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.kho_bau"},
	{ID: "atlas.page.co_vat.chia_khoa_co", Source: "item.consumable.chia_khoa_co", Kind: EventNone, Mode: ApplyCount, Thresholds: [3]uint64{1, 20, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.chia_khoa"},
	{ID: "atlas.page.co_vat.ruou_nep", Source: "item.consumable.ruou_nep", Kind: EventNone, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ruou_nep"},
	{ID: "atlas.page.co_vat.cui_lua_trai", Source: "item.material.cui_lua_trai", Kind: EventNone, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.cui_lua"},
	{ID: "atlas.page.co_vat.can_cau_tre", Source: "item.tool.can_cau_tre", Kind: EventNone, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.can_cau"},
	{ID: "atlas.page.co_vat.ca_bong", Source: "item.material.ca_bong", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ca_bong2"},
	{ID: "atlas.page.co_vat.ca_chep", Source: "item.material.ca_chep", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ca_chep2"},
	{ID: "atlas.page.co_vat.tom_song", Source: "item.material.tom_song", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.tom_song2"},
	{ID: "atlas.page.co_vat.ca_chep_hoa_rong", Source: "item.material.ca_chep_hoa_rong", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.ca_chep_hoa_rong"},
	{ID: "atlas.page.co_vat.ca_bong_kho", Source: "item.consumable.food.ca_bong_kho", Kind: EventDishCooked, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bep_bong"},
	{ID: "atlas.page.co_vat.ca_chep_nuong", Source: "item.consumable.food.ca_chep_nuong", Kind: EventDishCooked, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bep_chep"},
	{ID: "atlas.page.co_vat.tom_nuong", Source: "item.consumable.food.tom_nuong", Kind: EventDishCooked, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.bep_tom"},
	{ID: "atlas.page.co_vat.linh_dan", Source: "item.material.linh_dan.*", Kind: EventNone, Mode: ApplyCount, Thresholds: [3]uint64{1, 50, 200}, Special: [3]int64{1, 2, 2}, Title: "cosmetic.title.atlas.linh_dan2"},
	{ID: "atlas.page.season.0.lang_da.dom_dom_nguyen", Source: "monster.lang_da.dom_dom_nguyen", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.dom_nguyen"},
	{ID: "atlas.page.season.0.lang_da.hon_gao", Source: "monster.lang_da.hon_gao", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.hon_gao"},
	{ID: "atlas.page.season.0.lang_da.bup_lua", Source: "monster.lang_da.bup_lua", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.bup_lua"},
	{ID: "atlas.page.season.0.lang_da.vong_bien", Source: "monster.lang_da.vong_bien", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.vong_bien"},
	{ID: "atlas.page.season.0.lang_da.tinh_buoi", Source: "monster.lang_da.tinh_buoi", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.tinh_buoi"},
	{ID: "atlas.page.season.0.lang_da.co_lua", Source: "monster.lang_da.co_lua", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.co_lua"},
	{ID: "atlas.page.season.0.lang_da.ky_xuan", Source: "chest.hidden.season.0.lang_da.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.ky_xuan"},
	{ID: "atlas.page.season.0.lang_da.ca_linh_giang", Source: "item.material.ca_linh_giang", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.ca_linh"},
	{ID: "atlas.page.season.0.lang_da.di_tich_quy_xuan", Source: "relic.season.0.quy_xuan", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.di_quy_xuan"},
	{ID: "atlas.page.season.0.lang_da.di_tich_hon_dau", Source: "relic.season.0.hon_dau", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.0.di_hon_dau"},
	{ID: "atlas.page.season.1.u_minh.ma_rung", Source: "monster.rung_u_minh.ma_rung", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.ma_rung"},
	{ID: "atlas.page.season.1.u_minh.dom_lua", Source: "monster.rung_u_minh.dom_lua", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.dom_lua"},
	{ID: "atlas.page.season.1.u_minh.bong_nguoi", Source: "monster.rung_u_minh.bong_nguoi", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.bong_nguoi"},
	{ID: "atlas.page.season.1.u_minh.tinh_cay", Source: "monster.rung_u_minh.tinh_cay", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.tinh_cay"},
	{ID: "atlas.page.season.1.u_minh.dai_tinh_cay", Source: "monster.rung_u_minh.dai_tinh_cay", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.dai_tinh_cay"},
	{ID: "atlas.page.season.1.u_minh.vong_rung_sau", Source: "monster.rung_u_minh.vong_rung_sau", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.vong_rung_sau"},
	{ID: "atlas.page.season.1.u_minh.ky_ram", Source: "chest.hidden.season.1.u_minh.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.ky_ram"},
	{ID: "atlas.page.season.1.u_minh.ca_sam", Source: "item.material.ca_sam_u_minh", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.ca_sam"},
	{ID: "atlas.page.season.1.u_minh.di_mieu", Source: "relic.season.1.moc_mieu", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.di_mieu"},
	{ID: "atlas.page.season.1.u_minh.di_moc", Source: "relic.season.1.moc_tinh", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.1.di_moc"},
	{ID: "atlas.page.season.2.ben_nuoc.ma_da", Source: "monster.ben_nuoc_den.ma_da", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.ma_da"},
	{ID: "atlas.page.season.2.ben_nuoc.ca_tinh", Source: "monster.ben_nuoc_den.ca_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.ca_tinh"},
	{ID: "atlas.page.season.2.ben_nuoc.quy_song_dem", Source: "monster.ben_nuoc_den.quy_song_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.quy_song_dem"},
	{ID: "atlas.page.season.2.ben_nuoc.bong_nuoc_ma", Source: "monster.ben_nuoc_den.bong_nuoc_ma", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.bong_nuoc_ma"},
	{ID: "atlas.page.season.2.ben_nuoc.hon_chet_duoi", Source: "monster.ben_nuoc_den.hon_chet_duoi", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.hon_chet_duoi"},
	{ID: "atlas.page.season.2.ben_nuoc.ca_tinh_gia", Source: "monster.ben_nuoc_den.ca_tinh_gia", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.ca_tinh_gia"},
	{ID: "atlas.page.season.2.ben_nuoc.ky_ben", Source: "chest.hidden.season.2.ben_nuoc.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.ky_ben"},
	{ID: "atlas.page.season.2.ben_nuoc.ca_bong_den", Source: "item.material.ca_bong_den", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.ca_bong_den"},
	{ID: "atlas.page.season.2.ben_nuoc.di_xom", Source: "relic.season.2.xom_chim", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.di_xom"},
	{ID: "atlas.page.season.2.ben_nuoc.di_ma_da", Source: "relic.season.2.ma_da_gia", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.2.di_ma_da"},
	{ID: "atlas.page.season.3.deo_may.ma_tranh", Source: "monster.deo_may.ma_tranh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ma_tranh"},
	{ID: "atlas.page.season.3.deo_may.khi_nui", Source: "monster.deo_may.khi_nui", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.khi_nui"},
	{ID: "atlas.page.season.3.deo_may.ma_van_dem", Source: "monster.deo_may.ma_van_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ma_van_dem"},
	{ID: "atlas.page.season.3.deo_may.ho_con_tinh", Source: "monster.deo_may.ho_con_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ho_con_tinh"},
	{ID: "atlas.page.season.3.deo_may.vong_rung", Source: "monster.deo_may.vong_rung", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.vong_rung"},
	{ID: "atlas.page.season.3.deo_may.ho_tinh_lon", Source: "monster.deo_may.ho_tinh_lon", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ho_tinh_lon"},
	{ID: "atlas.page.season.3.deo_may.ky_deo", Source: "chest.hidden.season.3.deo_may.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ky_deo"},
	{ID: "atlas.page.season.3.deo_may.ruou_nep", Source: "item.consumable.ruou_nep", Kind: EventDishCooked, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.ruou_nep"},
	{ID: "atlas.page.season.3.deo_may.di_hang", Source: "relic.season.3.hang", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.di_hang"},
	{ID: "atlas.page.season.3.deo_may.di_ho_ve", Source: "relic.season.3.ho_ve", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.3.di_ho_ve"},
	{ID: "atlas.page.season.4.thanh_co.tuong_da", Source: "monster.thanh_co.tuong_da", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.tuong_da"},
	{ID: "atlas.page.season.4.thanh_co.hon_binh", Source: "monster.thanh_co.hon_binh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.hon_binh"},
	{ID: "atlas.page.season.4.thanh_co.ma_co", Source: "monster.thanh_co.ma_co", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.ma_co"},
	{ID: "atlas.page.season.4.thanh_co.qua_tinh", Source: "monster.thanh_co.qua_tinh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.qua_tinh"},
	{ID: "atlas.page.season.4.thanh_co.hon_tran_linh", Source: "monster.thanh_co.hon_tran_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.hon_tran_linh"},
	{ID: "atlas.page.season.4.thanh_co.qua_tinh_lon", Source: "monster.thanh_co.qua_tinh_lon", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.qua_tinh_lon"},
	{ID: "atlas.page.season.4.thanh_co.ky_thanh", Source: "chest.hidden.season.4.thanh_co.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.ky_thanh"},
	{ID: "atlas.page.season.4.thanh_co.ca_bong_kho", Source: "item.consumable.food.ca_bong_kho", Kind: EventDishCooked, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.ca_bong_kho"},
	{ID: "atlas.page.season.4.thanh_co.di_den", Source: "relic.season.4.den", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.di_den"},
	{ID: "atlas.page.season.4.thanh_co.di_thach", Source: "relic.season.4.thach", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.4.di_thach"},
	{ID: "atlas.page.season.5.nui_thieng.vong_linh", Source: "monster.nui_thieng.vong_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.vong_linh"},
	{ID: "atlas.page.season.5.nui_thieng.tinh_thu", Source: "monster.nui_thieng.tinh_thu", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.tinh_thu"},
	{ID: "atlas.page.season.5.nui_thieng.than_rung_dem", Source: "monster.nui_thieng.than_rung_dem", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 30}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.than_rung_dem"},
	{ID: "atlas.page.season.5.nui_thieng.ma_nui", Source: "monster.nui_thieng.ma_nui", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.ma_nui"},
	{ID: "atlas.page.season.5.nui_thieng.hon_binh_co", Source: "monster.nui_thieng.hon_binh_co", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.hon_binh_co"},
	{ID: "atlas.page.season.5.nui_thieng.dai_vong_linh", Source: "monster.nui_thieng.dai_vong_linh", Kind: EventMonsterKilled, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 100}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.dai_vong_linh"},
	{ID: "atlas.page.season.5.nui_thieng.ky_nui", Source: "chest.hidden.season.5.nui_thieng.01", Kind: EventChestOpened, Mode: ApplyCount, Thresholds: [3]uint64{1, 5, 20}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.ky_nui"},
	{ID: "atlas.page.season.5.nui_thieng.ca_suong", Source: "item.material.ca_suong_ho", Kind: EventFishCaught, Mode: ApplyCount, Thresholds: [3]uint64{1, 10, 50}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.ca_suong"},
	{ID: "atlas.page.season.5.nui_thieng.di_cong", Source: "relic.season.5.cong", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.di_cong"},
	{ID: "atlas.page.season.5.nui_thieng.di_linh_ve", Source: "relic.season.5.linh_ve", Kind: EventBossWitness, Mode: ApplyCount, Thresholds: [3]uint64{1, 3, 10}, Special: [3]int64{0, 0, 0}, Title: "cosmetic.title.season.5.di_linh_ve"},
}
