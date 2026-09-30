# Presentation Asset Manifest & Addressables Delivery Schema
status: LOCKED

## Scope

Đặc tả cấu trúc nhóm tài nguyên Addressables (Unity Addressables 2.11.2), quy chuẩn định danh tài nguyên (Asset Keys), ngân sách dung lượng tải/bộ nhớ và quy trình bàn giao tài nguyên đồ họa/âm thanh cho game Thỉnh Thần.

Tài liệu này là đặc tả cho `IMP-063` (Addressables Asset Pipeline) và `IMP-070..076`, `IMP-104`, `IMP-105` (sản xuất, truy xuất nguồn gốc, kiểm tra asset phát hành). `IMP-063` chỉ dựng pipeline; nó không bàn giao mỹ thuật/âm thanh cuối cùng.

## 1. Cấu trúc Nhóm Addressables & Ngân sách Bộ nhớ

Tên nhóm, nội dung nhóm và quy tắc khóa là canonical tại `../04_architecture/client_assets.md` § Grouping / § Stable Asset Keys (ADR-0071). Mục này là nguồn canonical duy nhất cho ngân sách và thời điểm nạp/giải phóng:

| Nhóm Addressables | Ngân sách nén tải về | Ngân sách RAM runtime | Nạp / giải phóng |
|---|---:|---:|---|
| `bootstrap.local` | ≤ 12 MB (trong player) | ≤ 24 MB | nạp ở `BOOT`; giữ suốt phiên |
| `shared.local` | ≤ 70 MB (trong player) | ≤ 110 MB | nạp ở `AUTH_TITLE`; giữ suốt phiên |
| `icons.shared` | ≤ 20 MB | ≤ 32 MB | nạp ở `CHARACTER_SELECT`; giữ suốt phiên |
| `beast.shared` | ≤ 20 MB | ≤ 36 MB | nạp ở `CHARACTER_SELECT`; giữ suốt phiên |
| `cosmetic.shared` | ≤ 60 MB | ≤ 48 MB (chỉ phần đang hiển thị) | Pack Separately; nạp từng cosmetic khi entity hiển thị cần; giải phóng khi không còn entity nào dùng |
| `region.<zone_key>` (6) | ≤ 60 MB mỗi vùng | ≤ 120 MB | nạp khi đích chuyển map thuộc vùng; giải phóng khi rời vùng (kể cả khi vào instance) |
| `dungeon.<dungeon_key>`, `dungeon.finale`, `pvp.shared` | ≤ 25 MB mỗi nhóm | ≤ 60 MB | nạp khi vào instance/trận; giải phóng khi rời |
| `audio.bgm.<zone_key>` (6), `audio.bgm.shared` | ≤ 15 MB mỗi nhóm | ≤ 4 MB (bộ đệm streaming) | stream khi vào map dùng BGM đó; không nạp cả file vào RAM |

```text
RAM runtime            = bộ nhớ texture tính từ định dạng x kích thước x số mip đã import + mesh + audio đã giải nén,
                         tính tất định từ import settings (validator IMP-063), không đo từ process
resident steady        tổng RAM các nhóm đang nạp <= 450 MB   (tối đa: bootstrap + shared + icons + beast + cosmetic + 1 region
                         hoặc 1 dungeon/pvp)
resident transfer peak <= 570 MB (nhóm đích nạp trước khi nhóm nguồn giải phóng)
ràng buộc              resident transfer peak + engine/managed/native <= 1.3 GB resident của ANDROID_MIN
                         (../04_architecture/client_performance.md § Memory and GC)
base install           bootstrap.local + shared.local <= 82 MB nén
```

## 2. Quy chuẩn Định danh Khóa Tài nguyên (Asset Key Namespace)

Quy tắc khóa duy nhất: `../04_architecture/client_assets.md` § Stable Asset Keys (`asset.<catalog_id>.<facet>` giữ nguyên ID catalog kể cả tiền tố loại; asset không thuộc catalog dùng `asset.<kind>.<name>.<facet>`; không có đoạn variant). Facet bắt buộc theo loại nội dung:

