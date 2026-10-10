package balance

import (
	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// Check re-derives every balance_validation.md release guardrail from the
// compiled candidate and returns diagnostics for each violation. It never
// widens a declared guardrail; a detected violation means the content
// magnitudes are wrong, not the window.
func Check(c *config.CandidateSnapshot) config.Diagnostics {
	var d config.Diagnostics
	if c == nil {
		d.Addf(config.DiagIntegrationCheck, "", 0, "nil candidate snapshot")
		return d
	}
	cat, ld := equipment.Load(c)
	d = append(d, ld...)
	checkCompiledInventory(c, &d)
	checkTTK(c, cat, &d)
	checkPowerBudget(c, cat, &d)
	checkSustain(c, cat, &d)
	checkReach(c, &d)
	checkColliders(c, &d)
	return d
}

// Register attaches the balance re-verification suite to an activation
// gate (equipment.Register precedent).
func Register(g *config.Gate) {
	g.RegisterCheck(Check)
}

// checkCompiledInventory asserts the families the suite consumes are
// present (ordered step 1): equipment roll pool, item expansion, skill
// geometry, collider profiles, monster/boss stats.
func checkCompiledInventory(c *config.CandidateSnapshot, d *config.Diagnostics) {
	for _, fam := range []struct {
		name string
		min  int
	}{
		{"equipment", 168},
		{"equipment_set", 12},
		{"monster", 1},
		{"boss", 1},
		{"skill", 1},
		{"skill_action", 45},
		{"basic_proc", 1},
		{"spatial_effect", 1},
	} {
		if len(familyRecs(c, fam.name)) < fam.min {
			d.Addf(config.DiagSourceSchemaMissing, "", 0,
				"balance: family %q has %d records, want >= %d", fam.name,
				len(familyRecs(c, fam.name)), fam.min)
		}
	}
}

// ---- shared record helpers -----------------------------------------------

func familyRecs(c *config.CandidateSnapshot, name string) []config.Record {
	f := c.Definitions[name]
	if f == nil {
		return nil
	}
	out := make([]config.Record, 0, len(f.Records))
	for _, r := range f.Records {
		out = append(out, r)
	}
	return out
}

func fieldStr(r config.Record, k string) string {
	v, ok := r.Fields[k]
	if ok && v.Kind == config.KindString {
		return v.Str
	}
	return ""
}

func fieldInt(r config.Record, k string) (int64, bool) {
	v, ok := r.Fields[k]
	if ok && v.Kind == config.KindInt {
		return v.Int, true
	}
	return 0, false
}

func fieldRat(r config.Record, k string) (config.Rat, bool) {
	v, ok := r.Fields[k]
	if !ok {
		return config.Rat{}, false
	}
	switch v.Kind {
	case config.KindRational:
		return v.Rat, true
	case config.KindInt:
		return config.Rat{Num: v.Int, Den: 1}, true
	}
	return config.Rat{}, false
}

// geomRec reads the typed `geometry` record of a skill_action row.
func geomRec(r config.Record) map[string]config.Value {
	v, ok := r.Fields["geometry"]
	if !ok || v.Kind != config.KindRecord {
		return nil
	}
	return v.Rec
}

func ratFloat(r config.Rat) float64 {
	if r.Den == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Den)
}

// geomMM reads a millimetre geometry argument (int or rational mm).
func geomMM(g map[string]config.Value, k string) (int64, bool) {
	v, ok := g[k]
	if !ok {
		return 0, false
	}
	switch v.Kind {
	case config.KindInt:
		return v.Int, true
	case config.KindRational:
		if v.Rat.Den != 0 {
			return v.Rat.Num / v.Rat.Den, true
		}
	}
	return 0, false
}
