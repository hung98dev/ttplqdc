# Architecture Conformance Contract
status: LOCKED

## Scope

Hợp đồng kiểm soát kiến trúc, hướng phụ thuộc giữa các tầng (layer dependency direction), quyền sở hữu giao dịch cơ sở dữ liệu (SQL ownership), ranh giới mã sinh tự động (generated code boundaries) và cơ chế tự động hóa kiểm tra tính tuân thủ cho repository Thỉnh Thần.

## 1. Hợp đồng Topology Thống nhất (ADR-0044 Alignment)

Để đồng bộ giữa `AGENTS.md` ("one process = Edge + Sim + Durable + Global") và các tài liệu triển khai sản xuất (`backend.md`, `deployment.md`):

1. **Một module & Một Production Binary duy nhất:**
   - Toàn bộ backend nằm trong module `thinhthan`; production chỉ deploy `thinhthan-server` từ `server/cmd/server/`. `IMP-006` tạo entry point tối thiểu và `IMP-069` là owner cuối cùng của composition/lifecycle wiring.
   - `cmd/compiler`, `cmd/migrate`, và `cmd/verify` là build/operator tools, không phải server processes hoặc independently deployed services.
   - `server/` hiện là planned path và chỉ được materialize theo `repository_layout.md`.
   - Tuyệt đối cấm tạo repository/module thứ hai hoặc tách microservices độc lập.
2. **Một Tiến trình Duy nhất Chứa Đủ 4 Subsystems:**
   - Quy tắc bất biến từ `AGENTS.md` và `ADR-0044`: `one process = Edge + Sim + Durable + Global`.
   - Mỗi server instance vận hành như một tiến trình tự chứa (self-contained process).
3. **Ephemeral Global là In-Process Single-Writer:**
   - Global subsystem (Party, Chat Fanout, Matchmaking, Spirit Surge) chạy in-process như một single-writer trong cùng tiến trình máy chủ.
   - Cấm dùng Redis, Kafka, NATS, microservices, hay distributed leader lease trong PostgreSQL.
   - Chỉ có một thế giới, một tiến trình (ADR-0052); quá tải được xử lý bằng hàng đợi đăng nhập `WORLD_CCU_CAP`, không xé nhỏ một thế giới thành nhiều tiến trình phân tán.
## 2. Ma trận Phụ thuộc giữa các Tầng (Allowed Dependency Matrix)

Mọi package trong `server/internal/` phải tuân thủ nghiêm ngặt ma trận phụ thuộc một chiều:

```text
       +---------------------------------------------+
       |             Edge / Session                  |
       +---------------------------------------------+
              |                               |
              v                               v
       +---------------+              +---------------+
       |  Sim (World)  |              |    Global     |
       +---------------+              +---------------+
              |                               |
              +--------------+                |
                             v                v
                      +-------------------------------+
                      |        Durable Domain         |
                      +-------------------------------+
                                      |
                                      v
                      +-------------------------------+
                      |     PostgreSQL Database       |
                      +-------------------------------+
```

### Quy tắc Chi tiết:
1. **Edge / Session (`server/internal/edge`):**
   - Được phép gọi sang `sim` để chuyển tiếp input và `durable` để xác thực phiên.
   - CẤM: Không tự thực thi logic chiến đấu, không tự tính toán vị trí, không trực tiếp chạy câu lệnh SQL cập nhật tài sản.
2. **Simulation (`server/internal/sim`):**
   - Vận hành tick loop cố định 20 Hz trong bộ nhớ RAM.
   - CẤM: Không import `database/sql`, không import `github.com/jackc/pgx`, không import `server/migrations`.
   - Mọi nhu cầu lưu trữ (boss death consequence, nhặt đồ, trừ tiền) phải đóng gói thành lệnh có định kiểu gửi sang `server/internal/durable`.
3. **Durable Domain (`server/internal/durable`):**
   - Nơi duy nhất được phép kết nối và thực thi SQL tới PostgreSQL.
   - CẤM: Không import logic thời gian thực của `sim/combat` hay `sim/movement`.
4. **Wire Protocol (`server/internal/protocol/v1/`):**
   - Mã sinh tự động từ `proto/thinhthan/v1/`.
   - CẤM: Không được sửa tay bất kỳ dòng code nào. Mọi package khác chỉ import để đọc/ghi protobuf message.

## 3. Quy tắc Độc tôn Sở hữu (Single-Owner Invariants)

