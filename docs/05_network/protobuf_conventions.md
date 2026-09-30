# Protocol Buffers Conventions & Wire Baseline
status: LOCKED

## Scope

Quy chuẩn kỹ thuật cấu trúc Protobuf v3, sơ đồ phân tách file trong `proto/thinhthan/v1/`, quy tắc định số trường (field numbering), khung truyền tải (wire envelope), và cơ chế tương thích ngược (backward compatibility) cho toàn bộ message IDs đăng ký trong `messages.md` (ADR-0054).

Tài liệu này là hợp đồng wire ràng buộc giữa Go server và Unity client (C# 9.0).

## 1. Package & Namespace Layout

Mọi file `.proto` phải nằm trong thư mục `proto/thinhthan/v1/` và tuân thủ khai báo chuẩn:

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
| `session.proto` | 1 .. 15 | Hello, Attach, Detach, SessionReplaced, Heartbeat, ServerDraining, Error, CharacterCreate, CharacterList, PlacementPending (authentication itself is HTTPS, `../07_security/auth.md`) |
| `movement.proto` | 100 .. 117 | C2S_MOVEMENT_EDGE (108), C2S_CHANNEL_SWITCH (109), S2C_CHANNEL_SWITCH_RESULT (110), dungeon entry/exit (111..115, 117), S2C_INTERACT_RESULT (116), PositionSnapshot, Velocity, Knockback |
| `combat.proto` | 200 .. 208, 300 .. 306 | CombatAction, HitResult, world replication (baseline, spawn/despawn, state delta, baseline ack), S2C_COMBAT_EVENT (304), StatusEffectDelta |
| `durable.proto` | 400 .. 441 | InventoryMutate, Loadout (equip/skill/soul contract), Craft, Enhance, RewardClaim, Beast operations (410..417, 430..431), EntitlementClaim (418..419), NpcShop buy/sell (420..421, 426..427), Cosmetic redeem/equip (422..425), InventoryExpand (428..429), state pushes (432..438), reward-claim paging (439..441) |
| `content.proto` | 500 .. 515 | Quest accept/turn-in/abandon, AtlasClaim, ProgressionEvent, story branch, skill upgrade, potential allocate, respec, ProgressionState |
| `social.proto` | 600 .. 655 | Friend, Block, Chat (World/Party/Guild/Whisper), Party, Guild (incl. create/disband/applications/storage claims/blessing vote) |
| `market.proto` | 700 .. 744 (incl. 710) | Direct Trade, Auction list/buy/cancel/search/reclaim/proceeds |
| `pvp.proto` | 800 .. 819 | Sparring, Duel, Five Element Arena, Guild War match states |

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

`ErrorCode` (`common.proto`): `ERROR_CODE_UNSPECIFIED = 0` means `NONE` (success). Every other code listed in `errors.md` gets `ERROR_CODE_<CODE>` with an explicit number. The baseline (`IMP-061`) numbers the codes 1..N in order of first appearance of each `[A-Z][A-Z0-9_]{2,}` token inside the fenced code blocks of `errors.md` § Canonical Codes, blocks in document order, each block read row-major (left to right, then top to bottom); prose is never scanned, so every error code must appear in one of those fenced blocks (the baseline has 110); from then on `common.proto` is the numbering authority: new codes append with the next unused number, numbers are never reused or reordered, and a parity test checks that every code in `errors.md` has exactly one enum value. `Retryability` mirrors `errors.md` § Retryability (`RETRYABILITY_UNSPECIFIED = 0`, then NEVER .. BACKOFF in listed order).

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

## Invariants

```text
proto/thinhthan/v1/*.proto là wire source of truth duy nhất
generated Go destination = server/internal/protocol/v1/
Go và C# giải mã cùng binary fixture cho kết quả đồng nhất
số field đã commit là bất biến; cấm tái sử dụng số đã xóa
codegen drift check bắt buộc trong verify scripts (drift = 0)
```
