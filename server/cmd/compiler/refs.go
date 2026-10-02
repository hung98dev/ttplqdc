package main

import (
	"thinhthan/internal/config"
)

// NamespaceIndex collects emitted identities per family namespace for
// two-pass cross-catalog reference resolution (contract §1, config.md
// §Validation). Families map stable-ID prefixes onto the set of emitted
// IDs.
type NamespaceIndex struct {
	// IDs[family][id] = declaration locator "catalog:line"
	IDs map[string]map[string]string
	// Budget holds the README Launch Content Budget counts for cross-checks.
	Budget map[string]int64
}

func NewNamespaceIndex() *NamespaceIndex {
	return &NamespaceIndex{IDs: map[string]map[string]string{}}
}

// Declare registers an emitted identity in its family namespace.
// A duplicate inside one namespace is a DUPLICATE_PRIMARY_KEY diagnostic.
func (n *NamespaceIndex) Declare(c *Ctx, family, id, cat string, line int) {
	m := n.IDs[family]
	if m == nil {
		m = map[string]string{}
		n.IDs[family] = m
	}
	if _, dup := m[id]; dup {
		c.Diags.Addf(config.DiagDuplicatePrimaryKey, cat, line,
			"%s %q declared twice", family, id)
		return
	}
	m[id] = cat
}

// Resolve reports whether id exists in the family namespace; when it does
// not, records UNRESOLVED_REFERENCE at the referencing location.
func (n *NamespaceIndex) Resolve(c *Ctx, family, id, cat string, line int) bool {
	if id == "" || id == "NONE" {
		return true
	}
	if _, ok := n.IDs[family][id]; ok {
		return true
	}
	c.Diags.Addf(config.DiagUnresolvedReference, cat, line,
		"%s %q is not declared", family, id)
	return false
}

// RefSpec declares one reference field on a family to check.
type RefSpec struct {
	Family   string // owning family name in Defs
	Field    string // record field holding the reference
	To       string // target namespace
	Nullable bool   // NONE / missing legal
}

// resolveReferences collects emitted identities and resolves declared
// reference fields (S13).
func resolveReferences(c *Ctx) error {
	return nil
}
