# ADR-0020: Map Channel Count and Channel Capacity Contract
status: ACCEPTED
amended_by: [ADR-0035]

> **AMENDMENT NOTICE**: Hard channel cap of 20 players was reduced to **18 players** (`MAX_PLAYERS_PER_CHANNEL = 18`, map cap `540`) by **ADR-0035** (`docs/02_world/world_rules.md`).

## Context
Normal-world maps in 2D side-scrolling action MMORPGs require bounded player density to ensure visual clarity, fair monster competition, responsive 20 Hz server simulation (canonical per ADR-0007), and smooth mobile client performance. Overcrowded maps cause extreme sprite overlapping, frame drops on mobile devices, and mob-tagging frustration.

The game architecture requires explicit, deterministic channel counts and per-channel capacity ceilings.

## Effective Contract (ADR-0035)

Implement these values. Do not read 20/600 from the historical Decision body.

```text
CHANNELS_PER_MAP        = 30
MAX_PLAYERS_PER_CHANNEL = 18
TOTAL_MAP_CAPACITY      = 540
SOFT_THRESHOLD_CHANNEL  = 18
Load bands: 1..11 Normal, 12..17 Busy, 18 Full
```

## Decision

> **HISTORICAL — do not implement.** Original 20/600 text is retained for provenance. Effective contract is the section above (ADR-0035).

1. **Fixed Channels per Map**:
   - Every normal open-world map (safe anchors and adventure fields) has exactly **30 channels** (`CHANNELS_PER_MAP = 30`), indexed `1..30`.
   - Player-facing presentation term: **Khu vực** (vi-VN: *Khu 1* .. *Khu 30*) / **Channel** (en-US: *Channel 1* .. *Channel 30*).

2. **Per-Channel Player Capacity (superseded)**:
   - Historical maximum was **20 players** (`MAX_PLAYERS_PER_CHANNEL = 20`). **Not current.**
   - Historical total map capacity was `30 * 20 = 600`. **Not current.** Current map cap is **540**.

4. **Channel Switching Rules**:
   - Players can choose and switch channels via the in-game channel menu.
   - Channel switching is rejected while `in_combat` or under a build/content lock.
   - Successful channel switch enforces a **10s cooldown** (`CHANNEL_SWITCH_COOLDOWN_SECONDS = 10`).
   - Party cohesion (historical): join leader channel if capacity `< 20`. **Current:** `< 18` (`MAX_PLAYERS_PER_CHANNEL`).

5. **Simulation & Spawning**:
   - Each channel instance maintains its own authoritative monster spawns and local dynamic entities.
   - Player progression, inventory, quests, chat, auction, and guild state remain globally shared within the logical world across all channels.

## Consequences
- Preserves tight, readable 2D side-scrolling combat without visual clutter or mobile performance degradation.
- Eliminates dynamic channel thrashing by establishing a fixed 30-channel topology per map.
- Provides predictable memory and goroutine sizing for Go backend simulation workers.

## Amendment — Routing Rule

> **Revised (then superseded in part by ADR-0035):** The original §2 routing rule directed new arrivals to the "lowest-population" channel. This was revised to route to the **most-populated channel still below the soft threshold** (`SOFT_THRESHOLD_CHANNEL = 18`). The sentence "hard cap of 20 ... unchanged" in the original amendment is **historical**; effective hard cap is 18.

## Amendment — ADR-0035

> **Superseded in part**: `MAX_PLAYERS_PER_CHANNEL = 20` defined in §2 was reduced to **18** by the decision recorded in **ADR-0035 §Amendment** (Spawn Density Increase, channel-cap fallback). This aligns the hard cap with the existing `SOFT_THRESHOLD_CHANNEL = 18` rather than inventing a new number. See ADR-0035 for the supply/demand rationale. `SOFT_THRESHOLD_CHANNEL` is unchanged; the full/busy/normal band boundaries require re-evaluation against the new hard cap in `02_world/world_rules.md`.
