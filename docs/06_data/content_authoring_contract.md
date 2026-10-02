# Content Authoring Contract & Ingest Schema
status: LOCKED

## Scope

Hợp đồng quy định cú pháp nguồn, schema trường, kiểu dữ liệu, quy tắc mở rộng hữu hạn (finite expansion), hàm băm phiên bản (content hash) và vòng đời kích hoạt nguyên tử (atomic activation) cho toàn bộ 24 catalogs trong `docs/07_content/` và các registered competitive-space source sections trong `../03_systems/pvp.md` + `../03_systems/guild_war.md` (§3).

Tài liệu này là đặc tả đầu vào duy nhất cho Content Compiler (`IMP-003`, `server/cmd/compiler/`) và Activation Gate (`IMP-004`, `server/internal/config/`).

## 1. Canonical Markdown Source Grammar

The only hand-authored catalog source is the 24 Markdown files in §3, plus two registered spec sections: `../03_systems/pvp.md` (§ Competitive Space Geometry table + § Five Element Arena anchor fence) and `../03_systems/guild_war.md` (§ Map geometry and objective-ID fences). Each declares its own `Compiler Source Schema` under this grammar and emits `space_geometry`/space-anchor bindings only; they are registered spec-section compile sources, not catalogs. The source is **not tables-only**: registered heading identities, inherited `text` assignment fences, typed tables and named finite rules are legal. No second JSON/YAML catalog or private scraper is permitted. Generated snapshots are compiler outputs, not authoring inputs.

Every data-owning catalog has a named `Compiler Source Schema` section. Its source registry uses `source_section | output / key | typed inputs | defaults / finite rule`. A source locator is an exact heading ancestry and, for a table, its exact header signature; repeated headers are disambiguated by ancestry. Case/punctuation aliases are legal only when that registry explicitly lists them (`Tier` is not globally rewritten to `tier`). A family can have several tables; a catalog never has one universal primary key.

### Lexing and source selection

- Read UTF-8, accept LF or CRLF, normalize line endings to LF. A heading is `#{1,6} SPACE title`; fences suspend heading/table recognition. Heading levels establish scope; encountering the next same-or-higher-level heading ends inheritance. Strip a matching single pair of inline backticks from a scalar token, never delete embedded backticks or arbitrary characters.
- Tables use leading/trailing `|`, equal header/cell count, and a divider cell matching `:?-{3,}:?`. Split only unescaped pipes outside inline-code spans; `\|` is a literal pipe. Trim cell margins. Headers have unique names after the registered alias mapping. Unknown/missing required columns reject; alignment punctuation carries no semantics.
- Registered `text` fences contain one `field = value` assignment per nonblank line. Split at the first `=`. Duplicate explicit fields in one record reject. Nested records/lists or non-assignment profile blocks require their own catalog-local grammar; they are not assignment fences by default.
- A heading exactly containing a backticked stable ID supplies identity only inside a registered family (`item.*`, `soul.*`, etc.). It cannot create a record in an unrelated scope. A registered ancestor fence supplies that section's defaults; then the leaf fence/table supplies explicit values. Table/leaf overrides of inherited defaults are legal; two explicit values for the same output field reject. Defaults do not leak across sibling sections.
- Empty cells are errors unless a field's schema explicitly declares an empty default. Missing optional field, `NONE`, empty list and zero are distinct. `NONE` is legal only for a nullable field or an enum which names it. There are no inferred zero/false/empty defaults.
- Headings, explanatory prose, `Display:` lines, cultural notes and example fences are ignored unless registered as a typed string or source token. Runtime fields may **not** be recovered from English/Vietnamese prose, display names, sprites or examples. Catalog owners normalize operative prose to authoritative typed tables/fences in the same owning Markdown file; the old prose is explanatory only.
- Each source registry must enumerate all runtime/output fields, explicit required/default values, heading-to-record inheritance and finite rules. A missing source/schema/field, unregistered data table inside a registered runtime section, unknown rule or duplicate source binding rejects. A compile coverage report lists every consumed section and every emitted field's source or named derivation; a field cannot be supplied by an implementation-only constant.

### Typed scalar and collection grammar