```text
nhân vật hệ phái / quái / boss / Linh Thú / NPC   prefab (+ portrait nếu UI dùng)
skill                                              vfx, icon
item / equipment / cosmetic                        icon; cosmetic appearance thêm prefab
map / dungeon / instance / PvP space               scene; bgm nếu map có BGM riêng (nếu không: PresentationAlias tới asset.bgm.<name>.bgm)
SFX cue (§6)                                       asset.sfx.<cue>.clip
```

## 3. Sprite Import và Entity Scale

ADR-0046, ADR-0055 và `physics_geometry_contract.md` quy định:

```text
Reference px      = 50 px/m (ART_PIXELS_PER_METER); mọi giới hạn px trong spec là reference px
TEXTURE_SCALE     = 2  -> texture px = 2 x reference px
Pixels Per Unit   = 100 (import), nên kích thước thế giới không đổi; UI 200; PARALLAX_FAR 1x = 50 (§3.1)
Filter Mode       = Bilinear
Mesh Type         = Tight khi cạnh dài texture >= 256 texture px và có lề trong suốt (a = 0) ở bất kỳ cạnh nào;
                    còn lại Full Rect (../04_architecture/client_performance.md § Smoothness by Construction item 6)
Pivot = Bottom Center (0.5, 0.0)
Generate Physics Shape = false
Mip Maps = false cho gameplay sprite 2D
Transform scale = (1,1,1)
```

Flip hướng dùng `SpriteRenderer.flipX`; cấm dùng negative scale. Collision/hitbox đến từ canonical profile, không đến từ alpha/physics shape của sprite.

| `size_profile` | Silhouette idle tối đa (ref px) | Cell mỗi frame (ref px) | Cell texture thực (2x) | Collider tham chiếu (ref px) |
|---|---:|---:|---:|---:|
| `CHARACTER` | `64x96` | `96x128` | `192x256` | `40x90` |
| `MONSTER_SMALL` | `50x50` | `64x64` | `128x128` | `30x30` |
| `MONSTER_MEDIUM` | `75x100` | `96x128` | `192x256` | `50x70` |
| `MONSTER_ELITE` | `125x150` | `160x192` | `320x384` | `80x120` |
| `BOSS_LARGE` | `200x220` | `256x256` | `512x512` | `120x160` |
| `WORLD_BOSS` | `250x280` | `320x320` | `640x640` | `150x200` |
| `SPIRIT_BEAST` | `48x48` | `64x64` | `128x128` | không có (companion chỉ hiển thị, `../03_systems/spirit_beasts.md`) |

Mọi Linh Thú dùng `SPIRIT_BEAST`. Asset không có `size_profile` khai báo `cell_ref` (reference px, bội số của 16, tối đa `512x512`) trong metadata import: `PROP` và `VFX_SOFT`/VFX gameplay theo khai báo đó; texture = đúng 2 x `cell_ref`. `PROP` áp Đệm cell và Kích thước cell của §3.2 theo `cell_ref`; VFX chỉ áp "texture = 2 x cell_ref". Icon item/equipment/skill/cosmetic = `64x64` ref (§3.1).

Cell được phép có transparent padding; silhouette không được tự co giãn để lấp cell. Với nhân vật, body idle/run/jump cao `88..96px`; tóc/trang phục/vũ khí có thể vượt tối đa `8px` mỗi phía nhưng phải nằm trong cell. VFX/weapon trail vượt cell là asset con riêng.

