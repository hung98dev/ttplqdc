# Authority Model
status: LOCKED

## Scope
Defines which component may decide, mutate, persist, predict, or only present each class of game state.

Canonical stack:
- client = Unity/C#
- backend = Go
- durable database = PostgreSQL

The fixed-step single-owner simulation decision is ADR-0007.

# Core Rule
The Go backend is authoritative for every gameplay-critical or persistent result. Unity sends intent, never a trusted result.

The client may predict for responsiveness, but prediction has no ownership over final position, damage, cooldown, item, currency, quest, reward, PvP, guild, or persistence state.

# Authority Matrix
| Domain | Unity may | Go authority | PostgreSQL role |
|---|---|---|---|
| input | sample/queue local input | validate/accept/reject | none |
| player movement | predict/interpolate | legal movement/position/velocity | bounded recovery/checkpoint only |
| combat action | animate predicted startup | action acceptance/timing/hit/effect | persistent results only |
| HP/MP/status/cooldown | display/predict cosmetic UI | canonical values/timers | persistent subset where required |
| monsters/bosses | render/interpolate | AI/state/combat/death | durable rewards/progression only |
| inventory/equipment | display/request mutation | ownership/equip/use/craft/enhance | canonical durable truth |
| currency/economy | display/request spend | validate/debit/credit | canonical durable truth |
| rewards | preview configured possibilities | eligibility/RNG/settlement | canonical committed result |
| quests/progression | display objectives | objective progress/completion | canonical durable truth |
| Souls/builds | edit requested loadout | validate/activate/effects | canonical durable truth |
| map transfer | request portal/use | destination/access/ownership handoff | recovery flags where required |
| PvP/Guild War | input/presentation | teams/combat/result/rating/reward | canonical durable result |
| guild/social | present/request | permission/membership/mutation | canonical durable truth |
| Auction/trade | present offers/request | price/escrow/settlement | canonical durable truth |
| content revision | bundle compatible data | select/validate active revision | active revision metadata |

# Unity Trust Boundary
Client-owned data is never accepted as proof of:
- position or velocity,
- hit/damage/healing/shield amount,
- status success/cooldown completion,
- resource/item/currency ownership,
- reward result/RNG output,
- quest completion/boss contribution,
- trade/Auction/PvP result,
- authoritative server time.

Client fields may be used as requested intent when the server independently validates them. Examples include movement axis/jump/drop, desired facing, skill ID, requested target entity or AREA_POSITION point, portal/NPC/interaction ID, inventory operation, and input sequence/ack metadata.

A requested target coordinate is not trusted world state. The server checks range, collision, ownership, visibility/eligibility, and content rules.

# Simulation Authority
Live world state is partitioned into single-owner simulation scopes:
- normal map = map_id + channel_id,
- dungeon/finale/PvP/Guild War = instance_id.

Exactly one Go simulation owner may mutate an entity at one time. Network goroutines, persistence workers, schedulers, admin tooling, and other partitions may send typed commands to the owner but do not directly mutate its live state.

# Durable Authority
PostgreSQL is canonical durable truth for ownership/value/progression state.

Durable mutations complete only after the owning PostgreSQL transaction commits. A success response must never be sent merely because an in-memory optimistic mutation occurred.

Where simulation needs an immediate mirror of durable state:
1. submit a stable operation ID,
2. transactional domain layer validates and commits,
3. committed outcome returns,
4. simulation/client mirror applies the committed result.

A timeout after commit is resolved by idempotent retry/reconstruction, not duplicate execution.

# Movement Prediction
Unity may predict the local character to keep controls responsive. Server correction always wins when prediction differs from authority.

Allowed reconciliation:
- small error -> smooth visual correction,
- large/illegal error -> authoritative snap/correction,
- repeated illegal movement is rejected rather than normalized into accepted client transforms.

Remote entities are interpolation/extrapolation presentation only.

# Combat Prediction
Unity may start local animation/VFX before the authoritative result arrives, but it must not locally finalize damage, death, loot, hard control, projectile hit, cooldown/resource refund, boss phase, or reward.

Authoritative server events may cancel or correct predicted presentation.

# Server Time
Three time concepts are distinct:
- fixed-step simulation time for combat/timers,
- monotonic process time for elapsed-duration measurement,
- UTC wall time for schedules, timestamps, expiries, daily/weekly reset boundaries.

Client wall clock never decides an authoritative timer/reset.

# Session Authority
One authenticated live session epoch owns player input routing at a time **per account** (ADR-0030).

A reconnect/login replacement creates a newer account session epoch. Messages from an older epoch are rejected even if they arrive later. A second connection cannot attach another character of the same account while the first is live.

A transport connection itself is not character authority; current session and simulation ownership records decide authority.

# Transfer Authority
Cross-partition transfer follows:
ACTIVE_SOURCE -> TRANSFER_FROZEN -> HANDOFF_CREATED -> DESTINATION_ACCEPTED -> SOURCE_RELEASED -> ACTIVE_DESTINATION.

During TRANSFER_FROZEN, normal gameplay mutation is rejected for that character. The destination may not accept gameplay input until its ownership epoch is authoritative, and the source may not resume after destination acceptance commits.

Overall budget is `TRANSFER_BUDGET_WORLD = 30s` or `TRANSFER_BUDGET_INSTANCE = 120s` in `concurrency.md` (reconnect attachment grace). Timeout is `TRANSFER_FAILED` plus canonical source/entry/checkpoint recovery; it never creates two active copies.

REFLECT and ABSORB secondary results for a TRANSFER_FROZEN entity: if an action whose hit resolution was already in flight on the **source partition** (i.e., the hit occurred before the freeze committed) would generate a REFLECT or ABSORB secondary result, that result is generated and applied by the source partition before the freeze completes. It is never deferred to the destination, and it is never silently dropped. The source partition records the result in the transfer payload so the destination can present it after handoff. An action that originates **after** TRANSFER_FROZEN is committed is rejected along with all other normal gameplay mutations; no REFLECT or ABSORB secondary result is generated for it.


# Administrative / Tooling Boundary
Operational/admin tooling may invoke explicit authenticated server operations. It may not use ad-hoc direct PostgreSQL row edits as a normal gameplay-management path.

Value-affecting admin operations require operator identity, reason, stable operation/audit ID, reconstructable outcome, and permission scope. Operator authentication, roles and the two-person rule are canonical in `../07_security/auth.md` § Operator.

# Invariants
- Unity owns presentation/input, not gameplay truth.
- Go owns authoritative live gameplay.
- PostgreSQL owns canonical durable truth.
- One live entity has one simulation owner.
- One account has one live gameplay session epoch (ADR-0030); at most one character attached.
- Client clock/RNG/result is never authority.
- Durable success requires a committed transaction.
- Prediction is always correctable.
- REFLECT and ABSORB secondary results from in-flight actions are generated and applied by the source partition before freeze completes; they are never silently dropped.
- reflect damage, lifesteal heal, and absorb shield are non-predictable; rendered only from S2C_COMBAT_EVENT.
