package progression

import (
	progressiond "thinhthan/internal/durable/progression"
	protocolv1 "thinhthan/internal/protocol/v1"
	progressions "thinhthan/internal/sim/progression"
)

// ProgressionPush builds the S2C_PROGRESSION_STATE (515) payload from
// the durable aggregate: skills in catalog document order, the derived
// potential_earned_total, and the default skill_loadout (basic_1 plus
// learned actives in document order — a persisted SKILL_SET owned by
// IMP-012 overrides the derived loadout when one exists).
//
// The domain ordering/derivation lives in sim/progression, which the
// durable layer may not import — so the wire projection is assembled at
// the edge seam where both directions are legal.
func ProgressionPush(p progressiond.Progression) *protocolv1.S2CProgressionState {
	v := progressions.Project(progressions.PlayerProgress{
		ClassID:                p.ClassID,
		Level:                  p.Level,
		Exp:                    p.CurrentExp,
		UnspentSkillPoints:     p.UnspentSkillPoints,
		UnspentPotentialPoints: p.UnspentPotentialPoints,
		PotentialAllocated: progressions.PotentialDelta{
			Str: p.Allocated[0],
			Vit: p.Allocated[1],
			Int: p.Allocated[2],
			Agi: p.Allocated[3],
		},
		Skills: p.Skills,
	}, p.Revision, nil)

	skills := make([]*protocolv1.SkillView, 0, len(v.Skills))
	for _, row := range v.Skills {
		skills = append(skills, &protocolv1.SkillView{
			SkillId: row.SkillID,
			Level:   uint32(row.Level),
		})
	}
	return &protocolv1.S2CProgressionState{
		Level:                  uint32(v.Level),
		CurrentExp:             int64(v.CurrentExp),
		UnspentSkillPoints:     uint32(v.UnspentSkillPoints),
		UnspentPotentialPoints: uint32(v.UnspentPotentialPoints),
		PotentialAllocated: &protocolv1.PotentialDelta{
			Str: uint32(v.PotentialAllocated.Str),
			Vit: uint32(v.PotentialAllocated.Vit),
			Int: uint32(v.PotentialAllocated.Int),
			Agi: uint32(v.PotentialAllocated.Agi),
		},
		PotentialEarnedTotal: uint32(v.PotentialEarnedTotal),
		Skills:               skills,
		SkillLoadout: &protocolv1.SkillLoadoutView{
			BasicSkillId: v.Loadout.BasicSkillID,
			ActiveSlots:  v.Loadout.ActiveSlots[:],
		},
		ProgressionRevision: v.ProgressionRevision,
	}
}
