// Package beast evaluates Spirit Beast passive budgets at content-activation
// time (IMP-050). It re-derives the invariants of spirit_beasts.md § Passive
// Budget Rules A-D and § Passive 2 legal types independently of the content
// compiler against the emitted CandidateSnapshot: beast_detail passive
// payloads, beast_passive_rule rows, and the compiled beast_budget_check
// reference parameters (../07_content/spirit_beast_catalog.md Passive Rules +
// Power Budget Note). A non-empty Diagnostics result blocks activation.
package beast

import (
	"regexp"
	"strconv"
	"strings"

	"thinhthan/internal/config"
)

// Check runs the full passive-budget suite on a compiled candidate.
func Check(c *config.CandidateSnapshot) config.Diagnostics {
	var d config.Diagnostics
	if c == nil {
		d.Addf(config.DiagIntegrationCheck, "", 0, "nil candidate snapshot")
		return d
	}
	refs := budgetRefs(c, &d)
	checkPassive1(c, &d)
	checkPassive2(c, &d)
	checkFlatStatBudget(c, &d, refs)
	return d
}

// ---- compiled-input accessors ---------------------------------------------

func familyRecs(c *config.CandidateSnapshot, name string) []config.Record {
	f := c.Definitions[name]
	if f == nil {
		return nil
	}
	out := make([]config.Record, 0, len(f.Records))
	for _, k := range f.SortedKeys() {
		out = append(out, f.Records[config.KeyString(k)])
	}
	return out
}

// paramRecords returns the field maps of every record inside the folded
// validation_parameters sub-family `family`.
func paramRecords(c *config.CandidateSnapshot, family string) []map[string]config.Value {
	if c.ValidationParameters == nil {
		return nil
	}
	rec, ok := c.ValidationParameters.Records[config.KeyString([]config.Value{config.VStr(family)})]
	if !ok {
		return nil
	}
	recs, ok := rec.Fields["records"]
	if !ok || recs.Kind != config.KindList {
		return nil
	}
	out := make([]map[string]config.Value, 0, len(recs.Elems))
	for _, e := range recs.Elems {
		if e.Kind != config.KindRecord {
			continue
		}
		if fv, ok := e.Rec["fields"]; ok && fv.Kind == config.KindRecord {
			out = append(out, fv.Rec)
		}
	}
	return out
}

func fieldStr(r config.Record, name string) (string, bool) {
	v, ok := r.Fields[name]
	if !ok || v.Kind != config.KindString {
		return "", false
	}
	return v.Str, true
}

// ---- required budget parameters -------------------------------------------

// budgetRefs collects the compiled beast_budget_check fence records'
// reference_lv60_* stats; every required stat must exist as an integer — a
// missing or unparseable record is a compile error that blocks activation.
// The returned map feeds the flat-stat budget evaluation.
func budgetRefs(c *config.CandidateSnapshot, d *config.Diagnostics) map[string]int64 {
	refs := map[string]int64{}
	for _, fields := range paramRecords(c, "beast_budget_check") {
		for name, v := range fields {
			if !strings.HasPrefix(name, "reference_lv60_") {
				continue
			}
			if v.Kind != config.KindInt {
				d.Addf(config.DiagIntegrationCheck, "", 0,
					"beast_budget_check %s is not an integer", name)
				continue
			}
			refs[name] = v.Int
		}
	}
	for _, req := range []string{"reference_lv60_max_hp", "reference_lv60_attack", "reference_lv60_defense"} {
		if _, ok := refs[req]; !ok {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"beast_budget_check params missing %s", req)
		}
	}
	return refs
}

// ---- flat-stat budget (beast_budget_check) ---------------------------------

// budgetStats are the three reference-stat pool members covered by the 12%
// flat-stat ceiling (spirit_beasts.md § Validation flat-stat fence).
var budgetStats = []struct {
	stat string
	ref  string
}{
	{"MAX_HP", "reference_lv60_max_hp"},
	{"ATTACK", "reference_lv60_attack"},
	{"DEFENSE", "reference_lv60_defense"},
}

// ratFloor returns floor(r) for non-negative rationals.
func ratFloor(r config.Rat) int64 {
	return r.Num / r.Den
}

