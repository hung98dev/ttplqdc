package equipment

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

var (
	itemIDRe   = regexp.MustCompile(`^item\.eq\.(t[1-6])\.([a-z0-9_]+)\.([a-z]+)$`)
	acquireRef = regexp.MustCompile(`\b(dungeon|boss)\.[a-z0-9_.]+`)
	// statModRe matches `STAT +<num>` / `STAT -<num>` modifier terms;
	// each must close with a typed stage keyword (§ Typed Set-Effect
	// Convention — unqualified decimal modifiers are invalid).
	statModRe = regexp.MustCompile(`\b(MAX_HP|MAX_MP|ATTACK|DEFENSE|CRIT_CHANCE|ATTACK_SPEED|CAST_SPEED|MOVE_SPEED|DAMAGE_REDUCTION|HEALING_RECEIVED|LIFESTEAL|REFLECT|ABSORB|HEAL_REDUCTION)\s*[+-]\s*[0-9.]`)
)

// Check re-derives the equipment expansion invariants independently of
// the compiler and returns diagnostics for every violation of the spec's
// Validation reject list (equipment_catalog.md §Validation). A non-empty
// result blocks activation. Behaves as the activation-check suite —
// see Register.
func Check(c *config.CandidateSnapshot) config.Diagnostics {
	var d config.Diagnostics
	cat, ld := Load(c)
	d = append(d, ld...)
	if c == nil {
		return d
	}
	checkExpansion(c, cat, &d)
	checkItems(cat, &d)
	checkRollPool(cat, &d)
	checkFlatRollRanges(cat, &d)
	checkSets(cat, &d)
	checkAcquisition(c, cat, &d)
	checkEnhancementBase(cat, &d)
	checkGuaranteedRecipes(c, cat, &d)
	return d
}

// checkExpansion verifies the finite rule: exactly 168 well-formed
// `item.eq.<tier>.<set_key>.<slot>` records forming a complete
// 12-set x 14-slot cross product.
func checkExpansion(c *config.CandidateSnapshot, cat *Catalog, d *config.Diagnostics) {
	if len(cat.Items) != 168 {
		d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
			"equipment expansion emitted %d items, want 168 (12 x 14)", len(cat.Items))
	}
	if len(cat.Sets) != 12 {
		d.Addf(config.DiagValueOutOfBounds, "equipment_set", 0,
			"emitted %d equipment sets, want 12", len(cat.Sets))
	}
	for id := range cat.Items {
		m := itemIDRe.FindStringSubmatch(id)
		if m == nil {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"equipment item %s violates the item.eq.<tier>.<key>.<slot> scheme", id)
			continue
		}
		it := cat.Items[id]
		if s, ok := cat.Sets[it.SetKey]; !ok {
			d.Addf(config.DiagUnresolvedReference, "equipment", 0,
				"item %s references unknown set_key %s", id, it.SetKey)
		} else if !strings.EqualFold(s.Tier, it.Tier) {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s tier %s != set %s tier %s", id, it.Tier, s.ID, s.Tier)
		}
	}
	for key, s := range cat.Sets {
		for _, slot := range CanonicalSlotOrder {
			if _, ok := cat.Items[ItemID(s.Tier, key, slot)]; !ok {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"set %s (tier %s) missing slot %s", s.ID, s.Tier, slot)
			}
		}
	}
}

