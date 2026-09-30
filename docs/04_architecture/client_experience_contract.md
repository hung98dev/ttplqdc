# Client Experience & Presentation Contract
status: LOCKED

## Scope

Hợp đồng quy chuẩn trải nghiệm người dùng (UX), máy trạng thái giao diện (UI State Machine), bố cục màn hình (HUD Layout), sơ đồ gán phím (Input Action Mapping), vùng an toàn thiết bị di động (Safe Area) và các trạng thái lỗi/chuyển vùng cho Unity client (Unity 6000.6.1f1).

Tài liệu này là đặc tả cho `IMP-065` (Client Bootstrap / Session), `IMP-066` (Input / UI State Machine) và các task giao diện người dùng.

## 1. UI State Machine Tổng thể

Toàn bộ client vận hành dựa trên một Finite State Machine cấp cao duy nhất:

```text
[BOOT]
  -> Kiểm tra phiên bản & nạp Addressables Catalog
[PATCHING_UPDATE]
  -> Tải cập nhật tài nguyên nếu có; hiển thị % và thanh tiến trình
[AUTH_TITLE]
  -> Đăng nhập: tên đăng nhập + mật khẩu, hoặc liên kết Apple / Google / Steam; nút Đăng ký
     (tên đăng nhập, mật khẩu, email; ../07_security/auth.md § Password Provider, ADR-0051)
[LOGIN_QUEUED]
  -> Hàng chờ đăng nhập khi thế giới đầy (SERVER_OVERLOADED + queue_position,
     ../07_security/session.md § Login Queue): hiển thị vị trí, tự thử lại sau retry_after_ms, nút Hủy
[CHARACTER_SELECT]
  -> Chọn hoặc tạo nhân vật (tối đa 3 nhân vật; cấm xóa nhân vật)
[TRANSFERRING_MAP]
  -> Màn hình chờ tải cảnh chuyển vùng (giữ kết nối WSS, tải Addressables nhóm bản đồ mới)
[IN_WORLD]
  -> Vào thế giới thực tế, hiển thị HUD chiến đấu đầy đủ
[DISCONNECTED]
  -> Mất kết nối, hiển thị đếm ngược thử lại tự động (1..5s)
```

Chuyển trạng thái (tên trạng thái dùng thống nhất trong toàn bộ tài liệu và code):
```text
BOOT -> PATCHING_UPDATE (catalog mới) | AUTH_TITLE
PATCHING_UPDATE -> AUTH_TITLE
AUTH_TITLE -> CHARACTER_SELECT (đăng nhập thành công) | LOGIN_QUEUED (SERVER_OVERLOADED có queue_position)
LOGIN_QUEUED -> CHARACTER_SELECT (được nhận) | AUTH_TITLE (Hủy, hoặc mất chỗ sau 60 s không attach)
CHARACTER_SELECT -> TRANSFERRING_MAP (attach) -> IN_WORLD
CHARACTER_SELECT -> AUTH_TITLE (đăng xuất)
IN_WORLD -> TRANSFERRING_MAP (chuyển map/instance) | DISCONNECTED (mất kết nối) | AUTH_TITLE (SESSION_REPLACED, đăng xuất)
IN_WORLD -> CHARACTER_SELECT (Đổi nhân vật: C2S_CHARACTER_DETACH (10) -> S2C_CHARACTER_DETACH_OK (11); bị từ chối thì ở lại IN_WORLD)
TRANSFERRING_MAP.PLACEMENT_PENDING: tiểu trạng thái khi nhận S2C_PLACEMENT_PENDING (15) cho FIRST_LOGIN | RECONNECT | INSTANCE_RETURN;
  hiển thị "Đang chờ chỗ trong khu vực", không có timeout phía client, chờ server tự đặt chỗ (retry_after_ms chỉ để hiển thị);
  ngân sách 30s/120s (§5) chỉ bắt đầu khi rời tiểu trạng thái này (nhận S2C_TRANSFER_PREPARE)
IN_WORLD (nhân vật DEAD) + S2C_PLACEMENT_PENDING reason RESPAWN: ở lại IN_WORLD, hiện lớp phủ chờ hồi sinh, không timeout
DISCONNECTED -> IN_WORLD (resume) | TRANSFERRING_MAP (resume vào map khác) | AUTH_TITLE (hết lượt thử hoặc SERVER_DRAINING sau hạn)
bất kỳ -> AUTH_TITLE khi CLIENT_UPDATE_REQUIRED / PROTOCOL_UNSUPPORTED (kèm hướng dẫn cập nhật)
```

