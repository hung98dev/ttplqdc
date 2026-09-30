# Reconnect
status: LOCKED

## Scope
Defines disconnect detection, session replacement, reconnect/resume, simulation attachment, and recovery after process/network failure.

## Detection
A connection is considered lost after the heartbeat/traffic timeout from `protocol.md`:
```text
15s without valid traffic
```

Transport close/error may detect loss earlier.

Loss detection does not itself delete character state or roll back already committed durable operations.

## Session Epoch
At most one authenticated live gameplay session exists per **account** (ADR-0030). At most one character is attached on that session.

A successful newer account login/reconnect:
- creates/increments the account session epoch,
- invalidates old-epoch traffic on every previous connection of that account,
- sends `S2C_SESSION_REPLACED` when feasible,
- preserves one simulation ownership authority for the attached character, if any,
- re-attaches that live character (in the world or inside grace) to the new session exactly like a resume, whether the HELLO carried a resume credential or a gameplay ticket (`S2C_HELLO_OK.resumed_character_id` set, then `S2C_CHARACTER_ATTACH_OK` without `C2S_CHARACTER_ATTACH`; ADR-0069).

The replacing client is not rejected with `CHARACTER_ALREADY_ACTIVE`. Late packets from replaced connections are rejected. After the new session is live, attaching a second character on that session without `C2S_CHARACTER_DETACH` (10) is `CHARACTER_ALREADY_ACTIVE`.


## Resume Credential
Reconnect uses a short-lived server-issued resume credential bound to:
- account/session identity,
- character identity when attached,
- session lineage,
- expiry,
- protocol/build compatibility.

It is not a permanent password/token and cannot be used as arbitrary API authorization.

## Normal-World Grace
Default normal-world reconnect attachment grace:
```text
30s
```

During grace, the simulation may keep the character entity under normal world rules while input is absent. The character receives no client authority and may remain vulnerable where gameplay rules allow.

After grace, normal-world runtime entity may be removed and later restored from authoritative durable/checkpoint/map recovery state.

## Instanced Content
Owning content rules override generic grace when explicitly defined.

Launch dungeon reconnect guarantee remains:
```text
120s same-instance grace
```
as defined in `../02_world/dungeons.md`.

PvP/Guild-War disconnect handling uses their owning gameplay rules; network reconnect does not invent a win/loss exception.

## Reconnect Flow
1. Unity detects transport loss.
2. Client stops treating predicted state as final.
3. Client opens `wss` and sends `C2S_HELLO` with its newest resume credential (from `S2C_HELLO_OK` or the latest `S2C_RESUME_CREDENTIAL` (16), rotated every 300 s, so it is at most 5 minutes old at disconnect); if it expired (`RESUME_EXPIRED`) the client refreshes and requests a new gameplay ticket (`../07_security/auth.md` § HTTPS Endpoints), which bypasses the login queue while a character of the account is live or inside grace (`../07_security/session.md` § Login Queue).
4. Server authenticates and creates a newer valid session epoch.
5. Routing resolves current simulation ownership.
6. If previous live partition is recoverable, attach to it.
7. Otherwise execute owning world/instance recovery policy.
8. Server sends fresh authoritative baseline.
9. Client discards stale local prediction and resumes only after baseline/attach success.

## Process Failure
If the simulation process failed:
- committed PostgreSQL state remains,
- uncommitted transient combat/projectile/AI state is lost,
- dungeon behavior follows canonical restart failure/closure rules,
- normal-world character returns through authoritative recovery/checkpoint logic,
- no client snapshot reconstructs server truth.

## Durable Operation Ambiguity
If the client disconnected after sending a value-affecting operation:
- retry uses the same `operation_id`,
- committed outcome is returned/reconstructed,
- uncommitted operation may execute once,
- RNG/reward/enhancement is never rerolled merely because response was lost.

## Transfer Interruption
Reconnect during map/instance transfer resolves by transfer ownership epoch/record.

Presentation-ready and handoff share `TRANSFER_BUDGET_WORLD = 30s` / `TRANSFER_BUDGET_INSTANCE = 120s` in `../04_architecture/concurrency.md`. Never reactivate both source and destination. Ambiguous handoff uses the canonical transfer recovery path from architecture/world specs.


## Disconnect During Multi-Party State Machines

### Mid-Trade Disconnect
A direct player-to-player trade is a multi-party state machine, not a single-party idempotent operation.

Rules when any trade participant disconnects:
- The trade is **cancelled immediately** regardless of its current stage (offer, confirmation, or finalisation).
- All items involved in the trade return atomically to their owner's inventory; no items change hands.
- Both parties receive `S2C_TRADE_CANCELLED` (or, when the disconnecting party cannot be reached, only the remaining connected party receives it) with error code `TRADE_PARTNER_DISCONNECTED`.
- This cancellation is **not** a retryable durable operation; there is no `operation_id` to retry. Either party may initiate a fresh trade after reconnect.

### Gilded Chest Claim Window After Disconnect
`../02_world/bosses.md` keeps the Gilded Chest interactable for 3 minutes; normal-world reconnect grace is 30 s.

If a character disconnects during an active Gilded Chest claim window:
- Eligibility is server-authoritative and persisted per character in `boss_chest_eligibility` (`../06_data/data_model.md`), not per session or account (ADR-0029).
- Eligibility survives until chest despawn (3-minute window) or claim, whichever comes first.
- The fresh `S2C_WORLD_BASELINE` sent on reconnect includes the Gilded Chest entity with the reconnecting character's eligibility flag intact, provided the chest has not yet despawned.
- If the character reconnects after the chest has despawned, the loot rewards are automatically placed in Reward Claims and are retrievable there as normal.

### Mid-Auction-Session Disconnect
A committed auction purchase is a durable operation and survives disconnection normally via `operation_id` retry of `C2S_AUCTION_BUY` (732).

An in-flight listing or purchase **session** (interaction UI open, listing/purchase form not yet submitted) is not a durable operation:
- If the character disconnects while a listing or purchase form is open but the C2S request has not been sent (or was sent but not yet acknowledged), the session is abandoned.
- On reconnect the client receives no partially-committed listing or purchase for that in-flight session; it must re-open the auction UI and re-enter the listing/purchase.
- If the C2S request was received and committed by the server before disconnect (confirmed by `operation_id` round-trip), the committed result stands and is retrievable via the normal durable-operation retry path.

## Client Backoff
Reconnect attempts use bounded exponential backoff with jitter. The client must not create a tight reconnect loop.

Server-directed retry hints may be honored but never bypass authentication/version/drain gates.

## Maintenance / Drain
When the world process is intentionally draining for a maintenance restart (ADR-0052):
- client receives `S2C_SERVER_DRAINING`,
- new logins and instance creation are refused until the new process is ready,
- clients reconnect after restart through checkpoint recovery,
- durable state is not duplicated to simulate seamlessness.

## Invariants
- one account has one live gameplay session (ADR-0030)
- at most one attached character per account
- old packets cannot regain authority
- normal-world default reconnect grace = 30s
- dungeon same-instance grace = 120s
- reconnect starts from a fresh authoritative baseline
- lost response never means duplicate durable mutation
- client state is never recovery truth
- any participant disconnect cancels direct trade immediately; items return atomically; non-retryable
- Gilded Chest eligibility is server-authoritative and persists through reconnect until despawn or claim
- auction committed purchases are durable; in-flight sessions are not
