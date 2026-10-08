# Save Rules
status: LOCKED

## Scope
Defines what persists, commit timing, transactional groups, conflict handling, crash recovery, and what remains runtime-only.

## Principle
Persist ownership/value/progression at the operation boundary. Do not persist high-frequency simulation every frame.

A client-visible durable success requires PostgreSQL commit first.

# Save Classes
## IMMEDIATE_DURABLE
Commit before success when state changes durable ownership/value/progression.

Includes account/character lifecycle, level/EXP/skill/potential, currencies, items/locations, inventory expansion, equipment/loadouts, craft/enhance, Souls/build config, quest/first discovery/first clear, Reward Claims, trade/Auction/proceeds, guild state/storage, cosmetics, durable PvP/Guild-War settlement, and session/security revocation.

## CHECKPOINT_DURABLE
Persist at explicit safe lifecycle boundaries:
- active checkpoint,
- last canonical safe map/entry needed for recovery,
- successful transfer recovery record where required,
- instance membership/reconnect record where owning rules require it,
- boss aftermath relic upsert (`world_consequence_relics` + `region_di_tich_markers`) on every `DEFEATED -> COOLDOWN` transition and on relic expiry/despawn.

## RUNTIME_ONLY
Normally in memory:
- position/velocity every tick,
- current action phase,
- projectile/hitbox lifetime,
- AI working state,
- current target,
- short combat statuses/cooldowns,
- encounter-local timers,
- transient aggro,
- interpolation state.

Runtime-only state becomes persistent only if an owning spec explicitly adds a recovery contract.

# Normal World Placement
Never write position every tick.

Normal reconnect within live grace may reattach to the in-memory authoritative position.

After owner/process loss, restore through canonical checkpoint/map-entry recovery; client position snapshot is never recovery truth. Channel ID is routing/capacity state and is not permanent character identity.

# Atomic Transaction Groups
Craft:
~~~
consume inputs + debit currency + create output + location + operation outcome
~~~

Enhancement:
~~~
consume cost/material + finalize RNG + mutate enhancement + operation outcome
~~~

Quest/reward:
~~~
completion uniqueness + finalized reward slots + direct delivery/Reward Claim + operation outcome
~~~

Auction purchase:
~~~
ACTIVE->SOLD + buyer debit + escrow->buyer item + tax + seller proceeds + operation outcome
~~~

Direct trade:
~~~
both currency changes + all item location changes + COMPLETED + operation outcome
~~~

Guild creation:
~~~
currency debit + guild create + membership + leader role
~~~

Failure of any member rolls back the whole invariant group.

# Revisions / Conflicts
Use monotonic aggregate revision where concurrent mutation can race.

No last-write-wins for inventory, economy or role ownership. Stale revision rejects/reloads. Value mutation retries reuse the same operation ID.

# Autosave
There is no generic "save entire character every N seconds" blob.

Periodic work may persist explicitly declared recovery projections, but may never overwrite newer transactional inventory/currency/progression with an old whole-character snapshot.

Forbidden:
~~~
serialize whole character -> blind UPDATE
periodic snapshot overwrites committed value
client save restores online progression
~~~

# Disconnect / Logout
Normal logout stops new intents, resolves transfer ownership, commits required recovery state, waits only for bounded already-started durable operations or exposes retry-safe outcome, then removes runtime entity by world rules.

Unexpected disconnect preserves committed DB state; uncommitted transaction rolls back/fails; live simulation may retain reconnect grace; lost response does not roll back committed reward/currency.

# Crash Recovery
After Go process crash:
- PostgreSQL committed state is canonical,
- runtime-only state is discarded unless another canonical owner already accepted transfer,
- ambiguous transfer resolves through ownership epoch/transfer record,
- value-operation retry reconstructs prior commit by operation ID,
- normal-world placement uses checkpoint/entry recovery,
- instance recovery follows owning content rules,
- active boss aftermath relics are recovered from `world_consequence_relics` (rows with `relic_active = true` and `expires_at > now()`, keyed on `map_id + channel_id + relic_id`); remaining duration is clamped to at least 1 second; the server resolves the running partition by `map_id + channel_id` and re-applies the channel-wide buff to characters already in that partition.

# Database Outage
When a durable commit is required and PostgreSQL is unavailable:
- fail closed,
- do not confirm a value mutation,
- do not queue unbounded memory-only credits,
- reuse the same operation ID after recovery.