// checkItems validates per-item invariants: canonical slot ordinal,
// closed element matching the set's declared layout, level range and
// rarity from the tier budget, binding defaults, fixed-stat shape
// (ring/charm utility terms excluded from enhanceable) and the
// secondary_rolls threshold per tier.
func checkItems(cat *Catalog, d *config.Diagnostics) {
	slotOrd := map[string]int64{}
	for i, s := range CanonicalSlotOrder {
		slotOrd[s] = int64(i)
	}
	slotSet := map[string]bool{}
	for _, s := range CanonicalSlotOrder {
		slotSet[s] = true
	}
	for _, id := range cat.ItemIDs {
		it := cat.Items[id]
		if !slotSet[it.Slot] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s slot %s not in the canonical 14-slot set", id, it.Slot)
		} else if it.SlotOrdinal != slotOrd[it.Slot] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s slot_ordinal %d != canonical %d", id, it.SlotOrdinal, slotOrd[it.Slot])
		}
		if !ElementNames[it.Element] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s element %s not in the closed set", id, it.Element)
		}
		if s, ok := cat.Sets[it.SetKey]; ok {
			if s.Layout != "A" && s.Layout != "B" {
				d.Addf(config.DiagValueOutOfBounds, "equipment_set", 0,
					"set %s layout %q not A/B", s.ID, s.Layout)
			} else if elem, ok := cat.Layouts[s.Layout][it.Slot]; ok && elem != it.Element {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"item %s element %s != layout %s slot %s element %s",
					id, it.Element, s.Layout, it.Slot, elem)
			}
		}
		if tb, ok := cat.Budgets[it.Tier]; ok {
			if it.SecondaryRolls != tb.SecondaryRolls {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"item %s secondary_rolls %d != tier %s budget %d",
					id, it.SecondaryRolls, it.Tier, tb.SecondaryRolls)
			}
			if it.Rarity != tb.Rarity {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"item %s rarity %s != tier %s budget %s",
					id, it.Rarity, it.Tier, tb.Rarity)
			}
		}
		if it.SecondaryRolls != 1 && it.SecondaryRolls != 2 {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s secondary_rolls %d outside the 1..2 envelope", id, it.SecondaryRolls)
		}
		if it.Binding != "UNBOUND" || it.BindingTrigger != "ON_EQUIP" || it.StackLimit != 1 {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s binding defaults (%s/%s/stack %d) != UNBOUND/ON_EQUIP/1",
				id, it.Binding, it.BindingTrigger, it.StackLimit)
		}
		checkFixedStats(it, d)
	}
}

// checkFixedStats validates one item's resolved fixed-stat terms: at
// least one term, utility terms only on ring (CRIT_CHANCE) and charm
// (COOLDOWN_REDUCTION) and excluded from enhanceable_stats, flat terms
// enhanceable and strictly positive.
func checkFixedStats(it *ItemDef, d *config.Diagnostics) {
	if len(it.FixedStats) == 0 {
		d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
			"item %s has no fixed stats", it.ID)
		return
	}
	enh := map[string]bool{}
	for _, s := range it.Enhanceable {
		enh[s] = true
	}
	stats := map[string]bool{}
	for _, t := range it.FixedStats {
		stats[t.Stat] = true
		if t.IsUtility() {
			want := (it.Slot == "ring" && t.Stat == "CRIT_CHANCE") ||
				(it.Slot == "charm" && t.Stat == "COOLDOWN_REDUCTION")
			if !want {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"item %s carries utility fixed stat %s outside ring/charm", it.ID, t.Stat)
			}
			if enh[t.Stat] {
				d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
					"item %s utility stat %s must not be enhanceable", it.ID, t.Stat)
			}
			continue
		}
		if !StatNames[t.Stat] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s fixed stat %s not in the typed stat set", it.ID, t.Stat)
		}
		if t.Flat <= 0 {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s flat stat %s resolved to %d", it.ID, t.Stat, t.Flat)
		}
		if !enh[t.Stat] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s flat stat %s missing from enhanceable_stats", it.ID, t.Stat)
		}
	}
	for _, s := range it.Enhanceable {
		if !stats[s] {
			d.Addf(config.DiagValueOutOfBounds, "equipment", 0,
				"item %s enhanceable stat %s has no fixed term", it.ID, s)
		}
	}
}

// checkRollPool verifies the closed 12-ID pool: every pool member
// emitted exactly once, flat rolls carrying `FLAT`/`FLAT_UNIT` (their
// coefficient bounds live in the flat_roll_ranges parameter, verified
// by checkFlatRollRanges), utility rolls FLAT_ADD with floored
// inclusive T1..T6 bounds.
func checkRollPool(cat *Catalog, d *config.Diagnostics) {
	if len(cat.Rolls) != len(RollPool) {
		d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
			"secondary roll pool emitted %d defs, want %d", len(cat.Rolls), len(RollPool))
	}
	for _, id := range RollPool {
		rd, ok := cat.Rolls[id]
		if !ok {
			d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
				"secondary roll pool missing closed ID %s", id)
			continue
		}
		stat, flat := FlatRollStats[id]
		switch {
		case flat:
			if rd.Kind != "FLAT" || rd.RangeKind != "FLAT_UNIT" || rd.Stat != stat {
				d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
					"flat roll %s emitted kind %s/%s stat %s, want FLAT/FLAT_UNIT %s",
					id, rd.Kind, rd.RangeKind, rd.Stat, stat)
			}
			continue
		default:
			if rd.Kind != "UTILITY" || rd.Stage != "FLAT_ADD" {
				d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
					"utility roll %s emitted kind %s stage %s, want UTILITY FLAT_ADD",
					id, rd.Kind, rd.Stage)
			}
		}
		for _, tier := range TierNames {
			r, ok := rd.TierRanges[tier]
			if !ok {
				d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
					"roll %s missing tier %s range", id, tier)
				continue
			}
			lo, hi := r[0], r[1]
			if lo.Den <= 0 || hi.Den <= 0 || lo.Num < 0 || lo.Num*hi.Den > hi.Num*lo.Den {
				d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
					"roll %s tier %s range [%v, %v] not a valid floored inclusive bound",
					id, tier, lo, hi)
			}
		}
	}
	for id := range cat.Rolls {
		found := false
		for _, want := range RollPool {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			d.Addf(config.DiagValueOutOfBounds, "roll_def", 0,
				"roll %s outside the closed 12-ID pool", id)
		}
	}
}

