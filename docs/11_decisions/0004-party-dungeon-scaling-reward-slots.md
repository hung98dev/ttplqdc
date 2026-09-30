# ADR-0004: PARTY Dungeon Scaling and Reward Slot Semantics
status: ACCEPTED

## Context
Launch PARTY dungeons allow `1..5` members, but no scaling rule previously defined how the same NORMAL encounter remains viable solo and with five players. Reward specs also used both lifetime first-clear grants and daily-first grants while generic dungeon/boss text used the term "first clear" ambiguously.

## Decision
- PARTY major encounters sample active eligible member count once per attempt.
- Default launch scaling is `HP = 1 + 0.55*(n-1)` and `damage = 1 + 0.04*(n-1)` for `n=1..5`.
- Active attempts do not live-rescale; wipe/reset may resample.
- Rewards use distinct slots: `REPEAT`, lifetime `FIRST_CLEAR`, and UTC-day `DAILY_FIRST`.
- Reward slots never create entry lockouts.
- Standalone PUBLIC boss respawn timers are not MAIN-story gates unless an always-available instanced equivalent exists.

## Consequences
- One NORMAL dungeon definition supports solo-through-five-player play without a generic difficulty ladder.
- Disconnect/reconnect cannot cheaply manipulate active boss durability.
- Guaranteed Boss Souls and dungeon first-clear set pieces have unambiguous lifetime semantics.
- Optional daily accelerators remain separate from permanent first-clear rewards.
