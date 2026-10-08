package progression

// Skill catalog tables embedded from docs/07_content/class_skill_catalog.md.
// Every class shares the same document layout (12 rows): two basics, one
// passive, one active, one passive, one basic, two actives, one passive,
// one basic, two actives — with unlock levels 1, 4, 11, 8, 27, 18, 14, 22,
// 50, 36, 32, 45 in document order. Only the per-class skill name tail
// differs; ids are `skill.<class>.<kind>.<name>`.

// SkillKind is the three-skill taxonomy of skills.md.
type SkillKind uint8

const (
	SkillKindBasic SkillKind = iota
	SkillKindActive
	SkillKindPassive
)

// MaxLevel is the per-kind skill-level cap of progression.md § Skill
// Points (basics and actives 12, passives 6).
func (k SkillKind) MaxLevel() int32 {
	switch k {
	case SkillKindPassive:
		return 6
	default:
		return 12
	}
}

// SkillDef is one catalog row projected to progression's needs.
type SkillDef struct {
	ID          string
	Kind        SkillKind
	UnlockLevel int32
}

// classElements maps the permanent class id to its catalog id segment.
var classElements = map[string]string{
	"class.kim":  "kim",
	"class.moc":  "moc",
	"class.thuy": "thuy",
	"class.hoa":  "hoa",
	"class.tho":  "tho",
}

// layoutKinds / layoutUnlocks are the shared 12-row document layout.
var layoutKinds = [12]SkillKind{
	SkillKindBasic, SkillKindBasic, SkillKindPassive, SkillKindActive,
	SkillKindPassive, SkillKindBasic, SkillKindActive, SkillKindActive,
	SkillKindPassive, SkillKindBasic, SkillKindActive, SkillKindActive,
}
var layoutUnlocks = [12]int32{1, 4, 11, 8, 27, 18, 14, 22, 50, 36, 32, 45}

// classSkillNames holds each class's 12 skill-name tails in catalog
// document order (index aligned with layoutKinds/layoutUnlocks).
var classSkillNames = map[string][12]string{
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

var classSkillTables = buildTables()

func buildTables() map[string][]SkillDef {
	out := make(map[string][]SkillDef, len(classSkillNames))
	for classID, names := range classSkillNames {
		element := classElements[classID]
		kindNames := [3]string{"basic", "active", "passive"}
		defs := make([]SkillDef, 0, len(names))
		for i, name := range names {
			defs = append(defs, SkillDef{
				ID:          "skill." + element + "." + kindNames[layoutKinds[i]] + "." + name,
				Kind:        layoutKinds[i],
				UnlockLevel: layoutUnlocks[i],
			})
		}
		out[classID] = defs
	}
	return out
}

// ClassSkills returns the class's 12 catalog rows in document order;
// nil for an unknown class id.
func ClassSkills(classID string) []SkillDef { return classSkillTables[classID] }

// skillsUnlockedAt returns the catalog skill ids whose unlock level equals
// level, in document order (at most one per class).
func skillsUnlockedAt(classID string, level int32) []string {
	var ids []string
	for _, d := range classSkillTables[classID] {
		if d.UnlockLevel == level {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// LearnableAt returns the catalog skill ids learnable at level (unlock ≤
// level), in catalog document order — the deterministic learned set.
func LearnableAt(classID string, level int32) []string {
	var ids []string
	for _, d := range classSkillTables[classID] {
		if d.UnlockLevel <= level {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// Lookup returns the catalog row for a skill id, or false when the id is
// not a launch class skill.
func Lookup(skillID string) (SkillDef, bool) {
	for _, defs := range classSkillTables {
		for _, d := range defs {
			if d.ID == skillID {
				return d, true
			}
		}
	}
	return SkillDef{}, false
}

// BasicOne returns the class's basic_1 skill id — the Lv1-unlocked first
// BASIC row used by the default skill_loadout projection (skills.md
// § Active and Basic Loadout).
func BasicOne(classID string) string {
	for _, d := range classSkillTables[classID] {
		if d.Kind == SkillKindBasic {
			return d.ID
		}
	}
	return ""
}

// LearnMilestones is the skill-learn milestone set of progression.md
// (the 12 distinct catalog unlock levels).
var LearnMilestones = [12]int32{1, 4, 8, 11, 14, 18, 22, 27, 32, 36, 45, 50}

// BookFlagLevels is the ordered bonus-book milestone set: a potential book
// and a skill book grant flag at each of these levels
// (`progression.book.<potential|skill>.<level>`).
var BookFlagLevels = [8]int32{25, 30, 35, 40, 45, 50, 55, 60}

// booksAt returns the book count at a milestone level: one book below 45,
// two books at 45 and above (progression.md § Bonus Books).
func booksAt(flagLevel int32) int32 {
	if flagLevel >= 45 {
		return 2
	}
	return 1
}

// PotentialBookPoints is the cumulative potential points granted by bonus
// books for every milestone ≤ level (10 points per book).
func PotentialBookPoints(level int32) int32 {
	var pts int32
	for _, m := range BookFlagLevels {
		if level >= m {
			pts += 10 * booksAt(m)
		}
	}
	return pts
}

// SkillBookPoints is the cumulative skill points granted by bonus books
// for every milestone ≤ level (1 point per book).
func SkillBookPoints(level int32) int32 {
	var pts int32
	for _, m := range BookFlagLevels {
		if level >= m {
			pts += booksAt(m)
		}
	}
	return pts
}

// PotentialPerLevel is the potential grant per level gained.
const PotentialPerLevel int32 = 4

// BonusSkillPoints is the extra skill-point grant for a level gained:
// +2 at 55 and 60 (progression.md § Level Rewards).
func BonusSkillPoints(level int32) int32 {
	if level == 55 || level == 60 {
		return 2
	}
	return 0
}

// SkillPointsEarned is the level-up skill-point budget at level (excludes
// books): (level-1) base + bonus points at 55/60. At 60 → 63.
func SkillPointsEarned(level int32) int32 {
	if level < StartLevel {
		level = StartLevel
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	return level - StartLevel + BonusSkillPoints(55)*bool32(level >= 55) + BonusSkillPoints(60)*bool32(level >= 60)
}

// PotentialPointsEarned is the level-up potential budget at level
// (excludes books): 4·(level-1). At 60 → 236.
func PotentialPointsEarned(level int32) int32 {
	if level < StartLevel {
		level = StartLevel
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	return PotentialPerLevel * (level - StartLevel)
}

func bool32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