Simulation-originated commands (kill/boss/quest/discovery settlements, `WriteWorldConsequence`):
```text
operation_id        = UUIDv5(CONTENT_GRANT_NAMESPACE_UUID, source_event_name)  -- encoding below
holding             = per-partition durable-result queue (64, ../04_architecture/concurrency.md)
retry               = same operation_id, exponential backoff 100 ms .. 5 s while the DB is unavailable
queue full          = partition enters DURABLE_BACKPRESSURE until the queue drains below 32:
                      no new boss/encounter activation, dungeon completion or quest turn-in is accepted;
                      NORMAL/ELITE kills still resolve but grant no rewards (no queued credit);
                      clients get the system notice loc.system.rewards_paused and blocked requests return `DURABLE_BACKPRESSURE` (`../05_network/errors.md`); metric + alert
crash before commit = uncommitted commands are lost; none was shown as final; loss is bounded by the queue
```

`partition_incarnation_id` is a fresh crypto-random UUID v4 created once whenever a simulation owner starts/restarts, including reactivation of the same map/channel. `source_event_id` is a uint64 counter starting at 1 and incremented once per finalized source event in that incarnation; overflow stops the partition before reuse. The source event carries the original `tick` and complete finalized outputs. Retries, all recipient settlements and maintenance-journal replay reuse this event identity; they do not regenerate it in a new partition.

Canonical `source_event_name` is UTF-8 ASCII `sim:<map_id>:<channel_id>:<instance_id>:<partition_incarnation_id>:<source_event_id>:<tick>`: stable lowercase map ID; decimal channel without leading zeroes (`0` for an instance); lowercase canonical durable instance UUID (`0` for a normal channel); canonical lowercase incarnation UUID; unsigned decimal counter and tick without leading zeroes. Static IDs cannot contain `:`. UUID v5 uses namespace network-order bytes and these exact name bytes (`ids.md`). A source event emits one settlement command per owner/family containing every finalized output for that owner; multiple recipients share the UUID but not the owner-scoped key. Two new incarnations with identical counters/ticks therefore have different operations; replay of one event has the identical operation.
A partition cannot start while PostgreSQL is unavailable (its `world_consequence` recovery read must succeed); entry attempts return `TEMPORARY_DEPENDENCY_FAILURE` and the client retries with backoff.

# Closed Durable Queue Producer Registry (JRN-001..004)

Every admitted command, including work already inside a transaction but not acknowledged, is immutable and encodable by `protobuf_conventions.md` §7 **before** crossing the Durable queue. There is no fallback byte payload or unregistered command kind. The complete launch producer registry is:

| Producer / complete queued family | Typed command | operation_family / owner | Authoritative source and commit outputs |
|---|---|---|---|
| Edge/Sim: every durable C2S request in the closed §7 client expansion, including inventory/loadout/craft/enhance/claims/beasts/entitlements/shops/cosmetics, quest/story/progression/Atlas, chat/report/friend/block/guild/storage, market and client placement/instance boundaries | CLIENT / JournalClientCommand (or its embedded finalized craft/trade/checkpoint snapshot) | Exact §7 family map; CHARACTER authenticated actor, ACCOUNT only for 12 create; original client UUIDv7 | Typed original request + admitted server evidence/RNG; 404/103 COOK freezes separate JournalCraftSnapshot created items/material-instance costs/currency/LIFE_SKILL outputs under owning craft rules, never a CRAFT claim source; complete typed response/assigned IDs/delivered batch/revisions; `durable_command_receipts` admission/terminal proof |
| World/Instance: NORMAL/ELITE kills, instanced/standalone boss reward, dungeon/finale completion, Spirit Surge/event completion | REWARD / JournalRewardCommand kind KILL/BOSS/DUNGEON/EVENT | `sim.<kind lowercase>_settlement` / CHARACTER recipient | Original source incarnation/counter/tick/encounter/generation/chain + immutable revision; finalized loot/new-Soul UUID+initial state/duplicate+Atlas+resonance/sheens/captured Soul revision/character EXP/existing-Soul EXP/Atlas/quest/chivalry/flags/beasts/cosmetics; one owner/family transaction, no omitted subaggregate |
| World/Instance: standalone quest objective/completion/first clue, zone/anchor discovery, spotted/opened chest firsts, Atlas promotion/feat/chivalry/Soul/beast/cosmetic/Guild Stone generated grant not already inside an owning settlement | REWARD / JournalRewardCommand kind QUEST/DISCOVERY/ATLAS/FEAT/CHIVALRY/SOUL/BEAST/COSMETIC/GUILD_STONE | `sim.<kind lowercase>_settlement` / CHARACTER; authored content grant retains `content.<kind lowercase>.grant` and its authored UUIDv5 scope where explicitly one-time | Original standalone source and all finalized outputs, including typed new-Soul acquisition distinct from existing-Soul EXP; **not** a kill alias. Already included kill/dungeon/event suboutputs never enqueue a second logical grant |
| World: bonfire-rest EXP tick (10 s) and Linh Thú bond grant (300 s interval) from `BONFIRE_REST` | REWARD / JournalRewardCommand kind REST (rest EXP + `character_rest_daily` counter) / kind BEAST (bond + `character_beast_bond_daily` counter) | `sim.rest_settlement` and `sim.beast_settlement` / CHARACTER | Original bonfire entity and completed-tick/interval identity + finalized EXP/bond outputs and daily counters; daily-cap rejection is enforced inside the settlement transaction, never by skipping the journal row |
| World/Instance defeat/seasonal completion; running-partition expiry/despawn; Durable relic sweep/startup repair | WORLD_CONSEQUENCE / JournalWorldConsequence | `world.consequence` / WORLD_OWNER_ID | Original relic spawn/expiry and launch defeat metadata; marker-first lock, exclusivity/social-proof rules; no revival from old expiry record |
| World/Instance contribution/defeat/undefeated timeout; process-start eligibility cleanup | BOSS_ELIGIBILITY / JournalBossEligibility | `boss.eligibility` / CHARACTER target | Exact generation + defeated/undefeated channel-copy key and eligibility deadline; DEFEATED fixes enclosing record.content_revision as reward_content_revision and pins it through all slot fallbacks; transition receipt excludes unrelated copies |
| World/Instance PUBLIC chest interaction/timeout; Durable startup fallback | BOSS_CHEST / JournalBossChest (client INTERACT embeds same source/outputs) | `boss.chest` / CHARACTER target; public client103 keeps its CLIENT operation key while source reward uses generation/slot UUIDv5 | Copy eligibility and generation-level slot ledger; fixed personal rolls/claim outputs; first-clear identity separately retained |
| World/Instance checkpoint/logout/disconnect/drain/transfer/instance membership recovery boundaries | CHECKPOINT / JournalCheckpoint (client-owned boundary remains CLIENT, not a synthetic v5 replacement) | `sim.checkpoint` / CHARACTER target; client boundary uses original UUIDv7 family from §7 | Source event + ownership epoch, checkpoint/map/entry, transfer/instance membership/return-source tuple; no whole-character or per-tick-position save |
| Ephemeral Global PUBLIC initialize/open/close/boot-close | PUBLIC_SCHEDULE / JournalPublicSchedule | `boss.schedule` / WORLD_OWNER_ID | Boss/revision/transition natural key; fixed generation UUID, opened/close/next time and sampled delay; single-row schedule transaction before any spawn/despawn |
| Global/Instance competitive PREPARING/ACTIVE/RESOLVING/terminal admission transitions after ready-check | COMPETITIVE_ADMISSION / JournalCompetitiveAdmission | `competitive.admission` / WORLD_OWNER_ID; UUIDv5 server job key `scope:match_id:state` | Original match/season/admitted/deadline/cutoff identity, typed durable RESOLVING snapshot, exact terminal time; priority-0 season marker fence |
| Instance PvP Duel/Arena/Guild War RESOLVING/VOID outcomes; restart reconciliation | MATCH / JournalMatch | `pvp.settlement` / CHARACTER; `guild_war.<settlement_type lowercase>` / GUILD for GUILD_RATING/GUILD_PROGRESSION, CHARACTER for PERSONAL_REWARD/SEASON_PARTICIPATION | Durable match/admission identity, explicit per-recipient result/participation, fixed ratings/revisions/sanctions/reward slots, Monday-week progression/contribution snapshots; no re-evaluation against next season |
| Guild EventSink from boss/dungeon/Surge/bonfire, ritual/vessel/Stone contribution | GUILD_EVENT / JournalGuildEvent | `guild.event` / GUILD target | Source operation/event key + historical guild/membership/occurrence/element/EXP/ritual points/chain or aligned gathering slot; no current-membership reinterpretation |
| Direct-trade finalization after both confirmed | CLIENT 708 with embedded JournalTrade only | `trade.finalise` / CHARACTER initiator, same original client UUIDv7/receipt | Exact typed original708 request, authenticated account/actor/epochs/spatial evidence/fingerprint plus complete two sides/locked quantities/revision/fee/settlement ID; one atomic exchange; no requestless TRADE alternative or replacement intent. Restart cancels ordinary uncommitted OPEN memory, not a published admitted COMMITTING command |
| Character first-attach/session activity/detach lifecycle | ACTIVITY / JournalActivity | `character.activity` / CHARACTER target | Immutable character/session epoch/transition/occurrence; first-attach historical cutoff event and activity projection commit once |
| Chat delivery subsystem's bounded asynchronous moderation-log batches (delivery does not wait for DB) | CHAT_LOG / JournalChatLog, one typed message per batch entry | `chat.log` / CHARACTER sender, UUIDv5 server key message_id | Original authoritative message UUID/sender/scope/text/time; append-once moderation log, retention/erasure-aware terminal disposition; never replay chat fanout |
| Durable/Worker Auction expiry/fallback, quest daily reset/expiry, guild cycle/vote/blessing/storage-claim expiry, competitive cutoff/drain/freeze/delivery, PREPARED erasure resume, persisted payment reconcile/notification, retention/rollup/review, approved compensation | JOB / JournalJob's **closed typed target oneof** | `job.<kind lowercase>` / CHARACTER for quest/compensation; GUILD for guild; ACCOUNT for payment; WORLD_OWNER_ID for auction/season/erasure/maintenance | Exact typed target/window keys in §7 and `ids.md`; persist/reuse finalized source rows, freeze award/catalog/cycle provenance and preserve PREPARED/payment identity. ERASURE_RESUME is only the verified durable intent/PREPARED/fence continuation handoff in data_model.md, never destruction/completion ack; missing source remains unresolved |

