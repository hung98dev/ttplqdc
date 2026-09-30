# Implementation Wave Prompts
status: LOCKED

Operational prompts for executing every `IMP-*` task. Status, dependencies, scope, paths, tests and evidence are canonical in `task_queue.md`; the workflow, claim and merge sequence are canonical in `agent_execution_protocol.md` (§5a). This file never overrides them.

Waves are the longest-path levels of the `depends_on` DAG: wave 0 has no dependency and wave N tasks depend on a task in wave N-1. A task may start as soon as all its own dependencies are `DONE` on `main`; the coordinator recomputes readiness from `main` before every claim instead of waiting for a whole wave.

## How the Owner Runs the Project
1. Once, after Owner Setup: start the **Reviewer** session (§3 below) and the **Spec-owner** session (§4 below); they keep running and react to PRs / blockers by themselves.
2. For each wave in order, open one AI session in the repo and just say **"Làm wave N"** (e.g. "Làm wave 0 đi"; "làm wave tiếp" = next unfinished wave). `AGENTS.md` § Owner commands and `/run-wave` automatically run `/plan-wave`: `wave-planner` prepares a fresh plan, a separate `verifier` audits it, then the coordinator executes only passing, ready task plans. Do not ask the owner to approve a plan. Nothing else is needed from the owner except resolving `OPS-xxx` issues labelled `ops-blocked`.

### Wave Prompt (copy-paste, replace `<N>`)
```text
Thực hiện Wave <N> của repo thinhthan. Bạn là coordinator cho wave này.
Đọc AGENTS.md, docs/10_implementation/wave_execution_prompts.md, agent_execution_protocol.md (§3–§5b) và audit_gates.md (Bootstrap Mode).
1. Pull main. Nếu AUTO_MERGE_FROZEN=true hoặc có OPS-xxx mở với `blocks: ALL`: báo lại và dừng.
   Với mỗi task của Wave <N>: nếu một depends_on chưa DONE trên main (bị BLOCKED hoặc chưa xong), ghi task đó vào báo cáo là "chờ <dep>" và bỏ qua.
2. Trước mọi claim hoặc code: chạy /plan-wave với Wave <N> và SHA main vừa cập nhật.
   Agent wave-planner chỉ đọc, lập plan động theo On-Demand Planning Contract bên dưới; một verifier khác đọc nguồn và kiểm tra plan.
   Plan thiếu/bị FAIL/cũ: sửa plan và audit lại, không claim/code. Không có task sẵn sàng: báo chờ/BLK/OPS và dừng, không tự chạy wave khác.
3. Chỉ task có plan đầy đủ + audit PASS + DoR còn đúng trên main mới được claim theo §3 (IMP-000 tự claim trong PR của nó).
   Giao toàn bộ task plan và audit, cùng worktree, branch imp/IMP-XXX-<slug>, base SHA, DB port và Unity cache riêng, cho một implementer.
   Agent con chạy Implementer Prompt bên dưới và sống tới khi PR merge hoặc task BLOCKED.
   Tối đa 8 task song song, trong đó tối đa 4 task có client/ trong owned_paths (ADR-0075, ADR-0078); task còn lại chờ.
4. Merge slot (§5a): mỗi lúc chỉ một PR mang label `merge-slot`; trao cho PR sẵn sàng (reviewer APPROVE + CI xanh) có chỉ số topo nhỏ nhất
   (PR claim/block/ops trước). Sau khi PR đó merge thì trao tiếp. Không trao slot cho PR khác giữa merge IMP-068 và merge imp/IMP-068-done.
5. Theo dõi tới khi mọi task có thể chạy của wave là DONE trên main (two-phase task: cả PR imp/IMP-XXX-done theo §5a) hoặc BLOCKED; task chờ dep vẫn được báo riêng.
   Task BLOCKED vì BLK: spec-owner xử lý; khi task trở lại NOT_STARTED thì lập lại plan/audit và recheck DoR trước khi claim. BLOCKED vì OPS: chờ chủ repo đóng issue ops-blocked, rồi mở PR ops/ trả task về NOT_STARTED; lập lại plan phần bị ảnh hưởng.
6. Báo cáo cuối: plan revision + audit, từng task -> DONE/BLOCKED (mã BLK/OPS)/chờ dep, PR đã merge, số lần CI chạy, việc chủ repo cần làm (nếu có).
Không tự sửa spec, không hỏi con người trong lúc chạy, không push thẳng main, không rebase/force-push.
```

