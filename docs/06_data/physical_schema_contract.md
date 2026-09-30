# Physical Schema & Migration Contract
status: LOCKED

## Scope

Hợp đồng kỹ thuật quy định các kiểu dữ liệu vật lý chính xác trong PostgreSQL 18.6, quy tắc đặt tên, ràng buộc toàn vẹn (constraints), hành vi khóa ngoại (foreign keys), thứ tự khóa dòng giao dịch (transaction lock ordering) và chính sách quản lý migration bằng `golang-migrate` v4.20.1.

Tài liệu này đảm bảo hai AI agent triển khai các task persistence độc lập (`IMP-005` đến `IMP-010`, `IMP-030`, `IMP-036`, `IMP-053`) không thể tạo ra hai physical schemas khác nhau cho cùng một logical model.

## 1. Hệ Thống Cơ Sở Dữ Liệu & Encoding

- **Hệ quản trị CSDL:** PostgreSQL `18.6` (chính xác theo `docs/00_context/technology_versions.md`).
- **Character Encoding:** `UTF-8`.
- **Collation:** `C` (hoặc `und-x-icu`) cho các cột index nhị phân / canonical key; `vi-VN-x-icu` cho tìm kiếm văn bản tiếng Việt nếu cần.
- **Timezone:** `UTC` tuyệt đối. Server Go và PostgreSQL đều cấu hình timezone UTC.

## 2. Quy tắc Ánh xạ Kiểu Dữ Liệu Vật lý (Type Mapping)

| Khái niệm nghiệp vụ | Kiểu PostgreSQL vật lý | Quy tắc & Giới hạn |
|---|---|---|
| **Primary Key / Durable UUID** | `UUID NOT NULL` | Sinh bằng RFC 4122 UUID v4 (`crypto/rand`). Cấm dùng `SERIAL` hay số tự tăng cho ID bền vững. |
| **Stable Content ID** | `VARCHAR(64) NOT NULL` | Chữ thường ASCII, chấm, gạch dưới (e.g. `monster.lang_da.chuot_dong`). |
| **Normalized Name Key** | `VARCHAR(256) NOT NULL` | Canonical Unicode NFC + Case-folded (case-fold may expand code points). Có ràng buộc `UNIQUE`. |
| **Display Text / Player Names** | `VARCHAR(64) NOT NULL` (guild: `VARCHAR(96)`) | Chuỗi hiển thị giữ nguyên hoa/thường và dấu tiếng Việt; giới hạn grapheme và byte theo `text.md` § Name Limits (nhân vật 16 graphemes ≤ 64 byte UTF-8; guild 24 graphemes ≤ 96 byte). |
| **Currency Balances** | `BIGINT NOT NULL` | Số nguyên 64-bit có dấu. Bắt buộc có `CHECK (balance >= 0)`. |
| **Total Cumulative EXP** | `INTEGER NOT NULL` | Số nguyên 32-bit theo `data_model.md`; bắt buộc `CHECK (current_exp >= 0 AND current_exp <= 702100000)`. |
| **Levels / Item Counts / Slots** | `INTEGER NOT NULL` | Số nguyên 32-bit. Bắt buộc `CHECK (level >= 1)` hoặc `CHECK (quantity > 0)`. |
| **Timestamps** | `TIMESTAMPTZ NOT NULL` | Luôn có múi giờ UTC. Mặc định `DEFAULT NOW()`. |
| **Boolean Flags** | `BOOLEAN NOT NULL` | Mặc định `DEFAULT FALSE`. |
| **Arbitrary / Bounded JSON** | `JSONB NOT NULL` | Mặc định `DEFAULT '{}'::jsonb` hoặc `'[]'::jsonb`. Không dùng `TEXT` để lưu JSON. |

## 3. Danh mục Bảng Khởi tạo (Baseline Table Inventory)

Chưa có migration nào tồn tại. IMP-005 tạo baseline `server/migrations/000001_baseline_schema.up.sql` và snapshot `server/migrations/schema_snapshot.sql`; baseline tạo **mọi bảng khai báo trong `data_model.md`** (kể cả `characters.updated_at`); danh sách dưới đây là các bảng cần lưu ý đặc biệt về kiểu dữ liệu/ràng buộc, không phải danh sách giới hạn:

1. `accounts` — Tài khoản người chơi, trạng thái `ACTIVE`, `TOMBSTONE_ERASED`; không có cột điểm hoàn tiền IAP (điểm được suy ra từ `account_refund_consumed_events` trong 180 ngày, ADR-0060). Baseline chèn sẵn hàng `TOMBSTONE_ACCOUNT_ID` (`data_model.md`, ADR-0065); cột `erased_at`, `credential_guard_until`, `economy_review_flagged_at`.
2. `account_identities` — Liên kết OAuth bên thứ ba (Apple, Google, Steam), PK `(provider_id, provider_subject)`, ràng buộc `ON DELETE RESTRICT`; `account_login_history` — tín hiệu đăng nhập 90 ngày (ADR-0065).
   `account_password_credentials` — Username/email/Argon2id hash cho provider `password` (ADR-0051); `UNIQUE(username_key)`, `UNIQUE(email_key)`.
