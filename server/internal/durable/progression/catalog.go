package progression

// Skill catalog facts mirrored from sim/progression (which durable may
// not import — architecture boundary). The 12-row-per-class document
// layout of class_skill_catalog.md is re-expressed compactly: kinds and
// unlock levels are shared across classes; only the name tail differs.
// Any catalog change must be applied to sim/progression and mirrored
// here in the same change.
var (
	layoutKinds = [12]string{
		"basic", "basic", "passive", "active",
		"passive", "basic", "active", "active",
		"passive", "basic", "active", "active",
	}
	layoutUnlocks = [12]int32{1, 4, 11, 8, 27, 18, 14, 22, 50, 36, 32, 45}
	classElements = map[string]string{
		"class.kim": "kim", "class.moc": "moc", "class.thuy": "thuy",
		"class.hoa": "hoa", "class.tho": "tho",
	}
	classSkillNames = map[string][12]string{
		"class.kim": {
			"kiem_thuc", "truy_phong_kiem", "kiem_tam", "xuyen_phong",
			"lien_kiem", "pha_khong_kiem", "hoi_kiem", "pha_giap",
			"kiem_y_bat_diet", "vo_song_kiem", "kiem_tran", "nhat_kiem_dinh_hon",
		},
		"class.moc": {
			"linh_diep", "thao_kich", "duoc_tinh", "moc_bo",
			"sinh_tuc", "truc_phi_tieu", "hoi_xuan", "thanh_dang",
			"thao_moc_dong_hoa", "co_thu_kich", "van_doc", "van_moc_hoi_sinh",
		},
		"class.thuy": {
			"thuy_tien", "bang_phien", "han_khi", "luu_bo",
			"luu_chuyen", "am_luu", "trieu_quyen", "thuy_kinh",
			"bang_giap_tam", "huyen_bang_kich", "han_trieu", "thien_ha",
		},
		"class.hoa": {
			"hoa_phu", "viem_dan", "du_hoa", "boc_bo",
			"cuong_hoa", "hoa_xa", "lien_bao", "hoa_giap",
			"hoa_hon", "lua_tao_quan", "hoa_vuc", "cuu_hoa_lien",
		},
		"class.tho": {
			"tran_quyen", "pha_thach_kich", "bat_dong", "thach_kich",
			"hau_tho", "dia_liet_kich", "tho_giap", "dia_chan",
			"son_ha_ho_the", "kim_cang_quyen", "son_bich", "thien_son_tran",
		},
	}
)

// skillInfo is the mirrored per-skill fact row.
type skillInfo struct {
	kind    string // "basic" | "active" | "passive"
	unlock  int32  // catalog unlock level
	maxLvl  int32
	skillID string
}

var skillFacts = buildFacts()

func buildFacts() map[string]skillInfo {
	out := map[string]skillInfo{}
	for classID, names := range classSkillNames {
		element := classElements[classID]
		for i, name := range names {
			kind := layoutKinds[i]
			max := int32(12)
			if kind == "passive" {
				max = 6
			}
			id := "skill." + element + "." + kind + "." + name
			out[id] = skillInfo{kind: kind, unlock: layoutUnlocks[i],
				maxLvl: max, skillID: id}
		}
	}
	return out
}

// learnedLevel mirrors sim.LearnedLevel: a catalog skill is learned at
// level 1 once the character reaches its unlock milestone even without a
// persisted row; a persisted row wins.
func learnedLevel(rowLvl int32, rowExists bool, charLevel int32, info skillInfo) (int32, bool) {
	if rowExists && rowLvl >= 1 {
		return rowLvl, true
	}
	if charLevel >= info.unlock {
		return 1, true
	}
	return 0, false
}
