# Network Errors
status: LOCKED

## Scope
Defines stable client-visible protocol/session/network error codes, retryability classes, and disconnect policy.

Domain-specific gameplay failures may use typed operation-result errors, but they follow the same retryability principles.

## Error Shape
Client-visible error includes:
```text
error_code
retryability
correlation_id when applicable
retry_after_ms when applicable
safe_display_key or safe_message
```

Do not expose stack traces, SQL text, internal addresses, secrets, raw panic text, or anti-cheat thresholds.

## Retryability
```text
NEVER
RETRY_SAME_OPERATION
RECONNECT
UPDATE_REQUIRED
REAUTHENTICATE
BACKOFF
```

`RETRY_SAME_OPERATION` means reuse the same stable operation ID.

## Canonical Codes
Every error code is listed in a fenced block of this section; the `ErrorCode` enum is numbered from these blocks only, row-major, in document order (`protobuf_conventions.md` § 6, ADR-0069). A new code is appended to the last block.
Connection/protocol:
```text
PROTOCOL_MALFORMED
PROTOCOL_UNSUPPORTED
PROTOCOL_VIOLATION
MESSAGE_UNKNOWN
MESSAGE_TOO_LARGE
MESSAGE_NOT_ALLOWED_IN_STATE
CLIENT_UPDATE_REQUIRED
CONTENT_INCOMPATIBLE
RATE_LIMITED
SERVER_OVERLOADED
SERVER_DRAINING
```

`PROTOCOL_VIOLATION` closes the connection for wire-level misuse: a frame other than one `C2S_HELLO` before `S2C_HELLO_OK`, a server-only field or message ID sent by the client, or an exhausted protocol reject budget. Sequence regression is `STALE_INPUT`, epoch mismatch is `SESSION_EPOCH_STALE`, an unregistered message ID is `MESSAGE_UNKNOWN` (reject without close), a parse failure is `PROTOCOL_MALFORMED`. Order and close rules: `protocol.md` § Envelope Validation.

Authentication/session:
```text
AUTH_REQUIRED
AUTH_INVALID
AUTH_EXPIRED
SESSION_EPOCH_STALE
SESSION_REPLACED
CHARACTER_ALREADY_ACTIVE
RESUME_EXPIRED
ACCOUNT_BANNED
ACCOUNT_PENDING_DELETION
ACCOUNT_SUSPENDED
ENTITLEMENT_REVOKED
USERNAME_INVALID
EMAIL_INVALID
PASSWORD_INVALID
USERNAME_TAKEN
EMAIL_TAKEN
PROVIDER_ALREADY_LINKED
LAST_LOGIN_METHOD
CREDENTIAL_CHANGE_LOCKED
CHARACTER_NAME_INVALID
CHARACTER_NAME_TAKEN
CHARACTER_SLOTS_FULL
```

Password registration errors (ADR-0051; rules in `../07_security/auth.md`): `USERNAME_INVALID`, `EMAIL_INVALID`, `PASSWORD_INVALID` — input fails validation; `USERNAME_TAKEN`, `EMAIL_TAKEN` — key already registered. Retryability: **NEVER** with the same input. Password login failure always returns `AUTH_INVALID`.

Account/character management (ADR-0064; `../07_security/auth.md` § HTTPS Endpoints, `messages.md` IDs 12..13): `PROVIDER_ALREADY_LINKED` (the provider subject is linked to another account), `LAST_LOGIN_METHOD` (unlink would remove the last usable login method), `CREDENTIAL_CHANGE_LOCKED` (account-takeover lock: password change or unlink within 24 h of a risky login needs the current password, `../07_security/anti_cheat.md`), `CHARACTER_NAME_INVALID` (`../01_gameplay/character.md` § Name, `../06_data/text.md`), `CHARACTER_NAME_TAKEN` (`name_key` collision), `CHARACTER_SLOTS_FULL` (3 characters). Retryability: **NEVER** with the same input.

