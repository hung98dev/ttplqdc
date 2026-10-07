package progression

import (
	"errors"
	"fmt"
)

// Reject is the closed set of client-visible rejection kinds for the three
// progression ops (messages.md 511–514). The durable and edge layers map
// each kind onto the wire ErrorCode; sim keeps the domain name only.
type Reject uint8

const (
	RejectSkillNotLearned Reject = iota
	RejectSkillMaxLevel
	RejectSkillPointsInsufficient
	RejectStateConflict
	RejectPotentialPointsInsufficient
	RejectPotentialCapExceeded
	RejectInsufficientCurrency
	RejectInvalidKind
	RejectInvalidDelta
)

var rejectNames = map[Reject]string{
	RejectSkillNotLearned:             "SKILL_NOT_LEARNED",
	RejectSkillMaxLevel:               "SKILL_MAX_LEVEL",
	RejectSkillPointsInsufficient:     "SKILL_POINTS_INSUFFICIENT",
	RejectStateConflict:               "STATE_CONFLICT",
	RejectPotentialPointsInsufficient: "POTENTIAL_POINTS_INSUFFICIENT",
	RejectPotentialCapExceeded:        "POTENTIAL_CAP_EXCEEDED",
	RejectInsufficientCurrency:        "INSUFFICIENT_CURRENCY",
	RejectInvalidKind:                 "INVALID_KIND",
	RejectInvalidDelta:                "INVALID_DELTA",
}

func (r Reject) String() string { return rejectNames[r] }

// OpError carries a Reject so callers can distinguish a rule rejection
// from infrastructure errors without string matching.
type OpError struct{ Code Reject }

func (e *OpError) Error() string { return "progression: rejected " + e.Code.String() }

// RejectOf extracts the rejection kind from an error, or false when the
// error is not a rule rejection.
func RejectOf(err error) (Reject, bool) {
	var oe *OpError
	if errors.As(err, &oe) {
		return oe.Code, true
	}
	return 0, false
}

func reject(code Reject) error { return &OpError{Code: code} }

// PlayerProgress is the in-memory progression aggregate the ops mutate:
// the 515 projection state plus the learned-skill ledger. Fields mirror
// the durable columns exactly so a SQL row maps 1:1.
type PlayerProgress struct {
	ClassID                string
	Level                  int32
	Exp                    int32 // absolute cumulative, ×100 scale, ≤ ExpCap
	UnspentSkillPoints     int32
	UnspentPotentialPoints int32
	PotentialAllocated     PotentialDelta
	Skills                 map[string]int32 // learned skill_id -> level (1..max)
}

// EarnedPotentialTotal is the derivable lifetime potential total.
func (p PlayerProgress) EarnedPotentialTotal() int32 {
	return EarnedTotal(p.UnspentPotentialPoints, p.PotentialAllocated)
}

// SpentSkillPoints is the sum of points invested above level 1 — the
// refund basis for a skill respec.
func (p PlayerProgress) SpentSkillPoints() int32 {
	var spent int32
	for _, lvl := range p.Skills {
		if lvl > 1 {
			spent += lvl - 1
		}
	}
	return spent
}

// ApplySkillUpgrade applies C2S_SKILL_UPGRADE semantics: +1 level to a
// learned skill for 1 unspent skill point (ADR-0060). expectedLevel is the
// client-echoed current level; a mismatch is STATE_CONFLICT. Check order:
// learned → conflict → max → insufficient.
func (p PlayerProgress) ApplySkillUpgrade(skillID string, expectedLevel uint32) (PlayerProgress, error) {
	lvl, ok := p.Skills[skillID]
	if !ok {
		return p, reject(RejectSkillNotLearned)
	}
	def, ok := Lookup(skillID)
	if !ok {
		return p, reject(RejectSkillNotLearned)
	}
	if expectedLevel != 0 && int32(expectedLevel) != lvl {
		return p, reject(RejectStateConflict)
	}
	if lvl >= def.Kind.MaxLevel() {
		return p, reject(RejectSkillMaxLevel)
	}
	if p.UnspentSkillPoints < 1 {
		return p, reject(RejectSkillPointsInsufficient)
	}
	p.Skills = cloneSkills(p.Skills)
	p.Skills[skillID] = lvl + 1
	p.UnspentSkillPoints--
	return p, nil
}

