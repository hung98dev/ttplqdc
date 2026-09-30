# Atlas Catalog — Hon Giam & Quai Dam
status: LOCKED

## Scope
Concrete 104-page launch atlas roster for `../03_systems/atlas.md`. Each page is a folklore collection entry with tiered unlocks and non-power rewards.

This catalog is **not** a loot table; it defines collection triggers and rewards owned by `03_systems/atlas.md` and `economy_catalog.md` (for special amounts). Atlas unlocks do not grant combat stats.

## Atlas Budget
```text
quai_dam  = 58  (46 NORMAL + 12 ELITE monsters from monster_catalog.md)
hon_giam  = 25  (25 Souls from soul_catalog.md)
di_tich   = 8   (8 major bosses from boss_catalog.md)
co_vat    = 13  (hidden chests + fishing including rare carp + cooking)
TOTAL     = 104
```

## Atlas Page ID Contract
```text
atlas.page.quai_dam.<monster_key>
atlas.page.hon_giam.<soul_key>
atlas.page.di_tich.<boss_key>
atlas.page.co_vat.<key>
```
All lowercase, ASCII. `<monster_key>` is suffix of `monster.*` after first dot (e.g. `lang_da.dom_dom_ma`). `<soul_key>` is suffix of `soul.*` after `soul.<rank>.` (e.g. `coc_thanh_tinh`). `<boss_key>` is suffix of `boss.*` (e.g. `quy_nhap_trang`).

## Shared Tier Model
Each page has up to 3 tiers. Each tier resolves one non-power, idempotent `atlas_reward_bundle` per `atlas.tier.<character_id>.<atlas_page_id>.<tier>` (auto-settled at promotion, `../03_systems/atlas.md`); a bundle may include the shown `currency.special` amount and the shown presentation entitlement together. The bundle is one settlement, not two independently rerollable rewards.

| Tier | Name | Generic condition | Reward type |
|---|---|---|---|
| 1 | **Seen** | First qualifying event (1 kill / 1 acquire / 1 witness relic / 1 open/catch/cook) | `1 currency.special` |
| 2 | **Studied** | 10 kills OR Soul Lv3 OR 3 relic witnesses OR 10 opens/catches | cosmetic title fragment / 2 special |
| 3 | **Mastered** | 100 kills (30 for night-only pages) OR Soul Lv5 OR 10 witnesses OR 50 opens/catches | glowing title (cosmetic) / 2 special |

Total launch special from 104 pages tiers (if all completed):
```
Tier1: 104 * 1 = 104 special
Tier2: 104 * 2 = 208 special
Tier3: 104 * 2 = 208 special
Max theoretical = 520 per character (lifetime, non-repeatable)
```
T3 capped at 2 special (reduced from 3) to keep the total Atlas faucet (520) within the ~545 sink surface documented by economy_catalog.md. Total character faucet including base PvE = 540, which does not exceed the sink surface.
Practical per-character by Lv60 is far lower; validation must not assume 520 is expected.
LIFE_SKILL: each successful atlas tier-up is 1 action granting LIFE_SKILL per-unit for the character's current act (`progression_route.md`): I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494. Key `life_skill.atlas.<atlas_page_id>.<tier>.<character_id>`.
Rewards never grant equipment, skill/potential points, enhancement, or bound currency efficiency.

## Quai Dam — 58 Pages
Source: `monster_catalog.md`. Each monster maps to one atlas page.

Note: five monsters share a name with a major boss (`quy_nhap_trang`, `thuong_luong`, `ho_tinh`, `ngu_tinh`, `than_trung`). Their quai_dam pages document the field creature variant ("Theo lời kể dân gian..." framing). The boss aftermath is separately covered by the corresponding `atlas.page.di_tich.*` page. The two pages never overlap in unlock trigger, ID, or title reward.

### Act I — Làng Đa
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.lang_da.dom_dom_ma` | `monster.lang_da.dom_dom_ma` | 1 kill | 10 kills | 100 kills | Đom đóm ma lập lòe chỗ bến Đình Làng | `cosmetic.title.atlas.dom_dom_ma` |
| `atlas.page.quai_dam.lang_da.bu_nhin_rom` | `monster.lang_da.bu_nhin_rom` | 1 | 10 | 100 | Bù nhìn canh đồng lúa đêm trăng | `cosmetic.title.atlas.bu_nhin_rom` |
| `atlas.page.quai_dam.lang_da.coc_thanh_tinh` | `monster.lang_da.coc_thanh_tinh` | 1 | 10 | 100 | Cóc cụ bên miếu Đình | `cosmetic.title.atlas.coc_thanh_tinh` |
| `atlas.page.quai_dam.lang_da.hon_xo_non` | `monster.lang_da.hon_xo_non` | 1 | 10 | 100 | Theo lời kể dân gian, hồn xó nón giữ đường ranh làng; ai vô tình phá cột mốc sẽ bị ngăn lại | `cosmetic.title.atlas.hon_xo_non` |
| `atlas.page.quai_dam.lang_da.quy_nhap_trang` | `monster.lang_da.quy_nhap_trang` | 1 | 10 | 100 | Theo lời kể dân gian, quỷ nhập tràng là hồn lạc nhập vào xác đêm sương, chưa đủ mạnh để thành tướng đầu sỏ | `cosmetic.title.atlas.quy_nhap_trang_dong` |
| `atlas.page.quai_dam.lang_da.vong_hon` | `monster.lang_da.vong_hon` | 1 | 10 | 100 | Vong lang thang bờ tre | `cosmetic.title.atlas.vong_hon` |
| `atlas.page.quai_dam.lang_da.hon_ma_co_thu` | `monster.lang_da.hon_ma_co_thu` | 1 | 10 | 100 | Theo lời kể dân gian, hồn ma cổ thụ là linh hồn gắn vào cây già trăm năm; tán lá che khuất bao điều | `cosmetic.title.atlas.hon_ma_co_thu` |
| `atlas.page.quai_dam.lang_da.hon_do_trang` | `monster.lang_da.hon_do_trang` | 1 | 5 | 30 | Theo lời kể dân gian, hồn đo trăng lang thang dưới trăng khuya, dùng bóng làm tấm che che mặt | `cosmetic.title.atlas.hon_do_trang` |
| `atlas.page.quai_dam.lang_da.ma_xo` | `monster.lang_da.ma_xo` | 1 | 10 | 100 | Ma xó giữ nhà | `cosmetic.title.atlas.ma_xo` |
| `atlas.page.quai_dam.lang_da.vong_hon_gia` | `monster.lang_da.vong_hon_gia` | 1 | 10 | 100 | Theo lời kể dân gian, vong hồn già là những linh hồn lâu năm không siêu thoát, chập chờn theo ánh đèn dầu | `cosmetic.title.atlas.vong_hon_gia` |

### Act II — Rừng U Minh
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.rung_u_minh.ma_rung` | `monster.rung_u_minh.ma_rung` | 1 | 10 | 100 | Tiếng hú trong rừng U Minh | `cosmetic.title.atlas.ma_rung` |
| `atlas.page.quai_dam.rung_u_minh.dom_lua` | `monster.rung_u_minh.dom_lua` | 1 | 10 | 100 | Đóm lửa rừng thiêng | `cosmetic.title.atlas.dom_lua` |
| `atlas.page.quai_dam.rung_u_minh.bong_nguoi` | `monster.rung_u_minh.bong_nguoi` | 1 | 10 | 100 | Bóng người chập chờn | `cosmetic.title.atlas.bong_nguoi` |
| `atlas.page.quai_dam.rung_u_minh.ma_tranh` | `monster.rung_u_minh.ma_tranh` | 1 | 10 | 100 | Ma trành dắt lối | `cosmetic.title.atlas.ma_tranh` |
| `atlas.page.quai_dam.rung_u_minh.tinh_cay` | `monster.rung_u_minh.tinh_cay` | 1 | 10 | 100 | Cây cổ thụ thành tinh | `cosmetic.title.atlas.tinh_cay` |
| `atlas.page.quai_dam.rung_u_minh.dai_tinh_cay` | `monster.rung_u_minh.dai_tinh_cay` | 1 | 10 | 100 | Theo lời kể dân gian, đại tinh cây là cổ thụ trăm tuổi đã thành tinh; rễ nó kéo lại những ai dám chặt cành | `cosmetic.title.atlas.dai_tinh_cay` |
| `atlas.page.quai_dam.rung_u_minh.moc_tinh` | `monster.rung_u_minh.moc_tinh` | 1 | 10 | 100 | Mộc tinh canh rừng | `cosmetic.title.atlas.moc_tinh` |
| `atlas.page.quai_dam.rung_u_minh.vong_rung_sau` | `monster.rung_u_minh.vong_rung_sau` | 1 | 10 | 100 | Theo lời kể dân gian, vong rừng sâu dẫn lối người lạ đi vòng mãi không ra; hai con đường nó vẽ chỉ một là thật | `cosmetic.title.atlas.vong_rung_sau` |