`ACCOUNT_BANNED` is issued when the account has `status = BANNED` at login. Retryability: **NEVER** (until ban is lifted by admin action). `ACCOUNT_PENDING_DELETION` is returned on gameplay attach while the account is in its 7-day deletion window; the client offers only "cancel deletion" (`../07_security/data_protection.md`). `ACCOUNT_SUSPENDED` is issued when the account has `status = SUSPENDED_PAYMENT_RECONCILIATION`. `ENTITLEMENT_REVOKED` is issued when access is denied due to a revoked IAP entitlement (chargeback reconciliation; see `../03_systems/monetization.md`).

A newer login/reconnect replaces the previous connection: the new client proceeds; the old client receives `SESSION_REPLACED` when feasible. Do not return `CHARACTER_ALREADY_ACTIVE` to the replacing client.

`CHARACTER_ALREADY_ACTIVE` applies only when this live session already has an attached character and the request attempts to attach a different character without an authoritative detach.


Routing/simulation:
```text
OWNER_NOT_FOUND
OWNER_CHANGED
TRANSFER_IN_PROGRESS
TRANSFER_FAILED
STALE_INPUT
INVALID_TARGET
INVALID_STATE
MAP_CAPACITY_FULL
```

`MAP_CAPACITY_FULL` is issued when a normal-map entry or channel-switch target cannot be placed because the destination channel is at 18 or all 30 channels of the map are at 18 (`../02_world/world_rules.md`). Retryability: **BACKOFF**. `retry_after_ms = 5000`. A retry uses the same entry intent and a new `operation_id` after that interval. The server does not queue, evict, or silently route to another map. Only player-initiated entries receive this code; server-initiated placements never do (`../02_world/world_rules.md` § Forced Placement, ADR-0061).

Durable operation envelope:
```text
OPERATION_CONFLICT
OPERATION_IN_PROGRESS
OPERATION_REJECTED
TEMPORARY_DEPENDENCY_FAILURE
IAP_RECEIPT_ACCOUNT_MISMATCH
IAP_PRODUCT_MISMATCH
IAP_RECEIPT_INVALID
IAP_SEASON_TRACK_DUPLICATE
IAP_VERIFICATION_PENDING
CHEST_ELIGIBILITY_INVALID
```

`IAP_RECEIPT_INVALID` (definitive negative provider answer: not purchased, cancelled, malformed, wrong app) and `IAP_SEASON_TRACK_DUPLICATE` (a second valid season-track purchase for a season the account already holds) set the entitlement `REJECTED`; retryability **NEVER**. `IAP_VERIFICATION_PENDING` (provider timeout/5xx/ambiguous after retries) keeps it `PENDING`; retryability **RETRY_SAME_OPERATION** with the same `platform_receipt` (`../07_security/validation.md` § IAP Receipt Verification).

`IAP_PRODUCT_MISMATCH` is issued when a platform receipt's product ID does not match any registered product in the IAP catalog. Retryability: **NEVER**. Referenced in `../07_security/validation.md`.

`CHEST_ELIGIBILITY_INVALID` is issued when a chest-open request is rejected because the character does not meet eligibility requirements (e.g. boss participation threshold). Retryability: **NEVER**. Referenced in `../07_security/validation.md`.

`IAP_RECEIPT_ACCOUNT_MISMATCH` is issued when a `platform_receipt` is already bound to a different `account_id`. Retryability: **NEVER**. No entitlement is created for the requesting account.

