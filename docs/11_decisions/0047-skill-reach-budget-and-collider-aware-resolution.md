# ADR-0047: Skill Reach Budget and Collider-Aware Resolution
status: ACCEPTED

## Context

ADR-0005 made primary skill geometry explicit, but it did not define how ranges are measured against actor colliders or whether launch reach remains readable after ADR-0046 fixed the camera at `25.6m x 14.4m` and entity sizes. The 45 launch basic/active rows had concrete numbers, yet no activation gate compared them with the viewport.

The audit also found four spatial contract defects: `son_bich` represented a wall as a circular area; `luu_bo` applied contact CHILL without a contact sweep; `xuyen_phong` and `lien_bao` declared `DISPLACEMENT` without a forced-target movement effect; and `van_moc_hoi_sinh` described knockback without the tag or distance.

## Decision

1. Skill coordinates use the authoritative caster anchor and `SKILL_ORIGIN_Y = 0.9m`. Shapes hit when they intersect an authoritative target hurtbox; target pivot/sprite/alpha overlap is never sufficient.
   Physics quantization remains `0.001m`: a surface gap `<=0.001m` is contact and a quantized gap `>=0.002m` is not.
2. Launch reach budgets are canonical in `docs/01_gameplay/skills.md`. At 16:9, a positioned effect may extend at most `11.0m` from its caster, leaving `1.8m` inside the `12.8m` camera half-width. Projectile travel plus hit radius may not exceed `8.8m`.
3. All 45 primary geometry rows were audited. Existing numeric reaches remain unchanged because they satisfy their role bands and preserve melee/ranged identity.
4. Add typed variants:
   - `MOVE_CONTACT_LINE(distance_m, duration_ms, hit_half_height_m)` for movement with a swept contact effect;
   - `BARRIER_POSITION(cast_range_m, thickness_m, height_m, duration_ms)` for a grounded blocking wall.
5. `skill.thuy.active.luu_bo` uses `MOVE_CONTACT_LINE(4.5m, 280ms, 1.0m)` and may apply CHILL to the first eligible contact only.
6. `skill.tho.active.son_bich` uses `BARRIER_POSITION(6.5m, 0.8m, 4.0m, 5000ms)` with bottom-center ground anchoring.
7. Remove invalid `DISPLACEMENT` tags from `skill.kim.active.xuyen_phong` and `skill.hoa.active.lien_bao`. Add it to `skill.moc.active.van_moc_hoi_sinh`, whose activation knockback is a collision-clamped radial `1.5m`.
8. `skill.hoa.active.boc_bo`'s ember trail is presentation of its resolved `DASH_LINE` sweep, not a second persistent damage zone.
9. Every spatial secondary effect must declare an origin, shape/distance, selection order, target-cap interaction, and collision rule in the catalog. `skill.tho.active.dia_chan` declares its `0.8s` AIRBORNE presentation arc with a `1.2m` apex while the server collision anchor remains on the world plane.

## Consequences

- `docs/01_gameplay/skills.md` owns measurement semantics, reach bands, and the closed geometry union.
- `docs/01_gameplay/combat.md` resolves range by authoritative shape/hurtbox intersection.
- `docs/04_architecture/physics_geometry_contract.md` supplies collider profiles, millimeter quantization, and contact epsilon.
- `docs/06_data/config.md` and `docs/06_data/content_authoring_contract.md` require typed skill geometry before compile/activation.
- `docs/07_content/class_skill_catalog.md` owns all 45 primary values and concrete secondary spatial effects.
- `docs/07_content/integration_validation.md` rejects out-of-budget, prose-only, mistagged, or viewport-incompatible geometry.
- `docs/07_content/balance_validation.md` gates camera readability and melee/ranged separation.
- `docs/09_testing/gameplay.md` owns boundary, corrected-variant, secondary-geometry, and tag-consistency regressions.
- `docs/10_implementation/task_queue.md` adds compile/runtime tests to IMP-003, IMP-004, IMP-014, IMP-015, and IMP-049.
- No damage coefficient, cooldown, resource cost, target cap, or skill unlock level changes in this decision.
