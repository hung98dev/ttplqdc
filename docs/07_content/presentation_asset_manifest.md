# Presentation Asset Manifest & Addressables Delivery Schema
status: LOCKED

## Scope

Đặc tả cấu trúc nhóm tài nguyên Addressables (Unity Addressables 2.11.2), quy chuẩn định danh tài nguyên (Asset Keys), ngân sách dung lượng tải/bộ nhớ và quy trình bàn giao tài nguyên đồ họa/âm thanh cho game Thỉnh Thần.

Tài liệu này là đặc tả cho `IMP-063` (Addressables Asset Pipeline) và `IMP-070..076`, `IMP-104`, `IMP-105` (sản xuất, truy xuất nguồn gốc, kiểm tra asset phát hành). `IMP-063` chỉ dựng pipeline; nó không bàn giao mỹ thuật/âm thanh cuối cùng.

## Compiler Source Schema

The source below uses the registered Markdown grammar in `../06_data/content_authoring_contract.md`. Numbered `## N.` headings are the section namespace; `text` fences carrying labeled rows (`<label>  <rule>`, `asset_key  <desc>`) are registered field/value grammars; gate fences declare named threshold rules.

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `1. Cấu trúc Nhóm Addressables` / group table `Nhóm, Ngân sách nén, Ngân sách RAM, Nạp/giải phóng` + `text` fence | addressables group / `<group_key>` | group id (incl. `region.<zone_key>`×6, `dungeon.<key>`/finale/pvp, `audio.bgm.*`, `localization.*`); budgets in MB; load/release lifecycle tokens | 11 row families; totals fence: base install ≤92 MB, resident steady ≤450 MB, transfer peak ≤570 MB, +engine ≤1.3 GB. |
| `2. Quy chuẩn Định danh` / facet fence | asset-key facet rules | `asset.<catalog_id>.<facet>` / `asset.<kind>.<name>.<facet>`; per-content-type facet list | Facet set per content kind (prefab/portrait, vfx+icon, icon, scene/bgm, `asset.sfx.<cue>.clip`). |
| `3. Sprite Import` / import fence + `size_profile` table | import settings + size profiles / `size_profile` | import fence assignments (PPU, Filter, Mesh Type rule, Pivot, Mip Maps, Transform); table rows: silhouette/cell/texture/collider `WxH` pairs | 8 size profiles; `TEXTURE_SCALE = 2` (PARALLAX_FAR 1x, 50 PPU); NPC roster = all 42 `npc_id` from npc_shop_catalog with `asset.<npc_id>.prefab`; `cell_ref` declared for PROP/VFX (multiple of 16, ≤512x512). |
| `3.1 Authoring Resolution` + `3.1a Phạm vi gate` / bullets + `asset_class` table | authoring rules + gate-scope dispatch / `asset_class` | asset_class:enum(ACTOR,COSMETIC_APPEARANCE,PROP,ITEM_ICON,EQUIPMENT_ICON,UI_ART,FONT_ATLAS,TILE,PARALLAX_NEAR,PARALLAX_FAR,VFX_SOFT); table columns = named gate sub-sets | 2x authoring rule, no Unity resize, min 2px outlines, compression ASTC 4x4/6x6 / BC7, mip carve-out (ART-006); per-class gate subset dispatch. |
| `3.2 Cutout Quality Gate` / labeled `text` fence + translucent-mask paragraph | cutout gate rules | labeled rows: Định dạng, Dải bán trong suốt, Viền màu, Pixel trong suốt, Đốm rác, Răng cưa, Lỗ trong thân, Đệm cell, Kích thước | 9 named checks with numeric thresholds; translucent mask = `<texture>.translucent.png`, ACTOR-only kinds, ≤60% silhouette. |
| `3.3 Visual Review` + `3.3a Observed Graphics` + activation table | visual review contract | render set (1280x720, 1920x1080, 2400x1080, day/night, zoom 100/200) + rubric + LOW-profile 960x540 clip rule; activation table `Pass, Owner, Graphics, Required output` | Data-driven capture by `VisualReviewBatch.Run`; 5-row activation table; rubric ≥80% & no 0-score criterion (ART-011). |
| `3.5 Art Direction` / `text` fence + `Lớp` table | art-direction rules | labeled rows (lighting/value/volume/rim/outline/view/material/contact-shadow/colour); layer table `Lớp, Nội dung, Parallax, Quy tắc` (L0..L4) | Painted-volume 2D chibi rules; 5 depth layers with parallax ranges. |
| `3.6 Volume & Depth Gate` / labeled `text` fence + determinism paragraphs + fixture list | volume gate rules | labeled rows (Định nghĩa, Dải giá trị, Tầng giá trị, Sáng từ trên, Tách viền, Không mảng phẳng, Môi trường, Actor trên nền) | Named thresholds (L* p95−p5 ≥40, ≥3 tiers ≥5%, rim ≥60% \|ΔL*\|≥12, flat-region ≤20%); deterministic completion rules; 5 mandatory validator fixtures. |
| `3.7 Animation Contract` / table `size_profile, Kỹ thuật, Clip bắt buộc, Frame/fps` + frame fence | animation contract / `size_profile` | technique enum (skeletal/frame-by-frame/flipbook); clip lists; frame counts + fps | 5 profile rows; PSB layer list fixed; ΔE00 ≤3, bbox ≤8px (≤32 attack/hit/defeat); pivot stable. |
| `3.8 Style Pack` + `3.9 Tile/9-slice/VFX` + `3.10 Hitbox` + `3.11 Atlas` | style/tile/vfx/atlas rules | pack layout path + palette gate fence (ΔE00 ≤8, ≥85% S); tile/9-slice/VFX fence; hitbox rule; atlas padding rule | Style packs per region/actor group under `client/Assets/Art/StyleRef/<fragment>/<pack_id>/`; flipbook ≤16 frames ≤1024x1024, blend ∈ {ADDITIVE,ALPHA}; collider-vs-silhouette ±4 ref px & 0.5..0.9 width ratio; atlas padding ≥4. |
| `4. Vòng đời` | lifecycle rules | M0..M4 placeholder rules; M10 release rules | Placeholders need correct cell/silhouette + canonical key; 100% final assets at M10. |
| `5. Nguồn asset` / `source_kind` table + license rules + folklore-card sentence | source licensing rules | source_kind:enum(AI_CREATED,FREE_LICENSED); license:enum(CC0-1.0,CC-BY-4.0,OFL-1.1,AI_TOOL_TERMS); banned-motif seed list; `folklore_card` shape `{source_tales[], regional_variants, motifs_checked[]}` | Licensing verified at acquisition time; AI tool prerequisites (provider/version/terms/seed evidence); motifs banned list initial set. |
| `6. Sổ nguồn gốc` / field `text` fence | provenance record schema / `file_path` | 17 registered record fields (asset_key, file_path, content_id, source_kind, creator, source_uri, license_id, license_uri, acquired_at_utc, source_sha256, final_sha256, changes, attribution, style_pack_id, generation_record, folklore_card, inputs, review_state) | One record per shipped media file; `asset_source_register.json` merged by IMP-076; audio minimum: BGM per 33 scenes + 14 named SFX cues. |
| `7. Kiểm tra Tự động` / numbered gate list | asset verification gate | enumerated reject conditions | `verify_assets.ps1`/`AddressablesValidationTests` gate; run by IMP-076 before IMP-067. |

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
| `localization.locales` | ≤ 1 MB (trong player) | ≤ 2 MB | nạp ở `BOOT`; giữ suốt phiên |
| `localization.shared` | ≤ 1 MB (trong player) | ≤ 4 MB | nạp ở `BOOT`; giữ suốt phiên |
| `localization.strings.<locale_key>` (2: `vi_vn`, `en_us`) | ≤ 4 MB mỗi nhóm (trong player) | ≤ 8 MB | nạp ở `BOOT`; giữ suốt phiên |

