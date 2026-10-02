package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"

	"thinhthan/internal/config"
)

// runValidation applies the config.md §Validation reject list, the
// integration.* rule set, the balance_validation.md hard gates and the
// CAT-001/002/003 checks after references resolve (S14).
func runValidation(c *Ctx) {
	gates := parseBalanceGates(c)
	validateSizeProfiles(c)
	validatePlayableSpaces(c)
	validateSkillGeometry(c, gates)
	validateDisplacementParity(c)
	validateQuestGraph(c)
	validateSoulElements(c)
	validateBarrierPayload(c)
	validateRegisteredStrings(c)
	validateLaunchBudget(c)
	validateBalance(c, gates)
}

// ---------------------------------------------------------------------
// record helpers
// ---------------------------------------------------------------------

func famRecs(c *Ctx, name string) []config.Record {
	f := c.Defs.Families[name]
	if f == nil {
		return nil
	}
	out := make([]config.Record, 0, len(f.Records))
	for _, k := range f.SortedKeys() {
		out = append(out, f.Records[config.KeyString(k)])
	}
	return out
}

func fStr(r config.Record, k string) string {
	v, ok := r.Fields[k]
	if !ok || v.Kind != config.KindString {
		return ""
	}
	return v.Str
}

func fBool(r config.Record, k string) bool {
	v, ok := r.Fields[k]
	return ok && v.Kind == config.KindBool && v.Bool
}

func fNum(r config.Record, k string) float64 {
	v, ok := r.Fields[k]
	if !ok {
		return 0
	}
	switch v.Kind {
	case config.KindInt:
		return float64(v.Int)
	case config.KindRational:
		return float64(v.Rat.Num) / float64(v.Rat.Den)
	default:
		return 0
	}
}

func fInt(r config.Record, k string) int64 {
	v, ok := r.Fields[k]
	if ok && v.Kind == config.KindInt {
		return v.Int
	}
	return 0
}

// geomMM reads a millimetre-valued field from a geometry record, which uses
// bare ints for millimetre fields.
func geomMM(g map[string]config.Value, k string) int64 {
	v, ok := g[k]
	if !ok {
		return 0
	}
	switch v.Kind {
	case config.KindInt:
		return v.Int
	case config.KindRational:
		return v.Rat.Num / v.Rat.Den
	default:
		return 0
	}
}

func geomFields(r config.Record) map[string]config.Value {
	v, ok := r.Fields["geometry"]
	if !ok || v.Kind != config.KindRecord {
		return nil
	}
	return v.Rec
}

// ---------------------------------------------------------------------
// size_profile: exactly one resolved profile per monster/boss
// ---------------------------------------------------------------------

func validateSizeProfiles(c *Ctx) {
	declared := c.Refs.IDs["size_profile"]
	for _, fam := range []string{"monster", "boss"} {
		for _, r := range famRecs(c, fam) {
			id := fStr(r, fam+"_id")
			sp := fStr(r, "size_profile")
			if sp == "" {
				c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
					"%s %q has no size_profile", fam, id)
				continue
			}
			if _, ok := declared[sp]; !ok {
				c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
					"%s %q size_profile %q is not a declared profile", fam, id, sp)
			}
		}
	}
}

// ---------------------------------------------------------------------
// playable space geometry: exact bounds + layout per compiled space
// ---------------------------------------------------------------------

func validatePlayableSpaces(c *Ctx) {
	f := c.Geom
	spaces := f.Families["spaces"]
	if spaces == nil {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"no compiled geometry.spaces family")
		return
	}
	expected := expectedSpaceIDs(c)
	compiled := map[string]bool{}
	for _, r := range spaces.Records {
		compiled[fStr(r, "space_id")] = true
	}
	for id := range expected {
		if !compiled[id] {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"declared space %q has no compiled geometry row", id)
		}
	}
	if expected != nil && len(spaces.Records) != len(expected) {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"geometry.spaces row count %d != %d declared playable PvE spaces",
			len(spaces.Records), len(expected))
	}
	for _, k := range spaces.SortedKeys() {
		r := spaces.Records[config.KeyString(k)]
		id := fStr(r, "space_id")
		if _, ok := r.Fields["bounds_max_m"]; !ok {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"space %q has no bounds_max_m", id)
		}
		if fStr(r, "layout_profile") == "" && fStr(r, "layout") == "" {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"space %q has no layout_profile", id)
		}
	}
}

