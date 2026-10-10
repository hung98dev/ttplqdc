package cosmetics

import (
	"strings"

	"thinhthan/internal/durable/atlas"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Category is the cosmetic kind bucket used for slot and guild
// category validation (cosmetics.md § Categories).
type Category int

const (
	CategoryTitle Category = iota + 1
	CategoryTitleGlow
	CategoryProfileFrame
	CategoryNameplate
	CategoryAppearance
	CategoryWeaponTrail
	CategoryAura
	CategoryEmote
	CategoryCharacterShrine
	CategoryGuildStoneInscription
	CategoryGuildShrineVisual
	CategoryGuildBanner
	CategoryGuildCrestAccent
)

// Scope is the ownership scope of the cosmetic (cosmetics.md §
// Identity / Ownership).
type Scope int

const (
	ScopeCharacter Scope = iota + 1
	ScopeAccount
	ScopeGuild
)

// Def is one stable launch cosmetic.
type Def struct {
	ID       string
	Category Category
	Scope    Scope
}

// Slot maps a character-equip slot category onto the wire slot enum.
// EMOTE and the guild categories have no character slot.
func (c Category) Slot() (protocolv1.CosmeticSlot, bool) {
	switch c {
	case CategoryTitle:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE, true
	case CategoryTitleGlow:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE_GLOW, true
	case CategoryProfileFrame:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME, true
	case CategoryNameplate:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_NAMEPLATE, true
	case CategoryAppearance:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_APPEARANCE, true
	case CategoryWeaponTrail:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_WEAPON_TRAIL, true
	case CategoryAura:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_AURA, true
	case CategoryCharacterShrine:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_CHARACTER_SHRINE, true
	case CategoryGuildStoneInscription:
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_GUILD_STONE_INSCRIPTION, true
	}
	return protocolv1.CosmeticSlot_COSMETIC_SLOT_UNSPECIFIED, false
}

// Routes are the authored redemption inputs for a cosmetic. A nil
// field means the route does not exist for that cosmetic; a cosmetic
// may expose alternative material and special routes
// (cosmetics.md § Special-Currency Redemption).
type Routes struct {
	MaterialItem  string
	MaterialQty   int64
	SpecialAmount int64
	CommonAmount  int64
}

// FeatDef is one authored Folklore Feat (cosmetic_catalog.md §
// FOLKLORE FEATS CATALOG). Counters are cumulative except flags
// (ENHANCEMENT_FLAG/PVP_RANK_FLAG), which latch on first reach.
type FeatDef struct {
	ID        string
	Flag      bool   // boolean flag feats vs cumulative counters
	Threshold int64  // milestone_threshold value reaching the grant
	Reward    string // cosmetic_id granted on milestone completion
}

// catalog is the compiled mirror of cosmetic_catalog.md: 127 core +
// atlas play titles, 9 frames, 4 appearances, 20 common sinks, 20
// special sinks, 36 seasonal free/paid, 60 seasonal atlas titles, 13
// IAP store ids, 5 guild cosmetics = 294 stable ids.
var catalog = buildCatalog()

func buildCatalog() map[string]Def {
	out := map[string]Def{}
	put := func(id string, cat Category, scope Scope) {
		out[id] = Def{ID: id, Category: cat, Scope: scope}
	}
	// TITLE — 20 core (atlas titles arrive via durable/atlas's roster).
	for _, c := range []struct{ id string }{
		{"cosmetic.title.lang_da"}, {"cosmetic.title.u_minh"},
		{"cosmetic.title.ben_nuoc"}, {"cosmetic.title.deo_may"},
		{"cosmetic.title.thanh_co"}, {"cosmetic.title.canh_cuoi"},
		{"cosmetic.title.tho_san_thuong_luong"},
		{"cosmetic.title.doi_mat_than_trung"},
		{"cosmetic.title.khac_tinh_ma_da"},
		{"cosmetic.title.dung_si_tru_ho"},
		{"cosmetic.title.ngu_ong_ben_do"},
		{"cosmetic.title.ban_tay_than"},
		{"cosmetic.title.tuyet_dinh_than_binh"},
		{"cosmetic.title.thien_ha_de_nhat"},
		{"cosmetic.title.trang_si_giup_doi"},
		{"cosmetic.title.dai_hiep_lang_que"},
		{"cosmetic.title.hiep_nghia_vo_song"},
		{"cosmetic.title.tam_giao_vien_man"},
		{"cosmetic.title.pvp.than_thoai"},
		{"cosmetic.title.guild_war.hung_binh"},
		// Atlas milestone titles (durable/atlas.Milestones owners).
		{"cosmetic.title.atlas.nha_suu_tam"},
		{"cosmetic.title.atlas.hoc_gia_dan_gian"},
		{"cosmetic.title.atlas.bach_khoa_dan_gian"},
	} {
		put(c.id, CategoryTitle, ScopeCharacter)
	}
	// Atlas + seasonal-atlas titles are finite expansions over the
	// durable/atlas compiled roster (one concept = one owner).
	for _, p := range atlas.Roster() {
		if p.Title == "" {
			continue
		}
		put(p.Title, CategoryTitle, ScopeCharacter)
	}
	// PROFILE_FRAME — 9.
	for _, fid := range []string{
		"cosmetic.frame.ben_da", "cosmetic.frame.ho_tinh",
		"cosmetic.frame.nui_thieng", "cosmetic.frame.pvp.bac",
		"cosmetic.frame.pvp.vang", "cosmetic.frame.pvp.ngoc",
		"cosmetic.frame.pvp.linh", "cosmetic.frame.pvp.than_thoai",
		"cosmetic.frame.guild_war.chien_ky",
	} {
		put(fid, CategoryProfileFrame, ScopeCharacter)
	}
	// CHARACTER_APPEARANCE — 4.
	for _, aid := range []string{
		"cosmetic.appearance.non_la_moc", "cosmetic.appearance.ao_toi_la",
		"cosmetic.appearance.khan_ben_nuoc",
		"cosmetic.appearance.ao_vai_hoa_van",
	} {
		put(aid, CategoryAppearance, ScopeCharacter)
	}
	// CURRENCY.COMMON COSMETIC SINKS — 20 (6 inscriptions + 6 shrines
	// + 8 title glows; prices per economy_catalog.md).
	for _, s := range []struct {
		id  string
		cat Category
	}{
		{"cosmetic.guild_stone.inscription.trung_nguyen", CategoryGuildStoneInscription},
		{"cosmetic.guild_stone.inscription.long_van", CategoryGuildStoneInscription},
		{"cosmetic.guild_stone.inscription.phuong_vi", CategoryGuildStoneInscription},
		{"cosmetic.guild_stone.inscription.ngoc_bich", CategoryGuildStoneInscription},
		{"cosmetic.guild_stone.inscription.kim_bach", CategoryGuildStoneInscription},
		{"cosmetic.guild_stone.inscription.thien_long", CategoryGuildStoneInscription},
		{"cosmetic.shrine.lua_do", CategoryCharacterShrine},
		{"cosmetic.shrine.thuy_ngoc", CategoryCharacterShrine},
		{"cosmetic.shrine.moc_xanh", CategoryCharacterShrine},
		{"cosmetic.shrine.tho_vang", CategoryCharacterShrine},
		{"cosmetic.shrine.kim_trang", CategoryCharacterShrine},
		{"cosmetic.shrine.linh_khoi", CategoryCharacterShrine},
		{"cosmetic.title_glow.do_quang", CategoryTitleGlow},
		{"cosmetic.title_glow.xanh_nhat", CategoryTitleGlow},
		{"cosmetic.title_glow.vang_nhat", CategoryTitleGlow},
		{"cosmetic.title_glow.tim_nhat", CategoryTitleGlow},
		{"cosmetic.title_glow.bach_nhat", CategoryTitleGlow},
		{"cosmetic.title_glow.hong_phuc", CategoryTitleGlow},
		{"cosmetic.title_glow.linh_hoa", CategoryTitleGlow},
		{"cosmetic.title_glow.thien_van", CategoryTitleGlow},
	} {
		put(s.id, s.cat, ScopeCharacter)
	}
	// SPECIAL-CURRENCY SINKS — 20 (economy_catalog.md prices).
	for _, s := range []struct {
		id  string
		cat Category
	}{
		{"cosmetic.title_glow.kim_quang", CategoryTitleGlow},
		{"cosmetic.title_glow.hoa_quang", CategoryTitleGlow},
		{"cosmetic.title_glow.thuy_linh", CategoryTitleGlow},
		{"cosmetic.title_glow.tho_bach", CategoryTitleGlow},
		{"cosmetic.title_glow.moc_xanh", CategoryTitleGlow},
		{"cosmetic.shrine_special.dai_hong", CategoryCharacterShrine},
		{"cosmetic.shrine_special.ngoc_bich", CategoryCharacterShrine},
		{"cosmetic.shrine_special.hong_tran", CategoryCharacterShrine},
		{"cosmetic.shrine_special.cu_thach", CategoryCharacterShrine},
		{"cosmetic.shrine_special.bach_ngoc", CategoryCharacterShrine},
		{"cosmetic.frame.rong_vang", CategoryProfileFrame},
		{"cosmetic.frame.phuong_hoang", CategoryProfileFrame},
		{"cosmetic.frame.bach_ho", CategoryProfileFrame},
		{"cosmetic.frame.huyen_vu", CategoryProfileFrame},
		{"cosmetic.frame.lan_linh", CategoryProfileFrame},
		{"cosmetic.nameplate.ky_luat_vang", CategoryNameplate},
		{"cosmetic.nameplate.linh_thuyen", CategoryNameplate},
		{"cosmetic.nameplate.son_ha", CategoryNameplate},
		{"cosmetic.aura.quy_khi", CategoryAura},
		{"cosmetic.aura.linh_khi", CategoryAura},
	} {
		put(s.id, s.cat, ScopeCharacter)
	}
	// SEASONAL COSMETICS — free/paid per season 0..5.
	put("cosmetic.title.season.0.lang_da_ky_ghe", CategoryTitle, ScopeCharacter)
	put("cosmetic.title.season.1.u_minh_suong", CategoryTitle, ScopeCharacter)
	put("cosmetic.title.season.2.ben_den", CategoryTitle, ScopeCharacter)
	put("cosmetic.title.season.3.deo_suong", CategoryTitle, ScopeCharacter)
	put("cosmetic.title.season.4.thanh_mua", CategoryTitle, ScopeCharacter)
	put("cosmetic.title.season.5.nui_mua", CategoryTitle, ScopeCharacter)
	for n := 0; n <= 5; n++ {
		p := "cosmetic." + []string{
			"frame.season.0", "frame.season.1", "frame.season.2",
			"frame.season.3", "frame.season.4", "frame.season.5",
		}[n]
		put(p, CategoryProfileFrame, ScopeCharacter)
		p = "cosmetic." + []string{
			"shrine.season.0", "shrine.season.1", "shrine.season.2",
			"shrine.season.3", "shrine.season.4", "shrine.season.5",
		}[n]
		put(p, CategoryCharacterShrine, ScopeCharacter)
	}
	// Season 0 paid titles carry a per-key display suffix; seasons 1..5
	// use the flat `.paid` ids per the authored matrix.
	put("cosmetic.title.season.0.paid.dem_lang", CategoryTitle, ScopeCharacter)
	for n := 1; n <= 5; n++ {
		put("cosmetic.title.season."+itoa(n)+".paid", CategoryTitle, ScopeCharacter)
	}
	for n := 0; n <= 5; n++ {
		put("cosmetic.frame.season."+itoa(n)+".paid", CategoryProfileFrame, ScopeCharacter)
		put("cosmetic.emote.season."+itoa(n)+".paid", CategoryEmote, ScopeCharacter)
	}
	// Season 0 paid emote keeps its authored key suffix.
	delete(out, "cosmetic.emote.season.0.paid")
	put("cosmetic.emote.season.0.paid.chap_tay", CategoryEmote, ScopeCharacter)
	// IAP STORE COSMETICS — 13, account-entitled.
	for _, s := range []struct {
		id  string
		cat Category
	}{
		{"cosmetic.iap.appearance.co_tam_truyen", CategoryAppearance},
		{"cosmetic.iap.appearance.co_tien", CategoryAppearance},
		{"cosmetic.iap.appearance.vo_quan_thanh_co", CategoryAppearance},
		{"cosmetic.iap.appearance.nu_tuong_trong_dong", CategoryAppearance},
		{"cosmetic.iap.trail.phuong_hoang_vu", CategoryWeaponTrail},
		{"cosmetic.iap.trail.bao_gam", CategoryWeaponTrail},
		{"cosmetic.iap.trail.long_hoa", CategoryWeaponTrail},
		{"cosmetic.iap.emote.bai_chao_lang", CategoryEmote},
		{"cosmetic.iap.emote.vo_tay_thang_tran", CategoryEmote},
		{"cosmetic.iap.emote.ngoi_thien_dinh", CategoryEmote},
		{"cosmetic.iap.frame.thien_long", CategoryProfileFrame},
		{"cosmetic.iap.nameplate.hun_thuoc_co", CategoryNameplate},
		{"cosmetic.iap.title_glow.long_nhan", CategoryTitleGlow},
	} {
		put(s.id, s.cat, ScopeAccount)
	}
	// GUILD-SCOPED COSMETICS — 5.
	put("cosmetic.guild.crest.ritual_4", CategoryGuildCrestAccent, ScopeGuild)
	put("cosmetic.guild.banner.ritual_8", CategoryGuildBanner, ScopeGuild)
	put("cosmetic.guild.shrine.ritual_12", CategoryGuildShrineVisual, ScopeGuild)
	put("cosmetic.guild.shrine.guild_war_top10", CategoryGuildShrineVisual, ScopeGuild)
	put("cosmetic.guild.banner.guild_war_champion", CategoryGuildBanner, ScopeGuild)
	return out
}

func itoa(n int) string {
	return string(rune('0' + n))
}

// Lookup resolves a stable cosmetic id to its definition.
func Lookup(cosmeticID string) (Def, bool) {
	d, ok := catalog[cosmeticID]
	return d, ok
}

// Roster exposes the full compiled id set (count assertions).
func Roster() []Def {
	out := make([]Def, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, d)
	}
	return out
}

