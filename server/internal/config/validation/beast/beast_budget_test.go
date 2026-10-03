package beast

import (
	"strconv"
	"strings"
	"testing"

	"thinhthan/internal/config"
)

// ---- fixture ---------------------------------------------------------------

func put(t *testing.T, f *config.Family, key []config.Value, fields map[string]config.Value) {
	t.Helper()
	if dup, _ := f.Put(key, fields); dup {
		t.Fatalf("duplicate key %v in %s", key, f.Name)
	}
}

type testBeast struct {
	id      string
	element string
	p1ID    string
	p1Desc  string
	p1Lv60  string
	p2ID    string
	p2Type  string
	p2Pay   string
	icd     [3]int64
	base    map[string][2]int64 // stat -> lv60 value as rational num/den
}

// launchBeasts mirrors the compiled roster (spirit_beast_catalog.md Passive
// Rules + Detailed Beast Profiles): real Lv60 payloads, P2 types and ICDs.
func launchBeasts() []testBeast {
	return []testBeast{
		{"beast.kim.ho_vang", "KIM", "beast.skill.ho_vang.ho_uy",
			"Increases owner's `CRIT_DAMAGE`.", "+20.0%",
			"beast.skill.ho_vang.khoa_huyet", "Anti-Heal", "", [3]int64{60, 50, 45},
			map[string][2]int64{"ATTACK": {20, 1}, "CRIT_CHANCE": {11, 1000}, "MAX_HP": {100, 1}}},
		{"beast.kim.nghe_dong", "KIM", "beast.skill.nghe_dong.dong_tam",
			"Increases owner's `DEFENSE` and reduces incoming critical strike chance.",
			"+8.0% DEFENSE, -5.0% enemy crit",
			"beast.skill.nghe_dong.pha_hon", "CC Cleanse", "", [3]int64{75, 60, 45},
			map[string][2]int64{"DEFENSE": {15, 1}, "MAX_HP": {150, 1}, "ACCURACY": {7, 1000}}},
		{"beast.moc.huou_sao", "MOC", "beast.skill.huou_sao.linh_duoc",
			"Increases owner's received healing effectiveness and passively regenerates HP.",
			"+12.0% incoming heal, +0.4% MAX_HP every 4s",
			"beast.skill.huou_sao.ho_menh", "Emergency Shield", "", [3]int64{90, 75, 60},
			map[string][2]int64{"MAX_HP": {200, 1}, "HP_REGEN": {4, 1}, "DEFENSE": {10, 1}}},
		{"beast.moc.chim_lac", "MOC", "beast.skill.chim_lac.phi_vu",
			"Increases owner's `ATTACK_SPEED`.", "+8.0% ATTACK_SPEED",
			"beast.skill.chim_lac.truy_kich", "Kill/Assist Resource Restore", "3% MAX_MP", [3]int64{60, 50, 45},
			map[string][2]int64{"ATTACK": {15, 1}, "ATTACK_SPEED": {13, 1000}, "MAX_MP": {50, 1}}},
		{"beast.thuy.rai_ca", "THUY", "beast.skill.rai_ca.luot_song",
			"Increases owner's `DODGE_CHANCE` and grants slow resistance.",
			"+8.5% DODGE_CHANCE, +20% slow resist",
			"beast.skill.rai_ca.thoat_xac", "Mist Escape", "", [3]int64{60, 50, 45},
			map[string][2]int64{"DODGE_CHANCE": {13, 1000}, "ATTACK": {13, 1}, "MAX_HP": {116, 1}}},
		{"beast.thuy.rua_than", "THUY", "beast.skill.rua_than.mai_rua",
			"Increases owner's `DAMAGE_REDUCTION`.", "+10.0% DAMAGE_REDUCTION",
			"beast.skill.rua_than.thuy_khien", "Emergency Shield", "", [3]int64{80, 65, 50},
			map[string][2]int64{"DAMAGE_REDUCTION": {17, 1000}, "DEFENSE": {18, 1}, "MAX_HP": {183, 1}}},
		{"beast.hoa.ga_than", "HOA", "beast.skill.ga_than.hoa_long",
			"Amplifies all fire and burn damage dealt by the owner.", "+25.0% Burn damage",
			"beast.skill.ga_than.kich_no", "Kill/Assist Resource Restore", "3% MAX_HP", [3]int64{75, 60, 45},
			map[string][2]int64{"ATTACK": {22, 1}, "CRIT_CHANCE": {8, 1000}, "MAX_MP": {66, 1}}},
		{"beast.hoa.hoa_diep", "HOA", "beast.skill.hoa_diep.hoa_tran",
			"Increases owner's fire elemental penetration and adds AoE fire splash to basic attacks.",
			"+15.0% Fire Pen, +25% splash in 1.8m",
			"beast.skill.hoa_diep.boc_liet", "Anti-Heal", "", [3]int64{65, 50, 45},
			map[string][2]int64{"ATTACK": {20, 1}, "MAX_MP": {58, 1}, "CRIT_CHANCE": {8, 1000}}},
		{"beast.tho.coc_than", "THO", "beast.skill.coc_than.tran_tho",
			"Increases owner's `MAX_HP` and grants heavy displacement resistance.",
			"+8.0% MAX_HP, +20% knockback resist",
			"beast.skill.coc_than.khi_bao", "Emergency Shield", "", [3]int64{90, 85, 80},
			map[string][2]int64{"MAX_HP": {250, 1}, "DEFENSE": {15, 1}, "DODGE_CHANCE": {7, 1000}}},
		{"beast.tho.trau_dong", "THO", "beast.skill.trau_dong.thiet_nguu",
			"Increases owner's `DEFENSE`.", "+8.0% DEFENSE",
			"beast.skill.trau_dong.chan_dia", "CC Cleanse", "", [3]int64{90, 75, 60},
			map[string][2]int64{"DEFENSE": {20, 1}, "MAX_HP": {233, 1}, "DAMAGE_REDUCTION": {13, 1000}}},
	}
}

