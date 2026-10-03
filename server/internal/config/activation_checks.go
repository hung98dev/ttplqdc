package config

// Candidate re-verification suite for the activation gate (IMP-004):
// every check inspects a compiled *CandidateSnapshot and appends
// diagnostics; activation is all-or-nothing. The suite re-derives
// structural checks and bound fences where both operands compile to
// values (wave-4 plan F-4.2) — the numeric benchmark engine lives in
// cmd/compiler package main and is not importable here; hard balance
// failures already surface as carried compile diagnostics.
//
// Audit note (wave-4 audit correction 1): there is no manifest/LOCKED
// status family in the emitted snapshot — manifest locking is a
// compile-enforced invariant upstream, so no checkManifestLocked exists
// here (deliberately not an IMP-003 emit change).

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// runActivationChecks applies the whole rejection suite; prev is the
// currently active store entry (nil before first activation) and m is the
// candidate's deployment metadata.
func runActivationChecks(c *CandidateSnapshot, prev *revisionEntry, m ActivationMeta) Diagnostics {
	var d Diagnostics
	if c == nil {
		d.Addf(DiagIntegrationCheck, "", 0, "nil candidate snapshot")
		return d
	}
	checkCompileDiagnostics(c, &d)
	checkSchemaVersions(c, &d)
	checkReferences(c, &d)
	checkEntitySizes(c, &d)
	checkSpaces(c, &d)
	checkSkillEnvelope(c, &d)
	checkSkillAssertions(c, &d)
	checkSoul(c, &d)
	checkBarrier(c, &d)
	checkRegisteredItemStrings(c, &d)
	checkHardBalance(c, &d)
	var prevSnap *CandidateSnapshot
	if prev != nil {
		prevSnap = prev.snap
	}
	checkSchemaCoupled(c, prevSnap, m, &d)
	return d
}

// ---- record helpers -----------------------------------------------------

func familyOf(c *CandidateSnapshot, name string) *Family {
	if c == nil || c.Definitions == nil {
		return nil
	}
	return c.Definitions[name]
}

func familyRecs(c *CandidateSnapshot, name string) []Record {
	f := familyOf(c, name)
	if f == nil {
		return nil
	}
	out := make([]Record, 0, len(f.Records))
	for _, k := range f.SortedKeys() {
		out = append(out, f.Records[KeyString(k)])
	}
	return out
}

func fieldStr(r Record, k string) string {
	v, ok := r.Fields[k]
	if !ok || v.Kind != KindString {
		return ""
	}
	return v.Str
}

func fieldBool(r Record, k string) bool {
	v, ok := r.Fields[k]
	return ok && v.Kind == KindBool && v.Bool
}

func fieldInt(r Record, k string) (int64, bool) {
	v, ok := r.Fields[k]
	if ok && v.Kind == KindInt {
		return v.Int, true
	}
	return 0, false
}

// fieldMM reads a millimetre/millisecond geometry argument (Rational mm).
func fieldMM(g map[string]Value, k string) (int64, bool) {
	v, ok := g[k]
	if !ok {
		return 0, false
	}
	switch v.Kind {
	case KindInt:
		return v.Int, true
	case KindRational:
		if v.Rat.Den != 0 {
			return v.Rat.Num / v.Rat.Den, true
		}
	}
	return 0, false
}

func geomRec(r Record) map[string]Value {
	v, ok := r.Fields["geometry"]
	if !ok || v.Kind != KindRecord {
		return nil
	}
	return v.Rec
}

// ---- carried compile diagnostics ----------------------------------------

func checkCompileDiagnostics(c *CandidateSnapshot, d *Diagnostics) {
	*d = append(*d, c.Diagnostics...)
}

// ---- schema / rule versions ---------------------------------------------

var requiredRuleVersions = map[string]int{
	"item_definition_strings": 1,
	"soul_element_bindings":   1,
	"skill_barrier_payload":   1,
}

func checkSchemaVersions(c *CandidateSnapshot, d *Diagnostics) {
	if c.AuthoringSchemaVersion != AuthoringSchemaVersion {
		d.Addf(DiagSourceSchemaMissing, "", 0,
			"authoring schema version %d, supported %d",
			c.AuthoringSchemaVersion, AuthoringSchemaVersion)
	}
	if c.ContentSchemaVersion != ContentSchemaVersion {
		d.Addf(DiagSourceSchemaMissing, "", 0,
			"content schema version %d, supported %d",
			c.ContentSchemaVersion, ContentSchemaVersion)
	}
	for name, want := range requiredRuleVersions {
		if got, ok := c.RuleVersions[name]; !ok || got != want {
			d.Addf(DiagSourceSchemaMissing, "", 0,
				"rule_versions.%s = %d, required %d", name, got, want)
		}
	}
}

// ---- cross-family references --------------------------------------------

