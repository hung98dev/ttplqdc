# ADR-0005: Skill Action Timing and Geometry Contract
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: For equipped BASIC_ATTACK skills only, `ATTACK_SPEED` also lowers the interval floor: `interval_ms = max(ceil(cooldown_ms / (1 + ATTACK_SPEED)), startup_ms + active_ms)` (`../01_gameplay/skills.md`). Stored cooldowns are unchanged, `COOLDOWN_REDUCTION` never applies to basics, and ACTIVE cooldowns remain unaffected by speed stats.
amended_by: [ADR-0047]

> **AMENDMENT NOTICE**: ADR-0047 adds collider-aware reach budgets and two geometry variants. It also corrects the launch action count below to 20 basics and 25 actives.

## Context
Launch skills already define execution type, targeting, effects, cooldowns, and semantic tags, but authoritative startup/active/recovery timing and spatial geometry were not part of the required runtime shape. That leaves hit timing, projectile travel, cast reach, dash distance, and even baseline DPS dependent on implementation guesses. Skill points also require concrete per-level outcomes so combat balance can be simulated deterministically.

## Decision
- Every ACTIVE/basic skill carries explicit `startup_ms`, `active_ms`, `recovery_ms`, `timing_speed_stat`, and a typed geometry definition.
- Geometry uses a small closed union owned by `skills.md`; client animation/VFX never defines authoritative hit reach or projectile travel.
- `ATTACK_SPEED` is used by launch basic attacks; `CAST_SPEED` may accelerate authored ACTIVE startup/recovery; pure movement may opt into `NONE`.
- Speed stats modify startup/recovery only. The ACTIVE window, projectile speed, forced-movement duration/distance, zone lifetime, and cooldown remain authored values unless a definition explicitly says otherwise.
- `class_skill_catalog.md` owns concrete launch timing/geometry values for all 20 basics and 25 actives.
- Every upgradeable skill level must change at least one concrete numeric outcome. Launch scaling formulas remain content data in `class_skill_catalog.md`; no hidden runtime multiplier is inferred from rarity, animation, or tags.

## Consequences
- Server hit validation, client prediction, animation authoring, and balance simulation share one deterministic source of truth.
- TTK/DPS testing no longer needs guessed attack cadence or guessed ranges.
- Attack/Cast Speed have explicit timing surfaces and cannot silently shorten telegraphs, projectile travel, or movement distance.
- Skill-level tuning can be patched as content while preserving the same runtime schema.

## Amendment — ADR-0047

Range measurement, viewport budgets, `MOVE_CONTACT_LINE`, and `BARRIER_POSITION` are canonical in ADR-0047 and `docs/01_gameplay/skills.md`. The original action-count wording was a documentation error; the five classes have always owned four basics and five actives each.
