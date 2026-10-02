package config

import "fmt"

// DiagnosticCode is one of the ten canonical compiler diagnostic codes
// (content_authoring_contract.md §6).
type DiagnosticCode string

const (
	DiagCatalogFileNotFound DiagnosticCode = "CATALOG_FILE_NOT_FOUND"
	DiagTableSyntaxError    DiagnosticCode = "TABLE_SYNTAX_ERROR"
	DiagDuplicatePrimaryKey DiagnosticCode = "DUPLICATE_PRIMARY_KEY"
	DiagUnresolvedReference DiagnosticCode = "UNRESOLVED_REFERENCE"
	DiagValueOutOfBounds    DiagnosticCode = "VALUE_OUT_OF_BOUNDS"
	DiagBalanceGuardrail    DiagnosticCode = "BALANCE_GUARDRAIL_FAILED"
	DiagIntegrationCheck    DiagnosticCode = "INTEGRATION_CHECK_FAILED"
	DiagSourceSchemaMissing DiagnosticCode = "SOURCE_SCHEMA_MISSING"
	DiagUnregisteredSource  DiagnosticCode = "UNREGISTERED_SOURCE"
	DiagAmbiguousSource     DiagnosticCode = "AMBIGUOUS_SOURCE"
)

// Diagnostic is one compile diagnostic with a canonical file:line locator.
type Diagnostic struct {
	Code    DiagnosticCode
	File    string
	Line    int
	Message string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %s:%d: %s", d.Code, d.File, d.Line, d.Message)
}

// Diagnostics is the ordered diagnostic set attached to a candidate.
type Diagnostics []Diagnostic

// HasErrors reports whether any diagnostic exists. The §6 code set has no
// severity levels; every diagnostic is an error that aborts emission.
func (ds Diagnostics) HasErrors() bool {
	return len(ds) > 0
}

// Addf appends a diagnostic; line 0 is legal when the failure is not
// line-addressable (e.g. a cross-catalog check).
func (ds *Diagnostics) Addf(code DiagnosticCode, file string, line int, format string, args ...any) {
	*ds = append(*ds, Diagnostic{
		Code:    code,
		File:    file,
		Line:    line,
		Message: fmt.Sprintf(format, args...),
	})
}