Trade and Auction specific:
```text
TRADE_PARTNER_DISCONNECTED
TRADE_ELIGIBILITY_LEVEL_REQUIRED
TRADE_ELIGIBILITY_AGE_REQUIRED
AH_ELIGIBILITY_LEVEL_REQUIRED
AH_ELIGIBILITY_AGE_REQUIRED
TRADE_COMMON_BOTH_SIDES
TRADE_PRICE_FLOOR_NOT_MET
AH_PRICE_FLOOR_NOT_MET
SAME_ACCOUNT_FORBIDDEN
```
`TRADE_COMMON_BOTH_SIDES`, `TRADE_PRICE_FLOOR_NOT_MET`: `../03_systems/trading_auction.md`. Retryability: **NEVER** with the same offer.
`TRADE_PARTNER_DISCONNECTED` is issued when a direct trade is cancelled because a participant disconnected. Retryability: **NEVER** — the trade is cancelled non-retryably; offered items never left their owners' inventories and their session locks are released. Both connected parties receive this code.

`TRADE_ELIGIBILITY_LEVEL_REQUIRED` and `TRADE_ELIGIBILITY_AGE_REQUIRED` (ADR-0041): issued when direct trade is initiated with or by a character with level < 10 or age < 24h. Retryability: **NEVER** (until eligibility condition is satisfied).

`AH_ELIGIBILITY_LEVEL_REQUIRED` and `AH_ELIGIBILITY_AGE_REQUIRED` (ADR-0041, ADR-0063): `AH_ELIGIBILITY_LEVEL_REQUIRED` for any auction operation by a character below the Level-15 unlock; `AH_ELIGIBILITY_AGE_REQUIRED` for a listing by a character younger than 24h (`../03_systems/trading_auction.md` § Auction Eligibility Gates). Retryability: **NEVER** (until eligibility condition is satisfied).

`AH_PRICE_FLOOR_NOT_MET`: listing price below the auction floor (`trading_auction.md` § Listing Price Floor). `SAME_ACCOUNT_FORBIDDEN`: direct trade between, or auction purchase of a listing from, characters of the same account. Retryability: **NEVER** with the same input.

Domain operation results (shared by every durable/gameplay `*_RESULT` message; owning specs state which apply):
```text
INVENTORY_FULL            INSUFFICIENT_CURRENCY     CURRENCY_CAP_EXCEEDED
COOLDOWN_ACTIVE           ITEM_NOT_FOUND            ITEM_LOCKED
NOT_OWNER                 PERMISSION_DENIED         TARGET_INVALID
OUT_OF_RANGE              IN_COMBAT                 LEVEL_TOO_LOW
CAPACITY_FULL             ALREADY_OWNED             EXPIRED
STATE_CONFLICT            NOT_IN_SAFE_ANCHOR        DAILY_LIMIT_REACHED
GUILD_DISBAND_BLOCKED     GUILD_STORAGE_SAME_ACCOUNT  GUILD_MEMBERSHIP_TOO_NEW
AUCTION_LISTING_NOT_ACTIVE  CLAIM_WINDOW_CLOSED       DURABLE_BACKPRESSURE
BEAST_NOT_OWNED           SLOT_MISMATCH             SLOT_EMPTY
INSUFFICIENT_ITEM         DAILY_FOOD_CAP_REACHED    MAX_BOND_REACHED
FRIEND_LIMIT_REACHED      TARGET_BLOCKED            ALREADY_FRIENDS
PENDING_REQUEST_EXISTS    CLAIM_CAP_REACHED         CHARM_INELIGIBLE
SKILL_NOT_LEARNED         SKILL_MAX_LEVEL           SKILL_POINTS_INSUFFICIENT
SKILL_LOADOUT_INVALID     POTENTIAL_POINTS_INSUFFICIENT  POTENTIAL_CAP_EXCEEDED
INSUFFICIENT_MP           SOUL_CONTRACT_LIMIT_REACHED    STORY_CHOICE_REQUIRED
NOT_DISCOVERED            GUILD_NAME_TAKEN          GUILD_NAME_INVALID
ATLAS_TIER_NOT_REACHED    CHAT_TEXT_INVALID
```
Meaning of the progression/build codes: `SKILL_NOT_LEARNED` (skill not unlocked yet), `SKILL_MAX_LEVEL` (Basic/Active 12, Passive 6), `SKILL_POINTS_INSUFFICIENT` / `POTENTIAL_POINTS_INSUFFICIENT` (no unspent points), `POTENTIAL_CAP_EXCEEDED` (60% per-stat cap, `../01_gameplay/stats.md`), `SKILL_LOADOUT_INVALID` (wrong skill type, duplicate or unlearned in a slot), `INSUFFICIENT_MP` (skill MP cost), `SOUL_CONTRACT_LIMIT_REACHED` (any `soul_contracts.md` § Contract limit), `STORY_CHOICE_REQUIRED` (dungeon entry while the act-closing MAIN quest is ACTIVE with its branch unset, message 509), `NOT_DISCOVERED` (travel destination not discovered), `CLAIM_CAP_REACHED` (100 PENDING Reward Claims, `../03_systems/reward_claims.md`), `CHARM_INELIGIBLE` (charm stacking or level eligibility, `../03_systems/crafting.md`), `GUILD_NAME_TAKEN` / `GUILD_NAME_INVALID` (`../06_data/text.md`), `ATLAS_TIER_NOT_REACHED` (atlas acknowledge for an unreached tier, `../03_systems/atlas.md`), `CHAT_TEXT_INVALID` (chat text rejected by `../03_systems/social.md` § Message Content).
`DURABLE_BACKPRESSURE` (retry after state refresh) is returned for boss activation, dungeon completion and quest turn-in while a partition is in durable backpressure (`../06_data/save_rules.md`).
Retryability: `STATE_CONFLICT`, `COOLDOWN_ACTIVE` and `DURABLE_BACKPRESSURE` may be retried after the client refreshes state; the others are **NEVER** with the same input. A system needing a new reason adds it to this list in the same change.
## Disconnect Policy
Immediate connection close after response or without response where unsafe:
- malformed envelope that prevents safe parsing,
- hard frame-size violation,
- repeated authentication/session spoofing,
- unsupported protocol major,
- server-enforced session replacement,
- sustained outbound backpressure/slow consumer,
- abuse threshold requiring connection termination.

