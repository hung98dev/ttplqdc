# Gameplay Specification Index
status: LOCKED

This directory defines the launch gameplay rules. Implementation should read `core_loop.md` first, then the owning concept document.

## Canonical Documents
- `core_loop.md` — primary loop, session rhythm, recurring hooks, power-layer guardrails.
- `character.md` — character identity, creation, persistent/runtime state, lifecycle.
- `classes.md` — five classes, Ngũ Hành combat relationship, class identity.
- `movement.md` — side-scroll locomotion, double jump, platforms, movement authority.
- `combat.md` — action pipeline, targeting, `in_combat`, friendly fire, damage execution.
- `stats.md` — potential conversion, class growth, stat formulas, caps, damage/healing pipeline.
- `status_effects.md` — BURN/SLOW/FREEZE/STUN/ROOT, stacking, dispel, persistence.
- `skills.md` — class skill pools, unlocks, loadouts, costs, cooldowns, interrupts.
- `progression.md` — Level 1..60, EXP curve, skill/potential points, feature milestones, respec.
- `death_respawn.md` — death state, checkpoints, respawn, protection, no-loss policy.

## Launch Gameplay Shape
```text
EXPLORE
-> FIGHT
-> COMPLETE OBJECTIVES
-> EARN REWARDS
-> IMPROVE BUILD
-> UNLOCK HARDER CONTENT
-> REPEAT
```

The launch build intentionally keeps permanent power layers limited to those already named in `core_loop.md`.

Do not add another mandatory progression bar, resource bar, class advancement tree, durability loop, stamina gate, or permanent daily-streak power system without replacing/simplifying an existing layer and updating the owning canonical spec.

## Balance Principles
- every class remains solo-PvE viable
- party composition does not require one of each class/element
- Ngũ Hành control advantage is meaningful but small
- combat danger comes from readable mechanics, not unavoidable burst
- max-level build expression comes from choices across existing systems, not endless new subsystems
- failure costs time/retry, not permanent item/EXP/currency loss
- short sessions should still produce useful progress

## Engagement Principles
The game should be compelling through:
- responsive combat mastery
- visible character/build improvement
- exploration and folklore mystery
- optional social cooperation
- rotating but non-mandatory world/guild/PvP goals
- aspirational cosmetics/prestige

Avoid retention through punishment, hidden decay, mandatory login streaks, energy timers, or irreversible missed-day power loss.

## Source-of-Truth Rule
If two gameplay documents appear to disagree, the concept-owning document wins. In particular:

```text
in_combat -> combat.md
stats/damage -> stats.md
skill execution -> skills.md
death/respawn -> death_respawn.md
level/points -> progression.md
```
