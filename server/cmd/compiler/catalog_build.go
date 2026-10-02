package main

import (
	"regexp"
	"sort"
	"strings"

	"thinhthan/internal/config"
)

// compileBuild — build_catalog.md driver: Meridian resonances (15) and
// Formations (12) over fixed slot rings; includes the catalog's own
// reachability enumeration (256 BASIC / 64 ADVANCED sequences) which must
// prove every definition has a selected-winner witness.
func compileBuild(c *Ctx, f *File, r *Registry) {
	st := &buildState{
		resonances: map[string]*buildDef{},
		formations: map[string]*buildDef{},
	}
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Shared Rules"):
			buildSharedRules(c, f, b)
		case strings.HasPrefix(path, "Meridian"):
			st.wantRes, st.hasResDecl = bindingDecl(b, reDeclMeridian)
			buildMeridian(c, f, b, st)
		case strings.HasPrefix(path, "Formations"):
			st.wantForm, st.hasFormDecl = bindingDecl(b, reDeclFormations)
			buildFormations(c, f, b, st)
		case strings.HasPrefix(path, "Reachability Validation"):
			// compile-time enumeration runs in buildVerifyReachable;
			// register the declared assertion shape
			if sec := f.Root.SectionAt("Reachability Validation"); sec != nil {
				for _, bl := range sec.Content {
					for j, l := range bl.FLines {
						l = strings.TrimSpace(l)
						if l == "" || strings.HasPrefix(l, "for ") || strings.HasPrefix(l, "}") {
							continue
						}
						c.EmitParam(f.Name, b.Raw, "reachability_assertion",
							[]config.Value{config.VStr(l)},
							map[string]config.Value{"expr_text": config.VStr(l)}, bl.Line+1+j)
					}
				}
			}
			c.consumed(f, b)
		case strings.HasPrefix(path, "Power Budget"):
			// re-point blocks + budget totals -> params
			if sec := f.Root.SectionAt("Power Budget Verification — ADR-0037"); sec != nil {
				for _, ch := range sec.Children {
					c.EmitParam(f.Name, b.Raw, "power_budget",
						[]config.Value{config.VStr(ch.Title)},
						map[string]config.Value{"note": config.VStr(ch.Title)}, ch.Line)
				}
			}
			c.consumed(f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"build binding %q has no driver", b.Raw)
		}
	}
	buildVerifyReachable(c, f, st)
}

type buildDef struct {
	id       string
	display  string
	priority int
	group    string
	matcher  buildMatcher
	effects  []string
	witness  string
	line     int
	isForm   bool
}

// ---- matcher model -------------------------------------------------------

type matcherClause struct {
	relations []string // SINH_OUT+..., or one element
	op        string   // >= | <= | =
	n         int
}

type buildMatcher struct {
	kind      string // ELEMENT_COUNT | RELATION_CHAIN | RELATION_COUNT | RELATION_SUBSEQUENCE | FULL_RING | RELATION_COUNT_PATTERN | EXPLICIT_SEQUENCE | PAIRED_COUNT
	clauses   []matcherClause
	element   string
	min       int
	relations []string
	sequence  []string // element sequence for EXPLICIT_SEQUENCE
	rotate    bool
	reflect   bool
}

var (
	sinhNext = map[string]string{"KIM": "THUY", "THUY": "MOC", "MOC": "HOA", "HOA": "THO", "THO": "KIM"}
	khacNext = map[string]string{"KIM": "MOC", "MOC": "THO", "THO": "THUY", "THUY": "HOA", "HOA": "KIM"}
)

func linkRelation(a, b string) string {
	switch {
	case a == b:
		return "DONG_HE"
	case sinhNext[a] == b:
		return "SINH_OUT"
	case sinhNext[b] == a:
		return "SINH_IN"
	case khacNext[a] == b:
		return "KHAC_OUT"
	default:
		return "KHAC_IN"
	}
}

