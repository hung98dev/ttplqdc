# ADR-0044: Launch Topology — One Process Hosting Edge, Sim, Durable and Global
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0052)**: §3 "additional independent world processes" is superseded. Launch has exactly one world: one process, one PostgreSQL database, a `WORLD_CCU_CAP` login queue, and maintenance-restart deploys.

## Context

`AGENTS.md` đặt ra quy tắc bất biến cốt lõi cho kiến trúc máy chủ Thỉnh Thần:
- `one process = Edge + Sim + Durable + Global`
- `Global = in-process single-writer; not Redis; not PostgreSQL for ordinary parties`
- `channel cap = 18 (ADR-0035); map = 540`
- Cấm: Redis, Kafka, NATS, microservices, extra server binaries.

Tuy nhiên, một số tài liệu mô tả các role phân tán dẫn đến nguy cơ hiểu nhầm và tự sáng tác cơ chế phân tán (như database leader lease hay chia tách các pool process riêng rẽ). Cần một quyết định kiến trúc chính thức khóa chặt mô hình một tiến trình duy nhất cho toàn bộ hệ thống.

## Decision

Khóa mô hình kiến trúc máy chủ chính thức cho ngày phát hành (Launch Topology):

### 1. Một Tiến trình Duy Nhất (`One Process Architecture`)
Toàn bộ mã nguồn máy chủ nằm trong Go module `thinhthan` và chỉ biên dịch ra đúng **một binary thực thi duy nhất**:
`server/cmd/server/` -> executable binary: `thinhthan-server`.

Khi chạy, mỗi thực thể máy chủ chạy đúng **MỘT tiến trình duy nhất** chứa toàn bộ 4 subsystem:
```text
thinhthan-server (Single Process)
├── Edge Subsystem     (WSS Listener, TLS Termination, Session Auth, Packet Routing)
├── Sim Subsystem      (World & Instance 20 Hz Authoritative Simulation Loops)
├── Durable Subsystem  (PostgreSQL pgx Connection Pool, Transactions, Save Rules)
└── Global Subsystem   (In-Process Single-Writer: Party, Chat Fanout, Matchmaking, Spirit Surge)
```

### 2. Ephemeral Global là In-Process Single-Writer Tuyệt Đối
- Subsystem Global chạy trực tiếp trên một goroutine single-writer bên trong tiến trình của thế giới đó.
- **Không dùng Redis, không dùng Kafka, không dùng database leader lease:**
  - Không có bảng `global_leader_lease` trong cơ sở dữ liệu.
  - Tổ đội thông thường (ordinary world party) lưu trữ hoàn toàn trong RAM của tiến trình và tự động giải tán khi tiến trình khởi động lại (`party.md`).
  - Chat fanout thế giới, bang hội, tổ đội và thì thầm được broadcast in-process tới các session client đang kết nối.
  - Hàng đợi ghép trận PvP/Guild War nằm trong bộ nhớ của tiến trình.

### 3. Đơn vị Mở rộng (Scaling Unit)
- Đơn vị mở rộng quy mô là **từng tiến trình thế giới (World Process)**:
  - Một tiến trình phục vụ một Logical World (sức chứa tối đa 540 người chơi đồng thời trên 30 kênh của mỗi bản đồ theo ADR-0020 và ADR-0035).
  - Khi cần phục vụ nhiều người chơi hơn quy mô 1 thế giới, mở thêm các Thế giới độc lập (World 1, World 2, Shard A, Shard B), mỗi thế giới là một tiến trình `thinhthan-server` độc lập kết nối vào cơ sở dữ liệu PostgreSQL của nó.
  - Tuyệt đối không xé nhỏ 1 thế giới thành nhiều microservices.

### 4. Chuyển vùng & Khởi động lại
- **Chuyển vùng cục bộ (Intra-World Transfer):** Di chuyển giữa các bản đồ/kênh trong cùng một thế giới diễn ra tức thì thông qua bộ nhớ trong của tiến trình, với ngân sách chuẩn bị presentation `TRANSFER_BUDGET_WORLD = 30s` (hoặc `120s` cho phó bản).
- **Khởi động lại (Process Restart):** Khi tiến trình dừng hoặc khởi động lại, trạng thái nhân vật khôi phục từ checkpoint đã commit trong PostgreSQL theo quy tắc `save_rules.md`.

## Consequences

- **Specs được đồng bộ:**
  - `AGENTS.md`: Tái khẳng định quy tắc `one process = Edge + Sim + Durable + Global`.
  - `docs/10_implementation/architecture_conformance.md`: Khóa ranh giới layer trong 1 tiến trình.
  - `docs/04_architecture/backend.md` & `service_boundaries.md`: Bỏ cơ chế role tách rời và database lease; xác nhận Global là in-process single-writer.
  - `docs/06_data/data_model.md`: Loại bỏ bảng `global_leader_lease`.
  - `docs/10_implementation/repository_layout.md`: `server/cmd/server/` là điểm entry duy nhất.
- **Điều cấm bất biến:**
  - Cấm chia tách Edge/Sim/Durable/Global thành các tiến trình hoặc microservices riêng biệt.
  - Cấm dùng Redis, Kafka, NATS, hay distributed leader leases.
