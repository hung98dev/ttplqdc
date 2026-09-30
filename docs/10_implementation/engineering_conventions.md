# Engineering Conventions & Code Standards
status: LOCKED

## Scope

Quy chuẩn kỹ thuật bắt buộc cho toàn bộ mã nguồn Go, C# (Unity), Protobuf và SQL trong repository Thỉnh Thần.

Mục tiêu: Đảm bảo mọi AI agent khi sinh code đều tuân thủ cùng một phong cách, kiến trúc, xử lý lỗi, logging, thời gian, RNG và quản lý bộ nhớ, không tạo ra các pattern phân kỳ.

## 1. Go Backend Conventions

### 1.1 Toolchain & Formatting
- **Go Toolchain:** `1.27.1` (pin tại `docs/00_context/technology_versions.md`).
- **Formatting:** `gofmt` tiêu chuẩn với tabs cho thụt lề; cấm dùng linter tự ý reformat khác chuẩn `gofmt`.
- **Static checks (`CODE-003`, Q4):** `gofmt -l` prints nothing; `go vet ./...` is clean; `staticcheck ./...` (pinned `honnef.co/go/tools v0.8.1`, default check set, no `staticcheck.conf`) is clean. A `//lint:ignore <check> <reason>` needs a non-empty reason and reviewer approval; `//lint:file-ignore` is forbidden.
- **Package Layout:** Nằm trong `server/internal/<domain>/`. Tên package ngắn gọn, chữ thường, không gạch dưới, không camelCase.

### 1.2 Structured Logging
- **Thư viện chuẩn:** `log/slog` từ standard library. Tuyệt đối cấm thêm `zap`, `logrus`, hay `zerolog`.
- **Định dạng:** JSON format cho production (`slog.NewJSONHandler`), Text format cho local development.
- **Thuộc tính bắt buộc (Attributes):**
  - Mọi log ghi nhận nghiệp vụ/lỗi phải có: `op` (operation name), `revision` (content revision), `actor_id` / `session_id` (nếu trong context session).
  - Không log dữ liệu cá nhân nhạy cảm (plaintext passwords, raw payment tokens).

### 1.3 Context, Time & RNG Injection
- **Context:** `ctx context.Context` luôn là tham số đầu tiên của mọi hàm I/O, database query hoặc network call. Tôn trọng deadline và `ctx.Done()`.
- **Time Authority:**
  - Server là nguồn chân lý thời gian tuyệt đối. Luôn dùng `time.Now().UTC()`.
  - Không tin cậy và không dùng timestamp do client gửi lên cho bất kỳ logic tính toán hay lưu trữ nào.
- **RNG Injection:**
  - Gameplay RNG (combat rolls, drop tables, crits, status procs): Phải dùng `math/rand/v2` PCG-64 được inject qua interface từ `server/internal/core/rng/`. Tuyệt đối cấm import `math/rand` (v1).
  - Security / Cryptography / UUIDs: Sử dụng `crypto/rand`.

### 1.4 Concurrency & Simulation Single-Writer
- Mỗi map channel / instance simulation loop là một single-writer duy nhất chạy trên một goroutine chuyên trách.
- Không chia sẻ bộ nhớ mutable giữa các goroutines mà không thông qua channel hoặc atomic state snapshot.
- Ephemeral Global (party, chat fanout, matchmaking): Single-writer in-process; không dùng Redis hay message broker ngoài.

### 1.5 Error Handling & Wire Error Mapping
- Lỗi nghiệp vụ nội bộ dùng domain error types hoặc sentinel errors (`errors.Is`, `errors.As`).
- Khi phản hồi cho client qua network, lỗi phải được ánh xạ 1-1 với mã lỗi trong `docs/05_network/errors.md`.
- Cấm panic trong code xử lý request hoặc simulation tick. `panic` chỉ được phép ở giai đoạn khởi động (startup assertion) khi thiếu config cốt lõi không thể phục hồi.

### 1.6 Database & SQL (pgx/v5)
- **Truy cập:** Sử dụng `github.com/jackc/pgx/v5` với native pgxpool. Cấm dùng ORM (GORM, ent) hoặc abstraction layer không được duyệt.
- **Parameterized SQL:** 100% câu lệnh SQL phải dùng tham số (`$1`, `$2`), tuyệt đối cấm nối chuỗi câu lệnh SQL.
- **Giao dịch (Transactions):** Các thao tác cập nhật số dư, inventory, reward claims phải nằm trong transaction với lock ordering nhất quán (e.g. sort theo ID trước khi SELECT FOR UPDATE) để ngăn deadlock.
- **Migrations:**
  - Sử dụng `golang-migrate/migrate/v4 v4.20.1`.
  - Planned repository path is `server/migrations/` with `000001_baseline_schema.up.sql` / `000001_baseline_schema.down.sql`; `IMP-005` owns materializing it.
  - Không sửa file migration cũ đã merge; mọi sửa đổi schema phải là migration mới tăng dần.