// activationRefSpecs mirrors the compiler's two-pass reference table
// (config.md § Cross-Catalog Validation Examples): (family, field) →
// required namespace.
var activationRefSpecs = []struct {
	fam, field, ns string
}{
	{"monster", "drop_table_id", "drop"},
	{"boss", "drop_table_id", "drop"},
	{"boss", "space_id", "space"},
	{"boss_encounter", "boss_id", "boss"},
	{"boss_anchor", "boss_id", "boss"},
	{"boss_anchor", "map_id", "map"},
	{"npc", "map_id", "map"},
	{"portal", "source_map", "map"},
	{"portal", "destination", "space"},
	{"portal", "destination_spawn", "spawn"},
	{"spawn_group", "map_id", "map"},
	{"spawn_group", "anchor_id", "anchor"},
	{"map_pool", "group", "spawn"},
	{"map_pool", "map_id", "map"},
	{"map_pool", "monster_id", "monster"},
	{"quest", "quest_giver", "npc"},
	{"spawn_anchor", "map_id", "map"},
	{"encounter_monster", "monster_id", "monster"},
	{"encounter_dungeon", "dungeon_id", "dungeon"},
	{"dungeon_stage", "dungeon_id", "dungeon"},
	{"dungeon_secret", "dungeon_id", "dungeon"},
	{"endgame_variant", "dungeon_id", "dungeon"},
	{"endgame_remix", "dungeon_id", "dungeon"},
	{"weekly_highlight", "dungeon_id", "dungeon"},
	{"weekly_highlight", "reward_table_id", "drop"},
	{"mechanic_payload", "boss_id", "boss"},
	{"mechanic_payload_part", "boss_id", "boss"},
	{"finale_phase", "boss_id", "boss"},
	{"first_clear_grant", "boss_id", "boss"},
	{"first_clear_grant", "soul_id", "soul"},
	{"first_clear_reward", "boss_id", "boss"},
	{"first_progression_clear", "boss_id", "boss"},
	{"soul", "source_id", "monster"},
	{"soul", "effect_id", "effect"},
	{"soul_effect", "soul_id", "soul"},
	{"soul_effect", "effect_id", "effect"},
	{"set_effect", "effect_id", "effect"},
	{"basic_proc", "skill_id", "skill"},
	{"passive_payload", "skill_id", "skill"},
	{"skill_action", "skill_id", "skill"},
	{"skill_effect", "skill_id", "skill"},
	{"quest_object", "map_id", "map"},
	{"quest_object", "quest_id", "quest"},
	{"quest_anchor", "map_id", "map"},
	{"quest_anchor", "object_id", "quest_object"},
	{"quest_objective", "quest_id", "quest"},
	{"quest_reward", "quest_id", "quest"},
	{"quest_platform", "quest_id", "quest"},
	{"quest_platform", "map_id", "map"},
	{"recipe", "output_item_id", "equipment"},
	{"recipe_input", "recipe_id", "recipe"},
	{"recipe_input", "item_id", "item"},
	{"shop_offer", "item_id", "item"},
	{"atlas_page_tier", "atlas_page_id", "atlas_page"},
	{"atlas_milestone", "title_id", "cosmetic"},
	{"cosmetic_price", "cosmetic_id", "cosmetic"},
	{"material_sink", "cosmetic_id", "cosmetic"},
	{"redemption_route", "cosmetic_id", "cosmetic"},
	{"product", "cosmetic_id", "cosmetic"},
	{"special_sink", "cosmetic_id", "cosmetic"},
	{"feat", "cosmetic_id", "cosmetic"},
	{"catch_entry", "item_id", "item"},
	{"catch_entry", "table_id", "fishing"},
	{"book_grant", "item_id", "item"},
	{"beast_equipment", "item_id", "item"},
	{"beast_passive_rule", "beast_id", "beast"},
	{"beast_detail", "beast_id", "beast"},
	{"beast_upgrade_cost", "material_item_id", "item"},
	{"tier_material", "material_id", "item"},
	{"regional_mapping", "material_id", "item"},
	{"surge_field", "map_id", "map"},
	{"surge_reward", "drop_table_id", "drop"},
	{"daily_anchor", "map_id", "map"},
	{"chest", "map_id", "map"},
	{"checkpoint", "map_id", "map"},
	{"fishing_spot", "map_id", "map"},
	{"map_water", "map_id", "map"},
	{"discovery_reward", "map_id", "map"},
	{"relic_anchor", "map_id", "map"},
	{"season_relic", "map_id", "map"},
	{"field_map", "map_id", "map"},
	{"board_weight", "template_id", "daily_template"},
	{"canonical_boss", "boss_id", "boss"},
	{"roster_override", "monster_id", "monster"},
	{"beast_detail", "class_id", "class"},
	{"attack", "monster_id", "monster"},
	{"npc_service_route", "npc_id", "npc"},
	{"npc_service_route", "map_id", "map"},
}

// activationIDFields maps each emitted family to its identity fields; the
// value declares under its first dotted segment (monster.* → "monster").
var activationIDFields = map[string][]string{
	"monster":          {"monster_id"},
	"boss":             {"boss_id"},
	"dungeon":          {"dungeon_id"},
	"world_map":        {"map_id"},
	"npc":              {"npc_id"},
	"item":             {"item_id"},
	"equipment":        {"item_id"},
	"equipment_set":    {"set_id"},
	"soul":             {"soul_id", "effect_id"},
	"beast":            {"beast_id"},
	"drop_table":       {"drop_table_id"},
	"portal":           {"portal_id"},
	"spawn_group":      {"spawn_group_id"},
	"quest":            {"quest_id"},
	"quest_object":     {"object_id"},
	"quest_anchor":     {"anchor_id"},
	"daily_template":   {"template_id"},
	"daily_anchor":     {"anchor_id"},
	"atlas_page":       {"atlas_page_id"},
	"cosmetic":         {"cosmetic_id"},
	"recipe":           {"recipe_id"},
	"skill":            {"skill_id"},
	"attack":           {"attack_id"},
	"mechanic":         {"mechanic_id"},
	"effect_template":  {"effect_id"},
	"fishing_spot":     {"spot_id"},
	"chest":            {"chest_id"},
	"checkpoint":       {"checkpoint_id"},
	"formation":        {"formation_id"},
	"resonance":        {"resonance_id"},
	"village_object":   {"object_id"},
	"spatial_effect":   {"spatial_effect_id"},
	"world_event":      {"event_id"},
	"shop_offer":       {"offer_id", "shop_id"},
	"catch_table":      {"table_id"},
	"season_relic":     {"relic_id"},
	"spawn_anchor":     {"anchor_id"},
	"boss_anchor":      {"anchor_id"},
	"relic_anchor":     {"anchor_id"},
	"safe_anchor":      {"map_id"},
	"dungeon_secret":   {"secret_id"},
	"weekly_highlight": {"dungeon_id"},
	"quest_platform":   {"platform_id"},
	"atlas_milestone":  {"title_id"},
	"product":          {"product_id"},
	"feat":             {"feat_id"},
	"surge_chain_step": {"step_id"},
	"set_support":      {"support_id"},
	"passive_scaling":  {"passive_id"},
	"set_effect":       {"effect_id"},
	"spatial_soul":     {"spatial_id"},
	"soul_effect":      {"effect_id"},
	"beast_equipment":  {"item_id"},
}

var closedSizeProfiles = map[string]bool{
	"MONSTER_SMALL": true, "MONSTER_MEDIUM": true, "MONSTER_ELITE": true,
	"BOSS_LARGE": true, "WORLD_BOSS": true,
}

var classIDs = map[string]bool{
	"class.kim": true, "class.moc": true, "class.thuy": true,
	"class.hoa": true, "class.tho": true,
}

func nsOf(id string) string {
	if i := strings.Index(id, "."); i > 0 {
		return id[:i]
	}
	return id
}