### 3.1 Kích thước vẽ (Authoring Resolution)
- Mỗi texture cuối cùng phải được vẽ/hoàn thiện **đúng kích thước 2x mục tiêu**: cell theo bảng trên; icon UI `64x64` ref → `128x128`; UI/HUD theo mặt phẳng `1280x720` ref → `2560x1440`, sprite UI import `200 PPU` (Canvas Reference PPU `100`) để kích thước hiển thị bằng reference px; tile `50x50` ref → `100x100`.
- AI hoặc họa sĩ có thể tạo ở kích thước lớn hơn, nhưng bước hoàn thiện cuối (thu nhỏ bằng bộ lọc area/Lanczos theo tỉ lệ nguyên hoặc hữu tỉ, rồi làm sạch nét/viền và sharpen) phải làm ở đúng kích thước 2x. Cấm để Unity resize (`Max Size` phải ≥ kích thước thật; không `Non-Power-of-2` scaling).
- Nét viền/chi tiết quan trọng dày tối thiểu `2` texture px (= 1 ref px) để còn đọc được ở 720p.
- Parallax xa (lớp `L3`/`L4`, `asset_class = PARALLAX_FAR`) được phép `TEXTURE_SCALE = 1` để tiết kiệm bộ nhớ, khi đó import `50 PPU` để kích thước thế giới không đổi; mọi lớp gameplay, nhân vật, quái, boss, Linh Thú, vật phẩm, UI, icon, VFX gameplay và telegraph là 2x.
- Nén: nhân vật/quái/boss/Linh Thú/UI/icon/font = ASTC 4x4 (mobile), BC7 (desktop); nền/parallax = ASTC 6x6 / BC7. Sprite gameplay tắt mipmap.
- Frame animation: chiều cao silhouette giữa các frame idle lệch ≤ `4` texture px; pivot không trôi.

### 3.1a Phạm vi gate theo loại asset
| asset_class | Cutout Gate §3.2 | Volume Gate §3.6 | Ghi chú |
|---|---|---|---|
| `ACTOR` (nhân vật, quái, boss, Linh Thú), `COSMETIC_APPEARANCE` | toàn bộ | toàn bộ | |
| `PROP`, `ITEM_ICON`, `EQUIPMENT_ICON` | toàn bộ | toàn bộ trừ "Actor trên nền" | |
| `UI_ART` (khung, nút, 9-slice) | Định dạng, Dải bán trong suốt, Viền màu, Pixel trong suốt | miễn | 9-slice: không áp Đệm cell/Kích thước cell; biên ngoài được phép cứng |
| `FONT_ATLAS` (SDF) | miễn | miễn | kiểm tra glyph coverage (IMP-073) |
| `TILE` (L1 gameplay, lặp) | Định dạng, Viền màu, Pixel trong suốt | Dải giá trị, Sáng từ trên | cạnh tile có thể đặc để lát liền |
| `PARALLAX_NEAR` (L0/L2) | như `PROP` | Môi trường | 2x |
| `PARALLAX_FAR` (L3/L4) | Định dạng, Viền màu | Môi trường | được phép 1x (§3.1) |
| `VFX_SOFT` (additive, khói, hạt, telegraph mềm) | chỉ Định dạng | miễn | khai báo `soft_edges`; đọc được theo §3.3 |
Mỗi file khai báo `asset_class` trong metadata import; validator áp đúng cột trên. Khai báo sai loại để né gate là vi phạm review.

### 3.2 Cutout Quality Gate (tự động, theo phạm vi §3.1a)
Nền phải được tạo sẵn trong suốt (native alpha) hoặc trên nền phẳng màu khóa `#FF00FF` không có trong bảng màu asset, rồi mới tách nền. Cấm đưa thẳng kết quả "remove background" tự động vào build mà không qua gate này. Đo trên texture 2x cuối cùng:

```text
Định dạng          PNG RGBA 8-bit/kênh, sRGB, có kênh alpha thật; 4 góc 4x4 px của mỗi cell phải alpha = 0
                   (bắt lỗi nền ca-rô giả hoặc nền đặc bị nướng vào ảnh)
Dải bán trong suốt pixel có 1 <= a <= 254 phải nằm trong 2 px (Chebyshev) quanh một pixel a = 255,
                   trừ pixel nằm trong mask translucent đã khai báo
                   -> chặn vệt lem, bóng mờ, khói nền còn sót
Viền màu (fringe)  pixel viền = pixel có 1 <= a <= 254 kề (8-connectivity) một pixel a = 255;
                   0 pixel viền có hue trong ±15° của màu khóa với saturation (HSV) > 0.30;
                   0 pixel viền có |ΔL*| > 35 so với pixel a = 255 gần nhất (Euclid; hòa -> thứ tự quét hàng) (chặn viền trắng/đen)
Pixel trong suốt   mọi pixel a = 0 có khoảng cách Chebyshev <= 4 px tới một pixel a > 0 phải mang RGB của pixel a = 255 gần nhất
                   (Euclid; hòa -> thứ tự quét hàng); pixel a = 0 xa hơn 4 px không bị ràng buộc RGB
                   -> không có quầng tối/sáng khi lọc bilinear/atlas
Đốm rác            mọi thành phần liên thông (8-connectivity, a >= 16) tách khỏi thân chính phải có diện tích >= 64 px;
                   asset có phần rời hợp lệ (vũ khí bay, VFX hạt) phải khai báo detached_parts
Răng cưa           >= 50% pixel biên (a > 0 kề a = 0) phải có 1 <= a <= 254 (biên được khử răng cưa);
                   cutout nhị phân 0/255 bị reject, trừ asset khai báo pixel_art = true (không có ở launch)
Lỗ trong thân      0 pixel a < 250 bị bao kín trong silhouette, trừ vùng khai báo translucent (ma, khói, nước)
Đệm cell           silhouette cách mép trái/phải/trên cell >= 4 texture px; mép dưới 0..4 px (chân chạm đất)
Kích thước         texture = đúng 2 x cell ref; bbox silhouette (a >= 128) <= 2 x giới hạn; thân nhân vật cao 176..192 px
```
Mask translucent: file `<texture>.translucent.png` cùng kích thước, 1-bit (trắng = translucent), khai báo trong metadata import; chỉ `ACTOR` loại ma/linh hồn, khói, nước được dùng; diện tích mask ≤ 60% silhouette. Mask miễn trừ đúng các dòng ghi "translucent" ở §3.2/§3.6, không miễn trừ dòng khác.
Validator ghi số đo từng file vào báo cáo; một vi phạm là fail. Ngưỡng chỉ được nới bằng ADR (gate ratchet, ADR-0050).

### 3.3 Duyệt hiển thị trong game (Visual Review Gate)
Ảnh review được render trên job Linux của CI (GitHub-hosted, không GPU) bằng Mesa llvmpipe dưới xvfb (ADR-0058); metadata ảnh ghi `renderer=llvmpipe`. Ảnh chỉ dùng để soi hình ảnh, không dùng cho số liệu hiệu năng GPU.
Mỗi entity/UI được chụp trong các scene review `client/Assets/Scenes/Review/` (IMP-070), dựng từ các lớp map thật của vùng/instance mà entity xuất hiện, ở `1280x720`, `1920x1080` và profile điện thoại `2400x1080`, cả ngày và đêm, ở zoom 100% và 200%. Job Linux render và upload artifact `visual-review`; ảnh là review artifact được manifest evidence tham chiếu, không commit và không phải evidence. Agent `reviewer` (khác người tạo) ghi kết luận vào PR review comment và đặt `review_state` của bản ghi nguồn:
- không thấy viền lem, quầng màu, răng cưa hay đốm rác ở cả hai mức zoom;
- silhouette đọc rõ trên nền: `ΔL*` trung bình giữa dải biên actor và nền cục bộ ≥ 20 (cùng phép đo "Actor trên nền" §3.6), hoặc asset có outline;
- telegraph/VFX đọc được mà không phụ thuộc chỉ vào màu (`../00_context/constraints.md`);
- nét sắc ở 1920x1080 (không mờ do phóng to) và không nhiễu/nhấp nháy khi di chuyển ở 1280x720;
- trông có khối, không phẳng như tranh: hướng sáng thống nhất với cảnh, đọc được 3 mặt phẳng độ sâu (foreground / gameplay / background), actor nổi rõ hơn nền cả ngày lẫn đêm dưới ánh sáng runtime.
Task sản xuất (IMP-071..075, IMP-104, IMP-105) tham chiếu artifact `visual-review` của lần chạy CI trong manifest evidence; IMP-076 kiểm lại.

### 3.5 Art Direction — Painted-Volume 2D Chibi (ADR-0056)
Asset không được trông như tranh phẳng. Mọi actor, prop, vật phẩm và lớp môi trường gameplay tuân thủ:

```text
Ánh sáng chính   một nguồn chung từ trên-trước (top, hơi về phía camera), đối xứng trái/phải để flipX không làm sai hướng sáng
Giá trị (value)  3 tầng rõ: sáng / trung gian / bóng + ambient occlusion ở nếp gấp, khe, chỗ tiếp xúc
Khối             bóng đổ nội bộ (mũ lên mặt, tay lên thân), gradient chuyển trên khối trụ/cầu, chồng lớp trước-sau
Rim light        viền sáng 2..4 texture px ở mép trên/sau của silhouette để tách khỏi nền
Outline          viền màu (tông tối của màu nền cục bộ, không đen thuần) 2..4 texture px; nét trong mảnh hơn nét ngoài
Góc nhìn actor   3/4 nghiêng (không profile phẳng, không chính diện)
Chất liệu        highlight khác nhau cho vải / tre / gỗ / đồng / sơn mài / nước; kim loại có specular nhỏ, sắc
Bóng chạm đất    runtime contact shadow (ellipse mềm) — không vẽ bóng dưới chân vào sprite
Màu              entity bão hòa và tương phản cao hơn nền; nền giảm bão hòa theo độ sâu
```

Môi trường dùng 5 lớp độ sâu:

| Lớp | Nội dung | Parallax | Quy tắc |
|---|---|---:|---|
| `L0` foreground | cây/rèm/cột che phía trước | 1.10..1.20 | tối hơn, ≤ 15% diện tích màn hình, không che telegraph |
| `L1` gameplay | tile, nền đứng, prop tương tác | 1.00 | tương phản cao nhất của cảnh; nền đứng có mặt trên (lip) sáng + mặt trước tối + bóng đổ dưới mái/chìa |
| `L2` near background | nhà, hàng rào, cây gần | 0.60..0.80 | tương phản thấp hơn L1 |
| `L3` mid background | làng xa, đồi, rừng | 0.30..0.50 | nhạt dần về màu trời, ít bão hòa |
| `L4` far / sky | núi xa, trời, mây | 0.00..0.15 | tương phản thấp nhất, lạnh/sáng hơn |

`generation_record.prompt` của asset AI phải chứa các chỉ dẫn ánh sáng/khối ở trên; asset chỉ đạt khi qua §3.6 và Visual Review.

### 3.6 Volume & Depth Gate (tự động)
Đo trên texture 2x cuối, trong silhouette S = pixel a ≥ 128 (mask translucent bị loại khỏi S), màu sRGB → CIELAB (D65). Một thước đo độ sáng duy nhất cho mọi gate/review: `L*` / `ΔL*` CIELAB.

```text
Định nghĩa       dải biên B   = pixel của S có khoảng cách Chebyshev <= 3 px tới một pixel ngoài S
                 lõi kề K(p)  = pixel của S có khoảng cách Chebyshev 5..8 px tới ngoài S; với mỗi p ∈ B lấy pixel K gần nhất
                                (Euclid; hòa -> thứ tự quét hàng)
                 vùng liền màu = thành phần liên thông 8-connectivity của đồ thị trên S, cạnh nối hai pixel kề có ΔE00 < 2
Dải giá trị      L*(p95) - L*(p5) >= 40
Tầng giá trị     k-means 1 chiều k=5 trên L*, tâm khởi tạo = L* tại p10, p30, p50, p70, p90, lặp Lloyd tới khi phân cụm
                 không đổi hoặc 100 vòng (tất định, không seed): >= 3 cụm, mỗi cụm >= 5% diện tích S
Sáng từ trên     mean L* của 1/3 trên bbox(S) - mean L* của 1/3 dưới >= 6
Tách viền        >= 60% pixel p ∈ B có |L*(p) - L*(K(p))| >= 12 (rim light hoặc outline)
Không mảng phẳng không vùng liền màu nào > 20% diện tích S
Môi trường       đo trên render review 1280x720 ban ngày của từng lớp riêng (các lớp khác ẩn), pixel a >= 128 của lớp đó:
                 contrast L*(p95-p5): L1 >= L2 >= L3 >= L4 và L4 <= 0.5 x L1; bão hòa (C*ab) trung bình giảm dần L1 -> L4
Actor trên nền   trên render review (§3.3): mean L* của B - mean L* của nền trong vành 4..12 px ngoài S, |chênh| >= 20
```
Vi phạm là fail; ngưỡng chỉ nới bằng ADR (gate ratchet).

### 3.4 File gốc
File làm việc lớn (PSD/PSB/ảnh AI gốc) lưu qua LFS ngoài thư mục Addressables và không vào build; sổ nguồn gốc ghi hash cả file gốc và file cuối.

