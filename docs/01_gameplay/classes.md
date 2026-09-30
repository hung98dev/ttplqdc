# Classes
status: LOCKED

## Core Rule
The game has exactly five base classes, each bound to one primary element.

| class_id | Element | Display identity | Combat identity |
|---|---|---|---|
| `class.kim` | KIM | Kiếm Khách | fast single-target pressure, crit, penetration-style effects |
| `class.moc` | MOC | Dược Sư | sustain, healing, poison, buffs/debuffs |
| `class.thuy` | THUY | Thủy Sư | ranged control, mobility, slow/freeze |
| `class.hoa` | HOA | Phù Sư | burst, area damage, burn |
| `class.tho` | THO | Hộ Pháp | durability, protection, displacement resistance |

Display names are localization content; stable IDs are authoritative.

Every class must be viable for solo PvE. Roles describe strengths, not mandatory party slots.

## Class Combat Cadence and Basic Attack Profiles
Each class possesses an explicitly distinct basic attack cooldown profile and combat rhythm balanced against its damage coefficients, range, and crowd control under ADR-0016. Per-class cooldown values are canonical in `skills.md`; the ranges below are documentation only — `class_skill_catalog.md` owns the authoritative `base_coefficient` per basic skill.

| Class | Element | Cadence / Role | Hit Coefficient (range — see catalog) | Core Proc Effect |
|---|:---:|---|:---:|---|
| **Kiếm Khách** | KIM | Rapid melee pressure & crit | 1.00 - 1.25x | BLEED, VULNERABLE, CRIT_MARK |
| **Thủy Sư** | THUY | Ranged kite & chilling control | 0.92 - 1.18x | CHILL, SLOW, FREEZE |
| **Dược Sư** | MOC | Ranged sustain & stacking poison | 0.90 - 1.15x | POISON, HEAL_REDUCTION |
| **Phù Sư** | HOA | High-impact ranged burn & splash | 0.95 - 1.22x | BURN, RESIST_SHRED |
| **Hộ Pháp** | THO | Heavy seismic melee & hard control | 1.05 - 1.35x | STUN, ROOT, WEAKEN |

Slower classes (e.g. THO, HOA) deal higher damage per hit and inflict heavier crowd-control/area effects to compensate for lower attack frequency. Faster classes (e.g. KIM, THUY) rely on high attack cadence and proc stacking. Cooldowns are strictly non-uniform across classes.

## Vietnamese-Folklore Presentation
Class mechanics use Ngũ Hành, but class presentation must belong to the same Vietnamese folk-fantasy world as the encounter catalog.

Allowed visual/material direction includes:
```text
plain village/travel clothing
woven cloth, bamboo, wood, bronze/iron, paper, cord, herbal bundles
Vietnamese rural craft/tool silhouettes
fictional folk charms and protective marks
river/mountain/forest working gear appropriate to the world
```

Do not default classes to:
```text
Chinese cultivation-sect robes or immortal swordsman styling
Japanese onmyoji/shrine-warrior templates
Western plate-mage-priest class silhouettes
glowing crystal armor with no local material logic
oversized Hán-fantasy ceremonial costumes as everyday combat wear
```

Hán-Việt class/skill vocabulary may remain where natural to Vietnamese fantasy language, but naming alone does not establish cultural identity. Art, props, animation, VFX motifs, NPC reactions, and skill presentation should reinforce Vietnamese material culture and folklore.

Class VFX should prefer grounded motifs before abstract magic:
- KIM: forged metal, blade arcs, bronze/iron sparks, decisive footwork.
- MOC: herbs, leaves, roots, medicinal smoke, woven/herbal implements.
- THUY: river water, rain, mist, ferry/stream motion rather than celestial ocean magic.
- HOA: lamps, hearth/fire, fictional paper charms, ember trails rather than immortal-flame iconography.
- THO: packed earth, stone, brick, boundary markers, grounded stance rather than giant fantasy crystal walls.

These are presentation constraints, not new combat mechanics.
## Fixed Class Appearance
Base character appearance is fixed upon creation according to class. Players cannot customize hair, face, skin, or body presets during character creation. Each class possesses one canonical 2D chibi visual design embodying its Vietnamese-folklore combat identity. Visual changes after creation occur solely via non-power cosmetics (`../03_systems/cosmetics.md`).

## Ngũ Hành
Generation cycle:

```text
MOC -> HOA -> THO -> KIM -> THUY -> MOC
```

Control cycle:

```text
MOC > THO
THO > THUY
THUY > HOA
HOA > KIM
KIM > MOC
```

### Elemental Combat Modifier
Only elemental damage uses the control modifier.

If attack element controls target primary element:

```text
element_multiplier = 1.05
```

Otherwise:

```text
element_multiplier = 1.00
```

Physical/non-elemental damage receives no element modifier.

Generation relationships do not grant a universal combat bonus. Systems such as Spirit Meridian, Formations, Guild Ritual, and explicit skills may use them separately.

### KHAC DETONATION
When an elemental damage component of element **A** connects with a target that carries an active `DOT`-tagged status whose `element` (from its effect template, `status_effects.md`) is controlled by **A** (per the control cycle above), all remaining ticks of that DoT resolve immediately at `1.50×` and the DoT instance is consumed. Only `DOT`-tagged statuses are detonation targets; non-DOT markers (CHILL, WEAKEN, SLOW, …) are never consumed or detonated.

```text
Limit: one detonation per target per 2.0s.
```

Resolution (deterministic):
- Trigger: the triggering component commits `hp_damage >= 0` without `DODGED`; detonation resolves after that commit and before the component's own status applications.
- Every eligible DoT instance on the target is consumed in the tick order of `status_effects.md` § Determinism; the whole set counts as one detonation for the 2.0s limit. While the limit is active nothing is consumed.
- Per consumed instance: `remaining_ticks` = number of scheduled ticks `t` with `now < t <= expires_at` (`../07_content/class_skill_catalog.md` § Canonical Basic Effect Templates). It commits one damage result owned by the DoT's `source_id` with `raw_damage = remaining_ticks * stack_count * floor(source_attack_snapshot * per_tick_attack_ratio)`, `source_damage_multiplier = 1.50` (stats.md Damage Pipeline step 2), the DoT template element, `can_crit=false`, `can_dodge=false`. `remaining_ticks = 0` consumes the instance with no damage.
- Ranked PvP multiplier overrides replace `1.50` only.

Launch player DoTs and the resulting cross-class combos:

| Attacker element | Controls | Detonates (launch DoT template) |
|---|---|---|
| KIM | MOC | MOC POISON (`effect.basic.poison_4s`, `poison_stack_4s`) |
| HOA | KIM | KIM BLEED (`effect.basic.bleed_3s`) |
| THUY | HOA | HOA BURN (`effect.basic.burn_3s`, `burn_true_3s`) |
| THO | THUY | none at launch (no THUY-element DoT template) |
| MOC | THO | none at launch (no THO-element DoT template) |

THO and MOC attackers gain detonations only if a future THUY/THO-element DoT template is added; the rule above needs no change. KIM, MOC and HOA DoT appliers are the setup side; KIM, HOA and THUY attackers are the payoff side.

KHAC DETONATION adds no resource bar and no power layer. It reuses the existing status tag system (`DOT` tag) and the existing `element_multiplier` pipeline.

Ranked PvP may override the multiplier in `../03_systems/pvp.md`.

## Selection
- Class is chosen during character creation.
- Class change is not supported.
- Class advancement/evolution is not enabled initially.

## Skills
- Each class owns its own skill pool from `skills.md`.
- Cross-class skill learning is not supported.

## Equipment
- Equipment has no class restriction.
- Equipment has no armor-class restriction.
- Cross-element equipment is allowed.
- Class identity comes primarily from skills and stat growth, not item lockouts.

## Stats
Class base growth and potential conversion are canonical in `stats.md`.

## New Stats and Class Identity

The following is identity guidance for content authors — designers building class-specific items, set bonuses, Soul effects, or Formation entries. These stats are **not new class mechanics** and carry **no per-class stat bonuses** unless explicitly defined by separate content. Any class may roll any of these stats on equipment.

| Stat | Neighbours class | Rationale |
|---|---|---|
| `HEAL_REDUCTION` | MOC | MOC already owns `HEAL_REDUCTION` as a core proc (see Combat Cadence table); HEAL_REDUCTION as a secondary-roll stat reinforces MOC as the anti-sustain class |
| `LIFESTEAL` | KIM | KIM already owns conditional lifesteal (`kiem_y_bat_diet`); LIFESTEAL as an equipment stat is KIM-flavoured |
| `ABSORB` | THO | THO already owns shields (`tho_giap`, `son_bich`, `hau_tho`); ABSORB shield creation sits in THO-space |
| `REFLECT` | THUY | THUY already has on-melee-hit retaliation via `bang_giap_tam` (CHILL reflect); REFLECT neighbours that mechanic |

This alignment expresses the four-stat interlock from the design intent: MOC becomes the anti-sustain class (counters KIM lifesteal), THO's absorb shields are immune to MOC heal reduction, and THUY reflect punishes melee pressure. Content authors should lean on this alignment to reinforce class identity, but it is not a constraint on item generation or player choice.

## Party Design
No party composition requires one of each class or element. Tương sinh synergy exists only when an explicit skill/system effect defines it.