// declareNamespaces builds the id namespace index from the candidate's
// emitted families (same rules as the compiler's declare pass).
func declareNamespaces(c *CandidateSnapshot) map[string]map[string]bool {
	ids := map[string]map[string]bool{}
	decl := func(ns, id string) {
		if ids[ns] == nil {
			ids[ns] = map[string]bool{}
		}
		ids[ns][id] = true
	}
	eachFam := func(f *Family, fn func(Record)) {
		if f == nil {
			return
		}
		for _, k := range f.SortedKeys() {
			fn(f.Records[KeyString(k)])
		}
	}
	for fam, f := range c.Definitions {
		eachFam(f, func(rec Record) {
			decl(fam, KeyString(rec.Key))
			for _, kv := range rec.Key {
				if kv.Kind != KindString || kv.Str == "" {
					continue
				}
				decl(fam, kv.Str)
				decl(nsOf(kv.Str), kv.Str)
			}
		})
	}
	for fam, fields := range activationIDFields {
		eachFam(familyOf(c, fam), func(rec Record) {
			for _, fld := range fields {
				v, ok := rec.Fields[fld]
				if !ok || v.Kind != KindString || v.Str == "" {
					continue
				}
				decl(fam, v.Str)
				decl(nsOf(v.Str), v.Str)
			}
		})
	}
	// geometry spaces resolve under "space"/"space_geometry" and their ns
	if c.Geometry != nil {
		eachFam(c.Geometry, func(rec Record) {
			if v := fieldStr(rec, "space_id"); v != "" {
				decl("space", v)
				decl("space_geometry", v)
				decl(nsOf(v), v)
			}
		})
	}
	for id := range classIDs {
		decl("class", id)
	}
	for id := range closedSizeProfiles {
		decl("size_profile", id)
	}
	// spawn-point namespace: entry spawns + spawn groups + portal returns
	eachFam(familyOf(c, "world_map"), func(rec Record) {
		if v := fieldStr(rec, "entry_spawn"); v != "" {
			decl("spawn", v)
		}
	})
	eachFam(familyOf(c, "spawn_group"), func(rec Record) {
		if v := fieldStr(rec, "spawn_group_id"); v != "" {
			decl("spawn", v)
		}
	})
	eachFam(familyOf(c, "portal"), func(rec Record) {
		for _, fld := range []string{"destination_spawn", "return_spawn"} {
			if v := fieldStr(rec, fld); v != "" {
				decl("spawn", v)
			}
		}
	})
	return ids
}

func checkReferences(c *CandidateSnapshot, d *Diagnostics) {
	ids := declareNamespaces(c)
	for _, rs := range activationRefSpecs {
		f := familyOf(c, rs.fam)
		if f == nil {
			continue
		}
		for _, k := range f.SortedKeys() {
			rec := f.Records[KeyString(k)]
			v, ok := rec.Fields[rs.field]
			if !ok || v.Kind != KindString {
				continue
			}
			id := v.Str
			if id == "" || id == "NONE" {
				continue
			}
			if strings.Contains(id, "<tier>") {
				for _, t := range []string{"t1", "t2", "t3", "t4", "t5", "t6"} {
					inst := strings.ReplaceAll(id, "<tier>", t)
					if ids[rs.ns][inst] || ids[nsOf(inst)][inst] {
						continue
					}
					d.Addf(DiagUnresolvedReference, "", 0,
						"%s %q is not declared", rs.ns, inst)
				}
				continue
			}
			if rs.ns == "space" && ids["space_geometry"][id] {
				continue
			}
			if ids[rs.ns][id] || ids[nsOf(id)][id] {
				continue
			}
			d.Addf(DiagUnresolvedReference, "", 0,
				"%s %q is not declared", rs.ns, id)
		}
	}
}

// ---- entity size profiles ------------------------------------------------

func checkEntitySizes(c *CandidateSnapshot, d *Diagnostics) {
	for _, fam := range []string{"monster", "boss"} {
		for _, r := range familyRecs(c, fam) {
			id := fieldStr(r, fam+"_id")
			sp := fieldStr(r, "size_profile")
			if sp == "" {
				d.Addf(DiagIntegrationCheck, "", 0,
					"%s %q has no size_profile", fam, id)
				continue
			}
			if !closedSizeProfiles[sp] {
				d.Addf(DiagIntegrationCheck, "", 0,
					"%s %q size_profile %q is not a declared profile", fam, id, sp)
			}
		}
	}
}

// ---- playable space records ----------------------------------------------

// checkSpaces enforces emitted-field completeness on every geometry
// record (audit correction 2: scene keys / anchors / .geom.json export
// markers are not emitted fields) plus the normal-world span band and
// span-pair/layout distinctness (integration_validation.md Presentation
// Scale). The space set and kinds are data-driven (F-4.1).
func checkSpaces(c *CandidateSnapshot, d *Diagnostics) {
	if c.Geometry == nil {
		d.Addf(DiagSourceSchemaMissing, "", 0, "geometry.spaces family missing")
		return
	}
	seenSpan := map[string]string{}
	seenProfile := map[string]string{}
	for _, r := range c.Geometry.Records {
		id := fieldStr(r, "space_id")
		if id == "" {
			d.Addf(DiagIntegrationCheck, "", 0, "space record without space_id")
			continue
		}
		kind := fieldStr(r, "kind")
		if kind == "" {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing kind", id)
		}
		if fieldStr(r, "layout_profile") == "" {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing layout_profile", id)
		}
		if fieldStr(r, "required_topology") == "" {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing required_topology", id)
		}
		span, ok := recPair(r, "span_screens")
		if !ok || span[0].Num <= 0 || span[1].Num <= 0 {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing positive span_screens", id)
		}
		if _, ok := recPair(r, "bounds_max_m"); !ok {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing bounds_max_m", id)
		}
		if _, ok := intPair(r, "reference_extent_px"); !ok {
			d.Addf(DiagIntegrationCheck, "", 0, "space %q missing reference_extent_px", id)
		}
		if kind != "FIELD_OR_TOWN" || !ok {
			continue
		}
		// normal-world band: width 2.0..5.0 screens, never the viewport (1.0)
		w := span[0]
		if w.Num < 2*w.Den || w.Num > 5*w.Den {
			d.Addf(DiagValueOutOfBounds, "", 0,
				"space %q normal-world width %s outside 2.0..5.0 screens", id, w)
		}
		key := span[0].String() + "x" + span[1].String()
		if prev, dup := seenSpan[key]; dup {
			d.Addf(DiagIntegrationCheck, "", 0,
				"normal-world maps %q and %q reuse span %s", prev, id, key)
		}
		seenSpan[key] = id
		prof := fieldStr(r, "layout_profile")
		if prev, dup := seenProfile[prof]; dup && prof != "" {
			d.Addf(DiagIntegrationCheck, "", 0,
				"normal-world maps %q and %q reuse layout_profile %q", prev, id, prof)
		}
		seenProfile[prof] = id
	}
}

// recPair reads a {x,y} record of rationals.
func recPair(r Record, k string) ([2]Rat, bool) {
	v, ok := r.Fields[k]
	if !ok || v.Kind != KindRecord {
		return [2]Rat{}, false
	}
	x, xok := v.Rec["x"]
	y, yok := v.Rec["y"]
	if !xok || x.Kind != KindRational || !yok || y.Kind != KindRational {
		return [2]Rat{}, false
	}
	return [2]Rat{x.Rat, y.Rat}, true
}

func intPair(r Record, k string) ([2]int64, bool) {
	v, ok := r.Fields[k]
	if !ok || v.Kind != KindRecord {
		return [2]int64{}, false
	}
	x, xok := v.Rec["x"]
	y, yok := v.Rec["y"]
	if !xok || x.Kind != KindInt || !yok || y.Kind != KindInt {
		return [2]int64{}, false
	}
	return [2]int64{x.Int, y.Int}, true
}

// ---- skill envelope + geometry -------------------------------------------