// ---------------------------------------------------------------------
// skill geometry: 45 primary rows, role bands, envelopes, separation
// ---------------------------------------------------------------------

func validateSkillGeometry(c *Ctx, gates balanceGates) {
	actions := famRecs(c, "skill_action")
	if gates.hasGeomRows && int64(len(actions)) != gates.geomRows {
		c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
			"primary geometry row count %d != %d", len(actions), gates.geomRows)
	}
	cats := map[string]string{}
	tags := map[string]map[string]bool{}
	for _, r := range famRecs(c, "skill") {
		id := fStr(r, "skill_id")
		cats[id] = fStr(r, "category")
		tags[id] = map[string]bool{}
		if v, ok := r.Fields["tags"]; ok {
			for _, e := range v.Elems {
				tags[id][e.Str] = true
			}
		}
	}
	// hostile melee / single-target reach sources for the separation gate
	var minRangedBasic, maxHostileMelee int64 = math.MaxInt64, 0
	for _, r := range actions {
		id := fStr(r, "skill_id")
		g := geomFields(r)
		if g == nil {
			c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
				"skill %q has no primary geometry", id)
			continue
		}
		kind := fStr(config.Record{Fields: g}, "kind")
		basic := cats[id] == "basic"
		band := func(name string, v, lo, hi int64) {
			if v < lo || v > hi {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"skill %q %s %d outside launch band %d..%d", id, name, v, lo, hi)
			}
		}
		switch kind {
		case "MELEE_BOX":
			band("reach", geomMM(g, "reach"), 1800, 2800)
			if geomMM(g, "reach") > maxHostileMelee {
				maxHostileMelee = geomMM(g, "reach")
			}
		case "DIRECTION_BOX":
			reach := geomMM(g, "reach")
			if reach == 0 {
				reach = geomMM(g, "length")
			}
			if basic {
				band("directional wave reach", reach, 2800, 3500)
			} else {
				band("directional wave reach", reach, 5000, 5500)
			}
		case "DASH_LINE":
			d := geomMM(g, "distance")
			if basic {
				band("dash distance", d, 2400, 2600)
			} else {
				band("dash distance", d, 4000, 4500)
			}
		case "PROJECTILE":
			rng, rad := geomMM(g, "range"), geomMM(g, "radius")
			if basic {
				band("projectile range", rng, 7500, 8500)
				if rng < minRangedBasic {
					minRangedBasic = rng
				}
			}
			if rng <= 0 || rad <= 0 || geomMM(g, "speed") <= 0 {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"skill %q projectile has non-positive speed/range/radius", id)
			}
			if gates.hasProjCap && float64(rng+rad) > gates.projCapMM {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"skill %q projectile max_range+hit_radius %d > %.1fm", id, rng+rad, gates.projCapMM/1000)
			}
		case "AREA_POSITION":
			cast, rad := geomMM(g, "cast"), geomMM(g, "radius")
			band("cast range", cast, 6500, 7500)
			band("radius", rad, 2500, 3500)
			if gates.hasAreaCap && float64(cast+rad) > gates.areaCapMM {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"skill %q cast_range+radius %d > %.1fm", id, cast+rad, gates.areaCapMM/1000)
			}
		case "AREA_SELF":
			band("radius", geomMM(g, "radius"), 2800, 3800)
		case "SINGLE_TARGET_RANGE":
			rng := geomMM(g, "range")
			support := tags[id]["HEAL"] || tags[id]["DEFENSIVE"]
			if support {
				band("ally single-target range", rng, 7000, 8000)
			} else {
				band("hostile single-target range", rng, 1800, 2800)
				if rng > maxHostileMelee {
					maxHostileMelee = rng
				}
			}
		case "BARRIER_POSITION":
			band("cast", geomMM(g, "cast"), 6000, 7000)
			band("thickness", geomMM(g, "thickness"), 500, 1000)
			band("height", geomMM(g, "height"), 3000, 4500)
		}
	}
	// hostile attack reaches also bound the separation gate
	for _, r := range famRecs(c, "attack") {
		if g := geomFields(r); g != nil && fStr(r, "shape") == "RECT" {
			if fStr(r, "profile") == "MELEE" || fStr(r, "suffix") == "basic" {
				if l := geomMM(g, "length_mm"); l > maxHostileMelee {
					maxHostileMelee = l
				}
			}
		}
	}
	if minRangedBasic == math.MaxInt64 {
		if gates.hasSepRatio {
			c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
				"no ranged-basic projectile to measure separation")
		}
	} else if gates.hasSepRatio && maxHostileMelee > 0 &&
		float64(minRangedBasic)/float64(maxHostileMelee) < gates.sepRatio {
		c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
			"separation ratio %.4f < %.2f (ranged %d / melee %d)",
			float64(minRangedBasic)/float64(maxHostileMelee), gates.sepRatio,
			minRangedBasic, maxHostileMelee)
	}
	// secondary spatial effects must carry typed geometry + deterministic
	// resolution fields
	for _, r := range famRecs(c, "spatial_effect") {
		id := fStr(r, "spatial_effect_id")
		if fStr(r, "origin_shape") == "" || fStr(r, "exact_resolution") == "" {
			c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
				"spatial_effect %q lacks typed geometry or deterministic resolution", id)
		}
	}
}

