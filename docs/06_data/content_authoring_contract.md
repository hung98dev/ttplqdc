# Content Authoring Contract & Ingest Schema
status: LOCKED

## Scope

Hợp đồng quy định cú pháp nguồn, schema trường, kiểu dữ liệu, quy tắc mở rộng hữu hạn (finite expansion), hàm băm phiên bản (content hash) và vòng đời kích hoạt nguyên tử (atomic activation) cho toàn bộ 24 catalogs trong `docs/07_content/`.

Tài liệu này là đặc tả đầu vào duy nhất cho Content Compiler (`IMP-003`, `server/cmd/compiler/`) và Activation Gate (`IMP-004`, `server/internal/config/`).

## 1. Canonical Authoring Format

1. **Source of Truth duy nhất:** Toàn bộ dữ liệu nội dung được viết dưới dạng **Markdown Tables** nằm trực tiếp trong các file `docs/07_content/*.md`.
2. **Không tạo định dạng thứ hai:** Không sử dụng JSON, YAML hay cơ sở dữ liệu riêng để nhập liệu catalog gốc. Compiler trực tiếp parse các bảng Markdown nguồn.
3. **Cú pháp bảng hợp lệ:**
   - Dòng tiêu đề chứa tên trường chính xác (chữ thường, gạch dưới).
   - Dòng phân cách chuẩn Markdown (`|---|---|`).
   - Dòng dữ liệu được phân tách bằng dấu gạch đứng `|`.
   - Các ô dữ liệu được trim khoảng trắng ở hai đầu; ký tự backtick (``` ` ```) được tự động loại bỏ trong quá trình parse.
   - Ô trống mang giá trị mặc định được định nghĩa theo schema của từng catalog.

## 2. Kiểu Dữ liệu & Quy tắc Parse (Type Parsing Rules)

| Kiểu dữ liệu | Định dạng hợp lệ | Quy tắc & Giới hạn |
|---|---|---|
| **String ID** | `[a-z0-9_]+(\.[a-z0-9_]+)*` | Chữ thường ASCII, số, gạch dưới, phân cách bởi dấu chấm. Không có khoảng trắng. Cấm suy ra từ tên hiển thị. |
| **Integer** | `-?[0-9]+` | Số nguyên 32-bit hoặc 64-bit có dấu. Không chứa dấu phẩy hay dấu chấm ngăn cách hàng nghìn. |
| **Float / Ratio** | `-?[0-9]+(\.[0-9]+)?` | Số thực dấu phẩy động; độ chính xác 4 chữ số thập phân. Epsilon so sánh: `0.0001`. |
| **Enum** | `[A-Z0-9_]+` | Chữ hoa ASCII. Phải thuộc tập giá trị enum đã khai báo trong spec/proto tương ứng. |
| **Boolean** | `true` \| `false` | Case-insensitive boolean. |
| **Array / List** | `item1, item2, item3` | Phân cách bằng dấu phẩy; các phần tử được trim khoảng trắng. |
| **Range / Interval** | `min..max` | Khoảng đóng từ `min` đến `max`. Yêu cầu `min <= max`. |
| **Basis Points (bp)** | Số nguyên `0..10000` | Tỷ lệ phần mười nghìn (10000 bp = 100%). |

## 3. Danh mục 24 Launch Catalogs và Primary Keys

| # | Catalog File | Primary Key | Mô tả thực thể |
|---|---|---|---|
| 1 | `monster_catalog.md` | `monster_id` | 58 quái vật (46 Normal + 12 Elite) |
| 2 | `boss_catalog.md` | `boss_id` | 8 trùm (2 PUBLIC + 5 boss phó bản + finale `boss.than_trung`; các giai đoạn finale là phase, không phải boss_id riêng) |
| 3 | `class_skill_catalog.md` | `skill_id` | 60 kỹ năng (12 kỹ năng × 5 phái) |
| 4 | `equipment_catalog.md` | `equipment_id` | 168 trang bị (12 bộ × 14 vị trí) |
| 5 | `item_catalog.md` | `item_id` | Vật phẩm tiêu hao, nguyên liệu, đá nâng cấp |
| 6 | `drop_tables.md` | `drop_table_id` | Bảng rơi đồ quái vật, trùm, rương kho báu |
| 7 | `crafting_catalog.md` | `recipe_id` | 168 công thức chế tạo đảm bảo thành công |
| 8 | `npc_shop_catalog.md` | `shop_id` | Cửa hàng hồi phục, tiện ích, dịch vụ |
| 9 | `quest_catalog.md` | `quest_id` | Nhiệm vụ chính tuyến, phụ tuyến, tuần hoàn |
| 10 | `dungeon_catalog.md` | `dungeon_id` | 5 phó bản thường và biến thể Endgame Lv60 |
| 11 | `world_route_catalog.md` | `map_id` | 24 bản đồ, 52 cổng dịch chuyển, checkpoints |
| 12 | `map_spawn_catalog.md` | `spawn_group_id` | 54 nhóm quái cố định trên 18 field maps |
| 13 | `atlas_catalog.md` | `atlas_page_id` | 104 trang nhật ký dân gian (4 danh mục) |
| 14 | `cosmetic_catalog.md` | `cosmetic_id` | 294 ngoại trang (145 cày cuốc, 13 IAP, 136 mùa) |
| 15 | `soul_catalog.md` | `soul_id` | 25 khế ước linh hồn thu thập |
| 16 | `build_catalog.md` | `entry_id` | Mạch khí (Meridian) và Trận pháp (Formations) |
| 17 | `spirit_beast_catalog.md` | `beast_id` | 10 linh thú đồng hành |
| 18 | `economy_catalog.md` | `table_id` | Bảng tỷ lệ câu cá, cá hiếm, nấu ăn |
| 19 | `world_event_catalog.md` | `event_id` | Lịch trình và biến thể sự kiện Linh Khí Trào Dâng |
| 20 | `encounter_catalog.md` | `encounter_id` | Cơ chế đặc biệt, bẫy môi trường, mật đạo |
| 21 | `progression_route.md` | `level` | Đường cong EXP Lv1..60 (702.100.000 tổng EXP) |
| 22 | `balance_validation.md` | `gate_id` | Các guardrails về TTK, sát thương và chỉ số |
| 23 | `integration_validation.md` | `rule_id` | Kiểm tra toàn vẹn liên kết chéo giữa các catalog |
| 24 | `README.md` | N/A | Chỉ mục và quy chuẩn trạng thái catalog |

`presentation_asset_manifest.md` là manifest quản lý phân nhóm tài nguyên client Addressables, không phải catalog gameplay được nạp vào server runtime. Server content compiler chỉ biên dịch đúng 24 launch catalogs trong bảng trên.

## 4. Quy tắc Mở rộng Hữu hạn (Finite Expansions)

Compiler chịu trách nhiệm mở rộng tự động các bảng dẫn xuất (derived tables) một cách tất định:
1. **Equipment Expansion (168 trang bị):** Mở rộng từ 12 bộ trang bị cơ sở × 14 vị trí trang bị với chỉ số chính, chỉ số phụ và thuộc tính Ngũ Hành theo công thức trong `equipment_catalog.md`.
2. **Portal Graph Expansion (52 cổng chuyển vùng):** Mở rộng thành đồ thị 2 chiều (bản đồ nguồn, tọa độ lối vào, bản đồ đích, tọa độ xuất hiện) từ `world_route_catalog.md`.
3. **Persistent Spawn Groups Expansion (54 nhóm quái):** Mở rộng thành các điểm spawn thực thể với bán kính leash, số lượng quái và thời gian hồi sinh từ `map_spawn_catalog.md`.
4. **Entity Size Resolution:** Resolve toàn bộ monster/boss thành đúng một `size_profile` từ `monster_catalog.md` / `boss_catalog.md`; không suy ra từ sprite, tên hoặc Transform scale.
5. **Playable Space Geometry Index:** Resolve `space_id`, `space_kind`, exact bounds, `layout_profile`, scene key và geometry export cho 24 world maps, 5 dungeons, finale, duel, arena và Guild War. `1280x720` chỉ là viewport; normal-world width phải là `2.0..5.0` screens.
6. **Skill Geometry Resolution:** Resolve exactly 45 primary action geometries (20 basics + 25 actives), every explicit `secondary_geometries[]` row, role-band/envelope checks, and tag/effect parity from `class_skill_catalog.md` under ADR-0047. Prose, animation, sprite bounds, or client distance cannot create geometry.

## 5. Content Hash & Canonical Ordering

1. **Ordering:** Trước khi biên dịch và băm SHA-256, tất cả các hàng trong mỗi bảng phải được sắp xếp theo thứ tự từ điển (lexicographical order) của `Primary Key`.
2. **Content Hash:**
   - Tạo chuỗi chuẩn hóa (canonical string) bằng cách nối các cell cách nhau bởi ký tự `\t`, mỗi dòng kết thúc bằng `\n`.
   - `content_revision = SHA-256(toàn_bộ_24_catalogs_sau_khi_chuẩn_hóa)`.
   - Hash này là định danh bất biến đi kèm trong metadata của server và client để phát hiện lệch phiên bản dữ liệu.

## 6. Diagnostic Codes

Mọi lỗi phát hiện trong quá trình compile phải trả về mã lỗi chuẩn và vị trí file/dòng:

```text
[CATALOG_FILE_NOT_FOUND]     -- Không tìm thấy file catalog theo quy định
[TABLE_SYNTAX_ERROR]         -- Sai cấu trúc bảng Markdown (thiếu cột, sai divider)
[DUPLICATE_PRIMARY_KEY]      -- Trùng lặp khóa chính trong cùng một catalog
[UNRESOLVED_REFERENCE]       -- Tham chiếu tới ID không tồn tại ở catalog khác
[VALUE_OUT_OF_BOUNDS]        -- Giá trị nằm ngoài giới hạn min..max hoặc sai enum
[BALANCE_GUARDRAIL_FAILED]   -- Vi phạm chỉ số cân bằng trong balance_validation.md
[INTEGRATION_CHECK_FAILED]   -- Vi phạm quy tắc liên kết trong integration_validation.md
```

Định dạng log: `[file:line:col] [ERROR_CODE] chi tiết lỗi`.

## 7. Atomic Activation Lifecycle

Vòng đời kích hoạt dữ liệu trong bộ nhớ (`server/internal/config/`):
1. **Candidate Staging:** Khi có phiên bản dữ liệu mới, compiler tải và biên dịch toàn bộ dữ liệu vào một đối tượng bộ nhớ độc lập `CandidateSnapshot`.
2. **Validation Gate:** Chạy kiểm tra toàn bộ 24 catalogs và các bài test integration / balance.
3. **Atomic Commit:**
   - Nếu 100% hợp lệ: Dùng con trỏ nguyên tử (`atomic.Pointer[ContentSnapshot]`) hoán đổi con trỏ đang active sang snapshot mới trong 1 thao tác CPU (zero latency, zero lock).
   - Nếu có ít nhất 1 lỗi: Hủy bỏ toàn bộ `CandidateSnapshot`, phát cảnh báo qua structured log, và **giữ nguyên phiên bản active trước đó**. Hệ thống tiếp tục vận hành bình thường không bị gián đoạn.

## Invariants

```text
nguồn duy nhất = docs/07_content/*.md tables; không có JSON/YAML thứ hai
primary key sorting bắt buộc trước khi tính content hash
bất kỳ lỗi nào cũng reject toàn bộ candidate revision
invalid candidate = previous valid revision stays active
```