### Act III — Bến Nước Đen
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.ben_nuoc_den.ma_da` | `monster.ben_nuoc_den.ma_da` | 1 | 10 | 100 | Ma da kéo giò bến nước đen | `cosmetic.title.atlas.ma_da` |
| `atlas.page.quai_dam.ben_nuoc_den.ca_tinh` | `monster.ben_nuoc_den.ca_tinh` | 1 | 10 | 100 | Cá tinh hóa người | `cosmetic.title.atlas.ca_tinh` |
| `atlas.page.quai_dam.ben_nuoc_den.quy_song_dem` | `monster.ben_nuoc_den.quy_song_dem` | 1 | 5 | 30 | Theo lời kể dân gian, quỷ sông đêm ẩn dưới bùn đen, dụ đèn lên rồi kéo người xuống | `cosmetic.title.atlas.quy_song_dem` |
| `atlas.page.quai_dam.ben_nuoc_den.thuong_luong` | `monster.ben_nuoc_den.thuong_luong` | 1 | 10 | 100 | Theo lời kể dân gian, thuồng luồng sông là loài thủy quái nhỏ lang thang theo dòng nước, khác với chúa tể đã thành tinh ở vùng sâu | `cosmetic.title.atlas.thuong_luong_song` |
| `atlas.page.quai_dam.ben_nuoc_den.bong_nuoc_ma` | `monster.ben_nuoc_den.bong_nuoc_ma` | 1 | 10 | 100 | Bóng nước ma mị | `cosmetic.title.atlas.bong_nuoc_ma` |
| `atlas.page.quai_dam.ben_nuoc_den.ma_da_gia` | `monster.ben_nuoc_den.ma_da_gia` | 1 | 10 | 100 | Ma da già giữ bến | `cosmetic.title.atlas.ma_da_gia` |
| `atlas.page.quai_dam.ben_nuoc_den.hon_chet_duoi` | `monster.ben_nuoc_den.hon_chet_duoi` | 1 | 10 | 100 | Hồn chết đuối oan khuất | `cosmetic.title.atlas.hon_chet_duoi` |
| `atlas.page.quai_dam.ben_nuoc_den.ca_tinh_gia` | `monster.ben_nuoc_den.ca_tinh_gia` | 1 | 10 | 100 | Theo lời kể dân gian, cá tinh già đã ẩn bao năm dưới bùn; đi chậm nhưng dòng nước sau lưng nó thì không thể chạy thoát | `cosmetic.title.atlas.ca_tinh_gia` |
| `atlas.page.quai_dam.ben_nuoc_den.thuy_quai` | `monster.ben_nuoc_den.thuy_quai` | 1 | 10 | 100 | Thủy quái bến đen | `cosmetic.title.atlas.thuy_quai` |
| `atlas.page.quai_dam.ben_nuoc_den.nguoi_song_co` | `monster.ben_nuoc_den.nguoi_song_co` | 1 | 10 | 100 | Theo lời kể dân gian, người sông cổ là hồn xưa nằm lại dưới đáy bùn từ thời loạn lạc; vẫn bắn hai tên một lúc như thuở chiến trận | `cosmetic.title.atlas.nguoi_song_co` |

### Act IV — Đèo Mây
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.deo_may.ma_tranh` | `monster.deo_may.ma_tranh` | 1 | 10 | 100 | Ma trành đèo mây | `cosmetic.title.atlas.ma_tranh_deo` |
| `atlas.page.quai_dam.deo_may.khi_nui` | `monster.deo_may.khi_nui` | 1 | 10 | 100 | Khỉ núi tinh ranh | `cosmetic.title.atlas.khi_nui` |
| `atlas.page.quai_dam.deo_may.ma_van_dem` | `monster.deo_may.ma_van_dem` | 1 | 5 | 30 | Theo lời kể dân gian, ma vân đêm là bóng mây tụ lại thành hình sau nửa đêm; tiếng nổ của nó là điềm báo giông tố | `cosmetic.title.atlas.ma_van_dem` |
| `atlas.page.quai_dam.deo_may.ho_tinh` | `monster.deo_may.ho_tinh` | 1 | 10 | 100 | Theo lời kể dân gian, hổ tinh đèo mây là hổ rừng đã luyện phép nhỏ; khác với chúa tinh chin đuôi đã thành tướng | `cosmetic.title.atlas.ho_tinh_dong` |
| `atlas.page.quai_dam.deo_may.ho_con_tinh` | `monster.deo_may.ho_con_tinh` | 1 | 10 | 100 | Hổ con tinh nghịch | `cosmetic.title.atlas.ho_con_tinh` |
| `atlas.page.quai_dam.deo_may.ma_tranh_gia` | `monster.deo_may.ma_tranh_gia` | 1 | 10 | 100 | Ma trành già đèo mây | `cosmetic.title.atlas.ma_tranh_gia` |
| `atlas.page.quai_dam.deo_may.vong_rung` | `monster.deo_may.vong_rung` | 1 | 10 | 100 | Vong rừng sương mù | `cosmetic.title.atlas.vong_rung` |
| `atlas.page.quai_dam.deo_may.ho_tinh_lon` | `monster.deo_may.ho_tinh_lon` | 1 | 10 | 100 | Theo lời kể dân gian, hổ tinh lớn đèo mây đã tích đủ khí để cào xé bất kỳ ai rời con đường mòn | `cosmetic.title.atlas.ho_tinh_lon` |
| `atlas.page.quai_dam.deo_may.ho_tinh_ve` | `monster.deo_may.ho_tinh_ve` | 1 | 10 | 100 | Hổ tinh vệ canh núi | `cosmetic.title.atlas.ho_tinh_ve` |
| `atlas.page.quai_dam.deo_may.vong_nui_gia` | `monster.deo_may.vong_nui_gia` | 1 | 10 | 100 | Theo lời kể dân gian, vong núi già là bóng ma núi lâu năm; vùng nguy hiểm của nó rộng dần như sương buổi sáng | `cosmetic.title.atlas.vong_nui_gia` |