## 2. Bố cục Giao diện In-World (HUD Layout)

Độ phân giải/viewport tham chiếu chuẩn: `1280 × 720` (`16:9`, ADR-0046). Canvas Scaler: `Scale With Screen Size`, Reference Resolution = `1280 × 720`, Match = `0.5`, Reference Pixels Per Unit = `100`; UI sprites are 2x textures imported at `200 PPU` (ADR-0055) so they render at reference size.

Đây chỉ là vùng nhìn camera/UI, không phải kích thước map. Một normal-world map rộng `2..5` viewport và camera cuộn theo nhân vật trong authored camera regions; kích thước cụ thể và topology từng map nằm trong `world_route_catalog.md`.

```text
+-----------------------------------------------------------------------------------+
| [Avatar] Lv.60 TenNhanVat                      [MiniMap: Lang Da - Khu 1] [Ping]  |
| HP: [=======================] 45000/45000      [Quy 3/3]                 [Menu =] |
| MP: [=======================]  8200/ 8200                                         |
| [Buff1][Buff2][Debuff1]                        [Nhiệm vụ:                         |
|                                                  - Diet 10 Chuot Dong (8/10)]     |
|                                                                                   |
|                                                                                   |
| [Chat Dock - 3 dong]                                                              |
| [Tab: The gioi / Bang]                                                            |
|                                                       (Active 4)  (Active 5)      |
|     ( ^ )                                        (Active 2)  (Active 3)           |
| ( <       > ) Virtual Joystick               (Active 1)                           |
|     ( v )                                         [== BASIC ATTACK ==]   (Jump ^) |
+-----------------------------------------------------------------------------------+
```

### Chi tiết các Cụm HUD:
1. **Top-Left (Thông tin Nhân vật):** Ảnh đại diện theo phái, cấp độ, thanh máu (Đỏ), thanh mana (Xanh lam), hàng icon trạng thái (Buff hình khiên/tròn viền xanh, Debuff hình tam giác nhọn viền đỏ kèm số giây đếm ngược).
2. **Top-Right (Bản đồ & Kênh):** Tên bản đồ hiện tại, số hiệu kênh (`Khu 1` .. `Khu 30`), biểu tượng cường độ mạng (Ping ms), nút mở Menu chính.
3. **Right-Center (Bảng Theo dõi Nhiệm vụ):** Tối đa 3 nhiệm vụ hiển thị (1 chính tuyến màu vàng cam, 2 phụ tuyến màu xanh ngọc).
4. **Bottom-Left (Di chuyển & Chat):** Cần điều khiển ảo (Virtual Joystick) trên màn hình cảm ứng; dock chat thu nhỏ hiển thị 3 dòng mới nhất, chạm vào để mở toàn màn hình chat.
5. **Bottom-Right (Hệ thống Kỹ năng):** Nút Đánh Thường (Basic Attack) kích thước lớn nhất, 5 nút Kỹ Năng Kích Hoạt (Active 1..5) xếp hình cánh cung quanh nút đánh thường, nút Nhảy (Jump).

## 3. Sơ đồ Gán Phím (Input Action Mapping)

Sử dụng Unity Input System (`com.unity.inputsystem 1.20.0`):

