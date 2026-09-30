# Meridian and Formation Catalog
status: LOCKED

## Scope
Concrete launch roster for the derived build systems owned by `../03_systems/spirit_meridian.md` and `../03_systems/formations.md`.

The visible naming/visual language follows Vietnamese village, craft, landscape, and ritual motifs. Ngũ Hành is the mechanical substrate underneath the presentation.

No resonance or Formation is an inventory collectible. They unlock as derived patterns when the owning level requirement is met and equipped elements/relations satisfy the matcher.

## Shared Rules
```text
Meridian unlock = Level 20
Formation unlock = Level 25
```
- Higher `priority` wins inside a mutually exclusive group, then stable ID lexical ascending.
- Percentages use canonical typed stat/effect stages from `../01_gameplay/stats.md`.
- Trigger recursion follows the global depth rules.
- Patterns read equipment state; they never mutate equipment element.

# Meridian — 15 Resonances

Topology is the 8-BASIC-slot ring:
```text
weapon -> hands -> ring -> necklace -> head -> body -> legs -> feet -> weapon
```

## Launch Equipment Domain
At every launch tier, the two set layouts expose these BASIC choices in canonical Meridian order:
```text
slot       Set A   Set B
weapon     KIM     HOA
hands      KIM     THO
ring       HOA     KIM
necklace   MOC     THUY
head       THO     MOC
body       MOC     THUY
legs       THO     KIM
feet       THUY    MOC
```
Therefore a tier has `2^8 = 256` legal BASIC element sequences.

The old launch assumptions `DONG_HE pair for every element`, `DONG_HE_CHAIN >= 3`, and two of the old explicit relation subsequences were unreachable under this domain. The definitions below replace those impossible matchers without changing the 15-resonance budget.

## Coexistence Groups
```text
THEME
FLOW
EXPLICIT
FULL_RING
```
At most one resonance per group. `FULL_RING` suppresses `EXPLICIT`. Thus one ACTIVE loadout has at most three meaningful Meridian effects: THEME + FLOW + one of EXPLICIT/FULL_RING.

## THEME — 5
Matcher primitive: `ELEMENT_COUNT`.

| resonance_id | Display | priority | Matcher | Effect |
|---|---|---:|---|---|
| `resonance.kim.tieng_dong` | Tiếng Đồng | 25 | `KIM >= 3` | Critical hit -> `+0.04 ATTACK_SPEED` for 3s; cooldown 6s. |
| `resonance.moc.loc_tre` | Lộc Tre | 24 | `MOC >= 3` | Applying ROOT or SLOW heals `1.5% MAX_HP`; cooldown 8s. |
| `resonance.thuy.mach_ben_nuoc` | Mạch Bến Nước | 23 | `THUY >= 3` | After paying MP for an active skill, restore `3%` of paid cost after 1s, minimum 1 MP; cooldown 3s. |
| `resonance.hoa.lua_den_dinh` | Lửa Đèn Đình | 22 | `HOA >= 2` | First damaging hit against target carrying BURN/POISON gains `+0.06` source-additive damage; cooldown 5s per target. |
| `resonance.tho.mach_dat_lang` | Mạch Đất Làng | 21 | `THO >= 3` | Taking hostile damage -> `+0.04 DAMAGE_REDUCTION` for 2.5s; cooldown 8s. |

The priority ordering intentionally resolves overlaps and was enumerated against all 256 legal sequences.

Selected-winner witness bitstrings (`0=Set A`, `1=Set B`, canonical BASIC order):
```text
KIM  00000010
MOC  00000001
THUY 00010100
HOA  10000000
THO  01000000
```

## FLOW — 5
Four definitions use `RELATION_CHAIN`; Đồng Khí uses `RELATION_COUNT` because a 3-link DONG_HE chain is impossible in the launch equipment domain.

| resonance_id | Display | priority | Matcher | Effect |
|---|---|---:|---|---|
| `resonance.chain.sinh_out` | Dòng Sinh Thuận | 34 | `SINH_OUT chain >= 2` | Every 4th active skill restores `4% MAX_MP`; counter resets after 8s inactivity. |
| `resonance.chain.sinh_in` | Dòng Sinh Hồi | 33 | `SINH_IN chain >= 2` | Receiving heal or shield -> `+0.05 MOVE_SPEED` for 3s; cooldown 6s. |
| `resonance.chain.khac_out` | Dòng Trấn Thuận | 32 | `KHAC_OUT chain >= 2` | Damaging enemy with NEGATIVE status -> `+0.05 DAMAGE_BONUS` against that target for 3s; cooldown 8s per target. |
| `resonance.chain.khac_in` | Dòng Trấn Hồi | 31 | `KHAC_IN chain >= 2` | Owner receives NEGATIVE status -> `+0.06 DAMAGE_REDUCTION` for 2s; cooldown 10s. |
| `resonance.chain.dong_he` | Dòng Đồng Khí | 30 | total `DONG_HE links >= 2` | `+0.04 MAX_HP` PERCENT_ADD and `+0.012 ABSORB` FLAT_ADD. |

