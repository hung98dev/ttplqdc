package balance

import (
	"fmt"
	"math"
	"sort"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// classGrowth is the per-level MAX_HP/ATTACK/DEFENSE growth
// (stats.md § Per-Level Class Growth).
var classGrowth = map[string][3]float64{
	"KIM":  {32, 5.5, 2.0},
	"MOC":  {36, 4.8, 2.2},
	"THUY": {32, 5.0, 1.9},
	"HOA":  {30, 5.5, 1.8},
	"THO":  {44, 4.4, 3.0},
}

// classMPGrowth: +MAX_MP per level per class (rotation MP budget).
var classMPGrowth = map[string]float64{
	"KIM": 8, "MOC": 12, "THUY": 12, "HOA": 14, "THO": 8,
}

// basic1 pins the class basic_1 suffix (canonical pin).
var basic1 = map[string]string{
	"KIM": "kiem_thuc", "MOC": "linh_diep", "THUY": "thuy_tien",
	"HOA": "hoa_phu", "THO": "tran_quyen",
}

var classIDs = []string{"KIM", "MOC", "THUY", "HOA", "THO"}

// rotationPriority is the spec's fixed Lv60 active-priority order.
var rotationPriority = map[string][]string{
	"KIM":  {"nhat_kiem_dinh_hon", "kiem_tran", "pha_giap", "hoi_kiem", "xuyen_phong"},
	"MOC":  {"van_moc_hoi_sinh", "van_doc", "thanh_dang", "moc_bo"},
	"THUY": {"thien_ha", "han_trieu", "trieu_quyen"},
	"HOA":  {"cuu_hoa_lien", "hoa_vuc", "lien_bao", "boc_bo"},
	"THO":  {"thien_son_tran", "dia_chan", "thach_kich"},
}

// endpoints are the six decade levels with reference enhancement
// T1+4 .. T6+8 (balance_validation.md).
var endpoints = []struct {
	Level int64
	Tier  string
	Enh   int64
}{
	{10, "T1", 4}, {20, "T2", 5}, {30, "T3", 6},
	{40, "T4", 6}, {50, "T5", 7}, {60, "T6", 8},
}

// bonusPotential mirrors the authored bonus-potential table.
func bonusPotential(level int64) int64 {
	bonus := map[int64]int64{25: 10, 30: 20, 35: 30, 40: 40, 45: 60, 50: 80, 55: 100, 60: 120}
	return bonus[level]
}

// tierFor maps a level to its equipment tier.
func tierFor(level int64) string {
	t := (level + 9) / 10
	if t < 1 {
		t = 1
	}
	if t > 6 {
		t = 6
	}
	return fmt.Sprintf("T%d", t)
}

// tierEnh is the reference enhancement per tier.
func tierEnh(tier string) int64 {
	switch tier {
	case "T1":
		return 4
	case "T2":
		return 5
	case "T3":
		return 6
	case "T4":
		return 6
	case "T5":
		return 7
	}
	return 8
}

// refBuild is the synthetic PvE reference vector: potential split
// (50% offensive primary / 25% VIT / remainder AGI), class growth, the
// deterministic single-set equipment contribution, expected crit.
type refBuild struct {
	class      string
	level      int64
	tier       string
	attack     float64
	defense    float64
	maxHP      float64
	maxMP      float64
	critChance float64
	attackSpd  float64
}

// equipBaseStats sums the alphabetically-first set_key's fixed base-stat
// lines at a tier — ATTACK/DEFENSE/MAX_HP only — each enhanced by
// floor(value * (1 + enh*0.025)).
func equipBaseStats(cat *equipment.Catalog, tier string, enh int64) map[string]float64 {
	out := map[string]float64{"ATTACK": 0, "DEFENSE": 0, "MAX_HP": 0}
	best := ""
	for _, it := range cat.Items {
		if it.Tier != tier {
			continue
		}
		if best == "" || it.SetKey < best {
			best = it.SetKey
		}
	}
	mult := 1 + float64(enh)*0.025
	for _, it := range cat.Items {
		if it.Tier != tier || it.SetKey != best {
			continue
		}
		for _, t := range it.FixedStats {
			if t.IsUtility() {
				continue
			}
			if _, ok := out[t.Stat]; !ok {
				continue
			}
			out[t.Stat] += math.Floor(float64(t.Flat) * mult)
		}
	}
	return out
}

// eligibleSlots counts the tier items that may roll a stat — the
// "every eligible slot" count for the max-everywhere stress fixture.
// The secondary roll pool is slot-agnostic (equipment_catalog.md: every
// item draws K rolls from the shared 12-type pool), so every slot in the
// first set is eligible for every roll type.
func eligibleSlots(cat *equipment.Catalog, tier, _ string) int64 {
	var n int64
	for _, it := range cat.Items {
		if it.Tier == tier && it.SetKey == firstSetKey(cat, tier) {
			n++
		}
	}
	return n
}

func firstSetKey(cat *equipment.Catalog, tier string) string {
	best := ""
	for _, it := range cat.Items {
		if it.Tier == tier && (best == "" || it.SetKey < best) {
			best = it.SetKey
		}
	}
	return best
}

// referenceBuild mirrors the canonical build exactly; rolls overlays
// fixture-B magnitudes on top.
func referenceBuild(cat *equipment.Catalog, class string, level int64, rolls map[string]float64) refBuild {
	g := classGrowth[class]
	earned := 4*(level-1) + bonusPotential(level)
	off := int64(math.Floor(float64(earned) * 0.50))
	vit := int64(math.Floor(float64(earned) * 0.25))
	agi := earned - off - vit
	tier := tierFor(level)
	eq := equipBaseStats(cat, tier, tierEnh(tier))
	b := refBuild{class: class, level: level, tier: tier}
	b.attack = math.Floor(40 + g[1]*float64(level-1) + float64(off)*0.75 + eq["ATTACK"] + rolls["ATTACK"])
	b.defense = math.Floor(20 + g[2]*float64(level-1) + math.Floor(float64(vit)*0.20) + eq["DEFENSE"] + rolls["DEFENSE"])
	b.maxHP = math.Floor(500 + g[0]*float64(level-1) + float64(vit)*6 + eq["MAX_HP"] + rolls["MAX_HP"])
	b.maxMP = math.Floor(100 + classMPGrowth[class]*float64(level-1) + rolls["MAX_MP"])
	b.critChance = math.Min(0.05+float64(agi)*0.0005+rolls["CRIT_CHANCE"], 0.60)
	b.attackSpd = rolls["ATTACK_SPEED"]
	return b
}

// basicPin resolves the compiled basic_1 coefficient, cooldown and
// startup+active window for a class.
func basicPin(c *config.CandidateSnapshot, class string) (coef, cdS, startup, active float64, ok bool) {
	sid := "skill." + lower(class) + ".basic." + basic1[class]
	for _, r := range familyRecs(c, "basic_proc") {
		if fieldStr(r, "skill_id") != sid {
			continue
		}
		if cf, ok2 := fieldRat(r, "base_coefficient"); ok2 {
			coef = ratFloat(cf)
		}
		if cd, ok2 := fieldRat(r, "base_cd_s"); ok2 {
			cdS = ratFloat(cd)
		}
		ok = true
	}
	for _, r := range familyRecs(c, "skill_action") {
		if fieldStr(r, "skill_id") != sid {
			continue
		}
		if v, ok2 := fieldInt(r, "startup_ms"); ok2 {
			startup = float64(v)
		}
		if v, ok2 := fieldInt(r, "active_ms"); ok2 {
			active = float64(v)
		}
	}
	return
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// defMult is the canonical defense curve (floor 0.25).
func defMult(def, targetLevel float64) float64 {
	k := 100 + 20*targetLevel
	m := k / (k + math.Max(0, def))
	if m < 0.25 {
		m = 0.25
	}
	return m
}

// basicDamage computes one expected basic hit: floor(attack*coef) ->
// expected crit -> defense curve.
func basicDamage(b refBuild, coeff, targetDef, targetLevel float64) float64 {
	raw := math.Floor(b.attack * coeff)
	pre := math.Floor(raw * (1 + b.critChance*0.50))
	return math.Floor(pre * defMult(targetDef, targetLevel))
}

// basicDPS returns hits/s * damage with the tick-quantized cadence
// (50ms ticks; interval = max(cooldown_ms, startup+active)).
func basicDPS(c *config.CandidateSnapshot, class string, b refBuild, targetDef, targetLevel float64) float64 {
	coeff, cdS, startup, active, ok := basicPin(c, class)
	if !ok || cdS <= 0 {
		return 0
	}
	cdS = cdS / (1 + b.attackSpd)
	interval := math.Max(cdS*1000, startup+active)
	hits := 1000 / (50 * math.Ceil(interval/50))
	return basicDamage(b, coeff, targetDef, targetLevel) * hits
}

// heavyHit returns the post-mitigation damage of one boss heavy hit
// (1.40 * boss ATTACK) at the reference build.
func heavyHit(bossAttack float64, b refBuild, level int64) float64 {
	raw := math.Floor(bossAttack * 1.40)
	return math.Floor(raw * defMult(b.defense, float64(level)))
}

// bossRow is one authored boss stat row.
type bossRow struct {
	ID      string
	Level   int64
	Mode    string
	Profile string
	BaseHP  int64
	Attack  int64
	Defense int64
}

func bosses(c *config.CandidateSnapshot) []bossRow {
	var out []bossRow
	for _, r := range familyRecs(c, "boss") {
		b := bossRow{ID: fieldStr(r, "boss_id"), Mode: fieldStr(r, "mode"),
			Profile: fieldStr(r, "size_profile")}
		b.Level, _ = fieldInt(r, "lv")
		b.BaseHP, _ = fieldInt(r, "base_hp")
		b.Attack, _ = fieldInt(r, "attack")
		b.Defense, _ = fieldInt(r, "defense")
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normHP(l int64) float64 {
	return math.Floor(180 + 30*float64(l) + 0.45*float64(l)*float64(l))
}
func normDef(l int64) float64 { return math.Floor(8 + 1.6*float64(l)) }

// ttkRow is one emitted report row per (class, endpoint).
type ttkRow struct {
	Class        string
	Level        int64
	Tier         string
	Attack       float64
	Defense      float64
	MaxHP        float64
	CritChance   float64
	AttackSpd    float64
	NormalTTK    float64
	EliteTTK     float64
	BossSoloTTK  float64
	BossPartyTTK float64
	HeavyHitPct  float64
	BossID       string
}

// windowCheck evaluates all guardrails for one build at one level.
// NORMAL/ELITE gate only at decade endpoints; boss windows gate every
// authored boss row at its own level. label identifies the fixture
// (A = no-roll reference; B = roll magnitude). Windows never widen.
func windowCheck(c *config.CandidateSnapshot, cat *equipment.Catalog, class string, level int64, rolls map[string]float64, label string, d *config.Diagnostics) ttkRow {
	b := referenceBuild(cat, class, level, rolls)
	row := ttkRow{Class: class, Level: level, Tier: b.tier,
		Attack: b.attack, Defense: b.defense, MaxHP: b.maxHP,
		CritChance: b.critChance, AttackSpd: b.attackSpd}
	if level%10 == 0 {
		dpsN := basicDPS(c, class, b, normDef(level), float64(level))
		if dpsN <= 0 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.ttk: %s: no basic_1 DPS for %s Lv%d", label, class, level)
			return row
		}
		row.NormalTTK = normHP(level) / dpsN
		if row.NormalTTK < 2.0 || row.NormalTTK > 6.0 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.ttk_normal: %s: %s Lv%d %.2fs outside 2.0..6.0", label, class, level, row.NormalTTK)
		}
		row.EliteTTK = math.Floor(normHP(level)*4.00) / basicDPS(c, class, b, math.Floor(normDef(level)*1.20), float64(level))
		if row.EliteTTK < 8.5 || row.EliteTTK > 24.0 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.ttk_elite: %s: %s Lv%d %.2fs outside 8.5..24", label, class, level, row.EliteTTK)
		}
	}
	for _, bs := range bosses(c) {
		if bs.Level != level {
			continue
		}
		row.BossID = bs.ID
		dps := basicDPS(c, class, b, float64(bs.Defense), float64(bs.Level))
		row.BossSoloTTK = float64(bs.BaseHP) / dps
		if row.BossSoloTTK < 55 || row.BossSoloTTK > 130 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.ttk_boss_solo: %s: %s vs %s %.2fs outside 55..130", label, class, bs.ID, row.BossSoloTTK)
		}
		row.BossPartyTTK = row.BossSoloTTK * 0.64
		if row.BossPartyTTK < 35 || row.BossPartyTTK > 85 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.ttk_boss_party: %s: %s vs %s %.2fs outside 35..85", label, class, bs.ID, row.BossPartyTTK)
		}
		row.HeavyHitPct = heavyHit(float64(bs.Attack), b, level) / b.maxHP
		if row.HeavyHitPct < 0.06 || row.HeavyHitPct > 0.18 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.heavy_hit: %s: %s vs %s %.4f of MAX_HP outside 0.06..0.18", label, class, bs.ID, row.HeavyHitPct)
		}
	}
	return row
}