```text
RAM runtime            = bộ nhớ texture tính từ định dạng x kích thước x số mip đã import + mesh + audio đã giải nén,
                         tính tất định từ import settings (validator IMP-063), không đo từ process
resident steady        tổng RAM các nhóm đang nạp <= 450 MB   (tối đa: bootstrap + shared + icons + beast + cosmetic +
                         localization.* + 1 region hoặc 1 dungeon/pvp)
resident transfer peak <= 570 MB (nhóm đích nạp trước khi nhóm nguồn giải phóng)
ràng buộc              resident transfer peak + engine/managed/native <= 1.3 GB resident của ANDROID_MIN
                         (../04_architecture/client_performance.md § Memory and GC)
base install           bootstrap.local + shared.local + localization.* <= 92 MB nén
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
| `NPC_HUMANOID` | `64x96` | `96x128` | `192x256` | không có (interaction anchor/range là authority, không collider từ art) |

Mọi Linh Thú dùng `SPIRIT_BEAST`. Asset không có `size_profile` khai báo `cell_ref` (reference px, bội số của 16, tối đa `512x512`) trong metadata import: `PROP` và `VFX_SOFT`/VFX gameplay theo khai báo đó; texture = đúng 2 x `cell_ref`. `PROP` áp Đệm cell và Kích thước cell của §3.2 theo `cell_ref`; VFX chỉ áp "texture = 2 x cell_ref". Icon item/equipment/skill/cosmetic = `64x64` ref (§3.1).

**NPC final roster owner = IMP-104:** toàn bộ 18 regional service + 24 ambient `npc_id` của `npc_shop_catalog.md` (42 ID, không tự thêm roster) dùng `ACTOR` / `NPC_HUMANOID`, exact 2x/100 PPU/Bottom Center và clips NPC §3.7. Mỗi ID có `asset.<npc_id>.prefab`; portrait bắt buộc chỉ khi owning dialogue/UI dùng portrait. Có thể chia sẻ visual bằng `PresentationAlias` đúng một hop, nhưng coverage phải duyệt đủ 42 ID, giải alias tới prefab final và kiểm profile/clips, vùng xuất hiện, `folklore_card`/provenance của visual cùng review các ID dùng nó. Media nằm `client/Assets/Art/Actors/Npcs/`; fragment/style/terms thuộc `actors_creatures` của IMP-104. Không coi NPC là prop hay player placeholder, không thêm physics collider.

Cell được phép có transparent padding; silhouette không được tự co giãn để lấp cell. Với nhân vật, body idle/run/jump cao `88..96px`; tóc/trang phục/vũ khí có thể vượt tối đa `8px` mỗi phía nhưng phải nằm trong cell. VFX/weapon trail vượt cell là asset con riêng.

### 3.1 Kích thước vẽ (Authoring Resolution)
- Mỗi texture cuối cùng phải được vẽ/hoàn thiện **đúng kích thước 2x mục tiêu**: cell theo bảng trên; icon UI `64x64` ref → `128x128`; UI/HUD theo mặt phẳng `1280x720` ref → `2560x1440`, sprite UI import `200 PPU` (Canvas Reference PPU `100`) để kích thước hiển thị bằng reference px; tile `50x50` ref → `100x100`.
- AI hoặc họa sĩ có thể tạo ở kích thước lớn hơn, nhưng bước hoàn thiện cuối (thu nhỏ bằng bộ lọc area/Lanczos theo tỉ lệ nguyên hoặc hữu tỉ, rồi làm sạch nét/viền và sharpen) phải làm ở đúng kích thước 2x. Cấm để Unity resize (`Max Size` phải ≥ kích thước thật; không `Non-Power-of-2` scaling).
- Nét viền/chi tiết quan trọng dày tối thiểu `2` texture px (= 1 ref px) để còn đọc được ở 720p.
- Parallax xa (lớp `L3`/`L4`, `asset_class = PARALLAX_FAR`) được phép `TEXTURE_SCALE = 1` để tiết kiệm bộ nhớ, khi đó import `50 PPU` để kích thước thế giới không đổi; mọi lớp gameplay, nhân vật, quái, boss, Linh Thú, vật phẩm, UI, icon, VFX gameplay và telegraph là 2x.
- Nén: nhân vật/quái/boss/Linh Thú/UI/icon/font = ASTC 4x4 (mobile), BC7 (desktop); nền/parallax = ASTC 6x6 / BC7. Sprite gameplay tắt mipmap ở launch. Quy tắc mip (`ART-006`, ADR-0076): nếu Visual Review 960x540 (§3.3) phát hiện nhấp nháy khi di chuyển trên actor sprite, bật đúng 1 mức mip cho `ACTOR` và tính lại RAM §1 (×1.25 cho texture đó) trong cùng spec-change; không bật mip cho loại khác.
- Frame animation: chiều cao silhouette giữa các frame idle lệch ≤ `4` texture px; pivot không trôi. Hợp đồng clip/frame/fps: §3.7.
- Upscale (`ART-012`): ảnh AI được upscale > 2x so với độ phân giải sinh gốc không được làm nguồn cuối nếu không qua bước thu nhỏ về đúng 2x ở trên; mọi bước upscale ghi trong `changes` của sổ nguồn (§6).

### 3.1a Phạm vi gate theo loại asset
| asset_class | Cutout Gate §3.2 | Volume Gate §3.6 | Ghi chú |
|---|---|---|---|
| `ACTOR` (nhân vật, quái, boss, Linh Thú, NPC), `COSMETIC_APPEARANCE` | toàn bộ | toàn bộ | |
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
Định dạng          PNG RGBA 8-bit/kênh, sRGB, có kênh alpha thật (mọi asset_class trong §3.1a);
                   thêm cho ACTOR, COSMETIC_APPEARANCE, PROP, ITEM_ICON, EQUIPMENT_ICON, PARALLAX_NEAR:
                   4 góc 4x4 px của mỗi cell phải alpha = 0 (bắt lỗi nền ca-rô giả hoặc nền đặc bị nướng vào ảnh);
                   TILE, UI_ART, PARALLAX_FAR, VFX_SOFT không áp quy tắc 4 góc (cạnh đặc hợp lệ) (ART-001)
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
Ảnh review chỉ render trên `Unity (Windows)` bằng graphics device đã **quan sát và chứng minh** tại §3.3a. `-force-d3d11` chọn API, không chứng minh WARP; metadata `renderer=warp` chỉ được ghi khi probe xác nhận adapter software WARP. Ảnh chỉ dùng để soi hình ảnh, không dùng cho số liệu hiệu năng GPU.
Scenes/Review của IMP-070 là renderer **data-driven**, không scene append-per-asset: đọc catalog presentation requirements, Addressables keys/one-hop aliases và tất cả provenance fragments ở head (sổ hợp nhất khi release), chọn asset theo `content_id` và vùng/instance từ catalog rồi nạp actual map layers, clip/profile và Style Pack của asset. Asset chung không có vùng được review trong tất cả vùng runtime tham chiếu; UI dùng UI review surface. Thiếu mapping/context/key/clip/pack là fail, không bỏ qua. Producer chỉ thêm media/keys/provenance vào owned paths và exact registry grants `repository_layout.md` § Addressables Append Registry Grants; không sửa `client/Assets/Scenes/Review/`. Mỗi entity/UI được chụp ở `1280x720`, `1920x1080`, profile `2400x1080`, ngày/đêm, zoom 100%/200%. Job upload `visual-review`; ảnh là review artifact được manifest evidence tham chiếu, không commit và không thay thế evidence. Agent `reviewer` khác người tạo ghi kết luận vào PR review comment và đặt `review_state`:
- không thấy viền lem, quầng màu, răng cưa hay đốm rác ở cả hai mức zoom;
- silhouette đọc rõ trên nền: `ΔL*` trung bình giữa dải biên actor và nền cục bộ ≥ 20 (cùng phép đo "Actor trên nền" §3.6), hoặc asset có outline;
- telegraph/VFX đọc được mà không phụ thuộc chỉ vào màu (`../00_context/constraints.md`);
- nét sắc ở 1920x1080 (không mờ do phóng to) và không nhiễu/nhấp nháy khi di chuyển ở 1280x720;
- trông có khối, không phẳng như tranh: hướng sáng thống nhất với cảnh, đọc được 3 mặt phẳng độ sâu (foreground / gameplay / background), actor nổi rõ hơn nền cả ngày lẫn đêm dưới ánh sáng runtime.

Bổ sung (ADR-0076):
- Profile LOW (`ART-006`): thêm render `960x540` (tương đương 1280x720 × render scale 0.75 của preset `LOW`) gồm một clip di chuyển ngang 2 s của mỗi actor; reviewer ghi `shimmer = none | visible`. `visible` kích hoạt quy tắc mip §3.1.
- Rubric (`ART-011`): mỗi tiêu chí ở trên chấm 0 / 1 / 2 (0 = lỗi, 2 = đạt rõ); asset đạt khi không tiêu chí nào 0 và tổng ≥ 80% điểm tối đa. Artifact `visual-review` có thêm contact sheet đặt asset cạnh các ảnh neo của Style Pack (§3.8); điểm và contact sheet ghi vào PR review comment.
Task sản xuất (IMP-071..075, IMP-104, IMP-105) tham chiếu artifact `visual-review` của lần chạy CI trong manifest evidence; IMP-076 kiểm lại.

### 3.3a Observed Graphics Capability and Activation

IMP-000 owns the graphics launch/probe and owner-derived plan; IMP-070 owns data-driven review capture; IMP-095 owns hotspot/overdraw accounting. Every graphical invocation uses the pinned native Windows Editor, URP 2D pipeline and `-force-d3d11` **without** `-nographics`. Before admitting graphical results, record actual Editor/package versions, `SystemInfo.graphicsDeviceType`, device/vendor/name/version/IDs, and the observed DXGI adapter software flag plus adapter description/LUID. D3D11 API + a generic device name alone is not WARP proof: match the initialized device to the software Microsoft Basic Render Driver/WARP adapter; ambiguity, null graphics device or a hardware adapter fails this required hosted software path. Device observation may use Windows platform diagnostics in the probe; no new runtime package, GPU host, Linux Unity or external GUI dependency is introduced.

The same invocation must render a known URP Sprite-Lit fixture under day/night lights and prove finite, nonblank output and the fixture's expected lit-region luminance change (day mean L* exceeds night by >= 5). Verify material/shader SRP-Batcher compatibility separately; do not infer every SpriteRenderer uses the SRP batching path. Check `RFloat` render-target support, then render a 16x16 additive overlap fixture: clear=0, one full-target layer=1, a second layer covering the left 8 columns gives left=2/right=1, readback error <= 0.001 per pixel. Actual readback completion/format and all finite values are required even if API support flags claim success. Missing/invalid probe output, URP lit failure, unsupported target or failed/all-zero/NaN readback fails closed before review/performance gates; never emit fabricated adapter metadata, substitute another format or skip rendering. Probe report, results XML and captured artifact refer to the same source identity and invocation.

**Executable graphics routes (not a third functional PlayMode owner):** the IMP-000-owned Editor helper `ThinhThan.Core.Assets.Editor.GraphicsCapabilityProbe.VerifyCurrentInvocation` (`client/Assets/Scripts/Core/Assets/Editor/GraphicsCapabilityProbe.cs`) performs the observations/fixtures above inside every process admitting graphical output. IMP-070's static `ThinhThan.Core.Assets.Editor.AssetProduction.VisualReviewBatch.Run` is launched with `-batchmode -projectPath client -force-d3d11 -executeMethod ThinhThan.Core.Assets.Editor.AssetProduction.VisualReviewBatch.Run -logFile <output>/visual-review.log`, with neither `-nographics` nor `-runTests`. It calls the probe, renders the data-driven capture matrix and required rendered fixtures, waits for capture/readback completion, writes `artifacts/visual-review/capture-report.json` and PNGs into `artifacts/visual-review/` and emits `artifacts/visual-review/visual-review-fixtures.xml` (category `GraphicsFixtures`).

| Pass | Owner activation (DONE on main or head) | Graphics | Required output |
|---|---|---|---|
| Functional EditMode | IMP-000 | no (`-nographics`) | nonempty EditMode XML |
| Functional PlayMode, excluding graphics categories | IMP-065 | no (`-nographics`) | nonempty functional PlayMode XML |
| Visual Review batch method + rendered EditMode fixtures | IMP-070, independent of IMP-065 | yes + capability probe in each invocation | `capture-report.json`, complete `visual-review` artifact + nonempty GraphicsFixtures XML |
| Performance hotspot/counters/overdraw | IMP-095 | yes + capability probe | Performance XML + probe/metric report |
| Graphical load/transfer harness using existing bundles | IMP-067 | yes + capability probe | load XML + bundle/load report |

Use the same activation predicate for planning and result verification. Status-only claims and full-approved no-client-change scopes do not launch these passes; apply only the canonical skip reasons of `audit_gates.md`, not a new skip exemption. Otherwise every active owner requires its own pass even if functional PlayMode is not active. Planning two functional modes never suppresses Visual Review/Performance/load passes; `-nographics` XML cannot satisfy a graphical category. IMP-070 active/IMP-065 not DONE still requires review rendering; IMP-095 active requires Performance; IMP-067 active requires load. Missing XML, empty expected category, absent images/metrics or invalid capability never passes. Actual hosted WARP/URP/RFloat and pinned model/tool proofs are environment prerequisites until observed, not assertions established by these docs.

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
                 bin Lab      = (floor(L*/3), floor(a*/6), floor(b*/6))
                 vùng liền màu = thành phần liên thông 8-connectivity của các pixel S cùng bin Lab (ART-002; một gradient
                                mịn trải qua nhiều bin nên không gộp thành một vùng)
                 cụm sắc độ   = k-means 2 chiều trên (a*, b*) của S, k = 4, tâm khởi tạo = (a*, b*) của pixel S tại hạng
                                L* p12.5/p37.5/p62.5/p87.5, lặp Lloyd tới khi không đổi hoặc 100 vòng (tất định, không seed)
Dải giá trị      L*(p95) - L*(p5) >= 40
Tầng giá trị     k-means 1 chiều k=5 trên L*, tâm khởi tạo = L* tại p10, p30, p50, p70, p90, lặp Lloyd tới khi phân cụm
                 không đổi hoặc 100 vòng (tất định, không seed): >= 3 cụm, mỗi cụm >= 5% diện tích S
Sáng từ trên     với mỗi cụm sắc độ C chiếm >= 5% S: d(C) = mean L* của 1/3 trên bbox(C) - mean L* của 1/3 dưới bbox(C);
                 trung vị có trọng số (trọng số = diện tích C) của d(C) >= 4 (ART-003; tóc/mũ tối không làm fail)
Tách viền        >= 60% pixel p ∈ B có |L*(p) - L*(K(p))| >= 12 (rim light hoặc outline)
Không mảng phẳng không vùng liền màu nào > 20% diện tích S
Môi trường       đo trên render review 1280x720 ban ngày của từng lớp riêng (các lớp khác ẩn), pixel a >= 128 của lớp đó:
                 contrast L*(p95-p5): L1 >= L2 >= L3 >= L4 và L4 <= 0.5 x L1; bão hòa (C*ab) trung bình giảm dần L1 -> L4
Actor trên nền   trên render review (§3.3): mean L* của B - mean L* của nền trong vành 4..12 px ngoài S, |chênh| >= 20
```
Vi phạm là fail; ngưỡng chỉ nới bằng ADR (gate ratchet).

