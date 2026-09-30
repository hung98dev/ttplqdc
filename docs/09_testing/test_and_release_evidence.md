# Test and Release Evidence Contract
status: LOCKED

## Scope

Evidence, verification commands and release acceptance for thinhthan. Every `DONE` must be reproducible from a clean checkout without relying on agent memory or chat. Identity rules: ADR-0045 as amended by ADR-0057.

## 1. Command Matrix (Linux and Windows, local and CI; ADR-0058)

| Operation | Command | Requirement |
|---|---|---|
| Full repository verify | Windows CI: `pwsh -NoProfile -File scripts/verify.ps1 -UnityResultsDir <dir>`; Linux CI: `pwsh -NoProfile -File scripts/verify.ps1` with Unity delegation per ADR-0078; local: `pwsh -NoProfile -File scripts/verify.ps1 -LocalDeferMissing` | Q0-Q6; non-zero exit on any failed gate; uses `THINHTHAN_TEST_PG_DSN` when set (Linux CI `postgres:18.6` service container), otherwise Windows starts pinned EDB binaries on a random port; Linux local starts pinned `postgres:18.6` digest via Docker when available. `-LocalDeferMissing` never in CI; missing local tools report `DEFERRED(local-missing)`, never CI evidence |
| Protobuf codegen | `pwsh -NoProfile -File scripts/codegen.ps1` | Go + C# output with zero drift |
| Content compile | `go -C server run ./cmd/compiler` | all content catalogs compile |
| Backend tests | `go -C server test ./...` on both OSes; `-race` for `sim\|edge\|durable\|global` on the Linux job only (ADR-0072) | unit and architecture tests |
| Go static checks | `gofmt -l server`, `go -C server vet ./...`, `staticcheck ./...` (pinned `v0.8.1`, run in `server/`) | Q4 `CODE-003`; empty output (ADR-0059) |
| Go allocation budgets / benchmarks | `go -C server test -run TestAllocs_ ./...` (non-race build); `go -C server test -run "^$" -bench . -benchmem -benchtime=200x -count=1 <hot packages>` | allocs/op exact (`../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets); ns/op report-only in `verify-report.json` |
| Unity EditMode | local: `"$UNITY_EDITOR_PATH" -batchmode -nographics -projectPath client -runTests -testPlatform EditMode -testResults <tmp>/editmode.xml -logFile <tmp>/editmode.log`; CI: native `Unity (Windows)` writes results into `artifacts/unity-tests/`; Linux delegates with `SKIP(windows-only)` | required when IMP-000 DONE on main/head; legal status/scope skips per audit_gates |
| Unity functional PlayMode / graphics | functional mode uses `-nographics`; graphics categories use `-force-d3d11` without `-nographics`, with observed WARP device proof (§3.3a manifest) | independent owners: functional PlayMode IMP-065, VisualReview IMP-070, Performance IMP-095, representative-load graphics IMP-067; required when own owner DONE on main/head, not hidden behind functional PlayMode |
| Android performance | `gcloud firebase test android run --type game-loop` on the Owner Setup device models | scheduled `device-perf` workflow on `main` (not a PR check); `DEFERRED(quota)` on exhausted quota (`../04_architecture/client_performance.md`) |

`scripts/verify.ps1` resolves the Unity editor from `UNITY_EDITOR_PATH` (must contain `6000.6.1f1`) and fails if the version differs; with `-UnityResultsDir` it gates on the Unity result XML instead (missing or failed results = FAIL). CI: Windows runs the pinned native install (ADR-0073); Linux runs no Unity (ADR-0078). Library cache and temp directories are per worktree (CI: `actions/cache` for Windows). CI has no GPU: rendering uses D3D11 WARP on Windows (ADR-0050, ADR-0078). Before any gate the `Unity (Windows)` job opens `client/` in the editor and uploads editor-created or -modified files as artifact `unity-materialized-windows`, failing with `commit unity-materialized` (`../10_implementation/audit_gates.md` § Job Preconditions). A missing tool or licence on CI is an OPS failure, never a silent skip.

Both required verify jobs checkout with `lfs: false` and `GIT_LFS_SKIP_SMUDGE=1`. Only Windows Unity/player-build/cache-warming jobs fetch media; graphics/provenance/texture/audio checks require hydrated assets there. Linux must not open Unity, fetch LFS media or wait for Windows.

## 2. Evidence Manifest

CI is the enforcement source. `verify.yml` checks out the PR head SHA, computes `source_tree_hash` and runs Q0-Q6 in the jobs `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)`; `Q0-Q6 verify (Windows)` waits for the Linux job (which finishes early), downloads its report of the same run (`actions/download-artifact`), merges both and uploads `manifest.json` as artifact `evidence` (ADR-0075, ADR-0078; no separate evidence job). The agent downloads it (`gh run download <id> -n evidence`) into `docs/10_implementation/evidence/<ID>/` and commits it unchanged. FAILED manifests are never committed.

```text
source_tree_hash = SHA-256 over sorted lines "path NUL git-blob-sha LF" from `git ls-files`, excluding
                   docs/10_implementation/evidence/**, docs/10_implementation/task_queue.md,
                   docs/10_implementation/known_blockers.md
```

Required JSON fields:

```text
schema_version = 2
task_id
source_tree_hash
toolchain.go = 1.27.1
toolchain.unity = 6000.6.1f1
toolchain.postgresql = 18.6
toolchain.protoc = 36.2
toolchain.protoc_gen_go = v1.36.12
commands[]
test_summary.total / passed / failed / skipped
skipped_reasons[] (id, reason, allowed)        -- owner/status/scope skips per audit_gates; windows-only permitted only in raw Linux reports and resolved from Windows before merged evidence
content_revision                               -- "none" while no content compiler exists at the tested source
ci_run_id, run_attempt
jobs[] (name, os = linux|windows, result = PASSED)   -- both verify jobs of the same run
gates[] (id, os, owner, result, reason)          -- complete per-OS report verdicts; merged Unity verdicts use Windows only
codegen_drift[] (path, result)                  -- complete generated output set; every result = IDENTICAL when Q2 active
worktree_clean = true
result = PASSED
```

Q6 validates only manifests added in the PR diff: `source_tree_hash` equals the PR head tree hash, and the GitHub API confirms `ci_run_id`/`run_attempt` belong to workflow `verify.yml` with conclusion `success`. Older manifests get schema checks only. A two-phase task's manifest comes from its follow-up status PR's own run on its final code head (fixes inside the packet's `owned_paths` allowed); its hash equals the merged head because `task_queue.md`, `known_blockers.md` and `evidence/**` are excluded (ADR-0068, ADR-0072). A head that sets `DONE` without a manifest passes Q0/Q6; the merged head must contain it. Milestone evidence lives in `docs/10_implementation/evidence/M<n>/manifest.json` with the same schema (`task_id` = `M<n>`).

### 2a. Stable Evidence Projection (`CI-004`)

Cold/warm comparison uses two successful runs on the same source tree, task and activation/scope plan. First validate both full manifests independently (schema, API identity, commands, clean state and all required results); projection equality never substitutes these validations. Compare canonical JSON containing exactly `schema_version`, `task_id`, `source_tree_hash`, all `toolchain` fields, `content_revision`, `worktree_clean`, `result`, `test_summary`, `jobs` tuples `(name,os,result)` sorted by `(os,name)`, `gates` tuples `(id,os,owner,result,reason)` sorted by `(os,id)`, `skipped_reasons` tuples `(id,reason,allowed)` sorted by `(id,reason)`, and `codegen_drift` tuples `(path,result)` sorted by path. Object keys sort lexically; scalar values retain types; no gate/skip/drift entry may be dropped. Required drift entries all equal `IDENTICAL`; merged Unity gates resolve Linux delegation using Windows evidence.

Exclude only execution-specific fields: `ci_run_id`, `run_attempt`, workflow/job/artifact URLs and timestamps, raw `commands` (which contain run-specific absolute/temp paths), command durations, report-only benchmark `ns/op` and `cached_steps` (`hit|miss`, `wall_seconds`). These remain in their owning original artifacts for independent validation/measurement. A result, test count, pin, semantic revision, skip, gate verdict or generated output difference fails CI-004. Whole manifest bytes differ legitimately by run identity; do not rewrite either artifact to make them equal.

## 3. Fixture Registry

All fixtures are registered and versioned:

1. **Protocol Binary Fixtures:** Lưu tại `proto/testdata/golden/`. Các message nhị phân mẫu được dùng chung để kiểm thử khả năng giải mã tương đồng giữa Go và C#.
2. **Catalog Compile Fixtures:** Lưu tại `server/internal/config/testdata/`. Gồm:
   - `golden_bundle/`: Bộ catalog hợp lệ đầy đủ.
   - `invalid_cross_ref/`: Catalog chứa tham chiếu hỏng (phải bị reject).
   - `invalid_balance_window/`: Catalog có TTK vi phạm guardrail.
3. **Deterministic Movement & Collision Vectors:** Lưu tại `server/internal/sim/spatial/testdata/`. Chứa tọa độ, vận tốc, phím bấm và kết quả tính toán vị trí chuẩn.
4. **Client Hotspot Stream:** `client/Assets/Tests/PlayMode/NetReceive/Fixtures/hotspot_stream_40.bytes`, 60 s of 10 Hz snapshot/delta envelopes for 40 replicated entities + local player, written by the seeded generator `HotspotStreamGenerator` in the same folder (IMP-065); a test regenerates it and byte-compares. Consumers: `PERF-024`, the IMP-095 hotspot scene (`../04_architecture/client_performance.md`).
5. **Sim Steady-State Fixture:** built in code by IMP-079 (`server/internal/sim/runtime/`): one channel with 64 replicated actors (22 player slots + 42 monster slots), fixed seed; consumers `HOT-001..003` (`../08_scale_ops/capacity.md`).

Fixtures are immutable; changing one requires the spec change that justifies it.

## 4. Content Compile Report

The compiler writes `content_compile_report.json` with `content_revision_hash`, `catalogs_evaluated`, `entity_counts` (counts come from the owning catalogs in `docs/07_content/`, never hard-coded here) and `validation_diagnostics` (0 errors, 0 warnings required).

## 5. Load & Scale Fingerprint

Kiểm thử chịu tải (M9/M10, IMP-046, IMP-055) bắt buộc phải đính kèm cấu hình môi trường thực thi:
- **Server Specs:** Số vCPU, dung lượng RAM, storage IOPS.
- **Topology:** một world process (ADR-0052), database pool size, `WORLD_CCU_CAP`.
- **Concurrency Targets:**
  - Admission cap: 18 người chơi / channel (ADR-0035); forced-placement cap: 22 (ADR-0061).
  - Bản đồ: 540 người chơi (admission).
  - Hotspot: 42 named mechanic + 22 người chơi (ADR-0066).
  - Entity cap: 100 entity = 22 slot người chơi + ngân sách theo lớp 42 spawn-group / 12 event / 8 boss / 16 transient; spawn vượt ngân sách lớp của nó bị từ chối, các lớp khác vẫn spawn (ADR-0070).
- **Metrics:** p50/p95/p99 tick duration; gate = p95 < 35ms ở 20 Hz (`../08_scale_ops/capacity.md`); network bandwidth per client measured by `load.md` scenario 13 (gate value per `../08_scale_ops/capacity.md` § Bandwidth Budget).

## 6. Flaky Test Policy

- A test with different results on the same source and environment is flaky and a P0 defect.
- No retry flags to hide it; no commenting out. Quarantine only through an ADR already on `main` (gate ratchet) plus a `BLK-xxx` for the fix (`../10_implementation/agent_execution_protocol.md` §5b).
- The M10 release candidate has zero flaky or quarantined tests.

## 7. Invariants

```text
no DONE without a CI-produced manifest committed unchanged
every verify command runs on a clean checkout
chat logs and screenshots never replace evidence (screenshots are review artifacts)
worktree_clean = true after verify
CI = GitHub-hosted jobs `Q0-Q6 verify (Linux)` + `Q0-Q6 verify (Windows)`; `policy-review` is the App check run; post-merge guard (ADR-0050, ADR-0057, ADR-0058, ADR-0072)
DEFERRED(local-missing) exists only locally; CI never passes -LocalDeferMissing
missing required CI tool/licence or graphics capability = OPS failure, never a skip; Linux Unity-only SKIP(windows-only) requires Windows resolution before evidence merge; owner/status/scope skips only per audit_gates; Test Lab quota = DEFERRED(quota)
```