3. `characters` — Nhân vật người chơi (tối đa 3 nhân vật, cấp 1..60, tên định danh duy nhất `name_key`); `current_exp INTEGER`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` và `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. `updated_at` chỉ ghi lần sửa gần nhất của chính hàng `characters`, theo `data_model.md`.
4. `operations` — Nhật ký idempotency chống xử lý lặp lại giao dịch tài sản. PK `(operation_family, owner_id, operation_id)` (ADR-0065).
5. `character_currencies` — Số dư 3 loại tiền (`common`, `bound`, `special`) có ràng buộc `>= 0`.
6. `item_instances` & `item_locations` — Thực thể vật phẩm, cấp cường hóa 0..16, và vị trí duy nhất.
7. `account_iap_entitlements` & `account_refund_consumed_events` — Quyền sở hữu IAP (`grant_state` gồm `REJECTED` kèm `reject_reason`, cột `platform`) và nhật ký sự kiện hoàn tiền trong cửa sổ 180 ngày.
8. `economy_account_daily_rollups` & `economy_character_daily_rollups` — Bảng tổng hợp luồng tiền và khối lượng giao dịch đối tác (`trade_partner_volumes`).
9. `world_consequence_relics` & `region_di_tich_markers` — Trạng thái thế giới sau khi diệt boss Di Tích (khóa `(map_id, channel_id, relic_id)`; partial index `(relic_id) WHERE relic_active`).
10. `rate_limit_counters` & `auth_failure_backoff` — Bộ đếm giới hạn tần suất L2 (cửa sổ trượt 2 cửa sổ) và backoff lũy tiến đăng nhập sai trên PostgreSQL (không dùng Redis; schema: `../07_security/external_integrations.md` § 3, ADR-0064).
11. `iap_notification_dedup` — Chống xử lý trùng lặp thông báo Apple/Google/Steam, PK `(provider, notification_key)`; `iap_provider_cursors` — con trỏ GetReport của Steam.
12. `audit_events` — Nhật ký kiểm toán an ninh và truy vết thao tác.
13. `character_inventories` — Metadata kho đồ nhân vật: capacity, revision.
14. `account_cosmetic_entitlements` (có `first_equipped_at`) & `account_entitlement_claims` — Quyền sở hữu mỹ phẩm IAP cấp account (per `data_model.md`).
    `character_cosmetic_entitlements` & `character_cosmetic_equips` — Mỹ phẩm cấp nhân vật và slot đang trang bị (ADR-0060).
15. `auth_session_families`, `auth_refresh_credentials`, `auth_revocations` — Phiên xác thực và thu hồi (schema: `data_model.md` § Auth sessions; access token, gameplay ticket, resume credential chỉ nằm trong bộ nhớ tiến trình).
16. `boss_chest_eligibility` — Quyền mở rương boss PUBLIC theo nhân vật (ADR-0053); `public_boss_schedules` — vòng đời generation boss PUBLIC, CHECK trạng thái `SCHEDULED`/`OPEN` (ADR-0061).
17. `character_feats` & `character_feat_milestones` — Feat và mốc thưởng (`../03_systems/cosmetics.md`).
18. `character_souls` — Soul instance, cấp, EXP, `contracted_item_instance_id UNIQUE` (ADR-0060); `character_soul_resonance` — bộ đếm cộng hưởng ký ức theo `(character_id, soul_id)`.
19. `character_beasts`, `character_beast_food_daily`, `beast_equipment_locations` — Linh Thú và bộ đếm điểm thức ăn theo ngày UTC.
20. `reward_claims`, `reward_claim_lines`, `reward_claim_contributions`; `auction_listings`, `auction_proceeds`, `trade_settlement_records`; bảng guild (`guilds`, `guild_memberships`, `guild_member_contributions`, `guild_invites`, `guild_applications`, `guild_progression`, `guild_ritual_cycles`, `guild_blessing_votes`, `guild_storage_claims`, `guild_storage_audit`) — schema trong `data_model.md` (ADR-0065).
21. `character_loadouts`, `character_atlas`, `atlas_milestones`, `chat_messages`, `guild_stone_category_completions`, `pending_erasure_ledger` (ADR-0070); `friends`, `friend_requests`, `blocks`, `character_chivalry`; `pvp_ratings`, `pvp_match_settlements`, `pvp_sanctions`, `guild_war_ratings`, `guild_war_settlements` — schema trong `data_model.md`.

## 4. Ràng buộc Toàn vẹn & Hành vi Khóa Ngoại (Foreign Keys)