**Deterministic numeric completion:** coordinates use image origin top-left, row-major `(y,x)` tie order; pixels outside the cell are outside S. Empty S/B or any required background/layer region fails its applicable gate. Convert sRGB using IEC sRGB transfer and D65 Lab; use binary64, fixed row-major reduction order, reject every nonfinite input/intermediate/result (NaN is never a comparison pass). Quantile q uses sorted `(L*,y,x)` and nearest rank `max(0,ceil(q*N)-1)` with no interpolation. Use that identical rank rule for all stated percentiles.

Both Lloyd algorithms assign by squared distance, equal distances choose the lower original cluster index. Keep duplicate initial centers; empty clusters keep their previous finite center, have area 0 and are not counted toward the >=3 value tiers or >=5% hue eligibility. Recompute nonempty centers in row-major order; stop on identical assignments or use the completed 100th iteration (no random reseed, no threshold epsilon stop). A completed iteration is assignment followed by mean recomputation: the final memberships, areas and means all come from that same iteration; do not run an unrecorded extra assignment after the iteration limit. All value/hue clusters being empty or no eligible hue cluster fails. Weighted median sorts `(d,cluster_index)`, chooses the first d whose doubled cumulative integer pixel area is >= the total eligible area (lower median on an exact half). For top/bottom thirds, bbox height h uses `max(1,ceil(h/3))` rows at each end; overlap for h <= 2 is intentional. If either part has no pixels of C, that eligible cluster's top-light measurement fails, not omit it.

