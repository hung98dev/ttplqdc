package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"thinhthan/internal/config"
)

// FamilyStore indexes emitted definition families by name.
type FamilyStore struct {
	Families map[string]*config.Family
}

// Get returns (creating) the named family. Families are declared with their
// composite key columns on first use.
func (s *FamilyStore) Get(name string, keyCols ...string) *config.Family {
	if s.Families == nil {
		s.Families = map[string]*config.Family{}
	}
	f, ok := s.Families[name]
	if !ok {
		f = config.NewFamily(name, keyCols...)
		s.Families[name] = f
	}
	return f
}

// Ctx is the compile-time context drivers share.
type Ctx struct {
	Defs     *FamilyStore
	Params   *FamilyStore // validation_parameters families
	Geom     *FamilyStore // geometry family
	Diags    *config.Diagnostics
	Cov      *config.CoverageReport
	Refs     *NamespaceIndex
	Rules    map[string]int
	Catalogs map[string]*File
	Dir      string

	// Data carries driver-collected state into the expansion/reference/
	// validation passes (rosters, profile tables, portal edges, ...).
	Data map[string]any

	// Warnings are emitted into the report alongside errors; the contract
	// requires zero of both for PASS, so drivers use them only for
	// documented count anomalies that a later stage resolves.
	Warnings *config.Diagnostics
}

// Emit inserts a record and records its coverage; duplicate keys produce
// DUPLICATE_PRIMARY_KEY with both locations.
func (c *Ctx) Emit(cat string, section string, fam string, key []config.Value, fields map[string]config.Value, line int) {
	f := c.Defs.Get(fam)
	if dup, old := f.Put(key, fields); dup {
		c.Diags.Addf(config.DiagDuplicatePrimaryKey, cat, line,
			"family %s key %s duplicates earlier row", fam, config.KeyString(key))
		_ = old
		return
	}
	if c.Cov != nil {
		c.Cov.Add(cat, section, fam, config.KeyString(key), config.CoverageEmittedField)
	}
}

// EmitParam emits into validation_parameters families.
func (c *Ctx) EmitParam(cat, section, fam string, key []config.Value, fields map[string]config.Value, line int) {
	f := c.Params.Get(fam)
	if dup, _ := f.Put(key, fields); dup {
		c.Diags.Addf(config.DiagDuplicatePrimaryKey, cat, line,
			"params family %s key %s duplicates earlier row", fam, config.KeyString(key))
		return
	}
	if c.Cov != nil {
		c.Cov.Add(cat, section, "validation_parameters."+fam, config.KeyString(key), config.CoverageEmittedField)
	}
}

// EmitGeom emits into the geometry family.
func (c *Ctx) EmitGeom(cat, section string, key []config.Value, fields map[string]config.Value, line int) {
	f := c.Geom.Get("spaces", "space_id")
	if dup, _ := f.Put(key, fields); dup {
		c.Diags.Addf(config.DiagDuplicatePrimaryKey, cat, line,
			"geometry spaces key %s duplicates earlier row", config.KeyString(key))
		return
	}
	if c.Cov != nil {
		c.Cov.Add(cat, section, "geometry.spaces", config.KeyString(key), config.CoverageEmittedField)
	}
}

// Driver compiles one catalog file.
type Driver struct {
	Catalog string
	// Compile consumes the file's registered sections and emits records.
	Compile func(c *Ctx, f *File, r *Registry)
	// NoRegistryTable marks README (manifest contract is prose-declared).
	NoRegistryTable bool
}

// Drivers in contract §3 manifest order; each Compile is assigned as its
// driver lands (a nil Compile is a SOURCE_SCHEMA_MISSING diagnostic).
var Drivers = []Driver{
	{Catalog: "monster_catalog.md", Compile: compileMonster},
	{Catalog: "boss_catalog.md", Compile: compileBoss},
	{Catalog: "class_skill_catalog.md"},
	{Catalog: "equipment_catalog.md"},
	{Catalog: "item_catalog.md"},
	{Catalog: "drop_tables.md"},
	{Catalog: "crafting_catalog.md"},
	{Catalog: "npc_shop_catalog.md", Compile: compileNPC},
	{Catalog: "quest_catalog.md"},
	{Catalog: "dungeon_catalog.md"},
	{Catalog: "world_route_catalog.md", Compile: compileWorldRoute},
	{Catalog: "map_spawn_catalog.md"},
	{Catalog: "atlas_catalog.md"},
	{Catalog: "cosmetic_catalog.md"},
	{Catalog: "soul_catalog.md"},
	{Catalog: "build_catalog.md"},
	{Catalog: "spirit_beast_catalog.md", Compile: compileBeast},
	{Catalog: "economy_catalog.md"},
	{Catalog: "world_event_catalog.md"},
	{Catalog: "encounter_catalog.md", Compile: compileEncounter},
	{Catalog: "progression_route.md", Compile: compileProgressionRoute},
	{Catalog: "balance_validation.md"},
	{Catalog: "integration_validation.md"},
	{Catalog: "README.md", Compile: compileManifest, NoRegistryTable: true},
}