type testEquip struct {
	id    string
	slot  string
	req   int64
	stats config.Value
}

// launchEquipment mirrors the 18-item Beast Equipment Roster (6 tiers × 3
// slots, fixed primary stats as emitted fixed_stat lists).
func launchEquipment() []testEquip {
	mk := func(tier, req int64, slot string, statPairs ...[2]any) testEquip {
		var stats []config.Value
		for _, p := range statPairs {
			stat := p[0].(string)
			var v config.Value
			var err error
			switch n := p[1].(type) {
			case int:
				v, err = config.VRat(int64(n), 1)
			case float64:
				// decimal d -> rat over 10^k without float math
				s := strings.TrimRight(strings.TrimRight(
					strconv.FormatFloat(n, 'f', 4, 64), "0"), ".")
				ip, frac, _ := strings.Cut(s, ".")
				den := int64(1)
				for range frac {
					den *= 10
				}
				whole, _ := strconv.ParseInt(ip+frac, 10, 64)
				v, err = config.VRat(whole, den)
			}
			if err != nil {
				panic(err)
			}
			stats = append(stats, config.VRec(map[string]config.Value{
				"value": v, "stat": config.VStr(stat),
			}))
		}
		return testEquip{
			id:    "item.beast_eq.t" + strconv.FormatInt(tier, 10) + "." + slot,
			slot:  slot,
			req:   req,
			stats: config.VList(stats...),
		}
	}
	return []testEquip{
		mk(1, 10, "vong_co", [2]any{"ATTACK", 2}, [2]any{"CRIT_CHANCE", 0.002}),
		mk(1, 10, "ao_giap", [2]any{"MAX_HP", 16}, [2]any{"DEFENSE", 2}),
		mk(1, 10, "linh_chau", [2]any{"DODGE_CHANCE", 0.002}, [2]any{"ATTACK_SPEED", 0.002}),
		mk(2, 20, "vong_co", [2]any{"ATTACK", 4}, [2]any{"CRIT_CHANCE", 0.003}),
		mk(2, 20, "ao_giap", [2]any{"MAX_HP", 36}, [2]any{"DEFENSE", 5}),
		mk(2, 20, "linh_chau", [2]any{"DODGE_CHANCE", 0.003}, [2]any{"ATTACK_SPEED", 0.003}),
		mk(3, 30, "vong_co", [2]any{"ATTACK", 7}, [2]any{"CRIT_CHANCE", 0.003}),
		mk(3, 30, "ao_giap", [2]any{"MAX_HP", 66}, [2]any{"DEFENSE", 9}),
		mk(3, 30, "linh_chau", [2]any{"DODGE_CHANCE", 0.003}, [2]any{"ATTACK_SPEED", 0.003}),
		mk(4, 40, "vong_co", [2]any{"ATTACK", 11}, [2]any{"CRIT_CHANCE", 0.004}),
		mk(4, 40, "ao_giap", [2]any{"MAX_HP", 108}, [2]any{"DEFENSE", 14}),
		mk(4, 40, "linh_chau", [2]any{"DODGE_CHANCE", 0.004}, [2]any{"ATTACK_SPEED", 0.004}),
		mk(5, 50, "vong_co", [2]any{"ATTACK", 17}, [2]any{"CRIT_CHANCE", 0.005}),
		mk(5, 50, "ao_giap", [2]any{"MAX_HP", 158}, [2]any{"DEFENSE", 20}),
		mk(5, 50, "linh_chau", [2]any{"DODGE_CHANCE", 0.005}, [2]any{"ATTACK_SPEED", 0.005}),
		mk(6, 60, "vong_co", [2]any{"ATTACK", 25}, [2]any{"CRIT_CHANCE", 0.007}),
		mk(6, 60, "ao_giap", [2]any{"MAX_HP", 233}, [2]any{"DEFENSE", 27}),
		mk(6, 60, "linh_chau", [2]any{"DODGE_CHANCE", 0.007}, [2]any{"ATTACK_SPEED", 0.006}),
	}
}

