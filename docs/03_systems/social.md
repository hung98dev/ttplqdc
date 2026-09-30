# Social
status: LOCKED

## Scope
Defines friends, blocks, presence, chat, whispers, reports, and moderation hooks.

## Identity
Social identity is character-scoped:
```text
character_id
```

Account IDs are never exposed through normal social APIs.

## Friends
Friendship is mutual:
```text
A <-> B
```

Friend-list capacity:
```text
100 friends per character
```

Outgoing pending friend requests (ADR-0065 completion):
```text
100 PENDING requests per requester (CAPACITY_FULL)
```

Friend request lifetime:
```text
7 days
```

Request states:
```text
PENDING
ACCEPTED
DECLINED
CANCELLED
EXPIRED
```

Crossed requests do not create duplicates. If A already has a pending request to B and B sends one to A, the server treats B's action as acceptance of the existing request.

Either side may remove the friendship without approval from the other.

## Blocking
Block is directional:
```text
A blocks B
```

If either side blocks the other, direct interaction between the pair is denied for:
- friend requests
- whispers
- party invites
- guild invites
- direct trade requests
- other direct person-to-person invitations

Creating a block atomically removes any existing friendship and invalidates pending friend requests between the pair.

Block does **not** automatically remove either player from an existing party, guild, dungeon, world map, boss, or PvP match.

Unblocking does not restore old friendship/invitations.

Block-list capacity: `500` blocked characters per blocker (`CAPACITY_FULL`).

Blocked-authored player chat is filtered from the blocker when practical. SYSTEM messages are never filtered by player block.

## Presence
Public friend presence:
```text
ONLINE
OFFLINE
```

Disconnect grace before advertising OFFLINE:
```text
30s
```

Friends may additionally see:
```text
zone/map display name
party status: IN_PARTY / NOT_IN_PARTY
activity: WORLD / DUNGEON / PVP
```

Do not expose:
- exact coordinates
- hidden `map_instance_id`
- private encounter state
- account/network identifiers

A player may later receive privacy settings, but no hidden location is exposed by default.

## Chat Channels
Canonical:
```text
LOCAL
WORLD
PARTY
GUILD
WHISPER
SYSTEM
```

`SYSTEM` may only be authored by trusted server code. LOCAL delivery is World/Instance Simulation. WORLD / PARTY / GUILD / WHISPER fanout is the ephemeral global runtime in `../04_architecture/service_boundaries.md`.


## LOCAL
Scope:
```text
same map_instance_id
within 25m equivalent distance
```

## WORLD
Scope:
```text
same logical game world
```

WORLD chat is not cross-world/cross-region.

Initial eligibility:
```text
character level >= 10
```

No currency/item fee is charged for WORLD chat.

## PARTY / GUILD
Sender must be a current member of the referenced party/guild.

Recipients are current members after block/moderation filtering.

## WHISPER
Direct online character-to-character message.

Rules:
- sender != target
- target must be ONLINE
- no block in either direction
- moderation restrictions pass

Offline whisper storage is not enabled.

## Message Content
Maximum visible Unicode grapheme clusters:
```text
240
```

Validation:
- canonical UTF-8/NFC/grapheme behavior follows `../06_data/text.md`,
- trim leading/trailing Unicode whitespace,
- empty-after-trim rejected,
- control characters rejected except explicitly supported newline behavior,
- untrusted client markup is escaped/treated as plain text,
- server grapheme count is authoritative; Unity-side count is UX only

## Rate Limits
Per-character baseline send limits:
```text
LOCAL:   5 messages / 10s
WORLD:   2 messages / 10s
PARTY:  10 messages / 10s
GUILD:  10 messages / 10s
WHISPER: 5 messages / 10s per target
```

Short bursts may be accepted within these windows; exceeding the limit is throttled/rejected.

Repeated identical messages receive stricter spam throttling.

Friend/party/guild invite creation is also rate-limited by server security configuration.

## Chat History
Server may retain recent chat for moderation/operations.