| Type | Source grammar | Semantic interpretation |
|---|---|---|
| `id` | `[a-z0-9_]+(\.[a-z0-9_]+)*` | Stable ASCII identity; references resolve against their declared family. |
| `int` | `-?[0-9]+` | Signed 64-bit checked arithmetic unless owning field gives a narrower bound. Presentation grouping `1,000` is allowed only for a column explicitly marked `grouped_int`; remove commas only after matching `[0-9]{1,3}(,[0-9]{3})+`. |
| `decimal`, `ratio` | `-?[0-9]+(\.[0-9]+)?` | Exact base-10 rational, reduced numerator/positive denominator. No binary-float parsing, implicit epsilon or scientific notation. Owning field declares precision/range and final rounding stage. |
| `bp` | integer `0..10000` | Integer basis points. A registered percentage alias parses an exact `%` decimal then multiplies by 100; reject nonintegral bp. |
| `enum(E)` | exact token in finite `E` | Case-sensitive; stable uppercase tokens may be distinct from lowercase content IDs. |
| `bool` | `true`, `false` | Exact lowercase tokens. |
| `range(T)` | `min..max` | Inclusive typed endpoints, `min <= max`. Legacy `1-10`/`Lv1`/`T1`/`10s` forms are accepted only by a registered column adapter with exact prefix/suffix and units. |
| `set(T)` | comma-delimited typed tokens | Unique elements; reject duplicates; sort canonical values. |
| `ordered(T)` | comma-delimited typed tokens | Preserve declared semantic order (slots, phase/stage sequence, effect order). Presentation row order never supplies an ordinal. |
| `pair(T)` | `a x b` or `axb` | Exactly two typed values; only where explicitly registered (bounds/screens/pixel extent). |
| `string` | UTF-8 text | Normalize to Unicode NFC; preserve interior whitespace/content. Strings never supply IDs or executable effects. |
| `expr` | grammar below | Typed expression tree, no arbitrary code or natural-language evaluator. |

Expressions have literals, declared field references, parentheses, unary `-`, binary `+ - * /`, and finite `floor`, `ceil`, `min`, `max`, `clamp` calls. Multiplication/division precede addition/subtraction; binary operators associate left. Arguments are comma-separated. No implicit multiplication, assignment, randomness, reflection or calls outside this set. Division is exact rational until an explicit rounding operator; division by zero/overflow reject. `round_half_up` is an explicitly declared field adapter, not an unstated integer default. All references resolve in the named finite rule's input environment. Enum selectors, matcher predicates, weighted rolls, DOT schedules and effect payloads use their owning typed tables/registered finite algorithms, not this arithmetic expression grammar.

`*` in a selector is permitted only in a registered finite expansion: enumerate the declared launch domain, sort its stable IDs, require nonempty resolved membership, then emit concrete references. Runtime wildcard identities are forbidden. Explicit `...` or omitted rows never denote data.

## 3. Danh mục 24 Launch Catalogs và Primary Keys