// redemption routes mirror cosmetic_catalog.md redemption fences +
// economy_catalog.md prices. A route value of 0 marks it absent.
var routes = map[string]Routes{
	"cosmetic.frame.nui_thieng": {
		MaterialItem: "item.material.vai_hoa_van", MaterialQty: 30,
		SpecialAmount: 20,
	},
	"cosmetic.appearance.ao_vai_hoa_van": {
		MaterialItem: "item.material.vai_hoa_van", MaterialQty: 20,
		SpecialAmount: 20,
	},
	// Common sinks (economy_catalog.md § Cosmetic Sink Catalog).
	"cosmetic.guild_stone.inscription.trung_nguyen": {CommonAmount: 50000},
	"cosmetic.guild_stone.inscription.long_van":     {CommonAmount: 80000},
	"cosmetic.guild_stone.inscription.phuong_vi":    {CommonAmount: 120000},
	"cosmetic.guild_stone.inscription.ngoc_bich":    {CommonAmount: 180000},
	"cosmetic.guild_stone.inscription.kim_bach":     {CommonAmount: 250000},
	"cosmetic.guild_stone.inscription.thien_long":   {CommonAmount: 400000},
	"cosmetic.shrine.lua_do":                        {CommonAmount: 60000},
	"cosmetic.shrine.thuy_ngoc":                     {CommonAmount: 100000},
	"cosmetic.shrine.moc_xanh":                      {CommonAmount: 150000},
	"cosmetic.shrine.tho_vang":                      {CommonAmount: 220000},
	"cosmetic.shrine.kim_trang":                     {CommonAmount: 320000},
	"cosmetic.shrine.linh_khoi":                     {CommonAmount: 500000},
	"cosmetic.title_glow.do_quang":                  {CommonAmount: 30000},
	"cosmetic.title_glow.xanh_nhat":                 {CommonAmount: 30000},
	"cosmetic.title_glow.vang_nhat":                 {CommonAmount: 30000},
	"cosmetic.title_glow.tim_nhat":                  {CommonAmount: 30000},
	"cosmetic.title_glow.bach_nhat":                 {CommonAmount: 30000},
	"cosmetic.title_glow.hong_phuc":                 {CommonAmount: 75000},
	"cosmetic.title_glow.linh_hoa":                  {CommonAmount: 75000},
	"cosmetic.title_glow.thien_van":                 {CommonAmount: 120000},
	// Special sinks (economy_catalog.md § Extended Special Cosmetic
	// Sink Catalog).
	"cosmetic.title_glow.kim_quang":     {SpecialAmount: 10},
	"cosmetic.title_glow.hoa_quang":     {SpecialAmount: 10},
	"cosmetic.title_glow.thuy_linh":     {SpecialAmount: 10},
	"cosmetic.title_glow.tho_bach":      {SpecialAmount: 10},
	"cosmetic.title_glow.moc_xanh":      {SpecialAmount: 10},
	"cosmetic.shrine_special.dai_hong":  {SpecialAmount: 20},
	"cosmetic.shrine_special.ngoc_bich": {SpecialAmount: 20},
	"cosmetic.shrine_special.hong_tran": {SpecialAmount: 20},
	"cosmetic.shrine_special.cu_thach":  {SpecialAmount: 20},
	"cosmetic.shrine_special.bach_ngoc": {SpecialAmount: 20},
	"cosmetic.frame.rong_vang":          {SpecialAmount: 25},
	"cosmetic.frame.phuong_hoang":       {SpecialAmount: 25},
	"cosmetic.frame.bach_ho":            {SpecialAmount: 25},
	"cosmetic.frame.huyen_vu":           {SpecialAmount: 25},
	"cosmetic.frame.lan_linh":           {SpecialAmount: 25},
	"cosmetic.nameplate.ky_luat_vang":   {SpecialAmount: 50},
	"cosmetic.nameplate.linh_thuyen":    {SpecialAmount: 50},
	"cosmetic.nameplate.son_ha":         {SpecialAmount: 50},
	"cosmetic.aura.quy_khi":             {SpecialAmount: 40},
	"cosmetic.aura.linh_khi":            {SpecialAmount: 40},
}

