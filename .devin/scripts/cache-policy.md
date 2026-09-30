# CI cache policy (IMP-106, engineering_conventions.md §6)

Applies to `.github/workflows/verify.yml`. Enforced by
`server/internal/conformance/caching` tests in CI (Q3).

## Mechanism

- Only `actions/cache@55cc8345863c7cc4c66a329aec7e433d2d1c52a9` (v6.1.0, pinned
  in `server/internal/stackpin/pins.go`) may be used. The combined action is
  required — `actions/cache/restore` / `actions/cache/save` are unlisted
  actions and fail Q1 `stackpin.github_actions`.
- `setup-go` keeps `cache: false` — the explicit `actions/cache` step is the
  only Go cache mechanism.
- A cache step restores AND saves (post-job). Save-on-failure poisoning is
  prevented by writing payloads atomically (`.part`/`mv`) and by validating
  content on restore (Windows editor installer sha-256 marker, CLI pin marker, EDB sha-256
  marker). An unloadable entry degrades to the uncached path, never to a
  failed gate.

## Keys and restore-keys

- Key grammar: `<scope>-${{ runner.os }}-<pin>-<content-hash>`. Every key must
  contain `${{ runner.os }}` plus every input that determines the payload:
  | scope           | pin                                  | content hash |
  |-----------------|--------------------------------------|--------------|
  | `go-build`      | `env.GO_VERSION`                     | `hashFiles('server/go.sum')` |
  | `unity-editor`  | `env.UNITY_WINDOWS_EDITOR_SHA256` (Windows only, ADR-0073) | (sha is the pin) |
  | `cli-tools`     | pinned pwsh/jq/gh/git-lfs versions + sha prefixes (Windows only) | (pins only) |
  | `unity-library` | Linux `env.UNITY_LINUX_IMAGE_DIGEST`, Windows `env.UNITY_WINDOWS_EDITOR_SHA256` | `hashFiles(manifest.json, packages-lock.json, ProjectSettings/**, Assets/**/csc.rsp)` |
  | `edb`           | `env.EDB_ZIP_SHA256` + version       | (sha is the pin) |
- `restore-keys:` entries must keep `${{ runner.os }}` AND the pin segment —
  a fallback may only roll the content hash within the same OS + same pinned
  toolchain/image/digest. Bare prefixes (`go-build-`, `unity-editor-`) that
  would substitute another pin or OS are forbidden (CI-001).
- Content-derived caches — payloads produced *from* the hashed inputs
  (`unity-library`: PackageCache/ScriptAssemblies = f(manifest, lock,
  ProjectSettings, compiler flags)) — restore on the exact key only and set
  no `restore-keys`: a prefix hit restores output built from different
  inputs, a silent wrong-content restore (BLK-005: stale PackageCache/
  ScriptAssemblies made Unity recompile `com.unity.ugui` under
  `-nullable:enable` → CS86xx, and correlated with editor self-SIGKILL at
  precompiled-dll registration). `restore-keys` remain legal on
  content-addressed stores whose entries stay valid under a partial restore
  (`go-build`) and on pure-pin payloads (`unity-editor`, `cli-tools`, `edb`).
- `UNITY_LINUX_IMAGE_DIGEST` must equal the `@sha256:` suffix of
  `UNITY_LINUX_IMAGE`, and `UNITY_WINDOWS_EDITOR_URL`/`_SHA256` must equal
  `stackpin.UnityWindowsInstallers["editor"]` — the tests assert both. No
  Unity image is cached (ADR-0073).

## Never cached (CI-002)

No `path`/`key`/`restore-keys` may cover licence or credential state:
`unity-lic`, `unity-cfg`, `unity-cache`, `Unity_lic.ulf`,
`~/.local/share/unity3d`, `~/.config/unity3d`, `~/.cache/unity3d`,
`ProgramData\Unity`, `.ulf` files. These are `$RUNNER_TEMP` dirs —
fresh every run, only bind-mounted into the Unity containers. Licence activation runs every attempt. A cache hit must never
skip a Q0-Q6 gate, the fork/freeze guards, the materialization retry loop,
the `commit unity-materialized` drift check, or the licence activation. The
ADR-0077 materialization snapshot `$RUNNER_TEMP/unity-lib-restored` is a
per-run retry copy, never a cache path.

## Telemetry (CI-003)

- Each cached step emits one JSONL line via `cache_telemetry.sh` /
  `cache_telemetry.ps1` to `$RUNNER_TEMP/cache-telemetry.jsonl`
  (`{step, result: hit|miss, wall_seconds}`). `RUNNER_TEMP` is outside the
  workspace so telemetry never dirties the tree (Q6 clean_tree).
- `scripts/verify.ps1` measures its own `go run` wall time, appends a `verify`
  entry (the ADR-0077 pre-Unity phase appends `verify-pre-unity` and never
  merges; the final phase does) (hit = `THINHTHAN_CACHE_HIT_GO`, set from the Go cache step output),
  then runs `server/internal/conformance/caching/cmd/cachemerge` to fold all
  entries into `verify-report.json` as `cached_steps[]`. The merge is
  best-effort — it can never fail verification.
- Evidence manifests (`gates.MergeReports`) decode reports into the fixed
  `VerifyReport` struct, so `cached_steps` is dropped before the manifest —
  evidence identity is cache-independent (CI-004).
- The Linux required job's evidence step (`verify.ps1 -MergeReports -Task`,
  ADR-0075) early-exits on branches whose head ref has no `IMP-\d+`
  (claim/ops/spec/status PRs): it skips *manifest generation* only, never a
  gate. No manifest on those branches is by design — not a failure.
- The `Unity (<os>)` job ships its `cache-telemetry.jsonl` inside
  `unity-test-results-<os>`; the required job appends it to its own telemetry
  before the verifier folds `cached_steps` (ADR-0075).

## Postgres service container

The Linux `services:` postgres container cannot be cached and stays a service
container — pulling `postgres:18.6` (~90 MB) is seconds; converting it to a
step-managed container would only save that pull, not worth the lifecycle
risk. The Windows EDB binaries ARE cached (large download, sha-asserted).

## Main-scope warming (ADR-0073)

Caches saved by a PR run are visible only to that PR. `.github/workflows/cache_warm.yml` saves the pure-pin caches (`unity-editor`, `cli-tools`, `edb`, `go-build`) on pushes to `main`; its cache steps must equal a `verify.yml` cache step byte-for-byte (key + path, `TestCacheWarmMirrorsVerifyCaches`). `unity-library` is warmed only by its `warm-library-*` jobs: exact key (no `restore-keys`, BLK-005), `lookup-only` so a hit downloads nothing, materialization identical to `verify.yml` on a miss; they alone read the Unity licence secrets, safe because the workflow never runs on `pull_request`. Keep-alive: `cache_warm.yml` also runs every 5 days (`schedule`, default branch only) and fully restores every main-scope cache (Library included: `lookup-only` is false on `schedule`) so the 7-day unused-cache eviction never fires; missing entries are re-created. `.github/workflows/cache_prune.yml` (push to `main` + hourly) deletes `refs/pull/<n>/merge` caches of closed PRs so closed-PR entries never crowd the repository cache budget: 10 GB is free; above that saves are billed and, with the configured spending budget, the cache turns read-only (observed 2026-09-28 at 10.8 GB: `Cache reservation failed: You have reached your configured budget`).