func ratAdd(a, b config.Rat) config.Rat {
	// Zero-value Rat{} (missing stat) is rational 0.
	if a.Den == 0 {
		a.Den = 1
	}
	if b.Den == 0 {
		b.Den = 1
	}
	return config.Rat{Num: a.Num*b.Den + b.Num*a.Den, Den: a.Den * b.Den}
}

func ratMulInt(r config.Rat, num, den int64) config.Rat {
	return config.Rat{Num: r.Num * num, Den: r.Den * den}
}

// exceedsBudget reports whether r > bound*num/den exactly.
func exceedsBudget(r config.Rat, bound int64, num, den int64) bool {
	return r.Num*den > bound*num*r.Den
}

// checkFlatStatBudget evaluates spirit_beasts.md § Validation flat-stat
// fence data-driven: for every beast, every emitted equipment tier and both
// resonance states, resonance_adjusted_total(stat) <= 0.12 x reference.
// resonance_adjusted_total = floor(transferred x 1.08) when Tương Sinh
// resonance applies, else transferred.
func checkFlatStatBudget(c *config.CandidateSnapshot, d *config.Diagnostics, refs map[string]int64) {
	for _, bs := range budgetStats {
		if _, ok := refs[bs.ref]; !ok {
			return // missing reference already diagnosed
		}
	}
	// Equipment stat totals grouped by tier (req_level bands t1..t6).
	tiers := map[int64]map[string]config.Rat{}
	for _, r := range familyRecs(c, "beast_equipment") {
		lv, ok := r.Fields["req_level"]
		if !ok || lv.Kind != config.KindInt {
			continue
		}
		fs, ok := r.Fields["fixed_stat"]
		if !ok || fs.Kind != config.KindList {
			continue
		}
		tot, ok := tiers[lv.Int]
		if !ok {
			tot = map[string]config.Rat{}
			tiers[lv.Int] = tot
		}
		for _, e := range fs.Elems {
			if e.Kind != config.KindRecord {
				continue
			}
			sv, sok := e.Rec["stat"]
			vv, vok := e.Rec["value"]
			if !sok || !vok || sv.Kind != config.KindString || vv.Kind != config.KindRational {
				continue
			}
			tot[sv.Str] = ratAdd(tot[sv.Str], vv.Rat)
		}
	}
	for _, r := range familyRecs(c, "beast_detail") {
		id, _ := fieldStr(r, "beast_id")
		base := map[string]config.Rat{}
		if bs, ok := r.Fields["base_stats"]; ok && bs.Kind == config.KindList {
			for _, e := range bs.Elems {
				if e.Kind != config.KindRecord {
					continue
				}
				sv, sok := e.Rec["stat"]
				vv, vok := e.Rec["lv60"]
				if sok && vok && sv.Kind == config.KindString && vv.Kind == config.KindRational {
					base[sv.Str] = vv.Rat
				}
			}
		}
		for tier, eq := range tiers {
			for _, resonance := range []bool{false, true} {
				for _, bs := range budgetStats {
					total := ratAdd(base[bs.stat], eq[bs.stat])
					if resonance {
						adjusted := ratFloor(ratMulInt(total, 108, 100))
						if adjusted*100 > refs[bs.ref]*12 {
							d.Addf(config.DiagBalanceGuardrail, "", 0,
								"%s %s resonance-adjusted total %d at tier %d exceeds 0.12 x %s (%d)",
								id, bs.stat, adjusted, tier, bs.ref, refs[bs.ref])
						}
					} else if exceedsBudget(total, refs[bs.ref], 100, 12) {
						d.Addf(config.DiagBalanceGuardrail, "", 0,
							"%s %s transferred total %s at tier %d exceeds 0.12 x %s (%d)",
							id, bs.stat, total.String(), tier, bs.ref, refs[bs.ref])
					}
				}
			}
		}
	}
}

// ---- Passive 1: Rules A-D --------------------------------------------------

// component is one parsed Passive-1 payload term (e.g. "+8.0% MAX_HP").
type component struct {
	effect string     // canonical effect key into ruleCaps / uncappedEffects
	text   string     // authored effect text, for diagnostics
	v      config.Rat // signed magnitude (fraction; % parsed to /100)
}