// ---------------------------------------------------------------------
// ADR-0047 displacement parity: DISPLACEMENT tag <=> forced-position or
// AIRBORNE spatial result
// ---------------------------------------------------------------------

var dispResultRe = regexp.MustCompile(`(?i)airborne|knock|push|pull|displace|forced`)

func validateDisplacementParity(c *Ctx) {
	spatial := map[string]config.Record{}
	for _, r := range famRecs(c, "spatial_effect") {
		spatial[fStr(r, "spatial_effect_id")] = r
	}
	skillSpatial := map[string][]string{}
	for _, r := range famRecs(c, "skill_effect") {
		if p, ok := r.Fields["payload"]; ok && p.Kind == config.KindRecord &&
			p.Rec["kind"].Str == "SPATIAL" {
			if ref, ok := p.Rec["ref_id"]; ok {
				skillSpatial[fStr(r, "skill_id")] = append(skillSpatial[fStr(r, "skill_id")], ref.Str)
			}
		}
	}
	for _, r := range famRecs(c, "basic_proc") {
		if v, ok := r.Fields["status_effects"]; ok {
			for _, e := range v.Elems {
				if strings.HasPrefix(e.Str, "spatial.") {
					skillSpatial[fStr(r, "skill_id")] = append(skillSpatial[fStr(r, "skill_id")], e.Str)
				}
			}
		}
	}
	for _, r := range famRecs(c, "skill") {
		id := fStr(r, "skill_id")
		tagged := false
		if v, ok := r.Fields["tags"]; ok {
			for _, e := range v.Elems {
				if e.Str == "DISPLACEMENT" {
					tagged = true
				}
			}
		}
		result := false
		for _, ref := range skillSpatial[id] {
			if se, ok := spatial[ref]; ok &&
				(dispResultRe.MatchString(ref) || dispResultRe.MatchString(fStr(se, "exact_resolution"))) {
				result = true
			}
		}
		if tagged != result {
			c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
				"skill %q DISPLACEMENT tag=%v but forced-position/AIRBORNE result=%v", id, tagged, result)
		}
	}
}

// ---------------------------------------------------------------------
// quest prerequisite graph must be acyclic (flags granted by quests gate
// later quests; quest->quest references join the same graph)
// ---------------------------------------------------------------------

var vQuestIDRe = regexp.MustCompile("`?(quest[.][a-z0-9_.]+)`?")

var progIDRe = regexp.MustCompile("`?(progression[.][a-z0-9_.]+)`?")