// runPipeline loads all manifest catalogs, parses registries, runs drivers,
// resolves references, runs validation, emits snapshot.
func runPipeline(c *Ctx) error {
	// load files
	for _, d := range Drivers {
		path := filepath.Join(c.Dir, d.Catalog)
		f, err := LoadFile(path, c.Diags)
		if err != nil {
			continue // diagnostic already recorded
		}
		c.Catalogs[d.Catalog] = f
	}
	if c.Diags.HasErrors() {
		return fmt.Errorf("load failed")
	}

	// registries
	regs := map[string]*Registry{}
	for _, d := range Drivers {
		f := c.Catalogs[d.Catalog]
		if d.NoRegistryTable {
			regs[d.Catalog] = &Registry{Catalog: f.Name}
			continue
		}
		r, err := LoadRegistry(f)
		if err != nil {
			continue
		}
		regs[d.Catalog] = r
	}

	// drivers in manifest order (README last: budget cross-check needs
	// emitted counts — run manifest structural pass first, budget later)
	for _, d := range Drivers {
		if d.Compile == nil {
			c.Diags.Addf(config.DiagSourceSchemaMissing, c.Dir, 0,
				"no driver registered for %s", d.Catalog)
			continue
		}
		d.Compile(c, c.Catalogs[d.Catalog], regs[d.Catalog])
	}
	if c.Diags.HasErrors() {
		return fmt.Errorf("driver compile failed")
	}

	// expansions (S12), references (S13), validation (S14)
	runExpansions(c)
	if c.Diags.HasErrors() {
		return fmt.Errorf("expansion failed")
	}
	if err := resolveReferences(c); err != nil {
		return err
	}
	runValidation(c)
	return nil
}

// ---------------------------------------------------------------------
// shared section/table resolution helpers
// ---------------------------------------------------------------------

// resolveSections resolves a binding's section path against f. Registry
// locators use exact heading ancestry; when the path is a loose prefix of
// a real heading (e.g. `Seven-Channel` for `Seven-Channel EXP Source
// Portfolio`), prefix matching resolves it.
func resolveSections(f *File, path string) []*Section {
	if sec := f.Root.SectionAt(path); sec != nil {
		return []*Section{sec}
	}
	last := path
	if i := strings.LastIndex(path, ">"); i >= 0 {
		last = strings.TrimSpace(path[i+1:])
	}
	// `#`/`##` heading marks in registry paths are decoration
	last = strings.TrimSpace(strings.TrimLeft(last, "#"))
	if secs := f.Root.FindSections(last); len(secs) > 0 {
		return secs
	}
	// prefix fallback over every heading (heading backticks are decoration)
	var out []*Section
	clean := func(s string) string { return strings.ReplaceAll(s, "`", "") }
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, ch := range s.Children {
			ct := clean(ch.Title)
			if strings.HasPrefix(ct, last) || strings.HasPrefix(last, ct) {
				out = append(out, ch)
			}
			walk(ch)
		}
	}
	walk(f.Root)
	return out
}

func headerSig(b *Block) string {
	parts := make([]string, len(b.Headers))
	for i, h := range b.Headers {
		parts[i] = strings.TrimSpace(h)
	}
	return strings.Join(parts, ",")
}

// allTables returns every table under sec (descendants included).
func allTables(sec *Section) []*Block {
	var out []*Block
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, b := range s.Content {
			if b.Kind == BlockTable {
				out = append(out, b)
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(sec)
	return out
}

// allFences returns every fence under sec (descendants included).
func allFences(sec *Section, lang string) []*Block {
	var out []*Block
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, b := range s.Content {
			if b.Kind == BlockFence && (lang == "" || b.Lang == lang) {
				out = append(out, b)
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(sec)
	return out
}