Để tránh việc các agent viết code trùng lặp hoặc tạo ra hai hệ thống cạnh tranh nhau trong cùng một repo:
1. **Một Engine Chiến Đấu duy nhất:** Mọi công thức sát thương, Just Guard, Status Effects, Shields chỉ nằm tại `server/internal/sim/combat/`. Cấm tạo helper combat cục bộ ở package khác.
2. **Một Primitive Sở hữu Vật phẩm duy nhất:** Toàn bộ việc tạo item instance, gán vị trí (`item_locations`), binding rules thuộc quyền sở hữu duy nhất của `server/internal/durable/items/`.
3. **Một Primitive Tiền tệ duy nhất:** Trừ, cộng, cap, kiểm toán 3 loại tiền (`common`, `bound`, `special`) thuộc quyền sở hữu duy nhất của `server/internal/durable/currency/`.
4. **Một Primitive Nhận Thưởng duy nhất:** Cơ chế nhận thưởng, chống tràn kho thuộc quyền sở hữu duy nhất của `server/internal/durable/reward/`.
5. **Một Giao diện RNG duy nhất:** Mọi tung xúc xắc gameplay phải gọi `server/internal/core/rng/` (Go `math/rand/v2` PCG-64).
6. **Một runtime cho mỗi subsystem:** tick loop/AOI/replication chỉ ở `server/internal/sim/runtime|aoi|replication/` (IMP-079); collision chỉ ở `server/internal/sim/spatial/collision/` (IMP-078); Global single writer chỉ ở `server/internal/global/runtime/` (IMP-080); WSS listener/heartbeat chỉ ở `server/internal/edge/listener|heartbeat/` (IMP-081); hàng đợi lệnh durable chỉ ở `server/internal/durable/queue/` (IMP-082); thứ tự khóa aggregate chỉ qua `server/internal/durable/lockorder/` (IMP-097); logging/metrics/correlation chỉ qua `server/internal/observability/core/` (IMP-098).
7. **Một owner migration:** chỉ IMP-005 tạo file trong `server/migrations/` (baseline `000001`).
8. **Just Guard và latency model** chỉ ở `server/internal/sim/combat/` (IMP-014); RTT sample đến từ `edge/heartbeat` qua interface định kiểu.
9. **Client frame runtime:** `FrameLoop`, `FrameTime`, `FrameBudget`, `Pool<T>`, `Log`, `PresentationRandom` chỉ ở `client/Assets/Scripts/Core/Runtime/` (IMP-065); camera service chỉ ở `client/Assets/Scripts/Systems/Camera/` (IMP-066); quality governor chỉ ở `client/Assets/Scripts/Core/Performance/` (IMP-095). Full list: `engineering_conventions.md` §2.6 (ADR-0059).

## 4. Tự động hóa Kiểm tra Kiến trúc (Executable Architecture Gates)

IMP-000 materialize verifier (`server/internal/conformance/gates/`); IMP-083 sở hữu Q0 task-graph (`server/internal/conformance/taskgraph/`) và Q4 kiến trúc (`server/internal/conformance/architecture/architecture_test.go`); IMP-068 sở hữu ratchet và trusted CI (`server/internal/conformance/ratchet/`, `trusted/`). Từ module root `server/`, wrapper gọi `go run ./cmd/verify`:
1. **Forbidden Dependencies Gate:** Gin, Chi, Echo, Fiber, Gorilla, Redis, Kafka, NATS, gRPC, GORM, sqlx, zap, logrus, zerolog, legacy `math/rand`.
2. **SQL ownership:** chỉ `durable` (và stackpin/conformance/migrate) được import pgx/`database/sql`.
3. **Sim isolation:** `sim` không import SQL hoặc `edge`.
4. **Durable isolation:** `durable` không import `sim`.
5. **Protocol isolation:** `protocol` không import domain/runtime.
6. **One production main:** chỉ `cmd/server`. Chỉ ba tool main được phép thêm là compiler/verify/migrate; mọi main khác bị từ chối.
7. **Generated source:** `.pb.go` phải reference proto source, không chỉ header DO NOT EDIT.
8. **Schema:** cấm `item_instances.durability` và `global_leader_lease`.
9. **asmdef:** không cycle; IMP-000 sở hữu cả 13 asmdef và reference graph của `repository_layout.md` § Mandatory Assemblies (ADR-0068); mọi asmdef lệch khỏi bảng đó bị từ chối; `ThinhThan.App` là composition root (IMP-067) và chỉ `ThinhThan.Tests.PlayMode` tham chiếu nó; `ThinhThan.Protocol` không tham chiếu assembly nào của dự án.
10. **Observability:** `server/internal/observability/` được mọi runtime package import, nhưng không import `sim`, `edge`, `durable`, `global`.
11. **Test placement:** Go test nằm trong chính package được test; Unity test nằm trong `client/Assets/Tests/{EditMode|PlayMode}/<Feature>/` thuộc owned_paths của packet (`repository_layout.md` § Ownership Rules).
12. **Code quality (IMP-000, `server/internal/conformance/style/`):** C# style, `.editorconfig`/`.gitattributes` keys, `csc.rsp`, `gofmt -l`, `go vet`, pinned `staticcheck` (`engineering_conventions.md` §1.1, §2.7; `CODE-001..003`).
13. **Client API fence (IMP-083):** token-based scan of the first-party runtime C# assemblies against `engineering_conventions.md` §2.5, including the FrameLoop-only Unity callbacks rule (`CODE-005`, `PERF-020`). Exceptions exist only in `server/internal/conformance/architecture/client_api_allowlist.txt` as `path:symbol  reason`, and an entry without a reason fails.
14. **Canonical implementations (IMP-083):** a type matching a concern pattern of `engineering_conventions.md` §2.6 (e.g. `*Pool`, `*FrameLoop*`, `*Scheduler`, `*Logger`, `UnityEngine.Pool.*`, `System.Random`) outside its owner path fails (`CODE-006`).

Các gate này là Q4 trong `audit_gates.md`. Mutation fixtures phải chứng minh từng rule fail closed; việc chỉ mô tả rule bằng Markdown không đạt Gate C.
## Invariants

```text
one process = Edge + Sim + Durable + Global (thinhthan-server)
Global = in-process single-writer; cấm Redis, Kafka, distributed leases
sim layer không bao giờ trực tiếp kết nối SQL
1 combat engine, 1 item primitive, 1 currency primitive, 1 reward primitive
1 sim runtime, 1 Global writer, 1 durable queue, 1 lock-order helper, 1 migration owner (IMP-005)
architecture test tự động hóa kiểm tra AST import fences
client: one FrameLoop / FrameBudget / Pool<T> / Log; forbidden Unity APIs fail Q4
```