func validateQuestGraph(c *Ctx) {
	quests := famRecs(c, "quest")
	qset := map[string]bool{}
	grants := map[string]string{} // progression flag -> granting quest
	for _, r := range quests {
		qset[fStr(r, "quest_id")] = true
	}
	for _, r := range famRecs(c, "quest_reward") {
		if flag := fStr(r, "flag"); strings.HasPrefix(flag, "progression.") {
			qid := fStr(r, "quest_id")
			if qset[qid] {
				if prev, dup := grants[flag]; dup && prev != qid {
					c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
						"progression flag %q granted by both %q and %q", flag, prev, qid)
				}
				grants[flag] = qid
			}
		}
	}
	edges := map[string][]string{}
	addEdge := func(from, to string) {
		if from != "" && from != to {
			edges[from] = append(edges[from], to)
		}
	}
	for _, r := range quests {
		qid := fStr(r, "quest_id")
		pre, _ := r.Fields["prerequisite"]
		if pre.Kind != config.KindString {
			continue
		}
		for _, m := range vQuestIDRe.FindAllStringSubmatch(pre.Str, -1) {
			addEdge(m[1], qid) // quest must come after its prerequisite
		}
		for _, m := range progIDRe.FindAllStringSubmatch(pre.Str, -1) {
			if g, ok := grants[m[1]]; ok {
				addEdge(g, qid)
			}
		}
	}
	// iterative DFS, white/gray/black
	const (
		white = iota
		gray
		black
	)
	color := map[string]int{}
	var stack []string
	for q := range qset {
		if color[q] != white {
			continue
		}
		stack = append(stack, q)
		color[q] = gray
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			done := true
			for _, n := range edges[top] {
				switch color[n] {
				case gray:
					c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
						"quest prerequisite cycle %q -> %q", top, n)
					done = true
				case white:
					color[n] = gray
					stack = append(stack, n)
					done = false
				}
			}
			if done {
				color[top] = black
				stack = stack[:len(stack)-1]
			}
		}
	}
}

// ---------------------------------------------------------------------
// CAT-001 soul element bindings
// ---------------------------------------------------------------------

func validateSoulElements(c *Ctx) {
	expect, wantTotal, declared := parseSoulExpectations(c)
	type split struct{ normal, elite, boss int }
	per := map[string]*split{}
	total := 0
	for _, r := range famRecs(c, "soul") {
		total++
		el := fStr(r, "element")
		if el == "" || el == "NONE" {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"soul %q must declare an explicit non-NONE element", fStr(r, "soul_id"))
			continue
		}
		s := per[el]
		if s == nil {
			s = &split{}
			per[el] = s
		}
		switch fStr(r, "rank") {
		case "NORMAL":
			s.normal++
		case "ELITE":
			s.elite++
		case "BOSS":
			s.boss++
		}
	}
	if !declared {
		return
	}
	if total != wantTotal {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"soul roster %d != declared %d", total, wantTotal)
	}
	for el, want := range expect {
		s := per[el]
		if s == nil {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"element %s has no souls, declared %d", el, want.total)
			continue
		}
		if s.normal != want.normal || s.elite != want.elite || s.boss != want.boss {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"element %s rank split %dN+%dE+%dB, declared %dN+%dE+%dB",
				el, s.normal, s.elite, s.boss, want.normal, want.elite, want.boss)
		}
	}
}

// ---------------------------------------------------------------------
// CAT-002 barrier payload signature + geometry + rule version
// ---------------------------------------------------------------------

