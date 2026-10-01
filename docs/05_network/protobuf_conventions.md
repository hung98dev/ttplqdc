# Protocol Buffers Conventions & Wire Baseline
status: LOCKED

## Scope

Quy chuẩn kỹ thuật cấu trúc Protobuf v3, sơ đồ phân tách file trong `proto/thinhthan/v1/`, quy tắc định số trường (field numbering), khung truyền tải (wire envelope), và cơ chế tương thích ngược (backward compatibility) cho toàn bộ message IDs đăng ký trong `messages.md` (ADR-0054).

Tài liệu này là hợp đồng wire ràng buộc giữa Go server và Unity client (C# 9.0).

## 1. Package & Namespace Layout

Mọi file **network** `.proto` trong baseline 9 file phải nằm trong thư mục `proto/thinhthan/v1/` và tuân thủ khai báo chuẩn dưới đây; schema journal Go-only nội bộ ở §7 có package/path riêng, không phải network baseline:

```protobuf
syntax = "proto3";

package thinhthan.v1;

option go_package = "thinhthan/internal/protocol/v1;protocolv1";
option csharp_namespace = "ThinhThan.Protocol.V1";
```

### Danh mục 9 file Proto Baseline

| File Proto | Message IDs | Nội dung chính |
|---|---|---|
| `common.proto` | N/A (Shared Types) | `ErrorCode`, `Retryability`, `ResultStatus`, `OperationResult`, `ItemQuantity`, `ItemGrant`, `CurrencyDelta`, `PositionMm`, `CharacterSummary`, `EntityState`, `RewardClaimView` (§ 6) |
| `session.proto` | 1 .. 16 | Hello, Attach, Detach, SessionReplaced, Heartbeat, ServerDraining, Error, CharacterCreate, CharacterList, PlacementPending, ResumeCredential (16; authentication itself is HTTPS, `../07_security/auth.md`) |
| `movement.proto` | 100 .. 117 | C2S_MOVEMENT_EDGE (108), C2S_CHANNEL_SWITCH (109), S2C_CHANNEL_SWITCH_RESULT (110), dungeon entry/exit (111..115, 117), S2C_INTERACT_RESULT (116), PositionSnapshot, Velocity, Knockback |
| `combat.proto` | 200 .. 208, 300 .. 308 | CombatAction, HitResult, world replication (baseline, spawn/despawn, state delta, baseline ack/resync 307..308), S2C_COMBAT_EVENT (304), StatusEffectDelta |
| `durable.proto` | 400 .. 443 | InventoryMutate, Loadout (equip/skill/soul contract), Craft, Enhance, RewardClaim, Beast operations (410..417, 430..431), EntitlementClaim (418..419), NpcShop buy/sell (420..421, 426..427), Cosmetic redeem/equip (422..425), InventoryExpand (428..429), state pushes (432..438), reward-claim paging (439..441), Soul Collection paging (442..443) |
| `content.proto` | 500 .. 518 | Quest accept/turn-in/abandon, AtlasClaim, ProgressionEvent, story branch, skill upgrade, potential allocate, respec, ProgressionState, Daily Board request/state (516..517), Atlas state (518) |
| `social.proto` | 600 .. 656 | Friend, Block, Chat (World/Party/Guild/Whisper), Party, Guild (incl. create/disband/applications/storage claims/blessing vote), Guild cosmetic selection (656) |
| `market.proto` | 700 .. 744 (incl. 710) | Direct Trade, Auction list/buy/cancel/search/reclaim/proceeds |
| `pvp.proto` | 800 .. 819 | Sparring, Duel, Five Element Arena, Guild War match states |

Ranges describe file ownership, not allocation: only IDs registered in `messages.md` exist; reserved/retired gaps are never generated or dispatched. Every registered network ID has exactly one owner in this nine-file table. The nine previously omitted IDs keep their numbers: 16 → `session.proto`; 307/308 → `combat.proto`; 442/443 → `durable.proto`; 516/517/518 → `content.proto`; 656 → `social.proto`.

## 2. Wire Envelope Specification

Mọi thông điệp trao đổi qua kết nối WebSocket (WSS) đều được bọc trong một khung truyền tải chuẩn `Envelope`:

```protobuf
message Envelope {
  uint32 protocol_major  = 1;  // major version; mismatch closes connection
  uint32 protocol_minor  = 2;  // minor version; additive-compatible
  uint32 message_id      = 3;  // registry: docs/05_network/messages.md
  uint64 session_epoch   = 4;  // must match current authenticated session; older epoch → rejected
  uint64 client_seq      = 5;  // C→S only; absent (0) in S→C
  uint64 server_seq      = 6;  // S→C only; absent (0) in C→S
  uint64 correlation_id  = 7;  // request/response correlation; absent (0) when not applicable
  bytes  payload         = 8;  // serialised message body for message_id
}
```

Canonical field authority is `../05_network/protocol.md`. This protobuf layout must remain consistent with that spec; if they diverge, `protocol.md` governs.

### Quy tắc Ánh xạ Envelope & Payload
- Quan hệ giữa `message_id` và kiểu message trong `payload` là **ánh xạ 1-1 tất định** (deterministic 1:1 mapping).
- Server và client duy trì bảng tra cứu registry; nhận được `message_id` nào thì bắt buộc deserialize `payload` theo đúng schema của message đó.
- Nếu `message_id` không tồn tại trong registry: trả `S2C_ERROR` với `MESSAGE_UNKNOWN`, không dispatch, không đóng kết nối (vượt ngân sách từ chối mới đóng; `protocol.md` § Envelope Validation).
- `session_epoch` được kiểm tra trước khi dispatch payload; lệch epoch → `SESSION_EPOCH_STALE` rồi đóng kết nối (`protocol.md` § Envelope Validation).

## 3. Quy tắc Đánh số Field & Tương thích Thêm mới (Additive Evolution)

1. **Field Numbering Ban đầu:**
   - Khi khởi tạo file proto, các field được đánh số tuần tự bắt đầu từ `1`, `2`, `3`...
   - Các trường hay xuất hiện nhất (hot path) phải mang số field từ `1` đến `15` (chiếm 1 byte encoding trong protobuf).
2. **Bất biến sau khi commit:**
   - Một khi file `.proto` đã được commit vào nhánh chính, số thứ tự field (`field number`) là **bất biến vĩnh viễn**.
   - Cấm đổi số field, cấm đổi kiểu dữ liệu của field hiện có.
3. **Thu hồi field (Deprecation & Reservation):**
   - Nếu một field không còn được sử dụng, không được xóa hẳn mà phải đổi tên thành `deprecated_<name>` hoặc chuyển sang khối `reserved`:
     ```protobuf
     reserved 4, 7 to 9;
     reserved "old_field_name";
     ```
   - Cấm tái sử dụng số field đã reserve cho bất kỳ trường mới nào khác.
4. **Presence Semantics:**
   - Các trường kiểu số hoặc chuỗi mặc định dùng implicit presence của proto3 (giá trị 0 / chuỗi rỗng = không truyền).
   - Nếu cần phân biệt giữa "giá trị 0" và "không có giá trị", trường phải được khai báo với từ khóa `optional`.

## 4. Quy chuẩn Enum Values

- Giá trị đầu tiên của mọi enum bắt buộc phải là `0` và mang hậu tố `_UNSPECIFIED`:
  ```protobuf
  enum CharacterClass {
    CHARACTER_CLASS_UNSPECIFIED = 0;
    CHARACTER_CLASS_KIM = 1;
    CHARACTER_CLASS_MOC = 2;
    CHARACTER_CLASS_THUY = 3;
    CHARACTER_CLASS_HOA = 4;
    CHARACTER_CLASS_THO = 5;
  }
  ```
- Tên giá trị enum viết HOA kiểu `SCREAMING_SNAKE_CASE` và có tiền tố là tên của enum để tránh đụng độ namespace trong C++.

## 5. Codegen & Parity Testing

1. **Mã sinh tự động (Generated Code):**
   - Target Go: `server/internal/protocol/v1/`
   - Target C#: `client/Assets/Scripts/Protocol/`
   - Cấm sửa tay bất kỳ dòng code nào trong hai thư mục trên.
2. **Codegen Scripts:**
   - `pwsh -NoProfile -File scripts/codegen.ps1` trên Linux và Windows (script duy nhất; ADR-0050 bỏ wrapper Bash, ADR-0058 chạy bằng PowerShell 7)
   - Script phải sử dụng đúng phiên bản `protoc 36.2` và `protoc-gen-go v1.36.12` theo `docs/00_context/technology_versions.md`.
3. **Kiểm tra lệch mã sinh (Codegen Drift Check):**
   - Trong CI (job Linux và Windows) và `verify.ps1`, script sẽ chạy lệnh codegen và kiểm tra `git status --porcelain`.
   - Bất kỳ sự khác biệt nào giữa mã nguồn đã commit và mã sinh mới đều khiến bài test thất bại ngay lập tức (exit code 1).
4. **Golden Binary Fixtures:**
   - Bộ fixture nhị phân mẫu được lưu tại `proto/testdata/golden/*.bin`.
   - Cả bộ test Go (`server/internal/testing/protocol/`) và C# (`client/Assets/Tests/EditMode/`) phải cùng đọc các file này và xác nhận parse ra cùng một giá trị trường chính xác 100%.

## 6. Wire Scalar Types (ADR-0064)

Một cách biểu diễn duy nhất cho mỗi loại giá trị; `messages.md` dùng các tên logic dưới đây:
```text
logical type        proto3 type        rule
UUID                bytes              exactly 16 bytes (RFC 4122/9562 binary, network order); empty = absent;
                                       any other length -> PROTOCOL_MALFORMED. Never a string on the wire.
runtime entity ID   uint64             partition-scoped (entity_id, action_instance_id, event_id, encounter_id)
content ID          string             canonical dotted ID (../06_data/ids.md); max 128 bytes
timestamp           int64              Unix epoch milliseconds, UTC; every wall-clock field (`*_at`, `*_at_ms`,
                                       `timestamp`, `expires_at`, `server_time_ms`) uses this type
duration            uint32             unit in the field name (`_ms`, `_seconds`)
tick                uint64             simulation tick (`*_tick`, `server_tick`)
position/velocity   sint32             millimetres / millimetres per second (`*_mm`, `*_mm_s`)
money/count         int64 / uint32     currency amounts int64; quantities, levels, slots uint32; never float
ratio               uint32             basis points (10000 = 1.0)
error_code          ErrorCode enum     see below
status              ResultStatus enum  RESULT_STATUS_UNSPECIFIED = 0, _SUCCESS = 1, _ERROR = 2
```

`ErrorCode` (`common.proto`): `ERROR_CODE_UNSPECIFIED = 0` means `NONE` (success). Every other code listed in `errors.md` gets `ERROR_CODE_<CODE>` with an explicit number. The baseline (`IMP-061`) numbers the codes 1..N in order of first appearance of each `[A-Z][A-Z0-9_]{2,}` token inside the fenced code blocks of `errors.md` § Canonical Codes, blocks in document order, each block read row-major (left to right, then top to bottom); prose is never scanned, so every error code must appear in one of those fenced blocks (the baseline has 112); from then on `common.proto` is the numbering authority: new codes append with the next unused number, numbers are never reused or reordered, and a parity test checks that every code in `errors.md` has exactly one enum value. `Retryability` mirrors `errors.md` § Retryability (`RETRYABILITY_UNSPECIFIED = 0`, then NEVER .. BACKOFF in listed order).

Shared messages (`common.proto`):
```text
OperationResult   operation_id (UUID), status, error_code
ItemQuantity      item_id, quantity
ItemGrant         item_instance_id (UUID), item_id, quantity
CurrencyDelta     currency_id, amount (int64, signed)
PositionMm        x_mm, y_mm
CharacterSummary  messages.md ID 14
EntityState       messages.md § Replication
RewardClaimView   messages.md ID 434
```
Every `*_RESULT` message embeds `OperationResult` as field 1. State events that report an outcome without an operation are named `*_OUTCOME` (e.g. 813 `S2C_SPARRING_OUTCOME`, 818 `S2C_DUEL_OUTCOME`) and never carry `OperationResult`. Attach (6), detach (10) and HELLO (1) are answered by their `*_OK` message or `S2C_ERROR` (3) (ADR-0069).

## 7. Internal Durable Journal Schema (ADR-0070, ADR-0079)

The nine files above remain the **network** baseline. Journal records are not envelopes and receive no network IDs. `IMP-061` owns the planned additional source `proto/thinhthan/internal/v1/durable_journal.proto`, package `thinhthan.internal.v1`, `option go_package = "thinhthan/internal/durable/journal/v1;journalv1"`, generated Go output `server/internal/durable/journal/v1/durable_journal.pb.go`. It imports the network files; no network file imports it. Codegen generates **Go only** for this source, no C# output, new executable or RPC. This avoids a `durable.proto` ↔ content/social/market/session import cycle while reusing concrete network request types. Requirement authority: JRN-001..010 in `../08_scale_ops/deployment.md`.
Soul acquisition batch semantics additionally satisfy JRN-011 in `../03_systems/soul_contracts.md` § Atomic Acquisition Batch.

The following declarations plus the deterministic expansions below are the complete baseline field-number authority. UUID fields are `bytes`, exactly 16 RFC 9562 network-order bytes; fingerprints are exactly 32 SHA-256 bytes; content revisions are strings of exactly 64 lowercase hexadecimal characters. Required fields are validated even though proto3 has no `required` syntax. Message-valued fields are required unless declared optional. Unknown fields/enums, duplicate singular fields/oneof members, missing required fields and type/family/owner mismatch are corruption, not additive compatibility. Encoding is deterministic protobuf, ascending field order, shortest varints, no unknown fields; decode/re-encode must reproduce the bytes. No `Any`, `Struct`, payload bytes, arbitrary JSON, SQL or arbitrary table patches are permitted.

The closed producer union has **13 command kinds**. Trade finalization is always CLIENT request 708 with its embedded `JournalTrade`; there is no standalone TRADE producer. Retired command enum number 11 and record field 30 remain reserved; all other existing numbers are unchanged.

```protobuf
enum JournalCommandType {
  JOURNAL_COMMAND_TYPE_UNSPECIFIED = 0;
  JOURNAL_COMMAND_TYPE_CLIENT = 1;
  JOURNAL_COMMAND_TYPE_REWARD = 2;
  JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE = 3;
  JOURNAL_COMMAND_TYPE_BOSS_ELIGIBILITY = 4;
  JOURNAL_COMMAND_TYPE_BOSS_CHEST = 5;
  JOURNAL_COMMAND_TYPE_CHECKPOINT = 6;
  JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE = 7;
  JOURNAL_COMMAND_TYPE_MATCH = 8;
  JOURNAL_COMMAND_TYPE_GUILD_EVENT = 9;
  JOURNAL_COMMAND_TYPE_JOB = 10;
  reserved 11;
  JOURNAL_COMMAND_TYPE_ACTIVITY = 12;
  JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION = 13;
  JOURNAL_COMMAND_TYPE_CHAT_LOG = 14;
}
enum JournalOwnerKind {
  JOURNAL_OWNER_KIND_UNSPECIFIED = 0;
  JOURNAL_OWNER_KIND_ACCOUNT = 1;
  JOURNAL_OWNER_KIND_CHARACTER = 2;
  JOURNAL_OWNER_KIND_GUILD = 3;
  JOURNAL_OWNER_KIND_WORLD = 4;
}
message DurableCommandRecord {
  uint32 schema_version = 1;
  string operation_family = 2;
  JournalOwnerKind owner_kind = 3;
  bytes owner_id = 4;
  bytes operation_id = 5;
  JournalCommandType command_type = 6;
  int64 enqueued_at_ms = 7;
  string content_revision = 8;
  bytes request_fingerprint = 9;
  uint64 admission_sequence = 10;
  bytes producer_incarnation_id = 11;
  reserved 30;
  oneof command {
    JournalClientCommand client = 20;
    JournalRewardCommand reward = 21;
    JournalWorldConsequence world_consequence = 22;
    JournalBossEligibility boss_eligibility = 23;
    JournalBossChest boss_chest = 24;
    JournalCheckpoint checkpoint = 25;
    JournalPublicSchedule public_schedule = 26;
    JournalMatch match = 27;
    JournalGuildEvent guild_event = 28;
    JournalJob job = 29;
    JournalActivity activity = 31;
    JournalCompetitiveAdmission competitive_admission = 32;
    JournalChatLog chat_log = 33;
  }
}
message JournalSource {
  string map_id = 1; uint32 channel_id = 2; optional bytes instance_id = 3;
  bytes partition_incarnation_id = 4; uint64 source_event_id = 5; uint64 tick = 6;
  int64 occurred_at_ms = 7; string source_content_id = 8;
  optional bytes encounter_instance_id = 9; optional bytes generation_id = 10; optional bytes chain_id = 11;
  optional uint64 rng_seed_hi = 12; optional uint64 rng_seed_lo = 13;
}
message JournalStat { string stat_id = 1; sint64 value = 2; uint32 scale = 3; }
message JournalPity { uint32 target_level = 1; uint32 fail_count = 2; }
message JournalItem {
  string item_id = 1; uint64 quantity = 2; string effective_binding = 3; string content_revision = 4;
  repeated JournalStat base_rolls = 5; repeated JournalStat secondary_rolls = 6;
  uint32 enhancement = 7; repeated JournalPity pity = 8; optional bytes item_instance_id = 9;
  optional string soul_id = 10; uint32 soul_level = 11; uint64 soul_exp = 12;
  optional int64 created_at_ms = 13;
}
message JournalCurrency { string currency_id = 1; int64 amount = 2; }
message JournalSoulExp { bytes soul_instance_id = 1; uint64 amount = 2; }
message JournalObjective { string quest_id = 1; uint32 objective_index = 2; uint64 delta = 3; }
message JournalAtlas { string page_id = 1; uint64 delta = 2; }
message JournalFlag { string flag_id = 1; string value = 2; }
message JournalSoulResonance {
  uint32 memory_count_before = 1; uint32 memory_count_after = 2;
  optional int64 sheen_unlocked_before_ms = 3; optional int64 sheen_unlocked_after_ms = 4;
}
message JournalSoulAcquisition {
  bytes soul_instance_id = 1; string soul_id = 2; uint32 initial_level = 3; uint64 initial_soul_exp = 4;
  bool duplicate = 5; JournalAtlas atlas_progress = 6; bool atlas_page_mastered_before = 7;
  optional JournalSoulResonance resonance = 8; uint64 expected_soul_revision = 9;
}
message JournalRewardSlot {
  string reward_slot = 1; repeated JournalItem items = 2; repeated JournalCurrency currencies = 3;
  uint64 character_exp = 4; repeated JournalSoulExp soul_exp = 5; repeated JournalObjective quest_progress = 6;
  repeated JournalAtlas atlas_progress = 7; uint32 chivalry_points = 8; repeated string beast_ids = 9;
  repeated string cosmetic_ids = 10; repeated JournalFlag flags = 11;
  string source_type = 12; string source_reference = 13; bytes source_reward_operation_id = 14;
  repeated JournalSoulAcquisition soul_acquisitions = 15;
}
message JournalRewardCommand {
  optional JournalSource source = 1; string kind = 2; repeated JournalRewardSlot slots = 3;
  optional string quest_id = 4; optional string discovery_id = 5; optional string dungeon_id = 6;
  optional string run_tag = 7; optional uint32 season_id = 8;
  optional string grant_scope = 9;
}
message JournalWorldConsequence {
  optional JournalSource source = 1; string map_id = 2; uint32 channel_id = 3; string relic_id = 4;
  string region_id = 5; string marker_boss_id = 6; string source_id = 7; string buff_effect_id = 8;
  int64 spawned_at_ms = 9; int64 expires_at_ms = 10; string transition = 11;
  optional bytes defeated_encounter_instance_id = 12; optional uint32 defeated_participant_count = 13;
  optional int64 defeated_at_ms = 14;
}
message JournalBossEligibility {
  optional JournalSource source = 1; bytes character_id = 2; bytes generation_id = 3;
  string boss_id = 4; string copy_map_id = 5; uint32 copy_channel_id = 6;
  string transition = 7; optional int64 eligible_until_ms = 8;
}
message JournalBossChest {
  optional JournalSource source = 1; bytes character_id = 2; bytes generation_id = 3; string boss_id = 4;
  string copy_map_id = 5; uint32 copy_channel_id = 6; string reason = 7;
  repeated JournalRewardSlot slots = 8;
}
message JournalCheckpoint {
  bytes character_id = 1; uint64 ownership_epoch = 2; string checkpoint_id = 3;
  string safe_map_id = 4; string entry_spawn_id = 5; optional bytes transfer_id = 6;
  optional bytes instance_id = 7; optional string instance_dungeon_id = 8; optional string instance_run_tag = 9;
  optional string source_map_id = 10; optional uint32 source_channel_id = 11;
  string membership_state = 12; int64 recorded_at_ms = 13; uint64 source_tick = 14;
  optional JournalSource source = 15;
}
message JournalPublicSchedule {
  string boss_id = 1; uint64 expected_revision = 2; string transition = 3;
  optional bytes prior_generation_id = 4; optional bytes new_generation_id = 5;
  optional int64 opened_at_ms = 6; optional int64 next_spawn_at_ms = 7;
  int64 transition_at_ms = 8; optional uint32 respawn_delay_seconds = 9;
}
message JournalParticipant {
  bytes character_id = 1; optional bytes guild_id = 2; string participation = 3;
  uint32 team_index = 4; uint32 active_seconds = 5; uint32 disconnected_seconds = 6; uint32 afk_seconds = 7;
  bool reward_eligible = 8; sint32 mmr_before = 9; sint32 mmr_after = 10;
  sint32 season_rating_before = 11; sint32 season_rating_after = 12; uint64 expected_rating_revision = 13;
  uint32 abandon_penalty = 14; optional uint32 bound_slot = 15; optional int64 bound_window_start_ms = 16;
  repeated JournalRewardSlot slots = 17; optional bytes sanction_id = 18; optional uint32 sanction_ladder_step = 19;
  optional int64 sanction_starts_at_ms = 20; optional int64 sanction_ends_at_ms = 21;
  string result = 22;
  optional bytes membership_id = 23;
  uint64 guild_contribution = 24;
}
message JournalMatch {
  bytes match_id = 1; string mode_id = 2; uint32 season_id = 3; string match_state = 4;
  string result = 5; string settlement_type = 6; repeated JournalParticipant participants = 7;
  int64 resolved_at_ms = 8; JournalSource source = 9;
  repeated JournalGuildMatchOutput guild_outputs = 10;
}
message JournalGuildMatchOutput {
  bytes guild_id = 1; string result = 2; uint64 expected_rating_revision = 3;
  sint32 mmr_before = 4; sint32 mmr_after = 5;
  optional int64 progression_week_monday_ms = 6; optional uint32 progression_slot = 7;
  uint64 guild_exp = 8; repeated JournalGuildContribution contributions = 9;
}
message JournalGuildContribution { bytes character_id = 1; bytes membership_id = 2; uint64 amount = 3; }
message JournalGuildEvent {
  bytes guild_id = 1; bytes character_id = 2; bytes membership_id = 3; bytes source_operation_id = 4;
  string source_kind = 5; string source_reference = 6; int64 occurred_at_ms = 7;
  uint32 element = 8; uint32 ritual_points = 9; uint64 guild_exp = 10;
  optional bytes chain_id = 11; optional int64 gathering_slot_start_ms = 12; uint32 season_id = 13;
  repeated JournalGuildContribution credited_members = 14;
  string cycle_id = 15; uint64 expected_guild_revision = 16;
}
message JournalTradeSide { bytes character_id = 1; bytes account_id = 2; repeated JournalItem items = 3; int64 common_amount = 4; }
message JournalTrade {
  bytes trade_id = 1; uint64 expected_revision = 2; JournalTradeSide initiator = 3;
  JournalTradeSide counterpart = 4; bytes settlement_id = 5; int64 fee_common = 6;
  int64 finalized_at_ms = 7; bytes initiating_client_operation_id = 8;
}
message JournalActivity { bytes character_id = 1; uint64 session_epoch = 2; string transition = 3; int64 occurred_at_ms = 4; }
message JournalCompetitiveAdmission {
  string scope = 1; bytes match_id = 2; uint32 season_id = 3;
  int64 admitted_at_ms = 4; int64 deadline_at_ms = 5; string state = 6;
  optional int64 terminal_at_ms = 7; optional JournalMatch resolution_snapshot = 8;
  string content_revision = 9;
}
message JournalChatLog {
  bytes message_id = 1; bytes sender_account_id = 2; bytes sender_character_id = 3;
  string channel = 4; optional string scope_id = 5; string content = 6; int64 created_at_ms = 7;
}
message JournalJob {
  string kind = 1; string job_key = 2; int64 due_at_ms = 3;
  oneof target {
    JournalAuctionJob auction = 10; JournalQuestJob quest = 11; JournalGuildJob guild = 12;
    JournalSeasonJob season = 13; JournalErasureJob erasure = 14; JournalPaymentJob payment = 15;
    JournalMaintenanceJob maintenance = 16; JournalCompensationJob compensation = 17;
  }
}
message JournalAuctionJob { bytes listing_id = 1; optional bytes escrow_asset_id = 2; uint64 expected_revision = 3; }
message JournalQuestJob { bytes character_id = 1; string cycle_id = 2; optional string quest_id = 3; }
message JournalGuildJob { bytes guild_id = 1; string cycle_id = 2; optional bytes claim_id = 3; }
message JournalSeasonJob {
  string scope = 1; uint32 season_id = 2; int64 cutoff_at_ms = 3; optional bytes match_id = 4;
  optional string cosmetic_id = 5; optional bytes award_owner_id = 6; optional bytes membership_id = 7;
}
message JournalErasureJob { bytes operation_id = 1; bytes account_id_hash = 2; int64 prepared_at_ms = 3; }
message JournalPaymentJob { bytes entitlement_id = 1; string provider = 2; string notification_key = 3; }
message JournalMaintenanceJob {
  string row_family = 1; int64 window_start_ms = 2; int64 window_end_ms = 3;
  optional bytes target_id = 4; optional string cursor_key = 5;
}
message JournalCompensationJob { bytes approved_audit_event_id = 1; bytes character_id = 2; repeated JournalRewardSlot slots = 3; }
```

### Deterministic Client and Outcome Declaration Expansion

This is declaration grammar, not prose payload encoding. A field cell `name:type=number` expands to a proto3 field declaration; `optional` and `repeated` are literal protobuf modifiers; `U` expands to `bytes`; `v1.X` expands to `thinhthan.v1.X`. Each semicolon separates fields. The complete message declarations are:

```text
JournalAggregateRevision: aggregate:string=1; owner_id:U=2; revision:uint64=3
JournalEnhanceResult: success:bool=1; level_before:uint32=2; level_after:uint32=3; final_rate_bp:uint32=4; pity_fail_count:uint32=5; consumed:repeated v1.ItemQuantity=6; currency_delta:repeated JournalCurrency=7
JournalConsumedItem: item_instance_id:U=1; item_id:string=2; quantity:uint64=3
JournalCraftSnapshot: recipe_id:string=1; batch_quantity:uint32=2; created_items:repeated JournalItem=3; consumed:repeated JournalConsumedItem=4; currency_delta:repeated JournalCurrency=5; character_exp:uint64=6
JournalClientCommand: account_id:U=1; character_id:optional U=2; session_epoch:uint64=3; ownership_epoch:optional uint64=4; admitted_at_ms:int64=5; issued_at_ms:int64=6; spatial_source:optional JournalSource=7; finalized_rewards:repeated JournalRewardSlot=8; trade:optional JournalTrade=9; expected_revisions:repeated JournalAggregateRevision=10; rng_outputs:optional JournalEnhanceResult=11; checkpoint:optional JournalCheckpoint=12; entry_members:repeated JournalEntryMember=13; craft:optional JournalCraftSnapshot=14; oneof request=CLIENT_EXPANSION
JournalEntryMember: character_id:U=1; account_id:U=2; decision:string=3; checkpoint:JournalCheckpoint=4
JournalCreatedId: kind:string=1; id:optional U=2; content_id:optional string=3
JournalOutcome: status:v1.ResultStatus=1; error_code:v1.ErrorCode=2; operation_id:U=3; reward_slots:repeated JournalRewardSlot=4; created_ids:repeated JournalCreatedId=5; revisions:repeated JournalAggregateRevision=6; enhancement:optional JournalEnhanceResult=7; checkpoint:optional JournalCheckpoint=8; schedule:optional JournalPublicSchedule=9; match:optional JournalMatch=10; competitive_admission:optional JournalCompetitiveAdmission=11; craft:optional JournalCraftSnapshot=12; oneof client_result=RESULT_EXPANSION
```

`CLIENT_EXPANSION` creates one concrete oneof member per listed request ID in the following table: for ID `n`, field number `1000+n`, field name the exact registry name lowercased, type `thinhthan.v1.<ProtoName>`. ProtoName retains the C2S/S2C prefix and joins each remaining underscore token as first letter uppercase/remainder lowercase: `C2S_INVENTORY_MUTATE` → `C2SInventoryMutate`; `S2C_DAILY_BOARD_STATE` → `S2CDailyBoardState`. These PascalCase names match `engineering_conventions.md`. `RESULT_EXPANSION` uses the same rule with `2000+n`, for response IDs **13,110,112,115,116,401,403,405,407,409,411,413,415,417,419,421,423,425,427,429,431,501,503,505,508,510,514,633,649,654,709,731,733,735,741,743**. A server-only outcome has no client_result; a client outcome has exactly its owning typed response. Chat600/result655 are runtime delivery, not queued client value mutations; their separately queued moderation log uses JournalChatLog.

| Client IDs | Source proto file | Stable family / receipt owner |
|---|---|---|
| 12 | `session.proto` | `character.create` / ACCOUNT |
| 103 | `movement.proto` | `interaction.<interact_kind lowercase>` / CHARACTER |
| 104,109,111,113,114,117 | `movement.proto` | respectively `placement.portal`, `placement.channel`, `instance.enter`, `instance.respond`, `instance.leave`, `instance.cancel` / CHARACTER; codec/receipt applies whenever their validated intent crosses Durable for placement/checkpoint/membership; purely runtime prompt/cancel responses do not enqueue |
| 400,402,404,406,408,410,412,414,416,418,420,422,424,426,428,430 | `durable.proto` | respectively `inventory.mutate`, `loadout.change`, `craft.create`, `enhance.apply`, `reward.claim`, `beast.active`, `beast.equip`, `beast.unequip`, `beast.feed`, `entitlement.claim`, `shop.buy`, `cosmetic.redeem`, `cosmetic.equip`, `shop.sell`, `inventory.expand`, `beast.level_up` / CHARACTER |
| 500,502,504,507,509,511,512,513 | `content.proto` | respectively `quest.accept`, `quest.turn_in`, `atlas.acknowledge`, `quest.abandon`, `story.choose`, `skill.upgrade`, `potential.allocate`, `progression.respec` / CHARACTER |
| 608,610,611,613,614,615,617,618,623,624,625,626,627,629,630,632,637,638,639,640,642,643,644,645,646,648,650,651,652,656 | `social.proto` | `client.<ID>` / CHARACTER (authenticated actor, including guild requests; authorized target guild is not the receipt owner) |
| 708,730,732,734,740,742 | `market.proto` | respectively `trade.finalise`, `auction.list`, `auction.buy`, `auction.cancel`, `auction.reclaim`, `auction.proceeds` / CHARACTER |

Client account_id is always present; character/ownership epoch absent only for 12. Spatial_source is required for interactions, craft/enhance/shop/respec and trade admission; not for nonspatial durable requests. 103 covers every durable interaction: pickup/chest, CAST/HOOK, kindle/cook/rest, quest object, checkpoint/travel and TALK that changes quest/discovery; non-mutating TALK never enters Durable. Request operation_id equals record operation_id. Authenticated server admission freezes source spatial evidence/epochs; restart replay does not query a vanished actor or require a fresh session. Mutable DB resource/revision checks remain transactional. All finalized RNG outputs, consumed item/currency inputs, assigned instance IDs and eligible contracted Souls are frozen before the first settlement attempt. `finalized_rewards` is present precisely for finalized reward-slot outputs, not as a generic RNG/crafting container; deterministic reward outputs are not omitted. `rng_outputs` is present only for enhance. 404 always requires `craft`, including guaranteed recipes; 103 COOK also requires `craft` for its recipe outputs/extra_output, consumed inputs and authored LIFE_SKILL character EXP. Any resulting separately keyed Atlas/quest rewards retain their own canonical reward source and typed slots; cooking itself is not a new claim source. Other requests have no `craft`.

708 always requires the original `v1.C2STradeFinalise` request and both finalized `trade` sides; other requests have no `trade`. Request trade_id/expected_revision equal the embedded snapshot, and `initiating_client_operation_id` equals request/record operation_id. The client character/account are the authenticated finalizing actor and must match one captured side, never be replaced by a synthetic server owner. Session/ownership epochs, admitted/issued times, spatial_source and expected_revisions stay in the CLIENT record. Its family `trade.finalise`, CHARACTER receipt owner, concrete request discriminator 708 and complete typed snapshot participate in the canonical fingerprint/receipt rules in `save_rules.md`; replay resolves that original receipt, not a separate trade command identity.

### Closed Semantics and Provenance

- `schema_version=1`; admission_sequence starts at 1 per boot, never reused; producer_incarnation_id is crypto-v4 fixed per producer lifetime. `JournalSource` preserves original incarnation/counter/tick from `save_rules.md`; normal channels 1..30 have no instance; instance channel 0 requires instance UUID. Encounter/generation/chain IDs are present precisely for encounter/public/surge sources. Source content IDs resolve only in the record's immutable revision.
- Lists sort by content ID, then UUID raw bytes, then slot/index; duplicate semantic keys are invalid. Every reward slot is nonempty/unique. Item carries finalized rolls, enhancement/pity, binding, any contracted Soul state and creation revision; no opaque item_state JSON or later inferred rolls. `created_at_ms` is required for direct creation and existing owned-item snapshots (including trade), preserving the fixed creation time; absent for unmaterialized claim payloads until their successful materialization. Stat scale is 1 for integral flat stats and 10000 for basis-point ratios, value the exact signed numerator. Stat/roll identities/ranges come from `equipment_catalog.md`/`stats.md`. Plain stackables have empty rolls/pity, zero enhancement/Soul numeric fields, absent soul_id. Unmaterialized claim creation has absent item_instance_id; direct creation allocates/fixes it before first attempt. Binding is UNBOUND/ACCOUNT_BOUND/CHARACTER_BOUND. No durability state exists. `JournalItem.soul_id/soul_level/soul_exp` describe only a Soul already contracted to that equipment, never acquisition into `character_souls`.
- Currency is exactly currency.common/currency.bound/currency.special. Reward amounts are positive, client debits signed. Each new contribution fits int64/uint64; NUMERIC(38,0) totals remain database-owned and are never truncated into the codec. 408 reads existing typed claim lines and stores its exact committed capacity-fitting batch, not the current remainder on retry.
- Reward kind is exactly KILL/BOSS/DUNGEON/EVENT/QUEST/DISCOVERY/ATLAS/FEAT/CHIVALRY/SOUL/BEAST/COSMETIC/GUILD_STONE. Quest/discovery/dungeon/run/season optional fields are present iff part of that kind's natural scope. `source_type` and source_reference use the canonical `reward_claims.md` source enum/format; no arbitrary new reward source. Kill transaction includes all loot/EXP/Soul/Atlas/quest/chivalry outputs for its recipient; standalone QUEST and DISCOVERY are real codec cases, not kill aliases. Guild EventSink outputs keep their own stable key.
- A new owned Soul is encoded only as `JournalRewardSlot.soul_acquisitions`, never as a fake item, currency, flag or existing-Soul EXP recipient. Its fixed UUID and `soul_id` create one character-owned `character_souls` row, initially level 1/EXP 0/uncontracted, using the record's immutable Soul definition revision. `soul_id` resolves in `soul_catalog.md`, fits the persisted VARCHAR(64) bound, and has the matching `atlas_catalog.md` hon_giam page; `atlas_progress` is that page's acquisition-counter delta 1 for both first and duplicate copies. This delta is encoded here only, not duplicated in the slot's general atlas_progress. Duplicate is the finalized already-owned-definition decision; the actual extra instance is retained and grants zero duplicate Soul EXP. `atlas_page_mastered_before` records the matching page's prior Mastered state, not a promotion later in this settlement.
- `resonance` is present iff duplicate and the page was already Mastered. Its nonnegative before/after counts fit PostgreSQL int (`0..2147483647`) and after is exactly before+1; overflow fails before earning, never clamps. Sheen timestamps are absent for non-BOSS Souls. For BOSS, an existing timestamp is preserved byte-for-byte; when the increment first reaches 10 it fixes the unlock timestamp once before admission, with before absent and after present; below 10 both are absent. Replay neither increments again nor substitutes its clock. `expected_soul_revision` is the captured collection revision (nonnegative PostgreSQL BIGINT), shared by acquisitions finalized in the same transaction. Under the character lock, receipt lookup precedes revision/state checks; new instance, duplicate Atlas/resonance/sheens mutations and collection revision commit atomically. A revision/state conflict never reclassifies or rerolls the frozen acquisition. `soul_exp` remains separate: at most three previously contracted ACTIVE-loadout instance UUIDs eligible at settlement start, with existing source amounts/Lv5-zero behavior from `soul_contracts.md`; none may be a newly acquired UUID. All other finalized source outputs remain in their typed slot fields, including applicable Atlas tier rewards, discovery/progression and presentation outputs.
- Soul acquisition lists use the whole-recipient batch order/virtual-prefix semantics in `soul_contracts.md` § Atomic Acquisition Batch, overriding the generic list sort for these entries. Uniqueness is by new instance UUID, not soul_id. All entries share the original expected_soul_revision; validate it once and each captured duplicate/resonance/sheens tuple against the virtual prefix, including across reward slots. Two independent 9→10 snapshots are invalid where the required chain is 9→10→11 with one preserved sheen time. Commit the batch and one collection revision atomically; no added field is needed.
- `JournalCraftSnapshot` freezes a recipe resolved in `crafting_catalog.md` at record revision, the exact original batch, every created `JournalItem`, actual consumed material instance UUID/item ID/quantity, and signed currency deltas. Recipe/batch match 404's original request (batch 1..99), or 103 COOK's hearth recipe (one authored cook action). Quantities are positive and bounded by the current item stack/recipe/batch contracts and network uint32 quantity range; consumed entries uniquely identify the owned stacks selected before admission, and their item IDs and total quantities equal authored inputs times batch. Currency IDs/deltas exactly match authored costs (negative debits, no invented currency or reward source); absent costs have no line. Created outputs include every primary/extra output with fixed UUID, binding, content revision, creation time, rolls, enhancement/pity and Soul fields. Equipment creates one distinct quantity-1 instance per output, finalized catalog rolls and +0 at launch; new output has no contracted Soul and empty initial pity. Stackable output uses its canonical stack limit and no equipment/Soul state. Utility/food recipes retain every authored output; guaranteed success never makes rolled equipment stats absent. `character_exp` is exactly the authored LIFE_SKILL per-unit award for a 103 COOK action at the character's captured act (`crafting_catalog.md`/`progression_route.md`); zero for 404, which has no launch EXP award. It commits with the cook outputs under the existing `life_skill.cook.<recipe_id>.<character_id>.<operation_id>` identity, never as a fabricated CRAFT reward slot.
- Craft is capacity-prevalidated, all-or-nothing inventory creation, not a Reward Claim: no CRAFT source_type, no synthetic reward slot for its created items or costs. `expected_revisions` retains the captured inventory and any other actually read aggregate revisions; the original request, actor/epochs and spatial evidence remain in CLIENT. Commit revalidates ownership, selected material quantities, currency balances, recipe requirements, combat/locks and complete output capacity under the existing transaction rules before any consumption. Failure consumes/creates nothing; retry resolves the original operation and never selects replacement inputs, UUIDs or rolls. Successful outcome preserves the same `craft` snapshot, exact ITEM created_ids and resulting revisions plus the matching 405 (404) or 116 (103 COOK) typed response; 405's consumed/granted/currency summaries are projections of that snapshot, not its replacement.
- World transition is CREATE/EXPIRE/DESPAWN. CREATE includes original spawn/expiry/buff and source; EXPIRE/DESPAWN includes the original relic spawn/expiry to reject an old expiry against a replacement. Defeat metadata fields are all present only for launch boss CREATE, absent for seasonal/expiry. Marker lock and social-proof tie-break remain in `data_model.md`.
- Boss eligibility transition is ELIGIBLE/DEFEATED/UNDEFEATED_DESPAWN; eligible_until present only for DEFEATED. DEFEATED atomically persists the enclosing `DurableCommandRecord.content_revision` as `boss_chest_eligibility.reward_content_revision` with defeated-copy provenance; that column is required iff defeated and held unchanged through INTERACT/TIMEOUT/RESTART fallback, never sourced from the current catalog. Chest reason INTERACT/TIMEOUT/RESTART always preserves defeated copy identity; generation/slot ledger lookup precedes finalization/delivery. Existing slots return their original grant even if another copy produced this record. New slots roll exactly once from recorded revision/source identity.
- Checkpoint membership NONE/MEMBER/EXITED/ABANDONED; optional instance/dungeon/run/source-map/channel fields all present for instance membership, absent for NONE. Transfer ID present only for a transfer boundary. Source is required for every server CHECKPOINT, retaining original source_event_id/incarnation/tick/map/channel, and source_tick equals source.tick. Embedded client boundaries may omit source only when their original client operation key and JournalClientCommand.spatial_source already provide provenance. Ownership epoch rejects stale overwrites; no raw per-tick position or whole-character save is encoded.
- Schedule transition INITIALIZE/OPEN/CLOSE/BOOT_CLOSE. OPEN has new generation/opened_at only; other transitions have next_spawn/sample respawn_delay (1800..2700 inclusive integer seconds) only. Prior generation is present only for CLOSE/BOOT_CLOSE. Expected revision, transition time, UUID and sampled delay are fixed before admission; replay never samples a new delay.
- Match state COMPLETED/VOID; record result identifies the command owner's perspective WIN/LOSS/DRAW/NONE (NONE iff VOID), and **each participant and guild output also has its own explicit result**. Opposing teams never inherit the same global WIN. settlement_type is PVP/GUILD_RATING/GUILD_PROGRESSION/PERSONAL_REWARD/SEASON_PARTICIPATION; participation NORMAL/AFK/ABANDONED; guild_id/membership_id present iff Guild War. Sanction fields and bound slot/window fields are each all-present/all-absent. Participant snapshots include finalized rating revisions/deltas, participation/AFK facts and reward slots. Guild outputs carry guild rating revisions/deltas, Monday-week progression slot 1..3 and finalized EXP/member-contribution amounts (30 both sides, +10 winner) with historical membership; no qualifying slot means absent week/slot and zero progression outputs. VOID has no rewards/rating/progression change. No replay recomputes a result or rerates against the next season.
- Guild source_kind is exactly DUNGEON/BOSS/WORLD_EVENT/GUILD_ACTIVITY/RITUAL/GUILD_STONE, corresponding to the source rows in `guild_progression.md`/Guild Stone; element is 1..5 in KIM/MOC/THUY/HOA/THO order. Chain ID is required only for Surge, gathering slot only for bonfire. credited_members carries **all** eligible historical membership UUIDs and finalized contribution amounts, not just the triggering character; cycle_id and expected_guild_revision bind the rotation/vessel transaction. Guild/membership/occurrence/revision provenance is historical, never re-inferred from current membership.
- Random source seed halves are both present iff authoritative PCG-64 randomness was used, absent for deterministic nonrandom sources. They identify the original stream for provenance only: every sampled choice/value is already finalized in the typed outputs and replay never advances that stream or rerolls from seeds.
- Competitive admission scope is RANKED_DUEL/FIVE_ELEMENT_ARENA/GUILD_WAR; state PREPARING/ACTIVE/RESOLVING/COMPLETED/VOID/CANCELLED. Match UUID, season, admitted/deadline times and revision are fixed at PREPARING. RESOLVING carries the complete typed JournalMatch snapshot; terminal states carry terminal_at, nonterminal states do not. This is the actual durable transition emitted after runtime ready-check, not a queued client challenge/queue prompt. On restart PREPARING/ACTIVE terminalizes VOID by the owning admission rule; RESOLVING completes its original snapshot before season close. JournalOutcome gains `competitive_admission:optional JournalCompetitiveAdmission=11`.
- Source presence is exact, not fabricated: simulation-created reward/WorldConsequence/eligibility/chest commands require JournalSource and preserve its original tuple. Authored one-time content grants instead require grant_scope and the original authored UUIDv5 scope/revision; no fake partition. Durable WorldConsequence expiry/startup repair and eligibility undefeated cleanup use the already-persisted relic/copy natural key and may omit source; their server job identity is defined in `ids.md`. PUBLIC chest fallback may omit simulation source only when defeated copy state, original reward content_revision and generation/slot grant identity are persisted in boss_chest_eligibility; these provenance fields are fixed at defeated-copy commit. `eligible_until`, source generation/copy and revision cannot be inferred from current catalog. If no prior slot outcome exists, personal reward roll uses the original deterministic generation/character/slot stream and recorded revision, freezes every result before first delivery and uses the same source grant UUIDv5 on timeout/restart/client interaction; no boot-time seed/new operation.
- Chat log channel is WORLD/PARTY/GUILD/WHISPER/LOCAL and content uses the existing 1..240-grapheme/UTF-8 bounds from social/text contracts. scope_id is absent only for WORLD, otherwise the original canonical party/guild/whisper target/map-channel key. The existing authoritative message UUID/sender/occurrence/text is frozen at accepted runtime delivery; asynchronous bounded moderation-log queue entries/batches journal as one CHAT_LOG per message, UUIDv5(SERVER_JOB_NAMESPACE_UUID, "chat.log:<message UUID>"), family chat.log, CHARACTER sender owner. Insert dedups on chat_messages.message_id and verifies identical row; it never redelivers chat, changes original timestamp or resurrects an erased/retention-expired message. Such messages terminally reconcile as intentional policy deletion after verifying original identity/time/subject fence, not a gameplay grant or new chat success.
- Job kind/target pairs are closed: auction EXPIRE/ESCROW_FALLBACK; quest DAILY_RESET/QUEST_EXPIRE; guild CYCLE_FREEZE/VOTE_FINALIZE/BLESSING_EXPIRE/STORAGE_CLAIM_EXPIRE; season CUTOFF/DRAIN_MATCH/FREEZE_AWARDS/DELIVER_AWARD; erasure ERASURE_RESUME; payment PAYMENT_RECONCILE/PAYMENT_NOTIFICATION; maintenance RETENTION_PURGE/ECONOMY_ROLLUP/ECONOMY_REVIEW/SECURITY_REVIEW; compensation COMPENSATION_DELIVER. Keys follow `ids.md`; each command names one typed durable target/window, not SQL. Optional target fields are present iff part of that kind's key. Erasure references original PREPARED operation/hash/time; payment references persisted entitlement/provider notification, no raw receipt/token. Season awards reference frozen award owner/membership/cutoff/revision. Missing required durable sources stop reconciliation, not success. Maintenance row_family is a physical table family whose cleanup/rollup/review is explicitly owned by the data model/register/anti-cheat; never a grant instruction.
- ERASURE_RESUME is the source-backed continuation handoff in `data_model.md` § Account Erasure/`save_rules.md`, not destructive completion. Validate its operation/hash against the durable erasure intent and exact GET-verified PREPARED object/key; `prepared_at_ms` equals the intent's canonical millisecond projection only. The full UTC-microsecond prepared_at comes from the durable intent/object, never reconstructed from milliseconds. Terminal disposition closes only this queue/journal reference after the existing PREPARED/fence proof, with no destructive callback, completed_at, successful erasure JournalOutcome or client-completion acknowledgment; the worker performs destruction only after all referring files are unlinked/fsynced at the specified startup continuation step.
- aggregate revisions use only inventory/loadout/progression/beast/soul/cosmetic/claims/guild/guild_storage/auction/trade. CreatedId kind is ITEM/SOUL/BEAST/CLAIM/GUILD/LISTING/PROCEEDS/REPORT/CHARACTER/SETTLEMENT; BEAST has content_id only, all others a UUID (no beast instance identity). JournalOutcome records exact assigned IDs, currency/EXP/rolls, claim-delivered batches, revisions and typed response. Reconstructing it never re-executes a handler.
- Bounds: encoded record ≤1 MiB, depth ≤16, ≤512 total nested list entries; ≤20 match participants, ≤12 trade items per side, ≤3 Soul EXP recipients per reward slot. Content IDs ≤128 UTF-8 bytes, family ≤48 ASCII bytes, source_reference ≤160, reward_slot ≤64, job_key ≤512; user text retains network limits. Oversized finalized sources may split **before earning/admission** only at existing independent owner/reward-slot boundaries, never split an atomic kill/trade/match or omit outputs. An indivisible source that cannot fit fails before final reward confirmation.

## Invariants

```text
proto/thinhthan/v1/*.proto là wire source of truth duy nhất
generated Go destination = server/internal/protocol/v1/
Go và C# giải mã cùng binary fixture cho kết quả đồng nhất
số field đã commit là bất biến; cấm tái sử dụng số đã xóa
codegen drift check bắt buộc trong verify scripts (drift = 0)
```