Gameplay clients receive only bounded recent history when supported:
```text
LOCAL/WORLD: no guaranteed reconnect history
PARTY/GUILD: up to recent 50 messages while the group context remains valid
WHISPER: no offline history guarantee
```

Moderation chat logs persist in `chat_messages` (`../06_data/data_model.md`) with 90-day retention (`../07_security/data_protection.md`).

## Moderation
Canonical communication restriction:
```text
NONE
MUTED
```

A mute may target all player-authored channels or an explicit subset and has server-authoritative expiry.

Submitting a report never automatically mutes/bans/removes the target.

Automated filtering may reject unsafe/spam content, but punitive sanctions require explicit moderation policy/action.

## Reports
Stable:
```text
report_id
```

Canonical reason IDs:
```text
SPAM
HARASSMENT
HATE_OR_ABUSE
CHEATING
SCAM
INAPPROPRIATE_NAME
OTHER
```

A report may reference `chat_message_id` plus optional reporter notes (at most 200 graphemes, same text rules as chat).

Duplicate reports by the same reporter against the same target/reason are rate-limited.

Baseline report limit:
```text
10 submissions / 24h per account
```

Security/moderation may override for abuse prevention.

## Shared Direct-Interaction Gate
Systems should reuse:
```text
can_direct_interact(source, target)
```

At minimum validates:
```text
source != target where required
target exists
no block either direction
source/target restrictions for the requested interaction
```

Party, guild invite, and direct trade must use this gate.

Once an atomic trade is already `COMMITTING`, a newly created block does not split/reverse that transaction.

## Persistence
Persist:
```text
friendships
blocks
pending friend requests
moderation state where required
```

Presence is runtime-derived.

## Concurrency
Invalid final state:
```text
friends AND blocked for the same pair
```

Block wins over friendship in a concurrent race.

At most one friendship exists per unordered pair and one directional block per ordered pair.

## Design Guardrails
- Useful social tools without requiring social media-like complexity.
- No offline mailbox/voice/custom rooms initially.
- Block consistently prevents bypass through other direct systems.
- Public location exposure stays coarse.
- WORLD chat has a small level gate to reduce fresh-account spam, not a monetary gate.

## Chivalry System (Điểm Hiệp Nghĩa)
The Chivalry system rewards veteran players for mentoring newcomers under ADR-0023:
- **Eligibility**: A character of `Level >= 40` completing an Act I/II (T1/T2) normal dungeon in a party containing at least one novice character of `Level <= 25`.
- **Earning**: Awards `15 chivalry_points` per qualifying dungeon completion, up to a daily character cap of `100 points`. The grant is **clamped**: if the current UTC-day counter is already at or above `100`, no points are awarded; if the counter is below `100` but adding 15 would exceed it, only enough points to reach exactly `100` are awarded (e.g., at 90 points the 7th completion grants 10, not 15). Persist both the UTC-day counter and lifetime total; duplicate completion settlement cannot increment either twice.
- **Non-Currency Rule**: `chivalry_points` is an authoritative non-spendable progression score, not a fourth currency (`economy.md`). It cannot be debited, exchanged, transferred, or converted.
- **Rewards**:
  - Milestone Titles (character-scoped, ADR-0029):
    - 500: `cosmetic.title.trang_si_giup_doi` — Tráng Sĩ Giúp Đời
    - 2,000: `cosmetic.title.dai_hiep_lang_que` — Đại Hiệp Làng Quê
    - 5,000: `cosmetic.title.hiep_nghia_vo_song` — Hiệp Nghĩa Vô Song (title aura)
  - Each milestone grants its listed cosmetic entitlement directly once; it grants no items, currency, combat power, enhancement support, or Linh Thú food.

## Invariants
```text
social identity = character_id
friendship = mutual
block = directional
block either direction -> direct interaction denied
friends and block cannot coexist
SYSTEM cannot be player-authored
friend cap = 100; outgoing pending requests = 100; block list = 500
message max = 240 graphemes
```