// Canonical launch reach bands (class_skill_catalog.md § Launch Reach
// Audit; skills.md § Range Measurement).
var (
	projEnvelopeMM  = int64(8800)  // max_range + hit_radius <= 8.8m
	areaEnvelopeMM  = int64(11000) // cast_range + radius <= 11.0m
	basicDashes     = [2]int64{2400, 2600}
	activeDashes    = [2]int64{4000, 4500}
	basicWaves      = [2]int64{2800, 3500}
	activeWaves     = [2]int64{5000, 5500}
	meleeReach      = [2]int64{1800, 2800}
	projectileRange = [2]int64{7500, 8500}
	areaCast        = [2]int64{6500, 7500}
	areaRadius      = [2]int64{2500, 3500}
	selfRadius      = [2]int64{2800, 3800}
	allyRange       = [2]int64{7000, 8000}
	contactDistance = [2]int64{4000, 4500}
	barrierCast     = [2]int64{6000, 7000}
	barrierThick    = [2]int64{500, 1000}
	barrierHeight   = [2]int64{3000, 4500}
)

func banded(d *Diagnostics, what, id string, v int64, band [2]int64, present bool) {
	if !present {
		d.Addf(DiagValueOutOfBounds, "", 0, "skill %q missing %s", id, what)
		return
	}
	if v < band[0] || v > band[1] {
		d.Addf(DiagValueOutOfBounds, "", 0,
			"skill %q %s %dmm outside launch band %d..%d", id, what, v, band[0], band[1])
	}
}

func checkGeomKind(c *CandidateSnapshot, d *Diagnostics, id string, g map[string]Value, isBasic bool, skillTags map[string]bool) {
	kind := ""
	if g != nil {
		if v, ok := g["kind"]; ok && v.Kind == KindString {
			kind = v.Str
		}
	}
	if kind == "" {
		d.Addf(DiagIntegrationCheck, "", 0, "skill %q missing typed geometry kind", id)
		return
	}
	switch kind {
	case "MELEE_BOX":
		banded(d, "reach", id, mustMM(g, "reach"), meleeReach, hasMM(g, "reach"))
	case "DIRECTION_BOX":
		l, ok := fieldMM(g, "length")
		if isBasic {
			banded(d, "directional wave reach", id, l, basicWaves, ok)
		} else {
			banded(d, "directional wave reach", id, l, activeWaves, ok)
		}
	case "DASH_LINE":
		dist, ok := fieldMM(g, "distance")
		if isBasic {
			banded(d, "dash distance", id, dist, basicDashes, ok)
		} else {
			banded(d, "dash distance", id, dist, activeDashes, ok)
		}
	case "PROJECTILE":
		rng, okR := fieldMM(g, "range")
		rad, okRad := fieldMM(g, "radius")
		spd, okS := fieldMM(g, "speed")
		banded(d, "projectile range", id, rng, projectileRange, okR)
		if okRad && rad <= 0 {
			d.Addf(DiagValueOutOfBounds, "", 0, "skill %q projectile radius %d <= 0", id, rad)
		}
		if okS && spd <= 0 {
			d.Addf(DiagValueOutOfBounds, "", 0, "skill %q projectile speed %d <= 0", id, spd)
		}
		if okR && okRad && rng+rad > projEnvelopeMM {
			d.Addf(DiagValueOutOfBounds, "", 0,
				"skill %q projectile max_range+hit_radius %d > 8.8m", id, rng+rad)
		}
	case "AREA_POSITION":
		cast, okC := fieldMM(g, "cast")
		rad, okR := fieldMM(g, "radius")
		banded(d, "cast range", id, cast, areaCast, okC)
		banded(d, "radius", id, rad, areaRadius, okR)
		if okC && okR && cast+rad > areaEnvelopeMM {
			d.Addf(DiagValueOutOfBounds, "", 0,
				"skill %q cast_range+radius %d > 11.0m", id, cast+rad)
		}
	case "AREA_SELF":
		banded(d, "radius", id, mustMM(g, "radius"), selfRadius, hasMM(g, "radius"))
	case "SINGLE_TARGET_RANGE":
		rng, ok := fieldMM(g, "range")
		if skillTags["HEAL"] || skillTags["DEFENSIVE"] {
			banded(d, "ally single-target range", id, rng, allyRange, ok)
		} else {
			banded(d, "hostile single-target range", id, rng, meleeReach, ok)
		}
	case "MOVE_CONTACT_LINE":
		banded(d, "movement contact distance", id, mustMM(g, "distance"), contactDistance, hasMM(g, "distance"))
		if dur, ok := fieldMM(g, "duration"); !ok || dur <= 0 {
			d.Addf(DiagValueOutOfBounds, "", 0, "skill %q contact duration missing/<=0", id)
		}
	case "BARRIER_POSITION":
		banded(d, "barrier cast", id, mustMM(g, "cast"), barrierCast, hasMM(g, "cast"))
		banded(d, "barrier thickness", id, mustMM(g, "thickness"), barrierThick, hasMM(g, "thickness"))
		banded(d, "barrier height", id, mustMM(g, "height"), barrierHeight, hasMM(g, "height"))
		if dur, ok := fieldMM(g, "duration"); !ok || dur <= 0 {
			d.Addf(DiagValueOutOfBounds, "", 0, "skill %q barrier duration missing/<=0", id)
		}
	case "SELF":
		// no spatial reach
	default:
		d.Addf(DiagIntegrationCheck, "", 0, "skill %q unknown geometry kind %q", id, kind)
	}
}

func hasMM(g map[string]Value, k string) bool { _, ok := fieldMM(g, k); return ok }
func mustMM(g map[string]Value, k string) int64 {
	v, _ := fieldMM(g, k)
	return v
}

// dispRe matches forced-position / canonical AIRBORNE spatial results.
var dispRe = regexp.MustCompile(`(?i)airborne|knock|push|pull|displace|forced`)

func checkSkillEnvelope(c *CandidateSnapshot, d *Diagnostics) {
	skillTags := map[string]map[string]bool{}
	skillCat := map[string]string{}
	for _, r := range familyRecs(c, "skill") {
		id := fieldStr(r, "skill_id")
		skillCat[id] = fieldStr(r, "category")
		tags := map[string]bool{}
		if v, ok := r.Fields["tags"]; ok && (v.Kind == KindSet || v.Kind == KindList) {
			for _, e := range v.Elems {
				tags[e.Str] = true
			}
		}
		skillTags[id] = tags
	}
	basics, actives := 0, 0
	for _, r := range familyRecs(c, "skill_action") {
		id := fieldStr(r, "skill_id")
		cat := skillCat[id]
		switch cat {
		case "basic":
			basics++
		case "active":
			actives++
		default:
			d.Addf(DiagIntegrationCheck, "", 0,
				"skill_action %q has no basic/active skill row", id)
		}
		isBasic := cat == "basic"
		g := geomRec(r)
		checkGeomKind(c, d, id, g, isBasic, skillTags[id])
		if v, ok := r.Fields["air_geometry"]; ok && v.Kind == KindRecord {
			checkGeomKind(c, d, id+" (air)", v.Rec, isBasic, skillTags[id])
		}
	}
	if basics != 20 || actives != 25 {
		d.Addf(DiagIntegrationCheck, "", 0,
			"primary geometry rows %d basics + %d actives, want 20 + 25", basics, actives)
	}
	// secondary spatial effects need complete typed fields
	for _, r := range familyRecs(c, "spatial_effect") {
		id := fieldStr(r, "spatial_effect_id")
		for _, req := range []string{"source", "origin_shape", "exact_resolution", "target_cap_interaction"} {
			if fieldStr(r, req) == "" {
				d.Addf(DiagIntegrationCheck, "", 0,
					"spatial_effect %q missing %s", id, req)
			}
		}
	}
	checkDisplacementParity(c, skillTags, d)
}

