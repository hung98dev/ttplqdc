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
| OpenTelemetry SDK + OTLP exporters | `go.opentelemetry.io/otel/sdk v1.46.0`, `go.opentelemetry.io/otel/sdk/metric v1.46.0`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.46.0` | Metrics and traces over OTLP/HTTP to the local Collector (`../08_scale_ops/observability.md` § Launch Telemetry Stack, ADR-0066). Logs stay `log/slog` JSON to stdout (journald); no OTLP log exporter. Transitive dependency closure of the pinned OTLP HTTP exporters includes `go.opentelemetry.io/proto/otlp v1.11.0` and `google.golang.org/grpc v1.83.1` (type definitions only; ADR-0079); first-party direct gRPC service or client imports remain strictly forbidden. |
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
- gRPC is **not selected** for gameplay or internal communication; do not add `Grpc.Tools`, a gRPC service layer, or direct first-party `grpc-go` imports (the only permitted presence is the transitive type closure of the approved OTLP HTTP exporters: `google.golang.org/grpc v1.83.1` and `go.opentelemetry.io/proto/otlp v1.11.0`; ADR-0079),
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
| GitHub Actions `actions/upload-artifact` | `v4.6.2` (`ea165f8d65b6e75b540449e92b4886f43607fa02`) | Upload `verify-report.json`, `evidence`, `visual-review` and `unity-materialized-windows` artifacts. Floating `@v4` forbidden. |
| GitHub Actions `actions/download-artifact` | `v4.3.0` (`d3f86a106a0bac45b974a628896c90dbdf5c8093`) | `Q0-Q6 verify (Windows)` downloads `unity-test-results-windows` from its parallel Unity job and `verify-report-linux` from the Linux job to merge the evidence manifest (ADR-0072, ADR-0075, ADR-0078). Floating `@v4` forbidden. |
| Git LFS | `3.8.0` | Installed on both runners from release assets: `git-lfs-linux-amd64-v3.8.0.tar.gz` (SHA-256 `e455e00f15d9b95661b8d53498ffb0c3367962cf1ec73c31ab7369516cd6ab8d`) / `git-lfs-windows-amd64-v3.8.0.zip` (SHA-256 `b62e7b8ceddee635f691233d77de8eaa4b213e9209e0173811d8cfa77f7882c1`). Only `Unity (Windows)` checks out with `lfs: true` and restores cached `.git/lfs` objects; required verify jobs checkout with `lfs: false` and `GIT_LFS_SKIP_SMUDGE=1` (ADR-0078). |
| Server runtime packaging | static binary + systemd unit | No container base image in production (`../08_scale_ops/deployment.md`). |
| TLS root CA bundle | Mozilla via curl.se `cacert-2026-08-13.pem`, SHA-256 `f66dff1bdf8f96060b8177976f8b7d9254bc89bc4db933d769f7384d28480bc9` | Committed at `deploy/prod/cacert.pem`; verify hash in Q1; refresh only by updating this row. |
| GitHub Actions `actions/create-github-app-token` | `v3.2.0` (`bcd2ba49218906704ab6c1aa796996da409d3eb1`) | Merge-guard App token for post-merge revert PRs (ADR-0057, ADR-0058). |
| GitHub CLI `gh` | `2.101.0` | PR, auto-merge, run download, rulesets evidence. CI installs the release asset `gh_2.101.0_linux_amd64.tar.gz` (SHA-256 `9bca2d1c16825f109907a23307628a2f0698fbf99662b73a5cf0b020293072b8`) / `gh_2.101.0_windows_amd64.zip` (SHA-256 `bc6c814367b193cd8e713611d61e36013c0ef843b8f516458fe3eda039192794`) from `https://github.com/cli/cli/releases/download/v2.101.0/`; the runner's preinstalled `gh` is never used. |
| Git for Windows | `2.55.0.windows.5` | Local Windows agent machines only: Git + Git Bash for `.devin` hooks (not a CI verify/codegen wrapper). |
| `jq` | `1.8.2` | JSON in local hooks and CI scripts. CI installs `jq-linux-amd64` (SHA-256 `b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f`) / `jq-windows-amd64.exe` (SHA-256 `a6fc67fedaf9128a3309a1e2ebb8b986aeccf70122ee46d2cb4849e423f0c627`) from `https://github.com/jqlang/jq/releases/download/jq-1.8.2/`; the preinstalled `jq` is never used. |
| Google Cloud SDK `gcloud` | `586.0.0` | Scheduled `device-perf` workflow only: `google-cloud-cli-586.0.0-linux-x86_64.tar.gz` (SHA-256 `6c774c76793eedd501150b59da653610fbe3eaac169e822965b722de75a2f001`, 87,996,132 bytes) on `ubuntu-24.04` (ADR-0078). Standard PR jobs do not install gcloud. |
| PostgreSQL test server (Windows) | `postgresql-18.6-1-windows-x64-binaries.zip` (https://get.enterprisedb.com/postgresql/postgresql-18.6-1-windows-x64-binaries.zip) | Windows CI job and local Windows: official EDB binaries, SHA-256 `fbe23da234ee31547bf8a36d29dfd81e82b849df2d2b78d2eecb43d360252f8c` (343,808,005 bytes, verified 2026-09-25; asserted by `server/internal/stackpin/`); `verify.ps1` unpacks into ignored `tools/`, starts on a random port, exports `THINHTHAN_TEST_PG_DSN` unless it is already set. |
| GitHub-hosted runner images | `ubuntu-24.04`, `windows-2022` | Only runners allowed (ADR-0058). `*-latest`, self-hosted, GPU and larger runners are forbidden. Unity on `windows-2022` runs natively (ADR-0073); `ubuntu-24.04` runs Go/PostgreSQL only and never invokes Unity (ADR-0078). |
| PowerShell | `7.6.6` (`pwsh`) | Runs `scripts/verify.ps1` / `scripts/codegen.ps1` on Linux and Windows; workflow steps use `shell: pwsh`. Windows PowerShell 5.1 is not a supported host. CI installs `powershell-7.6.6-linux-x64.tar.gz` (SHA-256 `ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc`) / `PowerShell-7.6.6-win-x64.zip` (SHA-256 `02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860`) from `https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/` in a `bash`/`cmd` bootstrap step before any `shell: pwsh` step and prepends it to `PATH`; the preinstalled `pwsh` is never used. |
| GitHub Actions `actions/cache` | `v6.1.0` (`55cc8345863c7cc4c66a329aec7e433d2d1c52a9`) | Caches: `client/Library` (Windows only), Go build/mod, EDB, native Windows editor, Windows CLI installs, Windows Android module, and LFS objects; 30 GB repository budget (ADR-0078, `.devin/scripts/cache-policy.md`). |
| Unity Windows Android module (native install) | `https://download.unity3d.com/download_unity/7efac9f6c10e/TargetSupportInstaller/UnitySetup-Android-Support-for-Editor-6000.6.1f1.exe` SHA-256 `7fce3760578959becae2aadf80ecc788fd5cdfd9df0e8d45f60faeb643f97aa3` (1,324,518,752 bytes) | IMP-067 / IMP-096 Android IL2CPP player build on the native Windows editor (ADR-0078). Cached as `unity-android-module-Windows-<shas>`. |
| Android OpenJDK 17.0.18+8 | `https://download.unity3d.com/download_unity/open-jdk/open-jdk-win-x64/jdk17.0.18-8_15e8817d1f5db6db3571ebe7430ef37f7fa8e60e8ff6f3e18ca1cb4c29f78774.zip` SHA-256 `15e8817d1f5db6db3571ebe7430ef37f7fa8e60e8ff6f3e18ca1cb4c29f78774` (118,110,508 bytes) | Pinned Android OpenJDK for native Windows editor. Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/OpenJDK`. |
| Android SDK/NDK Tools package | `https://download.unity3d.com/download_unity/android-sdk-tools/1_5312bb398affd0d94b90d3780976e1a162aa91944ef6bb40c8feb10ad6cc360d.zip` SHA-256 `5312bb398affd0d94b90d3780976e1a162aa91944ef6bb40c8feb10ad6cc360d` (166 bytes) | Pinned SDK tools manifest placeholder. Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK`. |
| Android NDK r27c | `https://dl.google.com/android/repository/android-ndk-r27c-windows.zip` SHA-256 `27e49f11e0cee5800983d8af8f4acd5bf09987aa6f790d4439dda9f3643d2494` (781,511,249 bytes) | Pinned Android NDK for IL2CPP compilation. Extracted and renamed to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/NDK`. |
| Android SDK CMake 3.22.1 | `https://dl.google.com/android/repository/cmake-3.22.1-windows.zip` SHA-256 `c9a9d568452a20cf27d703cb21ef9529dd67bda6be048c4d4b884acb3ac3a2b8` (16,116,742 bytes) | Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK/cmake/3.22.1`. |
| Android SDK Build-Tools 36.0.0 | `https://dl.google.com/android/repository/build-tools_r36_windows.zip` SHA-256 `aa1095cb14d83e483818a748a2c06faaeb8e601561b06a356a119a1b2ca280d3` (58,699,878 bytes) | Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK/build-tools/36.0.0`. |
| Android SDK Platform-Tools 36.0.0 | `https://dl.google.com/android/repository/platform-tools_r36.0.0-win.zip` SHA-256 `12c2841f354e92a0eb2fd7bf6f0f9bf8538abce7bd6b060ac8349d6f6a61107c` (7,138,784 bytes) | Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK/platform-tools`. |
| Android SDK Platforms (API 34, 36, 37.0) | API 34: `platform-34-ext7_r02.zip` SHA-256 `5323311cc3e4ad614f0b8053c72b651726f3422448cedd39e48f00737cda8ad0`; API 36: `platform-36_r02.zip` SHA-256 `37607369a28c5b640b3a7998868d45898ebcb777565a0e85f9acf36f29631d2e`; API 37: `platform-37.0_r02.zip` SHA-256 `840b23e827f96e64aea4c89a1194aac3dc5f6bad37edb231c5c795d890330e8d` | Pinned platforms for Unity 6000.6.1f1 player targeting. Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK/platforms`. |
| Android SDK Command-Line Tools 16.0 (build 12266719) | `https://dl.google.com/android/repository/commandlinetools-win-12266719_latest.zip` SHA-256 `f9088c04a44f1f37a8a3a228a7663e11ae9445fa07529c96cef38acb985a88f3` (143,481,958 bytes) | Pinned cmdline-tools (the filename suffix `_latest.zip` is Google's immutable release asset naming for build 12266719; content is pinned by exact SHA-256, ADR-0078). Extracted to `{UNITY_PATH}/Editor/Data/PlaybackEngines/AndroidPlayer/SDK/cmdline-tools/16.0`. |
| Unity Windows editor (native install) | `https://download.unity3d.com/download_unity/7efac9f6c10e/Windows64EditorInstaller/UnitySetup64-6000.6.1f1.exe` SHA-256 `8884daa489c8708c17da571c46a869839dd7bbf45f69048bd4db5c6d054d5e36` | Windows job (ADR-0073): silent install into `${{ runner.temp }}/unity-editor`, directory cached by installer SHA-256; mirrored by `server/internal/stackpin` `UnityWindowsInstallers`. |
| Unity Windows IL2CPP module (native install) | `https://download.unity3d.com/download_unity/7efac9f6c10e/TargetSupportInstaller/UnitySetup-Windows-IL2CPP-Support-for-Editor-6000.6.1f1.exe` SHA-256 `03f0cadf1e54f3eb80bb59865e95bfd7725c9c3b94d22ed0f175a23ac5e5bde4` | IMP-067 Windows IL2CPP player build on the native editor (ADR-0073). |
| PostgreSQL test container (Linux CI) | `postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722` | Service container of `Q0-Q6 verify (Linux)`; exports `THINHTHAN_TEST_PG_DSN`. Local Linux agent machines run the same digest via `docker run` when Docker exists, otherwise Q5 is `DEFERRED(local-missing)` under `verify.ps1 -LocalDeferMissing` (ADR-0072). Tests only; production stays container-free. |
| Staticcheck | `2026.2.1` (module `honnef.co/go/tools v0.8.1`, released 2026-08-21, supports Go 1.27) | Q4 `CODE-003`: installed by `go install honnef.co/go/tools/cmd/staticcheck@v0.8.1` (checksum-database verified) into an ignored tool dir; never added to `server/go.mod`. Default check set, no `staticcheck.conf` (`../10_implementation/engineering_conventions.md` §1.1). |
| Go race detector | Go `1.27.1` `-race` | Runs only in `Q0-Q6 verify (Linux)` (cgo + the runner's system C compiler); the Windows job runs the same Go tests without `-race`; no C toolchain is installed on Windows (ADR-0072). |
| GitHub merge queue | not used | Unavailable for user-owned repositories; merges are serialized by the coordinator's merge slot (`../10_implementation/agent_execution_protocol.md` §5a). |

CI runs only on GitHub-hosted `ubuntu-24.04` and `windows-2022` runners; each job installs the pinned Go, `pwsh`, `gh`, `jq` and Git LFS; Unity runs natively only on `windows-2022` (ADR-0073, ADR-0078); `ubuntu-24.04` runs Go/PostgreSQL only and never invokes Unity or containerized editor images (ADR-0078; Owner Setup in `../10_implementation/audit_gates.md`). Every Action is pinned by commit SHA. Do not add unlisted tools (e.g. Python, unapproved linters) to CI workflows without recording ownership and pins in this matrix. C# style and the client API fence are checked by the Go verifier; no .NET SDK, Roslyn analyzer or C# formatter is pinned or installed (ADR-0059). `scripts/verify.ps1` must not call `python`.

## Content production tools (ADR-0072)

| Component | Canonical version | Rule |
|---|---|---|
| Art/audio generation tool | Direct AI Generation (In-Session Multimodal) | Approved by Owner (2026-09-30): AI agents generate visual assets directly in the task session using multimodal image generation with the exact model ID and version recorded per asset in provenance register fragments (`asset_source_register.json` / `fragments/*.json`), without external desktop/GUI tools. Access method: in-session agent execution. Commercial terms: commercial distribution permitted by the generating model terms. Every `AI_CREATED` record names tool/model, seed, prompt, and parameters. No external desktop/GUI tool is required; final-art tasks (`IMP-071`..`IMP-075`, `IMP-104`, `IMP-105`) are unblocked without `OPS-xxx`. |
| Art tool selection criteria (ADR-0076) | direct AI generation satisfies all | reference-image or style-adapter input; reproducible seed; PNG output with alpha; terms permitting commercial distribution; exact model/version identifiable. Style is locked by Style Packs (`../07_content/presentation_asset_manifest.md` §3.8); no LoRA training at launch. Animation uses the pinned PSD Importer 15.0.0 + 2D Animation 16.0.0 (skeletal) or frame-by-frame per §3.7; no normal/mask maps at launch. |

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