This registry also classifies work that **does not** enter Durable: continuous input/movement/combat/target and baseline ack/resync; party/board/ordinary matchmaking/challenge/ready-check prompts and Spirit Surge time scheduling are runtime-only. Any resulting durable competitive admission/sanction/match/placement/reward is COMPETITIVE_ADMISSION/CLIENT/CHECKPOINT/MATCH/JOB above, not the runtime prompt itself. HTTPS auth/password/link/refresh/revocation/rate-limit mutations, raw IAP verification/init/provider ingestion, operator/TOTP/provisioning/config and the SYSTEM alert insert are synchronous transaction/source staging in their owning contracts; they never queue credentials or raw provider/audit text. If they schedule subsequent work, it must be the closed JOB target referencing the already-persisted authoritative source (erasure/payment/compensation/maintenance). This does not exempt a queued producer: unknown mutation types refuse queue admission until the owning codec/family/source/retention contract is extended.

Every server/source command retains its original owner and UUIDv5 natural identity. Generic `operations` outcomes are held for **all** queued/in-flight/journal references, not only client receipts, until terminal disposition; purge enumerates that complete local inventory under the queue lifecycle lock before deletion and workers cannot start before outbox recovery. PUBLIC/content/match natural-key ledgers additionally dedup their own scopes. Direct kill EXP/Soul/objective outputs have no substitute natural ledger: a retained outcome is mandatory for committed duplicate reconstruction. Missing/purged outcome without conclusive owning source proof is a reconciliation failure, never permission to replay a possibly committed kill as new. No second generic server dedup ledger is introduced. ERASURE_RESUME creates no generic erasure-success outcome; its pending intent plus exact object/fence is the canonical continuation proof in data_model.md, retained until actual destruction and cleanup.

Maintenance publication is stronger than crash loss only **after** complete fsync/rename/directory-fsync publication (`deployment.md`). Every published command is reconciled to exact committed outcome, explicit typed terminal nonexecution, or the canonical source-backed ERASURE_RESUME continuation handoff (not erasure completion); an expired uncommitted client receipt is not a successful grant. Abrupt death before publication retains the bounded never-final crash semantics. Receipt/source/content-revision holds, purge and erasure ordering are in `ids.md`; archived immutable content revisions referenced by queued/published commands cannot be removed until every reference is terminal/disposed.

# Content Revision
Store content revision/provenance when required to reconstruct randomized/generated outcomes. `content_revision` is exactly 64 lowercase SHA-256 hexadecimal characters, defined in `content_authoring_contract.md`; PostgreSQL uses `CHAR(64)` with `[0-9a-f]{64}` validation, protobuf/JSON use the same string. Committed result never rerolls/reinterprets because active content revision changes.

# Background Jobs
Auction expiry, resets and cleanup use deterministic job/window keys and idempotent per-target operations under the same transaction rules. Duplicate worker execution must be safe.

# Cleanup
Physical cleanup never removes pending claims/proceeds/escrow, owned items, retry-critical operation outcomes or audit/security evidence before retention policy permits.

# Invariants
~~~
durable success -> DB commit first
no per-tick position save
no blind whole-character autosave
value mutation = atomic + idempotent
lost response != lost/duplicate commit
client save is never online truth
DB outage cannot create memory-only durable success
recovery uses checkpoint/ownership rules
boss aftermath relic state is CHECKPOINT_DURABLE; world_consequence_relics keyed on map_id + channel_id + relic_id (never map_instance_id); crash recovery restores active relics from DB, resolves partition by map_id + channel_id, duration clamped >= 1 second
~~~