// checkDisplacementParity enforces ADR-0047: DISPLACEMENT tag <=> a
// forced-position or canonical AIRBORNE spatial result.
func checkDisplacementParity(c *CandidateSnapshot, skillTags map[string]map[string]bool, d *Diagnostics) {
	spatial := map[string]Record{}
	for _, r := range familyRecs(c, "spatial_effect") {
		spatial[fieldStr(r, "spatial_effect_id")] = r
	}
	skillSpatial := map[string][]string{}
	for _, r := range familyRecs(c, "skill_effect") {
		p, ok := r.Fields["payload"]
		if !ok || p.Kind != KindRecord {
			continue
		}
		if p.Rec["kind"].Str == "SPATIAL" {
			if ref, ok := p.Rec["ref_id"]; ok && ref.Kind == KindString {
				sid := fieldStr(r, "skill_id")
				skillSpatial[sid] = append(skillSpatial[sid], ref.Str)
			}
		}
	}
	for _, r := range familyRecs(c, "basic_proc") {
		if v, ok := r.Fields["status_effects"]; ok && v.Kind == KindList {
			sid := fieldStr(r, "skill_id")
			for _, e := range v.Elems {
				if e.Kind == KindString && strings.HasPrefix(e.Str, "spatial.") {
					skillSpatial[sid] = append(skillSpatial[sid], e.Str)
				}
			}
		}
	}
	for id, tags := range skillTags {
		result := false
		for _, ref := range skillSpatial[id] {
			se, ok := spatial[ref]
			if ok && (dispRe.MatchString(ref) || dispRe.MatchString(fieldStr(se, "exact_resolution"))) {
				result = true
			}
		}
		if tags["DISPLACEMENT"] != result {
			d.Addf(DiagIntegrationCheck, "", 0,
				"skill %q DISPLACEMENT tag=%v but forced-position/AIRBORNE result=%v",
				id, tags["DISPLACEMENT"], result)
		}
	}
}

// ---- class_skill catalog assertions 4, 19, 20, 21 ------------------------

// Per-class basic cooldown bands in milliseconds (skills.md § cooldown
// table — authoritative class bands).
var basicCdBands = map[string][2][2]int64{ // class -> {Lv1 lo,hi},{Lv12 lo,hi}
	"kim":  {{500, 560}, {200, 240}},
	"thuy": {{620, 680}, {300, 320}},
	"moc":  {{690, 720}, {340, 360}},
	"hoa":  {{780, 820}, {390, 410}},
	"tho":  {{950, 1000}, {480, 500}},
}

// canonicalSkillTags is the closed launch tag set (skills.md).
var canonicalSkillTags = map[string]bool{
	"DAMAGING": true, "AREA": true, "PROJECTILE": true, "MOVEMENT": true,
	"HEAL": true, "SHIELD": true, "STATUS_APPLY": true, "DISPLACEMENT": true,
	"DEFENSIVE": true, "SIGNATURE": true, "BASIC_ATTACK": true,
	"PENETRATE": true,
}

var skillIDRe = regexp.MustCompile(`^skill\.([a-z]+)\.(basic|passive|active)\.`)