Map extent trong catalog là độ phủ tham chiếu, không phải yêu cầu một texture bitmap duy nhất. Tilemap, props tái sử dụng và parallax layers ghép thành scene; không import ảnh nền `2560..6400px` như một collider hoặc một sprite gameplay duy nhất.

## 4. Vòng đời Tài nguyên: Placeholder vs. Release Candidate

Để đảm bảo việc triển khai kỹ thuật không bị đình trệ vì tiến độ vẽ mỹ thuật:

1. **Giai đoạn Milestone Nội bộ (M0 .. M4):**
   - Cho phép sử dụng **Placeholder Assets**.
   - Sprite placeholder dùng đúng cell/silhouette của `size_profile`, vẽ viền collider riêng theo `physics_geometry_contract.md`; không kéo hình khối AABB thành toàn bộ silhouette.
   - Bắt buộc phải gắn đúng `Addressable Key` chuẩn ngay từ đầu.
   - Nhãn chữ hiển thị tên entity trên bề mặt sprite để nhận diện lúc test.
2. **Giai đoạn Release Candidate (M10):**
   - 100% tài nguyên trong phạm vi phát hành phải là asset hoàn thiện (Production Quality), không chấp nhận concept, ảnh mẫu hay asset mặc định của công cụ làm sản phẩm cuối.
   - Kiểm tra không còn bất kỳ asset nào mang nhãn placeholder.
   - Toàn bộ font chữ tiếng Việt hiển thị trọn vẹn dấu thanh Unicode, không lỗi ô vuông/ký tự lạ (`tofu`).

## 5. Nguồn asset và quyền sử dụng

AI agent chịu trách nhiệm tạo hoặc tìm, chỉnh sửa, tích hợp và kiểm tra **mọi** hình ảnh, animation, UI, VFX, font và âm thanh thuộc phạm vi phát hành. Có hai nguồn hợp lệ:

| `source_kind` | Điều kiện |
|---|---|
| `AI_CREATED` | Agent tự vẽ/tạo bằng công cụ tạo ảnh/âm thanh do chủ repo cung cấp và ghi trong Owner Setup (`../10_implementation/audit_gates.md`) và `../00_context/technology_versions.md` § Content production tools (ADR-0072); chưa ghi thì final-art task chưa sẵn sàng, các task khác dùng placeholder theo §4 tới M10. Ghi công cụ, phiên bản, điều khoản sử dụng tại thời điểm tạo, prompt/nguồn tham chiếu và thao tác hậu kỳ. Không dùng tên nghệ sĩ còn sống, thương hiệu, nhân vật/asset có bản quyền hoặc ảnh tham chiếu không có quyền làm chỉ dẫn sao chép. |
| `FREE_LICENSED` | Agent tự tìm trên mạng và tải bản gốc từ trang tác giả/nguồn phát hành; giá sử dụng bằng 0. Chỉ nhận `CC0-1.0`, `CC-BY-4.0`, hoặc `OFL-1.1` **chỉ cho font**. Với `CC-BY-4.0`, ghi tác giả, URL nguồn, URL giấy phép và thay đổi trong credits. Với font `OFL-1.1`, đóng gói copyright notice + toàn văn OFL; nếu sửa font có Reserved Font Name thì đổi tên theo giấy phép. |