// beastCandidate builds a minimal complete candidate carrying the 10-beast
// roster with P1/P2 passives plus the compiled beast_budget_check reference
// params (the fence's current pinned values are read data-driven; only the
// reference_lv60_* key presence feeds the check).
func beastCandidate(t *testing.T) *config.CandidateSnapshot {
	t.Helper()
	defs := map[string]*config.Family{}
	fam := func(name string, keys ...string) *config.Family {
		f := config.NewFamily(name, keys...)
		defs[name] = f
		return f
	}
	roster := fam("beast", "beast_id")
	detail := fam("beast_detail", "beast_id")
	rules := fam("beast_passive_rule", "beast_id")
	equip := fam("beast_equipment", "item_id")
	for _, e := range launchEquipment() {
		put(t, equip, []config.Value{config.VStr(e.id)}, map[string]config.Value{
			"item_id":    config.VStr(e.id),
			"slot":       config.VStr(e.slot),
			"req_level":  config.VInt(e.req),
			"fixed_stat": e.stats,
		})
	}
	for _, b := range launchBeasts() {
		put(t, roster, []config.Value{config.VStr(b.id)}, map[string]config.Value{
			"beast_id": config.VStr(b.id), "element": config.VStr(b.element),
		})
		var curves []config.Value
		for stat, v := range b.base {
			lv60, err := config.VRat(v[0], v[1])
			if err != nil {
				t.Fatalf("base stat %s %v: %v", stat, v, err)
			}
			curves = append(curves, config.VRec(map[string]config.Value{
				"stat": config.VStr(stat), "curve": config.VStr("linear"),
				"lv1": config.VInt(0), "lv60": lv60,
			}))
		}
		put(t, detail, []config.Value{config.VStr(b.id)}, map[string]config.Value{
			"beast_id":   config.VStr(b.id),
			"element":    config.VStr(b.element),
			"base_stats": config.VList(curves...),
			"passive1": config.VRec(map[string]config.Value{
				"skill_id":     config.VStr(b.p1ID),
				"lv1_payload":  config.VStr("+1.0%"),
				"lv60_payload": config.VStr(b.p1Lv60),
				"description":  config.VStr(b.p1Desc),
			}),
			"passive2": config.VRec(map[string]config.Value{
				"skill_id":       config.VStr(b.p2ID),
				"clutch":         config.VBool(true),
				"p2_type":        config.VStr(b.p2Type),
				"icd_seconds":    config.VList(config.VInt(b.icd[0]), config.VInt(b.icd[1]), config.VInt(b.icd[2])),
				"unlock_level":   config.VInt(20),
				"upgrade_levels": config.VList(config.VInt(40), config.VInt(60)),
			}),
		})
		put(t, rules, []config.Value{config.VStr(b.id)}, map[string]config.Value{
			"beast_id":    config.VStr(b.id),
			"p2_type":     config.VStr(b.p2Type),
			"p2_payload":  config.VStr(b.p2Pay),
			"icd_seconds": config.VList(config.VInt(b.icd[0]), config.VInt(b.icd[1]), config.VInt(b.icd[2])),
		})
	}
	params := config.NewFamily("validation_parameters", "family")
	put(t, params, []config.Value{config.VStr("beast_budget_check")}, map[string]config.Value{
		"family":      config.VStr("beast_budget_check"),
		"key_columns": config.VStrs("source", "line"),
		"records": config.VList(config.VRec(map[string]config.Value{
			"key": config.VList(config.VStr("fence"), config.VInt(1)),
			"fields": config.VRec(map[string]config.Value{
				"reference_lv60_max_hp":  config.VInt(4794),
				"reference_lv60_attack":  config.VInt(724),
				"reference_lv60_defense": config.VInt(426),
			}),
		})),
	})
	return &config.CandidateSnapshot{
		AuthoringSchemaVersion: config.AuthoringSchemaVersion,
		ContentSchemaVersion:   config.ContentSchemaVersion,
		RuleVersions:           map[string]int{},
		Definitions:            defs,
		ValidationParameters:   params,
	}
}

