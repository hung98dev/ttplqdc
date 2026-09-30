# Implementation Wave Prompts
status: LOCKED

Operational prompts for executing every `IMP-*` task. Status, dependencies, scope, paths, tests and evidence are canonical in `task_queue.md`; the workflow, claim and merge sequence are canonical in `agent_execution_protocol.md` (§5a). This file never overrides them.

Waves are the longest-path levels of the `depends_on` DAG: wave 0 has no dependency and wave N tasks depend on a task in wave N-1. A task may start as soon as all its own dependencies are `DONE` on `main`; the coordinator recomputes readiness from `main` before every claim instead of waiting for a whole wave.

## How the Owner Runs the Project
1. Once, after Owner Setup: start the **Reviewer** session (§3 below) and the **Spec-owner** session (§4 below); they keep running and react to PRs / blockers by themselves.
2. For each wave in order, open one AI session in the repo and just say **"Làm wave N"** (e.g. "Làm wave 0 đi"; "làm wave tiếp" = next unfinished wave). `AGENTS.md` § Owner commands and the `/run-wave` skill make the agent execute the Wave Prompt below with that N. Wait for its final report, then send the next wave. Nothing else is needed from the owner except resolving `OPS-xxx` issues labelled `ops-blocked`.

### Wave Prompt (copy-paste, replace `<N>`)
```text
Thực hiện Wave <N> của repo thinhthan. Bạn là coordinator cho wave này.
Đọc AGENTS.md, docs/10_implementation/wave_execution_prompts.md, agent_execution_protocol.md (§3–§5b) và audit_gates.md (Bootstrap Mode).
1. Pull main. Nếu AUTO_MERGE_FROZEN=true hoặc có OPS-xxx mở với `blocks: ALL`: báo lại và dừng.
   Với mỗi task của Wave <N>: nếu một depends_on chưa DONE trên main (bị BLOCKED hoặc chưa xong), ghi task đó vào báo cáo là "chờ <dep>" và bỏ qua.
2. Với từng task sẵn sàng: claim theo §3 (IMP-000 tự claim trong PR của nó), rồi giao cho một agent con riêng
   (worktree, branch imp/IMP-XXX-<slug>, DB port và Unity cache riêng) chạy Implementer Prompt bên dưới; agent con sống tới khi PR merge hoặc task BLOCKED.
   Tối đa 5 task song song, trong đó tối đa 2 task có client/ trong owned_paths (ADR-0058, ADR-0072); task còn lại chờ.
   Final-art task khi chưa có art tool trong Owner Setup: không claim, mở OPS có phạm vi qua PR ops/ (§3).
3. Merge slot (§5a): mỗi lúc chỉ một PR mang label `merge-slot`; trao cho PR sẵn sàng (reviewer APPROVE + CI xanh) có chỉ số topo nhỏ nhất
   (PR claim/block/ops trước). Sau khi PR đó merge thì trao tiếp. Không trao slot cho PR khác giữa merge IMP-068 và merge imp/IMP-068-done.
4. Theo dõi tới khi mọi task của wave là DONE trên main (two-phase task: cả PR imp/IMP-XXX-done theo §5a) hoặc BLOCKED.
   Task BLOCKED vì BLK: spec-owner xử lý; khi task trở lại NOT_STARTED thì claim và chạy lại. BLOCKED vì OPS: chờ chủ repo đóng issue ops-blocked, rồi mở PR ops/ trả task về NOT_STARTED.
5. Báo cáo cuối: từng task -> DONE/BLOCKED (mã BLK/OPS)/chờ dep, PR đã merge, số lần CI chạy, việc chủ repo cần làm (nếu có).
Không tự sửa spec, không hỏi con người trong lúc chạy, không push thẳng main, không rebase/force-push.
```

## Session Prompts (copy-paste)

Start one long-running session per role. If the harness has no `/run-imp-task` skill, the agent follows `.devin/skills/run-imp-task/SKILL.md` as a checklist. Role rules: `agent_execution_protocol.md` §1.

