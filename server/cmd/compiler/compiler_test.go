package main

// IMP-003 named tests. Launch-count assertions (168/52/45/25/...) always read
// the count declared inside the bundle under test, never a literal hardcoded
// here — the compiler derives every magnitude from catalog declarations.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"thinhthan/internal/config"
)

var realCatalogDir = filepath.Join("..", "..", "..", "docs", "07_content")
var goldenDir = filepath.Join("..", "..", "internal", "config", "testdata", "golden_bundle")

func newTestCtx(dir string) *Ctx {
	return &Ctx{
		Defs:     &FamilyStore{},
		Params:   &FamilyStore{},
		Geom:     &FamilyStore{},
		Diags:    &config.Diagnostics{},
		Warnings: &config.Diagnostics{},
		Cov:      &config.CoverageReport{},
		Refs:     NewNamespaceIndex(),
		Rules:    map[string]int{},
		Catalogs: map[string]*File{},
		Data:     map[string]any{},
		Dir:      dir,
	}
}

func compileClean(t *testing.T, dir string) (*Ctx, *config.CandidateSnapshot) {
	t.Helper()
	c := newTestCtx(dir)
	if err := runPipeline(c); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, d := range *c.Diags {
		t.Errorf("unexpected diagnostic: %s", d)
	}
	for _, d := range *c.Warnings {
		t.Errorf("unexpected warning: %s", d)
	}
	snap, err := emitSnapshot(c)
	if err != nil {
		t.Fatalf("emitSnapshot: %v", err)
	}
	return c, snap
}

func compileAny(t *testing.T, dir string) *Ctx {
	t.Helper()
	c := newTestCtx(dir)
	_ = runPipeline(c)
	return c
}