// setP1 overwrites one beast's compiled Lv60 payload.
func setP1(t *testing.T, c *config.CandidateSnapshot, id, lv60 string) {
	t.Helper()
	rec, ok := c.Definitions["beast_detail"].Records[config.KeyString([]config.Value{config.VStr(id)})]
	if !ok {
		t.Fatalf("no beast_detail %s", id)
	}
	p1 := rec.Fields["passive1"].Rec
	p1["lv60_payload"] = config.VStr(lv60)
}

// setP2Rule overwrites fields of one beast's compiled passive rule row.
func setP2Rule(t *testing.T, c *config.CandidateSnapshot, id string, fields map[string]config.Value) {
	t.Helper()
	rec, ok := c.Definitions["beast_passive_rule"].Records[config.KeyString([]config.Value{config.VStr(id)})]
	if !ok {
		t.Fatalf("no beast_passive_rule %s", id)
	}
	for k, v := range fields {
		rec.Fields[k] = v
	}
}

func hasDiag(d config.Diagnostics, code config.DiagnosticCode) bool {
	for _, e := range d {
		if e.Code == code {
			return true
		}
	}
	return false
}

// setBaseStat overwrites one base-stat curve's lv60 value in beast_detail.
func setBaseStat(t *testing.T, c *config.CandidateSnapshot, id, stat string, num, den int64) {
	t.Helper()
	rec, ok := c.Definitions["beast_detail"].Records[config.KeyString([]config.Value{config.VStr(id)})]
	if !ok {
		t.Fatalf("no beast_detail %s", id)
	}
	bs := rec.Fields["base_stats"]
	v, err := config.VRat(num, den)
	if err != nil {
		t.Fatalf("rat: %v", err)
	}
	for i, e := range bs.Elems {
		if e.Rec["stat"].Str == stat {
			e.Rec["lv60"] = v
			bs.Elems[i] = e
			return
		}
	}
	t.Fatalf("base stat %s not found on %s", stat, id)
}

// ---- named tests ------------------------------------------------------------