func validateBarrierPayload(c *Ctx) {
	var eff config.Record
	found := false
	for _, r := range famRecs(c, "skill_effect") {
		if p, ok := r.Fields["payload"]; ok && p.Kind == config.KindRecord &&
			p.Rec["kind"].Str == "BARRIER" {
			eff, found = r, true
		}
	}
	if !found {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"no skill_effect BARRIER payload")
		return
	}
	sid := fStr(eff, "skill_id")
	p := eff.Fields["payload"].Rec
	if fStr(config.Record{Fields: p}, "geometry_ref") != "PRIMARY" ||
		!fBool(config.Record{Fields: p}, "block_enemy_movement") ||
		!fBool(config.Record{Fields: p}, "block_projectiles") {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"BARRIER payload on %q must be geometry_ref=PRIMARY with both block flags true", sid)
	}
	geomOK := false
	for _, r := range famRecs(c, "skill_action") {
		if fStr(r, "skill_id") == sid {
			if g := geomFields(r); g != nil && g["kind"].Str == "BARRIER_POSITION" {
				geomOK = true
			}
		}
	}
	if !geomOK {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"skill %q has no BARRIER_POSITION primary geometry", sid)
	}
	rv := c.Defs.Families["rule_versions"]
	if rv == nil {
		return
	}
	rec, ok := rv.Records["skill_barrier_payload"]
	if !ok || fInt(rec, "version") != 1 {
		c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
			"rule_versions.skill_barrier_payload != 1")
	}
}

// ---------------------------------------------------------------------
// CAT-003 registered item strings must be NFC-normalized
// ---------------------------------------------------------------------

func validateRegisteredStrings(c *Ctx) {
	for _, r := range famRecs(c, "item") {
		for _, k := range []string{"display", "identity_note"} {
			if s := fStr(r, k); s != "" && !norm.NFC.IsNormalString(s) {
				c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
					"item %q %s is not NFC-normalized", fStr(r, "item_id"), k)
			}
		}
	}
}

// ---------------------------------------------------------------------
// README launch-budget cross-checks
// ---------------------------------------------------------------------