For an empty 5..8 px core ring K (legitimate thin silhouette), use the deepest pixels of S by Chebyshev distance to outside S as K, then nearest Euclidean/row-major tie as above; report `core_mode=DEEPEST`. K must be nonempty and disjoint from B; a silhouette only one edge-band thick with no deeper pixel fails the applicable rim gate rather than returning NaN/auto-pass. Thresholds remain unchanged. Fixtures include duplicate-centroid grayscale, equal-rank pixels, empty clusters/thirds, a thin PROP with valid deepest core, an all-edge silhouette failure and missing background.

Fixtures bắt buộc của validator (IMP-070, `client/Assets/Tests/EditMode/` của packet đó): `gradient_smooth_pass.png` (khối trụ tô gradient mịn, phải PASS "Không mảng phẳng"), `flat_fill_fail.png` (mảng một màu > 20% S, phải FAIL), `dark_hair_toplit_pass.png` (chibi tóc đen, sáng từ trên đúng, phải PASS "Sáng từ trên"), `bottom_lit_fail.png` (sáng từ dưới, phải FAIL), `tile_solid_edge_pass.png` (`TILE` cạnh đặc, phải PASS "Định dạng").

### 3.7 Animation Contract (ADR-0076, `ART-004`)
Kỹ thuật theo `size_profile`:

| `size_profile` | Kỹ thuật | Clip bắt buộc | Frame tối thiểu / fps |
|---|---|---|---|
| `CHARACTER` | skeletal (PSB layer → PSD Importer 15.0.0 + 2D Animation 16.0.0); một skeleton dùng chung cho 5 class; cosmetic/trang bị đổi bằng Sprite Library/Resolver | `idle, run, jump_up, fall, land, attack_basic, cast, hit, guard, defeat` | skeletal: key ≥ 4 mỗi clip, sample 30 fps |
| `MONSTER_MEDIUM`, `MONSTER_ELITE`, `BOSS_LARGE`, `WORLD_BOSS` | skeletal (skeleton riêng mỗi rig) | `idle, move, attack_<n>` (mỗi skill của catalog), `hit, defeat`; boss thêm `phase_transition` mỗi phase | key ≥ 4, 30 fps |
| `MONSTER_SMALL`, `SPIRIT_BEAST` | frame-by-frame | `idle, move, attack_basic, hit, defeat` (Linh Thú: `idle, move, cast`) | ≥ 4 frame, 12 fps |
| `NPC_HUMANOID` | frame-by-frame (noncombat NPCs, no player rig requirement) | `idle, interact`; `move` only if catalog explicitly declares PATROL | ≥ 4 frame mỗi clip, 12 fps |
| VFX gameplay | flipbook (§3.9) | theo skill | ≤ 16 frame, 12 hoặc 24 fps |