// checkFlatRollRanges verifies the `flat_roll_ranges` parameter: one
// FLAT_UNIT bound per flat stat with a tier-unit selector and
// non-degenerate floored inclusive coefficients (0 < lo <= hi).
func checkFlatRollRanges(cat *Catalog, d *config.Diagnostics) {
	wantStats := map[string]bool{}
	for _, s := range FlatRollStats {
		wantStats[s] = true
	}
	seen := map[string]bool{}
	for _, fr := range cat.FlatRanges {
		if !wantStats[fr.Stat] {
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"flat range entry for non-flat stat %s", fr.Stat)
			continue
		}
		if seen[fr.Stat] {
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"duplicate flat range entry for %s", fr.Stat)
		}
		seen[fr.Stat] = true
		if fr.RangeKind != "FLAT_UNIT" {
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"flat range %s range_kind %s, want FLAT_UNIT", fr.Stat, fr.RangeKind)
		}
		switch fr.Unit {
		case "A", "D", "H", "M":
		default:
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"flat range %s unit %s not a tier unit (A/D/H/M)", fr.Stat, fr.Unit)
		}
		lo, hi := fr.Lo, fr.Hi
		if lo.Den <= 0 || hi.Den <= 0 || lo.Num <= 0 || lo.Num*hi.Den > hi.Num*lo.Den {
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"flat range %s [%v, %v] not a positive floored inclusive bound",
				fr.Stat, lo, hi)
		}
	}
	for s := range wantStats {
		if !seen[s] {
			d.Addf(config.DiagValueOutOfBounds, "flat_roll_ranges", 0,
				"flat range missing stat %s", s)
		}
	}
}

// checkSets validates set fences: exactly the 2/4/6 thresholds,
// `effect.<set_id>.<n>` ids, one support signature per set with
// priority 20 and 2pc-derived payloads — support must not reference
// enhancement, rarity or roll generation (§ Support Signature Rule),
// and every stat-modifier term carries a typed stage.
func checkSets(cat *Catalog, d *config.Diagnostics) {
	for _, s := range cat.Sets {
		for _, thr := range SetThresholds {
			e, ok := s.Effects[thr]
			if !ok {
				d.Addf(config.DiagValueOutOfBounds, "set_effect", 0,
					"set %s missing %dpc effect", s.ID, thr)
				continue
			}
			wantID := "effect." + s.ID + "." + thrString(thr)
			if e.ID != wantID {
				d.Addf(config.DiagValueOutOfBounds, "set_effect", 0,
					"set %s %dpc effect_id %s, want %s", s.ID, thr, e.ID, wantID)
			}
			checkTypedPayload("set_effect", e.ID, e.Payload, d)
		}
		for thr := range s.Effects {
			valid := false
			for _, t := range SetThresholds {
				if thr == t {
					valid = true
				}
			}
			if !valid {
				d.Addf(config.DiagValueOutOfBounds, "set_effect", 0,
					"set %s emitted effect at non-fence threshold %d", s.ID, thr)
			}
		}
		if s.Support == nil {
			d.Addf(config.DiagValueOutOfBounds, "set_support", 0,
				"set %s missing its 2pc-derived support signature", s.ID)
			continue
		}
		wantSup := "support.set." + s.Key
		if s.Support.ID != wantSup {
			d.Addf(config.DiagValueOutOfBounds, "set_support", 0,
				"set %s support_id %s, want %s", s.ID, s.Support.ID, wantSup)
		}
		if s.Support.Priority != 20 {
			d.Addf(config.DiagValueOutOfBounds, "set_support", 0,
				"support %s priority %d, want fixed 20", s.Support.ID, s.Support.Priority)
		}
		checkTypedPayload("set_support", s.Support.ID, s.Support.Payload, d)
		for _, banned := range []string{"enhancement", "rarity", "roll.", "soul"} {
			if strings.Contains(s.Support.Payload, banned) {
				d.Addf(config.DiagBalanceGuardrail, "set_support", 0,
					"support %s payload references %q — support is a flat 2pc-derived signature",
					s.Support.ID, banned)
			}
		}
	}
}

