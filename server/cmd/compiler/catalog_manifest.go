package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileManifest — README.md driver (contract §3 row 24).
// The manifest contract is prose-declared (NoRegistryTable): the 24-file
// input set keyed by `catalog_file`, each required `status: LOCKED`, plus
// the Launch Content Budget counts compiled into validation_parameters for
// later cross-checks. README has no gameplay records.
func compileManifest(c *Ctx, f *File, _ *Registry) {
	// -- manifest: the §3 filenames are the input set; each must exist
	// (already enforced by LoadFile) with `status: LOCKED` on line 2.
	for _, cf := range c.Catalogs {
		if len(cf.Lines) < 2 || strings.TrimSpace(cf.Lines[1]) != "status: LOCKED" {
			c.Diags.Addf(config.DiagSourceSchemaMissing, cf.Path, 2,
				"manifest member %s lacks `status: LOCKED` line", cf.Name)
		}
	}

	// -- Launch Content Budget: `N <label>` lines inside the section's text
	// fence become validation_parameters.launch_budget[(label)] = N.
	sec := f.Root.SectionAt("Launch Content Budget")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1,
			"README has no `Launch Content Budget` section")
		return
	}
	fences := allFences(sec, "text")
	if len(fences) == 0 {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
			"Launch Content Budget has no text fence")
		return
	}
	counts := map[string]int64{}
	re := regexp.MustCompile(`^([0-9]+)\s+(.+)$`)
	for _, fb := range fences {
		for j, l := range fb.FLines {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "TOTAL") {
				continue
			}
			m := re.FindStringSubmatch(l)
			if m == nil {
				// channel-portfolio lines like `FIELD_COMBAT ... 40%`
				// are parsed below into the channel map
				parseBudgetChannel(c, f, l, fb.Line+1+j)
				continue
			}
			n, err := TypeSpec{Name: "int"}.ParseValue(m[1])
			if err != nil {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, fb.Line+1+j, "%v", err)
				continue
			}
			label := m[2]
			// strip parenthetical clause: "294 cosmetics (PLAY_PLUS_GUILD=145; ...)"
			if i := strings.Index(label, " ("); i >= 0 {
				label = label[:i]
			}
			label = strings.TrimSpace(label)
			// strip trailing provenance notes after " — "
			if i := strings.Index(label, " — "); i >= 0 {
				label = label[:i]
			}
			if _, dup := counts[label]; dup {
				c.Diags.Addf(config.DiagAmbiguousSource, f.Path, fb.Line+1+j,
					"duplicate budget label %q", label)
				continue
			}
			counts[label] = n.Int
			c.EmitParam(f.Name, "Launch Content Budget", "launch_budget",
				[]config.Value{config.VStr(label)},
				map[string]config.Value{"count": n}, fb.Line+1+j)
		}
	}
	// register for the S14 README budget cross-checks
	c.Refs.Budget = counts

	if c.Cov != nil {
		c.Cov.Add(f.Name, "Compiler Source Schema", "manifest", "catalog_file",
			config.CoverageConsumed)
	}
}

// parseBudgetChannel parses seven-channel portfolio lines of the form
// `NAME (desc)    NN%   target NNh` into the channel_EXP family.
var channelRe = regexp.MustCompile(`^([A-Z_]+)\s*(?:\([^)]*\))?\s+([0-9]+)%\s+target\s+([0-9,]+)h`)

func parseBudgetChannel(c *Ctx, f *File, line string, ln int) {
	l := strings.TrimSpace(strings.TrimRight(line, "─"))
	if l == "" || !channelRe.MatchString(l) {
		return
	}
	m := channelRe.FindStringSubmatch(l)
	name := m[1]
	pct, _ := TypeSpec{Name: "int"}.ParseValue(m[2])
	hours, err := TypeSpec{Name: "grouped_int"}.ParseValue(m[3])
	if err != nil {
		if v, e2 := (TypeSpec{Name: "int"}).ParseValue(m[3]); e2 == nil {
			hours = v
		} else {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, ln, "target hours %q", m[3])
			return
		}
	}
	c.EmitParam(f.Name, "Launch Content Budget", "channel_portfolio",
		[]config.Value{config.VStr(name)},
		map[string]config.Value{
			"share_bp":     pct,
			"target_hours": hours,
		}, ln)
}