Layer PSB tối thiểu cho skeletal: `head, hair, torso, arm_front, arm_back, leg_front, leg_back, weapon` (+ `accessory_*` tùy chọn); tên layer cố định để Sprite Library ánh xạ cosmetic.

```text
Nhất quán frame   fit hue centers only on idle_0 (§3.6); freeze their original indices and discard empty idle_0
                  clusters from the candidate set. Assign every frame pixel to the nearest remaining idle_0 center
                  with lower-original-index ties. Each candidate must have nonempty corresponding frame membership;
                  ΔE00 between its mean Lab and idle_0 mean <= 3. Empty idle_0 centers never steal pixels or get reseeded.
                  New frame clusters are not fitted; empty frame silhouette or any nonfinite mean/difference fails.
                  độ rộng bbox(S) lệch <= 8 texture px so với idle_0 trừ clip attack/hit/defeat (được lệch <= 32)
Pivot             pivot Bottom Center giữ nguyên mọi frame/clip; chân chạm y = 0 ở idle/run/land
```
Ngưỡng ΔE00 3 / 8 px là đề xuất: hiệu chỉnh trên lô asset đầu tiên bằng gate-ratchet ADR.

### 3.8 Style Pack (ADR-0076, `ART-005`)
Style được khóa bằng ảnh tham chiếu/adapter, không train LoRA ở launch (chỉ train nếu lô đầu fail gate bảng màu, qua ADR):
- Đường dẫn: `client/Assets/Art/StyleRef/<fragment>/<pack_id>/` (LFS, ngoài Addressables, không vào build; `<fragment>` = tên fragment §6 của packet sở hữu, ví dụ `actors_players`); mỗi vùng (`region.<zone_key>`) và mỗi nhóm actor (class, quái vùng, boss) có một pack.
- Nội dung pack: 6–10 ảnh neo đã APPROVED; `palette.json` (danh sách màu Lab theo vùng/nhóm); turnaround 4 góc (trước 3/4, sau 3/4, nghiêng, chính diện) cho mỗi class và mỗi boss; `style.md` ghi prompt khung ánh sáng/khối của §3.5.
- Turnaround được reviewer duyệt trước khi sản xuất sprite của entity đó.

```text
Gate bảng màu    >= 85% pixel S (a >= 128, ngoài mask translucent) có ΔE00 <= 8 tới màu gần nhất trong palette.json của pack
                 khai báo (style_pack_id trong §6); ngưỡng đề xuất, hiệu chỉnh trên lô đầu bằng gate-ratchet ADR
```

### 3.9 Tile, 9-slice và VFX (ADR-0076, `ART-007`)
```text
TILE        |ΔE00 trung bình| giữa cột pixel trái và phải, và giữa hàng trên và dưới <= 2; ghép 3x3 không lộ lưới (reviewer)
UI_ART      9-slice khai báo border (Sprite Editor) bắt buộc; dải giữa theo trục kéo giãn có độ lệch chuẩn L* <= 2,
            nếu không thì Draw Mode = Tiled
VFX         flipbook <= 16 frame, sheet <= 1024x1024 texture px, 12 hoặc 24 fps; khai báo blend = ADDITIVE (ánh sáng/lửa/
            phép) | ALPHA (khói/bụi/nước) và max_instances; hotspot scene vẫn đạt PERF-016 (overdraw <= 2.5)
```