## On-Demand Planning Contract

This section owns the planning output and quality gate; `/plan-wave` owns the method. The Waves table is a DAG grouping, not an implementation plan. Generate plans only when requested or when wave execution reaches this phase; never commit prewritten plans for all waves.

### Inputs and authority

The coordinator supplies N, a clean read-only snapshot at a recorded `main` SHA, current freeze/OPS state, and active claims/resource reservations. Standalone planning may inspect local `main` without claiming; any unverified execution precondition remains explicitly unresolved and cannot authorize dispatch. Derive wave membership from the complete `depends_on` DAG and compare the table; a mismatch goes to `spec-owner`, not a guessed schedule.

For replan/resume, the coordinator additionally supplies `resume_task_ids` (default empty) and each existing claim's verified implementer, branch/worktree and isolated resources. An IN_PROGRESS task is plan-eligible only when explicitly selected for that same live claim, dependencies/DoR still hold and no conflicting claim/blocker/freeze exists; retain its IN_PROGRESS disposition. Its complete plan/audit may authorize only resuming that implementer, never a second claim/implementer. Other IN_PROGRESS tasks receive accounting only. Record missing claim/resource proof as a finding; no inferred resume selection.

Read each packet in full, its owning specs and applicable ADRs, direct dependency packets/actual outputs, live blockers, layout, conventions and relevant verification/evidence rules. Use source sections/ranges; do not invent existing code for planned paths. Required contracts consumed from a `PROPOSED` ADR are unresolved, not accepted decisions.

The planner and plan auditor are read-only delegates, not new privileged GitHub roles: they receive the coordinator's snapshot and need no token/App key. They neither record task status nor resolve a BLK. Pre-implementation gaps stay in session `Findings` for `spec-owner`, without allocating BLK IDs or appending the implementation register; `known_blockers.md` is for blockers encountered during claimed implementation (§6). A `BLOCKED` plan disposition/audit verdict is not packet status. External setup failures follow the coordinator's OPS workflow.

### Required plan artifact

Return one session artifact in Markdown with these named sections. The coordinator preserves each revision as an immutable artifact/message reference; increment `plan_revision` whenever content changes and never reuse the old revision's audit.

| Section | Required content |
|---|---|
| Identity | wave N, `plan_revision`, `plan_artifact_ref`, `planning_main_sha`, source readset with path + section/range, verified and unresolved execution preconditions |
| Readiness | exactly one row per wave task: current status, each dependency/status, applicable BLK/OPS (task **or owning spec**), DoR evidence/gaps, disposition `READY_TO_PLAN` / `WAITING_DEP` / `BLOCKED` / `IN_PROGRESS` / `DONE`; retain all reasons when both blocked and waiting |
| Dispatch | only completely planned, audited and still-ready task IDs may enter executable batches; account for active claims, 8 total/4 client limits, owned-path/shared-resource collisions, serialized integration and the IMP-068 cutover |
| Findings | every unresolved contract/environment/plan-quality gap, source evidence, affected tasks and responsible role; no guessed resolution or invented BLK ID |
| Task plans | the complete per-task format below for each `READY_TO_PLAN` task and explicitly selected eligible same-claim IN_PROGRESS resume; no speculative handoff for other in-progress/waiting/blocked/done tasks |

Each task plan must contain:

