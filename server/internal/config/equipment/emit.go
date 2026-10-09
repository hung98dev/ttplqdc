package equipment

import (
	"sort"

	"thinhthan/internal/config"
)

// Catalog is the typed equipment definition set a compiled snapshot
// resolves to; maps are keyed by the emitted identities, ItemIDs is the
// deterministic sorted expansion order.
type Catalog struct {
	Items      map[string]*ItemDef          // item_id
	Sets       map[string]*SetDef           // set_key
	Rolls      map[string]*RollDef          // roll.* id
	FlatRanges []*FlatRollRange             // flat_roll_ranges param
	Budgets    map[string]*TierBudget       // T1..T6
	Layouts    map[string]map[string]string // layout -> slot -> element
	EnhBase    map[string]*EnhancementBase  // tier
	ItemIDs    []string
}

// Load parses every equipment-domain family of the snapshot into the
// typed Catalog. Missing families yield an empty catalog; malformed
// member records surface as diagnostics (missing identity fields are
// skipped, never silently defaulted).
func Load(c *config.CandidateSnapshot) (*Catalog, config.Diagnostics) {
	var d config.Diagnostics
	cat := &Catalog{
		Items:   map[string]*ItemDef{},
		Sets:    map[string]*SetDef{},
		Rolls:   map[string]*RollDef{},
		Budgets: map[string]*TierBudget{},
		Layouts: map[string]map[string]string{},
		EnhBase: map[string]*EnhancementBase{},
	}
	if c == nil {
		d.Addf(config.DiagIntegrationCheck, "", 0, "nil candidate snapshot")
		return cat, d
	}
	for _, rec := range familyRecs(c, "equipment") {
		it := parseItem(rec)
		if it.ID == "" {
			d.Addf(config.DiagIntegrationCheck, "equipment", 0, "item record without item_id")
			continue
		}
		if _, dup := cat.Items[it.ID]; dup {
			d.Addf(config.DiagDuplicatePrimaryKey, "equipment", 0,
				"duplicate generated item ID %s", it.ID)
			continue
		}
		cat.Items[it.ID] = it
		cat.ItemIDs = append(cat.ItemIDs, it.ID)
	}
	for _, rec := range familyRecs(c, "equipment_set") {
		s := parseSet(rec)
		if s.Key == "" {
			d.Addf(config.DiagIntegrationCheck, "equipment_set", 0, "set record without set_key")
			continue
		}
		cat.Sets[s.Key] = s
	}
	for _, rec := range familyRecs(c, "set_effect") {
		key, e := parseEffect(rec)
		if s, ok := cat.Sets[key]; ok && e != nil {
			s.Effects[e.Threshold] = e
		}
	}
	for _, rec := range familyRecs(c, "set_support") {
		key, s := parseSupport(rec)
		if set, ok := cat.Sets[key]; ok && s != nil {
			s.SetKey = key
			set.Support = s
		}
	}
	for _, rec := range familyRecs(c, "roll_def") {
		rd := parseRollDef(rec)
		if rd.ID != "" {
			cat.Rolls[rd.ID] = rd
		}
	}
	for _, rec := range familyRecs(c, "tier_budget") {
		tb := parseTierBudget(rec)
		if tb.Tier != "" {
			cat.Budgets[tb.Tier] = tb
		}
	}
	for _, rec := range familyRecs(c, "element_layout") {
		layout, slot := strField(rec, "layout"), strField(rec, "slot")
		if layout == "" || slot == "" {
			continue
		}
		if cat.Layouts[layout] == nil {
			cat.Layouts[layout] = map[string]string{}
		}
		cat.Layouts[layout][slot] = strField(rec, "element")
	}
	for _, rec := range familyRecs(c, "enhancement_base") {
		eb := parseEnhancementBase(rec)
		if eb.Tier != "" {
			cat.EnhBase[eb.Tier] = eb
		}
	}
	sort.Strings(cat.ItemIDs)

	// support priorities ride the validation_parameters sub-family.
	for _, pr := range paramRecs(c, "support_priority") {
		if s, ok := cat.Sets[paramKey(pr, 0)]; ok && s.Support != nil {
			s.Support.Priority = paramInt(pr, "priority")
		}
	}
	// flat roll bounds ride the flat_roll_ranges sub-family (one
	// `secondary_pool` record whose `ranges` list carries {stat, lo_coef,
	// unit, hi_coef, range_kind} entries).
	for _, pr := range paramRecs(c, "flat_roll_ranges") {
		f, ok := pr.Rec["fields"]
		if !ok {
			continue
		}
		ranges, ok := f.Rec["ranges"]
		if !ok || ranges.Kind != config.KindList {
			continue
		}
		for _, rv := range ranges.Elems {
			fr := &FlatRollRange{
				Stat:      rv.Rec["stat"].Str,
				Unit:      rv.Rec["unit"].Str,
				RangeKind: rv.Rec["range_kind"].Str,
			}
			if lo, ok := rv.Rec["lo_coef"]; ok {
				fr.Lo, _ = ratField(lo)
			}
			if hi, ok := rv.Rec["hi_coef"]; ok {
				fr.Hi, _ = ratField(hi)
			}
			cat.FlatRanges = append(cat.FlatRanges, fr)
		}
	}
	return cat, d
}

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

// paramRecs unfolds one validation_parameters sub-family into its
// {key,fields} record list (paramsFamily fold shape).
func paramRecs(c *config.CandidateSnapshot, name string) []config.Value {
	vp := c.ValidationParameters
	if vp == nil {
		return nil
	}
	rec, ok := vp.Records[config.KeyString([]config.Value{config.VStr(name)})]
	if !ok {
		return nil
	}
	if v, ok := rec.Fields["records"]; ok && v.Kind == config.KindList {
		return v.Elems
	}
	return nil
}

func paramKey(v config.Value, i int) string {
	if v.Rec == nil {
		return ""
	}
	k, ok := v.Rec["key"]
	if !ok || k.Kind != config.KindList || i >= len(k.Elems) {
		return ""
	}
	return k.Elems[i].Str
}

func paramInt(v config.Value, field string) int64 {
	f, ok := v.Rec["fields"]
	if !ok {
		return 0
	}
	iv, ok := f.Rec[field]
	if !ok || iv.Kind != config.KindInt {
		return 0
	}
	return iv.Int
}
