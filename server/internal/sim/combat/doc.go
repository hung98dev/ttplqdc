// Package combat implements the authoritative combat action state
// machine of docs/01_gameplay/combat.md: STARTUP -> ACTIVE -> RECOVERY
// action phases, runtime action states, the in_combat combat lock,
// deterministic targeting under ADR-0018 caps, the full damage pipeline
// (dodge -> mitigation -> Just Guard -> shields -> HP), stage-7
// secondary results (reflect/lifesteal/absorb per ADR-0037), the
// edge-triggered Just Guard streak mechanic (ADR-0034) with its bounded
// latency-compensation model (ADR-0038), and the wire surface of
// messages.md 200..208 + S2C_COMBAT_EVENT (304).
//
// The package is self-contained: skill data, stat projections,
// hurtboxes, status classification and the once-only
// progression.first_session.just_guard_hint flag write are injected
// ports, so world/edge wiring lands through the owning lanes.
package combat