| Section | Required content |
|---|---|
| Scope/readset | task ID/profile, goal and non-goals, exact packet/spec/ADR/code sections to read; owned/forbidden paths and absent-but-owned outputs; canonical pins by source |
| Contract map | each contract input/output, producer and consumer, exact types/fields/signatures or approved schema references; relevant errors, limits, state transitions, durable keys, replay/rollback and allocation/frame budgets; explicit source-backed not-applicable cases |
| File/API map | exact authored files and symbols/signatures to create/change, existing implementation to reuse with source location, generated/materialized outputs and their authoritative generator; names of private implementation helpers may be chosen, never new product/wire/data contracts |
| Ordered steps | local step IDs with prerequisites, exact file/symbol targets, concrete change/algorithm, invariants and stopping condition; no "implement feature", "handle errors", "add tests" or decisions left for the implementer |
| Coverage matrix | every semantic obligation in `## Change`, every `## Acceptance` item/requirement ID, every named `## Tests` case and cleanup obligation mapped to step IDs and observable verification; split compound obligations, never count headings as semantic coverage |
| Verification | exact commands/platform/prerequisites and expected behavior, boundary/error/replay cases as applicable, an actual runtime/UI/CLI smoke scenario, codegen/materialization handling, evidence location and canonical DONE/merge references; planned checks are not executed results |
| Handoff | whole task plan + matching audit revision, then coordinator-assigned branch/worktree/post-claim base SHA, implementer and isolated resource reservations; no task-ID-only or lossy summary prompt |

Use canonical requirement IDs where present; otherwise use source section + local item/step labels bound to the snapshot. These labels are not new project requirements. An implementation step can cover multiple obligations only when its concrete changes and verification satisfy each one.

### Quality gate

The planner first audits its own matrix for omissions and contradictions. A **different `verifier` session** then reads the original sources, not just the planner's citations, and returns: wave N, `planning_main_sha`, `plan_artifact_ref`, `plan_revision`, checked source list, per-task `PASS` / `REWORK` / `BLOCKED`, every defect with evidence and required correction, and the exact `dispatchable_task_ids`. The planner never issues its own passing audit.

Native Devin delegation uses `wave-planner` and `wave-plan-auditor`; the latter runs the verifier's Wave Plan Audit method with a real `read/grep/glob`-only tool set, not the exec-capable normal verifier profile. Inline skill `allowed-tools` alone is not a read-only boundary. Other harnesses must supply equivalent capability-restricted independent delegates.

Keep reviewed artifact revisions immutable. The matching audit, selected executable batches and later branch/worktree/base-SHA/resource assignments travel in a separate coordinator handoff envelope referencing the original artifact; adding execution facts never silently rewrites an audited plan. A design/content change still creates a new revision and requires a new audit.
For an eligible same-claim resume, apply all six checks to the full regenerated task plan and verified claim/resources; a per-task PASS may enter `dispatchable_task_ids` only for that existing implementer. Dispatch records distinguish new claim from same-claim resume. Empty eligible candidate set means no task plans or handoffs.


`PASS` requires all of the following; a score/majority is not sufficient:

1. All wave tasks and all semantic task obligations are accounted for; no duplicate, missing or silently deferred scope.
2. Contracts are satisfiable across producers/consumers; existing APIs actually exist and chosen implementation details obey the owning spec. Missing product decisions are blockers.
3. Paths, dependencies, pins, authority boundaries, error/recovery behavior and resource reservations are valid.
4. Each step is actionable without designing a missing contract; all authored/generated outputs and named tests have owners.
5. Verification detects plausible consumer-visible defects and observes the changed path; no source-text/mock-echo tests or imagined passing CI.
6. No unresolved gap affects a dispatchable task; plan revision and readiness still match the source snapshot.

Counts and matrices expose omissions; they do not prove semantic correctness. Audit findings go back to the planner for complete revision and a new audit. If sources are insufficient, route to the owning role and stop that task rather than lower the quality bar.

