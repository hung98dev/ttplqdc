package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// NamespaceIndex collects emitted identities per family namespace for
// two-pass cross-catalog reference resolution (contract §1, config.md
// §Validation). Families map stable-ID prefixes onto the set of emitted
// IDs.
type NamespaceIndex struct {
	// IDs[family][id] = declaration locator "catalog:line"
	IDs map[string]map[string]string
	// Budget holds the README Launch Content Budget counts for cross-checks.
	Budget map[string]int64
}

func NewNamespaceIndex() *NamespaceIndex {
	return &NamespaceIndex{IDs: map[string]map[string]string{}}
}

// Declare registers an emitted identity in its family namespace.
// A duplicate inside one namespace is a DUPLICATE_PRIMARY_KEY diagnostic.
func (n *NamespaceIndex) Declare(c *Ctx, family, id, cat string, line int) {
	m := n.IDs[family]
	if m == nil {
		m = map[string]string{}
		n.IDs[family] = m
	}
	if _, dup := m[id]; dup {
		c.Diags.Addf(config.DiagDuplicatePrimaryKey, cat, line,
			"%s %q declared twice", family, id)
		return
	}
	m[id] = cat
}

// Resolve reports whether id exists in the family namespace; when it does
// not, records UNRESOLVED_REFERENCE at the referencing location.
func (n *NamespaceIndex) Resolve(c *Ctx, family, id, cat string, line int) bool {
	if id == "" || id == "NONE" {
		return true
	}
	if _, ok := n.IDs[family][id]; ok {
		return true
	}
	c.Diags.Addf(config.DiagUnresolvedReference, cat, line,
		"%s %q is not declared", family, id)
	return false
}