func validateLaunchBudget(c *Ctx) {
	pf := c.Params.Families["launch_budget"]
	if pf == nil {
		return
	}
	defs := c.Defs.Families
	countWhere := func(fam string, pred func(config.Record) bool) int {
		n := 0
		for _, r := range famRecs(c, fam) {
			if pred(r) {
				n++
			}
		}
		return n
	}
	all := func(config.Record) bool { return true }
	isLaunch := func(r config.Record) bool { return fBool(r, "in_launch") }
	byType := func(t string) func(config.Record) bool {
		return func(r config.Record) bool { return fStr(r, "type") == t }
	}
	byRole := func(role string) func(config.Record) bool {
		return func(r config.Record) bool { return fStr(r, "role") == role }
	}
	distinct := func(fam, field string, pred func(config.Record) bool) int {
		seen := map[string]bool{}
		for _, r := range famRecs(c, fam) {
			if pred(r) {
				seen[fStr(r, field)] = true
			}
		}
		return len(seen)
	}
	nonSeason := func(r config.Record) bool { return fStr(r, "domain") != "season" }
	currencies := func() int {
		seen := map[string]bool{}
		for _, recs := range defs {
			for _, k := range recs.SortedKeys() {
				r := recs.Records[config.KeyString(k)]
				if v := fStr(r, "currency"); strings.HasPrefix(v, "currency.") {
					seen[v] = true
				}
			}
		}
		return len(seen)
	}
	classIDs := func() map[string]int {
		per := map[string]int{}
		for _, r := range famRecs(c, "skill") {
			parts := strings.Split(fStr(r, "skill_id"), ".")
			if len(parts) > 1 {
				per[parts[1]]++
			}
		}
		return per
	}
	checks := map[string]func() int{
		"NORMAL monsters": func() int {
			return countWhere("monster", func(r config.Record) bool { return fStr(r, "rank") == "NORMAL" && isLaunch(r) })
		},
		"ELITE monsters": func() int {
			return countWhere("monster", func(r config.Record) bool { return fStr(r, "rank") == "ELITE" && isLaunch(r) })
		},
		"major bosses":                  func() int { return countWhere("boss", all) },
		"MAIN quests":                   func() int { return countWhere("quest", byType("MAIN")) },
		"SIDE quests":                   func() int { return countWhere("quest", byType("SIDE")) },
		"Souls":                         func() int { return countWhere("soul", all) },
		"Spirit Beasts":                 func() int { return countWhere("beast", all) },
		"Spirit Surge event definition": func() int { return countWhere("world_event", all) },
		"adventure field maps":          func() int { return countWhere("world_map", byType("FIELD")) },
		"authored entry portals":        func() int { return countWhere("portal", all) },
		"cosmetics":                     func() int { return countWhere("cosmetic", all) },
		"launch atlas pages":            func() int { return countWhere("atlas_page", nonSeason) },
		"equipment sets":                func() int { return countWhere("equipment_set", all) },
		"beast equipment definitions":   func() int { return countWhere("beast_equipment", all) },
		"normal dungeons":               func() int { return countWhere("dungeon", all) },
		"Formations":                    func() int { return countWhere("formation", all) },
		"Meridian resonances":           func() int { return countWhere("resonance", all) },
		"Daily bounty templates standard + 1 mystery_meta": func() int {
			return countWhere("daily_template", func(r config.Record) bool {
				return fStr(r, "template_id") != "daily.surge_if_active"
			})
		},
		"canonical currencies":    currencies,
		"bonus progression books": func() int { return distinct("book_grant", "item_id", all) },
		"decorative ambient NPCs": func() int { return countWhere("npc", byRole("ambient")) },
		"persistent launch service NPCs": func() int {
			return countWhere("npc", func(r config.Record) bool { return fStr(r, "role") != "ambient" })
		},
		"world regions":                          func() int { return countWhere("progression_region", all) },
		"safe/social anchors":                    func() int { return countWhere("safe_anchor", all) },
		"normal-world checkpoints":               func() int { return countWhere("checkpoint", all) },
		"normal-world first-discovery EXP slots": func() int { return countWhere("world_map", all) },
		"major progression first-clear EXP slots": func() int {
			return distinct("first_clear_exp", "act", all)
		},
		"regional power-crafting materials": func() int { return countWhere("regional_mapping", all) },
		"set equipment definitions":         func() int { return countWhere("equipment", all) },
		"classes":                           func() int { return len(classIDs()) },
		"basic + 5 active + 3 passive skills per class = 60 upgradeable class skills": func() int {
			return countWhere("skill", all)
		},
		"skills per class": func() int {
			per := classIDs()
			if len(per) == 0 {
				return 0
			}
			n := -1
			for _, v := range per {
				if n == -1 {
					n = v
				}
				if v != n {
					return -1 // unequal per-class counts
				}
			}
			return n
		},
	}
	for _, k := range pf.SortedKeys() {
		r := pf.Records[config.KeyString(k)]
		label := r.Key[0].Str
		want := fInt(r, "count")
		// the "4 basic + 5 active + 3 passive = 60" line parses 4 as its
		// count but gates the total upgradeable-skill count of 60
		if label == "basic + 5 active + 3 passive skills per class = 60 upgradeable class skills" {
			want = 60
		}
		check, ok := checks[label]
		if !ok {
			continue // prose-only budget line with no compiled counterpart
		}
		if got := check(); got != int(want) {
			c.Diags.Addf(config.DiagIntegrationCheck, "", 0,
				"launch budget %q: compiled %d, README declares %d", label, got, want)
		}
	}
}

// ---------------------------------------------------------------------
// balance_validation.md hard gates (reference build, damage pipeline)
// ---------------------------------------------------------------------

// bonus levels are declared by the bundle's balance catalog fences

var classGrowth = map[string][3]float64{ // MAX_HP, ATTACK, DEFENSE per level
	"KIM":  {32, 5.5, 2.0},
	"MOC":  {36, 4.8, 2.2},
	"THUY": {32, 5.0, 1.9},
	"HOA":  {30, 5.5, 1.8},
	"THO":  {44, 4.4, 3.0},
}

var basic1 = map[string]string{
	"KIM": "kiem_thuc", "MOC": "linh_diep", "THUY": "thuy_tien",
	"HOA": "hoa_phu", "THO": "tran_quyen",
}

var classes = []string{"KIM", "MOC", "THUY", "HOA", "THO"}

type refBuild struct {
	attack, defense, maxHP float64
	critChance             float64
}

func tierFor(level int64) int64 {
	t := (level + 9) / 10
	if t < 1 {
		t = 1
	}
	if t > 6 {
		t = 6
	}
	return t
}