func thrString(t int64) string {
	return string(rune('0' + t))
}

// checkTypedPayload enforces the Typed Set-Effect Convention on one
// emitted payload: any `<STAT> +/-<decimal>` modifier must be closed by
// a stage keyword, and the legacy stat alias is rejected outright.
func checkTypedPayload(family, id, payload string, d *config.Diagnostics) {
	if strings.Contains(payload, "target_healing_received_multiplier") {
		d.Addf(config.DiagValueOutOfBounds, family, 0,
			"%s uses legacy target_healing_received_multiplier notation", id)
	}
	for _, m := range statModRe.FindAllStringIndex(payload, -1) {
		rest := payload[m[1]:]
		typed := false
		for _, stage := range StatStages {
			if strings.HasPrefix(rest, stage) || strings.Contains(rest, " "+stage) {
				typed = true
				break
			}
		}
		if !typed {
			d.Addf(config.DiagValueOutOfBounds, family, 0,
				"%s modifier %q has no typed stage (PERCENT_ADD/FLAT_ADD/SOURCE_ADDITIVE)",
				id, payload[m[0]:m[1]])
		}
	}
}

// checkAcquisition resolves structured `dungeon.*`/`boss.*` tokens in
// each set's source_identity against the compiled catalogs.
func checkAcquisition(c *config.CandidateSnapshot, cat *Catalog, d *config.Diagnostics) {
	for _, s := range cat.Sets {
		for _, tok := range acquireRef.FindAllString(s.SourceIdentity, -1) {
			parts := strings.SplitN(tok, ".", 2)
			fam := c.Definitions[parts[0]]
			if fam == nil {
				continue
			}
			found := false
			for _, k := range fam.SortedKeys() {
				if len(k) > 0 && k[0].Str == tok {
					found = true
					break
				}
			}
			if !found {
				d.Addf(config.DiagUnresolvedReference, "equipment_set", 0,
					"set %s acquisition ref %s does not resolve", s.ID, tok)
			}
		}
	}
}

// checkEnhancementBase verifies the spec's constant rule: every tier's
// base attempt cost is exactly 1 material unit plus the emitted common
// currency — never a per-item scaling constant.
func checkEnhancementBase(cat *Catalog, d *config.Diagnostics) {
	for _, tier := range TierNames {
		eb, ok := cat.EnhBase[tier]
		if !ok {
			d.Addf(config.DiagValueOutOfBounds, "enhancement_base", 0,
				"enhancement base missing tier %s", tier)
			continue
		}
		if eb.MaterialUnits != 1 {
			d.Addf(config.DiagValueOutOfBounds, "enhancement_base", 0,
				"tier %s material_units %d != constant 1", tier, eb.MaterialUnits)
		}
	}
}

// checkGuaranteedRecipes verifies every emitted item resolves at least
// one GUARANTEED recipe in the crafting catalog expansion (spec
// Validation: zero-recipe items are rejected).
func checkGuaranteedRecipes(c *config.CandidateSnapshot, cat *Catalog, d *config.Diagnostics) {
	fam := c.Definitions["recipe"]
	if fam == nil {
		d.Addf(config.DiagUnresolvedReference, "recipe", 0,
			"no recipe family — guaranteed-recipe coverage unverifiable")
		return
	}
	covered := map[string]bool{}
	for _, k := range fam.SortedKeys() {
		rec := fam.Records[config.KeyString(k)]
		out := strField(rec, "output_item_id")
		if out != "" && strField(rec, "success_mode") == "GUARANTEED" {
			covered[out] = true
		}
	}
	for _, id := range cat.ItemIDs {
		if !covered[id] {
			d.Addf(config.DiagUnresolvedReference, "recipe", 0,
				"equipment item %s has no guaranteed recipe", id)
		}
	}
}