### 1. Coordinator (one session, runs the whole pipeline)
```text
Bạn là coordinator của repo thinhthan. Đọc AGENTS.md, docs/10_implementation/agent_execution_protocol.md (§3, §5a, §5b), audit_gates.md và file này.
Lặp liên tục cho tới khi IMP-048 DONE:
1. Pull main. Nếu biến repo AUTO_MERGE_FROZEN=true hoặc có OPS-xxx mở với `blocks: ALL` thì dừng và chờ (ghi OPS cho issue ops-blocked do merge guard mở qua PR ops/).
2. Tính các task sẵn sàng (mọi depends_on DONE trên main, không BLK/OPS mở nêu tên task, tuân Bootstrap Mode trước IMP-068).
3. Claim theo §3 (IMP-000: claim trong chính PR của nó), tối đa 5 task IN_PROGRESS cùng lúc (tối đa 2 task có client/), ưu tiên thứ tự topo nhỏ nhất.
4. Giao mỗi task cho đúng một implementer bằng Implementer Prompt (task, branch, worktree, base SHA).
5. Quản lý merge slot theo §5a (label `merge-slot`, một PR mỗi lúc).
6. Task BLOCKED vì BLK -> giao cho spec-owner; OPS đã được chủ repo đóng issue -> PR ops/ đánh dấu Resolved; claim quá 24h không hoạt động -> trả về NOT_STARTED.
Không tự viết code, không sửa spec, không hỏi con người.
```

### 2. Implementer (one session per task)
```text
Thực hiện <IMP-XXX> trên branch <branch>, worktree <path>, base <sha>.
Chạy /run-imp-task (hoặc làm đúng checklist .devin/skills/run-imp-task/SKILL.md) theo docs/10_implementation/agent_execution_protocol.md §4–§5b.
Chỉ sửa owned_paths và path test/evidence của packet. Không hỏi con người. Không push thẳng main, không workflow_dispatch, không rebase/force-push.
CI báo `commit unity-materialized`: tải artifact unity-materialized-<os>, commit nguyên văn, push (§4b).
Thiếu hoặc mâu thuẫn spec: mở PR block/IMP-XXX-<n> (thêm BLK-xxx, đặt task BLOCKED), chờ nó merge, đóng draft PR và dừng (§6).
Khi PR sẵn sàng (reviewer APPROVE + CI xanh): báo coordinator, chờ label `merge-slot`, rồi làm bước 6–9 của §5a.
Xong khi PR đã merge (two-phase task: cả PR imp/IMP-XXX-done).
```

### 3. Reviewer (one session, separate OS account holding the App key; env THINHTHAN_AGENT_ROLE=reviewer, THINHTHAN_POLICY_APP_ID, THINHTHAN_POLICY_APP_KEY_FILE)
```text
Bạn là reviewer độc lập (.devin/agents/reviewer.md). Với mỗi PR mở hoặc có push mới:
review `git diff origin/main...HEAD` theo checklist của profile (spec-change PR dùng spec checklist; PR claim/block/ops chỉ kiểm field được phép), chạy verify_delta --full,
đăng verdict + danh sách spec đã đối chiếu thành PR review comment, rồi tạo check run `policy-review` cho đúng head SHA bằng
pwsh -NoProfile -File .devin/scripts/policy_review.ps1 -Repo <owner/repo> -Sha <head> -Conclusion success|failure -SummaryFile <file>.
Không sửa code, không tự duyệt PR do chính session này tạo.
```

### 4. Spec-owner (one session, env THINHTHAN_AGENT_ROLE=spec-owner)
```text
Bạn là spec-owner (.devin/agents/spec-owner.md). Xử lý từng BLK-xxx mở trong docs/10_implementation/known_blockers.md:
quyết định phương án tốt nhất, sửa spec/ADR và mọi consumer trong một spec-change PR (branch spec/BLK-xxx-<slug>), gắn requirement ID cho yêu cầu đo được,
thêm regression test vào ## Tests của task liên quan, đóng BLK và trả task về NOT_STARTED. Không viết code triển khai, không hỏi con người.
```