Selected-winner witnesses:
```text
SINH_OUT 00111000
SINH_IN  00001110
KHAC_OUT 00000000
KHAC_IN  00000001
DONG_HE  00001010
```

## EXPLICIT — 3
Matcher primitive: cyclic `RELATION_SUBSEQUENCE` = the required relations on consecutive ring links (contiguous, may wrap from `.08` to `.01`), per `../03_systems/spirit_meridian.md`. A witness is valid only if the resonance is the selected winner after FULL_RING suppression (enumerated over all 256 sequences; ADR-0071).

| resonance_id | Display | priority | Required subsequence | Effect |
|---|---|---:|---|---|
| `resonance.explicit.cau_tre` | Nhịp Cầu Tre | 42 | `SINH_OUT, SINH_OUT, SINH_IN` | After voluntary movement >= one character-width, next damaging active within 4s gains `+0.05` source-additive damage; cooldown 7s. |
| `resonance.explicit.ben_bo` | Bến Bờ | 41 | `SINH_IN, DONG_HE, DONG_HE` | Crossing below 40% HP restores `5% MAX_MP` and grants `+0.05 DAMAGE_REDUCTION` for 3s; cooldown 20s. |
| `resonance.explicit.luy_tre` | Lũy Tre | 40 | `DONG_HE, KHAC_OUT, DONG_HE` | Applying displacement or movement control grants a `4% MAX_HP` shield for 4s; cooldown 10s. |

Selected-winner witnesses:
```text
cau_tre 00111101
ben_bo   00100000
luy_tre  10101100
```

## FULL_RING — 2
All eight BASIC slots must be occupied.

### `resonance.full_ring.duong_lang` — Đường Làng Liền Mạch
priority: `60`
```text
FULL_RING
SINH_OUT + SINH_IN link count >= 4
KHAC_IN link count <= 1
```
Effect:
- every 5th active skill restores `5% MAX_MP`
- grants `+0.05 ATTACK_SPEED` for 4s
- counter resets after 10s inactivity

Selected-winner witness: `00000110`.

### `resonance.full_ring.vong_dinh` — Vòng Đình Khép Kín
priority: `61`
```text
FULL_RING
DONG_HE link count >= 3
KHAC_OUT + KHAC_IN link count >= 2
```
Effect:
- hostile damage grants one `DINH_GUARD` stack for 6s, max 3
- each stack grants `+0.02 DAMAGE_REDUCTION`
- stack-gain cooldown 2s
- all stacks expire together

Selected-winner witness: `00001000`.

# Formations — 12

Topology is the 6-ADVANCED-slot ring:
```text
costume -> talisman -> jade -> seal -> relic -> charm -> costume
```
All six slots must be occupied. Only one Formation is active.

## Launch Equipment Domain
At every launch tier:
```text
slot       Set A   Set B
costume    THO     HOA
talisman   HOA     THO
jade       THUY    MOC
seal       KIM     THUY
relic      MOC     KIM
charm      HOA     THO
```
Therefore one tier exposes `2^6 = 64` legal ADVANCED sequences.

The previous launch matchers requiring six identical elements, a literal full five-element generation/control cycle, or exact two-element alternation had zero legal witnesses. They are replaced below with reachable count/relation patterns while keeping the same identities/effects where possible.

## Element Emphasis — 5
Matcher primitive: `ELEMENT_COUNT_PATTERN`.

| formation_id | Display | priority | Matcher | Effect |
|---|---|---:|---|---|
| `formation.kim.chuong_dong` | Chuông Đồng | 25 | `KIM >= 2` | `+0.06 CRIT_DAMAGE`; crit -> `+0.03 MOVE_SPEED` 2s, cooldown 5s. |
| `formation.moc.luy_tre` | Lũy Tre Xanh | 24 | `MOC >= 2` | `HEALING_RECEIVED +0.08 FLAT_ADD`; receiving positive heal restores `2% MAX_MP`, cooldown 8s. |
| `formation.thuy.ben_nuoc` | Bến Nước | 23 | `THUY >= 2` | movement-tagged active -> next active within 4s costs `10%` less MP, cooldown 6s. |
| `formation.hoa.den_dinh` | Đèn Đình | 22 | `HOA >= 2` | damage against BURN/POISON target gains `+0.05` source-additive damage. |
| `formation.tho.luy_dat` | Lũy Đất | 21 | `THO >= 2` | `+0.05 MAX_HP` PERCENT_ADD; target shield-received multiplier `+0.06`. |

Selected-winner witnesses (`0=A`, `1=B`):
```text
KIM  000010
MOC  001001
THUY 000100
HOA  000000
THO  000001
```

## Generation Relation Patterns — 2
### `formation.vong_mua_thuan` — Vòng Mùa Thuận
priority `42`, `RELATION_COUNT_PATTERN`:
```text
SINH_OUT >= 3
KHAC_OUT = 0
KHAC_IN  = 0
```
Witness `001100` -> `THO, HOA, MOC, THUY, MOC, HOA`.