// seqRelations computes cyclic link relations for an element sequence.
func seqRelations(seq []string) []string {
	n := len(seq)
	rel := make([]string, n)
	for i := 0; i < n; i++ {
		rel[i] = linkRelation(seq[i], seq[(i+1)%n])
	}
	return rel
}

func elemCount(seq []string, elem string) int {
	n := 0
	for _, e := range seq {
		if e == elem {
			n++
		}
	}
	return n
}

func relCount(rels []string, kinds []string) int {
	n := 0
	for _, r := range rels {
		for _, k := range kinds {
			if r == k {
				n++
				break
			}
		}
	}
	return n
}

// chainLen counts the longest cyclic run of rel==kind.
func chainLen(rels []string, kind string) int {
	n := len(rels)
	best := 0
	for s := 0; s < n; s++ {
		cur := 0
		for i := 0; i < n && rels[(s+i)%n] == kind; i++ {
			cur++
		}
		if cur > best {
			best = cur
		}
	}
	return best
}

// subsequenceMatch checks cyclic contiguous run equal to want.
func subsequenceMatch(rels, want []string) bool {
	n := len(rels)
	for s := 0; s < n; s++ {
		ok := true
		for i, w := range want {
			if rels[(s+i)%n] != w {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func cmpClause(cnt int, op string, n int) bool {
	switch op {
	case ">=":
		return cnt >= n
	case "<=":
		return cnt <= n
	default:
		return cnt == n
	}
}

func matchOK(m buildMatcher, seq []string, rels []string) bool {
	switch m.kind {
	case "ELEMENT_COUNT":
		return elemCount(seq, m.element) >= m.min
	case "RELATION_CHAIN":
		return chainLen(rels, m.relations[0]) >= m.min
	case "RELATION_COUNT":
		return relCount(rels, m.relations) >= m.min
	case "RELATION_SUBSEQUENCE":
		return subsequenceMatch(rels, m.relations)
	case "FULL_RING":
		for _, cl := range m.clauses {
			if !cmpClause(relCount(rels, cl.relations), cl.op, cl.n) {
				return false
			}
		}
		return true
	case "RELATION_COUNT_PATTERN":
		for _, cl := range m.clauses {
			if !cmpClause(relCount(rels, cl.relations), cl.op, cl.n) {
				return false
			}
		}
		return true
	case "PAIRED_COUNT":
		for _, cl := range m.clauses {
			if !cmpClause(elemCount(seq, cl.relations[0]), cl.op, cl.n) {
				return false
			}
		}
		return true
	case "EXPLICIT_SEQUENCE":
		return seqMatch(seq, m)
	}
	return false
}

func seqMatch(seq []string, m buildMatcher) bool {
	n := len(seq)
	if len(m.sequence) != n {
		return false
	}
	want := func(s []string, at func(i int) string) bool {
		for i := 0; i < n; i++ {
			if seq[i] != at(i) {
				return false
			}
		}
		return true
	}
	for rot := 0; rot < n; rot++ {
		if !m.rotate && rot > 0 {
			break
		}
		r := rot
		if want(m.sequence, func(i int) string { return m.sequence[(i+r)%n] }) {
			return true
		}
	}
	if m.reflect {
		for rot := 0; rot < n; rot++ {
			if !m.rotate && rot > 0 {
				break
			}
			r := rot
			if want(m.sequence, func(i int) string { return m.sequence[(r-i+n)%n] }) {
				return true
			}
		}
	}
	return false
}

// ---- parsing --------------------------------------------------------------

var buildUnlockRe = regexp.MustCompile(`^(Meridian|Formation) unlock = Level ([0-9]+)`)
var relChainRe = regexp.MustCompile(`^([A-Z_]+) chain >= ([0-9]+)$`)
var relTotalRe = regexp.MustCompile(`^total ([A-Z_]+) links >= ([0-9]+)$`)
var elemMinRe = regexp.MustCompile(`^([A-Z_]+) >= ([0-9]+)$`)
var relClauseRe = regexp.MustCompile(`^([A-Z_+ ]+?)\s*(link count\s*)?(>=|<=|=)\s*([0-9]+)$`)
var seqArrowRe = regexp.MustCompile(`^([A-Z_]+)\s*(->\s*[A-Z_]+\s*)+$`)

func parseMatcherCell(c *Ctx, f *File, raw string, group string, line int) buildMatcher {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "`", "")
	if m := elemMinRe.FindStringSubmatch(raw); m != nil && strings.Contains(raw, ">=") {
		n, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
		return buildMatcher{kind: "ELEMENT_COUNT", element: m[1], min: int(n.Int)}
	}
	if m := relChainRe.FindStringSubmatch(raw); m != nil {
		n, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
		return buildMatcher{kind: "RELATION_CHAIN", relations: []string{m[1]}, min: int(n.Int)}
	}
	if m := relTotalRe.FindStringSubmatch(raw); m != nil {
		n, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
		return buildMatcher{kind: "RELATION_COUNT", relations: []string{m[1]}, min: int(n.Int)}
	}
	if group == "EXPLICIT" && strings.Contains(raw, ",") {
		var rels []string
		for _, t := range strings.Split(raw, ",") {
			rels = append(rels, strings.TrimSpace(t))
		}
		return buildMatcher{kind: "RELATION_SUBSEQUENCE", relations: rels}
	}
	c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "matcher %q", raw)
	return buildMatcher{}
}

// parseMatcherFence parses the per-subsection matcher fence.
func parseMatcherFence(c *Ctx, f *File, lines []string, line int) buildMatcher {
	// EXPLICIT_SEQUENCE: `E -> E -> ...` + allow_* lines
	var seq []string
	var clauses []matcherClause
	full := false
	rotate, reflect := false, false
	seqSeen := false
	for _, l := range lines {
		l = strings.TrimSpace(strings.Trim(l, "`"))
		if l == "" {
			continue
		}
		if l == "FULL_RING" {
			full = true
			continue
		}
		if strings.HasPrefix(l, "allow_rotation") {
			rotate = strings.HasSuffix(l, "true")
			seqSeen = true
			continue
		}
		if strings.HasPrefix(l, "allow_reflection") {
			reflect = strings.HasSuffix(l, "true")
			continue
		}
		if seqArrowRe.MatchString(l) {
			for _, t := range strings.Split(l, "->") {
				seq = append(seq, strings.TrimSpace(t))
			}
			seqSeen = true
			continue
		}
		if m := relClauseRe.FindStringSubmatch(l); m != nil {
			var toks []string
			for _, t := range strings.Split(m[1], "+") {
				toks = append(toks, strings.TrimSpace(t))
			}
			nv, _ := (TypeSpec{Name: "int"}).ParseValue(m[4])
			clauses = append(clauses, matcherClause{relations: toks, op: m[3], n: int(nv.Int)})
			continue
		}
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "matcher fence line %q", l)
	}
	switch {
	case full:
		return buildMatcher{kind: "FULL_RING", clauses: clauses}
	case seqSeen:
		return buildMatcher{kind: "EXPLICIT_SEQUENCE", sequence: seq, rotate: rotate, reflect: reflect}
	case len(clauses) > 0:
		// element-token clause -> PAIRED_COUNT (elements), else relation counts
		isElem := clauses[0].relations[0] == "KIM" || clauses[0].relations[0] == "MOC" ||
			clauses[0].relations[0] == "THUY" || clauses[0].relations[0] == "HOA" ||
			clauses[0].relations[0] == "THO"
		if isElem {
			return buildMatcher{kind: "PAIRED_COUNT", clauses: clauses}
		}
		return buildMatcher{kind: "RELATION_COUNT_PATTERN", clauses: clauses}
	}
	return buildMatcher{}
}

// parseWitnessFence: `KEY bitstring` lines.
func parseWitnessFence(lines []string) map[string]string {
	out := map[string]string{}
	for _, l := range lines {
		l = strings.TrimSpace(strings.Trim(l, "`"))
		f := strings.Fields(l)
		if len(f) == 2 && regexp.MustCompile(`^[01]+$`).MatchString(f[1]) {
			out[f[0]] = f[1]
		}
	}
	return out
}

// statOpRe extracts `+0.04 STAT` / `-0.08 DEFENSE` / `STAT +0.08` deltas.
var statOpRe = regexp.MustCompile("([+-])([0-9.]+)%?\\s*\\`?([A-Z_]+)\\`?|\\`?([A-Z_]+)\\`?\\s*([+-])([0-9.]+)")

func parseEffectOps(text string) []config.Value {
	var ops []config.Value
	for _, m := range statOpRe.FindAllStringSubmatch(text, -1) {
		var sign, num, stat string
		if m[1] != "" {
			sign, num, stat = m[1], m[2], m[3]
		} else {
			stat, sign, num = m[4], m[5], m[6]
		}
		if strings.Contains(num, "%") {
			continue
		}
		d, err := parseDecimal(sign + num)
		if err != nil {
			continue
		}
		ops = append(ops, config.VRec(map[string]config.Value{
			"stat": config.VStr(stat), "value": mustRat(d),
		}))
	}
	return ops
}

func emitBuildDef(c *Ctx, f *File, d *buildDef, family string) {
	matcherVal := map[string]config.Value{"kind": config.VStr(d.matcher.kind)}
	if d.matcher.element != "" {
		matcherVal["element"] = config.VStr(d.matcher.element)
	}
	if d.matcher.min > 0 {
		matcherVal["min"] = config.VInt(int64(d.matcher.min))
	}
	if len(d.matcher.relations) > 0 {
		var rs []config.Value
		for _, r := range d.matcher.relations {
			rs = append(rs, config.VStr(r))
		}
		matcherVal["relations"] = config.VList(rs...)
	}
	if len(d.matcher.clauses) > 0 {
		var cls []config.Value
		for _, cl := range d.matcher.clauses {
			var rs []config.Value
			for _, r := range cl.relations {
				rs = append(rs, config.VStr(r))
			}
			cls = append(cls, config.VRec(map[string]config.Value{
				"relations": config.VList(rs...), "op": config.VStr(cl.op),
				"n": config.VInt(int64(cl.n)),
			}))
		}
		matcherVal["clauses"] = config.VList(cls...)
	}
	if len(d.matcher.sequence) > 0 {
		var sq []config.Value
		for _, e := range d.matcher.sequence {
			sq = append(sq, config.VStr(e))
		}
		matcherVal["sequence"] = config.VList(sq...)
		matcherVal["allow_rotation"] = config.VBool(d.matcher.rotate)
		matcherVal["allow_reflection"] = config.VBool(d.matcher.reflect)
	}
	var effs []config.Value
	for _, e := range d.effects {
		effs = append(effs, config.VStr(e))
	}
	effectText := strings.Join(d.effects, "; ")
	fields := map[string]config.Value{
		"display":     config.VStr(d.display),
		"priority":    config.VInt(int64(d.priority)),
		"group":       config.VStr(d.group),
		"matcher":     config.VRec(matcherVal),
		"effect_text": config.VStr(effectText),
		"effects":     config.VList(effs...),
		"stat_ops":    config.VList(parseEffectOps(effectText)...),
	}
	if d.witness != "" {
		fields["witness"] = config.VStr(d.witness)
	}
	fields[func() string {
		if d.isForm {
			return "formation_id"
		}
		return "resonance_id"
	}()] = config.VStr(d.id)
	c.Emit(f.Name, d.group, family,
		[]config.Value{config.VStr(d.id)}, fields, d.line)
}

// ---- sections ---------------------------------------------------------------

func buildSharedRules(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Shared Rules")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Shared Rules missing")
		return
	}
	for _, fb := range allFences(sec, "text") {
		for j, l := range fb.FLines {
			l = strings.TrimSpace(l)
			if m := buildUnlockRe.FindStringSubmatch(l); m != nil {
				lv, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				c.EmitParam(f.Name, b.Raw, "build_unlock",
					[]config.Value{config.VStr(strings.ToUpper(m[1]))},
					map[string]config.Value{
						"system":       config.VStr(strings.ToUpper(m[1])),
						"unlock_level": lv,
					}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

// buildRingSections reads ring/domain/group structures shared by Meridian
// and Formations.
func buildRingSections(c *Ctx, f *File, root *Section, system string) []string {
	// ring cycle fence at the root level
	var slots []string
	for _, bl := range root.Content {
		if bl.Kind == BlockFence {
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if strings.Contains(l, "->") {
					for _, t := range strings.Split(l, "->") {
						slots = append(slots, strings.TrimSpace(t))
					}
				}
			}
		}
	}
	if len(slots) > 1 && slots[0] == slots[len(slots)-1] {
		slots = slots[:len(slots)-1] // cycle repeats first slot
	}
	if len(slots) == 0 {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1,
			"%s ring fence missing", system)
		return nil
	}
	var sv []config.Value
	for _, s := range slots {
		sv = append(sv, config.VStr(s))
	}
	c.EmitParam(f.Name, system, "ring_topology",
		[]config.Value{config.VStr(system)},
		map[string]config.Value{"slots": config.VList(sv...)}, 0)
	return slots
}

func buildDomainFence(c *Ctx, f *File, sec *Section, system string, slots []string) map[string][2]string {
	dom := map[string][2]string{}
	if sec == nil {
		return dom
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockFence {
			continue
		}
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			f := strings.Fields(l)
			if len(f) == 3 && f[0] != "slot" {
				dom[f[0]] = [2]string{f[1], f[2]}
			}
		}
	}
	var keys []string
	for k := range dom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	order := func(s2 string) int {
		for i, sl := range slots {
			if sl == s2 {
				return i
			}
		}
		return len(keys)
	}
	for _, slot := range keys {
		pair := dom[slot]
		c.Emit(f.Name, system+" Launch Equipment Domain", "ring_domain",
			[]config.Value{config.VStr(system), config.VStr(slot)},
			map[string]config.Value{
				"system": config.VStr(system), "slot": config.VStr(slot),
				"ordinal": config.VInt(int64(order(slot))),
				"set_a":   config.VStr(pair[0]), "set_b": config.VStr(pair[1]),
			}, 0)
	}
	return dom
}

func buildMeridian(c *Ctx, f *File, b *SourceBinding, st *buildState) {
	raw := b.Raw
	switch {
	case strings.Contains(raw, "Meridian — 15 Resonances / Launch Equipment Domain") ||
		strings.Contains(raw, "ring fence +") && strings.Contains(raw, "Meridian") && !strings.Contains(raw, "THEME") && !strings.Contains(raw, "FLOW") && !strings.Contains(raw, "EXPLICIT") && !strings.Contains(raw, "FULL_RING"):
		st.meridianSlots = buildRingSections(c, f, f.Root.SectionAt("Meridian — 15 Resonances"), "MERIDIAN")
		st.meridianDom = buildDomainFence(c, f,
			f.Root.SectionAt("Meridian — 15 Resonances > Launch Equipment Domain"), "MERIDIAN", st.meridianSlots)
		if gsec := f.Root.SectionAt("Meridian — 15 Resonances > Coexistence Groups"); gsec != nil {
			for _, bl := range gsec.Content {
				if bl.Kind == BlockFence {
					var groups []config.Value
					for _, l := range bl.FLines {
						l = strings.TrimSpace(l)
						if l != "" {
							groups = append(groups, config.VStr(l))
						}
					}
					st.groups = groups
					c.EmitParam(f.Name, "Coexistence Groups", "coexistence_groups",
						[]config.Value{config.VStr("MERIDIAN")},
						map[string]config.Value{"groups": config.VList(groups...)}, bl.Line)
				}
			}
		}
	case strings.Contains(raw, "THEME — 5"):
		buildTableGroup(c, f, "Meridian — 15 Resonances > THEME — 5", "resonance", "THEME", st)
	case strings.Contains(raw, "FLOW — 5"):
		buildTableGroup(c, f, "Meridian — 15 Resonances > FLOW — 5", "resonance", "FLOW", st)
	case strings.Contains(raw, "EXPLICIT — 3"):
		buildTableGroup(c, f, "Meridian — 15 Resonances > EXPLICIT — 3", "resonance", "EXPLICIT", st)
	case strings.Contains(raw, "FULL_RING — 2"):
		buildHeadingGroup(c, f, "Meridian — 15 Resonances > FULL_RING — 2", "resonance", "FULL_RING", st)
	}
	c.consumed(f, b)
}

func buildFormations(c *Ctx, f *File, b *SourceBinding, st *buildState) {
	raw := b.Raw
	root := f.Root.SectionAt("Formations — 12")
	switch {
	case strings.Contains(raw, "ring fence"):
		st.formationSlots = buildRingSections(c, f, root, "FORMATION")
		st.formationDom = buildDomainFence(c, f,
			f.Root.SectionAt("Formations — 12 > Launch Equipment Domain"), "FORMATION", st.formationSlots)
	case strings.Contains(raw, "Element Emphasis — 5"):
		buildTableGroup(c, f, "Formations — 12 > Element Emphasis — 5", "formation", "EMPHASIS", st)
	case strings.Contains(raw, "Generation Relation Patterns") ||
		strings.Contains(raw, "Control Relation Patterns") ||
		strings.Contains(raw, "Paired Motifs"):
		// one binding covers all three pattern sections
		for _, secName := range []string{
			"Formations — 12 > Generation Relation Patterns — 2",
			"Formations — 12 > Control Relation Patterns — 2",
			"Formations — 12 > Paired Motifs — 2"} {
			buildHeadingGroup(c, f, secName, "formation", "PATTERN", st)
		}
	case strings.Contains(raw, "Rare Explicit"):
		buildHeadingGroup(c, f, "Formations — 12 > Rare Explicit — 1", "formation", "EXPLICIT_SEQUENCE", st)
	}
	c.consumed(f, b)
}

// buildTableGroup handles `## X — N` groups whose members are table rows.
func buildTableGroup(c *Ctx, f *File, secPath, family, group string, st *buildState) {
	sec := f.Root.SectionAt(secPath)
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "%s missing", secPath)
		return
	}
	witnesses := map[string]string{}
	for _, bl := range sec.Content {
		switch bl.Kind {
		case BlockTable:
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				id := cellAt(row, 0).Scalar()
				pr, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 2).Scalar())
				matcher := parseMatcherCell(c, f, cellAt(row, 3).Text, group, row[0].Line)
				d := &buildDef{
					id:       id,
					display:  cellAt(row, 1).Scalar(),
					priority: int(pr.Int),
					group:    group,
					matcher:  matcher,
					effects:  []string{strings.TrimSpace(cellAt(row, 4).Text)},
					line:     row[0].Line,
					isForm:   family == "formation",
				}
				if family == "formation" {
					st.formations[id] = d
				} else {
					st.resonances[id] = d
				}
			}
		case BlockFence:
			for k, v := range parseWitnessFence(bl.FLines) {
				witnesses[k] = v
			}
		}
	}
	// attach witnesses: THEME keys are elements, FLOW keys relations,
	// EXPLICIT keys are name tails
	for _, defs := range []map[string]*buildDef{st.resonances, st.formations} {
		for id, d := range defs {
			if d.group != group {
				continue
			}
			tail := id[strings.LastIndex(id, ".")+1:]
			for k, w := range witnesses {
				if k == tail || strings.Contains(id, strings.ToLower(k)) ||
					strings.EqualFold(k, tail) {
					d.witness = w
				}
			}
			// element/relation keys
			for k, w := range witnesses {
				if strings.HasPrefix(k, "KIM") || strings.HasPrefix(k, "MOC") ||
					strings.HasPrefix(k, "THUY") || strings.HasPrefix(k, "HOA") ||
					strings.HasPrefix(k, "THO") || strings.HasPrefix(k, "SINH") ||
					strings.HasPrefix(k, "KHAC") || strings.HasPrefix(k, "DONG") {
					if d.matcher.element == k ||
						(len(d.matcher.relations) > 0 && d.matcher.relations[0] == k) {
						d.witness = w
					}
				}
			}
			emitBuildDef(c, f, d, family)
		}
	}
}