## Waves

| Wave | Tasks | Notes |
|---|---|---|
| 0 | IMP-000 | two-phase: IMP-000; bootstrap-eligible: IMP-000 |
| 1 | IMP-001, IMP-061, IMP-063, IMP-064, IMP-101 | two-phase: IMP-061; bootstrap-eligible: IMP-001, IMP-061, IMP-063, IMP-064, IMP-101 |
| 2 | IMP-002, IMP-005, IMP-070, IMP-083 | two-phase: IMP-005, IMP-083; bootstrap-eligible: IMP-002, IMP-005, IMP-070, IMP-083 |
| 3 | IMP-003, IMP-071, IMP-073, IMP-074, IMP-075, IMP-104 | two-phase: IMP-003; bootstrap-eligible: IMP-003, IMP-071, IMP-073, IMP-074, IMP-075, IMP-104 |
| 4 | IMP-004 | two-phase: IMP-004; bootstrap-eligible: IMP-004 |
| 5 | IMP-050, IMP-068 | two-phase: IMP-068; bootstrap-eligible: IMP-050 |
| 6 | IMP-098 |  |
| 7 | IMP-078, IMP-079, IMP-081, IMP-082, IMP-097 |  |
| 8 | IMP-006, IMP-007, IMP-008, IMP-062, IMP-080 |  |
| 9 | IMP-072, IMP-100, IMP-105 |  |
| 10 | IMP-065, IMP-076 | two-phase: IMP-065 |
| 11 | IMP-013 |  |
| 12 | IMP-066 |  |
| 13 | IMP-009, IMP-011, IMP-018, IMP-095 |  |
| 14 | IMP-010, IMP-012, IMP-014, IMP-020, IMP-029, IMP-030, IMP-034, IMP-035, IMP-059, IMP-099 |  |
| 15 | IMP-015, IMP-026, IMP-036, IMP-054, IMP-058, IMP-060, IMP-094 |  |
| 16 | IMP-016, IMP-027, IMP-037, IMP-038, IMP-049 |  |
| 17 | IMP-017, IMP-019, IMP-028, IMP-031, IMP-032, IMP-033, IMP-053, IMP-084, IMP-088 |  |
| 18 | IMP-021, IMP-022, IMP-051, IMP-055, IMP-057, IMP-102 |  |
| 19 | IMP-023, IMP-025, IMP-039, IMP-089, IMP-092 |  |
| 20 | IMP-024, IMP-040, IMP-042, IMP-086, IMP-087, IMP-090, IMP-091 |  |
| 21 | IMP-041, IMP-052, IMP-085 |  |
| 22 | IMP-043, IMP-093 |  |
| 23 | IMP-047, IMP-056, IMP-077 |  |
| 24 | IMP-103 |  |
| 25 | IMP-067, IMP-069 | serialized integration: IMP-067, IMP-069 |
| 26 | IMP-044, IMP-045, IMP-046, IMP-096 |  |
| 27 | IMP-048 | serialized integration: IMP-048 |

Before `IMP-068 = DONE`, only tasks without `IMP-068` in their transitive `depends_on` run (Bootstrap Mode, `audit_gates.md`). Two-phase tasks merge `IN_PROGRESS` and get `DONE` from a follow-up status PR. Serialized integration tasks run alone on their owned paths (`server/cmd/server/`, `server/internal/app/`, `client/Assets/Scripts/App/`, release artifacts).

## Parallel Execution
The coordinator claims a batch of ready tasks in one claim PR, then gives each implementer only the Implementer Prompt with its own `IMP-*`, branch, worktree path and base SHA. Concurrent tasks never share a worktree, database, port or Unity project/cache directory. The concurrency limit (5 tasks, at most 2 with `client/` paths; ADR-0058, ADR-0072) bounds parallelism. Work and CI run in parallel; merges are serialized by the merge slot (§5a), because the ruleset requires up-to-date branches and no merge queue exists.