| Thao tác | Bàn phím & Chuột (PC) | Tay cầm (Gamepad / Xbox) | Màn hình cảm ứng (Mobile) | Gói tin mạng gửi đi |
|---|---|---|---|---|
| **Chạy Trái / Phải** | `A` / `D` hoặc `←` / `→` | Cần gạt trái (Left Stick) | Kéo Virtual Joystick | `C2S_MOVEMENT_EDGE` (108, PRESS \| FLIP) |
| **Dừng di chuyển** | Nhả phím di chuyển | Nhả cần gạt trái | Nhả Virtual Joystick | `C2S_MOVEMENT_EDGE` (108, RELEASE) |
| **Nhảy (Jump)** | `Space` hoặc `W` | Nút `A` | Chạm nút Nhảy | `C2S_JUMP` (101) |
| **Rơi sàn một chiều** | `S + Space` | `Down + A` | Kéo Joystick xuống + Nhảy | `C2S_DROP_THROUGH` (102) |
| **Đánh Thường** | `J` hoặc Chuột Trái | Nút `X` | Chạm nút Đánh Thường | `C2S_BASIC_ATTACK` (201) |
| **Kỹ năng Active 1..5** | `K`, `L`, `U`, `I`, `O` | `Y`, `B`, `RB`, `RT`, `LB` | Chạm nút Active 1..5 | `C2S_SKILL_USE` (200) |
| **Tương tác NPC / Nhặt** | `F` | Nút `LT` | Nút Tương tác ngữ cảnh | `C2S_INTERACT` (103) |
| **Đi qua Portal** | `F` (cùng phím ngữ cảnh) khi portal là đối tượng gần nhất | Nút `LT` | Nút Tương tác ngữ cảnh (hiện biểu tượng cổng) | `C2S_PORTAL_USE` (104) |
| **Đổi Mục tiêu** | `Tab` (mục tiêu kế tiếp), `Esc` (bỏ chọn) | `R3` (Nhấn cần phải) | Chạm trực tiếp vào quái; chạm nền trống để bỏ chọn | `C2S_TARGET_INTENT` (202; `target_entity_id = 0` = bỏ chọn) |
| **Mở Chat** | `Enter` | Nút `Back` / `View` | Chạm vào Chat Dock | N/A (Mở UI nội bộ) |

Quy tắc gửi input:
- Ngữ cảnh `F`/`LT`/nút ngữ cảnh: chọn đối tượng tương tác gần nhất trong tầm theo khoảng cách tới điểm neo nhân vật; portal thắng khi hòa. Đối tượng là portal → 104, còn lại → 103; không phím nào khác gửi 104.
- Hướng di chuyển giữ (`C2S_INPUT_STATE`, 100): gửi trong pha Input khi `input_flags` đổi, tối đa một lần mỗi `50 ms` (20/s); khi còn giữ ít nhất một cờ thì gửi lại tối thiểu mỗi `250 ms`. Cạnh nhấn/nhả/đổi hướng luôn gửi ngay bằng 108, không bị gộp. Không bao giờ vượt giới hạn `../07_security/rate_limits.md`.
- `Tab` duyệt hostile trong AOI theo khoảng cách tăng dần (hòa → `entity_id` nhỏ hơn); mọi đổi mục tiêu đi qua 202, client chỉ hiển thị mục tiêu server đã chấp nhận.

## 4. Vùng An toàn & Ma trận Thiết bị (Safe Area & Device Matrix)

### 4.1 Resolution và aspect ratio

- Desktop mặc định mở cửa sổ `1280x720`; người chơi được phép chọn fullscreen/native hoặc độ phân giải lớn hơn.
- Gameplay camera tại 16:9 luôn nhìn `25.6m x 14.4m`, orthographic size `7.2m`, theo `physics_geometry_contract.md`.
- Màn hình rộng hơn 16:9 giữ chiều cao world view và mở rộng chiều ngang tối đa 21:9; phần vượt 21:9 dùng pillarbox.
- Màn hình hẹp hơn 16:9 dùng letterbox để không cắt gameplay/HUD hoặc làm thay đổi vùng nhìn cạnh tranh.
- UI scale theo Canvas Scaler và safe area. Không đặt vị trí HUD bằng pixel tuyệt đối ngoài mặt phẳng tham chiếu `1280x720`.
- Thay đổi độ phân giải không thay `ART_PIXELS_PER_METER`, collider, movement speed, map bounds hoặc khoảng cách kỹ năng.