Before each claim/resume, compare fresh `main` and execution preconditions with the reviewed inputs. Relevant spec/packet/API/dependency-output/blocker/setup changes invalidate affected task plans; unrelated changes and ordinary claim-field transitions need a recorded recheck, not automatic reuse or unnecessary redesign. Preserve `planning_main_sha`; add the post-claim base SHA. Plan approval never replaces PR `policy-review`, CI, evidence or the merge slot.

## Session Prompts (copy-paste)

Start one long-running session per role. If the harness has no `/run-imp-task` skill, the agent follows `.devin/skills/run-imp-task/SKILL.md` as a checklist. Role rules: `agent_execution_protocol.md` §1.

### 1. Coordinator (one session, runs the whole pipeline)
```text
Bạn là coordinator của repo thinhthan. Đọc AGENTS.md, docs/10_implementation/agent_execution_protocol.md (§3, §5a, §5b), audit_gates.md và file này.
Lặp liên tục cho tới khi IMP-048 DONE:
1. Pull main. Nếu biến repo AUTO_MERGE_FROZEN=true hoặc có OPS-xxx mở với `blocks: ALL` thì dừng và chờ (ghi OPS cho issue ops-blocked do merge guard mở qua PR ops/).
2. Tính readiness từ main (mọi depends_on DONE, không BLK/OPS nêu task hoặc owning spec, tuân Bootstrap Mode); chạy /plan-wave cho wave có task kế tiếp.
3. Chỉ claim task có plan đầy đủ, audit PASS của verifier khác và DoR recheck đúng; theo §3 (IMP-000: claim trong PR của nó), tối đa 8 task IN_PROGRESS (tối đa 4 client), ưu tiên topo nhỏ nhất.
4. Giao mỗi task cho đúng một implementer bằng Implementer Prompt kèm toàn bộ task plan/audit revision, branch, worktree, base SHA và tài nguyên riêng.
5. Quản lý merge slot theo §5a (label `merge-slot`, một PR mỗi lúc).
6. Task BLOCKED vì BLK -> giao cho spec-owner; OPS đã được chủ repo đóng issue -> PR ops/ đánh dấu Resolved; claim quá 24h không hoạt động -> trả về NOT_STARTED.
Không tự viết code, không sửa spec, không hỏi con người.
```

### 2. Implementer (one session per task)
```text
Thực hiện <IMP-XXX> trên branch <branch>, worktree <path>, base <sha>, task plan <plan-artifact + task-section>, audit <PASS + matching plan_revision>.
Trước khi tạo branch/draft PR, chạy codegen hoặc sửa code: đọc task plan và audit; thiếu/cũ/mâu thuẫn thì trả coordinator, không tự đoán. Chạy /run-imp-task (hoặc .devin/skills/run-imp-task/SKILL.md) theo agent_execution_protocol.md §2a, §4–§5b.
Chỉ sửa owned_paths và path test/evidence của packet. Không hỏi con người. Không push thẳng main, không workflow_dispatch, không rebase/force-push.
CI báo `commit unity-materialized`: tải artifact unity-materialized-windows, commit nguyên văn, push (§4b).
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
| 1 | IMP-001, IMP-061, IMP-063, IMP-101, IMP-106 | two-phase: IMP-061; bootstrap-eligible: IMP-001, IMP-061, IMP-063, IMP-101, IMP-106 |
| 2 | IMP-002, IMP-005, IMP-064, IMP-070, IMP-083 | two-phase: IMP-005, IMP-083; bootstrap-eligible: IMP-002, IMP-005, IMP-064, IMP-070, IMP-083 |
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
The coordinator plans and separately audits ready tasks before claiming a batch, then gives each implementer the complete audited task plan plus the Implementer Prompt, its own `IMP-*`, branch, worktree path and base SHA. Concurrent tasks never share a worktree, database, port or Unity project/cache directory. The concurrency limit (8 tasks, at most 4 with `client/` paths; ADR-0075, ADR-0078) bounds parallelism. Work and CI run in parallel; merges are serialized by the merge slot (§5a), because the ruleset requires up-to-date branches and no merge queue exists.