func hasCode(c *Ctx, code config.DiagnosticCode) bool {
	for _, d := range *c.Diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// copyBundle clones every *.md file in dir into a fresh temp directory.
func copyBundle(t *testing.T, dir string) string {
	t.Helper()
	out := t.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// CAT-006 spec-section sources resolve at ../03_systems beside the
	// catalogs dir — replicate that layout beside the temp copy.
	specSrc := filepath.Join(dir, "..", "03_systems")
	if se, err := os.ReadDir(specSrc); err == nil {
		specDst := filepath.Join(out, "..", "03_systems")
		if err := os.MkdirAll(specDst, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, e := range se {
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(specSrc, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(specDst, e.Name()), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return out
}

func mutateFile(t *testing.T, dir, name string, fn func(string) string) {
	t.Helper()
	p := filepath.Join(dir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	m := fn(string(b))
	if m == string(b) {
		t.Fatalf("mutation on %s was a no-op", p)
	}
	if err := os.WriteFile(p, []byte(m), 0o644); err != nil {
		t.Fatal(err)
	}
}

func revision(t *testing.T, dir string) string {
	t.Helper()
	_, snap := compileClean(t, dir)
	return snap.ContentRevision
}

func TestCompileAllCatalogs(t *testing.T) {
	c, snap := compileClean(t, realCatalogDir)
	if snap == nil {
		t.Fatal("no snapshot")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(snap.ContentRevision) {
		t.Fatalf("content_revision %q is not a SHA-256 hex string", snap.ContentRevision)
	}
	rep := buildReport(c, snap)
	if got := len(rep.CatalogsEvaluated); got != len(Drivers) {
		t.Fatalf("catalogs_evaluated = %d, want %d", got, len(Drivers))
	}
	if rep.ContentRevisionHash != snap.ContentRevision {
		t.Fatalf("report hash %q != snapshot revision %q", rep.ContentRevisionHash, snap.ContentRevision)
	}
	if len(rep.ValidationDiagnostics) != 0 {
		t.Fatalf("report carries %d diagnostics on a clean compile", len(rep.ValidationDiagnostics))
	}

	// Determinism: a second in-process compile yields the identical payload.
	_, snap2 := compileClean(t, realCatalogDir)
	p1, err := config.CanonicalBytes(config.CanonicalPayload(snap))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := config.CanonicalBytes(config.CanonicalPayload(snap2))
	if err != nil {
		t.Fatal(err)
	}
	if string(p1) != string(p2) || snap.ContentRevision != snap2.ContentRevision {
		t.Fatal("two compiles of the same bundle diverged")
	}
}

// declaredCount reads `= N` from the named catalog's registry binding that
// names out substring — used where the finite rule declares an emission count.
// declaredExpansion returns the `= N` finite-rule count on the binding whose
// source_section contains rawMatch — e.g. "= 168 item_id", "= 52".
func declaredExpansion(t *testing.T, c *Ctx, catalog, rawMatch string) int64 {
	t.Helper()
	f := c.Catalogs[catalog]
	if f == nil {
		t.Fatalf("catalog %s not loaded", catalog)
	}
	reg, err := LoadRegistry(f)
	if err != nil {
		t.Fatalf("%s registry: %v", catalog, err)
	}
	for _, b := range reg.Bindings {
		if strings.Contains(b.Raw, rawMatch) {
			if n, ok := bindingDecl(b, reDeclEmission); ok {
				return n
			}
		}
	}
	t.Fatalf("no `= N` declaration found in %s for %q", catalog, rawMatch)
	return 0
}

func TestEquipmentExpansion168(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	want := declaredExpansion(t, c, "equipment_catalog.md", "Concrete Item-ID Expansion")
	if want != 168 {
		t.Fatalf("bundle declares %d equipment rows, want 168", want)
	}
	if got := len(famRecs(c, "equipment")); got != int(want) {
		t.Fatalf("compiled %d equipment records, bundle declares %d", got, want)
	}
}

func TestPortalExpansion52(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	f := c.Catalogs["world_route_catalog.md"]
	reg, err := LoadRegistry(f)
	if err != nil {
		t.Fatal(err)
	}
	var want int64
	var ok bool
	for _, b := range reg.Bindings {
		if strings.Contains(b.Raw, "Portal Count Validation") {
			if n, ok2 := bindingDecl(b, reDeclEmission); ok2 {
				want, ok = n, true
			}
		}
	}
	if !ok {
		t.Fatal("no portal count declaration in world_route_catalog.md")
	}
	if want != 52 {
		t.Fatalf("bundle declares %d portal rows, want 52", want)
	}
	if got := len(famRecs(c, "portal")); got != int(want) {
		t.Fatalf("compiled %d portal records, bundle declares %d", got, want)
	}
}

func TestEntitySizeProfileResolution(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	valid := map[string]bool{
		"MONSTER_SMALL": true, "MONSTER_MEDIUM": true, "MONSTER_ELITE": true,
		"BOSS_LARGE": true, "WORLD_BOSS": true,
	}
	seen := 0
	for _, fam := range []string{"monster", "boss"} {
		for _, r := range famRecs(c, fam) {
			sp := fStr(r, "size_profile")
			if !valid[sp] {
				t.Fatalf("%s %v resolves size_profile %q — not in the closed enum", fam, r.Key, sp)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no monster/boss records resolved a size profile")
	}
}

func TestPlayableSpaceGeometryIndex(t *testing.T) {
	c, snap := compileClean(t, realCatalogDir)
	want := expectedSpaceIDs(c)
	if len(want) == 0 {
		t.Fatal("bundle declares no playable spaces")
	}
	got := map[string]bool{}
	for _, r := range snap.Geometry.SortedKeys() {
		rec := snap.Geometry.Records[config.KeyString(r)]
		got[fStr(rec, "space_id")] = true
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("declared playable space %q missing from geometry.spaces", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Fatalf("geometry.spaces carries undeclared space %q", id)
		}
	}
}

// Competitive-space compile inputs (CAT-006, ADR-0080): pvp.md's geometry
// table emits the PVP rows; guild_war.md's canonical fence emits the
// GUILD_WAR row; declared anchor sets ride on each row's `anchors` field.
func TestCompetitiveSpaceGeometryIndex(t *testing.T) {
	_, snap := compileClean(t, realCatalogDir)
	get := func(id string) config.Record {
		for _, k := range snap.Geometry.SortedKeys() {
			r := snap.Geometry.Records[config.KeyString(k)]
			if fStr(r, "space_id") == id {
				return r
			}
		}
		t.Fatalf("geometry.spaces missing %q", id)
		return config.Record{}
	}
	anchors := func(r config.Record) []string {
		v, ok := r.Fields["anchors"]
		if !ok || v.Kind != config.KindList {
			return nil
		}
		out := []string{}
		for _, e := range v.Elems {
			out = append(out, e.Str)
		}
		return out
	}
	modes := func(r config.Record) []string {
		v, ok := r.Fields["modes"]
		if !ok || v.Kind != config.KindList {
			return nil
		}
		out := []string{}
		for _, e := range v.Elems {
			out = append(out, e.Str)
		}
		return out
	}
	join := func(ss []string) string { return strings.Join(ss, ",") }

	court := get("map.pvp.duel_court")
	if fStr(court, "kind") != "PVP" {
		t.Fatalf("duel_court kind %q, want PVP", fStr(court, "kind"))
	}
	if got := join(modes(court)); got != "DUEL,RANKED_DUEL" {
		t.Fatalf("duel_court modes %q, want DUEL,RANKED_DUEL", got)
	}
	if n := len(anchors(court)); n != 0 {
		t.Fatalf("duel_court declares %d anchors, want none", n)
	}

	arena := get("map.pvp.five_element_arena")
	if got := join(anchors(arena)); got != "altar.left,altar.center,altar.right" {
		t.Fatalf("arena anchors %q, want altar.left,altar.center,altar.right", got)
	}
	if got := join(modes(arena)); got != "FIVE_ELEMENT_ARENA" {
		t.Fatalf("arena modes %q, want FIVE_ELEMENT_ARENA", got)
	}

	war := get("map.guild_war.five_seal_conflict")
	if fStr(war, "kind") != "GUILD_WAR" {
		t.Fatalf("five_seal_conflict kind %q, want GUILD_WAR", fStr(war, "kind"))
	}
	if got := join(anchors(war)); got !=
		"guild_war.seal.moc,guild_war.seal.hoa,guild_war.seal.tho,guild_war.seal.kim,guild_war.seal.thuy" {
		t.Fatalf("five_seal_conflict anchors %q", got)
	}
	if fStr(war, "required_topology") == "" || fStr(war, "layout_profile") == "" {
		t.Fatalf("five_seal_conflict missing topology/profile fields")
	}
}

func TestSkillGeometryRows45(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	gates := parseBalanceGates(c)
	if gates.geomRows != 45 {
		t.Fatalf("bundle declares %d primary geometry rows, want 45", gates.geomRows)
	}
	if got := int64(len(famRecs(c, "skill_action"))); got != gates.geomRows {
		t.Fatalf("compiled %d skill_action geometry rows, bundle declares %d", got, gates.geomRows)
	}
}

func TestSkillSecondaryGeometryCompile(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	recs := famRecs(c, "spatial_effect")
	if len(recs) == 0 {
		t.Fatal("no spatial_effect records compiled")
	}
	for _, r := range recs {
		if !strings.HasPrefix(fStr(r, "spatial_effect_id"), "spatial.") {
			t.Fatalf("spatial_effect key %q lacks spatial. prefix", fStr(r, "spatial_effect_id"))
		}
	}
}

func TestSkillDisplacementTagConsistency(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	spatials := map[string]config.Record{}
	for _, r := range famRecs(c, "spatial_effect") {
		spatials[fStr(r, "spatial_effect_id")] = r
	}
	skillSpatial := map[string][]string{}
	for _, r := range famRecs(c, "skill_effect") {
		if p, ok2 := r.Fields["payload"]; ok2 && p.Kind == config.KindRecord &&
			p.Rec["kind"].Str == "SPATIAL" {
			if ref, ok3 := p.Rec["ref_id"]; ok3 {
				skillSpatial[fStr(r, "skill_id")] = append(skillSpatial[fStr(r, "skill_id")], ref.Str)
			}
		}
	}
	for _, r := range famRecs(c, "basic_proc") {
		if v, ok2 := r.Fields["status_effects"]; ok2 {
			for _, e := range v.Elems {
				if strings.HasPrefix(e.Str, "spatial.") {
					skillSpatial[fStr(r, "skill_id")] = append(skillSpatial[fStr(r, "skill_id")], e.Str)
				}
			}
		}
	}
	tagged, resolved := 0, 0
	for _, r := range famRecs(c, "skill") {
		id := fStr(r, "skill_id")
		var hasTag bool
		if v, ok2 := r.Fields["tags"]; ok2 {
			for _, e := range v.Elems {
				if e.Str == "DISPLACEMENT" {
					hasTag = true
				}
			}
		}
		hasResult := false
		for _, ref := range skillSpatial[id] {
			if se, ok2 := spatials[ref]; ok2 &&
				(dispResultRe.MatchString(ref) || dispResultRe.MatchString(fStr(se, "exact_resolution"))) {
				hasResult = true
			}
		}
		if hasTag != hasResult {
			t.Fatalf("skill %q DISPLACEMENT tag=%v but displacement-result=%v", id, hasTag, hasResult)
		}
		if hasTag {
			tagged++
		}
		if hasResult {
			resolved++
		}
	}
	if tagged == 0 {
		t.Fatal("no DISPLACEMENT-tagged skills in bundle")
	}
}

func TestInvalidReferenceRejection(t *testing.T) {
	base := filepath.Join("..", "..", "internal", "config", "testdata")
	if _, s := compileClean(t, goldenDir); s == nil || s.ContentRevision == "" {
		t.Fatal("golden_bundle must compile clean")
	}
	c := compileAny(t, filepath.Join(base, "invalid_cross_ref"))
	if !hasCode(c, config.DiagUnresolvedReference) {
		t.Fatal("invalid_cross_ref did not emit UNRESOLVED_REFERENCE")
	}
	c = compileAny(t, filepath.Join(base, "invalid_balance_window"))
	if !hasCode(c, config.DiagBalanceGuardrail) {
		t.Fatal("invalid_balance_window did not emit BALANCE_GUARDRAIL_FAILED")
	}
}

func TestSoulElementBindings25(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	exp, total, ok := parseSoulExpectations(c)
	if !ok {
		t.Fatal("bundle declares no soul element expectations")
	}
	if total != 25 {
		t.Fatalf("bundle declares %d souls, want 25", total)
	}
	if got := len(famRecs(c, "soul")); got != total {
		t.Fatalf("compiled %d soul records, bundle declares %d", got, total)
	}
	if len(exp) == 0 {
		t.Fatal("no per-element expectations declared")
	}
}

func TestSoulElementDoesNotInheritMonsterNone(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	elements := map[string]bool{"KIM": true, "MOC": true, "THUY": true, "HOA": true, "THO": true}
	for _, r := range famRecs(c, "soul") {
		e := fStr(r, "element")
		if e == "" || e == "NONE" || !elements[e] {
			t.Fatalf("soul %v element %q — souls declare their own element, never monster NONE", r.Key, e)
		}
	}
}

func TestAllActivePayloadConstructorsResolve(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	effects := map[string]bool{}
	for _, r := range famRecs(c, "effect_template") {
		effects[fStr(r, "effect_id")] = true
	}
	spatials := map[string]bool{}
	for _, r := range famRecs(c, "spatial_effect") {
		spatials[fStr(r, "spatial_effect_id")] = true
	}
	zones := map[string]bool{}
	for _, r := range famRecs(c, "zone_schedule") {
		zones[fStr(r, "zone_id")] = true
	}
	closed := map[string]bool{
		"DAMAGE": true, "HEAL": true, "EXECUTE": true, "STATUS": true,
		"SHIELD": true, "SPATIAL": true, "ZONE": true, "BARRIER": true,
	}
	seen := 0
	for _, r := range famRecs(c, "skill_effect") {
		p := r.Fields["payload"]
		if p.Kind != config.KindRecord {
			t.Fatalf("skill_effect %v payload is not a record", r.Key)
		}
		kind := p.Rec["kind"].Str
		if !closed[kind] {
			t.Fatalf("skill_effect %v unknown constructor %q", r.Key, kind)
		}
		switch kind {
		case "STATUS", "SHIELD":
			if !effects[p.Rec["effect_id"].Str] {
				t.Fatalf("skill_effect %v %s ref %q has no effect_template", r.Key, kind, p.Rec["effect_id"].Str)
			}
		case "SPATIAL":
			if !spatials[p.Rec["ref_id"].Str] {
				t.Fatalf("skill_effect %v SPATIAL ref %q has no spatial_effect", r.Key, p.Rec["ref_id"].Str)
			}
		case "ZONE":
			if !zones[p.Rec["ref_id"].Str] {
				t.Fatalf("skill_effect %v ZONE ref %q has no zone", r.Key, p.Rec["ref_id"].Str)
			}
		}
		seen++
	}
	if seen == 0 {
		t.Fatal("no skill_effect payloads compiled")
	}
}

func TestBarrierPayloadSignatureAndGeometry(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	actions := map[string]config.Record{}
	for _, r := range famRecs(c, "skill_action") {
		actions[fStr(r, "skill_id")] = r
	}
	n := 0
	for _, r := range famRecs(c, "skill_effect") {
		p := r.Fields["payload"]
		if p.Kind != config.KindRecord || p.Rec["kind"].Str != "BARRIER" {
			continue
		}
		n++
		if p.Rec["geometry_ref"].Str != "PRIMARY" ||
			!p.Rec["block_enemy_movement"].Bool || !p.Rec["block_projectiles"].Bool {
			t.Fatalf("BARRIER payload on %q has wrong signature", fStr(r, "skill_id"))
		}
		a, ok := actions[fStr(r, "skill_id")]
		if !ok {
			t.Fatalf("BARRIER skill %q has no skill_action", fStr(r, "skill_id"))
		}
		g := geomFields(a)
		if g["kind"].Str != "BARRIER_POSITION" {
			t.Fatalf("BARRIER skill %q geometry kind %q, want BARRIER_POSITION",
				fStr(r, "skill_id"), g["kind"].Str)
		}
	}
	if n == 0 {
		t.Fatal("no BARRIER payload in bundle")
	}
}

func TestRegisteredItemStringMutationChangesRevision(t *testing.T) {
	rev0 := revision(t, realCatalogDir)
	dir := copyBundle(t, realCatalogDir)
	mutateFile(t, dir, "item_catalog.md", func(s string) string {
		return strings.Replace(s, "Bùa May Mắn (Sơ Cấp)", "Bùa May Mắn (Sơ Cấp Sửa)", 1)
	})
	if rev1 := revision(t, dir); rev1 == rev0 {
		t.Fatal("mutating a registered item display string did not change content_revision")
	}
}

func TestItemSourceReorderAndUnicodeCanonicalEquivalence(t *testing.T) {
	rev0 := revision(t, realCatalogDir)

	// Reordering independent material rows preserves the revision.
	dir := copyBundle(t, realCatalogDir)
	mutateFile(t, dir, "item_catalog.md", func(s string) string {
		r1 := "| `item.material.lang_da.manh_dong` | Mảnh Đồng Làng | T1 | old bronze/metal fragments recovered around Làng Đa |"
		r2 := "| `item.material.u_minh.vo_cay` | Vỏ Cây U Minh | T2 | supernatural bark from fictional U Minh-region growths |"
		if !strings.Contains(s, r1) || !strings.Contains(s, r2) {
			t.Fatalf("fixture rows not found in item_catalog.md")
		}
		s = strings.Replace(s, r1, "@@ROW@@", 1)
		s = strings.Replace(s, r2, r1, 1)
		return strings.Replace(s, "@@ROW@@", r2, 1)
	})
	if rev1 := revision(t, dir); rev1 != rev0 {
		t.Fatal("reordering independent source rows changed content_revision")
	}

	// NFC vs NFD spellings of the same string normalize identically.
	dir = copyBundle(t, realCatalogDir)
	mutateFile(t, dir, "item_catalog.md", func(s string) string {
		nfc := "Mảnh Đồng Làng"
		nfd := norm.NFD.String(nfc)
		return strings.Replace(s, nfc, nfd, 1)
	})
	if rev1 := revision(t, dir); rev1 != rev0 {
		t.Fatal("Unicode-canonical-equivalent spelling changed content_revision")
	}
}

func TestAllDataOwningCatalogsDeclareCompilerSourceSchema(t *testing.T) {
	c, _ := compileClean(t, realCatalogDir)
	var missing []string
	for _, d := range Drivers {
		f := c.Catalogs[d.Catalog]
		if f == nil {
			missing = append(missing, d.Catalog+" (not loaded)")
			continue
		}
		if d.NoRegistryTable {
			continue
		}
		if f.Root.SectionAt("Compiler Source Schema") == nil &&
			len(f.Root.FindSections("Compiler Source Schema")) == 0 {
			missing = append(missing, d.Catalog)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("catalogs missing Compiler Source Schema: %s", strings.Join(missing, ", "))
	}
}

func TestGoModDeclaresXTextRequire(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	mod := string(b)
	requireRe := regexp.MustCompile(`(?m)^\s*golang\.org/x/text\s+v[0-9.]+\s*$`)
	m := requireRe.FindString(mod)
	if m == "" {
		t.Fatal("go.mod does not declare golang.org/x/text as a direct require")
	}
	if strings.Contains(m, "// indirect") {
		t.Fatal("golang.org/x/text is marked // indirect — must be a direct require")
	}
}
