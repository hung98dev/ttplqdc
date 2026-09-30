# Anti-Cheat
status: LOCKED

## Scope
Defines launch anti-cheat signals, evidence, response hooks, and enforcement principles.

## Principle
Primary anti-cheat is server authority.

The design does not depend on trusting Unity memory, client physics, client cooldowns, client RNG, or a client-side anti-cheat product for correctness.

## Prevented by Authority
Server directly prevents or rejects:
- impossible movement/teleport,
- skill use while unavailable/cooldown/resource-invalid,
- fabricated hit/damage/heal/crit,
- fabricated loot/reward/currency,
- duplicated trade/Auction/reward mutation,
- illegal inventory/equipment ownership,
- stale/replayed session input,
- client-selected boss/PvP result.

These should not be converted into post-hoc detection when they can be impossible by design.

## Anomaly Signals
Record bounded signals such as:
- repeated movement corrections beyond tolerance,
- impossible input/action cadence,
- repeated stale/forged entity targets,
- malformed/replayed operation IDs,
- abnormal reconnect/session replacement pattern,
- economic operation conflict/rejection pattern,
- automation-like chat/action cadence,
- impossible client build/protocol tampering.

### Economy Behavioural Signals (Anti-RMT / Gold Farming)

**Net common outflow — 7-day rolling window**
For each account, compute:
```text
outflow_7d     = SUM(common sent via direct trade + common spent on AH purchases)
inflow_7d      = SUM(common received via direct trade + common received as net AH seller proceeds)
net_outflow_7d = outflow_7d - inflow_7d
  over the rolling 7-day window ending at evaluation time
```
Flag for manual economy review when:
```text
net_outflow_7d > 20,000,000 common   AND
account is in the top 1% of net_outflow_7d across all accounts active in that window
```
A flagged account enters `ECONOMY_REVIEW` signal state (`accounts.economy_review_flagged_at`, `../06_data/data_model.md`). This is not a ban; it is a queue entry for the economy/security team to examine the trade graph. The flag clears automatically if net_outflow_7d drops below threshold.

Rationale: 20 M common in 7 days is well above normal player spending rates but reachable by active farmers. Combined with the top-1% percentile gate this minimises false positives from players who are legitimately buying from the AH heavily in one week.

**Trade-partner concentration — 30-day rolling window**
Computed from `economy_character_daily_rollups.trade_partner_volumes` (jsonb map of `{recipient_character_id: amount_sent}`) over the rolling 30-day window:
```text
total_outflow_30d    = total common sent to other characters via direct trade and Auction House purchases (buyer -> seller)
top_3_partners_share = common sent to the three most-received distinct characters
                     / total_outflow_30d
```
Flag for manual economy review when:
```text
total_outflow_30d >= 5,000,000 common   AND
top_3_partners_share >= 0.80            (>= 80% to <= 3 distinct recipients)
```
Rationale: legitimate players buy from many sellers; a farmer routing gold to a small fixed set of mule accounts shows high concentration. The 5 M minimum avoids flagging players who sent a single small gift. The 80% threshold tolerates some partner diversity while still catching tight farming rings.

**Item-transfer concentration — 30-day rolling window**
Computed from `economy_character_daily_rollups.item_partner_counts` (item instances/stack units received per source character via direct trade, Auction purchase, or Guild Storage withdrawal of another character's deposit):
```text
received_items_30d >= 50   AND   share from the top 3 source characters >= 0.80   -> ECONOMY_REVIEW
```

**Automation cadence — rolling session**
```text
combat or gather actions for >= 6 continuous hours (gaps < 5 min)
AND coefficient of variation of inter-action intervals < 0.05 over the last 1,000 actions
-> AUTOMATION_REVIEW (manual queue; no automatic ban)
```

**Account takeover — per login**
```text
login (any provider) whose device_id and /16 IPv4 (/48 IPv6) prefix pair is unseen on the account in 90 days
  (account_login_history.is_new_origin, ../06_data/data_model.md; device_id = auth.md § Device ID, not a fingerprint)
AND a password change or federated unlink within 1 hour of that login
-> revoke all other session families, set accounts.credential_guard_until = now + 24 h (credential changes then need the
   current password, else CREDENTIAL_CHANGE_LOCKED), emit ACCOUNT_SECURITY_REVIEW
```
Thresholds are runtime security config; the values above are launch defaults.

A signal is not automatically proof of cheating.

## Evidence
Permanent punitive action should rely on reconstructable evidence:
- stable account/character/session identifiers,
- server timestamps,
- relevant authoritative inputs/results,
- rule/invariant violated,
- content/protocol/build version,
- repeated pattern where appropriate.

Do not permanently ban solely because one packet arrived late, one prediction diverged, or one network burst exceeded normal cadence.

## Response Levels
Possible responses:
1. silently reject illegal operation,
2. authoritative correction,
3. rate limit/throttle,
4. disconnect current session,
5. temporary security restriction/review flag,
6. account enforcement under operator/policy process.

Gameplay code exposes enforcement hooks but does not hide irreversible punishment inside a random validation branch.

## Client Integrity
Optional client integrity/device attestation may be added as a risk signal later.

It must not become the sole authority for valuable state and must not block supported platforms without an explicit product decision.

## Honey / Secret Rules
Do not send hidden detection thresholds, server-only RNG state, or internal anti-cheat scoring to Unity.

Avoid security-by-obscurity as the only defense.

## False Positive Control
Telemetry separates:
- ordinary invalid action from latency/race,
- repeated impossible behavior,
- confirmed value duplication attempt,
- malformed exploit traffic.

Enforcement thresholds are versioned/configurable and reviewed against real network conditions.

## Audit
Security enforcement records operator/automated source, reason category, evidence reference, scope, start/end where temporary, and appeal/support reference where applicable.

## Invariants
- server authority prevents valuable cheats where possible,
- anomaly != guilt,
- permanent enforcement requires reconstructable evidence,
- client anti-cheat is optional defense-in-depth,
- one bad network packet is not a ban condition,
- security thresholds are not replicated to client,
- net common outflow > 20,000,000 common in 7 days AND top-1% percentile -> ECONOMY_REVIEW flag,
- trade-partner concentration >= 80% to <= 3 recipients over 30 days with >= 5,000,000 total outflow -> ECONOMY_REVIEW flag.
