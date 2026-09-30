# ADR-0078: Unity-on-Windows-Only CI and Go-Only Linux Runner Topology
status: ACCEPTED

## Context
Measured CI throughput under ADR-0073, ADR-0075 and ADR-0077 established that clean PR runs take 4.3–5.3 min, but tail runs took 8–10 min due exclusively to Linux Unity container retry paths: materialization kills at assembly reload in 5 of 14 runs and test watchdog kills in 2 of 14 runs. The Linux Unity container image (`unityci/editor:ubuntu-6000.6.1f1-base-3.2.2`) is 9.7 GB, requires a 76–101 s network pull on every run, and cannot be cached without consuming half the repository cache budget (ADR-0073).

Furthermore, running Unity on both Linux and Windows concurrently required 2 Unity licence activations per PR run. Under the personal/student licence profile (ADR-0072), concurrent activations across PR runs, post-merge runs and cache warming caused license contention and forced coordinator concurrency to stay bounded at at most 2 client-path tasks.

The owner decided (2026-09-30):
1. **Unity runs on Windows only.** Linux runs no Unity editor, no GameCI actions, no Docker Unity image, no xvfb and no llvmpipe.
2. **Linux runner stays for Go, PostgreSQL and Linux production parity.** The single world binary is deployed on Ubuntu Server 24.04 (`deploy/prod/`, ADR-0052, ADR-0066), Go `-race` detector requires cgo with a system C compiler which the Windows runner does not install (ADR-0072), and PostgreSQL 18.6 service containers run natively on Linux.
3. **Repository Actions cache budget is raised to 30 GB** (owner configuration).

## Decision

1. **Job Topology (3 jobs per PR run):**
   - `Unity (Windows)` (`unity-windows`, `windows-2022`): Runs every Unity step natively (ADR-0073) — diff-based scope detection (`SKIP(no-client-change)`), mode planning (`verify -plan-unity`), native Unity editor `6000.6.1f1` (installer-SHA cached), `client/Library` cache (exact-key only), materialization, planned tests (EditMode/PlayMode), rendering passes (Performance category and Visual Review screenshots), drift check (`commit unity-materialized-windows`), and uploads artifacts `unity-materialized-windows`, `unity-test-results-windows` and `visual-review`.
   - `Q0-Q6 verify (Windows)` (`windows-2022`, required check): Runs Go/EDB-Postgres/protoc gates in parallel with `Unity (Windows)`, runs the pre-Unity phase (`verify.ps1 -Phase pre-unity`), joins `Unity (Windows)` via `.devin/scripts/wait_job.sh`, downloads its test results, runs the Unity phase (`-Phase unity -PreReport ...`), joins `Q0-Q6 verify (Linux)` (which finishes early), downloads `verify-report-linux`, merges both reports with `verify.ps1 -MergeReports` into schema-v2 `manifest.json`, and uploads artifact `evidence`. Required check name `Q0-Q6 verify (Windows)` is unchanged.
   - `Q0-Q6 verify (Linux)` (`ubuntu-24.04`, required check): Runs Go build, `gofmt -l`, `go vet ./...`, staticcheck `2026.2.1`, Go unit tests with `-race` for `sim|edge|durable|global`, non-race allocation-budget pass, PostgreSQL 18.6 service-container tests (Q5 migrations apply/down/apply), and builds the production `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` binary. It runs no Unity steps, downloads no GameCI images, and activates no Unity licence. Unity-dependent gates in its report (`Q1.unity.editor`, Q3 Unity EditMode/PlayMode/Performance) are reported as `SKIP(windows-only)` and are supplied to the evidence manifest exclusively from the Windows report. Required check name `Q0-Q6 verify (Linux)` is unchanged.

2. **Evidence Merge Inversion:**
   Because the Linux Go job takes ~1.5–2 min and the Windows Unity pipeline takes ~4–5 min, the Windows required job joins the Linux required job and merges the evidence manifest (reversing ADR-0075 item 3). The Linux job never polls or waits for Windows, eliminating 50% of workflow polling requests.