// buildHeadingGroup handles `## X — N` groups whose members are `### `id` — name`
// subsections with priority line + matcher fence + Effect bullets + witness.
var buildHeadRe = regexp.MustCompile("^`(resonance|formation)\\.([a-z0-9_.]+)`\\s*—\\s*(.+)$")
var buildPrioRe = regexp.MustCompile("priority:?\\s*`?([0-9]+)`?")
var witnessLineRe = regexp.MustCompile("[Ww]itness[^`]*`([01]+)`")

func buildHeadingGroup(c *Ctx, f *File, secPath, family, group string, st *buildState) {
	sec := f.Root.SectionAt(secPath)
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "%s missing", secPath)
		return
	}
	for _, ch := range sec.Children {
		m := buildHeadRe.FindStringSubmatch(ch.Title)
		if m == nil {
			continue
		}
		d := &buildDef{
			id:      m[1] + "." + m[2],
			display: strings.TrimSpace(m[3]),
			group:   group,
			line:    ch.Line,
			isForm:  family == "formation",
		}
		inEffects := false
		for _, bl := range ch.Content {
			for _, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if pm := buildPrioRe.FindStringSubmatch(l); pm != nil {
					pr, _ := (TypeSpec{Name: "int"}).ParseValue(pm[1])
					d.priority = int(pr.Int)
				}
				if wm := witnessLineRe.FindStringSubmatch(l); wm != nil {
					d.witness = wm[1]
				}
				if strings.HasPrefix(l, "Effect:") || strings.HasPrefix(l, "Effect ") {
					inEffects = true
					continue
				}
				if inEffects && strings.HasPrefix(l, "-") {
					d.effects = append(d.effects, strings.TrimSpace(strings.TrimPrefix(l, "-")))
				}
			}
			if bl.Kind == BlockFence && d.matcher.kind == "" {
				d.matcher = parseMatcherFence(c, f, bl.FLines, bl.Line)
			}
		}
		if len(d.effects) == 0 {
			// single-line Effect: <text>
			for _, bl := range ch.Content {
				for _, l := range bl.Prose {
					l = strings.TrimSpace(l)
					if strings.HasPrefix(l, "Effect:") {
						d.effects = append(d.effects, strings.TrimSpace(strings.TrimPrefix(l, "Effect:")))
					}
				}
			}
		}
		if family == "formation" {
			st.formations[d.id] = d
		} else {
			st.resonances[d.id] = d
		}
		emitBuildDef(c, f, d, family)
	}
}