// ruleCaps maps each canonical Passive-1 effect to its budget rule letter and
// Lv60 ceiling (spirit_beasts.md § Rules A-D + Passive budget audit table).
var ruleCaps = map[string]struct {
	rule string
	cap  config.Rat
}{
	// Rule A — reference-stat pool members, 8.0% absolute at Lv60.
	"MAX_HP":  {"A", ratCap(8, 100)},
	"ATTACK":  {"A", ratCap(8, 100)},
	"DEFENSE": {"A", ratCap(8, 100)},
	// Rule B — 25% of the stat's global cap.
	"DAMAGE_REDUCTION":   {"B", ratCap(10, 100)},
	"DODGE_CHANCE":       {"B", ratCap(10, 100)},
	"ACCURACY":           {"B", ratCap(10, 100)},
	"CRIT_CHANCE":        {"B", ratCap(15, 100)},
	"COOLDOWN_REDUCTION": {"B", ratCap(875, 10000)},
	// Rule C — ATTACK_SPEED (no declared global cap).
	"ATTACK_SPEED": {"C", ratCap(8, 100)},
	// Rule D — non-transferred / sustain passives.
	"CRIT_DAMAGE":    {"D", ratCap(20, 100)},
	"burn_amp":       {"D", ratCap(25, 100)},
	"fire_pen":       {"D", ratCap(15, 100)},
	"incoming_heal":  {"D", ratCap(12, 100)},
	"aoe_splash":     {"D", ratCap(25, 100)},
	"control_resist": {"D", ratCap(20, 100)},
	"enemy_crit":     {"D", ratCap(5, 100)},
}

// uncappedEffects are legal authored components with no ceiling under Rules
// A-D (periodic sustain ticks, e.g. "+0.4% MAX_HP every 4s").
var uncappedEffects = map[string]bool{
	"sustain_regen": true,
}

// bannedEffects are forbidden passive components anywhere in a P1/P2 payload
// (spirit_beasts.md: Blind, accuracy-100, pre-mitigation reflect, backstab,
// iframe/invulnerability, projectile speed as a transferred stat).
var bannedEffects = []string{
	"blind",
	"accuracy-100",
	"accuracy_100",
	"accuracy 100",
	"pre-mitigation reflect",
	"backstab",
	"iframe",
	"invulnerab",
	"projectile speed",
	"projectile_speed",
	"projectile-speed",
}

var componentRe = regexp.MustCompile(`^([+-]?[0-9.]+)%\s*(.*)$`)
var sustainRe = regexp.MustCompile(`every\s+[0-9.]+\s*s\b`)
var radiusRe = regexp.MustCompile(`\s+in\s+[0-9.]+\s*m\b`)
var backtickStatRe = regexp.MustCompile("`([A-Z][A-Z0-9_]+)`")

// effectKey normalizes authored effect text to a canonical key.
func effectKey(text string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimRight(t, ".")
	if sustainRe.MatchString(t) {
		return "sustain_regen"
	}
	t = strings.TrimSpace(radiusRe.ReplaceAllString(t, ""))
	switch t {
	case "max_hp":
		return "MAX_HP"
	case "attack", "atk":
		return "ATTACK"
	case "defense", "def":
		return "DEFENSE"
	case "damage_reduction", "damage reduction":
		return "DAMAGE_REDUCTION"
	case "dodge_chance", "dodge chance", "dodge":
		return "DODGE_CHANCE"
	case "accuracy":
		return "ACCURACY"
	case "crit_chance", "crit chance":
		return "CRIT_CHANCE"
	case "cooldown_reduction", "cooldown reduction":
		return "COOLDOWN_REDUCTION"
	case "attack_speed", "attack speed":
		return "ATTACK_SPEED"
	case "crit_damage", "crit damage":
		return "CRIT_DAMAGE"
	case "burn damage", "fire damage", "burn/fire damage", "burn and fire damage":
		return "burn_amp"
	case "fire pen", "fire penetration":
		return "fire_pen"
	case "incoming heal", "incoming healing", "heal effectiveness", "received healing":
		return "incoming_heal"
	case "splash", "aoe splash":
		return "aoe_splash"
	case "knockback resist", "slow resist", "control resistance", "control resist":
		return "control_resist"
	case "enemy crit", "enemy-crit debuff", "enemy crit debuff":
		return "enemy_crit"
	}
	return ""
}