### Act V — Thành Cổ
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.thanh_co.tuong_da` | `monster.thanh_co.tuong_da` | 1 | 10 | 100 | Theo lời kể dân gian, tượng đá thành cổ là hồn lính chết nơi đồn trú không ai thờ cúng; đá hóa hình người khi đêm xuống — chậm chạp nhưng không thể xuyên qua được | `cosmetic.title.atlas.tuong_da` |
| `atlas.page.quai_dam.thanh_co.hon_binh` | `monster.thanh_co.hon_binh` | 1 | 10 | 100 | Theo lời kể dân gian, hồn binh thành cổ vẫn tuần tra theo khu vực đặt lính ngày trước; giáo mác vô hình nhưng nhát chém từ nó để lại dấu vết thật | `cosmetic.title.atlas.hon_binh` |
| `atlas.page.quai_dam.thanh_co.ma_co` | `monster.thanh_co.ma_co` | 1 | 10 | 100 | Theo lời kể dân gian, ma cổ thành hoang là bóng của kẻ chết không toàn thây trong trận chiến; nó đánh dấu đất trước khi xuất hiện, như lời cảnh báo duy nhất trước khi sập bẫy | `cosmetic.title.atlas.ma_co` |
| `atlas.page.quai_dam.thanh_co.oan_hon_dem` | `monster.thanh_co.oan_hon_dem` | 1 | 5 | 30 | Theo lời kể dân gian, oan hồn đêm thành cổ là linh hồn lính chết oan không ai thờ cúng; chữ nguyền khắc lên người bị nó chạm | `cosmetic.title.atlas.oan_hon_dem` |
| `atlas.page.quai_dam.thanh_co.thach_ve` | `monster.thanh_co.thach_ve` | 1 | 10 | 100 | Theo lời kể dân gian, thạch vệ là phiến đá cổng thành đã hóa linh sau nghìn năm canh gác; nó dùng thân làm mộc đỡ rồi phản công — không bao giờ tấn công kẻ đứng im | `cosmetic.title.atlas.thach_ve` |
| `atlas.page.quai_dam.thanh_co.qua_tinh` | `monster.thanh_co.qua_tinh` | 1 | 10 | 100 | Theo lời kể dân gian, quạ tinh canh thành là linh hồn sứ giả cũ đã hóa vào đàn quạ; tên bắn của nó bay lệch theo vòm đá như thuở chuyển thư hiệu lệnh qua pháo đài | `cosmetic.title.atlas.qua_tinh` |
| `atlas.page.quai_dam.thanh_co.hon_tran_linh` | `monster.thanh_co.hon_tran_linh` | 1 | 10 | 100 | Theo lời kể dân gian, hồn trận linh là linh hồn tướng lĩnh cũ vẫn giữ thế trận, nhát chém từ trên xuống rõ nét như thuở sinh thời | `cosmetic.title.atlas.hon_tran_linh` |
| `atlas.page.quai_dam.thanh_co.hon_tuong` | `monster.thanh_co.hon_tuong` | 1 | 10 | 100 | Theo lời kể dân gian, hồn tướng giữ thành là vị tướng chưa chịu rời vị trí; bóng phân thân nó dẫn lính ảo theo sau, như những ngày còn chỉ huy đồn thành | `cosmetic.title.atlas.hon_tuong` |
| `atlas.page.quai_dam.thanh_co.qua_tinh_lon` | `monster.thanh_co.qua_tinh_lon` | 1 | 10 | 100 | Theo lời kể dân gian, quạ tinh lớn thành cổ đã tích đủ năm tháng để bắn hai tên lệch hướng theo vòm đá | `cosmetic.title.atlas.qua_tinh_lon` |

### Act VI — Núi Thiêng
| atlas_page_id | Source monster | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title reward (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.quai_dam.nui_thieng.vong_linh` | `monster.nui_thieng.vong_linh` | 1 | 10 | 100 | Theo lời kể dân gian, vong linh núi thiêng là linh hồn kẻ chết trên đường hành hương chưa dứt được; nó dịch chỗ rất nhanh nhưng luôn để lại bóng báo trước hướng di chuyển | `cosmetic.title.atlas.vong_linh` |
| `atlas.page.quai_dam.nui_thieng.tinh_thu` | `monster.nui_thieng.tinh_thu` | 1 | 10 | 100 | Theo lời kể dân gian, tinh thú núi thiêng là thú rừng già đã hấp thụ linh khí từ các đá ranh; viên đạn lửa nó phun ra sáng rõ trước khi rời miệng, như điềm báo mỗi khi núi nổi giận | `cosmetic.title.atlas.tinh_thu` |
| `atlas.page.quai_dam.nui_thieng.than_rung_dem` | `monster.nui_thieng.than_rung_dem` | 1 | 5 | 30 | Theo lời kể dân gian, thần rừng đêm núi thiêng là bóng tối tụ hình sau hoàng hôn; chạy trốn nó theo vết mờ trên đất mà nó để lại trước khi lao tới | `cosmetic.title.atlas.than_rung_dem` |
| `atlas.page.quai_dam.nui_thieng.ngu_tinh` | `monster.nui_thieng.ngu_tinh` | 1 | 10 | 100 | Theo lời kể dân gian, ngư tinh núi thiêng là cá lạ bơi trong hồ trên cao; khác với đại vương đã hóa thành tướng tinh dưới vực sâu | `cosmetic.title.atlas.ngu_tinh_song` |
| `atlas.page.quai_dam.nui_thieng.ma_nui` | `monster.nui_thieng.ma_nui` | 1 | 10 | 100 | Theo lời kể dân gian, ma núi sương phủ là bóng người chết vì lạc đường trên núi; nó đánh dấu vùng đất nguy hiểm trước khi hiện ra, nhắc lại lời cảnh báo của những cột mốc ranh giới cũ | `cosmetic.title.atlas.ma_nui` |
| `atlas.page.quai_dam.nui_thieng.than_trung` | `monster.nui_thieng.than_trung` | 1 | 10 | 100 | Theo lời kể dân gian, thần trùng đồng là loài linh trùng nhỏ sống trong vùng đất thiêng; khác với thủ lĩnh đội lốt đã tích đủ khí để thống lĩnh | `cosmetic.title.atlas.than_trung_dong` |
| `atlas.page.quai_dam.nui_thieng.linh_ve` | `monster.nui_thieng.linh_ve` | 1 | 10 | 100 | Theo lời kể dân gian, linh vệ núi thiêng là bộ giáp cũ của người canh cổng núi đã thành tinh; nó giữ thế thủ rồi phản đòn theo kiểu riêng, khác với thạch vệ thành cổ dùng thân đỡ | `cosmetic.title.atlas.linh_ve` |
| `atlas.page.quai_dam.nui_thieng.hon_binh_co` | `monster.nui_thieng.hon_binh_co` | 1 | 10 | 100 | Theo lời kể dân gian, hồn binh cổ núi thiêng là linh hồn lính từ buổi đầu dựng cổng núi; nhát chém quét ngang của nó vẫn đủ lực sau bao thế kỷ canh gác một mình | `cosmetic.title.atlas.hon_binh_co` |
| `atlas.page.quai_dam.nui_thieng.dai_vong_linh` | `monster.nui_thieng.dai_vong_linh` | 1 | 10 | 100 | Theo lời kể dân gian, đại vong linh núi thiêng là những bóng ma lớn đã học được phép đổi hướng lúc chạy nước rút | `cosmetic.title.atlas.dai_vong_linh` |
| `atlas.page.quai_dam.nui_thieng.tinh_nui_gia` | `monster.nui_thieng.tinh_nui_gia` | 1 | 10 | 100 | Theo lời kể dân gian, tinh núi già núi thiêng ẩn dưới đám mây mù; vùng nguy hiểm của nó xuất hiện khi người dám đứng bên bờ vực | `cosmetic.title.atlas.tinh_nui_gia` |
| `atlas.page.quai_dam.nui_thieng.bong_vong` | `monster.nui_thieng.bong_vong` | 1 | 10 | 100 | Theo lời kể dân gian, bóng vong núi thiêng là bóng tối hút sinh lực của người đi lạc; khi nó nắm được người, một phần sức mạnh của nạn nhân biến thành lớp che chở tạm thời cho nó | `cosmetic.title.atlas.bong_vong` |

