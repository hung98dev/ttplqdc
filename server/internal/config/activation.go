package config

// Activation gate (IMP-004, config.md § Atomic Activation / Rollback /
// Schema-Coupled Revisions). One Gate validates a compiled
// CandidateSnapshot as a dependency graph, then installs it atomically as
// the immutable active snapshot; it owns the revision lifecycle
// (active/previous/pinned retention, rollback, readiness refusal).

// PinKind identifies the class of runtime reference that pins an exact
// compiled revision (config.md § Atomic Activation, save_rules.md,
// bosses.md).
type PinKind string

const (
	// PinCommand is a queued, in-flight or journalled command holding the
	// revision it started with (save_rules.md generic outcomes).
	PinCommand PinKind = "command"
	// PinBossReward is an unsettled defeated-copy boss reward slot pinning
	// its reward_content_revision through all slot fallbacks (bosses.md).
	PinBossReward PinKind = "boss_reward_slot"
)

// PinRef is one external reference retaining a revision until its
// canonical terminal disposition.
type PinRef struct {
	Kind     PinKind
	ID       string // unique reference id (command key / slot key)
	Revision string // pinned content revision
}

// ActivationMeta carries revision metadata that rides the deployment
// pipeline rather than the canonical payload (config.md § Schema-Coupled
// Content Revisions: the tag sits beside schema_version/content_revision/
// created_at in revision metadata).
type ActivationMeta struct {
	// SchemaCoupled marks a revision that changes a formula whose output is
	// persisted to character rows (EXP threshold tables, level cap, ...).
	// A revision meeting the criteria without the tag must not activate;
	// a tagged active revision refuses binary rollback.
	SchemaCoupled bool
}

// revisionEntry is one retained snapshot plus its activation metadata.
type revisionEntry struct {
	snap *CandidateSnapshot
	meta ActivationMeta
}

// Gate is the activation store: retained revisions keyed by content
// revision, the active/previous pair, and every live pin reference.
type Gate struct {
	revisions map[string]*revisionEntry
	pins      map[string]PinRef // ref ID -> pin
	active    string
	previous  string
}

// NewGate returns an empty activation gate.
func NewGate() *Gate {
	return &Gate{
		revisions: map[string]*revisionEntry{},
		pins:      map[string]PinRef{},
	}
}

// Active returns the active snapshot. The hand-out is read-only: callers
// must not mutate the returned snapshot or any nested structure.
func (g *Gate) Active() *CandidateSnapshot {
	if e := g.revisions[g.active]; e != nil {
		return e.snap
	}
	return nil
}

// ActiveRevision returns the active content revision ("" when none).
func (g *Gate) ActiveRevision() string { return g.active }

// Activate validates the whole candidate and installs it atomically. On a
// clean check suite the revision is stored, becomes active and the prior
// active becomes previous; on any diagnostic the previous validated
// revision remains active unchanged (revision_activation = REJECTED).
func (g *Gate) Activate(c *CandidateSnapshot, m ActivationMeta) Diagnostics {
	d := runActivationChecks(c, g.revisions[g.active], m)
	if d.HasErrors() {
		return d
	}
	if g.revisions[c.ContentRevision] == nil {
		g.revisions[c.ContentRevision] = &revisionEntry{}
	}
	e := g.revisions[c.ContentRevision]
	e.snap = c
	e.meta = m
	if c.ContentRevision != g.active {
		g.previous = g.active
		g.active = c.ContentRevision
	}
	g.gcLocked()
	return nil
}

// Rollback swaps active and previous. It never rewrites runtime player
// state; it refuses when no previous revision exists or when the active
// revision is schema-coupled (binary rollback cannot undo committed
// character rows — config.md § Schema-Coupled Content Revisions).
func (g *Gate) Rollback() Diagnostics {
	var d Diagnostics
	act := g.revisions[g.active]
	if act == nil {
		d.Addf(DiagIntegrationCheck, "", 0, "rollback: no active revision")
		return d
	}
	if act.meta.SchemaCoupled {
		d.Addf(DiagIntegrationCheck, "", 0,
			"rollback: active revision %s is schema-coupled; binary rollback forbidden",
			g.active)
		return d
	}
	if g.previous == "" {
		d.Addf(DiagIntegrationCheck, "", 0,
			"rollback: no previous revision to restore")
		return d
	}
	g.active, g.previous = g.previous, g.active
	g.gcLocked()
	return nil
}

// Pin records one runtime reference pinning rev. A missing or
// schema-incompatible pinned revision is recorded as a live pin (it must
// surface in Ready until terminal disposition) and reported as a
// not-ready diagnostic here as well.
func (g *Gate) Pin(rev string, r PinRef) Diagnostics {
	var d Diagnostics
	r.Revision = rev
	g.pins[r.ID] = r
	g.pinCheck(rev, &d)
	g.gcLocked()
	return d
}

// Dispose marks a pin reference terminally disposed. Once the last
// reference to a non-active, non-previous revision is disposed the
// revision is released from the retention set.
func (g *Gate) Dispose(refID string) {
	delete(g.pins, refID)
	g.gcLocked()
}

// Ready refuses readiness when any pinned revision is missing from the
// store or schema-incompatible (config.md: a missing/incompatible
// referenced revision refuses readiness).
func (g *Gate) Ready() Diagnostics {
	var d Diagnostics
	for _, r := range g.pins {
		g.pinCheck(r.Revision, &d)
	}
	return d
}

// pinCheck reports a not-ready diagnostic when the pinned revision is
// absent from the store or carries incompatible schema versions.
func (g *Gate) pinCheck(rev string, d *Diagnostics) {
	e := g.revisions[rev]
	if e == nil {
		d.Addf(DiagIntegrationCheck, "", 0,
			"pinned revision %s missing from store", rev)
		return
	}
	snap := e.snap
	if snap == nil || snap.AuthoringSchemaVersion != AuthoringSchemaVersion ||
		snap.ContentSchemaVersion != ContentSchemaVersion {
		d.Addf(DiagIntegrationCheck, "", 0,
			"pinned revision %s is schema-incompatible", rev)
	}
}

// pinned reports whether any live pin references rev.
func (g *Gate) pinned(rev string) bool {
	for _, r := range g.pins {
		if r.Revision == rev {
			return true
		}
	}
	return false
}

// gcLocked releases revisions outside the retention set
// {active, previous, every pinned revision} (config.md: release only
// after all references reach canonical terminal disposition).
func (g *Gate) gcLocked() {
	for rev := range g.revisions {
		if rev == g.active || rev == g.previous || g.pinned(rev) {
			continue
		}
		delete(g.revisions, rev)
	}
}