// ApplyAllocate applies C2S_POTENTIAL_ALLOCATE all-or-nothing semantics:
// every stat in deltas lands or the whole op rejects. Sum must be ≥1 and
// ≤ unspent; each resulting stat must stay within PerStatCap(earned).
func (p PlayerProgress) ApplyAllocate(d PotentialDelta) (PlayerProgress, error) {
	if d.Str < 0 || d.Vit < 0 || d.Int < 0 || d.Agi < 0 {
		return p, reject(RejectInvalidDelta)
	}
	sum := d.Sum()
	if sum < 1 {
		return p, reject(RejectInvalidDelta)
	}
	if sum > p.UnspentPotentialPoints {
		return p, reject(RejectPotentialPointsInsufficient)
	}
	next := p.PotentialAllocated.Plus(d)
	cap := PerStatCap(p.EarnedPotentialTotal())
	for _, id := range PotentialIDs {
		if next.At(id) > cap {
			return p, reject(RejectPotentialCapExceeded)
		}
	}
	p.PotentialAllocated = next
	p.UnspentPotentialPoints -= sum
	return p, nil
}

// RespecOutcome reports a respec application: the price charged and the
// point totals refunded. Charge and refund are one atomic application —
// a balance shortfall rejects with no change.
type RespecOutcome struct {
	Price             int64
	SkillRefunded     int32
	PotentialRefunded int32
}

// ApplyRespec applies C2S_RESPEC semantics (progression.md § Respec):
// kind SKILL sets every learned skill to level 1 (skills stay learned,
// loadout kept) and refunds the spent points; kind POTENTIAL zeroes all
// allocations and refunds them. The caller commits the price debit in the
// same transaction — balance validation happens here so a shortfall
// rejects before any mutation (INSUFFICIENT_CURRENCY).
func (p PlayerProgress) ApplyRespec(kind RespecKind, balance int64) (PlayerProgress, RespecOutcome, error) {
	out := RespecOutcome{Price: RespecPrice(p.Level)}
	if balance < out.Price {
		return p, RespecOutcome{}, reject(RejectInsufficientCurrency)
	}
	switch kind {
	case RespecKindSkill:
		out.SkillRefunded = p.SpentSkillPoints()
		p.Skills = cloneSkills(p.Skills)
		for id := range p.Skills {
			p.Skills[id] = 1
		}
		p.UnspentSkillPoints += out.SkillRefunded
	case RespecKindPotential:
		out.PotentialRefunded = p.PotentialAllocated.Sum()
		p.PotentialAllocated = PotentialDelta{}
		p.UnspentPotentialPoints += out.PotentialRefunded
	default:
		return p, RespecOutcome{}, reject(RejectInvalidKind)
	}
	return p, out, nil
}

// Clone copies p including its skill map.
func (p PlayerProgress) Clone() PlayerProgress {
	out := p
	out.Skills = cloneSkills(p.Skills)
	return out
}

func cloneSkills(in map[string]int32) map[string]int32 {
	out := make(map[string]int32, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Validate checks the invariants a stored row must satisfy; used by tests
// and the durable projection as a defensive check.
func (p PlayerProgress) Validate() error {
	if p.Level < StartLevel || p.Level > MaxLevel {
		return fmt.Errorf("progression: level %d out of range", p.Level)
	}
	if int64(p.Exp) < 0 || int64(p.Exp) > ExpCap {
		return fmt.Errorf("progression: exp %d out of range", p.Exp)
	}
	if p.UnspentSkillPoints < 0 || p.UnspentPotentialPoints < 0 {
		return fmt.Errorf("progression: negative unspent points")
	}
	for id, lvl := range p.Skills {
		def, ok := Lookup(id)
		if !ok {
			return fmt.Errorf("progression: unknown skill %q", id)
		}
		if lvl < 1 || lvl > def.Kind.MaxLevel() {
			return fmt.Errorf("progression: skill %q level %d out of range", id, lvl)
		}
	}
	return nil
}