All 58 monster sources resolve in `monster_catalog.md`.

## Hon Giam — 25 Pages
Source: `soul_catalog.md`. Each soul maps to one atlas page. Unlock via Soul acquisition/level.

| atlas_page_id | Source soul | T1 Seen (acquire) | T2 Studied (Soul Lv3) | T3 Mastered (Soul Lv5) | Title reward (T3) |
|---|---|---|---|---|---|
| `atlas.page.hon_giam.coc_thanh_tinh` | `soul.normal.coc_thanh_tinh` | acquire 1 | Lv3 | Lv5 | `cosmetic.title.atlas.hon_coc` |
| `atlas.page.hon_giam.ho_con_tinh` | `soul.normal.ho_con_tinh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ho_con` |
| `atlas.page.hon_giam.hon_binh` | `soul.normal.hon_binh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_binh_suu` |
| `atlas.page.hon_giam.tinh_cay` | `soul.normal.tinh_cay` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_tinh_cay` |
| `atlas.page.hon_giam.ma_rung` | `soul.normal.ma_rung` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_rung` |
| `atlas.page.hon_giam.khi_nui` | `soul.normal.khi_nui` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_khi_nui` |
| `atlas.page.hon_giam.ma_da` | `soul.normal.ma_da` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_da2` |
| `atlas.page.hon_giam.ca_tinh` | `soul.normal.ca_tinh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ca_tinh` |
| `atlas.page.hon_giam.hon_chet_duoi` | `soul.normal.hon_chet_duoi` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_chet_duoi2` |
| `atlas.page.hon_giam.dom_dom_ma` | `soul.normal.dom_dom_ma` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_dom_dom` |
| `atlas.page.hon_giam.dom_lua` | `soul.normal.dom_lua` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_dom_lua` |
| `atlas.page.hon_giam.qua_tinh` | `soul.normal.qua_tinh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_qua_tinh` |
| `atlas.page.hon_giam.bu_nhin_rom` | `soul.normal.bu_nhin_rom` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_bu_nhin` |
| `atlas.page.hon_giam.vong_hon` | `soul.normal.vong_hon` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_vong_hon2` |
| `atlas.page.hon_giam.ma_co` | `soul.normal.ma_co` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_co2` |
| `atlas.page.hon_giam.ho_tinh_ve` | `soul.elite.ho_tinh_ve` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ho_tinh_ve` |
| `atlas.page.hon_giam.thach_ve` | `soul.elite.thach_ve` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_thach_ve` |
| `atlas.page.hon_giam.moc_tinh` | `soul.elite.moc_tinh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_moc_tinh` |
| `atlas.page.hon_giam.ma_tranh` | `soul.elite.ma_tranh` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_tranh2` |
| `atlas.page.hon_giam.ma_da_gia` | `soul.elite.ma_da_gia` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_da_gia2` |
| `atlas.page.hon_giam.ma_xo` | `soul.elite.ma_xo` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_xo2` |
| `atlas.page.hon_giam.ma_tranh_gia` | `soul.elite.ma_tranh_gia` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ma_tranh_gia2` |
| `atlas.page.hon_giam.thuong_luong` | `soul.boss.thuong_luong` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_thuong_luong2` |
| `atlas.page.hon_giam.ho_tinh_chin_duoi` | `soul.boss.ho_tinh_chin_duoi` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_ho_tinh_chin` |
| `atlas.page.hon_giam.than_trung` | `soul.boss.than_trung` | acquire | Lv3 | Lv5 | `cosmetic.title.atlas.hon_than_trung2` |

