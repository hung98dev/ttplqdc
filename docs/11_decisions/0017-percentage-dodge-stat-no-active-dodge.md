# ADR-0017: Percentage Dodge Stat and Prohibition of Active Dodge Actions
status: ACCEPTED

## Context
Side-scrolling action combat games often face a choice between active iframe dodging (such as dodge roll or invulnerability dashing) and statistical evasion. Active iframe dodges can introduce desynchronization, latency exploitation, and bypass defensive build progression in server-authoritative 2D MMORPGs with 10,000+ CCU across PC and mobile.

Combat design explicitly requires that dodging is not an active player action, but rather a percentage-based character stat (`DODGE_CHANCE`).

## Decision
1. **No Active Dodge Action**:
   - There is no active dodge key, dodge roll, or universal invulnerability iframe action in combat.
   - Movement skills (dashes, jumps, positioning skills) reposition the character authoritatively but never grant universal dodge iframes.
   - Players evade area telegraphs by spatial positioning, but incoming attacks with spatial overlap are resolved statistically.

2. **Dodge as a Percentage Stat (`DODGE_CHANCE`)**:
   - `DODGE_CHANCE` is a canonical defensive stat representing the probability of evading an incoming hostile attack.
   - `ACCURACY` is the canonical offensive counter-stat.
   - Formula:
     ```text
     effective_dodge = clamp(target.DODGE_CHANCE - attacker.ACCURACY, 0.00, DODGE_CHANCE_CAP)
     ```
   - Standard parameters:
     - `BASE_DODGE_CHANCE = 0.03` (3% base).
     - `BASE_ACCURACY = 0.00` (0% base).
     - `DODGE_CHANCE_CAP = 0.40` (40% hard cap to preserve PK balance and prevent unkillable evasion builds).
     - `ACCURACY_CAP = 0.40` (40%).

3. **Potential Stat Integration (`AGI`)**:
   - `AGI` provides `+0.0004 DODGE_CHANCE` per point, integrating dodge naturally into agility-based builds alongside `CRIT_CHANCE` and `ATTACK_SPEED`.

4. **Combat Resolution Pipeline**:
   - Dodge is evaluated on the server before damage calculation:
     - If `roll < effective_dodge`: Outcome commits as `DODGED`.
     - Damage is reduced to `0`.
     - On-hit damage, weapon procs, and associated status effects from that hit are negated.
     - Presentation displays floating combat text "Né" (vi-VN) / "Dodge" (en-US).
     - Target does not trigger `takes hostile damage` or `takes HP damage`.

## Consequences
- Eliminates client-server latency disputes over active iframe timing in high-latency mobile networks.
- Enriches defensive build variety (Dodge builds vs. Defense/Damage Reduction builds vs. Shield builds).
- Preserves tactical PK positioning while keeping hit resolution deterministic and server-authoritative.