// idFields maps each emitted family to its identity field; the field's
// value is declared under the id's first dotted segment (monster.* →
// namespace "monster").
var idFields = map[string][]string{
	"monster":         {"monster_id"},
	"boss":            {"boss_id"},
	"dungeon":         {"dungeon_id"},
	"world_map":       {"map_id"},
	"npc":             {"npc_id"},
	"item":            {"item_id"},
	"equipment":       {"item_id"},
	"equipment_set":   {"set_id"},
	"soul":            {"soul_id", "effect_id"},
	"beast":           {"beast_id"},
	"drop_table":      {"drop_table_id"},
	"portal":          {"portal_id"},
	"spawn_group":     {"spawn_group_id"},
	"quest":           {"quest_id"},
	"quest_object":    {"object_id"},
	"quest_anchor":    {"anchor_id"},
	"daily_template":  {"template_id"},
	"daily_anchor":    {"anchor_id"},
	"atlas_page":      {"atlas_page_id"},
	"cosmetic":        {"cosmetic_id"},
	"recipe":          {"recipe_id"},
	"skill":           {"skill_id"},
	"attack":          {"attack_id"},
	"mechanic":        {"mechanic_id"},
	"effect_template": {"effect_id"},
	"fishing_spot":    {"spot_id"},
	"chest":           {"chest_id"},
	"checkpoint":      {"checkpoint_id"},
	"formation":       {"formation_id"},
	"resonance":       {"resonance_id"},
	"village_object":  {"object_id"},
	"spatial_effect":  {"spatial_effect_id"},
	"world_event":     {"event_id"},
	"shop_offer":      {"offer_id", "shop_id"},
	"catch_table":     {"table_id"},
	"season_relic":    {"relic_id"},
	"spawn_anchor":    {"anchor_id"},
	"boss_anchor":     {"anchor_id"},
	"relic_anchor":    {"anchor_id"},
	"safe_anchor":     {"map_id"},
	"dungeon_secret":  {"secret_id"},

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

// refSpecs lists (family, field) → namespace checks resolved after all
// catalogs compile.
var refSpecs = []struct {
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
	{"quest", "quest_giver", "npc"},
	{"spawn_anchor", "map_id", "map"},
	{"map_pool", "map_id", "map"},
	{"map_pool", "monster_id", "monster"},
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

// classIDs are the five class identity ids declared by the skill catalog.
var classIDs = []string{
	"class.kim", "class.moc", "class.thuy", "class.hoa", "class.tho",
}

func (n *NamespaceIndex) declareRaw(ns, id, loc string) {
	m := n.IDs[ns]
	if m == nil {
		m = map[string]string{}
		n.IDs[ns] = m
	}
	m[id] = loc
}

func nsOf(id string) string {
	if i := strings.Index(id, "."); i > 0 {
		return id[:i]
	}
	return id
}

// resolveReferences — S13 two-pass reference check: declare every emitted
// identity, then resolve declared reference fields. Unknown ids produce
// UNRESOLVED_REFERENCE and the candidate is rejected.
func resolveReferences(c *Ctx) error {
	// pass 1: declare — the full composite key is the identity under the
	// family namespace (unique); each dotted key element also aliases into
	// its own namespace (non-unique).
	for fam, f := range c.Defs.Families {
		for _, key := range f.SortedKeys() {
			rec := f.Records[config.KeyString(key)]
			c.Refs.Declare(c, fam, config.KeyString(key), fam, 0)
			for _, kv := range rec.Key {
				if kv.Kind != config.KindString || kv.Str == "" {
					continue
				}
				c.Refs.declareRaw(fam, kv.Str, fam)
				c.Refs.declareRaw(nsOf(kv.Str), kv.Str, fam)
			}
		}
	}
	// extra identity fields beyond the primary key (non-unique aliases).
	for fam, fields := range idFields {
		f := c.Defs.Families[fam]
		if f == nil {
			continue
		}
		for _, key := range f.SortedKeys() {
			rec := f.Records[config.KeyString(key)]
			for _, fld := range fields {
				v, ok := rec.Fields[fld]
				if !ok || v.Kind != config.KindString || v.Str == "" {
					continue
				}
				c.Refs.declareRaw(fam, v.Str, f.Name)
				c.Refs.declareRaw(nsOf(v.Str), v.Str, f.Name)
			}
		}
	}
	// geometry spaces resolve under "space"/"space_geometry" and alias their ns
	eachRec := func(fam string, fn func(config.Record)) {
		f := c.Defs.Families[fam]
		if f == nil {
			return
		}
		for _, key := range f.SortedKeys() {
			fn(f.Records[config.KeyString(key)])
		}
	}
	if sf := c.Geom.Families["spaces"]; sf != nil {
		for _, key := range sf.SortedKeys() {
			rec := sf.Records[config.KeyString(key)]
			if v, ok := rec.Fields["space_id"]; ok {
				c.Refs.declareRaw("space", v.Str, "geometry")
				c.Refs.declareRaw("space_geometry", v.Str, "geometry")
				c.Refs.declareRaw(nsOf(v.Str), v.Str, "geometry")
			}
		}
	}
	// class + closed enum namespaces
	for _, id := range classIDs {
		c.Refs.declareRaw("class", id, "class_skill_catalog.md")
	}
	for _, id := range []string{"MONSTER_SMALL", "MONSTER_MEDIUM", "MONSTER_ELITE", "BOSS_LARGE", "WORLD_BOSS"} {
		c.Refs.declareRaw("size_profile", id, "monster_catalog.md/boss_catalog.md")
	}
	// spawn-point namespace: entry spawns + spawn groups + portal returns
	eachRec("world_map", func(rec config.Record) {
		if v, ok := rec.Fields["entry_spawn"]; ok {
			c.Refs.declareRaw("spawn", v.Str, "world_route_catalog.md")
		}
	})
	eachRec("spawn_group", func(rec config.Record) {
		if v, ok := rec.Fields["spawn_group_id"]; ok {
			c.Refs.declareRaw("spawn", v.Str, "map_spawn_catalog.md")
		}
	})
	eachRec("portal", func(rec config.Record) {
		for _, fld := range []string{"destination_spawn", "return_spawn"} {
			if v, ok := rec.Fields[fld]; ok && v.Kind == config.KindString && v.Str != "" {
				c.Refs.declareRaw("spawn", v.Str, "world_route_catalog.md")
			}
		}
	})

	// pass 2: resolve
	for _, rs := range refSpecs {
		f := c.Defs.Families[rs.fam]
		if f == nil {
			continue
		}
		for _, key := range f.SortedKeys() {
			rec := f.Records[config.KeyString(key)]
			v, ok := rec.Fields[rs.field]
			if !ok || v.Kind != config.KindString {
				continue
			}
			id := v.Str
			if id == "" || id == "NONE" {
				continue
			}
			if strings.Contains(id, "<tier>") {
				for _, t := range []string{"t1", "t2", "t3", "t4", "t5", "t6"} {
					inst := strings.ReplaceAll(id, "<tier>", t)
					if _, ok := c.Refs.IDs[rs.ns][inst]; ok {
						continue
					}
					if _, ok := c.Refs.IDs[nsOf(inst)][inst]; ok {
						continue
					}
					c.Refs.Resolve(c, rs.ns, inst, f.Name, 0)
				}
				continue
			}
			// id may carry a dotted prefix differing from target ns
			// (e.g. map.* refs resolve under "map")
			if rs.ns == "space" {
				if _, ok := c.Refs.IDs["space_geometry"][id]; ok {
					continue
				}
			}
			ns := rs.ns
			if _, ok := c.Refs.IDs[ns][id]; ok {
				continue
			}
			// tolerate the literal dotted namespace prefix matching
			if _, ok := c.Refs.IDs[nsOf(id)][id]; ok {
				continue
			}
			c.Refs.Resolve(c, ns, id, f.Name, 0)
		}
	}
	if f := c.Defs.Families["atlas_page"]; f != nil {
		idRe := regexp.MustCompile("`?([a-z_]+[.][a-z0-9_.]+)`?")
		for _, key := range f.SortedKeys() {
			rec := f.Records[config.KeyString(key)]
			v, ok := rec.Fields["source_id"]
			if !ok || v.Kind != config.KindString {
				continue
			}
			m := idRe.FindStringSubmatch(v.Str)
			if m == nil || strings.HasSuffix(m[1], ".") {
				continue
			}
			id := m[1]
			if _, ok := c.Refs.IDs[nsOf(id)][id]; ok {
				continue
			}
			c.Refs.Resolve(c, nsOf(id), id, "atlas_catalog.md", 0)
		}
	}

	return nil
}