All 25 soul sources resolve in `soul_catalog.md`.

## Di Tich — 8 Pages
Source: `boss_catalog.md`. Each boss aftermath relic.

| atlas_page_id | Source boss | T1 Seen (1 witness) | T2 Studied (3 witnesses) | T3 Mastered (10 witnesses) | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.di_tich.quy_nhap_trang` | `boss.quy_nhap_trang` | 1 | 3 | 10 | `cosmetic.title.atlas.di_quy` |
| `atlas.page.di_tich.moc_tinh_da` | `boss.moc_tinh_da` | 1 | 3 | 10 | `cosmetic.title.atlas.di_moc` |
| `atlas.page.di_tich.thuong_luong` | `boss.thuong_luong` | 1 | 3 | 10 | `cosmetic.title.atlas.di_thuong` |
| `atlas.page.di_tich.ma_da_chua` | `boss.ma_da_chua` | 1 | 3 | 10 | `cosmetic.title.atlas.di_ma_da_chua` |
| `atlas.page.di_tich.ho_tinh` | `boss.ho_tinh` | 1 | 3 | 10 | `cosmetic.title.atlas.di_ho_tinh` |
| `atlas.page.di_tich.ho_tinh_chin_duoi` | `boss.ho_tinh_chin_duoi` | 1 | 3 | 10 | `cosmetic.title.atlas.di_ho_chin` |
| `atlas.page.di_tich.ngu_tinh` | `boss.ngu_tinh` | 1 | 3 | 10 | `cosmetic.title.atlas.di_ngu_tinh` |
| `atlas.page.di_tich.than_trung` | `boss.than_trung` | 1 | 3 | 10 | `cosmetic.title.atlas.di_than_trung` |

Witness = within 60m of `relic.boss.<boss_id>` while `buff.di_tich.<boss_id>` active.

## Co Vat — 13 Pages
Folklore objects & life skills.

| atlas_page_id | Source | T1 Seen | T2 Studied | T3 Mastered | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.co_vat.ruong_co_01` | Open 1 hidden chest (`chest.hidden.*`) | 1 | 10 | 36 (all) | `cosmetic.title.atlas.kho_bau` |
| `atlas.page.co_vat.chia_khoa_co` | Obtain `item.consumable.chia_khoa_co` | 1 | 20 | 50 | `cosmetic.title.atlas.chia_khoa` |
| `atlas.page.co_vat.ruou_nep` | Consume `item.consumable.ruou_nep` at bonfire | 1 | 10 | 50 | `cosmetic.title.atlas.ruou_nep` |
| `atlas.page.co_vat.cui_lua_trai` | Kindle bonfire with `item.material.cui_lua_trai` | 1 | 10 | 50 | `cosmetic.title.atlas.cui_lua` |
| `atlas.page.co_vat.can_cau_tre` | Obtain `item.tool.can_cau_tre` | acquire | catch 10 | catch 100 | `cosmetic.title.atlas.can_cau` |
| `atlas.page.co_vat.ca_bong` | Catch `item.material.ca_bong` | 1 | 10 | 50 | `cosmetic.title.atlas.ca_bong2` |
| `atlas.page.co_vat.ca_chep` | Catch `item.material.ca_chep` | 1 | 10 | 50 | `cosmetic.title.atlas.ca_chep2` |
| `atlas.page.co_vat.tom_song` | Catch `item.material.tom_song` | 1 | 10 | 50 | `cosmetic.title.atlas.tom_song2` |
| `atlas.page.co_vat.ca_chep_hoa_rong` | Catch `item.material.ca_chep_hoa_rong` | 1 | 3 | 10 | `cosmetic.title.atlas.ca_chep_hoa_rong` |
| `atlas.page.co_vat.ca_bong_kho` | Cook `item.consumable.food.ca_bong_kho` | 1 | 10 | 50 | `cosmetic.title.atlas.bep_bong` |
| `atlas.page.co_vat.ca_chep_nuong` | Cook `item.consumable.food.ca_chep_nuong` | 1 | 10 | 50 | `cosmetic.title.atlas.bep_chep` |
| `atlas.page.co_vat.tom_nuong` | Cook `item.consumable.food.tom_nuong` | 1 | 10 | 50 | `cosmetic.title.atlas.bep_tom` |
| `atlas.page.co_vat.linh_dan` | Obtain any `item.material.linh_dan.*` | 1 | 50 | 200 | `cosmetic.title.atlas.linh_dan2` |

All 13 sources resolve in `item_catalog.md` and `world_rules.md`. T1 Seen of `ca_chep_hoa_rong` does not emit a second `PHAT_HIEN` when the catch already emitted `source=FISH_RARE`.

## Rewards Detailed
For each page tier, deterministic rewards (no RNG):

- **T1**: `1 currency.special` + unlock lore illustration.
- **T2**: `2 currency.special` + page frame + progress toward milestone title.
- **T3**: `2 currency.special` + glowing title `cosmetic.title.atlas.*` (listed per row) + card aura.

Milestone rewards (accounted separately in `cosmetic_catalog.md`):
- 20 pages mastered → `cosmetic.title.atlas.nha_suu_tam`
- 50 pages mastered → `cosmetic.title.atlas.hoc_gia_dan_gian`
- 104 pages mastered → `cosmetic.title.atlas.bach_khoa_dan_gian` (mythic gold aura)

## Seasonal Atlas Pages
Seasonal Atlas pages are additional to the 104 launch pages. Each 8-week season contributes 10 pages tied to the featured region's folklore themes (see `../03_systems/seasons.md` for cadence rules).

