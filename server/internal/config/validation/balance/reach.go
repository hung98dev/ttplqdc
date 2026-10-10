package balance

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"thinhthan/internal/config"
)

// reachRow is one emitted line of the ADR-0047 audit matrix.
type reachRow struct {
	SkillID  string
	Kind     string
	OuterMM  int64 // outer envelope extent in mm
	BandNote string
	OK       bool
	Detail   string
}

const (
	projEnvelopeMM = 8800  // max_range + hit_radius <= 8.8m
	areaEnvelopeMM = 11000 // cast_range + radius <= 11.0m
	cameraHalfMM   = 12800 // 25.6m reference viewport half width
	marginMinMM    = 1800  // telegraph margin >= 1.8m
)

// separationRatio = min ranged-basic projectile range / max hostile
// melee reach; must be >= 2.50.
func separationRatioMM(c *config.CandidateSnapshot) (num, den int64, minRng, maxMelee int64) {
	minRng, maxMelee = -1, 0
	tags := skillTags(c)
	cats := skillCategories(c)
	for _, r := range familyRecs(c, "skill_action") {
		sid := fieldStr(r, "skill_id")
		g := geomRec(r)
		if g == nil {
			continue
		}
		kind := fieldStrGV(g, "kind")
		switch kind {
		case "PROJECTILE":
			if cats[sid] != "basic" {
				continue
			}
			if v, ok := geomMM(g, "range"); ok && (minRng < 0 || v < minRng) {
				minRng = v
			}
		case "MELEE_BOX":
			if v, ok := geomMM(g, "reach"); ok && v > maxMelee {
				maxMelee = v
			}
		case "SINGLE_TARGET_RANGE":
			if tags[sid]["HEAL"] || tags[sid]["DEFENSIVE"] {
				continue
			}
			if v, ok := geomMM(g, "range"); ok && v > maxMelee {
				maxMelee = v
			}
		}
	}
	// Monster melee attacks (attack family, RECT shape with MELEE/basic
	// profile) count toward the hostile-melee reach the same way.
	for _, r := range familyRecs(c, "attack") {
		if fieldStr(r, "shape") != "RECT" {
			continue
		}
		if p, s := fieldStr(r, "profile"), fieldStr(r, "suffix"); p != "MELEE" && s != "basic" {
			continue
		}
		if v, ok := fieldInt(r, "length_mm"); ok && v > maxMelee {
			maxMelee = v
		}
	}
	if minRng <= 0 || maxMelee <= 0 {
		return 0, 0, minRng, maxMelee
	}
	return minRng, maxMelee, minRng, maxMelee
}

func fieldStrGV(g map[string]config.Value, k string) string {
	if v, ok := g[k]; ok && v.Kind == config.KindString {
		return v.Str
	}
	return ""
}

func skillTags(c *config.CandidateSnapshot) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, r := range familyRecs(c, "skill") {
		id := fieldStr(r, "skill_id")
		tags := map[string]bool{}
		if v, ok := r.Fields["tags"]; ok && (v.Kind == config.KindSet || v.Kind == config.KindList) {
			for _, e := range v.Elems {
				tags[e.Str] = true
			}
		}
		out[id] = tags
	}
	return out
}

func skillCategories(c *config.CandidateSnapshot) map[string]string {
	out := map[string]string{}
	for _, r := range familyRecs(c, "skill") {
		out[fieldStr(r, "skill_id")] = fieldStr(r, "category")
	}
	return out
}

// outerReach returns a row's outer envelope extent in mm: the furthest
// point a hostile effect can reach from the caster.
func outerReach(g map[string]config.Value) int64 {
	var m int64
	if g == nil {
		return 0
	}
	get := func(k string) int64 {
		v, _ := geomMM(g, k)
		return v
	}
	switch fieldStrGV(g, "kind") {
	case "PROJECTILE":
		m = get("range") + get("radius")
	case "AREA_POSITION":
		m = get("cast") + get("radius")
	case "MELEE_BOX":
		m = get("reach")
	case "DIRECTION_BOX":
		m = get("length")
	case "DASH_LINE", "MOVE_CONTACT_LINE":
		m = get("distance")
	case "AREA_SELF":
		m = get("radius")
	case "SINGLE_TARGET_RANGE":
		m = get("range")
	case "BARRIER_POSITION":
		m = get("cast")
	}
	return m
}