func checkSkillAssertions(c *CandidateSnapshot, d *Diagnostics) {
	skills := map[string]Record{}
	for _, r := range familyRecs(c, "skill") {
		skills[fieldStr(r, "skill_id")] = r
	}
	actions := map[string]Record{}
	for _, r := range familyRecs(c, "skill_action") {
		actions[fieldStr(r, "skill_id")] = r
	}
	procs := map[string]Record{}
	for _, r := range familyRecs(c, "basic_proc") {
		procs[fieldStr(r, "skill_id")] = r
	}
	payloads := map[string][]map[string]Value{}
	for _, r := range familyRecs(c, "skill_effect") {
		sid := fieldStr(r, "skill_id")
		if p, ok := r.Fields["payload"]; ok && p.Kind == KindRecord {
			payloads[sid] = append(payloads[sid], p.Rec)
		}
	}
	templates := map[string]bool{}
	for _, r := range familyRecs(c, "effect_template") {
		templates[fieldStr(r, "effect_id")] = true
	}
	spatialIDs := map[string]bool{}
	for _, r := range familyRecs(c, "spatial_effect") {
		spatialIDs[fieldStr(r, "spatial_effect_id")] = true
	}

	signatures := map[string]int{}
	for id, r := range skills {
		cat := fieldStr(r, "category")
		tags := map[string]bool{}
		if v, ok := r.Fields["tags"]; ok && (v.Kind == KindSet || v.Kind == KindList) {
			for _, e := range v.Elems {
				tags[e.Str] = true
			}
		}
		// assert 20: damage_element == owning class element
		m := skillIDRe.FindStringSubmatch(id)
		wantElem := map[string]string{
			"kim": "KIM", "moc": "MOC", "thuy": "THUY", "hoa": "HOA", "tho": "THO",
		}
		cls := ""
		if m != nil {
			cls = m[1]
			if de := fieldStr(r, "damage_element"); de != wantElem[cls] {
				d.Addf(DiagIntegrationCheck, "", 0,
					"skill %q damage_element %q != owning class element %q", id, de, wantElem[cls])
			}
		}
		if ce := fieldStr(r, "class_element"); ce != fieldStr(r, "damage_element") {
			d.Addf(DiagIntegrationCheck, "", 0,
				"skill %q class_element %q != damage_element %q", id, ce, fieldStr(r, "damage_element"))
		}
		// assert 21: canonical tags + tag/payload consistency
		for t := range tags {
			if !canonicalSkillTags[t] {
				d.Addf(DiagIntegrationCheck, "", 0, "skill %q unknown tag %q", id, t)
			}
		}
		has := func(kind string) bool {
			for _, p := range payloads[id] {
				if p["kind"].Str == kind {
					return true
				}
			}
			return false
		}
		_, isBasic := procs[id]
		damaging := has("DAMAGE") || has("EXECUTE") || isBasic
		if tags["DAMAGING"] && !damaging {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q DAMAGING tag without damage result", id)
		}
		if !tags["DAMAGING"] && (has("DAMAGE") || has("EXECUTE")) {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q damage payload without DAMAGING tag", id)
		}
		if tags["HEAL"] && !has("HEAL") {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q HEAL tag without HEAL payload", id)
		}
		if tags["SHIELD"] && !has("SHIELD") {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q SHIELD tag without SHIELD payload", id)
		}
		if tags["STATUS_APPLY"] && !has("STATUS") {
			if pr, ok := procs[id]; !ok {
				d.Addf(DiagIntegrationCheck, "", 0, "skill %q STATUS_APPLY tag without status result", id)
			} else if v, ok2 := pr.Fields["status_effects"]; !ok2 || len(v.Elems) == 0 {
				d.Addf(DiagIntegrationCheck, "", 0, "skill %q STATUS_APPLY tag without status result", id)
			}
		}
		if tags["PROJECTILE"] {
			if g := geomRec(actions[id]); g == nil || g["kind"].Str != "PROJECTILE" {
				d.Addf(DiagIntegrationCheck, "", 0, "skill %q PROJECTILE tag without projectile geometry", id)
			}
		}
		if tags["MOVEMENT"] && fieldStr(r, "movement_behavior") != "FORCED" {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q MOVEMENT tag without FORCED movement_behavior", id)
		}
		if tags["BASIC_ATTACK"] && cat != "basic" {
			d.Addf(DiagIntegrationCheck, "", 0, "skill %q BASIC_ATTACK tag on non-basic skill", id)
		}
		if cat == "basic" && !tags["BASIC_ATTACK"] {
			d.Addf(DiagIntegrationCheck, "", 0, "basic %q missing BASIC_ATTACK tag", id)
		}
		if tags["SIGNATURE"] && cat == "active" {
			signatures[cls]++
		}
		// assert 19: status/shield effect_ids resolve into effect templates
		for _, p := range payloads[id] {
			switch p["kind"].Str {
			case "STATUS", "SHIELD":
				eid := p["effect_id"].Str
				if eid == "" || !templates[eid] {
					d.Addf(DiagUnresolvedReference, "", 0,
						"skill %q %s payload effect_id %q not in effect_template", id, p["kind"].Str, eid)
				}
			}
		}
		if pr, ok := procs[id]; ok {
			if v, ok2 := pr.Fields["status_effects"]; ok2 && v.Kind == KindList {
				for _, e := range v.Elems {
					switch {
					case strings.HasPrefix(e.Str, "effect.") && !templates[e.Str]:
						d.Addf(DiagUnresolvedReference, "", 0,
							"skill %q basic status effect %q not in effect_template", id, e.Str)
					case strings.HasPrefix(e.Str, "spatial.") && !spatialIDs[e.Str]:
						d.Addf(DiagUnresolvedReference, "", 0,
							"skill %q basic spatial effect %q not in spatial_effect", id, e.Str)
					}
				}
			}
		}
	}
	// SIGNATURE: exactly one signature active per class
	for cls := range basicCdBands {
		if signatures[cls] != 1 {
			d.Addf(DiagIntegrationCheck, "", 0,
				"class %s has %d SIGNATURE actives, want 1", cls, signatures[cls])
		}
	}
	// assert 4: basic cooldown bands + phase sum = 1000*base_cd_s
	for id, pr := range procs {
		m := skillIDRe.FindStringSubmatch(id)
		if m == nil || m[2] != "basic" {
			d.Addf(DiagIntegrationCheck, "", 0, "basic_proc row %q is not a basic skill id", id)
			continue
		}
		band, ok := basicCdBands[m[1]]
		if !ok {
			d.Addf(DiagIntegrationCheck, "", 0, "basic_proc %q unknown class %q", id, m[1])
			continue
		}
		base, _ := fieldMM2(pr, "base_cd_s")
		max, _ := fieldMM2(pr, "max_cd_s")
		if base < band[0][0] || base > band[0][1] {
			d.Addf(DiagValueOutOfBounds, "", 0,
				"skill %q Lv1 cooldown %dms outside %s band %d..%d", id, base, m[1], band[0][0], band[0][1])
		}
		if max < band[1][0] || max > band[1][1] {
			d.Addf(DiagValueOutOfBounds, "", 0,
				"skill %q Lv12 cooldown %dms outside %s band %d..%d", id, max, m[1], band[1][0], band[1][1])
		}
		if ar, ok := actions[id]; ok {
			if su, ok2 := fieldInt(ar, "startup_ms"); ok2 {
				ac, _ := fieldInt(ar, "active_ms")
				re, _ := fieldInt(ar, "recovery_ms")
				if su+ac+re != base {
					d.Addf(DiagIntegrationCheck, "", 0,
						"skill %q phases %d+%d+%d != base_cd %dms", id, su, ac, re, base)
				}
			}
		}
	}
	// assert 21: every ACTIVE skill in exactly one target_scaling group
	groupOf := map[string]int{}
	for _, r := range familyRecs(c, "target_scaling") {
		if v, ok := r.Fields["members"]; ok && v.Kind == KindList {
			for _, e := range v.Elems {
				if e.Kind == KindString {
					groupOf[e.Str]++
				}
			}
		}
	}
	for id, r := range skills {
		if fieldStr(r, "category") != "active" {
			continue
		}
		switch groupOf[id] {
		case 0:
			d.Addf(DiagIntegrationCheck, "", 0,
				"active skill %q not in any target_scaling group", id)
		case 1:
		default:
			d.Addf(DiagIntegrationCheck, "", 0,
				"active skill %q in %d target_scaling groups", id, groupOf[id])
		}
	}
}

// fieldMM2 reads a seconds field emitted as a Rational.
func fieldMM2(r Record, k string) (int64, bool) {
	v, ok := r.Fields[k]
	if !ok {
		return 0, false
	}
	if v.Kind == KindRational && v.Rat.Den != 0 {
		// seconds -> ms
		num := v.Rat.Num * 1000
		if num%v.Rat.Den == 0 {
			return num / v.Rat.Den, true
		}
		return num / v.Rat.Den, true
	}
	if v.Kind == KindInt {
		return v.Int, true
	}
	return 0, false
}

// ---- CAT-001: soul element bindings ---------------------------------------

// soulDistribution is the declared rank-by-element distribution
// (soul_catalog.md § Element Count Validation / Invariants).
var soulDistribution = map[string]struct{ n, e, b int }{
	"KIM":  {3, 2, 0},
	"MOC":  {3, 2, 0},
	"THUY": {3, 1, 1},
	"HOA":  {3, 1, 1},
	"THO":  {3, 1, 1},
}