3. **Software Rendering on Windows (D3D11 WARP):**
   Functional EditMode and PlayMode tests run with `-batchmode -nographics` on Windows (ADR-0073). Tests requiring a graphics device — the `Performance` category (`PERF-002`, `PERF-005`, `PERF-006`, `PERF-007`, `PERF-016`) and Visual Review screenshot captures (`client/Assets/Scenes/Review/`) — run on Windows without `-nographics` using `-force-d3d11`. The GPU-less `windows-2022` hosted runner provides Direct3D 11 via the Microsoft Basic Render Driver (WARP). Metadata recorded on Visual Review screenshots changes from `renderer=llvmpipe` to `renderer=warp`. `PERF-002` main-thread CPU budget excludes rendering markers and drops the Linux-only `LP_NUM_THREADS=1` environment variable.

4. **Native Android IL2CPP Build on Windows (IMP-067, IMP-096):**
   Android IL2CPP player builds move from the Linux `unityci/editor:ubuntu-6000.6.1f1-android-3.2.2` GameCI image to the native Windows Unity editor. IMP-000 pins the official Unity Android Support installer and its submodules (OpenJDK 17, Android NDK r27c, cmake 3.22.1, build-tools 36.0.0, platform-tools 36.0.0, platforms 34/36/37, commandlinetools 16.0/12266719) with exact SHA-256 hashes in `../00_context/technology_versions.md`. The Android modules are installed into `${{ runner.temp }}/unity-editor` and cached separately as `unity-android-module-Windows-<shas>` (restored only by player-build jobs; never by standard PR verify). The scheduled `device-perf` workflow runs its Android build on `windows-2022`, uploads the APK, and runs `gcloud firebase test` on `ubuntu-24.04` with the pinned gcloud tarball.

5. **Client Concurrency Expansion:**
   Because each client PR activates only 1 Unity licence (Windows only) instead of 2 (Linux + Windows), concurrent licence activations per client PR are halved. The coordinator concurrency limit for tasks with `client/` owned paths is raised from 2 to 4 tasks (ADR-0072 item 13 amended). Total concurrent task limit remains 8 on GitHub Pro.

6. **Cache Budget Alignment (30 GB):**
   The repository Actions cache limit is recorded as 30 GB (owner-configured in GitHub Settings), superseding the 50 GB figure in ADR-0075. The Linux `unity-library` cache is deleted, freeing ~2.5 GB. Caches on Windows: `unity-editor` (~3.2 GB), `unity-library` (~2.5 GB), `edb` (~0.35 GB), `cli-tools` (~0.1 GB), `go-build` (~1.5 GB). Total base cache ~7.7 GB, well within 30 GB with room for PR caches.

7. **Git LFS Object Cache:**
   To prevent exhausting the 10 GiB monthly LFS bandwidth quota, only the `Unity (Windows)` job fetches LFS content. Required verify jobs checkout with `lfs: false`. The Unity job caches `.git/lfs` keyed on the SHA-256 of sorted LFS object OIDs (`lfs-objects-${{ runner.os }}-<hash>`) with restore-keys fallback, then runs `git lfs pull` to fetch only missing objects. `cache_warm.yml` warms the LFS cache on pushes to `main`.

8. **Poll Rate-Limit Safety & PR Concurrency Group:**
   `.devin/scripts/wait_job.sh` enforces a 45-minute timeout (`WAIT_TIMEOUT_SECONDS=2700`) and backs off on HTTP 403/429 responses. `verify.yml` adds `concurrency: group: verify-${{ github.event.pull_request.number || github.ref }}, cancel-in-progress: ${{ github.event_name == 'pull_request' }}` so superseded commits immediately cancel and release runners and licence seats. Pushes to `main` (post-merge guard) never cancel.

## Consequences