// enumerateReach builds the 45-row matrix and evaluates the hard rejects
// that are not already covered by role-band envelope checks.
func enumerateReach(c *config.CandidateSnapshot) ([]reachRow, config.Diagnostics) {
	var d config.Diagnostics
	cats := skillCategories(c)
	rows := []reachRow{}
	basics, actives := 0, 0
	for _, r := range familyRecs(c, "skill_action") {
		sid := fieldStr(r, "skill_id")
		g := geomRec(r)
		row := reachRow{SkillID: sid}
		if g == nil {
			row.Detail = "missing geometry"
			rows = append(rows, row)
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.skill_geometry: %s missing typed geometry", sid)
			continue
		}
		row.Kind = fieldStrGV(g, "kind")
		row.OuterMM = outerReach(g)
		row.OK = true
		switch cats[sid] {
		case "basic":
			basics++
			row.BandNote = "basic"
		case "active":
			actives++
			row.BandNote = "active"
		default:
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.skill_geometry: %s has no basic/active skill row", sid)
			row.OK = false
		}
		// hard reject: envelope ceilings
		if row.Kind == "PROJECTILE" {
			rng, _ := geomMM(g, "range")
			rad, _ := geomMM(g, "radius")
			if rng+rad > projEnvelopeMM {
				row.OK = false
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.skill_geometry: %s projectile %dmm > 8800mm", sid, rng+rad)
			}
		}
		if row.Kind == "AREA_POSITION" {
			cast, _ := geomMM(g, "cast")
			rad, _ := geomMM(g, "radius")
			if cast+rad > areaEnvelopeMM {
				row.OK = false
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.skill_geometry: %s area %dmm > 11000mm", sid, cast+rad)
			}
		}
		if v, ok := r.Fields["air_geometry"]; ok && v.Kind == config.KindRecord {
			if fieldStrGV(v.Rec, "kind") == "" {
				row.OK = false
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.skill_geometry: %s air variant missing kind", sid)
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SkillID < rows[j].SkillID })
	if basics+actives != 45 {
		d.Addf(config.DiagBalanceGuardrail, "", 0,
			"balance.skill_geometry: primary geometry rows %d basics + %d actives, want 20 + 25",
			basics, actives)
	}
	// separation ratio >= 2.50
	if n, dd, minRng, maxMelee := separationRatioMM(c); n > 0 {
		if n*100 < dd*250 {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.skill_geometry: separation ratio %d/%dmm = %.4f < 2.50",
				minRng, maxMelee, float64(n)/float64(dd))
		}
	} else {
		d.Addf(config.DiagIntegrationCheck, "", 0,
			"balance.skill_geometry: cannot derive separation ratio (no ranged-basic or hostile melee)")
	}
	// camera margin: 12.8 - max outer >= 1.8
	var maxOuter int64
	for _, row := range rows {
		if row.OuterMM > maxOuter {
			maxOuter = row.OuterMM
		}
	}
	if maxOuter > 0 && cameraHalfMM-maxOuter < marginMinMM {
		d.Addf(config.DiagBalanceGuardrail, "", 0,
			"balance.skill_geometry: camera margin %dmm < %dmm",
			cameraHalfMM-maxOuter, marginMinMM)
	}
	// secondary spatial effects need complete typed fields
	for _, r := range familyRecs(c, "spatial_effect") {
		id := fieldStr(r, "spatial_effect_id")
		for _, req := range []string{"source", "origin_shape", "exact_resolution", "target_cap_interaction"} {
			if fieldStr(r, req) == "" {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.skill_geometry: spatial_effect %q missing %s", id, req)
			}
		}
	}
	checkTagDisplacement(c, &d)
	return rows, d
}

var dispRe = regexp.MustCompile(`(?i)airborne|knock|push|pull|displace|forced`)

// checkTagDisplacement enforces the ADR-0047 tag/effect displacement
// consistency: DISPLACEMENT tag <=> a forced-position or canonical
// AIRBORNE spatial result.
func checkTagDisplacement(c *config.CandidateSnapshot, d *config.Diagnostics) {
	tags := skillTags(c)
	spatial := map[string]config.Record{}
	for _, r := range familyRecs(c, "spatial_effect") {
		spatial[fieldStr(r, "spatial_effect_id")] = r
	}
	skillSpatial := map[string][]string{}
	for _, r := range familyRecs(c, "skill_effect") {
		p, ok := r.Fields["payload"]
		if !ok || p.Kind != config.KindRecord {
			continue
		}
		if p.Rec["kind"].Str == "SPATIAL" {
			if ref, ok := p.Rec["ref_id"]; ok && ref.Kind == config.KindString {
				sid := fieldStr(r, "skill_id")
				skillSpatial[sid] = append(skillSpatial[sid], ref.Str)
			}
		}
	}
	for _, r := range familyRecs(c, "basic_proc") {
		if v, ok := r.Fields["status_effects"]; ok && v.Kind == config.KindList {
			sid := fieldStr(r, "skill_id")
			for _, e := range v.Elems {
				if e.Kind == config.KindString && strings.HasPrefix(e.Str, "spatial.") {
					skillSpatial[sid] = append(skillSpatial[sid], e.Str)
				}
			}
		}
	}
	for id, t := range tags {
		result := false
		for _, ref := range skillSpatial[id] {
			se, ok := spatial[ref]
			if ok && (dispRe.MatchString(ref) || dispRe.MatchString(fieldStr(se, "exact_resolution"))) {
				result = true
			}
		}
		if t["DISPLACEMENT"] != result {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.skill_geometry: skill %q DISPLACEMENT tag=%v but forced-position/AIRBORNE result=%v",
				id, t["DISPLACEMENT"], result)
		}
	}
}

func checkReach(c *config.CandidateSnapshot, d *config.Diagnostics) {
	_, rd := enumerateReach(c)
	*d = append(*d, rd...)
}

// reachReportLine renders one audit row for the report.
func (r reachRow) String() string {
	res := "PASS"
	if !r.OK {
		res = "FAIL"
	}
	return fmt.Sprintf("%s | %s | outer %dmm | %s | %s", r.SkillID, r.Kind, r.OuterMM, r.BandNote, res)
}
