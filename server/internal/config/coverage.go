package config

// CoverageKind classifies one coverage entry: a consumed source section, an
// emitted output field's source, or a named finite-rule derivation.
type CoverageKind string

const (
	CoverageConsumed     CoverageKind = "consumed"
	CoverageEmittedField CoverageKind = "emitted-field"
	CoverageDerivedRule  CoverageKind = "derived-rule"
)

// CoverageEntry maps one emitted field or consumed section back to its
// named source locator (contract §1: every runtime field requires a named
// typed source/default/derivation; implementation-only constants forbidden).
type CoverageEntry struct {
	Catalog       string
	SourceSection string
	OutputFamily  string
	Key           string
	Kind          CoverageKind
}

// CoverageReport lists every consumed section and every emitted field's
// source or named derivation.
type CoverageReport struct {
	Entries []CoverageEntry
}

// Add records one entry.
func (r *CoverageReport) Add(catalog, section, family, key string, kind CoverageKind) {
	r.Entries = append(r.Entries, CoverageEntry{
		Catalog:       catalog,
		SourceSection: section,
		OutputFamily:  family,
		Key:           key,
		Kind:          kind,
	})
}