type buildState struct {
	resonances     map[string]*buildDef
	formations     map[string]*buildDef
	meridianSlots  []string
	meridianDom    map[string][2]string
	formationSlots []string
	formationDom   map[string][2]string
	groups         []config.Value

	wantRes     int64
	hasResDecl  bool
	wantForm    int64
	hasFormDecl bool
}

// ---- reachability validation ----------------------------------------------

// enumerate sequences for a domain: bit i of mask picks Set B for slot i.
func buildSequences(slots []string, dom map[string][2]string) [][]string {
	n := len(slots)
	var out [][]string
	for mask := 0; mask < (1 << n); mask++ {
		seq := make([]string, n)
		for i, s := range slots {
			pair := dom[s]
			if mask&(1<<i) != 0 {
				seq[i] = pair[1]
			} else {
				seq[i] = pair[0]
			}
		}
		out = append(out, seq)
	}
	return out
}

// bitstring renders mask in canonical slot order (0=A, 1=B).
func bitstring(mask, n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		if mask&(1<<i) != 0 {
			b[i] = '1'
		} else {
			b[i] = '0'
		}
	}
	return string(b)
}

// selectedGroupWinner: highest priority satisfied def; tie -> id asc.
func selectWinner(defs []*buildDef, seq []string, rels []string) *buildDef {
	var best *buildDef
	for _, d := range defs {
		if !matchOK(d.matcher, seq, rels) {
			continue
		}
		if best == nil || d.priority > best.priority ||
			(d.priority == best.priority && d.id < best.id) {
			best = d
		}
	}
	return best
}