- **Supersedes**:
  - ADR-0058: Linux job runs Unity, xvfb, llvmpipe, GameCI images, and Android build image. (Linux job is now Go/PostgreSQL only; Unity is Windows only).
  - ADR-0059: Overdraw via llvmpipe. (Overdraw measured under D3D11 WARP on Windows).
  - ADR-0066: `PERF-002` measured on Linux job under llvmpipe with `LP_NUM_THREADS=1`. (Measured on Windows job under WARP with `-force-d3d11`).
  - ADR-0070: `PERF-002` environment details.
  - ADR-0072: Item 8 (`-race` on Linux job only stands; Unity licence concurrency limit 2 raised to 4).
  - ADR-0073: Item 2 (Linux GameCI image pull eliminated entirely).
  - ADR-0075: Item 1 (`Unity (Linux)` job eliminated; 3 jobs per PR run), Item 3 (evidence merged in `Q0-Q6 verify (Windows)`), Item 5 (cache budget 50 GB amended to 30 GB).
  - ADR-0077: Item 1 (Linux warm materialization snapshot retired), Item 3 (Linux test verdict rules retired; Windows results XML is final), Item 4 (kill-probe and `UNITY_KILL_PROBE` retired), Item 5 (Linux overlapped image pull retired). Item 2 (compiler error stop) and Item 6 (two-phase verifier on Windows) stand.

- **Specs updated**:
  - `docs/00_context/technology_versions.md`: Remove GameCI Linux images and unused actions; add Unity Windows Android module + submodules exact pins; add gcloud Linux/Windows SHA-256 hashes; update runner matrix description.
  - `docs/10_implementation/audit_gates.md`: Owner Setup runners, cache limit 30 GB, LFS budget note, 3-job layout, Windows-only Unity materialization and WARP rendering, evidence merge in Windows required job, client concurrency 4.
  - `docs/10_implementation/task_queue.md`: `IMP-000` (ADR-0078 in adrs, acceptance and tests), `IMP-063`, `IMP-064`, `IMP-005`, `IMP-067`, `IMP-070`, `IMP-095`, `IMP-096`, `IMP-106`, art packets (`IMP-071`..`075`, `IMP-104`, `IMP-105`).
  - `docs/04_architecture/client_performance.md`: `PERF-002`, `PERF-005`, `PERF-007`, `PERF-016` measured on Windows Unity job under WARP; drop `LP_NUM_THREADS=1`; Android device-perf build on Windows.
  - `docs/07_content/presentation_asset_manifest.md`: Visual Review screenshots rendered on Windows CI job under WARP (`renderer=warp`).
  - `docs/09_testing/test_and_release_evidence.md`: Test table reflects Windows Unity job and Windows evidence merge.
  - `docs/10_implementation/agent_execution_protocol.md`: Artifact download is `unity-materialized-windows`; concurrency ≤ 8 tasks (≤ 4 client); evidence merge on Windows.
  - `docs/10_implementation/repository_layout.md`: Artifact index uses `unity-materialized-windows`.
  - `docs/10_implementation/wave_execution_prompts.md`: Concurrency 8 tasks (4 client); artifact download is `unity-materialized-windows`.
  - `AGENTS.md`: Update CI description (Unity on Windows only, `-race` on Linux only, artifact `unity-materialized-windows`).
  - `.devin/scripts/cache-policy.md`: Remove Linux `unity-library` and image digest; add `lfs-objects` and `unity-android-module`; document 30 GB budget.
  - `.devin/scripts/wait_job.sh`: Timeout safety, rate-limit backoff, Windows required job description.
  - `.devin/agents/coordinator.md`, `.devin/HANDBOOK.md`, `.devin/skills/produce-art-asset/SKILL.md`, `.devin/skills/run-imp-task/SKILL.md`: Update job references and concurrency numbers.

## Amendment (ADR-0079)
All checkouts, including Windows Unity, use `lfs: false` and `GIT_LFS_SKIP_SMUDGE=1`. Authorized Windows media jobs restore the OID-keyed object cache before explicit missing-object pull; required verify jobs never hydrate media. PR cancellation covers both pull_request and pull_request_target; main pushes do not cancel. Actual WARP/licence/art-provider capability must be observed before claiming performance/visual/art evidence; runner/model selection alone is not proof.