### 3.10 Hitbox khớp hình (`ART-008`)
Ở frame `idle_0`: tâm ngang collider canonical (`physics_geometry_contract.md`) nằm trong ±4 ref px so với tâm ngang silhouette (a ≥ 128); tỉ lệ độ rộng collider / độ rộng silhouette trong `0.5..0.9`. Ngưỡng đề xuất, hiệu chỉnh trên lô đầu bằng gate-ratchet ADR. Collider vẫn không suy ra từ sprite.

### 3.11 Atlas và sau nén (`ART-009`)
SpriteAtlas `Padding ≥ 4` texture px (khớp mức dilate §3.2). Validator giải nén texture đã import (ASTC 4x4 / 6x6, BC7) và chạy lại dòng "Viền màu" của §3.2 trên kết quả; vi phạm là fail.

### 3.4 File gốc
File làm việc lớn (PSD/PSB/ảnh AI gốc) lưu qua LFS ngoài thư mục Addressables và không vào build; sổ nguồn gốc ghi hash cả file gốc và file cuối.

Map extent trong catalog là độ phủ tham chiếu, không phải yêu cầu một texture bitmap duy nhất. Tilemap, props tái sử dụng và parallax layers ghép thành scene; không import ảnh nền `2560..6400px` như một collider hoặc một sprite gameplay duy nhất.

## 4. Vòng đời Tài nguyên: Placeholder vs. Release Candidate

Để đảm bảo việc triển khai kỹ thuật không bị đình trệ vì tiến độ vẽ mỹ thuật:

1. **Giai đoạn Milestone Nội bộ (M0 .. M4):**
   - Cho phép sử dụng **Placeholder Assets**.
   - Sprite placeholder dùng đúng cell/silhouette của `size_profile`, vẽ viền collider riêng theo `physics_geometry_contract.md`; không kéo hình khối AABB thành toàn bộ silhouette.
   - Bắt buộc phải gắn đúng `Addressable Key` chuẩn ngay từ đầu.
   - Addressables pipeline validation (IMP-063) ở giai đoạn Foundation (M0/Wave 1) kiểm tra tính hợp lệ của group definitions, bundle schemas, ngữ pháp key và các fixture/placeholder scene mà không đòi hỏi toàn bộ scene/art vật lý phải hiện diện; 100% tài nguyên phân giải sang asset hoàn thiện trên đĩa là điều kiện nghiệm thu phát hành do IMP-076 kiểm toán trước IMP-067.
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

Tiêu chí công cụ AI là **prerequisite thực tế**, không suy từ tên “Direct AI Generation”: trước claim final-art, Owner Setup và `technology_versions.md` phải ghi provider/access API, exact tool/model/version, terms URI và snapshot/hash tại thời điểm tạo, quyền thương mại và giới hạn truy cập/giá 0. Demonstrate reference-image/style input, reproducible explicit seed returned/accepted by that API and exact-size PNG/alpha export (native hoặc key-background cleanup qua gate). Không ghi seed/model/terms giả nếu tool không cung cấp: chọn in-session route đáp ứng contract khác hoặc dừng final-art đó; placeholder chỉ cho engineering scope §4, không thay final output. Owner-approved in-session production remains, không thêm external desktop GUI dependency hay loại miễn giấy phép.

Capability proof phải theo output: một actor sample đi từ generation/source tới layer PSB đúng tên, rig/clip import bằng pinned Unity packages và final packaged sprite/prefab qua gate/review; image generation không tự chứng minh layered rig/animation/audio. IMP-075 có thể dùng generated audio chỉ khi selected API/tool thật có capability và terms phù hợp, hoặc FREE_LICENSED CC0/CC-BY audio có URL/hash/license thật theo §5. Chứng minh cue playback và BGM loop/stream import; không bắt buộc chọn audio generator nếu permitted free-licensed route đã đáp ứng. Ghi actual capability artifact/source hashes trong setup/evidence, không lấy generic category approval làm proof mọi packet ready. Seed/terms/version unavailable là missing environment prerequisite, không tự miễn provenance.

Rủi ro bản quyền (ADR-0076): art thuần AI có thể không được bảo hộ bản quyền ở một số thị trường (ví dụ Mỹ); dự án chấp nhận rủi ro này. Nền tảng phát hành yêu cầu khai báo nội dung AI (Steam) được xử lý trong checklist phát hành của `IMP-067`.

Thẻ dân gian (`ART-010`): mọi quái, boss, Linh Thú, NPC, map và cosmetic có nguồn gốc văn hóa mang `folklore_card` trong bản ghi nguồn (§6): `{source_tales[], regional_variants, motifs_checked[]}`. Motif cấm (danh sách khởi đầu, spec-owner mở rộng qua spec-change): cổng torii, trang phục/mũ quan triều Thanh kiểu cương thi, kimono, hanbok, cờ/biểu tượng tôn giáo hoặc chính trị hiện đại, chữ Hán/Nôm vô nghĩa làm hoa văn. Reviewer đối chiếu thẻ trước khi `APPROVED`.

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
style_pack_id      image-level pack identity `<fragment>/<pack_id>` resolving §3.8, required for every image row
                   (including FREE_LICENSED with generation_record=null); null only for non-image media
