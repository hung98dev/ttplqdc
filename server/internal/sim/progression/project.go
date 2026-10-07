package progression

// Project builds the S2C_PROGRESSION_STATE (515) projection shape from the
// domain value: the learned-skill list in catalog document order, the
// derived earned totals, and the default skill_loadout (skills.md § Active
// and Basic Loadout — basic_1 plus learned actives in document order,
// padded to five slots; a persisted SKILL_SET owned by IMP-012 wins when
// one exists, so the builder accepts an override loadout).

// SkillRow is one entry of the 515 skills list.
type SkillRow struct {
	SkillID string
	Level   int32
}

// Loadout is the 515 skill_loadout shape.
type Loadout struct {
	BasicSkillID string
	ActiveSlots  [5]string
}

// StateView is the in-memory shape the 515 builder serializes.
type StateView struct {
	Level                  int32
	CurrentExp             int32
	UnspentSkillPoints     int32
	UnspentPotentialPoints int32
	PotentialAllocated     PotentialDelta
	PotentialEarnedTotal   int32
	Skills                 []SkillRow
	Loadout                Loadout
	ProgressionRevision    uint64
}

// Project renders the authoritative projection for a value. Skills are
// emitted in catalog document order filtered to learned rows; loadout is
// the default derivation unless loadout is non-nil (a persisted
// SKILL_SET).
func Project(p PlayerProgress, revision uint64, loadout *Loadout) StateView {
	defs := ClassSkills(p.ClassID)
	rows := make([]SkillRow, 0, len(defs))
	for _, d := range defs {
		if lvl, ok := p.Skills[d.ID]; ok {
			rows = append(rows, SkillRow{SkillID: d.ID, Level: lvl})
		}
	}
	lo := DefaultLoadout(p)
	if loadout != nil {
		lo = *loadout
	}
	return StateView{
		Level:                  p.Level,
		CurrentExp:             p.Exp,
		UnspentSkillPoints:     p.UnspentSkillPoints,
		UnspentPotentialPoints: p.UnspentPotentialPoints,
		PotentialAllocated:     p.PotentialAllocated,
		PotentialEarnedTotal:   p.EarnedPotentialTotal(),
		Skills:                 rows,
		Loadout:                lo,
		ProgressionRevision:    revision,
	}
}

// DefaultLoadout derives the skills.md default: basic_skill_id is the
// class's basic_1; active_slots carry the learned actives in catalog
// document order, remaining slots empty.
func DefaultLoadout(p PlayerProgress) Loadout {
	lo := Loadout{BasicSkillID: BasicOne(p.ClassID)}
	slot := 0
	for _, d := range ClassSkills(p.ClassID) {
		if slot == len(lo.ActiveSlots) {
			break
		}
		if d.Kind != SkillKindActive {
			continue
		}
		if _, ok := p.Skills[d.ID]; ok {
			lo.ActiveSlots[slot] = d.ID
			slot++
		}
	}
	return lo
}