### Night-Only Page Thresholds
Pages whose only source monster spawns exclusively in a `NIGHT_RARE` group (`map_spawn_catalog.md`: `hon_do_trang`, `quy_song_dem`, `ma_van_dem`, `oan_hon_dem`, `than_rung_dem`) use T2 = 5 kills and T3 = 30 kills, because each has at most one alive per channel at night with a 300 s respawn.

### Seasonal Page ID Contract
```text
atlas.page.season.<season_region_index>.<page_key>
```
Where `<season_region_index>` is `season_number mod 6` (0..5; canonical derivation in `../03_systems/seasons.md`) and `<page_key>` follows the same ASCII snake_case convention as launch pages. Repeat cycles (season_number ≥ 6) reuse the same IDs, meaning Season 6 reuses `atlas.page.season.0.*`, etc.

### Seasonal Page Template
Each seasonal chapter follows the launch tier conditions, but seasonal pages grant **no `currency.special`** so the per-character special faucet stays frozen at 540 (`economy_catalog.md`):

| Tier | Name | Condition | Reward |
|---|---|---|---|
| 1 | Seen | First qualifying event in the season's featured region | lore entry only |
| 2 | Studied | 10 kills / 10 gathers / 3 boss witnesses | page frame |
| 3 | Mastered | 100 kills / 50 gathers / 10 boss witnesses | glowing title `cosmetic.title.season.<n>.<key>` |

### Season 0 — Zone Làng Đa (season_number = 0, season_region_index = 0)
Concrete pages unlocked during the first Làng Đa featured season:

| atlas_page_id | Source | T1 Seen | T2 Studied | T3 Mastered | Lore snippet | Title (T3) |
|---|---|---|---|---|---|---|
| `atlas.page.season.0.lang_da.dom_dom_nguyen` | `monster.lang_da.dom_dom_nguyen` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, đom đóm nguyên sơ là linh hồn nhỏ đầu tiên xuất hiện ở bến Đình Làng khi trời chưa sáng; khác với đom đóm ma thường gặp, nó chỉ xuất hiện đúng dịp và không ai biết từ đâu đến | `cosmetic.title.season.0.dom_nguyen` |
| `atlas.page.season.0.lang_da.hon_gao` | `monster.lang_da.hon_gao` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, hồn gạo gò mả là linh hồn gắn vào những nắm gạo cúng để lâu không ai thu; đêm khuya nó lang thang theo bờ tre, tìm lại chỗ cũ như còn nhớ ơn người thờ cúng | `cosmetic.title.season.0.hon_gao` |
| `atlas.page.season.0.lang_da.bup_lua` | `monster.lang_da.bup_lua` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, búp lửa đồng là bông lửa nhỏ bay trên ruộng lúa đêm hè; có người nói nó là khí đất tích tụ, có người nói là hồn lúa chưa gặt kịp trước mưa lũ | `cosmetic.title.season.0.bup_lua` |
| `atlas.page.season.0.lang_da.vong_bien` | `monster.lang_da.vong_bien` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, vong biến bến đa là bóng người đã qua đò nhưng chưa đến nơi; nó lặp lại con đường mình quen, không biết mình đã không còn là người sống | `cosmetic.title.season.0.vong_bien` |
| `atlas.page.season.0.lang_da.tinh_buoi` | `monster.lang_da.tinh_buoi` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, tinh bưởi đồng làng là hồn cây bưởi trồng ven ruộng đã thành tinh nhỏ; khác với các tinh cây to lớn trong rừng sâu, nó chỉ bảo vệ mảnh vườn mà nó đã gắn bó từ lúc còn là hạt giống | `cosmetic.title.season.0.tinh_buoi` |
| `atlas.page.season.0.lang_da.co_lua` | `monster.lang_da.co_lua` | 1 kill | 10 kills | 100 kills | Theo lời kể dân gian, cô lúa già là bù nhìn đứng canh đồng từ nhiều mùa gặt; bao nhiêu hồn lúa thấm vào cọc tre cũ đã làm nó cử động vào đêm khuya, dù không ai thấy nó nhúc nhích khi trời sáng | `cosmetic.title.season.0.co_lua` |
| `atlas.page.season.0.lang_da.ky_xuan` | `chest.hidden.season.0.lang_da.01` | 1 | 5 | 20 | — | `cosmetic.title.season.0.ky_xuan` |
| `atlas.page.season.0.lang_da.ca_linh_giang` | `item.material.ca_linh_giang` | 1 | 10 | 50 | — | `cosmetic.title.season.0.ca_linh` |
| `atlas.page.season.0.lang_da.di_tich_quy_xuan` | `relic.season.0.quy_xuan` | 1 | 3 | 10 | — | `cosmetic.title.season.0.di_quy_xuan` |
| `atlas.page.season.0.lang_da.di_tich_hon_dau` | `relic.season.0.hon_dau` | 1 | 3 | 10 | — | `cosmetic.title.season.0.di_hon_dau` |

Season 0 relics reuse launch Di Tích witness (60m / `buff.di_tich`):
- `relic.season.0.quy_xuan` @ `map.lang_da.go_ma` after eligible `dungeon.dinh_lang_bo_hoang` completion while `season_region_index=0`
- `relic.season.0.hon_dau` @ `map.lang_da.go_ma` after kill of `monster.lang_da.hon_gao` while `season_region_index=0`