func checkSoul(c *CandidateSnapshot, d *Diagnostics) {
	monsterRank := map[string]string{}
	for _, r := range familyRecs(c, "monster") {
		monsterRank[fieldStr(r, "monster_id")] = fieldStr(r, "rank")
	}
	bossIDs := map[string]bool{}
	for _, r := range familyRecs(c, "boss") {
		bossIDs[fieldStr(r, "boss_id")] = true
	}
	per := map[string]map[string]int{}
	for _, r := range familyRecs(c, "soul") {
		id := fieldStr(r, "soul_id")
		el := fieldStr(r, "element")
		rank := fieldStr(r, "rank")
		if _, ok := soulDistribution[el]; el == "" || el == "NONE" || !ok {
			d.Addf(DiagIntegrationCheck, "", 0,
				"soul %q must declare an explicit non-NONE five-element element", id)
			continue
		}
		switch rank {
		case "NORMAL", "ELITE":
			if mr, ok := monsterRank[fieldStr(r, "source_id")]; !ok {
				d.Addf(DiagUnresolvedReference, "", 0,
					"soul %q source monster %q not declared", id, fieldStr(r, "source_id"))
			} else if mr != rank {
				d.Addf(DiagIntegrationCheck, "", 0,
					"soul %q rank %s mismatches source monster rank %s", id, rank, mr)
			}
		case "BOSS":
			if !bossIDs[fieldStr(r, "source_id")] {
				d.Addf(DiagUnresolvedReference, "", 0,
					"soul %q source boss %q not declared", id, fieldStr(r, "source_id"))
			}
		default:
			d.Addf(DiagIntegrationCheck, "", 0, "soul %q unknown rank %q", id, rank)
		}
		if per[el] == nil {
			per[el] = map[string]int{}
		}
		per[el][rank]++
	}
	for el, want := range soulDistribution {
		got := per[el]
		if got["NORMAL"] != want.n || got["ELITE"] != want.e || got["BOSS"] != want.b {
			d.Addf(DiagIntegrationCheck, "", 0,
				"soul element %s rank split %dN+%dE+%dB, declared %dN+%dE+%dB",
				el, got["NORMAL"], got["ELITE"], got["BOSS"], want.n, want.e, want.b)
		}
	}
}

// ---- CAT-002 + assertions 22, 23: closed barrier payload -------------------

var closedPayloadKinds = map[string]bool{
	"DAMAGE": true, "HEAL": true, "EXECUTE": true, "STATUS": true,
	"SHIELD": true, "SPATIAL": true, "ZONE": true, "BARRIER": true,
}

func checkBarrier(c *CandidateSnapshot, d *Diagnostics) {
	// assertion 22: closed dispatch; every ACTIVE has >=1 payload row;
	// ordinals contiguous; sole BARRIER payload must pair with a
	// BARRIER_POSITION primary geometry (catalog-assertions 22).
	payloads := map[string][]int{}
	barrierSkill := ""
	barrierCount := 0
	for _, r := range familyRecs(c, "skill_effect") {
		sid := fieldStr(r, "skill_id")
		p, ok := r.Fields["payload"]
		if !ok || p.Kind != KindRecord {
			d.Addf(DiagIntegrationCheck, "", 0, "skill_effect %q missing payload", sid)
			continue
		}
		kind := p.Rec["kind"].Str
		if !closedPayloadKinds[kind] {
			d.Addf(DiagIntegrationCheck, "", 0,
				"skill %q payload constructor %q outside closed dispatch", sid, kind)
		}
		if ord, ok := fieldInt(r, "ordinal"); ok {
			payloads[sid] = append(payloads[sid], int(ord))
		}
		if kind == "BARRIER" {
			barrierCount++
			barrierSkill = sid
			// assertion 23: sole literal primary_geometry variant; the
			// resolved payload carries only kind/geometry_ref/block flags —
			// dimensions/lifetime live on the primary geometry.
			if len(p.Rec) != 4 ||
				fieldStr(Record{Fields: p.Rec}, "geometry_ref") != "PRIMARY" ||
				!fieldBool(Record{Fields: p.Rec}, "block_enemy_movement") ||
				!fieldBool(Record{Fields: p.Rec}, "block_projectiles") {
				d.Addf(DiagIntegrationCheck, "", 0,
					"BARRIER payload on %q must be exactly kind=BARRIER, geometry_ref=PRIMARY, block_enemy_movement, block_projectiles", sid)
			}
		}
	}
	for _, r := range familyRecs(c, "skill") {
		if fieldStr(r, "category") != "active" {
			continue
		}
		sid := fieldStr(r, "skill_id")
		ords := payloads[sid]
		if len(ords) == 0 {
			d.Addf(DiagIntegrationCheck, "", 0,
				"active skill %q has no payload row", sid)
			continue
		}
		seen := map[int]bool{}
		for _, o := range ords {
			if o < 0 || o >= len(ords) || seen[o] {
				d.Addf(DiagIntegrationCheck, "", 0,
					"active skill %q payload ordinals not contiguous from 0", sid)
				break
			}
			seen[o] = true
		}
	}
	if barrierCount != 1 {
		d.Addf(DiagIntegrationCheck, "", 0,
			"bundle emits %d BARRIER payloads, want exactly 1", barrierCount)
	}
	if barrierSkill != "" {
		// barrier skill must carry a BARRIER_POSITION primary geometry and
		// nothing else (assertion 23: no damage, zone, or secondary geometry)
		found := false
		for _, r := range familyRecs(c, "skill_action") {
			if fieldStr(r, "skill_id") != barrierSkill {
				continue
			}
			if g := geomRec(r); g != nil && g["kind"].Str == "BARRIER_POSITION" {
				found = true
			}
			if v, ok := r.Fields["air_geometry"]; ok && v.Kind == KindRecord {
				d.Addf(DiagIntegrationCheck, "", 0,
					"barrier skill %q carries a secondary geometry", barrierSkill)
			}
		}
		if !found {
			d.Addf(DiagIntegrationCheck, "", 0,
				"barrier skill %q has no BARRIER_POSITION primary geometry", barrierSkill)
		}
		for _, r := range familyRecs(c, "skill_effect") {
			if fieldStr(r, "skill_id") != barrierSkill {
				continue
			}
			if p, ok := r.Fields["payload"]; ok && p.Kind == KindRecord {
				if k := p.Rec["kind"].Str; k != "BARRIER" {
					d.Addf(DiagIntegrationCheck, "", 0,
						"barrier skill %q carries a %s payload", barrierSkill, k)
				}
			}
		}
		for _, r := range familyRecs(c, "spatial_effect") {
			if strings.Contains(fieldStr(r, "source"), barrierSkill) {
				d.Addf(DiagIntegrationCheck, "", 0,
					"barrier skill %q carries secondary spatial effect %q",
					barrierSkill, fieldStr(r, "spatial_effect_id"))
			}
		}
	}
}

// ---- CAT-003: registered item strings --------------------------------------

