// Package skills is the compiled launch skill catalog and its resolution
// layer (IMP-015 Skill Runtime / Geometry).
//
// The registry embeds the authoritative source tables of
// docs/07_content/class_skill_catalog.md — the Canonical Runtime Matrix,
// Action Specifications, Active Payloads, Zone Schedules, Basic Attack
// Cooldown & Status Proc Matrix, Target Scaling and Secondary Spatial
// Effects — for the 45 basic/active launch definitions (20 basics +
// 25 actives across five classes; passives carry no execution geometry
// and are not runtime actions).
//
// Resolution semantics follow docs/01_gameplay/skills.md and
// docs/04_architecture/physics_geometry_contract.md: int-mm coordinates,
// RoundDiv quantization, 1 mm contact epsilon, SKILL_ORIGIN_Y =
// caster anchor + 0.9 m, STARTUP -> ACTIVE -> RECOVERY deadlines summed
// in milliseconds then rounded up to 50 ms ticks, ON_START cost/cooldown
// commit-once, and the authoritative basic interval formula.
//
// The package consumes sim/combat's action-FSM contracts (SkillDef
// projection, GeometryKind/TargetBand/TimingSpeed/CostTiming enums,
// Hurtbox) and sim/spatial's authoritative collision world. Damage,
// status lifecycle and shield pipelines remain owned by their specs —
// this package produces typed requests and spatial outcomes, it does
// not redefine them.
package skills