// equipBaseStats sums one deterministic set's fixed base-stat lines at the
// given tier, each line enhanced by floor(value * (1 + enh*0.025)).
func equipBaseStats(c *Ctx, g balanceGates, tier int64) map[string]float64 {
	best := ""
	for _, r := range famRecs(c, "equipment") {
		if fStr(r, "tier") != fmt.Sprintf("T%d", tier) {
			continue
		}
		if sk := fStr(r, "set_key"); sk != "" && (best == "" || sk < best) {
			best = sk
		}
	}
	out := map[string]float64{"ATTACK": 0, "DEFENSE": 0, "MAX_HP": 0}
	mult := 1 + float64(g.enh[tier])*0.025
	for _, r := range famRecs(c, "equipment") {
		if fStr(r, "tier") != fmt.Sprintf("T%d", tier) || fStr(r, "set_key") != best {
			continue
		}
		if v, ok := r.Fields["fixed_stats"]; ok {
			for _, e := range v.Elems {
				if e.Kind != config.KindRecord {
					continue
				}
				stat := e.Rec["stat"].Str
				if _, ok := out[stat]; !ok {
					continue
				}
				val := fNum(config.Record{Fields: e.Rec}, "value")
				out[stat] += math.Floor(val * mult)
			}
		}
	}
	return out
}

func referenceBuild(c *Ctx, gates balanceGates, class string, level int64, equip map[string]float64) refBuild {
	g := classGrowth[class]
	earned := 4*(level-1) + gates.bonus[level]
	off := int64(math.Floor(float64(earned) * gates.offPct / 100))
	vit := int64(math.Floor(float64(earned) * gates.vitPct / 100))
	agi := earned - off - vit
	b := refBuild{
		attack:     40 + g[1]*float64(level-1) + float64(off)*0.75 + equip["ATTACK"],
		defense:    20 + g[2]*float64(level-1) + math.Floor(float64(vit)*0.20) + equip["DEFENSE"],
		maxHP:      500 + g[0]*float64(level-1) + float64(vit)*6 + equip["MAX_HP"],
		critChance: math.Min(0.05+float64(agi)*0.0005, 0.60),
	}
	b.attack = math.Floor(b.attack)
	b.defense = math.Floor(b.defense)
	b.maxHP = math.Floor(b.maxHP)
	return b
}

// basicDamage computes one expected basic_1 hit through the canonical
// damage pipeline (element neutral, no dodge/procs).
func basicDamage(b refBuild, coeff float64, targetDef, targetLevel float64) float64 {
	raw := math.Floor(b.attack * coeff)
	critExp := 1 + b.critChance*(1.50-1)
	pre := math.Floor(raw * critExp)
	k := 100 + 20*targetLevel
	mult := k / (k + math.Max(0, targetDef))
	if mult < 0.25 {
		mult = 0.25
	}
	return math.Floor(pre * mult)
}

// basicDPS returns hits/s * damage using the observable-cadence formula.
func basicDPS(c *Ctx, class string, b refBuild, targetDef, targetLevel float64) float64 {
	sid := "skill." + strings.ToLower(class) + ".basic." + basic1[class]
	var coeff, cdS, startup, active float64
	found := false
	for _, r := range famRecs(c, "basic_proc") {
		if fStr(r, "skill_id") == sid {
			coeff = fNum(r, "base_coefficient")
			cdS = fNum(r, "base_cd_s")
			found = true
		}
	}
	for _, r := range famRecs(c, "skill_action") {
		if fStr(r, "skill_id") == sid {
			startup = fNum(r, "startup_ms")
			active = fNum(r, "active_ms")
		}
	}
	if !found || cdS <= 0 {
		return 0
	}
	interval := math.Max(cdS*1000, startup+active)
	hits := 1000 / (50 * math.Ceil(interval/50))
	return basicDamage(b, coeff, targetDef, targetLevel) * hits
}