| # | Catalog file | Compiled families / actual keys | Schema and finite source owner |
|---|---|---|---|
| 1 | `monster_catalog.md` | monsters `monster_id`; attacks `(monster_id, attack_id)`; mechanic payloads | Roster, stat/movement/combat profiles, explicit attacks and mechanic tables; 46 NORMAL + 12 ELITE. |
| 2 | `boss_catalog.md` | bosses `boss_id`; phases `(boss_id, phase_number)`; attacks/mechanics `(boss_id, mechanic_id)` | Roster + numeric/typed mechanic payloads; 8 bosses, finale phases are not extra IDs. |
| 3 | `class_skill_catalog.md` | skills `skill_id`; effect templates `effect_id`; geometry `(skill_id, geometry_id)` | Runtime, scaling, timing, payload and geometry matrices; 60 skills / 45 primary action geometries. |
| 4 | `equipment_catalog.md` | equipment `item_id`; sets `set_key`; budgets `tier`; slots `slot`; roll types `stat_id`; set effects `(set_key, threshold, effect_id)` | 12 set heading/fence records × 14 ordered slots = 168 `item.eq.*`; **not** `equipment_id`. |
| 5 | `item_catalog.md` | items `item_id`; catch entries `(table_id, item_id)`; book grants `(item_id, level)` | Regional inherited defaults + item-ID headings/assignment fences + typed use/catch/book tables. |
| 6 | `drop_tables.md` | tables `drop_table_id`; lines `(drop_table_id, reward_slot)`; weighted groups `(drop_table_id, group_id, entry_id)` | Typed templates, regional domains and explicit reward/first-clear mappings; grouped rolls retain declared ordinals. |
| 7 | `crafting_catalog.md` | recipes `recipe_id`; inputs `(recipe_id, item_id)`; tier/slot costs | Finite equipment recipe expansion plus explicit utility/food recipes; output ID is `item_id`. |
| 8 | `npc_shop_catalog.md` | NPCs `npc_id`; shops `shop_id`; offers `(shop_id, offer_id)`; service routes `(npc_id, service_id)` | Registered shop-ID ancestor fences inherit into **offer_id** tables; NPC capability/default/source-binding tables. |
| 9 | `quest_catalog.md` | quests `quest_id`; objectives `(quest_id, ordinal)`; rewards `(quest_id, reward_slot)`; Daily templates `template_id` | MAIN/SIDE/Daily tables, ordered typed objectives, deterministic board rule and inline rewards. |
| 10 | `dungeon_catalog.md` | dungeons `dungeon_id`; stages `(dungeon_id, stage_id)`; variants `(dungeon_id, variant_id)` | Shared defaults, bounds, stage headings/tables, typed objective/route/EXP/reward/remix rules; 5 dungeons. |
| 11 | `world_route_catalog.md` | maps `map_id`; portals `portal_id`; checkpoints `checkpoint_id`; anchors `anchor_id`; chests `chest_id` | Map/bounds/discovery, finite bidirectional edge expansion, entry/return gates, chest/social-anchor tables; 24 maps/52 entry portals. |
| 12 | `map_spawn_catalog.md` | groups `spawn_group_id`; memberships `(spawn_group_id, monster_id)` | Heading/fence inherited profiles and explicit selector/population/respawn tables; 54 persistent + 6 separate night groups. |
| 13 | `atlas_catalog.md` | pages `atlas_page_id`; tier rewards `(atlas_page_id, tier)` | Exact launch/season source tables + typed trigger/counter/tier rules; 104 launch + 60 first-cycle seasonal pages. |
| 14 | `cosmetic_catalog.md` | cosmetics `cosmetic_id`; unlock/redemption `(cosmetic_id, source_id)` | Typed unlock conditions, ownership/default tables and finite title/season/sink domains; 294 stable IDs. |
| 15 | `soul_catalog.md` | Souls `soul_id`; effects `(soul_id, effect_id)` | 25 typed roster rows with required explicit Soul element + authoritative trigger/scaling/payload/DOT/secondary geometry tables; element is not derived from source combat element. |
| 16 | `build_catalog.md` | resonances `resonance_id`; Formations `formation_id`; effect payloads | Exact matcher/effect tables + registered topology/selection rules; **not** `entry_id`; 15/12 definitions. |
| 17 | `spirit_beast_catalog.md` | beasts `beast_id`; passives `(beast_id, passive_slot)`; equipment `item_id`; costs `(target_level, item_id)` | Roster, base-stat curves, typed P1/P2 payloads/ICDs/resonance units, 18 equipment finite records. |
| 18 | `economy_catalog.md` | bands `(currency_id, rank, tier)`; faucets/sinks `source_id`; observation gates `gate_id` | Currency heading inheritance + exact numeric band/source/sink/service/affordability/observation tables. No fictitious universal `table_id`. |
| 19 | `world_event_catalog.md` | events `event_id`; variants `(event_id, region_id)`; stages/mechanics | Explicit UTC rotation, contribution, population, telegraph, EXP/reward and lifecycle rules. |
| 20 | `encounter_catalog.md` | regions `zone_id`; maps `map_id`; encounters `encounter_id`; mechanics `mechanic_id` | Narrative roster identities joined to typed mechanic topology/interaction payloads; narrative text cannot supply executable geometry. |
| 21 | `progression_route.md` | level thresholds `level`; portfolios `(act, channel)`; unit EXP `(act, source_kind)` | Finite `10000*L*L` L1..59 + named act/channel/unit-frequency/bypass tables; total 702100000. |
| 22 | `balance_validation.md` | fixtures/gates `gate_id`; inputs `(gate_id, class_id, tier)` | Named validation algorithms and typed benchmark/reference/rotation/roll-sensitive fixture parameters. No runtime entities. |
| 23 | `integration_validation.md` | checks `rule_id` | Named check registry and compiled owning parameters; no invented table PK and no runtime entities. |
| 24 | `README.md` | manifest `catalog_file`; required status | This exact finite manifest and status contract, not a prose scraper; budget narrative cross-checks owning typed counts. |