// checkTTK runs fixture A: NORMAL/ELITE at every decade endpoint and the
// boss windows + heavy hit + class spread on every authored boss row at
// its own level (tierFor its level).
func checkTTK(c *config.CandidateSnapshot, cat *equipment.Catalog, d *config.Diagnostics) []ttkRow {
	var rows []ttkRow
	var levels []int64
	for _, ep := range endpoints {
		levels = append(levels, ep.Level)
	}
	for _, bs := range bosses(c) {
		dup := false
		for _, l := range levels {
			if l == bs.Level {
				dup = true
			}
		}
		if !dup {
			levels = append(levels, bs.Level)
		}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	for _, l := range levels {
		for _, cl := range classIDs {
			r := windowCheck(c, cat, cl, l, nil, "A", d)
			if l%10 == 0 {
				rows = append(rows, r)
			}
		}
	}
	// class spread: max/min basic-only TTK per boss row across classes
	for _, bs := range bosses(c) {
		mn, mx := math.MaxFloat64, 0.0
		for _, cl := range classIDs {
			b := referenceBuild(cat, cl, bs.Level, nil)
			dps := basicDPS(c, cl, b, float64(bs.Defense), float64(bs.Level))
			if dps <= 0 {
				continue
			}
			t := float64(bs.BaseHP) / dps
			if t < mn {
				mn = t
			}
			if t > mx {
				mx = t
			}
		}
		if mn > 0 && mn != math.MaxFloat64 && mx/mn > 2.20 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.class_spread: max/min boss TTK %.4f > 2.20 vs %s", mx/mn, bs.ID)
		}
	}
	return rows
}
