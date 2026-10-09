// Package effects implements the status/shield pipeline of
// status_effects.md, combat.md § Secondary Results + Shields and
// class_skill_catalog.md § Canonical Effect Templates. Skills produce
// typed requests (skills.EffectRequest); this package owns the
// instances, reapply semantics, anchored DoT ticks, shield
// absorption, stage-7 secondary math, recursion guard and the 205
// replication surface.
package effects

import "sort"

// TagSet is the status tag bitmask of status_effects.md § Tags.
type TagSet uint64

const (
	TagSlow TagSet = 1 << iota
	TagControl
	TagDisplacement
	TagDoT
	TagNegative
	TagPositive
	TagVulnerable
	TagCritMark
	TagWeaken
	TagResistShred
	TagChill
	TagMAAM
	TagSlowImmune
	TagDisplacementImmune
	TagHealReduction
	TagWard
	TagShieldMarker
	TagHardControl
	TagMovementControl
	TagStun
	TagRoot
	TagFreeze
	TagAirborne
	TagReactive
)

// controlTags are the tags an active ward-type immunity grants.
// SLOW_IMMUNE rejects SLOW instances; DISPLACEMENT_IMMUNE rejects
// DISPLACEMENT instances and forced-position results.
const controlTags = TagStun | TagRoot | TagFreeze | TagAirborne

// Kind enumerates the pipeline an effect belongs to.
type Kind uint8

const (
	// KindStatus is a timed status instance.
	KindStatus Kind = iota
	// KindDoT is a status with anchored periodic damage.
	KindDoT
	// KindShield is a shield-book entry, not a status instance.
	KindShield
)

// ReapplyMode is the re-application rule of status_effects.md L48-51.
type ReapplyMode uint8

const (
	// RefreshDuration resets expires_at = now + duration
	// unconditionally; magnitude and source snapshot take the new
	// application. Control templates (STUN/ROOT/FREEZE/AIRBORNE) use
	// expires_at = max(expires_at, now + duration) instead — a shorter
	// control never shortens a longer one (Template.Control flag).
	RefreshDuration ReapplyMode = iota
	// Stack adds one stack up to max_stacks and refreshes expiry.
	Stack
	// ReplaceStronger keeps the larger absolute magnitude; ties go to
	// the later expires_at.
	ReplaceStronger
	// Ignore drops the re-application entirely.
	Ignore
)

// Instance is one live status. Field names mirror
// status_effects.md § Instance Model.
type Instance struct {
	EffectID string
	SourceID uint64
	TargetID uint64

	Stacks int32

	// StartedTick is the creation tick (creation seq ordering input).
	StartedTick uint64
	// ExpiresAtTick is the residual/display expiry.
	ExpiresAtTick uint64
	// AnchorTick is the first-application tick for DoTs; refresh
	// never moves it.
	AnchorTick uint64
	// DamageExpiresAtTick is the DoT damage deadline; residual
	// effects may keep the marker instance alive past it.
	DamageExpiresAtTick uint64
	// AttackSnapshot is the source ATTACK captured at (re)application.
	AttackSnapshot int64

	Kind        Kind
	Tags        TagSet
	Dispellable bool
	ReapplyMode ReapplyMode

	// Magnitude overrides from the applying request (0 = template
	// value): stat-driven HEAL_REDUCTION and passive-scaled buffs.
	MagHealingReceived float64
	MagAttackBP        int32
	MagDefenseBP       int32

	// seq is the partition-wide creation counter — the third
	// determinant of tick order and the replication eviction order.
	seq  uint64
	tmpl *Template
}

// active reports whether the instance still exists at tick.
func (i *Instance) active(tick uint64) bool { return tick < i.ExpiresAtTick }

// Seq exposes the creation counter for deterministic ordering.
func (i *Instance) Seq() uint64 { return i.seq }

// TemplateDef exposes the resolved template for stat aggregation.
func (i *Instance) TemplateDef() *Template { return i.tmpl }

// InstanceKey is the book lookup key. TARGET-keyed templates carry
// SourceID = 0 (one instance per (target, effect_id); the latest
// source wins). SOURCE-keyed templates carry the applier id.
type InstanceKey struct {
	EffectID string
	SourceID uint64
}

// Book is the per-target instance store.
type Book struct {
	TargetID uint64
	inst     map[InstanceKey]*Instance
}

func newBook(target uint64) *Book {
	return &Book{TargetID: target, inst: make(map[InstanceKey]*Instance)}
}

// Get returns the live instance for key, if present.
func (b *Book) Get(k InstanceKey) *Instance { return b.inst[k] }

// HasTag reports whether any active instance carries tag.
func (b *Book) HasTag(tag TagSet, tick uint64) bool {
	for _, i := range b.inst {
		if i.active(tick) && i.Tags&tag != 0 {
			return true
		}
	}
	return false
}

// immune reports whether an active instance confers one of the
// immunity tags (granted through Template.ImmunityTags on POSITIVE
// wards).
func (b *Book) immune(tick uint64) TagSet {
	var out TagSet
	for _, i := range b.inst {
		if i.active(tick) {
			out |= i.tmpl.ImmunityTags
		}
	}
	return out
}

// ordered returns instances in the determinism order of
// status_effects.md L209-214: source_id lex, then effect_id lex,
// then creation seq.
func (b *Book) ordered() []*Instance {
	out := make([]*Instance, 0, len(b.inst))
	for _, i := range b.inst {
		out = append(out, i)
	}
	sort.Slice(out, func(a, c int) bool {
		x, y := out[a], out[c]
		if x.SourceID != y.SourceID {
			return x.SourceID < y.SourceID
		}
		if x.EffectID != y.EffectID {
			return x.EffectID < y.EffectID
		}
		return x.seq < y.seq
	})
	return out
}

// remove deletes the instance (expiry, dispel, death/map cleanup,
// CHILL consumption).
func (b *Book) remove(i *Instance) {
	for k, v := range b.inst {
		if v == i {
			delete(b.inst, k)
			return
		}
	}
}