Điều kiện công cụ AI và giấy phép nguồn phải được kiểm tra **ở thời điểm lấy/tạo asset**; “tải miễn phí”, “royalty-free” hoặc một trang tổng hợp không ghi chủ sở hữu/giấy phép không đủ bằng chứng. Không dùng `NC`, `ND`, `SA`, editorial-only, trial, nguồn bị nghi lấy cắp, hay giấy phép riêng chưa được chấp thuận. Nếu không chứng minh được quyền sử dụng thương mại, **dừng asset đó**, tự tạo asset khác hoặc chọn nguồn hợp lệ khác; không âm thầm thay bằng placeholder. `CC0`/`CC BY` không tự giải quyết quyền hình ảnh cá nhân, nhãn hiệu hay hình tượng văn hóa nhạy cảm. Quy tắc cultural review của `cosmetic_catalog.md` vẫn áp dụng. Tham chiếu giấy phép chính thức: [CC0-1.0](https://creativecommons.org/publicdomain/zero/1.0/), [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/), [OFL-1.1](https://openfontlicense.org/open-font-license-official-text/).

Nét vẽ final phải thống nhất stylized 2D chibi và bản sắc dân gian Việt trong `../00_context/constraints.md`; đúng profile/cell ở Mục 3, silhouette và telegraph đọc được trên mobile. Tài nguyên tìm được có thể là nguyên liệu để agent biên tập thành sản phẩm cuối, không được sao chép nhận diện game tham khảo. Asset đưa vào repo/LFS và Addressables; không hotlink tới URL của bên thứ ba lúc chạy game.

## 6. Sổ nguồn gốc asset

`client/Assets/Art/Provenance/asset_source_register.json` là sổ **một bản ghi cho mỗi file media hình/âm thanh/font được đóng gói**. `IMP-070` tạo sổ rỗng; `IMP-071..075` ghi các fragment riêng tại `client/Assets/Art/Provenance/fragments/`; `IMP-076` hợp nhất chúng thành sổ phát hành. Mọi JSON có `schema_version: 1` và mảng `assets` sắp theo `file_path` tăng dần. Mỗi bản ghi có:

```text
asset_key          Addressable key ổn định; file phụ điền key của asset cha
file_path          đường dẫn repo chuẩn hóa, duy nhất
content_id         ID catalog nếu có; null cho asset chung
source_kind        AI_CREATED | FREE_LICENSED
creator            tác giả gốc hoặc tên công cụ + agent tạo
source_uri         URL trang gốc; null cho AI_CREATED
license_id         CC0-1.0 | CC-BY-4.0 | OFL-1.1 | AI_TOOL_TERMS
license_uri        URL điều khoản/giấy phép chính xác
acquired_at_utc    thời điểm lấy hoặc tạo (ISO 8601 UTC)
source_sha256      hash file đầu vào; bằng final_sha256 nếu không sửa
final_sha256       hash file được đưa vào build
changes            mô tả biến đổi; "none" nếu không có
attribution        dòng credit phát hành; null nếu không bắt buộc
generation_record  {tool, version, terms_uri, prompt, reference_uris} nếu dùng AI tạo/chỉnh; null nếu không
inputs             [] hoặc danh sách {creator, source_uri, license_id, license_uri, acquired_at_utc, sha256} cho nguồn ngoài dùng tạo/ghép
review_state       PENDING | APPROVED | REJECTED
```

Không ghi URL tìm kiếm thay cho URL nguồn gốc. Với `AI_CREATED` hoặc asset tải về rồi chỉnh bằng AI, `generation_record` phải chỉ ra điều khoản cho phép phân phối thương mại; mọi ảnh/âm thanh đầu vào bên thứ ba phải hiện trong `inputs` với giấy phép hợp lệ. `APPROVED` chỉ khi metadata, file/hash, giấy phép/điều khoản, thẩm mỹ và quyền liên quan đã được kiểm tra. File bị `PENDING` hoặc `REJECTED` không được vào release. Danh sách attribution của mọi bản ghi `CC-BY-4.0` và notice của font `OFL-1.1` phải được sinh từ sổ này và đóng gói để người chơi truy cập được trong credits.

Coverage được tính từ **toàn bộ catalog và feature phát hành**, không chỉ các key đã có trong Addressables: nhân vật/animation, quái/boss/Linh Thú, bản đồ/props/parallax, UI/font/icon, vật phẩm/equipment/cosmetics, skill VFX/telegraph, SFX/BGM và scene. Mỗi ID cần presentation có đúng một key hoặc một mapping tường minh tới asset chia sẻ; asset chia sẻ vẫn có sổ nguồn. Không tạo bản vẽ chỉ để đạt đủ số lượng nếu asset chia sẻ hợp lý và không làm mất nhận diện riêng của entity/map.

Âm thanh tối thiểu: mỗi trong 33 scene phát hành có BGM key hoặc mapping tường minh tới một BGM chia sẻ; các cue `ui_confirm`, `ui_cancel`, `ui_error`, `jump`, `land`, `basic_attack`, `hit`, `guard`, `just_guard_success`, `skill_cast`, `boss_telegraph`, `item_pickup`, `quest_complete`, `map_transfer` có SFX key. Mọi cue khác được runtime/UI tham chiếu cũng phải phân giải trước M10. Một file có thể phục vụ nhiều cue nếu mapping công khai và không làm mất phản hồi quan trọng.

## 7. Kiểm tra Tự động trong CI (Asset Verification Gate)

Script kiểm tra tự động `scripts/verify_assets.ps1` (hoặc test EditMode `AddressablesValidationTests.cs`) thực thi các bước:
1. Quét toàn bộ 24 files catalog trong `docs/07_content/` để trích xuất danh sách asset references.
2. Đối chiếu 1-1 với Addressable Asset Database của Unity.
3. Báo lỗi và dừng quy trình build nếu:
   - Có ID trong catalog nhưng không tìm thấy Addressable Key tương ứng.
   - Dung lượng của bất kỳ AssetBundle nào vượt quá ngân sách quy định ở Mục 1.
   - Định dạng/nén texture sai Mục 3.1 (ASTC 4x4 / ASTC 6x6 / BC7 theo loại asset).
   - Gameplay sprite sai `100 PPU`, kích thước 2x, Bottom Center pivot, cell, silhouette limit hoặc Transform scale `(1,1,1)`.
   - Bất kỳ texture có alpha nào vi phạm Cutout Quality Gate (Mục 3.2) hoặc Volume & Depth Gate (Mục 3.6), hoặc thiếu ảnh trong artifact `visual-review` của lần chạy CI (Mục 3.3).
   - Key sai quy tắc `../04_architecture/client_assets.md` § Stable Asset Keys, asset ngoài nhóm canonical, hoặc tổng RAM tính tất định vượt ngân sách resident Mục 1.
   - Mesh Type sai quy tắc Mục 3 hoặc `PARALLAX_FAR` 1x không import `50 PPU`.
   - Scene map thiếu Addressable key, geometry export, hoặc extent không khớp bounds catalog.
   - Asset final không có bản ghi nguồn, hash sai, `review_state != APPROVED`, giấy phép/điều khoản không hợp lệ, attribution/notice bắt buộc vắng mặt, hoặc còn placeholder.

`IMP-076` chạy gate này cho toàn bộ release scope trước `IMP-067`; kiểm tra quyền sử dụng thực chất và cultural review có bằng chứng vẫn là bước duyệt riêng, không thể suy ra chỉ từ tên giấy phép trong JSON.

## Invariants

```text
Addressable Key = asset.<catalog_id>.<facet> | asset.<kind>.<name>.<facet> (../04_architecture/client_assets.md)
base install (bootstrap.local + shared.local) <= 82 MB nén; resident steady <= 450 MB, transfer peak <= 570 MB
BGM streaming trực tiếp, không nạp toàn bộ vào RAM
100% asset references trong 24 catalogs phải phân giải được sang Addressable Key
gameplay sprite = texture 2x, import 100 PPU (reference 50 px/m; UI 200; PARALLAX_FAR 1x = 50); Bottom Center pivot; prefab Transform scale = (1,1,1)
mesh type Tight khi cạnh dài >= 256 texture px và có lề trong suốt, còn lại Full Rect
character reference silhouette <= 64x96 ref px (128x192 texture px); body height = 88..96 ref px
final texture drawn at exact 2x size; no import/runtime resize
every alpha texture passes the Cutout Quality Gate, the Volume & Depth Gate and the in-game Visual Review
art direction = painted-volume 2D chibi, one top-front key light, 5 environment depth layers (ADR-0056)
mọi placeholder phải đúng cell/silhouette và hiển thị collider canonical riêng
map extent là tile/scene coverage, không phải một bitmap hay một màn hình duy nhất
release asset source = AI_CREATED | FREE_LICENSED; giá sử dụng = 0; quyền thương mại/phái sinh/phân phối được xác minh
mọi file media phát hành có provenance APPROVED và hash đúng; CC-BY-4.0 có credit, OFL-1.1 có notice đóng gói
M10 không có placeholder, nguồn không rõ quyền, hay key catalog thiếu presentation
```
