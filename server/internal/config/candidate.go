package config

// CandidateSnapshot is the compiler's output contract to the activation
// pipeline (IMP-004). Diagnostics and Coverage are compile metadata excluded
// from the canonical payload and therefore from ContentRevision.
type CandidateSnapshot struct {
	AuthoringSchemaVersion int
	ContentSchemaVersion   int
	RuleVersions           map[string]int
	Definitions            map[string]*Family
	ValidationParameters   *Family
	Geometry               *Family
	Diagnostics            Diagnostics
	Coverage               *CoverageReport
	ContentRevision        string
}