### 1.7 Hot-Path Allocation & Benchmarks
- Allocation budgets are canonical in `../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets and are enforced by `TestAllocs_<Name>` tests using `testing.AllocsPerRun` (build tag `!race`, run in the non-race Q3 pass). The budget number is exact.
- Every budgeted path also has `Benchmark<Name>`. Q3 runs `go test -run '^$' -bench . -benchmem -benchtime=200x -count=1` for the packages listed there and records ns/op, B/op and allocs/op in `verify-report.json`. ns/op is informational only and never gated on hosted runners.
- Steady-state hot paths reuse buffers owned by the caller (`MarshalAppend(buf[:0], m)`, slices reset with `[:0]`, per-actor scratch structs). They never use `fmt`, `reflect`, maps built per tick, closures capturing per-tick state, or `interface{}` boxing.

## 2. C# / Unity Client Conventions

### 2.1 Language & Style
- **Language Level:** C# 9.0 (.NET Standard 2.1) tương thích Unity 6000.6.1f1.
- **Style:** Allman brace style (dấu `{` ở dòng riêng), 4 spaces cho thụt lề, PascalCase cho Types và Methods, camelCase cho biến cục bộ và parameters, `_camelCase` cho private fields.

### 2.2 Assembly Definitions (.asmdef)
Mọi code C# phải nằm dưới các assembly definition được cô lập rõ ràng. Danh sách đầy đủ 13 assembly và reference graph chính xác là canonical tại `repository_layout.md` § Mandatory Assemblies (IMP-000 tạo toàn bộ, ADR-0068); các mục dưới chỉ mô tả trách nhiệm:
- `ThinhThan.Protocol.asmdef`: Chỉ chứa code sinh tự động từ protobuf; không tham chiếu tới bất kỳ Unity assembly nào khác.
- `ThinhThan.Core.asmdef`: Chứa math, UUID utilities, text normalization, pure domain models, and the frame runtime (`Core/Runtime/`: `FrameLoop`, `FrameTime`, `FrameBudget`, `Pool<T>`, `Log`, `PresentationRandom`).
- `ThinhThan.Net.asmdef`: WSS client (`System.Net.WebSockets.ClientWebSocket`, background receive task), session state machine, serialization handling.
- `ThinhThan.Systems.asmdef`: Gameplay presentation, movement interpolation, combat controllers.
- `ThinhThan.UI.asmdef`: HUD, menu screens, input overlays.
- `ThinhThan.App.asmdef`: composition root (IMP-067); creates `FrameLoop` and every service; only `ThinhThan.Tests.PlayMode` references it.
- `ThinhThan.Tests.EditMode.asmdef` và `ThinhThan.Tests.PlayMode.asmdef`: Thư mục test riêng.
Cấm circular dependencies giữa các asmdef.

### 2.3 Frame Model & Memory Hygiene
- Runtime architecture (FrameLoop phases, `FrameTime`, `FrameBudget`, pre-warm, governor, UI, network receive path) is canonical in `../04_architecture/client_performance.md` § Smoothness by Construction. This section only lists the coding rules.
- Frame code (every `IFrameSystem.Tick`, network apply, UI phase) allocates 0 bytes: no LINQ, boxing, string concatenation/interpolation, closures capturing locals, `params` arrays, `foreach` over interfaces, or collection growth. Collections are pre-sized at load.
- Transient visuals (projectiles, floating text, VFX, UI rows, actor views) come from `Pool<T>` and are pre-sized at map load.
- Component references are cached at creation; `GetComponent` never runs per frame.

### 2.4 Input & UI State Machine
- Sử dụng Unity Input System (`com.unity.inputsystem 1.20.0`).
- Gửi cạnh di chuyển qua `C2S_MOVEMENT_EDGE` (ID 108, `DISCRETE_INTENT`) với wire enum `PRESS | RELEASE | FLIP`; không phát minh `STOP`. `client_mono_ms` chỉ advisory và server clamp bù trễ tối đa 80 ms theo `../05_network/messages.md`.
- UI điều khiển qua finite state machine, không gọi trực tiếp network socket từ view UI.

### 2.5 Client API Fence (`CODE-005`, `PERF-020`, Q4)
Applies to first-party runtime assemblies `ThinhThan.Core/Net/Systems/UI/App`, excluding `Editor/` folders, tests and generated `Protocol/`. Exceptions are exact `path:symbol` entries in `server/internal/conformance/architecture/client_api_allowlist.txt` (IMP-083, protected), each with a reason.

```text
forbidden                                              use instead
Update / FixedUpdate / LateUpdate / OnGUI              IFrameSystem.Tick via FrameLoop (FrameLoop itself allowlisted)
GameObject.Find*, FindObjectOfType, FindObjectsOfType,
  FindFirstObjectByType, FindAnyObjectByType           constructor/composition injection, cached references
SendMessage, BroadcastMessage, Invoke, InvokeRepeating direct calls, C# events, FrameBudget tasks
StartCoroutine, IEnumerator coroutines                 FrameBudget tasks or Unity Awaitable
async void                                             async Awaitable / async Task (Net only) with owner + cancellation
Task, Task.Run, Thread, ThreadPool outside ThinhThan.Net   Awaitable; Net background receive task only
Resources.Load*, *.WaitForCompletion() outside loading screens   Addressables async + FrameBudget
Camera.main outside the camera service                 injected camera service
System.Linq                                            explicit loops over pre-sized collections
Debug.Log*                                             Log facade (Core/Runtime)
.material getter, new Material(                        sharedMaterial, SpriteRenderer.color
GC.Collect outside loading screens                     incremental GC
UnityEngine.Random, System.Random                      Core PresentationRandom (seeded, presentation only)
UnityEvent fields in first-party types                 C# events with paired subscribe/unsubscribe
static mutable fields outside ThinhThan.App            instances owned by the composition root
```

### 2.6 Canonical Implementations (`CODE-006`, Q4)
Each concern has one implementation. A second type whose name or base type matches the concern's pattern outside the owner path fails Q4. Otherwise the reviewer checks it.

```text
concern             client owner (C#)                              server owner (Go)
frame driver        Core/Runtime FrameLoop, FrameTime              sim/runtime tick (IMP-079)
work scheduling     Core/Runtime FrameBudget                       -
pooling             Core/Runtime Pool<T> (no UnityEngine.Pool)     caller-owned buffers (§1.7)
logging             Core/Runtime Log (Log.Dev* [Conditional("THINHTHAN_DEV")];
                    Warn/Error always compiled, rate-limited)      observability/core (IMP-098)
time                FrameTime / injected IClock                    injected clock (§1.3)
randomness          Core/Runtime PresentationRandom (presentation)  core/rng (§1.3)
events              C# events / typed message bus in Core/Runtime  typed ports (architecture_conformance.md §3)
errors/results      Result<T, ErrorCode> (Core); wire codes errors.md   sentinel errors (§1.5)
services            constructor injection from ThinhThan.App       constructor injection from app (IMP-069)
```

### 2.7 Compiler, Style & Line Endings (`CODE-001`, `CODE-002`, `CODE-004`)
- `client/Assets/csc.rsp` contains exactly `-warnaserror+` and `-nullable:enable`. It applies to every assembly under `Assets/`, tests included. Nullable annotations are mandatory; `!` (null-forgiving) needs an adjacent comment that names the invariant.
- Generated C# (`client/Assets/Scripts/Protocol/`) starts with `#nullable disable` and `#pragma warning disable` for the protobuf-generated warning set; `scripts/codegen.ps1` prepends the header deterministically (Q2 byte-identical).
- Root `.editorconfig` is canonical for formatting:
  - C#: 4 spaces, Allman, `_camelCase` private instance fields, PascalCase types/methods/properties/constants, camelCase locals/parameters, block-scoped namespaces (C# 9).
  - Go: tabs. Proto: 2 spaces.
  - All files: UTF-8 without BOM, LF, final newline, no trailing whitespace.
- Root `.gitattributes`: `* text=auto eol=lf` (including `*.ps1`, `*.sh`, Unity YAML `*.unity *.prefab *.asset *.meta *.mat *.anim *.controller`); binary media per `repository_layout.md` LFS rules.
- The Go verifier (`server/internal/conformance/style/`) checks the C# style deterministically without a .NET SDK or Roslyn. It checks every first-party `.cs` file:
  - a line ending in `{` contains only `{`, and a line starting with `}` contains only `}` optionally followed by `;`, `,` or `)`;
  - indentation is a multiple of 4 spaces, with no tabs;
  - there is no trailing whitespace; the file uses LF, has no BOM and ends with a final newline;
  - private instance fields are named `_camelCase`;
  - there is one top-level type per file, and the file name equals the type name;
  - the namespace equals `ThinhThan.<Assembly>` plus the folder path below the assembly root.

## 3. Protocol Buffers Wire Conventions

- **Syntax:** `proto3`. Thụt lề 2 spaces.
- **Package:** `package thinhthan.v1;`.
- **Options:**
  - `option go_package = "thinhthan/internal/protocol/v1;protocolv1";`
  - `option csharp_namespace = "ThinhThan.Protocol.V1";`
- **Tên field:** snake_case cho field names, PascalCase cho message và enum names, SCREAMING_SNAKE_CASE cho enum values (với tiền tố enum name).
- **Generated Code:** Cấm sửa tay code sinh ra trong `server/internal/protocol/v1/` và `client/Assets/Scripts/Protocol/`. Mọi thay đổi phải sinh qua `scripts/codegen.ps1`; Go parity tests nằm ngoài generated-only tree tại `server/internal/testing/protocol/`.

## 4. Testing & Verification Conventions

- **Go Tests:** File đặt cạnh code nguồn `*_test.go`. Tên test: `Test<Feature>_<Scenario>`. Bắt buộc test cả happy path lẫn error/boundary conditions.
- **Unity Tests:** EditMode tests cho pure logic / data validation; PlayMode tests cho WSS session flow và component lifecycle.
- **Fixtures:** Fixture dữ liệu phải đặt trong thư mục `testdata/`, nội dung bất biến, có thể tái lập 100%.
- **Zero Flakiness:** Mọi test ngẫu nhiên phải dùng fixed seed được kiểm soát; không dùng thời gian thực để so sánh nếu không có tolerance hợp lý.

## 4a. Branch, Commit and PR Conventions

```text
branches   imp/IMP-XXX-<slug>   imp/IMP-XXX-done   spec/BLK-XXX-<slug>   claim/<yyyymmdd>-<n>
           block/IMP-XXX-<n>   ops/OPS-XXX-<open|resolved>   revert/<sha>   (roles: audit_gates.md § Protected Paths)
titles     <type>(IMP-XXX): <summary>      type = feat | fix | test | chore | docs | ci | perf
PR body    Change Packet (.github/pull_request_template.md, agent_execution_protocol.md §4a)
updates    git merge origin/main only, and only while holding the merge slot; never rebase, amend, force-push or push to main
merge      squash via auto-merge enabled by the merge-slot holder after the §5a sequence (ADR-0072)
```

## 5. Clean Cutover & Anti-Drift Rule

- Khi thay đổi một API hoặc contract:
  - Migrate toàn bộ caller trong cùng một commit.
  - Xóa bỏ hoàn toàn code cũ, không để lại alias, shim, deprecated stub hay commented code.
  - Đảm bảo git status hoàn toàn sạch sau khi chạy codegen và test verify.

## Requirement IDs
Covered by Q0 requirement coverage like spec tables (`audit_gates.md` Gate B).

| ID | Requirement | Gate |
|---|---|---|
| `CODE-001` | `client/Assets/csc.rsp` = `-warnaserror+ -nullable:enable`; every first-party assembly compiles with 0 warnings (§2.7) | every PR (Q3 Unity compile, Q4) |
| `CODE-002` | `.editorconfig` + `.gitattributes` present with the §2.7 keys; C# style check passes on every first-party `.cs` (§2.7) | every PR (Q4) |
| `CODE-003` | `gofmt -l` empty, `go vet ./...` and pinned `staticcheck ./...` clean; no `//lint:file-ignore` (§1.1) | every PR (Q4) |
| `CODE-004` | generated C# begins with the `#nullable disable` + pragma header, byte-deterministic (§2.7) | every PR (Q2) |
| `CODE-005` | client API fence with justified allowlist entries only (§2.5) | every PR (Q4) |
| `CODE-006` | one canonical implementation per concern; duplicates detected by name/base-type patterns (§2.6) | every PR (Q4) |

## Invariants

```text
Go toolchain = 1.27.1; Unity = 6000.6.1f1 (C# 9.0 Allman)
slog cho Go logging; pgx/v5 raw SQL cho database
math/rand/v2 cho gameplay RNG; crypto/rand cho security/UUID
proto/ là wire SoT; không sửa tay generated code
zero commented-out code, zero fake stubs, zero unapproved packages
C# warnings are errors; nullable enabled; LF everywhere; style checked by the verifier
gofmt + go vet + staticcheck clean; hot-path allocs/op are exact gates, ns/op is report-only
one FrameLoop, one FrameBudget, one Pool<T>, one Log facade on the client
```