// descEffect resolves the effect of a bare "+N%" component from the stat
// named in the passive description (e.g. "Increases owner's `CRIT_DAMAGE`").
func descEffect(desc string) string {
	for _, m := range backtickStatRe.FindAllStringSubmatch(desc, -1) {
		if k := effectKey(m[1]); k != "" {
			return k
		}
	}
	return ""
}

// parsePct parses "8.0" / "-5.0" style percentage strings into a fraction Rat
// (num/den, den includes the /100).
func parsePct(s string) (config.Rat, bool) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	ip, frac, _ := strings.Cut(s, ".")
	whole, err := strconv.ParseInt(ip, 10, 64)
	if err != nil || whole < 0 {
		return config.Rat{}, false
	}
	num, den := whole, int64(1)
	if frac != "" {
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil || f < 0 {
			return config.Rat{}, false
		}
		for range frac {
			den *= 10
		}
		num = num*den + f
	}
	if neg {
		num = -num
	}
	r, err := config.ReduceRat(num, den*100)
	if err != nil {
		return config.Rat{}, false
	}
	return r, true
}

func ratCap(num, den int64) config.Rat {
	r, err := config.ReduceRat(num, den)
	if err != nil {
		panic("invalid cap")
	}
	return r
}

// exceedsCap reports whether |v| > cap exactly (base-10 rational compare).
func exceedsCap(v, cap config.Rat) bool {
	n := v.Num
	if n < 0 {
		n = -n
	}
	return n*cap.Den > cap.Num*v.Den
}

// parsePassive1 splits a Lv60 payload into components, resolving bare
// "+N%" terms against the stat named in the description.
func parsePassive1(desc, lv60Payload string) ([]component, []string) {
	var comps []component
	var errs []string
	for _, part := range strings.Split(lv60Payload, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := componentRe.FindStringSubmatch(part)
		if m == nil {
			errs = append(errs, "unparseable component "+strconv.Quote(part))
			continue
		}
		v, ok := parsePct(m[1])
		if !ok {
			errs = append(errs, "bad percent "+strconv.Quote(part))
			continue
		}
		raw := strings.TrimSpace(m[2])
		key := ""
		if raw != "" {
			key = effectKey(raw)
		} else {
			key = descEffect(desc)
		}
		comps = append(comps, component{effect: key, text: part, v: v})
	}
	return comps, errs
}

// hasBanned reports whether any banned-effect token appears in text.
func bannedIn(text string) string {
	t := strings.ToLower(text)
	for _, b := range bannedEffects {
		if strings.Contains(t, b) {
			return b
		}
	}
	return ""
}

func checkPassive1(c *config.CandidateSnapshot, d *config.Diagnostics) {
	for _, r := range familyRecs(c, "beast_detail") {
		id, _ := fieldStr(r, "beast_id")
		p1, ok := r.Fields["passive1"]
		if !ok || p1.Kind != config.KindRecord {
			d.Addf(config.DiagIntegrationCheck, "", 0, "%s missing passive1", id)
			continue
		}
		lv60, _ := p1.Rec["lv60_payload"]
		desc := ""
		if dv, ok := p1.Rec["description"]; ok && dv.Kind == config.KindString {
			desc = dv.Str
		}
		if lv60.Kind != config.KindString || lv60.Str == "" {
			d.Addf(config.DiagIntegrationCheck, "", 0, "%s passive1 missing lv60_payload", id)
			continue
		}
		if b := bannedIn(lv60.Str + " " + desc); b != "" {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"%s passive1 uses banned effect %q", id, b)
		}
		comps, errs := parsePassive1(desc, lv60.Str)
		for _, e := range errs {
			d.Addf(config.DiagIntegrationCheck, "", 0, "%s passive1: %s", id, e)
		}
		for _, cp := range comps {
			if cp.effect == "" {
				d.Addf(config.DiagIntegrationCheck, "", 0,
					"%s passive1 component %q maps to no budget rule", id, cp.text)
				continue
			}
			if uncappedEffects[cp.effect] {
				continue
			}
			rc, ok := ruleCaps[cp.effect]
			if !ok {
				d.Addf(config.DiagIntegrationCheck, "", 0,
					"%s passive1 component %q maps to no budget rule", id, cp.text)
				continue
			}
			if exceedsCap(cp.v, rc.cap) {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"%s passive1 %q exceeds Rule %s cap %s at Lv60", id, cp.text, rc.rule, rc.cap.String())
			}
		}
	}
}

