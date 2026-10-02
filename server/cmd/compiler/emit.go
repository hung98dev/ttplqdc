package main

import (
	"fmt"

	"thinhthan/internal/config"
)

// emitSnapshot assembles the CandidateSnapshot from the compile context:
// definition families, validation_parameters, geometry, rule_versions,
// diagnostics + coverage, then computes the content revision over the
// canonical §5 payload.
func emitSnapshot(c *Ctx) (*config.CandidateSnapshot, error) {
	params := paramsFamily(c)
	geom := c.Geom.Get("spaces", "space_id")
	snap := &config.CandidateSnapshot{
		AuthoringSchemaVersion: config.AuthoringSchemaVersion,
		ContentSchemaVersion:   config.ContentSchemaVersion,
		RuleVersions:           ruleVersions(c),
		Definitions:            c.Defs.Families,
		ValidationParameters:   params,
		Geometry:               geom,
		Diagnostics:            *c.Diags,
		Coverage:               c.Cov,
	}
	payload, err := config.CanonicalBytes(config.CanonicalPayload(snap))
	if err != nil {
		return nil, fmt.Errorf("canonical payload: %w", err)
	}
	rev, err := config.ComputeContentRevision(payload)
	if err != nil {
		return nil, err
	}
	snap.ContentRevision = rev
	return snap, nil
}

// paramsFamily folds the emitted validation-parameter families into the
// snapshot's single `validation_parameters` family: each sub-family becomes
// a record keyed by its family name whose `records` field holds the sorted
// record list.
func paramsFamily(c *Ctx) *config.Family {
	out := config.NewFamily("validation_parameters", "family")
	for _, name := range sortedFamNames(c.Params.Families) {
		f := c.Params.Families[name]
		recs := make([]config.Value, 0, len(f.Records))
		for _, k := range f.SortedKeys() {
			rec := f.Records[config.KeyString(k)]
			recs = append(recs, config.VRec(map[string]config.Value{
				"fields": config.VRec(rec.Fields),
				"key":    config.VList(rec.Key...),
			}))
		}
		out.Put([]config.Value{config.VStr(name)}, map[string]config.Value{
			"family":      config.VStr(name),
			"key_columns": config.VStrs(f.KeyColumns...),
			"records":     config.VList(recs...),
		})
	}
	return out
}

// ruleVersions folds the emitted rule_versions params family into the
// snapshot's integer map (name -> version).
func ruleVersions(c *Ctx) map[string]int {
	out := map[string]int{}
	if f := c.Params.Families["rule_versions"]; f != nil {
		for k, rec := range f.Records {
			name := k
			if len(rec.Key) > 0 {
				name = rec.Key[0].Str
			}
			if v, ok := rec.Fields["version"]; ok {
				out[name] = int(v.Int)
			} else {
				out[name] = 1
			}
		}
	}
	for k, v := range c.Rules {
		out[k] = v
	}
	return out
}

func sortedFamNames(m map[string]*config.Family) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// buildReport fills the content_compile_report.json contract: revision,
// catalogs evaluated (in manifest order), entity counts, diagnostics.
func buildReport(c *Ctx, snap *config.CandidateSnapshot) *config.CompileReport {
	cats := []string{}
	for _, d := range Drivers {
		if _, ok := c.Catalogs[d.Catalog]; ok {
			cats = append(cats, d.Catalog)
		}
	}
	counts := map[string]int{}
	if snap != nil {
		for name, fam := range snap.Definitions {
			counts["definitions."+name] = len(fam.Records)
		}
		counts["geometry.spaces"] = len(snap.Geometry.Records)
	}
	rep := &config.CompileReport{
		CatalogsEvaluated: cats,
		EntityCounts:      counts,
	}
	if snap != nil {
		rep.ContentRevisionHash = snap.ContentRevision
	}
	diags := append(config.Diagnostics{}, (*c.Diags)...)
	diags = append(diags, (*c.Warnings)...)
	rep.ValidationDiagnostics = diags
	return rep
}
