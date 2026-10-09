package equipment

import "thinhthan/internal/config"

// StatTerm is one resolved fixed-stat term: an integer flat value
// (coefficient x tier unit, floored per term) or a fraction utility
// stat (ring CRIT_CHANCE / charm COOLDOWN_REDUCTION).
type StatTerm struct {
	Stat     string
	Flat     int64      // floored unit-resolved value for flat stats
	Fraction config.Rat // fraction value for utility stats
}

// IsUtility reports the term is a fraction utility stat (not enhanceable).
func (t StatTerm) IsUtility() bool { return t.Fraction.Den != 0 }

// ItemDef is one emitted `item.eq.*` definition.
type ItemDef struct {
	ID             string
	Kind           string
	Tier           string
	SetKey         string
	Slot           string
	SlotOrdinal    int64
	Element        string
	LevelMin       int64
	LevelMax       int64
	Rarity         string
	Binding        string
	BindingTrigger string
	StackLimit     int64
	FixedStats     []StatTerm
	Enhanceable    []string
	SecondaryRolls int64
}

// EffectDef is one `Npc [effect.set.<key>.<n>] -> payload` fence.
type EffectDef struct {
	ID        string
	Threshold int64
	Payload   string
}

// SupportDef is the set's 2pc-derived support signature.
type SupportDef struct {
	ID       string
	SetKey   string
	Payload  string
	Priority int64
}

// SetDef is one `set.tN.<key>` definition with its fences.
type SetDef struct {
	ID             string
	Key            string
	Tier           string
	Layout         string
	Display        string
	SourceIdentity string
	Effects        map[int64]*EffectDef
	Support        *SupportDef
}

// RollDef is one closed secondary-roll definition.
type RollDef struct {
	ID         string
	Stat       string
	Kind       string // FLAT or UTILITY
	Stage      string // utility rolls: FLAT_ADD
	RangeKind  string // flat rolls: FLAT_UNIT
	TierRanges map[string][2]config.Rat
}

// FlatRollRange is one entry of the `flat_roll_ranges` validation
// parameter: a floored inclusive coefficient bound resolved against a
// tier unit (§ flat-range fence).
type FlatRollRange struct {
	Stat      string
	Unit      string // A, D, H or M
	Lo        config.Rat
	Hi        config.Rat
	RangeKind string // FLAT_UNIT
}

// TierBudget is one Tier Budget row's authoring units.
type TierBudget struct {
	Tier           string
	Levels         string
	Rarity         string
	UnitA          int64
	UnitD          int64
	UnitH          int64
	UnitM          int64
	SecondaryRolls int64
}

// EnhancementBase is one tier's base attempt-cost units.
type EnhancementBase struct {
	Tier           string
	MaterialUnits  int64
	CommonCurrency int64
}

func strField(rec config.Record, name string) string {
	if v, ok := rec.Fields[name]; ok && v.Kind == config.KindString {
		return v.Str
	}
	return ""
}

func intField(rec config.Record, name string) (int64, bool) {
	v, ok := rec.Fields[name]
	if !ok {
		return 0, false
	}
	if v.Kind == config.KindInt {
		return v.Int, true
	}
	if v.Kind == config.KindRational {
		return v.Rat.Int()
	}
	return 0, false
}

func ratField(v config.Value) (config.Rat, bool) {
	if v.Kind == config.KindRational {
		return v.Rat, true
	}
	if v.Kind == config.KindInt {
		return config.Rat{Num: v.Int, Den: 1}, true
	}
	return config.Rat{}, false
}

