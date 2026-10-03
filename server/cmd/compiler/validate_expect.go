package main

// Bundle-declared expectations (F-2.3): every launch magnitude the
// validator enforces is read from the bundle's own authored fences — the
// balance_validation guardrail fences, the soul Element Count Validation
// fence, and the README Launch Content Budget — never hard-coded, so a
// minimal-but-complete fixture bundle can declare its own small domains.

import (
	"regexp"
	"strconv"
	"strings"
)

// fenceLines yields every fence body line in every section of f.
func fenceLines(f *File) []string {
	var out []string
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, b := range s.Content {
			if b.Kind == BlockFence {
				out = append(out, b.FLines...)
			}
		}
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	if f != nil && f.Root != nil {
		walk(f.Root)
	}
	return out
}

// sectionFenceLines yields fence body lines under sections whose cleaned
// title contains want (case-sensitive substring).
func sectionFenceLines(f *File, want string) []string {
	var out []string
	var walk func(s *Section)
	walk = func(s *Section) {
		if strings.Contains(strings.ReplaceAll(s.Title, "`", ""), want) {
			for _, b := range s.Content {
				if b.Kind == BlockFence {
					out = append(out, b.FLines...)
				}
			}
		}
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	if f != nil && f.Root != nil {
		walk(f.Root)
	}
	return out
}

func fAtof(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil
}

var (
	reTTKNormal = regexp.MustCompile(`([0-9.]+)s? <= NORMAL [a-z-]*.*TTK <= ([0-9.]+)s`)
	reTTKElite  = regexp.MustCompile(`([0-9.]+)s? <= ELITE [a-z-]*.*TTK <= ([0-9.]+)s`)
	reTTKSolo   = regexp.MustCompile(`([0-9.]+)s? <= solo [a-z-]*.*TTK <= ([0-9.]+)s`)
	reTTKParty  = regexp.MustCompile(`([0-9.]+)s? <= five-player [a-z-]*.*TTK <= ([0-9.]+)s`)
	reHeavyPct  = regexp.MustCompile(`([0-9.]+)%? *\.\. *([0-9.]+)%? ?reference MAX_HP`)
	reSpread    = regexp.MustCompile(`boss TTK / min basic-only boss TTK <= ([0-9.]+)`)
	reBossHP    = regexp.MustCompile(`MAX_HP *= *floor\(([0-9]+) *\+ *([0-9]+)\*L *\+ *([0-9]+)\*L\*L\)`)
	reGeomRows  = regexp.MustCompile(`primary geometry row count *!= *([0-9]+)`)
	reProjCap   = regexp.MustCompile(`projectile max_range_m *\+ *hit_radius_m *> *([0-9.]+)m`)
	reAreaCap   = regexp.MustCompile(`cast_range_m *\+ *radius_m *> *([0-9.]+)m`)
	reSepRatio  = regexp.MustCompile(`hostile melee reach *< *([0-9.]+)`)
	reEnhTier   = regexp.MustCompile(`^T([1-9]) *\+([0-9]+)`)
	reBonusLv   = regexp.MustCompile(`Lv([0-9]+) *= *([0-9]+)`)
	reSplitPct  = regexp.MustCompile(`floor\(([0-9]+)% of earned\)`)
)

// balanceGates holds the gate constants declared by the bundle's
// balance_validation catalog. present marks each declared group; an
// undeclared group skips its check rather than guessing a launch number.
type balanceGates struct {
	normalLo, normalHi   float64
	eliteLo, eliteHi     float64
	soloLo, soloHi       float64
	partyLo, partyHi     float64
	heavyLo, heavyHi     float64
	spreadMax            float64
	hpBase, hpLin, hpQ   float64
	geomRows             int64
	projCapMM, areaCapMM float64
	sepRatio             float64
	enh                  map[int64]float64
	bonus                map[int64]int64
	offPct, vitPct       float64

	hasNormal, hasElite, hasSolo, hasParty bool
	hasHeavy, hasSpread, hasHP             bool
	hasGeomRows, hasProjCap, hasAreaCap    bool
	hasSepRatio, hasEnh, hasBonus          bool
	hasSplit                               bool
}

// parseBalanceGates scans the bundle's balance_validation catalog fences
// for declared gate constants.
func parseBalanceGates(c *Ctx) balanceGates {
	g := balanceGates{
		enh:    map[int64]float64{1: 4, 2: 5, 3: 6, 4: 6, 5: 7, 6: 8},
		bonus:  map[int64]int64{25: 10, 30: 20, 35: 30, 40: 40, 45: 60, 50: 80, 55: 100, 60: 120},
		offPct: 50, vitPct: 25,
	}
	f := c.Catalogs["balance_validation.md"]
	if f == nil {
		return g
	}
	for _, l := range fenceLines(f) {
		l = strings.TrimSpace(l)
		if m := reTTKNormal.FindStringSubmatch(l); m != nil {
			g.normalLo, _ = fAtof(m[1])
			g.normalHi, _ = fAtof(m[2])
			g.hasNormal = true
		}
		if m := reTTKElite.FindStringSubmatch(l); m != nil {
			g.eliteLo, _ = fAtof(m[1])
			g.eliteHi, _ = fAtof(m[2])
			g.hasElite = true
		}
		if m := reTTKSolo.FindStringSubmatch(l); m != nil {
			g.soloLo, _ = fAtof(m[1])
			g.soloHi, _ = fAtof(m[2])
			g.hasSolo = true
		}
		if m := reTTKParty.FindStringSubmatch(l); m != nil {
			g.partyLo, _ = fAtof(m[1])
			g.partyHi, _ = fAtof(m[2])
			g.hasParty = true
		}
		if m := reHeavyPct.FindStringSubmatch(l); m != nil {
			g.heavyLo, _ = fAtof(m[1])
			g.heavyHi, _ = fAtof(m[2])
			g.hasHeavy = true
		}
		if m := reSpread.FindStringSubmatch(l); m != nil {
			g.spreadMax, _ = fAtof(m[1])
			g.hasSpread = true
		}
		if m := reBossHP.FindStringSubmatch(l); m != nil {
			g.hpBase, _ = fAtof(m[1])
			g.hpLin, _ = fAtof(m[2])
			g.hpQ, _ = fAtof(m[3])
			g.hasHP = true
		}
		if m := reGeomRows.FindStringSubmatch(l); m != nil {
			v, _ := fAtof(m[1])
			g.geomRows = int64(v)
			g.hasGeomRows = true
		}
		if m := reProjCap.FindStringSubmatch(l); m != nil {
			v, _ := fAtof(m[1])
			g.projCapMM = v * 1000
			g.hasProjCap = true
		}
		if m := reAreaCap.FindStringSubmatch(l); m != nil {
			v, _ := fAtof(m[1])
			g.areaCapMM = v * 1000
			g.hasAreaCap = true
		}
		if m := reSepRatio.FindStringSubmatch(l); m != nil {
			g.sepRatio, _ = fAtof(m[1])
			g.hasSepRatio = true
		}
		if m := reEnhTier.FindStringSubmatch(l); m != nil {
			t, _ := fAtof(m[1])
			v, _ := fAtof(m[2])
			g.enh[int64(t)] = v
			g.hasEnh = true
		}
		for _, m := range reBonusLv.FindAllStringSubmatch(l, -1) {
			lv, _ := fAtof(m[1])
			v, _ := fAtof(m[2])
			g.bonus[int64(lv)] = int64(v)
			g.hasBonus = true
		}
		if m := reSplitPct.FindStringSubmatch(l); m != nil {
			v, _ := fAtof(m[1])
			if !g.hasSplit {
				g.offPct = v
			} else {
				g.vitPct = v
			}
			g.hasSplit = true
		}
	}
	return g
}

// soulExpectation is one element's declared rank distribution.
type soulExpectation struct {
	normal, elite, boss, total int
}

var reSoulDist = regexp.MustCompile(`^([A-Z]+) *= *([0-9]+) NORMAL(?: *\+ *([0-9]+) ELITE)?(?: *\+ *([0-9]+) BOSS)? *= *([0-9]+)`)
var reSoulTotal = regexp.MustCompile(`^TOTAL *= *([0-9]+)`)

// parseSoulExpectations reads the soul catalog's Element Count Validation
// fence; nil when the bundle declares none.
func parseSoulExpectations(c *Ctx) (map[string]soulExpectation, int, bool) {
	f := c.Catalogs["soul_catalog.md"]
	if f == nil {
		return nil, 0, false
	}
	out := map[string]soulExpectation{}
	total := 0
	found := false
	for _, l := range sectionFenceLines(f, "Element Count Validation") {
		l = strings.TrimSpace(l)
		if m := reSoulDist.FindStringSubmatch(l); m != nil {
			e := soulExpectation{}
			e.normal, _ = strconv.Atoi(m[2])
			if m[3] != "" {
				e.elite, _ = strconv.Atoi(m[3])
			}
			if m[4] != "" {
				e.boss, _ = strconv.Atoi(m[4])
			}
			e.total, _ = strconv.Atoi(m[5])
			out[m[1]] = e
			found = true
		}
		if m := reSoulTotal.FindStringSubmatch(l); m != nil {
			total, _ = strconv.Atoi(m[1])
			found = true
		}
	}
	return out, total, found
}

// expectedSpaceIDs derives the playable space set from bundle content:
// every world map, every dungeon, every boss whose space is an
// `instance.*` arena, and every competitive space declared by the
// CAT-006 spec-section sources. Returns nil when the bundle declares no
// space sources.
func expectedSpaceIDs(c *Ctx) map[string]bool {
	out := map[string]bool{}
	for _, r := range famRecs(c, "world_map") {
		out[fStr(r, "map_id")] = true
	}
	for _, r := range famRecs(c, "dungeon") {
		out[fStr(r, "dungeon_id")] = true
	}
	for _, r := range famRecs(c, "boss") {
		if sp := fStr(r, "space_id"); strings.HasPrefix(sp, "instance.") {
			out[sp] = true
		}
	}
	if comp, _ := c.Data["competitive.spaces"].(map[string]bool); comp != nil {
		for id := range comp {
			out[id] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var (
	reDeclNE          = regexp.MustCompile(`([0-9]+) NORMAL \+ ([0-9]+) ELITE`)
	reDeclExactlyNorm = regexp.MustCompile(`Exactly ([0-9]+) NORMAL`)
	reDeclRows        = regexp.MustCompile(`Exactly ([0-9]+) rows?`)
	reDeclItems       = regexp.MustCompile(`= ?([0-9]+) items`)
	reDeclProfiles    = regexp.MustCompile(`([0-9]+) profiles`)
	reDeclTransitions = regexp.MustCompile(`([0-9]+) transitions`)
	reDeclMeridian    = regexp.MustCompile(`Meridian — ([0-9]+)`)
	reDeclFormations  = regexp.MustCompile(`Formations — ([0-9]+)`)
	reDeclSeasonalNxN = regexp.MustCompile(`([0-9]+) ?[×x] ?([0-9]+) ?= ?([0-9]+)`)
	reDeclRecipes     = regexp.MustCompile(`= ?([0-9]+) recipes`)
	reDeclSumTo       = regexp.MustCompile(`weights sum to ([0-9]+)`)
	reDeclSum         = regexp.MustCompile(`sum ?([0-9]+)`)
	reDeclEncounter   = regexp.MustCompile(`([0-9]+) regions, ([0-9]+) field maps, ([0-9]+) dungeons, ([0-9]+) major bosses`)
	reDeclTotal       = regexp.MustCompile(`\(([0-9]+) total\)`)
	reDeclDashN       = regexp.MustCompile(`— ([0-9]+)`)
	reDeclEmission    = regexp.MustCompile(`= ?([0-9]+)`)
	reDeclBudgetTotal = regexp.MustCompile(`TOTAL = ?([0-9,]+)`)
	reDeclPerClass    = regexp.MustCompile(`([0-9]+) basic \+ ([0-9]+) active \+ ([0-9]+) passive`)
	reDeclTierSpecial = regexp.MustCompile(`T1=([0-9]+)/T2=([0-9]+)/T3=([0-9]+) special`)
	reDeclMaxSpecial  = regexp.MustCompile(`max ([0-9]+)`)
	reDeclActValue    = regexp.MustCompile(`\b(I|II|III|IV|V|VI) ([0-9]+)\b`)
	reDeclCheckpoints = regexp.MustCompile(`Checkpoints — ([0-9]+)`)
)

// readmeSkillCats reads `N basic + M active + K passive skills per class`
// from the bundle README's content-budget fences.
func readmeSkillCats(c *Ctx) (int64, int64, int64, bool) {
	f := c.Catalogs["README.md"]
	if f == nil {
		return 0, 0, 0, false
	}
	for _, l := range fenceLines(f) {
		if m := reDeclPerClass.FindStringSubmatch(l); m != nil {
			b, e1 := strconv.ParseInt(m[1], 10, 64)
			a, e2 := strconv.ParseInt(m[2], 10, 64)
			p, e3 := strconv.ParseInt(m[3], 10, 64)
			if e1 == nil && e2 == nil && e3 == nil {
				return b, a, p, true
			}
		}
	}
	return 0, 0, 0, false
}

// fileDeclN scans all section titles in a catalog for a declared count.
func fileDeclN(c *Ctx, catalog string, re *regexp.Regexp) (int64, bool) {
	f := c.Catalogs[catalog]
	if f == nil || f.Root == nil {
		return 0, false
	}
	var found int64
	ok := false
	var walk func(s *Section)
	walk = func(s *Section) {
		if m := re.FindStringSubmatch(s.Title); m != nil {
			if v, err := strconv.ParseInt(m[len(m)-1], 10, 64); err == nil {
				found = v
				ok = true
			}
		}
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	walk(f.Root)
	return found, ok
}

// bindingDecl extracts a declared finite-rule count from a registry
// binding row (source_section / output / typed inputs / defaults cells).
// Patterns are matched in order; the last capture group wins.
func bindingDecl(b *SourceBinding, res ...*regexp.Regexp) (int64, bool) {
	if b == nil {
		return 0, false
	}
	text := b.Raw + " | " + b.Output + " | " + b.InputsText + " | " + b.DefaultsText
	for _, re := range res {
		if m := re.FindStringSubmatch(text); m != nil {
			v, err := strconv.ParseInt(strings.ReplaceAll(m[len(m)-1], ",", ""), 10, 64)
			if err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

// bindingDecl2 extracts a two-component declared count (e.g. `N NORMAL +
// M ELITE`); captures 1..2 are returned.
func bindingDecl2(b *SourceBinding, re *regexp.Regexp) (int64, int64, bool) {
	if b == nil {
		return 0, 0, false
	}
	text := b.Raw + " | " + b.Output + " | " + b.InputsText + " | " + b.DefaultsText
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0, 0, false
	}
	a, err1 := strconv.ParseInt(m[1], 10, 64)
	v, err2 := strconv.ParseInt(m[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return a, v, true
}

// bindingDecl4 extracts a four-component declared count (encounter budget).
func bindingDecl4(b *SourceBinding, re *regexp.Regexp) ([4]int64, bool) {
	var out [4]int64
	if b == nil {
		return out, false
	}
	text := b.Raw + " | " + b.Output + " | " + b.InputsText + " | " + b.DefaultsText
	m := re.FindStringSubmatch(text)
	if m == nil {
		return out, false
	}
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseInt(m[i+1], 10, 64)
		if err != nil {
			return out, false
		}
		out[i] = v
	}
	return out, true
}