// TestBeastPassiveBudgetRulesAD: each Rule A-D ceiling rejects a Passive 1
// that exceeds it at Lv60, citing rule and value; a missing
// reference_lv60_* param is a compile error.
func TestBeastPassiveBudgetRulesAD(t *testing.T) {
	cases := []struct {
		name    string
		beast   string
		payload string
	}{
		{"ruleA", "beast.tho.coc_than", "+9.0% MAX_HP, +20% knockback resist"},
		{"ruleB", "beast.thuy.rai_ca", "+11.0% DODGE_CHANCE, +20% slow resist"},
		{"ruleC", "beast.moc.chim_lac", "+9.0% ATTACK_SPEED"},
		{"ruleD", "beast.kim.ho_vang", "+21.0%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := beastCandidate(t)
			setP1(t, c, tc.beast, tc.payload)
			d := Check(c)
			if !d.HasErrors() || !hasDiag(d, config.DiagBalanceGuardrail) {
				t.Fatalf("want balance-guardrail rejection, got %v", d)
			}
		})
	}
	t.Run("missingReference", func(t *testing.T) {
		c := beastCandidate(t)
		rec := c.ValidationParameters.Records[config.KeyString([]config.Value{config.VStr("beast_budget_check")})]
		rec.Fields["records"] = config.VList()
		d := Check(c)
		if !d.HasErrors() || !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want missing-reference error, got %v", d)
		}
	})
}

// TestTenLaunchBeastsPassiveValidation: the compiled 10-beast roster passes
// the whole suite without rejection.
func TestTenLaunchBeastsPassiveValidation(t *testing.T) {
	c := beastCandidate(t)
	if d := Check(c); d.HasErrors() {
		t.Fatalf("10 launch beasts must pass: %v", d)
	}
}

// TestPassive2AuthoredIcdLadderDistinct: out-of-range ICDs and duplicate
// effective ICDs (post-[45,90]-clamp) are rejected (OBJ-SBB-003).
func TestPassive2AuthoredIcdLadderDistinct(t *testing.T) {
	t.Run("outOfRange", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.thuy.rai_ca", map[string]config.Value{
			"icd_seconds": config.VList(config.VInt(40), config.VInt(50), config.VInt(60)),
		})
		if d := Check(c); !hasDiag(d, config.DiagValueOutOfBounds) {
			t.Fatalf("want ICD range rejection, got %v", d)
		}
	})
	t.Run("duplicateEffective", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.thuy.rai_ca", map[string]config.Value{
			"icd_seconds": config.VList(config.VInt(45), config.VInt(45), config.VInt(60)),
		})
		if d := Check(c); !hasDiag(d, config.DiagValueOutOfBounds) {
			t.Fatalf("want duplicate-ICD rejection, got %v", d)
		}
	})
	t.Run("legal", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.thuy.rai_ca", map[string]config.Value{
			"icd_seconds": config.VList(config.VInt(90), config.VInt(70), config.VInt(45)),
		})
		if d := Check(c); d.HasErrors() {
			t.Fatalf("legal ladder must pass, got %v", d)
		}
	})
}

// TestPassive2LegalTypeAndFixedPayload: P2 must be one legal type with that
// type's fixed payload (non-restore types carry none).
func TestPassive2LegalTypeAndFixedPayload(t *testing.T) {
	t.Run("illegalType", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.thuy.rai_ca", map[string]config.Value{
			"p2_type": config.VStr("Invulnerability"),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want illegal-type rejection, got %v", d)
		}
	})
	t.Run("payloadDiffersFromFixed", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.kim.ho_vang", map[string]config.Value{
			"p2_payload": config.VStr("30% heal cut"),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want fixed-payload rejection, got %v", d)
		}
	})
}