func buildVerifyReachable(c *Ctx, f *File, st *buildState) {
	if st.hasResDecl && int64(len(st.resonances)) != st.wantRes {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"Meridian definitions %d != declared %d", len(st.resonances), st.wantRes)
	}
	if st.hasFormDecl && int64(len(st.formations)) != st.wantForm {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"Formation definitions %d != declared %d", len(st.formations), st.wantForm)
	}
	if len(st.meridianSlots) == 0 || len(st.formationSlots) == 0 {
		return
	}
	// group partition for resonances
	byGroup := map[string][]*buildDef{}
	for _, d := range st.resonances {
		byGroup[d.group] = append(byGroup[d.group], d)
	}
	resWins := map[string]int{}
	resWitnessSeq := map[string]string{}
	for mask, seq := range buildSequences(st.meridianSlots, st.meridianDom) {
		rels := seqRelations(seq)
		for g, defs := range byGroup {
			w := selectWinner(defs, seq, rels)
			if w == nil {
				continue
			}
			// FULL_RING suppresses EXPLICIT
			if g == "EXPLICIT" {
				if fw := selectWinner(byGroup["FULL_RING"], seq, rels); fw != nil {
					continue
				}
			}
			resWins[w.id]++
			resWitnessSeq[w.id] = bitstring(mask, len(st.meridianSlots))
		}
	}
	for id, d := range st.resonances {
		if resWins[id] == 0 {
			c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, d.line,
				"resonance %s unreachable: no selected-winner sequence", id)
			continue
		}
		// declared witness must itself select this definition
		if d.witness != "" {
			mask := 0
			for i, ch := range d.witness {
				if ch == '1' {
					mask |= 1 << i
				}
			}
			seq := make([]string, len(st.meridianSlots))
			for i, s := range st.meridianSlots {
				if mask&(1<<i) != 0 {
					seq[i] = st.meridianDom[s][1]
				} else {
					seq[i] = st.meridianDom[s][0]
				}
			}
			rels := seqRelations(seq)
			w := selectWinner(byGroup[d.group], seq, rels)
			if w == nil || w.id != id {
				c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, d.line,
					"resonance %s declared witness %s does not select it (winner %v)",
					id, d.witness, w)
				continue
			}
			if d.group == "EXPLICIT" {
				if fw := selectWinner(byGroup["FULL_RING"], seq, rels); fw != nil {
					c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, d.line,
						"resonance %s witness %s suppressed by FULL_RING %s",
						id, d.witness, fw.id)
				}
			}
		}
	}
	_ = resWitnessSeq
	// formations: single winner group (only one active per loadout)
	var formDefs []*buildDef
	for _, d := range st.formations {
		formDefs = append(formDefs, d)
	}
	formWins := map[string]int{}
	for _, seq := range buildSequences(st.formationSlots, st.formationDom) {
		rels := seqRelations(seq)
		if w := selectWinner(formDefs, seq, rels); w != nil {
			formWins[w.id]++
		}
	}
	for id, d := range st.formations {
		if formWins[id] == 0 {
			c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, d.line,
				"formation %s unreachable: no selected-winner sequence", id)
			continue
		}
		if d.witness != "" {
			mask := 0
			for i, ch := range d.witness {
				if ch == '1' {
					mask |= 1 << i
				}
			}
			seq := make([]string, len(st.formationSlots))
			for i, s := range st.formationSlots {
				if mask&(1<<i) != 0 {
					seq[i] = st.formationDom[s][1]
				} else {
					seq[i] = st.formationDom[s][0]
				}
			}
			rels := seqRelations(seq)
			w := selectWinner(formDefs, seq, rels)
			if w == nil || w.id != id {
				c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, d.line,
					"formation %s declared witness %s does not select it", id, d.witness)
			}
		}
	}
}