Effect:
- every 4th different active-skill ID within 10s grants `NHIP_MUA` 5s
- `NHIP_MUA`: `+0.05 ATTACK` PERCENT_ADD + `+0.04 MOVE_SPEED`
- cooldown 12s after proc

### `formation.vong_mua_nghich` — Vòng Mùa Nghịch
priority `31`, `RELATION_COUNT_PATTERN`:
```text
SINH_IN >= 5
```
Witness `001111`.

Effect:
- after hostile damage, arm 5s modifier for next heal/shield received
- that one resolved amount is multiplied by `1.12`
- cooldown 10s after consumption

## Control Relation Patterns — 2
### `formation.nam_cua_tran` — Năm Cửa Trấn
priority `32`:
```text
KHAC_OUT >= 3
```
Witness `010001`.

Effect: applying ROOT/STUN/FREEZE/SLOW/PULL/KNOCKBACK grants `+0.05 DAMAGE_REDUCTION` for 3s; cooldown 8s.

### `formation.nam_cua_pha` — Năm Cửa Phá
priority `33`:
```text
KHAC_IN >= 3
```
Witness `011010`.

Effect: first damaging hit against NEGATIVE-status target applies `-0.08 DEFENSE` through target PERCENT_ADD for 4s; same-source non-stack; cooldown 10s per target.

## Paired Motifs — 2
### `formation.tre_lua` — Tre Gặp Lửa
priority `40`:
```text
MOC >= 2
HOA >= 2
```
Witness `001000`.

Effect: BURN/POISON periodic damage may trigger one `0.25 ATTACK` area pulse every 8s; one pulse per root event; no self-retrigger.

### `formation.dat_nuoc` — Đất Giữ Nước
priority `41`:
```text
THO >= 2
THUY >= 2
```
Witness `000101`.

Effect:
- while shielded `+0.05 MOVE_SPEED`
- shield expiry or hostile break -> `+0.06 DAMAGE_REDUCTION` 3s, cooldown 10s

## Rare Explicit — 1
### `formation.trong_dong_sau_canh` — Trống Đồng Sáu Cánh
priority `60`, `EXPLICIT_SEQUENCE`:
```text
HOA -> THO -> MOC -> KIM -> MOC -> THO
allow_rotation = true
allow_reflection = false
```
Canonical witness bitstring: `111001`.

Effect:
- three different active-skill IDs within 6s grants `NHIP_TRONG` 6s
- `NHIP_TRONG`: `+0.06 ATTACK` PERCENT_ADD + `+0.06 MOVE_SPEED`
- cooldown 20s after proc

# Reachability Validation
Static compilation enumerates all `256` same-tier BASIC sequences and all `64` same-tier ADVANCED sequences, then runs the actual group/priority/tie-break algorithm.

Required assertions:
```text
for every resonance_id:
  selected_group_witness_count >= 1

for every formation_id:
  selected_winner_witness_count >= 1
```

A syntactically valid but unreachable/always-shadowed matcher fails activation. Validation tooling records at least one bitstring witness per definition.

# Invariants
```text
Meridian definitions = 15
Formation definitions = 12
Meridian active meaningful effects <= 3
Formation active per loadout <= 1
all 15 Meridian definitions have legal selected-group witnesses
all 12 Formation definitions have legal selected-winner witnesses
no collectible unlock item required
Vietnamese presentation; Ngũ Hành = mechanical substrate
```

# Power Budget Verification — ADR-0037

Two effect slots are re-pointed by this order. No new slots are added. Total definition counts are unchanged.

## Meridian: resonance.chain.dong_he re-point

Old slot: `+0.03 MAX_MP PERCENT_ADD` (resource stat; 0% contribution to global final damage)
New slot: `+0.012 ABSORB FLAT_ADD` (shield-from-damage stat; 0% contribution to global final damage)

Both old and new components are survivability/resource stats, not global damage multipliers. The slot re-point carries zero direct damage contribution. The combined unconditional global final-damage contribution of all 15 Meridian effects remains well under the declared 10% budget. ✓

Interlock check: ABSORB creates shields that are NOT affected by HEAL_REDUCTION (per ADR-0037 §4 interlock design and §5 post-mitigation reflect carve-out). Granting ABSORB from a Meridian passive does not break the lifesteal/heal-reduction/absorb counter-play loop. ✓

## Formation: formation.moc.luy_tre re-point

Old slot text: `target healing-received multiplier +0.08`
New slot text: `HEALING_RECEIVED +0.08 FLAT_ADD`

This is a notation change only. `HEALING_RECEIVED` is the promoted first-class stat that replaces the implicit `target_healing_received_multiplier` variable per ADR-0037 §2. The magnitude (0.08), effect type (FLAT_ADD to the multiplier stat, default 1.00), and all runtime behavior are unchanged. The global final-damage contribution of this Formation effect is 0% (it amplifies healing received, not damage output). ✓

## Budget totals unchanged

Meridian unconditional global final-damage contribution: <= 10% (unchanged). ✓
Formation unconditional global final-damage contribution: <= 10% (unchanged). ✓