// TestPassive2RiderRejected: riders (knockback/damage/stat modifier/etc.) on
// a P2 type or payload are rejected, as are banned effects.
func TestPassive2RiderRejected(t *testing.T) {
	t.Run("typeRider", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.tho.trau_dong", map[string]config.Value{
			"p2_type": config.VStr("CC Cleanse + knockback"),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want type-rider rejection, got %v", d)
		}
	})
	t.Run("bannedEffect", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.kim.ho_vang", map[string]config.Value{
			"p2_type":    config.VStr("Anti-Heal"),
			"p2_payload": config.VStr("apply blind"),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want banned-effect rejection, got %v", d)
		}
	})
	t.Run("restoreRider", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.moc.chim_lac", map[string]config.Value{
			"p2_payload": config.VStr("3% MAX_MP, +5% ATTACK"),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want restore-rider rejection, got %v", d)
		}
	})
}

// TestBeastBudgetCheckFlatStatBudget: a beast whose transferred flat-stat
// total (base + equipment, no resonance) exceeds 0.12 x reference is rejected
// citing stat and beast.
func TestBeastBudgetCheckFlatStatBudget(t *testing.T) {
	c := beastCandidate(t)
	// coc_than MAX_HP 250+233 = 483 <= 575; raise base so total breaks the cap.
	setBaseStat(t, c, "beast.tho.coc_than", "MAX_HP", 900, 1)
	if d := Check(c); !hasDiag(d, config.DiagBalanceGuardrail) {
		t.Fatalf("want flat-stat budget rejection, got %v", d)
	}
}

// TestBeastBudgetCheckResonanceAdjusted: Tương Sinh resonance applies the
// floor(x1.08) multiplier before the cap — a total inside the raw bound but
// pushed over by resonance is rejected.
func TestBeastBudgetCheckResonanceAdjusted(t *testing.T) {
	c := beastCandidate(t)
	// coc_than MAX_HP: 301+233 = 534 <= 575.28 raw, but floor(534*1.08)=576 > 575.28.
	setBaseStat(t, c, "beast.tho.coc_than", "MAX_HP", 301, 1)
	if d := Check(c); !hasDiag(d, config.DiagBalanceGuardrail) {
		t.Fatalf("want resonance-adjusted rejection, got %v", d)
	}
}

// TestBeastBudgetCheckViolationRejects: a clearly over-budget total is
// rejected under both resonance states with the offending stat cited.
func TestBeastBudgetCheckViolationRejects(t *testing.T) {
	c := beastCandidate(t)
	// ga_than ATTACK: 500+25 = 525 raw >> 0.12 x 724/723; fails without resonance.
	setBaseStat(t, c, "beast.hoa.ga_than", "ATTACK", 500, 1)
	d := Check(c)
	if !hasDiag(d, config.DiagBalanceGuardrail) {
		t.Fatalf("want over-budget rejection, got %v", d)
	}
	found := false
	for _, e := range d {
		if strings.Contains(e.Message, "beast.hoa.ga_than") && strings.Contains(e.Message, "ATTACK") {
			found = true
		}
	}
	if !found {
		t.Fatalf("rejection must cite beast and stat, got %v", d)
	}
}

// TestResourceRestoreCaps: Kill/Assist Resource Restore payload pct must be
// <= 0.03 of MAX_MP or MAX_HP.
func TestResourceRestoreCaps(t *testing.T) {
	t.Run("overCap", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.moc.chim_lac", map[string]config.Value{
			"p2_payload": config.VStr("4% MAX_MP"),
		})
		if d := Check(c); !hasDiag(d, config.DiagBalanceGuardrail) {
			t.Fatalf("want >3%% rejection, got %v", d)
		}
	})
	t.Run("missingPayload", func(t *testing.T) {
		c := beastCandidate(t)
		setP2Rule(t, c, "beast.moc.chim_lac", map[string]config.Value{
			"p2_payload": config.VStr(""),
		})
		if d := Check(c); !hasDiag(d, config.DiagIntegrationCheck) {
			t.Fatalf("want missing-payload rejection, got %v", d)
		}
	})
	t.Run("legalBoth", func(t *testing.T) {
		c := beastCandidate(t)
		if d := Check(c); d.HasErrors() {
			t.Fatalf("3%% MAX_MP/MAX_HP payloads must pass, got %v", d)
		}
	})
}