Do not disconnect for ordinary gameplay rejection such as cooldown, invalid target, inventory full, or insufficient currency.

## Abuse / Escalation
Repeated invalid messages may escalate:
1. reject individual message,
2. throttle/rate-limit,
3. close connection,
4. security system may apply temporary account/IP/device controls under security specs.

A single benign race caused by latency should not be treated as cheating.

## Retry Rules
Examples:
```text
RATE_LIMITED                 -> BACKOFF
SERVER_OVERLOADED            -> BACKOFF (carries retry_after_ms and, for the WORLD_CCU_CAP login queue, queue_position; ADR-0052;
                                also returned by dungeon entry 111 / 113 when instance creation would exceed the partition cap)
MAP_CAPACITY_FULL            -> BACKOFF (retry_after_ms = 5000)
SERVER_DRAINING              -> RECONNECT
AUTH_EXPIRED                 -> REAUTHENTICATE
SESSION_EPOCH_STALE          -> RECONNECT
CLIENT_UPDATE_REQUIRED       -> UPDATE_REQUIRED
TEMPORARY_DEPENDENCY_FAILURE -> RETRY_SAME_OPERATION when operation is idempotent
INVALID_TARGET               -> NEVER for that exact intent
IAP_RECEIPT_ACCOUNT_MISMATCH -> NEVER
```

## Logging
Server logs include internal diagnostic context separately from safe client error:
- trace/correlation ID,
- account/character/session identifiers where policy permits,
- current owner/partition,
- content/protocol version,
- internal cause category.

Sensitive values are redacted.

## Invariants
- stable error code drives client behavior,
- safe client error never leaks internals,
- retryable value mutation reuses operation ID,
- ordinary gameplay rejection does not disconnect,
- malformed/abusive protocol traffic may terminate connection.