1. **Quy tắc Xóa Khóa Ngoại (Delete Action):**
   - Mặc định tuyệt đối: `ON DELETE RESTRICT`. Ngăn chặn việc xóa vô tình dữ liệu cha khi còn dữ liệu con.
   - Trường hợp ngoại lệ duy nhất dùng `ON DELETE CASCADE`: Các bảng phiên làm việc ngắn hạn (`auth_refresh_credentials` thuộc `auth_session_families`).
   - Tuyệt đối cấm `ON DELETE CASCADE` trên các bảng tài sản: `characters`, `character_inventories`, `character_currencies`, `item_instances`.
   - **ON UPDATE:** mặc định `NO ACTION`; không dùng `ON UPDATE CASCADE`. Ngoại lệ duy nhất: hai composite FK của `account_entitlement_claims` khai báo `DEFERRABLE INITIALLY IMMEDIATE` để giao dịch erasure (`SET CONSTRAINTS ALL DEFERRED`) trỏ lại `account_id` của cha và con sang `TOMBSTONE_ACCOUNT_ID` (`data_model.md` § Account Erasure, ADR-0065).
   - Cột tham chiếu account/character trong bảng log không mang FK: `operations.owner_id`, `audit_events.*_id`, `auth_revocations.account_id`.
2. **Ràng buộc Miền Giá trị (`CHECK` Constraints):**
   - Mọi cột số dư tài sản phải có `CHECK (amount >= 0)`.
   - Mọi cột enum lưu dưới dạng `VARCHAR` phải có ràng buộc `CHECK (col IN ('VAL1', 'VAL2', ...))`.
3. **Đảm bảo Duy nhất (Unique Constraints):**
   - `operations(operation_family, owner_id, operation_id)` (PK): Đảm bảo tính duy nhất của mọi thao tác thay đổi trạng thái bền vững theo phạm vi chủ sở hữu (ADR-0065).
   - `account_iap_entitlements(platform_receipt)`: Chống nạp lặp biên lai IAP.
   - `characters(name_key)`: Đảm bảo tính duy nhất của tên nhân vật trên toàn thế giới logic.
## 5. Giao dịch & Thứ tự Khóa Tránh Deadlock (Lock Ordering)

Để đảm bảo không bao giờ xảy ra deadlock khi hai giao dịch đồng thời cập nhật nhiều hàng trong cùng một bảng (ví dụ: giao dịch trao đổi trực tiếp giữa 2 nhân vật, thanh toán đấu giá):

1. **Mức Cô lập Giao dịch (Isolation Level):**
   - Sử dụng `READ COMMITTED` làm mức mặc định.
   - Khóa dòng tường minh bằng `SELECT ... FOR UPDATE`.
2. **Quy tắc Sắp xếp Khóa Bắt buộc (Ascending Lock Ordering):**
   - Khi cần khóa từ 2 dòng trở lên trong cùng một bảng (e.g. khóa tài khoản người gửi và người nhận):
     ```sql
     -- Bắt buộc sort danh sách ID theo thứ tự tăng dần trước khi khóa
     SELECT character_id, common_balance
     FROM character_currencies
     WHERE character_id IN ($1, $2)
     ORDER BY character_id ASC
     FOR UPDATE;
     ```
   - Cấm truy vấn `FOR UPDATE` không có mệnh đề `ORDER BY ... ASC` khi thao tác trên nhiều đối tượng.

## 6. Chính sách Quản lý Migration (`Migration-First Policy`)

1. **Công cụ Migration:** `github.com/golang-migrate/migrate/v4 v4.20.1`.
2. **Thư mục lưu trữ:** `server/migrations/`.
3. **Quy ước đặt tên file:**
   - Đánh số tuần tự 6 chữ số có đệm số không: `000001_baseline_schema.up.sql` và `000001_baseline_schema.down.sql`.
   - Migration đầu tiên là `000001_baseline_schema`; mọi thay đổi schema sau khi `000001` đã merge dùng số kế tiếp `000002`, `000003`, ...
4. **Bất biến của Migration đã Merge:**
   - Tuyệt đối không sửa đổi nội dung của một file migration đã merge vào nhánh chính.
   - Mọi thay đổi schema mới (thêm cột, đổi index) bắt buộc phải là một cặp file migration mới có số thứ tự tăng dần tiếp theo.
5. **Schema Snapshot & Drift Test:**
   - File snapshot toàn bộ cấu trúc DB: `server/migrations/schema_snapshot.sql`.
   - Trong CI, lệnh verify sẽ áp dụng toàn bộ migration vào PostgreSQL test rỗng, chạy `pg_dump --schema-only` và so sánh diff với `schema_snapshot.sql`. Lệch diff = fail CI.

## Invariants

```text
PostgreSQL 18.6 UTF-8 UTC
ID bền vững = UUID; số dư = BIGINT CHECK (>= 0); ngày giờ = TIMESTAMPTZ
khóa ngoại = ON DELETE RESTRICT mặc định
khóa nhiều dòng = ORDER BY id ASC FOR UPDATE (chống deadlock)
migration = golang-migrate v4.20.1; số thứ tự 6 chữ số; bất biến sau khi merge
```