### Season 2 — Zone Rừng U Minh (season_number = 1)
| atlas_page_id | Source | T1 | T2 | T3 | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.season.1.u_minh.ma_rung` | `monster.rung_u_minh.ma_rung` | 1 | 10 | 100 | `cosmetic.title.season.1.ma_rung` |
| `atlas.page.season.1.u_minh.dom_lua` | `monster.rung_u_minh.dom_lua` | 1 | 10 | 100 | `cosmetic.title.season.1.dom_lua` |
| `atlas.page.season.1.u_minh.bong_nguoi` | `monster.rung_u_minh.bong_nguoi` | 1 | 10 | 100 | `cosmetic.title.season.1.bong_nguoi` |
| `atlas.page.season.1.u_minh.tinh_cay` | `monster.rung_u_minh.tinh_cay` | 1 | 10 | 100 | `cosmetic.title.season.1.tinh_cay` |
| `atlas.page.season.1.u_minh.dai_tinh_cay` | `monster.rung_u_minh.dai_tinh_cay` | 1 | 10 | 100 | `cosmetic.title.season.1.dai_tinh_cay` |
| `atlas.page.season.1.u_minh.vong_rung_sau` | `monster.rung_u_minh.vong_rung_sau` | 1 | 10 | 100 | `cosmetic.title.season.1.vong_rung_sau` |
| `atlas.page.season.1.u_minh.ky_ram` | `chest.hidden.season.1.u_minh.01` | 1 | 5 | 20 | `cosmetic.title.season.1.ky_ram` |
| `atlas.page.season.1.u_minh.ca_sam` | `item.material.ca_sam_u_minh` | 1 | 10 | 50 | `cosmetic.title.season.1.ca_sam` |
| `atlas.page.season.1.u_minh.di_mieu` | `relic.season.1.moc_mieu` | 1 | 3 | 10 | `cosmetic.title.season.1.di_mieu` |
| `atlas.page.season.1.u_minh.di_moc` | `relic.season.1.moc_tinh` | 1 | 3 | 10 | `cosmetic.title.season.1.di_moc` |

Relics: `relic.season.1.moc_mieu` @ `map.rung_u_minh.mieu_bo_hoang` after `dungeon.mieu_ba_trong_rung` while `season_region_index=1`. `relic.season.1.moc_tinh` after kill of `monster.rung_u_minh.moc_tinh`.

### Season 3 — Zone Bến Nước Đen (season_number = 2)
| atlas_page_id | Source | T1 | T2 | T3 | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.season.2.ben_nuoc.ma_da` | `monster.ben_nuoc_den.ma_da` | 1 | 10 | 100 | `cosmetic.title.season.2.ma_da` |
| `atlas.page.season.2.ben_nuoc.ca_tinh` | `monster.ben_nuoc_den.ca_tinh` | 1 | 10 | 100 | `cosmetic.title.season.2.ca_tinh` |
| `atlas.page.season.2.ben_nuoc.quy_song_dem` | `monster.ben_nuoc_den.quy_song_dem` | 1 | 5 | 30 | `cosmetic.title.season.2.quy_song_dem` |
| `atlas.page.season.2.ben_nuoc.bong_nuoc_ma` | `monster.ben_nuoc_den.bong_nuoc_ma` | 1 | 10 | 100 | `cosmetic.title.season.2.bong_nuoc_ma` |
| `atlas.page.season.2.ben_nuoc.hon_chet_duoi` | `monster.ben_nuoc_den.hon_chet_duoi` | 1 | 10 | 100 | `cosmetic.title.season.2.hon_chet_duoi` |
| `atlas.page.season.2.ben_nuoc.ca_tinh_gia` | `monster.ben_nuoc_den.ca_tinh_gia` | 1 | 10 | 100 | `cosmetic.title.season.2.ca_tinh_gia` |
| `atlas.page.season.2.ben_nuoc.ky_ben` | `chest.hidden.season.2.ben_nuoc.01` | 1 | 5 | 20 | `cosmetic.title.season.2.ky_ben` |
| `atlas.page.season.2.ben_nuoc.ca_bong_den` | `item.material.ca_bong_den` | 1 | 10 | 50 | `cosmetic.title.season.2.ca_bong_den` |
| `atlas.page.season.2.ben_nuoc.di_xom` | `relic.season.2.xom_chim` | 1 | 3 | 10 | `cosmetic.title.season.2.di_xom` |
| `atlas.page.season.2.ben_nuoc.di_ma_da` | `relic.season.2.ma_da_gia` | 1 | 3 | 10 | `cosmetic.title.season.2.di_ma_da` |

Relics: `relic.season.2.xom_chim` after `dungeon.xom_chim` while `season_region_index=2`. `relic.season.2.ma_da_gia` after kill of `monster.ben_nuoc_den.ma_da_gia`.

### Season 4 — Zone Đèo Mây (season_number = 3)
No `has_water` maps in this region: second co_vat is `DISH_COOKED`, not fish.
| atlas_page_id | Source | T1 | T2 | T3 | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.season.3.deo_may.ma_tranh` | `monster.deo_may.ma_tranh` | 1 | 10 | 100 | `cosmetic.title.season.3.ma_tranh` |
| `atlas.page.season.3.deo_may.khi_nui` | `monster.deo_may.khi_nui` | 1 | 10 | 100 | `cosmetic.title.season.3.khi_nui` |
| `atlas.page.season.3.deo_may.ma_van_dem` | `monster.deo_may.ma_van_dem` | 1 | 5 | 30 | `cosmetic.title.season.3.ma_van_dem` |
| `atlas.page.season.3.deo_may.ho_con_tinh` | `monster.deo_may.ho_con_tinh` | 1 | 10 | 100 | `cosmetic.title.season.3.ho_con_tinh` |
| `atlas.page.season.3.deo_may.vong_rung` | `monster.deo_may.vong_rung` | 1 | 10 | 100 | `cosmetic.title.season.3.vong_rung` |
| `atlas.page.season.3.deo_may.ho_tinh_lon` | `monster.deo_may.ho_tinh_lon` | 1 | 10 | 100 | `cosmetic.title.season.3.ho_tinh_lon` |
| `atlas.page.season.3.deo_may.ky_deo` | `chest.hidden.season.3.deo_may.01` | 1 | 5 | 20 | `cosmetic.title.season.3.ky_deo` |
| `atlas.page.season.3.deo_may.ruou_nep` | `item.consumable.ruou_nep` (`DISH_COOKED`) | 1 | 10 | 50 | `cosmetic.title.season.3.ruou_nep` |
| `atlas.page.season.3.deo_may.di_hang` | `relic.season.3.hang` | 1 | 3 | 10 | `cosmetic.title.season.3.di_hang` |
| `atlas.page.season.3.deo_may.di_ho_ve` | `relic.season.3.ho_ve` | 1 | 3 | 10 | `cosmetic.title.season.3.di_ho_ve` |

Relics: `relic.season.3.hang` after `dungeon.hang_ma_tranh` while `season_region_index=3`. `relic.season.3.ho_ve` after kill of `monster.deo_may.ho_tinh_ve`.

### Season 5 — Zone Thành Cổ (season_number = 4)
No `has_water` maps: second co_vat is `DISH_COOKED`.
| atlas_page_id | Source | T1 | T2 | T3 | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.season.4.thanh_co.tuong_da` | `monster.thanh_co.tuong_da` | 1 | 10 | 100 | `cosmetic.title.season.4.tuong_da` |
| `atlas.page.season.4.thanh_co.hon_binh` | `monster.thanh_co.hon_binh` | 1 | 10 | 100 | `cosmetic.title.season.4.hon_binh` |
| `atlas.page.season.4.thanh_co.ma_co` | `monster.thanh_co.ma_co` | 1 | 10 | 100 | `cosmetic.title.season.4.ma_co` |
| `atlas.page.season.4.thanh_co.qua_tinh` | `monster.thanh_co.qua_tinh` | 1 | 10 | 100 | `cosmetic.title.season.4.qua_tinh` |
| `atlas.page.season.4.thanh_co.hon_tran_linh` | `monster.thanh_co.hon_tran_linh` | 1 | 10 | 100 | `cosmetic.title.season.4.hon_tran_linh` |
| `atlas.page.season.4.thanh_co.qua_tinh_lon` | `monster.thanh_co.qua_tinh_lon` | 1 | 10 | 100 | `cosmetic.title.season.4.qua_tinh_lon` |
| `atlas.page.season.4.thanh_co.ky_thanh` | `chest.hidden.season.4.thanh_co.01` | 1 | 5 | 20 | `cosmetic.title.season.4.ky_thanh` |
| `atlas.page.season.4.thanh_co.ca_bong_kho` | `item.consumable.food.ca_bong_kho` (`DISH_COOKED`) | 1 | 10 | 50 | `cosmetic.title.season.4.ca_bong_kho` |
| `atlas.page.season.4.thanh_co.di_den` | `relic.season.4.den` | 1 | 3 | 10 | `cosmetic.title.season.4.di_den` |
| `atlas.page.season.4.thanh_co.di_thach` | `relic.season.4.thach` | 1 | 3 | 10 | `cosmetic.title.season.4.di_thach` |