generation_record  {tool, version, model_id, model_sha256, terms_uri, terms_snapshot_sha256, prompt, seed, parameters,
                    workflow_sha256, reference_uris, reference_sha256[], c2pa_present} nếu dùng AI tạo/chỉnh;
                    null nếu không. terms_snapshot_sha256 = hash bản sao điều khoản tại thời điểm tạo, lưu LFS tại
                    client/Assets/Art/Provenance/terms/<fragment>/<sha256>.txt; model_sha256/workflow_sha256 null nếu công cụ không lộ ra
                   style_pack_id is not nested in generation_record; legacy nested field is removed, not duplicated
folklore_card      {source_tales[], regional_variants, motifs_checked[]} cho entity văn hóa (§5); null cho asset chung
inputs             [] hoặc danh sách {creator, source_uri, license_id, license_uri, acquired_at_utc, sha256} cho nguồn ngoài dùng tạo/ghép
review_state       PENDING | APPROVED | REJECTED
```

Không ghi URL tìm kiếm thay cho URL nguồn gốc. Với `AI_CREATED` hoặc asset tải về rồi chỉnh bằng AI, `generation_record` phải chỉ ra điều khoản cho phép phân phối thương mại; mọi ảnh/âm thanh đầu vào bên thứ ba phải hiện trong `inputs` với giấy phép hợp lệ. `APPROVED` chỉ khi metadata, file/hash, giấy phép/điều khoản, thẩm mỹ và quyền liên quan đã được kiểm tra. File bị `PENDING` hoặc `REJECTED` không được vào release. Danh sách attribution của mọi bản ghi `CC-BY-4.0` và notice của font `OFL-1.1` phải được sinh từ sổ này và đóng gói để người chơi truy cập được trong credits.

Coverage được tính từ **toàn bộ catalog và feature phát hành**, không chỉ keys trong Addressables: nhân vật/animation, quái/boss/Linh Thú, đủ 42 NPC (§3), bản đồ/props/parallax, UI/font/icon, vật phẩm/equipment/cosmetics, skill VFX/telegraph, SFX/BGM và scene. Mỗi ID cần presentation có đúng một key hoặc mapping tường minh tới asset chia sẻ đúng một hop; asset chia sẻ vẫn có sổ nguồn. Không tạo bản vẽ chỉ để đạt đủ số lượng nếu chia sẻ hợp lý và không mất nhận diện riêng.

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
   - Vi phạm §3.7 (clip thiếu, frame/fps, nhất quán frame, pivot), §3.8 (thiếu `style_pack_id` hoặc gate bảng màu), §3.9 (tile seam, 9-slice border, VFX), §3.10 (hitbox–hình), §3.11 (atlas padding, viền sau nén).
   - Scene map thiếu Addressable key, geometry export, hoặc extent không khớp bounds catalog.
   - Asset final không có bản ghi nguồn, hash sai, `review_state != APPROVED`, giấy phép/điều khoản không hợp lệ, attribution/notice bắt buộc vắng mặt, hoặc còn placeholder.

`IMP-076` chạy gate này cho toàn bộ release scope trước `IMP-067`; kiểm tra quyền sử dụng thực chất và cultural review có bằng chứng vẫn là bước duyệt riêng, không thể suy ra chỉ từ tên giấy phép trong JSON.

## Invariants

```text
Addressable Key = asset.<catalog_id>.<facet> | asset.<kind>.<name>.<facet> (../04_architecture/client_assets.md)
base install (bootstrap.local + shared.local + localization.*) <= 92 MB nén; resident steady <= 450 MB, transfer peak <= 570 MB
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
style khóa bằng Style Pack + ảnh tham chiếu; animation skeletal cho CHARACTER/MONSTER_MEDIUM+/boss, frame-by-frame cho actor nhỏ/Linh Thú/VFX
không dùng normal map/mask map ở launch (ADR-0056)
```

## Requirement IDs

| ID | Requirement (section) | Gate |
|---|---|---|
| `ART-001` | quy tắc 4 góc alpha = 0 chỉ áp đúng asset_class (§3.2) | every PR (validator) |
| `ART-002` | "Không mảng phẳng" đo bằng bin Lab; fixture gradient PASS (§3.6) | every PR |
| `ART-003` | "Sáng từ trên" theo cụm sắc độ; fixture tóc tối PASS (§3.6) | every PR |
| `ART-004` | hợp đồng animation: kỹ thuật, clip, frame/fps, nhất quán frame, pivot (§3.7) | every PR |
| `ART-005` | Style Pack tồn tại cho mỗi pack khai báo; gate bảng màu (§3.8) | every PR |
| `ART-006` | review 960x540 LOW có chuyển động; quy tắc mip khi nhấp nháy (§3.1, §3.3) | review |
| `ART-007` | tile seam, 9-slice border, VFX flipbook/blend/max_instances (§3.9) | every PR |
| `ART-008` | hitbox khớp silhouette ở idle_0 (§3.10) | every PR |
| `ART-009` | atlas padding ≥ 4 px và viền sau nén ASTC/BC7 (§3.11) | every PR |
| `ART-010` | folklore_card cho entity văn hóa; motif cấm (§5) | review |
| `ART-011` | rubric Visual Review 0/1/2 và contact sheet với ảnh neo (§3.3) | review |
| `ART-012` | quy tắc upscale và trường provenance mở rộng (§3.1, §6) | every PR (validator) |