// parseItem reads one emitted `equipment` record.
func parseItem(rec config.Record) *ItemDef {
	it := &ItemDef{
		ID:         strField(rec, "item_id"),
		Kind:       strField(rec, "kind"),
		Tier:       strField(rec, "tier"),
		SetKey:     strField(rec, "set_key"),
		Slot:       strField(rec, "slot"),
		Element:    strField(rec, "element"),
		Rarity:     strField(rec, "rarity"),
		Binding:    strField(rec, "binding"),
		StackLimit: 1,
	}
	it.BindingTrigger = strField(rec, "binding_trigger")
	it.SlotOrdinal, _ = intField(rec, "slot_ordinal")
	it.LevelMin, _ = intField(rec, "level_min")
	it.LevelMax, _ = intField(rec, "level_max")
	if v, ok := intField(rec, "stack_limit"); ok {
		it.StackLimit = v
	}
	it.SecondaryRolls, _ = intField(rec, "secondary_rolls")
	if v, ok := rec.Fields["fixed_stats"]; ok && v.Kind == config.KindList {
		for _, term := range v.Elems {
			st := StatTerm{}
			if s, ok := term.Rec["stat"]; ok {
				st.Stat = s.Str
			}
			if val, ok := term.Rec["value"]; ok {
				switch val.Kind {
				case config.KindInt:
					st.Flat = val.Int
				case config.KindRational:
					st.Fraction = val.Rat
				}
			}
			it.FixedStats = append(it.FixedStats, st)
		}
	}
	if v, ok := rec.Fields["enhanceable_stats"]; ok &&
		(v.Kind == config.KindSet || v.Kind == config.KindList) {
		for _, e := range v.Elems {
			it.Enhanceable = append(it.Enhanceable, e.Str)
		}
	}
	return it
}

// parseSet reads one emitted `equipment_set` record.
func parseSet(rec config.Record) *SetDef {
	return &SetDef{
		ID:             strField(rec, "set_id"),
		Key:            strField(rec, "set_key"),
		Tier:           strField(rec, "tier"),
		Layout:         strField(rec, "layout"),
		Display:        strField(rec, "display"),
		SourceIdentity: strField(rec, "source_identity"),
		Effects:        map[int64]*EffectDef{},
	}
}

// parseEffect reads one emitted `set_effect` record into a SetDef.
func parseEffect(rec config.Record) (key string, e *EffectDef) {
	thr, _ := intField(rec, "threshold")
	return strField(rec, "set_key"), &EffectDef{
		ID:        strField(rec, "effect_id"),
		Threshold: thr,
		Payload:   strField(rec, "payload"),
	}
}

// parseSupport reads one emitted `set_support` record.
func parseSupport(rec config.Record) (key string, s *SupportDef) {
	return strField(rec, "set_key"), &SupportDef{
		ID:      strField(rec, "support_id"),
		SetKey:  strField(rec, "set_key"),
		Payload: strField(rec, "payload"),
	}
}

// parseRollDef reads one emitted `roll_def` record.
func parseRollDef(rec config.Record) *RollDef {
	rd := &RollDef{
		ID:         strField(rec, "stat_id"),
		Stat:       strField(rec, "stat"),
		Kind:       strField(rec, "kind"),
		Stage:      strField(rec, "stage"),
		RangeKind:  strField(rec, "range_kind"),
		TierRanges: map[string][2]config.Rat{},
	}
	if v, ok := rec.Fields["tier_ranges"]; ok && v.Kind == config.KindList {
		for _, tr := range v.Elems {
			tier := ""
			if t, ok := tr.Rec["tier"]; ok {
				tier = t.Str
			}
			if rng, ok := tr.Rec["range"]; ok && rng.Kind == config.KindList &&
				len(rng.Elems) == 2 && tier != "" {
				lo, lok := ratField(rng.Elems[0])
				hi, hok := ratField(rng.Elems[1])
				if lok && hok {
					rd.TierRanges[tier] = [2]config.Rat{lo, hi}
				}
			}
		}
	}
	return rd
}

// parseTierBudget reads one emitted `tier_budget` record.
func parseTierBudget(rec config.Record) *TierBudget {
	tb := &TierBudget{
		Tier:   strField(rec, "tier"),
		Levels: strField(rec, "levels"),
		Rarity: strField(rec, "rarity"),
	}
	tb.UnitA, _ = intField(rec, "unit_A")
	tb.UnitD, _ = intField(rec, "unit_D")
	tb.UnitH, _ = intField(rec, "unit_H")
	tb.UnitM, _ = intField(rec, "unit_M")
	tb.SecondaryRolls, _ = intField(rec, "secondary_rolls")
	return tb
}

// parseEnhancementBase reads one emitted `enhancement_base` record.
func parseEnhancementBase(rec config.Record) *EnhancementBase {
	eb := &EnhancementBase{Tier: strField(rec, "tier")}
	eb.MaterialUnits, _ = intField(rec, "material_units")
	eb.CommonCurrency, _ = intField(rec, "common_currency")
	return eb
}