// ---- Passive 2 --------------------------------------------------------------

var legalP2Types = map[string]bool{
	"Anti-Heal":                    true,
	"CC Cleanse":                   true,
	"Emergency Shield":             true,
	"Mist Escape":                  true,
	"Kill/Assist Resource Restore": true,
}

var restoreRe = regexp.MustCompile(`^([0-9.]+)%\s+(MAX_MP|MAX_HP)$`)

// checkPassive2 enforces the compiled P2 contract: one legal type with its
// fixed payload, authored ICD ladder in 45s..90s with distinct effective ICDs
// after the [45s,90s] clamp, Kill/Assist restore pct <= 0.03, and no riders
// or banned modifiers.
func checkPassive2(c *config.CandidateSnapshot, d *config.Diagnostics) {
	for _, r := range familyRecs(c, "beast_passive_rule") {
		id, _ := fieldStr(r, "beast_id")
		typ, _ := fieldStr(r, "p2_type")
		payload, _ := fieldStr(r, "p2_payload")
		if !legalP2Types[typ] {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"%s p2_type %q not in legal set", id, typ)
		}
		if b := bannedIn(typ + " " + payload); b != "" {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"%s passive2 uses banned effect %q", id, b)
		}
		checkP2Payload(id, typ, payload, d)
		checkIcdLadder(id, r, d)
	}
}

// checkP2Payload verifies the type's fixed payload: only Kill/Assist Resource
// Restore carries one, a single `N% MAX_MP|MAX_HP` term with N <= 3.
func checkP2Payload(id, typ, payload string, d *config.Diagnostics) {
	payload = strings.TrimSpace(payload)
	if typ == "Kill/Assist Resource Restore" {
		parts := strings.Split(payload, ",")
		m := restoreRe.FindStringSubmatch(strings.TrimSpace(parts[0]))
		if m == nil {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"%s Kill/Assist payload %q is not `N%% MAX_MP|MAX_HP`", id, payload)
		} else if v, ok := parsePct(m[1]); !ok || exceedsCap(v, ratCap(3, 100)) {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"%s Kill/Assist payload %q exceeds 0.03 cap", id, payload)
		}
		if len(parts) > 1 {
			d.Addf(config.DiagIntegrationCheck, "", 0,
				"%s Kill/Assist payload %q carries a rider component", id, payload)
		}
		return
	}
	if payload != "" {
		d.Addf(config.DiagIntegrationCheck, "", 0,
			"%s p2_type %q fixed payload differs from authored %q", id, typ, payload)
	}
}

// checkIcdLadder requires three authored ICDs in [45,90] whose effective
// values (clamped to [45,90]) are pairwise distinct — two tiers clamping to
// the same value make one upgrade invisible (OBJ-SBB-003).
func checkIcdLadder(id string, r config.Record, d *config.Diagnostics) {
	v, ok := r.Fields["icd_seconds"]
	if !ok || v.Kind != config.KindList || len(v.Elems) != 3 {
		d.Addf(config.DiagIntegrationCheck, "", 0, "%s icd_seconds must be 3 values", id)
		return
	}
	eff := [3]int64{}
	for i, e := range v.Elems {
		if e.Kind != config.KindInt {
			d.Addf(config.DiagIntegrationCheck, "", 0, "%s icd_seconds[%d] not int", id, i)
			return
		}
		icd := e.Int
		if icd < 45 || icd > 90 {
			d.Addf(config.DiagValueOutOfBounds, "", 0,
				"%s ICD %d outside 45..90s", id, icd)
		}
		eff[i] = min(max(icd, 45), 90)
	}
	if eff[0] == eff[1] || eff[1] == eff[2] || eff[0] == eff[2] {
		d.Addf(config.DiagValueOutOfBounds, "", 0,
			"%s ICD ladder %v has duplicate effective ICD", id, eff)
	}
}
