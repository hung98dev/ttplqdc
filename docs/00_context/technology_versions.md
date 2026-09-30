# Technology Versions
status: LOCKED
verified_at: 2026-09-25

## Scope
Canonical exact launch toolchain and approved core dependency versions.

AI implementation agents **must use these versions**. Do not independently choose a newer/older version, substitute library, preview build, or additional framework.

Version-policy decision: `../11_decisions/0010-exact-technology-version-pinning.md`.

# Unity Client

| Component | Canonical version / setting | Rule |
|---|---|---|
| Unity Editor | `6000.6.1f1` | Exact editor pin = installed Hub editor. |
| Unity Hub | `3.21.3` | Developer install/bootstrap version. |
| C# language | `C# 9.0` | Use the language level supported by the pinned Unity editor; do not force a newer LangVersion. |
| Unity API compatibility | `.NET Standard 2.1` | Cross-platform project API profile. |
| Release scripting backend | `IL2CPP` | Production PC/mobile player builds unless a target platform explicitly requires another supported backend. |
| Render pipeline | `com.unity.render-pipelines.universal 17.6.0` | Canonical URP for Unity 6000.6.1f1 (editor PackageManager manifest). Do not override independently. |
| SRP Core | `com.unity.render-pipelines.core 17.6.0` | Core dependency aligned with URP 17.6.0 / Unity 6000.6.1f1. |
| Shader Graph | `com.unity.shadergraph 17.6.0` | Use only if shader authoring needs it; version remains editor/URP-aligned. |
| Input System | `com.unity.inputsystem 1.20.0` | Shared keyboard/mouse/gamepad/touch input layer. |
| 2D Animation | `com.unity.2d.animation 16.0.0` | Character rig/2D animation package for Unity 6000.6.1f1. |
| PSD Importer | `com.unity.2d.psdimporter 15.0.0` | Approved layered PSB/PSD authoring importer. |
| Addressables | `com.unity.addressables 2.11.2` | Canonical asset loading, bundle/catalog management, local/remote presentation delivery. |
| Localization | `com.unity.localization 1.5.12` | Canonical string/asset localization; `vi-VN` (default) and `en-US` required launch locales. |
| UI | `com.unity.ugui` editor-bound core package from `6000.6.1f1` (includes TextMeshPro) | Canvas UI per `../04_architecture/client_experience_contract.md`; resolved version locked by `packages-lock.json`; no UI Toolkit runtime UI and no separate `com.unity.textmeshpro` line (ADR-0059). |
| Protocol Buffers C# runtime | `Google.Protobuf 3.36.2` | Generated network/data messages only; this does not enable gRPC. Source: NuGet package `https://api.nuget.org/v3-flatcontainer/google.protobuf/3.36.2/google.protobuf.3.36.2.nupkg` (SHA-256 `1182590db175f9057707857a1df48b217226d0732716cd353fa4aa4683d38dcb`); IMP-000 verifies the hash and commits `lib/netstandard2.0/Google.Protobuf.dll` to `client/Assets/Plugins/Google.Protobuf/` (ADR-0072). |

Unity project lock requirements:
```text
ProjectSettings/ProjectVersion.txt -> 6000.6.1f1
Packages/manifest.json             -> exact direct package versions
Packages/packages-lock.json        -> committed transitive lock
```

Do not float the editor (`6000.6`, `6000.6.1`, `latest`). The pin is the installed editor `6000.6.1f1`.

Development/test packages are also pinned when used:

| Component | Canonical version | Rule |
|---|---|---|
| Unity Test Framework | editor-bound core package from `6000.6.1f1` | Do not install a preview/alternate Test Framework line; resolved version is locked by the editor/project package lock. |
| Performance Test Framework | `com.unity.test-framework.performance 6.6.0` | Canonical Unity performance regression package for this editor. |
| Memory Profiler | `com.unity.memoryprofiler 1.1.12` | Canonical memory snapshot/profiling package for client leak/mobile analysis. |
| Profile Analyzer | `com.unity.performance.profile-analyzer 1.4.0` | Canonical multi-frame CPU profile comparison tool. |

Unity core graphics packages are still editor-coupled: the exact project resolution in `Packages/packages-lock.json` must agree with these pins. AI agents must not change URP/SRP/Shader Graph independently to a newer package line.

Addressables `2.11.2` is the canonical content-delivery package for this launch branch. Localization `1.5.12` is the canonical presentation string/asset localization package. Do not replace it with raw AssetBundle code, a custom patcher, or a different Addressables line without updating ADR-0014 and this matrix.