func checkRegisteredItemStrings(c *CandidateSnapshot, d *Diagnostics) {
	for _, r := range familyRecs(c, "item") {
		id := fieldStr(r, "item_id")
		disp, ok := r.Fields["display"]
		if !ok || disp.Kind != KindString || disp.Str == "" {
			d.Addf(DiagIntegrationCheck, "", 0, "item %q missing registered display string", id)
			continue
		}
		if !norm.NFC.IsNormalString(disp.Str) {
			d.Addf(DiagIntegrationCheck, "", 0, "item %q display is not NFC-normalized", id)
		}
		note, ok := r.Fields["identity_note"]
		if !ok {
			d.Addf(DiagIntegrationCheck, "", 0, "item %q missing identity_note field", id)
			continue
		}
		if note.Kind == KindString && !norm.NFC.IsNormalString(note.Str) {
			d.Addf(DiagIntegrationCheck, "", 0, "item %q identity_note is not NFC-normalized", id)
		}
		if note.Kind != KindString && note.Kind != KindNull {
			d.Addf(DiagIntegrationCheck, "", 0, "item %q identity_note has non-string kind %d", id, note.Kind)
		}
	}
}

// ---- schema-coupled detection ---------------------------------------------

// checkSchemaCoupled rejects a candidate that meets the schema-coupled
// criteria (a change to a formula whose output persists to character rows
// — EXP threshold table / level cap) without carrying the schema-coupled
// tag (config.md § Schema-Coupled Content Revisions; F-4.6).
func checkSchemaCoupled(c, prev *CandidateSnapshot, m ActivationMeta, d *Diagnostics) {
	if checkSchemaCoupledDiff(c, prev) && !m.SchemaCoupled {
		d.Addf(DiagIntegrationCheck, "", 0,
			"candidate changes a persisted-formula output (EXP thresholds / level cap) but lacks the schema-coupled tag")
	}
}

// checkSchemaCoupledDiff reports whether the candidate changes a
// persisted-formula output vs prev (level rows exp_required/exp_cumulative
// and the level cap = row count).
func checkSchemaCoupledDiff(c, prev *CandidateSnapshot) bool {
	if prev == nil {
		return false
	}
	lc, lp := familyOf(c, "level"), familyOf(prev, "level")
	if lc == nil || lp == nil {
		return lc != lp
	}
	if len(lc.Records) != len(lp.Records) {
		return true
	}
	for k, rec := range lc.Records {
		orec, ok := lp.Records[k]
		if !ok {
			return true
		}
		for _, f := range []string{"exp_required", "exp_cumulative"} {
			vc, ok1 := rec.Fields[f]
			vo, ok2 := orec.Fields[f]
			if ok1 != ok2 || (ok1 && vc.Kind == KindInt && vo.Kind == KindInt && vc.Int != vo.Int) {
				return true
			}
		}
	}
	return false
}

// ---- hard balance bounds ---------------------------------------------------

// checkHardBalance re-derives bound fences where both operands compile to
// values (F-4.2): the level formula `exp_required(L)=10000*L*L` and
// cumulative prefix sums, the act budget column sums against the declared
// TOTAL parameter, and the channel-share 100% bound.
func checkHardBalance(c *CandidateSnapshot, d *Diagnostics) {
	// level family: contiguous 1..cap, exp_required = 10000*L*L,
	// exp_cumulative = prefix sum.
	levels := familyOf(c, "level")
	if levels == nil || len(levels.Records) == 0 {
		d.Addf(DiagSourceSchemaMissing, "", 0, "level family missing")
	} else {
		var cum int64
		lmax := int64(0)
		seen := map[int64]bool{}
		for _, k := range levels.SortedKeys() {
			rec := levels.Records[KeyString(k)]
			l, ok := fieldInt(rec, "level")
			if !ok {
				continue
			}
			seen[l] = true
			if l > lmax {
				lmax = l
			}
			req, ok := fieldInt(rec, "exp_required")
			if ok && req != 10000*l*l {
				d.Addf(DiagBalanceGuardrail, "", 0,
					"level %d exp_required %d != 10000*L*L", l, req)
			}
			if ok {
				cum += req
			}
			if got, ok := fieldInt(rec, "exp_cumulative"); ok && got != cum {
				d.Addf(DiagBalanceGuardrail, "", 0,
					"level %d exp_cumulative %d != running total %d", l, got, cum)
			}
		}
		for l := int64(1); l <= lmax; l++ {
			if !seen[l] {
				d.Addf(DiagBalanceGuardrail, "", 0, "level table missing level %d", l)
			}
		}
	}
	// act EXP budget column sums against the declared TOTAL parameter.
	budget := map[string]int64{}
	for _, r := range familyRecs(c, "act_exp_budget") {
		act := fieldStr(r, "act")
		if tot, ok := fieldInt(r, "act_exp_total"); ok {
			budget[act] = tot
		}
	}
	if len(budget) > 0 {
		if total, ok := paramInt(c, "act_exp_total", "TOTAL", "exp"); ok {
			var sum int64
			for _, t := range budget {
				sum += t
			}
			if sum != total {
				d.Addf(DiagBalanceGuardrail, "", 0,
					"act EXP budgets sum %d != declared TOTAL %d", sum, total)
			}
		}
	}
	// portfolio per-act channel sums equal the act budget.
	if pf := familyOf(c, "portfolio"); pf != nil && len(budget) > 0 {
		sum := map[string]int64{}
		for _, r := range familyRecs(c, "portfolio") {
			act := fieldStr(r, "act")
			if e, ok := fieldInt(r, "exp"); ok {
				sum[act] += e
			}
		}
		for act, s := range sum {
			if want, ok := budget[act]; ok && s != want {
				d.Addf(DiagBalanceGuardrail, "", 0,
					"act %s channel EXP sums %d != act_exp_total %d", act, s, want)
			}
		}
	}
	// channel shares sum to 100% of act budget.
	if ch := familyOf(c, "exp_channel"); ch != nil && len(ch.Records) > 0 {
		var share int64
		for _, r := range familyRecs(c, "exp_channel") {
			if p, ok := fieldInt(r, "pct_of_act_budget"); ok {
				share += p
			}
		}
		if share != 100 {
			d.Addf(DiagBalanceGuardrail, "", 0,
				"channel EXP shares sum %d != 100%%", share)
		}
	}
}

// paramInt reads an int field of one record inside a folded
// validation_parameters sub-family.
func paramInt(c *CandidateSnapshot, family, key, field string) (int64, bool) {
	if c.ValidationParameters == nil {
		return 0, false
	}
	rec, ok := c.ValidationParameters.Records[KeyString([]Value{VStr(family)})]
	if !ok {
		return 0, false
	}
	recs, ok := rec.Fields["records"]
	if !ok || recs.Kind != KindList {
		return 0, false
	}
	for _, e := range recs.Elems {
		if e.Kind != KindRecord {
			continue
		}
		kv, ok := e.Rec["key"]
		if !ok || kv.Kind != KindList || len(kv.Elems) == 0 ||
			kv.Elems[0].Kind != KindString || kv.Elems[0].Str != key {
			continue
		}
		fields, ok := e.Rec["fields"]
		if !ok || fields.Kind != KindRecord {
			continue
		}
		if v, ok := fields.Rec[field]; ok && v.Kind == KindInt {
			return v.Int, true
		}
	}
	return 0, false
}
