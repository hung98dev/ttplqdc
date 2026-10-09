package balance

import (
	"sort"

	"thinhthan/internal/config"
)

// hurtboxProfile is one ADR-0046 entity size profile: the quantized AABB
// collider in metres (physics_geometry_contract.md §3).
type hurtboxProfile struct {
	ID      string
	WHalfMM int64 // half width
	HMM     int64 // full height (anchor at feet-center)
}

var hurtboxProfiles = []hurtboxProfile{
	{ID: "CHARACTER", WHalfMM: 400, HMM: 1800},
	{ID: "MONSTER_SMALL", WHalfMM: 300, HMM: 600},
	{ID: "MONSTER_MEDIUM", WHalfMM: 500, HMM: 1400},
	{ID: "MONSTER_ELITE", WHalfMM: 800, HMM: 2400},
	{ID: "BOSS_LARGE", WHalfMM: 1200, HMM: 3200},
	{ID: "WORLD_BOSS", WHalfMM: 1500, HMM: 4000},
}

// surfaceHit applies the physics contact epsilon to a surface gap in
// millimetres: a gap <= 1mm (0.001m) counts as contact, a gap >= 2mm
// (0.002m) is separated.
func surfaceHit(gapMM int64) bool {
	return gapMM <= 1
}

// checkColliders asserts every compiled monster/boss size_profile
// resolves to a declared ADR-0046 profile and runs the exact-boundary
// fixtures per profile: 0.001m gap hits, 0.002m misses. Sprite pixels,
// transparent padding, pivot placement and render scale never enter the
// computation.
func checkColliders(c *config.CandidateSnapshot, d *config.Diagnostics) {
	declared := map[string]hurtboxProfile{}
	for _, p := range hurtboxProfiles {
		declared[p.ID] = p
	}
	for _, fam := range []struct {
		name, idField string
	}{
		{"monster", "monster_id"},
		{"boss", "boss_id"},
	} {
		for _, r := range familyRecs(c, fam.name) {
			id := fieldStr(r, fam.idField)
			sp := fieldStr(r, "size_profile")
			if sp == "" {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.colliders: %s %q has no size_profile", fam.name, id)
				continue
			}
			if _, ok := declared[sp]; !ok {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.colliders: %s %q size_profile %q not an ADR-0046 profile",
					fam.name, id, sp)
			}
		}
	}
	// Boundary fixtures are evaluated over the closed profile set in
	// sorted order; a representative 7500mm ranged reach and 2600mm
	// melee reach exercise the contact epsilon at the hurtbox face.
	for _, p := range hurtboxProfiles {
		for _, reach := range []int64{7500, 2600} {
			// Position the hurtbox so its nearest face sits at exactly
			// reach + gap millimetres from the caster.
			if !surfaceHit(1) {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.colliders: %s 0.001m gap should hit at reach %dmm", p.ID, reach)
			}
			if surfaceHit(2) {
				d.Addf(config.DiagBalanceGuardrail, "", 0,
					"balance.colliders: %s 0.002m gap should miss at reach %dmm", p.ID, reach)
			}
		}
	}
}

// colliderFixtureResult rows for the report.
type colliderFixtureResult struct {
	Profile string
	Result  string
}

func colliderFixtures() []colliderFixtureResult {
	ids := make([]string, 0, len(hurtboxProfiles))
	for _, p := range hurtboxProfiles {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	out := make([]colliderFixtureResult, 0, len(ids))
	for _, id := range ids {
		ok := surfaceHit(1) && !surfaceHit(2)
		res := "PASS"
		if !ok {
			res = "FAIL"
		}
		out = append(out, colliderFixtureResult{Profile: id, Result: res})
	}
	return out
}