# Backend

| Component | Canonical version | Rule |
|---|---|---|
| Go toolchain | `1.27.1` | Exact CI/developer/server build toolchain. |
| PostgreSQL | `18.6` | Production stable database. PostgreSQL 19 beta/prerelease is forbidden for launch. |
| pgx | `github.com/jackc/pgx/v5 v5.11.0` | Canonical PostgreSQL driver/pool. Prefer native pgx/pgxpool. |
| WebSocket | `github.com/coder/websocket v1.8.15` | Canonical Go WSS library. Do not substitute Gorilla/random WS package. |
| DB migrations | `github.com/golang-migrate/migrate/v4 v4.20.1` | Canonical schema migration tool/library. |
| Protocol Buffers Go runtime | `google.golang.org/protobuf v1.36.12` | Canonical protobuf runtime. |
| Protocol Buffers Go generator | `protoc-gen-go v1.36.12` | Generator must match this pin. |
| OpenTelemetry Go | `go.opentelemetry.io/otel v1.46.0` | Canonical observability API/SDK family. |
| OpenTelemetry HTTP instrumentation | `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0` | HTTP instrumentation pin. |
| OpenTelemetry SDK + OTLP exporters | `go.opentelemetry.io/otel/sdk v1.46.0`, `go.opentelemetry.io/otel/sdk/metric v1.46.0`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.46.0` | Metrics and traces over OTLP/HTTP to the local Collector (`../08_scale_ops/observability.md` § Launch Telemetry Stack, ADR-0066). Logs stay `log/slog` JSON to stdout (journald); no OTLP log exporter. |
| Unicode normalization / case folding | `golang.org/x/text v0.42.0` | Canonical NFC + Unicode case-fold implementation for authoritative player text. |
| Password hashing | `golang.org/x/crypto v0.57.0` | Argon2id only (`golang.org/x/crypto/argon2`, ADR-0051). Latest stable 2026-09-08; requires Go >= 1.26 and `golang.org/x/text v0.42.0` (matches the pin). Transitive `x/sys v0.48.0`, `x/term v0.46.0`, `x/net v0.58.0` come only from this module's go.mod and are locked in `go.sum`. |
| Unicode grapheme segmentation | `github.com/clipperhouse/uax29/v2 v2.7.0` | Canonical UAX #29 Unicode-17 grapheme segmentation/counting. |

Go standard-library defaults:
```text
HTTP server/client -> net/http
structured logging -> log/slog
context/deadlines -> context
crypto/TLS         -> crypto/* + crypto/tls
```

Do not add a third-party HTTP router, logger framework, ORM, DI framework, event bus, Redis client, Kafka client, generic service framework, or alternate Unicode/text-normalization library unless a canonical architecture change explicitly approves and pins it.

When implementation creates `go.mod`:
- it must target the canonical Go 1.27.1 toolchain,
- every direct module is exact in `go.mod`,
- `go.sum` is committed,
- CI fails on uncommitted module drift.

# Network / Code Generation

| Component | Canonical version |
|---|---|
| Protocol Buffers compiler `protoc` | `36.2` |
| C# protobuf runtime | `Google.Protobuf 3.36.2` |
| Go protobuf runtime | `google.golang.org/protobuf v1.36.12` |
| Go protobuf generator | `protoc-gen-go v1.36.12` |

Rules:
- committed `.proto` files are the wire-schema source of truth,
- generated C# and Go outputs must be reproducible from the pins above,
- generator/version drift fails CI,
- gRPC is **not selected**; do not add `Grpc.Tools`, grpc-go, or a gRPC service layer unless architecture is intentionally changed,
- gameplay transport remains WSS as defined in `../05_network/protocol.md`.

# Database

Production database:
```text
PostgreSQL 18.6
UTF-8
UTC server-owned timestamps
schema changes via golang-migrate 4.20.1
Go access via pgx/v5 5.11.0
```

Do not:
- deploy PostgreSQL beta/RC for production,
- use an ORM as an implicit alternative to pgx/explicit SQL,
- let migration tooling auto-upgrade itself,
- depend on extensions without explicitly pinning/approving them.


# CI Tooling

| Component | Canonical version | Rule |
|---|---|---|
| GitHub Actions `actions/checkout` | `v4.2.2` (`11bd71901bbe5b1630ceea73d27597364c9af683`) | Pin tag and commit SHA. Floating `@v4` forbidden. |
| GitHub Actions `actions/setup-go` | `v5.3.0` (`f111f3307d8850f501ac008e886eec1fd1932a34`) | Pin tag and commit SHA; exact Go `1.27.1`. |
| GitHub Actions `actions/upload-artifact` | `v4.6.2` (`ea165f8d65b6e75b540449e92b4886f43607fa02`) | Upload `verify-report.json`, `evidence`, `visual-review` and `unity-materialized-<os>` artifacts. Floating `@v4` forbidden. |
| GitHub Actions `actions/download-artifact` | `v4.3.0` (`d3f86a106a0bac45b974a628896c90dbdf5c8093`) | `evidence manifest` job downloads both OS verify reports of the same run (ADR-0072). Floating `@v4` forbidden. |
| Git LFS | `3.8.0` | Every CI checkout uses `actions/checkout` with `lfs: true`; CI installs `git-lfs-linux-amd64-v3.8.0.tar.gz` (SHA-256 `e455e00f15d9b95661b8d53498ffb0c3367962cf1ec73c31ab7369516cd6ab8d`) / `git-lfs-windows-amd64-v3.8.0.zip` (SHA-256 `b62e7b8ceddee635f691233d77de8eaa4b213e9209e0173811d8cfa77f7882c1`) from `https://github.com/git-lfs/git-lfs/releases/download/v3.8.0/` before checkout; local agent machines use the same version (`../10_implementation/repository_layout.md` § Git Attributes). |
| Server runtime packaging | static binary + systemd unit | No container base image in production (`../08_scale_ops/deployment.md`). |
| TLS root CA bundle | Mozilla via curl.se `cacert-2026-08-13.pem`, SHA-256 `f66dff1bdf8f96060b8177976f8b7d9254bc89bc4db933d769f7384d28480bc9` | Committed at `deploy/prod/cacert.pem`; verify hash in Q1; refresh only by updating this row. |
| GitHub Actions `actions/create-github-app-token` | `v3.2.0` (`bcd2ba49218906704ab6c1aa796996da409d3eb1`) | Merge-guard App token for post-merge revert PRs (ADR-0057, ADR-0058). |
| GitHub CLI `gh` | `2.101.0` | PR, auto-merge, run download, rulesets evidence. CI installs the release asset `gh_2.101.0_linux_amd64.tar.gz` (SHA-256 `9bca2d1c16825f109907a23307628a2f0698fbf99662b73a5cf0b020293072b8`) / `gh_2.101.0_windows_amd64.zip` (SHA-256 `bc6c814367b193cd8e713611d61e36013c0ef843b8f516458fe3eda039192794`) from `https://github.com/cli/cli/releases/download/v2.101.0/`; the runner's preinstalled `gh` is never used. |
| Git for Windows | `2.55.0.windows.5` | Local Windows agent machines only: Git + Git Bash for `.devin` hooks (not a CI verify/codegen wrapper). |
| `jq` | `1.8.2` | JSON in local hooks and CI scripts. CI installs `jq-linux-amd64` (SHA-256 `b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f`) / `jq-windows-amd64.exe` (SHA-256 `a6fc67fedaf9128a3309a1e2ebb8b986aeccf70122ee46d2cb4849e423f0c627`) from `https://github.com/jqlang/jq/releases/download/jq-1.8.2/`; the preinstalled `jq` is never used. |
| Google Cloud SDK `gcloud` | `586.0.0` | `gcloud firebase test android run --type game-loop` in the scheduled `device-perf` workflow. |
| PostgreSQL test server (Windows) | `postgresql-18.6-1-windows-x64-binaries.zip` (https://get.enterprisedb.com/postgresql/postgresql-18.6-1-windows-x64-binaries.zip) | Windows CI job and local Windows: official EDB binaries, SHA-256 `fbe23da234ee31547bf8a36d29dfd81e82b849df2d2b78d2eecb43d360252f8c` (343,808,005 bytes, verified 2026-09-25; asserted by `server/internal/stackpin/`); `verify.ps1` unpacks into ignored `tools/`, starts on a random port, exports `THINHTHAN_TEST_PG_DSN` unless it is already set. |
| GitHub-hosted runner images | `ubuntu-24.04`, `windows-2022` | Only runners allowed (ADR-0058). `*-latest`, self-hosted, GPU and larger runners are forbidden. `windows-2022` matches the ltsc2022 GameCI Windows images. |
| PowerShell | `7.6.6` (`pwsh`) | Runs `scripts/verify.ps1` / `scripts/codegen.ps1` on Linux and Windows; workflow steps use `shell: pwsh`. Windows PowerShell 5.1 is not a supported host. CI installs `powershell-7.6.6-linux-x64.tar.gz` (SHA-256 `ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc`) / `PowerShell-7.6.6-win-x64.zip` (SHA-256 `02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860`) from `https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/` in a `bash`/`cmd` bootstrap step before any `shell: pwsh` step and prepends it to `PATH`; the preinstalled `pwsh` is never used. |
| GitHub Actions `actions/cache` | `v6.1.0` (`55cc8345863c7cc4c66a329aec7e433d2d1c52a9`) | Unity `client/Library` cache per OS, keyed on `packages-lock.json` + `ProjectVersion.txt`. |
| GitHub Actions `game-ci/unity-test-runner` | `v4.3.2` (`fa6ced25861c16ef56187828c43f76d00df43a23`) | Unity EditMode/PlayMode in CI; `customImage` set to a digest-pinned image below. |
| GitHub Actions `game-ci/unity-builder` | `v6.0.0` (`eb1b9fba120c6e62c9fb7a7a81d6c107ce004c45`) | IL2CPP Windows/Android player builds (IMP-067). |
| Unity CI image (Linux tests, screenshots) | `unityci/editor:ubuntu-6000.6.1f1-base-3.2.2@sha256:2197a718c75ba71d6d9a05cfdfbce31cc401113f530963ac789160dffc96763d` | Linux job; xvfb + Mesa llvmpipe rendering. |
| Unity CI image (Android build) | `unityci/editor:ubuntu-6000.6.1f1-android-3.2.2@sha256:33f6f1056b02dcabd46ed9bfb8ff26aae241e0af412f9628bc06fc760df248ab` | Linux job; IL2CPP Android build. |
| Unity CI image (Windows tests) | `unityci/editor:windows-6000.6.1f1-base-3.2.2@sha256:a995b9d1d03dc08c1702f91acc05c64297217522aebb9387af7ce912331fb534` | Windows job. |
| Unity CI image (Windows player build) | `unityci/editor:windows-6000.6.1f1-windows-il2cpp-3.2.2@sha256:5bd80a61ac442b81745f653dd39395f6e93167ebc51c4b494bdd42c2b656195b` | Windows job; IL2CPP Windows player build. |
| PostgreSQL test container (Linux CI) | `postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722` | Service container of `Q0-Q6 verify (Linux)`; exports `THINHTHAN_TEST_PG_DSN`. Local Linux agent machines run the same digest via `docker run` when Docker exists, otherwise Q5 is `DEFERRED(local-missing)` under `verify.ps1 -LocalDeferMissing` (ADR-0072). Tests only; production stays container-free. |
| Staticcheck | `2026.2.1` (module `honnef.co/go/tools v0.8.1`, released 2026-08-21, supports Go 1.27) | Q4 `CODE-003`: installed by `go install honnef.co/go/tools/cmd/staticcheck@v0.8.1` (checksum-database verified) into an ignored tool dir; never added to `server/go.mod`. Default check set, no `staticcheck.conf` (`../10_implementation/engineering_conventions.md` §1.1). |
| Go race detector | Go `1.27.1` `-race` | Runs only in `Q0-Q6 verify (Linux)` (cgo + the runner's system C compiler); the Windows job runs the same Go tests without `-race`; no C toolchain is installed on Windows (ADR-0072). |
| GitHub merge queue | not used | Unavailable for user-owned repositories; merges are serialized by the coordinator's merge slot (`../10_implementation/agent_execution_protocol.md` §5a). |

CI runs only on GitHub-hosted `ubuntu-24.04` and `windows-2022` runners; each job installs the pinned Go, `pwsh`, `gh`, `jq`, Git LFS and `gcloud` and runs Unity in the digest-pinned GameCI images above (ADR-0058; Owner Setup in `../10_implementation/audit_gates.md`). Every Action is pinned by commit SHA and every container image by digest. Do not add unlisted tools (e.g. Python, unapproved linters) to CI workflows without recording ownership and pins in this matrix. C# style and the client API fence are checked by the Go verifier; no .NET SDK, Roslyn analyzer or C# formatter is pinned or installed (ADR-0059). `scripts/verify.ps1` must not call `python`.

## Content production tools (ADR-0072)

| Component | Canonical version | Rule |
|---|---|---|
| Art/audio generation tool | owner-provided | Required only by final-art tasks (`IMP-071`, `IMP-072`, `IMP-073`, `IMP-074`, `IMP-075`, `IMP-104`, `IMP-105`). The owner records tool name, exact version/model, access method and commercial terms here and in Owner Setup (`../10_implementation/audit_gates.md`). Until then those tasks are not ready; the first claim attempt opens a scoped `OPS-xxx` that blocks only them. Other tasks use the placeholder regime of `../07_content/presentation_asset_manifest.md` §4 until the art-final milestone (M10). |

# Production Operations (ADR-0066)

Hosts and operations tooling for the one-world production deployment (`../08_scale_ops/deployment.md`, `../08_scale_ops/observability.md`, `../08_scale_ops/backup_recovery.md`). Linux amd64 release tarballs are verified against the SHA-256 below before install; PGDG packages are installed with exact `=version` pins from `apt.postgresql.org` (`noble-pgdg`).

| Component | Canonical version | Install source / SHA-256 | Rule |
|---|---|---|---|
| Production host OS | Ubuntu Server 24.04 LTS (`noble`), amd64 | official image; security updates via `unattended-upgrades` | World host, PostgreSQL host and ops host. No container runtime or Kubernetes (ADR-0052). |
| PostgreSQL server package | `postgresql-18=18.6-1.pgdg24.04+2` | PGDG apt | Same server version as the matrix pin `18.6`. |
| pgBackRest | `2.59.1` (`pgbackrest=2.59.1-1.pgdg24.04+1`) | PGDG apt | WAL archiving, full/differential backups and PITR to the S3-compatible repository configured in Owner Setup. |
| OpenTelemetry Collector (contrib) | `0.161.0` | `otelcol-contrib_0.161.0_linux_amd64.tar.gz` `778c689efa681ff6e4722ce9f66b9b7f57c3ba009ab2e2b43dc2e0315862c731` | Runs on the world host; OTLP/HTTP receiver on `127.0.0.1:4318`; exports traces to its local file exporter (14-day rotation) and metrics to Prometheus. |
| Prometheus | `3.14.0` | `prometheus-3.14.0.linux-amd64.tar.gz` `f665c6da19eb7ba399c915d30c7d9793c9b417bf8a749b504bc470678631478d` | Ops host; scrapes the Collector, node_exporter and postgres_exporter; 90-day retention. |
| Alertmanager | `0.34.1` | `alertmanager-0.34.1.linux-amd64.tar.gz` `265b9d1e55ef0d5306a436018af6d2b686c2ce051f03d968f7464ecb1372a7e8` | Ops host; receivers `ops-critical`, `ops-warning`, `security-queue`. |
| Grafana OSS | `13.2.2` | `grafana-13.2.2.linux-amd64.tar.gz` `9662c838a09824fdb072e5f6fbdd45b62cf541b20f3d609ea5011e6e5f544c8f` | Ops host; the seven launch dashboards are provisioned from `deploy/prod/grafana/`. |
| node_exporter | `1.12.1` | `node_exporter-1.12.1.linux-amd64.tar.gz` `b51d8a76aa2a9156a55d501aca6276fae09e262259a5e4e831d2c2222f084e63` | Every host. |
| postgres_exporter | `0.20.1` | `postgres_exporter-0.20.1.linux-amd64.tar.gz` `89d4f7e7920cad48fdc3133f789556ef5253c330a9f5fdace3bdb6344c0a8b5a` | PostgreSQL host. |

Download sources (ADR-0070; the deploy step downloads exactly these URLs and verifies the SHA-256 above before unpacking):
```text
otelcol-contrib    https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.161.0/otelcol-contrib_0.161.0_linux_amd64.tar.gz
prometheus         https://github.com/prometheus/prometheus/releases/download/v3.14.0/prometheus-3.14.0.linux-amd64.tar.gz
alertmanager       https://github.com/prometheus/alertmanager/releases/download/v0.34.1/alertmanager-0.34.1.linux-amd64.tar.gz
grafana            https://dl.grafana.com/oss/release/grafana-13.2.2.linux-amd64.tar.gz
node_exporter      https://github.com/prometheus/node_exporter/releases/download/v1.12.1/node_exporter-1.12.1.linux-amd64.tar.gz
postgres_exporter  https://github.com/prometheus-community/postgres_exporter/releases/download/v0.20.1/postgres_exporter-0.20.1.linux-amd64.tar.gz
pgbackrest, postgresql-18   PGDG apt repository https://apt.postgresql.org/pub/repos/apt noble-pgdg (exact package versions above)
```

These binaries are operations infrastructure, not part of the `thinhthan-server` artifact; the server binary imports none of them. Their configuration files live in `deploy/prod/` and are checked by the IMP-048 release tests.

# Pinned Content System Constants

These constants are fixed at project initialization and must never change after any content using them has been shipped. Changing a namespace UUID retroactively invalidates every idempotency key previously derived from it.

| Constant | Value | Rule |
|---|---|---|
| `CONTENT_GRANT_NAMESPACE_UUID` | `f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b` | UUID v5 namespace for all deterministic content-grant idempotency keys (seasonal cosmetics, Atlas reward tiers, Guild Stone completions, and any future one-time content delivery). Generated once with `crypto/rand`. **Immutable** — changing this value breaks every previously issued grant key. Do not rotate, substitute, or regenerate. See `../06_data/ids.md` "Deterministic Content-Grant Idempotency Keys". |
| `SERVER_JOB_NAMESPACE_UUID` | `64d34c40-8657-462b-887f-5970db9eaa5f` | UUID v5 namespace for server-initiated job `operation_id`s (`../06_data/ids.md` § Operation IDs, ADR-0070). Generated once with `crypto/rand`. **Immutable** — never rotate. |

# Verified Stable Choices
As of `2026-09-20`, the matrix pins the Unity editor installed on the implementation machine: Unity `6000.6.1f1`. Go `1.27.1` remains the current stable 1.27 patch; PostgreSQL `18.6` is stable while PostgreSQL 19 remains beta. CI tooling rows added by ADR-0058 were verified on `2026-09-25`; `gh`/`jq`/`pwsh` release-asset SHA-256 values were read from the GitHub release API on `2026-09-25` (ADR-0068). Staticcheck `2026.2.1` (ADR-0059) was verified against the GitHub release and the Go module proxy on `2026-09-25`. Production Operations rows (ADR-0066) were verified on `2026-09-25` against each project's latest non-prerelease GitHub release, its published SHA-256 file, the Go module proxy (OTel modules) and the PGDG `noble-pgdg` package index.

# Version Verification
When refreshing this matrix, verify candidate versions against the technology vendor's official release channel or canonical package registry. Record a new `verified_at` date. Do not infer "best" from version number alone: production selects the newest compatible **stable/LTS** release after compatibility review, not preview/beta/RC merely because it is newer.

The repository pin remains authoritative until an explicit tested update commits a new value; discovering a newer release on the internet does not authorize an implementation agent to upgrade itself.

# Version Selection Rules

## Exact Means Exact
Forbidden examples:
```text
latest
*
1.x
>= 1.2
preview
beta
rc
nightly
main/master HEAD
unpinned Git URL
```

A native ecosystem may record transitive constraints internally, but repository-controlled direct dependencies/toolchains remain exact and the resolved lockfile is committed.

## New Dependencies
Before adding any dependency not listed here:
1. prove the standard library/current stack cannot reasonably cover the need,
2. define ownership/use in the relevant architecture spec,
3. select one exact stable version,
4. add it here,
5. add license/security/compatibility checks,
6. add/update tests.

AI agents must not solve a local coding task by silently expanding the technology stack.

## Upgrade Cadence
Check for updates intentionally:
- critical security fix: immediately evaluate,
- normal patch/minor: batch into explicit dependency-update work,
- Unity LTS editor: patch upgrades only after project/package/build smoke tests,
- Go/PostgreSQL major changes: explicit compatibility/migration review,
- no automatic production upgrade from a floating tag.

# Reproducibility Gate
A build/release fails when:
- toolchain version differs from this file,
- Unity project version differs,
- direct dependency pin differs,
- lockfile/module checksum drift is uncommitted,
- protobuf generated output differs after regeneration with pinned tools,
- a prerelease dependency appears without explicit approval,
- an unlisted runtime/framework/infrastructure dependency is introduced.

# Invariants
```text
Unity = 6000.6.1f1
Go = 1.27.1
PostgreSQL = 18.6
protoc = 36.2
no floating versions
no agent-selected dependency substitutions
version change is an explicit reviewed repository change
CONTENT_GRANT_NAMESPACE_UUID = f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b (pinned immutable; never rotate)
SERVER_JOB_NAMESPACE_UUID    = 64d34c40-8657-462b-887f-5970db9eaa5f (pinned immutable; never rotate)
```