Per-section source registries live in the same owning files and are normative extensions of §1, not independent catalogs. Unmapped old operative prose has no fallback interpretation. The last three validation/index files are required compile inputs but do not create gameplay records. The registered competitive-space sections of `../03_systems/pvp.md` and `../03_systems/guild_war.md` are likewise required compile inputs — spec-section sources under §1, not additional catalogs. `presentation_asset_manifest.md` is a separate client asset manifest; scene geometry is a registered build artifact under `physics_geometry_contract.md`, not a 25th gameplay catalog.

## 4. Quy tắc Mở rộng Hữu hạn (Finite Expansions)

Compiler chịu trách nhiệm mở rộng tự động các bảng dẫn xuất (derived tables) một cách tất định:
1. **Equipment Expansion (168 trang bị):** Mở rộng từ 12 bộ trang bị cơ sở × 14 vị trí trang bị với chỉ số chính, chỉ số phụ và thuộc tính Ngũ Hành theo công thức trong `equipment_catalog.md`.
2. **Portal Graph Expansion (52 cổng chuyển vùng):** Mở rộng thành đồ thị 2 chiều (bản đồ nguồn, tọa độ lối vào, bản đồ đích, tọa độ xuất hiện) từ `world_route_catalog.md`.
3. **Persistent Spawn Groups Expansion (54 nhóm quái):** Mở rộng thành các điểm spawn thực thể với bán kính leash, số lượng quái và thời gian hồi sinh từ `map_spawn_catalog.md`.
4. **Entity Size Resolution:** Resolve toàn bộ monster/boss thành đúng một `size_profile` từ `monster_catalog.md` / `boss_catalog.md`; không suy ra từ sprite, tên hoặc Transform scale.
5. **Playable Space Geometry Index:** Resolve `space_id`, `space_kind`, exact bounds, `layout_profile`, scene key và geometry export cho 24 world maps, 5 dungeons, finale, duel, arena và Guild War (registered sources: `world_route_catalog.md`, `dungeon_catalog.md`, `../03_systems/pvp.md`, `../03_systems/guild_war.md`). `1280x720` chỉ là viewport; normal-world width phải là `2.0..5.0` screens.
6. **Skill Geometry Resolution:** Resolve exactly 45 primary action geometries (20 basics + 25 actives), every explicit `secondary_geometries[]` row, role-band/envelope checks, and tag/effect parity from `class_skill_catalog.md` under ADR-0047. Prose, animation, sprite bounds, or client distance cannot create geometry.

## 5. Semantic Content Revision and Canonical Ordering

`content_revision` is exactly **64 lowercase hexadecimal SHA-256 characters**, matching `^[0-9a-f]{64}$`, with no `sha256:` prefix, numeric surrogate or truncation. PostgreSQL provenance uses `CHAR(64)` with that constraint; protobuf and geometry/metadata JSON use the same string. It is not `asset_catalog_revision`.

Hash the **complete compiled semantic payload**, after defaults, heading inheritance, references, typed rules and finite expansion resolve, before activation. Hashing only Markdown table cells or raw file bytes is forbidden. Every registered emitted definition field participates, including presentation-safe strings such as item `display` and `identity_note`; being non-identity/noncombat data is not a hash exclusion.

Canonical payload is an RFC 8259 JSON object with `authoring_schema_version`, `content_schema_version`, `rule_versions`, `definitions`, `validation_parameters`, and `geometry`. Serialize UTF-8 without BOM/whitespace and one trailing LF. Object member keys are lexicographically sorted by UTF-8 bytes; string escaping uses `\"`, `\\`, `\b`, `\f`, `\n`, `\r`, `\t`, otherwise U+0000..001F as lowercase `\u00xx`; no escaping of `/` or other Unicode. Integer values use minimal decimal (`0`, never `-0`). Exact decimals/ratios serialize as `{"denominator":d,"numerator":n}` with reduced positive `d`, so `0.20` and `0.2` are identical. Null is explicit.