func validateBalance(c *Ctx, g balanceGates) {
	equip := map[int64]map[string]float64{}
	for t := int64(1); t <= 6; t++ {
		equip[t] = equipBaseStats(c, g, t)
	}
	build := func(class string, level int64) refBuild {
		return referenceBuild(c, g, class, level, equip[tierFor(level)])
	}
	// boss HP formula declared by the bundle's balance catalog
	if g.hasHP {
		for _, r := range famRecs(c, "boss") {
			l := fInt(r, "lv")
			want := int64(math.Floor(g.hpBase + g.hpLin*float64(l) + g.hpQ*float64(l)*float64(l)))
			if got := fInt(r, "base_hp"); got != want {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"boss %q base_hp %d != declared formula %d",
					fStr(r, "boss_id"), got, want)
			}
		}
	}
	// NORMAL / ELITE same-level generated targets
	normHP := func(l int64) float64 {
		return math.Floor(180 + 30*float64(l) + 0.45*float64(l)*float64(l))
	}
	normDef := func(l int64) float64 { return math.Floor(8 + 1.6*float64(l)) }
	for _, l := range []int64{10, 20, 30, 40, 50, 60} {
		for _, cl := range classes {
			b := build(cl, l)
			dpsN := basicDPS(c, cl, b, normDef(l), float64(l))
			if dpsN <= 0 {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"no basic_1 DPS for class %s level %d", cl, l)
				continue
			}
			ttk := normHP(l) / dpsN
			if g.hasNormal && (ttk < g.normalLo || ttk > g.normalHi) {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"NORMAL same-level TTK %.2fs for %s Lv%d outside %.2f..%.2f",
					ttk, cl, l, g.normalLo, g.normalHi)
			}
			dpsE := basicDPS(c, cl, b, math.Floor(normDef(l)*1.20), float64(l))
			ttkE := math.Floor(normHP(l)*4.00) / dpsE
			if g.hasElite && (ttkE < g.eliteLo || ttkE > g.eliteHi) {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"ELITE same-level TTK %.2fs for %s Lv%d outside %.2f..%.2f",
					ttkE, cl, l, g.eliteLo, g.eliteHi)
			}
		}
	}
	// boss durability: per authored boss level/DEFENSE
	for _, r := range famRecs(c, "boss") {
		l := fInt(r, "lv")
		def := fNum(r, "defense")
		hp := float64(fInt(r, "base_hp"))
		id := fStr(r, "boss_id")
		var minT, maxT float64 = math.MaxFloat64, 0
		for _, cl := range classes {
			b := build(cl, l)
			dps := basicDPS(c, cl, b, def, float64(l))
			if dps <= 0 {
				continue
			}
			ttk := hp / dps
			if g.hasSolo && (ttk < g.soloLo || ttk > g.soloHi) {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"solo boss TTK %.2fs for %s vs %s outside %.0f..%.0f",
					ttk, cl, id, g.soloLo, g.soloHi)
			}
			party := ttk * 0.64
			if g.hasParty && (party < g.partyLo || party > g.partyHi) {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"five-player boss TTK %.2fs for %s vs %s outside %.0f..%.0f",
					party, cl, id, g.partyLo, g.partyHi)
			}
			if ttk < minT {
				minT = ttk
			}
			if ttk > maxT {
				maxT = ttk
			}
			// heavy hit: 1.40 * boss ATTACK through mitigation
			raw := math.Floor(fNum(r, "attack") * 1.40)
			k := 100 + 20*float64(l)
			mult := k / (k + math.Max(0, b.defense))
			if mult < 0.25 {
				mult = 0.25
			}
			hit := math.Floor(raw * mult)
			ratio := hit / b.maxHP
			if g.hasHeavy && (ratio < g.heavyLo/100 || ratio > g.heavyHi/100) {
				c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
					"heavy hit %.4f of %s MAX_HP vs %s outside %.0f%%..%.0f%%",
					ratio, cl, id, g.heavyLo, g.heavyHi)
			}
		}
		if g.hasSpread && minT > 0 && maxT/minT > g.spreadMax {
			c.Diags.Addf(config.DiagBalanceGuardrail, "", 0,
				"class spread %.4f > %.2f vs %s", maxT/minT, g.spreadMax, id)
		}
	}
}