// RoutesFor returns the authored redemption routes for a cosmetic.
func RoutesFor(cosmeticID string) (Routes, bool) {
	r, ok := routes[cosmeticID]
	return r, ok
}

// FeatDefs is the authored FOLKLORE FEATS roster (6 launch feats).
var FeatDefs = []FeatDef{
	{ID: "feat.combat.slay_ma_da", Threshold: 1000,
		Reward: "cosmetic.title.khac_tinh_ma_da"},
	{ID: "feat.combat.slay_ho_tinh_boss", Threshold: 10,
		Reward: "cosmetic.title.dung_si_tru_ho"},
	{ID: "feat.life.catch_fish", Threshold: 200,
		Reward: "cosmetic.title.ngu_ong_ben_do"},
	{ID: "feat.craft.enhance_12", Flag: true, Threshold: 12,
		Reward: "cosmetic.title.ban_tay_than"},
	{ID: "feat.craft.enhance_16", Flag: true, Threshold: 16,
		Reward: "cosmetic.title.tuyet_dinh_than_binh"},
	{ID: "feat.pvp.rank1_season", Flag: true, Threshold: 1,
		Reward: "cosmetic.title.thien_ha_de_nhat"},
}

// FeatByID resolves a stable feat id.
func FeatByID(featID string) (FeatDef, bool) {
	for _, f := range FeatDefs {
		if f.ID == featID {
			return f, true
		}
	}
	return FeatDef{}, false
}

// IsMaDaFamily reports whether a monster/boss id belongs to the
// ma_da feat family (both variants plus boss.ma_da_chua count one
// each, cosmetic_catalog.md § Feat Counter Rules).
func IsMaDaFamily(sourceID string) bool {
	return strings.HasPrefix(sourceID, "monster.") &&
		strings.Contains(sourceID, "ma_da") ||
		sourceID == "boss.ma_da_chua"
}