Relics: `relic.season.4.den` after `dungeon.den_tran` while `season_region_index=4`. `relic.season.4.thach` after kill of `monster.thanh_co.thach_ve`.

### Season 6 — Zone Núi Thiêng (season_number = 5)
| atlas_page_id | Source | T1 | T2 | T3 | Title (T3) |
|---|---|---|---|---|---|
| `atlas.page.season.5.nui_thieng.vong_linh` | `monster.nui_thieng.vong_linh` | 1 | 10 | 100 | `cosmetic.title.season.5.vong_linh` |
| `atlas.page.season.5.nui_thieng.tinh_thu` | `monster.nui_thieng.tinh_thu` | 1 | 10 | 100 | `cosmetic.title.season.5.tinh_thu` |
| `atlas.page.season.5.nui_thieng.than_rung_dem` | `monster.nui_thieng.than_rung_dem` | 1 | 5 | 30 | `cosmetic.title.season.5.than_rung_dem` |
| `atlas.page.season.5.nui_thieng.ma_nui` | `monster.nui_thieng.ma_nui` | 1 | 10 | 100 | `cosmetic.title.season.5.ma_nui` |
| `atlas.page.season.5.nui_thieng.hon_binh_co` | `monster.nui_thieng.hon_binh_co` | 1 | 10 | 100 | `cosmetic.title.season.5.hon_binh_co` |
| `atlas.page.season.5.nui_thieng.dai_vong_linh` | `monster.nui_thieng.dai_vong_linh` | 1 | 10 | 100 | `cosmetic.title.season.5.dai_vong_linh` |
| `atlas.page.season.5.nui_thieng.ky_nui` | `chest.hidden.season.5.nui_thieng.01` | 1 | 5 | 20 | `cosmetic.title.season.5.ky_nui` |
| `atlas.page.season.5.nui_thieng.ca_suong` | `item.material.ca_suong_ho` | 1 | 10 | 50 | `cosmetic.title.season.5.ca_suong` |
| `atlas.page.season.5.nui_thieng.di_cong` | `relic.season.5.cong` | 1 | 3 | 10 | `cosmetic.title.season.5.di_cong` |
| `atlas.page.season.5.nui_thieng.di_linh_ve` | `relic.season.5.linh_ve` | 1 | 3 | 10 | `cosmetic.title.season.5.di_linh_ve` |

Relics: `relic.season.5.cong` after eligible `boss.than_trung` while `season_region_index=5`. `relic.season.5.linh_ve` after kill of `monster.nui_thieng.linh_ve`.

Subsequent cycle rosters (`season_number ≥ 6`) reuse the matching `season_number mod 6` IDs. No new ID is minted for a repeat cycle.

### Seasonal Special Budget
Seasonal pages grant `0` `currency.special`. The lifetime per-character special faucet remains 540 (520 launch Atlas + 20 base PvE); seasonal chapters reward only cosmetics (frame, title) and are character-scoped (ADR-0029).

## Persistence & Idempotency
Same as `03_systems/atlas.md`:
```
character_atlas(character_id, atlas_page_id, tier, seen_count, completed_at, reward_operation_id, acknowledged_at NULL)
```
Idempotency key per tier:
```
atlas.tier.<character_id>.<atlas_page_id>.<tier>   (canonical in 03_systems/atlas.md; promotion, EXP and reward settle atomically)
```
Retry never duplicates special/title. `currency.special` cap (1,000,000 per character) applies; special overflow is not allowed to block atlas; atlas special is character-scoped (ADR-0029) and credits the character's special balance directly.

## Validation
- All 58 quai_dam `monster.*` resolve.
- All 25 hon_giam `soul.*` resolve.
- All 8 di_tich `boss.*` resolve.
- All 13 co_vat item sources resolve.
- Total = 104.
- No atlas reward grants combat power.
- No duplicate atlas_page_id.
- Every atlas title resolves in `cosmetic_catalog.md`.
- quai_dam pages for quy_nhap_trang / thuong_luong / ho_tinh / ngu_tinh / than_trung are distinct from their respective di_tich pages in ID, unlock trigger, lore framing, and title reward.

## Invariants
```text
launch atlas pages = 104 (58 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat)
seasonal atlas pages = 10 per season (6 quai_dam + 2 co_vat + 2 di_tich), counted separately from 104 launch pages
quai_dam (launch) = 58 (46 NORMAL + 12 ELITE)
hon_giam (launch) = 25
di_tich (launch) = 8
co_vat (launch) = 13
max atlas special per character = 520 (104 pages × T1=1 + T2=2 + T3=2)
total faucet including base PvE = 540 (within ~545 sink surface)
atlas is non-power
atlas unlock is character-scoped
one page tier + character -> at most one reward
atlas never gates MAIN
seasonal pages never permanently lost (re-enter on region repeat)
```