1. **Khắc phục Tai thỏ / Dynamic Island / Nốt ruồi camera:**
   - Sử dụng component `SafeAreaFitter` trên Canvas gốc.
   - Truy vấn `Screen.safeArea` khi khởi động và khi xoay màn hình để tự động chèn padding lề trái/phải tối thiểu `48px` trên các thiết bị có viền khuyết tật.
2. **Ngăn chặn xung đột cảm ứng (Touch Gesture Conflicts):**
   - Vùng Virtual Joystick chiếm nửa dưới bên trái màn hình (`X: 0..0.4`, `Y: 0..0.5`).
   - Chạm trong vùng Joystick không kích hoạt mở Chat Dock hay bấm vào thông tin nhân vật.

## 5. Trạng thái Lỗi, Chuyển Vùng & Mất Kết Nối

1. **Chuyển vùng bản đồ (`TRANSFERRING_MAP`):**
   - Khóa toàn bộ input di chuyển và chiến đấu của người chơi.
   - Hiển thị màn hình mờ với tranh dân gian đặc trưng của vùng đất sắp đến.
   - Thanh tiến trình hiển thị tiến độ tải Addressables.
   - Nếu quá ngân sách `30s` (hoặc `120s` với phó bản), tính từ `S2C_TRANSFER_PREPARE`, tự động hủy và hiển thị thông báo `Chuyển vùng thất bại, đang quay lại điểm an toàn`. Thời gian ở tiểu trạng thái `PLACEMENT_PENDING` không tính vào ngân sách này.
2. **Mất kết nối mạng (`DISCONNECTED`):**
   - Hiển thị popup modal giữa màn hình: `Mất kết nối tới máy chủ. Đang thử kết nối lại... (Lần 1/5)`.
   - Nút `Thử lại ngay` và nút `Thoát ra màn hình chính`.
3. **Đăng nhập đè phiên (`SESSION_REPLACED`):**
   - Nhận `S2C_SESSION_REPLACED` (ID 8, `../05_network/messages.md`) từ server: Lập tức đóng kết nối WSS.
   - Hiển thị thông báo không thể đóng: `Tài khoản của bạn đã được đăng nhập từ một thiết bị khác.` kèm nút `Đồng ý` để quay về màn hình Title.

## 6. Khả năng Tiếp cận (Accessibility)

1. **Tương phản số liệu:** Toàn bộ chữ số sát thương bay (floating combat text) và thanh máu phải có viền đen dày `2px` (outline) để đọc rõ trên mọi nền địa hình sáng/tối.
2. **Phân biệt bằng hình dạng:** Không bao giờ dùng màu sắc đơn độc để biểu thị trạng thái.
   - Buff có khung viền tròn và icon mũi tên hướng lên.
   - Debuff có khung viền tam giác và icon đầu lâu/vết rách hướng xuống.
3. **Kích thước nút bấm tối thiểu:** Nút cảm ứng trên di động có kích thước tối thiểu `44 × 44` points để người chơi không bị bấm trượt.

## Invariants

```text
độ phân giải tham chiếu = 1280x720; Canvas Scaler match = 0.5
1280x720 là viewport, không phải map size; normal-world map rộng 2..5 viewport
desktop default window = 1280x720; fullscreen/native vẫn được hỗ trợ
SafeAreaFitter bắt buộc để tránh tai thỏ camera
input di chuyển gửi qua C2S_MOVEMENT_EDGE (108)
SESSION_REPLACED = modal ngắt kết nối không thể đóng
mỗi nút gamepad/phím gán đúng một thao tác trong cùng ngữ cảnh IN_WORLD
tên trạng thái UI: BOOT, PATCHING_UPDATE, AUTH_TITLE, LOGIN_QUEUED, CHARACTER_SELECT, TRANSFERRING_MAP, IN_WORLD, DISCONNECTED
icon trạng thái phân biệt bằng hình dạng (tròn/tam giác), không chỉ bằng màu
```