Definitions are grouped by stable family and sorted by their declared composite key (typed integer components numerically; IDs by UTF-8 bytes). Sets sort their canonical elements; ordered lists retain explicit ordinals. Effects, stages, phases, weighted groups and slot layouts never derive semantic order from Markdown row placement. Expressions serialize their typed syntax tree, with field names/operators/arguments, not source whitespace; finite-rule names/versions and resolved values participate. Geometry includes quantized bounds/segments/anchors/checkpoint inputs, excluding its own revision field to avoid a recursive hash. `created_at`, filesystem paths/line numbers, diagnostics, comments, Markdown decoration, explanatory prose and presentation ordering are excluded. Registered localization/presentation-safe definition fields remain semantic NFC string fields; changing a normalized value changes the canonical payload and bundle revision. Item `display` and nullable `identity_note` are included, with null explicit; display text remains distinct from stable ID. Canonically equivalent string spellings and nonsemantic Markdown reordering do not change the revision.

`content_revision = lowercase_hex(SHA256(canonical_payload_bytes))`. Both full output and client-safe subset carry this same full-bundle revision; hashing a subset does not redefine it. Compiler emits the canonical payload and a field-source coverage report with the candidate. A source schema/rule algorithm change requires its registered version change; consumers do not silently reinterpret an existing revision.
Keep the exact canonical payload/compiled snapshot for every revision pinned by queued/in-flight/journal recovery or unsettled defeated-copy reward slots (`config.md`). Changing active/previous pointers cannot remove a pinned revision or reinterpret its stored definitions; release follows terminal recovery disposition, not revision age.

Required mutation specification: change actual item charm bp, registered item `display` or `identity_note`, monster/spawn respawn, Soul coefficient/ICD, beast passive endpoint, shop price or inherited binding and verify the corresponding compiled field **and** revision change. Soul `element` must be the explicit roster enum: `ma_rung` remains Soul `MOC` even when its source combat element is `NONE`; a missing/NONE element or invalid rank-by-element distribution rejects. Every Active Payloads constructor must resolve, including `BARRIER(primary_geometry)` paired only with `BARRIER_POSITION`; wrong constructor arguments/type or nonpositive wall lifetime rejects. Reverse independent table rows, reorder sibling item headings, vary fence/table whitespace/backtick decoration and comments, or substitute NFC-equivalent strings: typed output/revision remain identical. Ordered stage/slot changes must change output/revision. Any failed candidate keeps the old active revision. IMP-003/004 must execute these against real owning source, not copied miniature rows alone.

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
[SOURCE_SCHEMA_MISSING]      -- Missing registered section/header/field/default
[UNREGISTERED_SOURCE]        -- Data appears in a runtime scope without a source binding
[AMBIGUOUS_SOURCE]           -- Duplicate explicit field or conflicting source binding
[TYPE_PARSE_ERROR]           -- Token does not satisfy its declared typed grammar
[FINITE_RULE_ERROR]          -- Unknown rule, invalid/nonfinite domain, zero match or overflow
[CANONICAL_HASH_ERROR]       -- Canonical serialization/64hex contract mismatch
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
nguồn duy nhất = owning Markdown registered headings/fences/tables + finite typed rules
hash = full canonical semantic output; table/prose presentation order is not semantic
bất kỳ lỗi nào cũng reject toàn bộ candidate revision
invalid candidate = previous valid revision stays active
```

## Requirement IDs

| ID | Requirement | Gate |
|---|---|---|
| `CAT-004` | every data-owning catalog declares its `Compiler Source Schema` registry (§1); compiler coverage report resolves through the registered sections | compiler compile + activation (IMP-003, IMP-004) |
| `CAT-006` | the competitive-space sections of `../03_systems/pvp.md` and `../03_systems/guild_war.md` declare `Compiler Source Schema` registries (§1) emitting `space_geometry` (`space_kind` `PVP`/`GUILD_WAR`) and their declared logical anchors; the compiled space index covers all 33 playable spaces | compiler compile + geometry parity (IMP-003, IMP-062) |
