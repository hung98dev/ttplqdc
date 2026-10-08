# Repository Layout and Path Ownership
status: LOCKED

## Scope

Canonical physical layout and the task allowed to materialize each path. Exact pins remain owned by `../00_context/technology_versions.md`.

A planned path is not permission to pre-scaffold it. Create it only while its owner task is `IN_PROGRESS`.

## Current Tree

No implementation task has started. The repository currently contains only:

```text
thinhthan/
├── .devin/
├── AGENTS.md
├── README.md
└── docs/
    ├── 00_context/ .. 11_decisions/
    └── templates/
```

Every path below, including `docs/10_implementation/evidence/<IMP>/`, is created by its owning `IMP-*` packet, starting with `IMP-000`.

## Planned Root Layout

```text
thinhthan/
├── .editorconfig  .gitattributes  .gitignore      # IMP-000
├── .github/
│   ├── pull_request_template.md                   # IMP-000
│   └── workflows/
│       ├── verify.yml                              # IMP-000 (on: pull_request); IMP-068 two-step cutover to pull_request_target
│       ├── post_merge_guard.yml                    # IMP-068
│       └── device_perf.yml                         # IMP-096 (scheduled Firebase Test Lab game-loop runs)
├── client/                                         # Unity 6000.6.1f1 project
├── deploy/
│   ├── prod/                                       # IMP-048 (systemd unit, pinned cacert.pem, runbooks; DSR runbook IMP-103)
│   └── load/                                       # IMP-046
├── docs/
├── proto/
│   ├── thinhthan/v1/                               # IMP-061: nine network schemas
│   ├── thinhthan/internal/v1/durable_journal.proto  # IMP-061: Go-only internal schema
│   └── testdata/golden/                            # IMP-061
├── scripts/
│   ├── verify.ps1                                  # IMP-000
│   ├── codegen.ps1                                 # IMP-061
│   ├── verify_client_build.ps1                     # IMP-067
│   └── device_perf.ps1                             # IMP-096
└── server/                                         # one Go module `thinhthan`
```

No `deploy/docker/`: PostgreSQL 18.6 tests use the digest-pinned `postgres:18.6` service container on the Linux CI job and the EDB Windows binaries on Windows (ADR-0058); `server/internal/testing/pgtest/` (IMP-005) connects through `THINHTHAN_TEST_PG_DSN`. Build output (Unity player builds, caches) goes to a directory outside the repository.

## Go Layout

```text
server/
├── go.mod  go.sum         # IMP-000
├── cmd/
│   ├── compiler/          # IMP-003, build-time tool
│   ├── migrate/           # IMP-005, operator tool
│   ├── server/            # IMP-006 bootstrap, IMP-069 final composition
│   └── verify/            # IMP-000, repository verifier
├── migrations/            # IMP-005 only; baseline 000001, immutable numbered pairs
└── internal/
    ├── app/               # IMP-069 production composition/lifecycle
    ├── conformance/       # gates+style IMP-000, taskgraph+architecture IMP-083, ratchet+trusted IMP-068, deviceperf IMP-096; caching/cmd/cachemerge IMP-106 (tool-only main exception)
    ├── stackpin/          # IMP-000
    ├── config/            # IMP-003/004; equipment IMP-026; validation/{balance,beast,drop} IMP-049/050/051
    ├── core/id/  core/rng/  # IMP-001, IMP-002
    ├── observability/     # core IMP-098, audit IMP-043
    ├── durable/           # db, idempotency, schema IMP-005; queue IMP-082; journal/v1 generated Go IMP-061; lockorder IMP-097; feature subpackages
    ├── edge/              # listener, heartbeat IMP-081; auth, session, router IMP-006; feature subpackages
    ├── global/            # runtime IMP-080 (in-process single writer); feature subpackages
    ├── protocol/v1/       # IMP-061 generated .pb.go; never hand-edit
    ├── sim/               # runtime, aoi, replication IMP-079; feature subpackages
    │   └── spatial/       # geometry, collision IMP-078; maps, parity, testdata IMP-062; capacity IMP-055
    └── testing/           # pgtest IMP-005, protocol IMP-061, fault IMP-044, load IMP-046, migration IMP-047, release IMP-048
```

`cmd/compiler`, `cmd/migrate`, and `cmd/verify` are build/operator tools, not production services. The only deployed server executable is:

```text
server/cmd/server -> thinhthan-server
```

`IMP-006` creates the minimum session-capable entry point. `IMP-069` is the serialized final owner that wires every completed Edge, Sim, Durable, and Global subsystem into that entry point and proves readiness, drain, shutdown, and restart behavior.

Do not add a second Go module, a role-specific production main, or an extra server binary.

## Unity Layout

```text
client/
├── BuildProfiles/                     # IMP-067 (Windows + Android IL2CPP)
├── Assets/
│   ├── AddressableAssetsData/         # IMP-063 (shared registry, see Ownership Rules)
│   ├── Art/
│   │   ├── Provenance/                # register + schema IMP-070, merged by IMP-076; fragments/<name>.json one per art packet; cultural_review.md IMP-074; terms/<fragment>/ per art packet (ADR-0076)
│   │   ├── StyleRef/<fragment>/        # Style Packs, LFS, not in build; one dir per art packet (ADR-0076)
│   │   ├── Actors/Players/            # IMP-071
│   │   ├── Actors/Creatures/          # IMP-104
│   │   ├── World/                     # IMP-072
│   │   ├── Instances/                 # IMP-105
│   │   ├── UI/  Items/  VFX/          # IMP-073
│   │   └── Cosmetics/                 # IMP-074
│   ├── Audio/                         # IMP-075
│   ├── Localization/                  # Settings/ + Tables/Core/ IMP-064; Tables/<Feature>/ per feature packet
│   ├── Notices/                       # IMP-076
│   ├── Plugins/Google.Protobuf/       # IMP-000 exact 3.36.2 runtime
│   ├── {Scripts,Tests}/**/csc.rsp     # IMP-000 — scoped per-asmdef (-warnaserror+ -nullable:enable); no root csc.rsp (ADR-0059)
│   ├── Scenes/
│   │   ├── Bootstrap/                 # IMP-067
│   │   ├── Collision/                 # IMP-062 collision-only authoring scenes (ServerGeometry)
│   │   ├── Review/                    # IMP-070 Visual Review scenes
│   │   ├── Perf/                      # IMP-095 hotspot scene
│   │   ├── World/                     # IMP-072, 24 normal-world scenes
│   │   └── Dungeons/  Finale/  Competitive/   # IMP-105
│   ├── Settings/Rendering/            # IMP-101
│   ├── Settings/Performance/          # IMP-095 shader warm-up collection, OverdrawCount test material
│   ├── Scripts/
│   │   ├── App/                       # IMP-067 composition root (asmdef IMP-000)
│   │   ├── Core/                      # all asmdefs IMP-000; Assets IMP-063 (Editor/AssetProduction IMP-070/076), Localization IMP-064,
│   │   │                              # Rendering IMP-101, Geometry (+ Editor exporter) IMP-062, Session + Runtime IMP-065, Input IMP-066,
│   │   │                              # Performance IMP-095, PerformanceDevice IMP-096
│   │   ├── Net/                       # IMP-065 (asmdef IMP-000)
│   │   ├── Protocol/                  # IMP-061 generated C# (asmdef + csc.rsp IMP-000); never hand-edit
│   │   ├── Systems/<Feature>/         # feature packets (asmdef IMP-000); Replication IMP-065, Camera IMP-066
│   │   └── UI/<Feature>/              # feature packets (asmdef IMP-000)
│   └── Tests/
│       ├── EditMode/<Feature>/        # one folder per packet (asmdef IMP-000)
│       └── PlayMode/<Feature>/        # one folder per packet; Harness/ IMP-065 (asmdef IMP-000)
├── Packages/                          # IMP-000
└── ProjectSettings/                   # IMP-000; QualitySettings.asset IMP-095
```

## Mandatory Assemblies

All 13 `.asmdef` files are authored by IMP-000 with exactly these references (name references, `autoReferenced: false`, `overrideReferences: true` where precompiled DLLs are listed); no later packet edits an asmdef (ADR-0068). An assembly whose folder has no script yet is valid by name. Folder paths are under `client/Assets/`. Each assembly's folder also carries an IMP-000-authored `csc.rsp` containing exactly `-warnaserror+` and `-nullable:enable` — compiler flags are scoped per-asmdef so they never reach `Library/PackageCache` package sources; there is no root `client/Assets/csc.rsp` (ADR-0059).

| Assembly | Folder | Platforms / constraints | References |
|---|---|---|---|
| `ThinhThan.Protocol` | `Scripts/Protocol/` | any | precompiled `Google.Protobuf.dll` only |
| `ThinhThan.Core` | `Scripts/Core/` | any | `Unity.InputSystem`, `Unity.RenderPipelines.Core.Runtime`, `Unity.RenderPipelines.Universal.Runtime`, `Unity.RenderPipelines.Universal.2D.Runtime` |
| `ThinhThan.Core.Assets` | `Scripts/Core/Assets/` | any | `ThinhThan.Core`, `Unity.Addressables`, `Unity.ResourceManager` |
| `ThinhThan.Core.Assets.Editor` | `Scripts/Core/Assets/Editor/` | Editor | `ThinhThan.Core`, `ThinhThan.Core.Assets`, `Unity.Addressables`, `Unity.Addressables.Editor`, `Unity.ResourceManager` |
| `ThinhThan.Core.Localization` | `Scripts/Core/Localization/` | any | `ThinhThan.Core`, `Unity.Localization`, `Unity.Addressables`, `Unity.ResourceManager` |
| `ThinhThan.Core.Localization.Editor` | `Scripts/Core/Localization/Editor/` | Editor | `ThinhThan.Core`, `ThinhThan.Core.Localization`, `Unity.Localization`, `Unity.Localization.Editor` |
| `ThinhThan.Core.Geometry.Editor` | `Scripts/Core/Geometry/Editor/` | Editor | `ThinhThan.Core` |
| `ThinhThan.Net` | `Scripts/Net/` | any | `ThinhThan.Core`, `ThinhThan.Protocol`; precompiled `Google.Protobuf.dll` |
| `ThinhThan.Systems` | `Scripts/Systems/` | any | `ThinhThan.Core`, `ThinhThan.Core.Assets`, `ThinhThan.Core.Localization`, `ThinhThan.Net`, `ThinhThan.Protocol`, `Unity.InputSystem`, `Unity.RenderPipelines.Core.Runtime`, `Unity.RenderPipelines.Universal.Runtime`, `Unity.2D.Animation.Runtime`; precompiled `Google.Protobuf.dll` |
| `ThinhThan.UI` | `Scripts/UI/` | any | `ThinhThan.Core`, `ThinhThan.Core.Assets`, `ThinhThan.Core.Localization`, `ThinhThan.Net`, `ThinhThan.Protocol`, `ThinhThan.Systems`, `Unity.InputSystem`, `UnityEngine.UI`, `Unity.TextMeshPro`; precompiled `Google.Protobuf.dll` |
| `ThinhThan.App` | `Scripts/App/` | any | every non-Editor `ThinhThan.*` assembly above, `Unity.InputSystem`, `Unity.RenderPipelines.Universal.Runtime`, `Unity.Addressables`, `Unity.ResourceManager`, `Unity.Localization`; precompiled `Google.Protobuf.dll` |
| `ThinhThan.Tests.EditMode` | `Tests/EditMode/` | Editor; define constraint `UNITY_INCLUDE_TESTS` | every `ThinhThan.*` assembly except `ThinhThan.App` and `ThinhThan.Tests.PlayMode`, every Unity package assembly listed above, `UnityEngine.TestRunner`, `UnityEditor.TestRunner`, `Unity.PerformanceTesting`; precompiled `nunit.framework.dll`, `Google.Protobuf.dll` |
| `ThinhThan.Tests.PlayMode` | `Tests/PlayMode/` | any; define constraint `UNITY_INCLUDE_TESTS` | every non-Editor `ThinhThan.*` assembly including `ThinhThan.App`, every non-Editor Unity package assembly listed above, `UnityEngine.TestRunner`, `Unity.PerformanceTesting`; precompiled `nunit.framework.dll`, `Google.Protobuf.dll` |

`ThinhThan.Protocol` contains generated protobuf code only and references no project assembly; `scripts/codegen.ps1` never touches its asmdef. `ThinhThan.App` is the composition root; only `ThinhThan.Tests.PlayMode` references it. Assembly references are acyclic.

## ProjectSettings Baseline

IMP-000 commits the `client/ProjectSettings/*.asset` files produced by the editor's first materialization in CI (`agent_execution_protocol.md` §4b, ADR-0072), then pre-declares every entry a later packet needs (ADR-0068). Only the assets referenced below use a path-derived GUID: the first 32 lowercase hex characters of SHA-256 over the asset's repository-relative path (UTF-8, `/` separators); the owning packet creates the asset with that GUID in its `.meta`. Every other GUID is editor-generated and committed as materialized. Only `QualitySettings.asset` (IMP-095) is edited later.

The `com.unity.addressableassets` config-object slot is pinned to the canonical settings asset: if a
package import hook creates `client/Assets/AddressableAssetsData/DefaultObject.asset` and repoints the slot,
the IMP-063 provisioner restores the slot to the canonical settings object and deletes the rogue asset
before materialization commits (ADR-0074); `unity-materialized-windows` must never contain `DefaultObject.asset`.

```text
EditorBuildSettings.asset  m_configObjects com.unity.addressableassets     -> client/Assets/AddressableAssetsData/AddressableAssetSettings.asset (IMP-063)
                           m_configObjects com.unity.localization.settings -> client/Assets/Localization/Settings/LocalizationSettings.asset (IMP-064)
GraphicsSettings.asset     m_CustomRenderPipeline                          -> client/Assets/Settings/Rendering/ThinhThanURP.asset (IMP-101)
TagManager.asset           tags: ServerGeometry (IMP-062 collision scenes)
ProjectSettings.asset      incremental GC on; Android Optimized Frame Pacing on (client_performance.md § Smoothness by Construction)
Physics2DSettings.asset    simulationMode = Script
```

## Protobuf Contract

Every source file under `proto/thinhthan/v1/` uses:

```protobuf
syntax = "proto3";
package thinhthan.v1;
option go_package = "thinhthan/internal/protocol/v1;protocolv1";
option csharp_namespace = "ThinhThan.Protocol.V1";
```

Generated destinations:

```text
Go: server/internal/protocol/v1/
C#: client/Assets/Scripts/Protocol/
```

Only `scripts/codegen.ps1` may regenerate them.
The Go-only journal source `proto/thinhthan/internal/v1/durable_journal.proto` and generated-only `server/internal/durable/journal/v1/durable_journal.pb.go` are owned by IMP-061. Package/options/closed field schema are in `../05_network/protobuf_conventions.md` §7. It imports network contracts one-way, has no C# output or network IDs, and is not a tenth network schema.


## Git Attributes

Binary art assets use LFS (Git LFS `3.8.0`) when IMP-000 creates `.gitattributes`. All CI jobs checkout with `lfs: false` and `GIT_LFS_SKIP_SMUDGE=1`; only `Unity (Windows)` restores the OID-keyed `.git/lfs` cache and runs `git lfs pull` for missing objects (ADR-0078). Unity YAML stays text and uses UnityYAMLMerge:

```gitattributes
*.png filter=lfs diff=lfs merge=lfs -text
*.psd filter=lfs diff=lfs merge=lfs -text
*.psb filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.fbx filter=lfs diff=lfs merge=lfs -text
*.bundle filter=lfs diff=lfs merge=lfs -text

*.unity merge=unityyamlmerge eol=lf
*.prefab merge=unityyamlmerge eol=lf
*.asset merge=unityyamlmerge eol=lf
*.meta merge=unityyamlmerge eol=lf
*.mat merge=unityyamlmerge eol=lf
```

Do not put `*.prefab`, `*.asset`, `*.meta`, or `*.unity` in LFS.

## Ownership Rules

- Import fences: `sim/` never imports pgx/SQL or `edge/`; `durable/` never imports `sim/`; `edge/` routes intent and never owns combat/value mutation; `global/` is in-process and persists through typed Durable interfaces; `app/` is wiring only; `protocol/v1/` is generated-only; handwritten protocol parity tests live under `testing/protocol/` (`architecture_conformance.md`).
- Every owned path has the owner(s) listed in § Path Ownership Index. Nested or equal ownership requires a transitive dependency, except the exact key-disjoint append registries in § Addressables Append Registry Grants; the earlier registry owner creates every baseline file. The exception is not general directory write permission.
- Go tests live in the package they test (`<package>/<name>_test.go`) and therefore inside the packet's owned directory.
- Unity tests live in `client/Assets/Tests/{EditMode|PlayMode}/<Feature>/`, one folder per packet, listed in its `owned_paths`. The root test asmdefs belong to IMP-000; `PlayMode/Harness/` belongs to IMP-065 and is read-only for other packets.
- Shared registries: `client/Assets/AddressableAssetsData/` is owned by IMP-063; a packet that depends on IMP-063 may append groups/entries only for keys it owns (append-only, key-owner checked by the IMP-063 validator). The append surface is granted by naming the exact registry files in the packet's `owned_paths` (Q0 `ownedFile` matches exact paths or `dir/` prefixes and implies `.meta`) together with a `depends_on` edge to the registry owner, which `paths.ownership_overlap` requires for any shared path; e.g. IMP-064 owns `AddressableAssetSettings.asset` plus the `AssetGroups/localization.*(.asset)` group and schema assets (ADR-0074). Localization string tables are per feature: the packet owning `client/Assets/Scripts/{Systems|UI}/<Feature>/` implicitly owns `client/Assets/Localization/Tables/<Feature>/`; `Tables/Core/` belongs to IMP-064.
- Module lockfiles: `server/go.mod`/`server/go.sum` are owned by IMP-000 and co-ownable — a packet whose code imports a module already pinned in `../00_context/technology_versions.md` lists both in `owned_paths` before landing its `require`/`go.sum` lines (every packet transitively depends on IMP-000, so `paths.ownership_overlap` ordering holds). Missing ownership is corrected in the packet and this index before implementation; a spec-change does not create runtime lockfiles in the docs-only baseline.
- `server/internal/sim/world/interact.go` stays wholly IMP-018-owned — the admitted `NPC_SERVICE` service set seals inside `NewInteractDispatcher`. Later services (IMP-020's `travel` and beyond) register post-construction through the exported `Runtime.Dispatcher()` accessor; the composition root (IMP-069, which transitively depends on each service packet) makes the `RegisterService` call in its wiring — no per-service co-ownership row, and `owned_paths` never lists the file outside IMP-018.
- Client inbound dispatch seam: `client/Assets/Scripts/Net/` (and `client/Assets/Tests/PlayMode/Harness/`) stays wholly IMP-065-owned — packets never list its files in `owned_paths`. The wire-id dispatch surfaces inside it are append surfaces keyed by S2C message id — `WireIds.cs` (constants), `EnvelopeCodec.cs` (`_s2cParsers` entries), `ReceiveQueue.cs` (`_replaceables`/`_barriers` membership), `SessionOrchestrator.cs` (`ApplyFrame` cases that hand the decoded frame to the packet's own Systems handler), and the harness emitters in `client/Assets/Tests/PlayMode/Harness/FakeServer.cs`. For every packet whose spec assigns it inbound S2C ids, the per-id binding across these files lands through a routed `fix(IMP-065)` gatefix PR that must merge before the consuming task's PR; the contract is one thin entry per id — no generic decoded-frame registry, orchestrator/queue mechanics unchanged.
- Provenance: `client/Assets/Art/Provenance/asset_source_register.json` is created empty by IMP-070 and merged by IMP-076 from `fragments/<name>.json`, each fragment owned by exactly one art packet.
- Evidence directories are implied by `evidence_location` only; no packet lists `docs/10_implementation/evidence/` in `owned_paths`.
- Unity `.meta` files are implied by ownership (ADR-0072): a packet owning `client/**` path P also owns `P.meta`, and the `.meta` of every folder it is the first to create; they are editor-materialized in CI (artifact `unity-materialized-windows`, ADR-0078) and committed byte-for-byte, never hand-written, except the path-derived GUIDs of § ProjectSettings Baseline.
- Only IMP-005 writes `server/migrations/`.
- Path ownership changes require updating this file and the owning task packet in the same change; Q0 checks parity.

### Addressables Append Registry Grants

IMP-063 materializes each canonical group and its schemas before consumers. Every row below grants only `client/Assets/AddressableAssetsData/AddressableAssetSettings.asset` and the exact `client/Assets/AddressableAssetsData/AssetGroups/<group>.asset` files expanded from its listed group names; corresponding `.meta` files are implied, but existing GUIDs remain immutable. These same exact files MUST appear in the packet and Path Ownership Index.

| Packet | Exact group names | Owned entry scope |
|---|---|---|
| IMP-071 | `shared.local` | Five class/player actor keys |
| IMP-072 | `region.lang_da`, `region.rung_u_minh`, `region.ben_nuoc_den`, `region.deo_may`, `region.thanh_co`, `region.nui_thieng` | World scene/tile/prop/parallax keys |
| IMP-073 | `bootstrap.local`, `shared.local`, `icons.shared` | UI/font/item/equipment/skill/status icon and skill VFX keys |
| IMP-074 | `cosmetic.shared`, `icons.shared` | Cosmetic visual/icon keys |
| IMP-075 | `shared.local`, `audio.bgm.lang_da`, `audio.bgm.rung_u_minh`, `audio.bgm.ben_nuoc_den`, `audio.bgm.deo_may`, `audio.bgm.thanh_co`, `audio.bgm.nui_thieng`, `audio.bgm.shared` | SFX/BGM keys |
| IMP-104 | `beast.shared`, `region.lang_da`, `region.rung_u_minh`, `region.ben_nuoc_den`, `region.deo_may`, `region.thanh_co`, `region.nui_thieng`, `dungeon.dinh_lang_bo_hoang`, `dungeon.mieu_ba_trong_rung`, `dungeon.xom_chim`, `dungeon.hang_ma_tranh`, `dungeon.den_tran`, `dungeon.finale` | Monster/boss/beast/NPC actor, required portrait and actor-only VFX keys |
| IMP-105 | `dungeon.dinh_lang_bo_hoang`, `dungeon.mieu_ba_trong_rung`, `dungeon.xom_chim`, `dungeon.hang_ma_tranh`, `dungeon.den_tran`, `dungeon.finale`, `pvp.shared` | Instance scene/tile/prop/parallax and encounter telegraph keys |
| IMP-076 | `shared.local` | Only `asset.ui.credits.text` |

Q0 allows unordered owners on these exact files only because entry GUIDs and addresses are disjoint and checked against the catalog/key owner. Relative to `main`, a producer may append its own entries only: no deletion/reassignment of another owner's entry, no settings/profile/schema change, no group schema/reference/GUID rewrite, and no group creation/rename. IMP-064's package-managed localization entries retain their existing exact grants and disjoint namespace. A duplicate GUID/address or foreign-key edit fails closed. Review scenes remain IMP-070-owned and data-driven from keys/catalog/provenance; producers do not edit them. The IMP-063 validator verifies the semantic registry diff, not merely filenames.


## Path Ownership Index

Generated from `task_queue.md` `owned_paths`.

| Path | Owner |
|---|---|
| `.devin/scripts/cache-policy.md` | IMP-106 |
| `.devin/scripts/cache_telemetry.ps1` | IMP-106 |
| `.devin/scripts/cache_telemetry.sh` | IMP-106 |
| `.editorconfig` | IMP-000 |
| `.gitattributes` | IMP-000 |
| `.github/pull_request_template.md` | IMP-000 |
| `.github/workflows/cache_prune.yml` | IMP-106 |
| `.github/workflows/cache_warm.yml` | IMP-106 |
| `.github/workflows/device_perf.yml` | IMP-096 |
| `.github/workflows/post_merge_guard.yml` | IMP-068 |
| `.github/workflows/verify.yml` | IMP-000, IMP-068, IMP-106 |
| `.gitignore` | IMP-000 |
| `client/Assets/AddressableAssetsData/` | IMP-063 |
| `client/Assets/AddressableAssetsData/AddressableAssetSettings.asset` | IMP-064, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.locales_BundledAssetGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.locales_ContentUpdateGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.shared_BundledAssetGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.shared_ContentUpdateGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.strings.en_us_BundledAssetGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.strings.en_us_ContentUpdateGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.strings.vi_vn_BundledAssetGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/Schemas/localization.strings.vi_vn_ContentUpdateGroupSchema.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.ben_nuoc_den.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.deo_may.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.lang_da.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.nui_thieng.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.rung_u_minh.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.shared.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/audio.bgm.thanh_co.asset` | IMP-075 |
| `client/Assets/AddressableAssetsData/AssetGroups/beast.shared.asset` | IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/bootstrap.local.asset` | IMP-073 |
| `client/Assets/AddressableAssetsData/AssetGroups/cosmetic.shared.asset` | IMP-074 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.den_tran.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.dinh_lang_bo_hoang.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.finale.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.hang_ma_tranh.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.mieu_ba_trong_rung.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/dungeon.xom_chim.asset` | IMP-104, IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/icons.shared.asset` | IMP-073, IMP-074 |
| `client/Assets/AddressableAssetsData/AssetGroups/localization.locales.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/localization.shared.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/localization.strings.en_us.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/localization.strings.vi_vn.asset` | IMP-064 |
| `client/Assets/AddressableAssetsData/AssetGroups/pvp.shared.asset` | IMP-105 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.ben_nuoc_den.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.deo_may.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.lang_da.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.nui_thieng.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.rung_u_minh.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/region.thanh_co.asset` | IMP-072, IMP-104 |
| `client/Assets/AddressableAssetsData/AssetGroups/shared.local.asset` | IMP-071, IMP-073, IMP-075, IMP-076 |
| `client/Assets/Art/Actors/Creatures/` | IMP-104 |
| `client/Assets/Art/Actors/Npcs/` | IMP-104 |
| `client/Assets/Art/Actors/Players/` | IMP-071 |
| `client/Assets/Art/Cosmetics/` | IMP-074 |
| `client/Assets/Art/Instances/` | IMP-105 |
| `client/Assets/Art/Items/` | IMP-073 |
| `client/Assets/Art/Provenance/asset_rights_review.md` | IMP-076 |
| `client/Assets/Art/Provenance/asset_source_register.json` | IMP-070, IMP-076 |
| `client/Assets/Art/Provenance/cultural_review.md` | IMP-074 |
| `client/Assets/Art/Provenance/fragments/actors_creatures.json` | IMP-104 |
| `client/Assets/Art/Provenance/fragments/actors_players.json` | IMP-071 |
| `client/Assets/Art/Provenance/fragments/audio.json` | IMP-075 |
| `client/Assets/Art/Provenance/fragments/cosmetics.json` | IMP-074 |
| `client/Assets/Art/Provenance/fragments/instances.json` | IMP-105 |
| `client/Assets/Art/Provenance/fragments/interface.json` | IMP-073 |
| `client/Assets/Art/Provenance/fragments/world.json` | IMP-072 |
| `client/Assets/Art/Provenance/register.schema.json` | IMP-070 |
| `client/Assets/Art/Provenance/terms/actors_creatures/` | IMP-104 |
| `client/Assets/Art/Provenance/terms/actors_players/` | IMP-071 |
| `client/Assets/Art/Provenance/terms/audio/` | IMP-075 |
| `client/Assets/Art/Provenance/terms/cosmetics/` | IMP-074 |
| `client/Assets/Art/Provenance/terms/instances/` | IMP-105 |
| `client/Assets/Art/Provenance/terms/interface/` | IMP-073 |
| `client/Assets/Art/Provenance/terms/world/` | IMP-072 |
| `client/Assets/Art/StyleRef/actors_creatures/` | IMP-104 |
| `client/Assets/Art/StyleRef/actors_players/` | IMP-071 |
| `client/Assets/Art/StyleRef/cosmetics/` | IMP-074 |
| `client/Assets/Art/StyleRef/instances/` | IMP-105 |
| `client/Assets/Art/StyleRef/interface/` | IMP-073 |
| `client/Assets/Art/StyleRef/world/` | IMP-072 |
| `client/Assets/Art/UI/` | IMP-073 |
| `client/Assets/Art/VFX/` | IMP-073 |
| `client/Assets/Art/World/` | IMP-072 |
| `client/Assets/Audio/` | IMP-075 |
| `client/Assets/DefaultVolumeProfile.asset` | IMP-000 |
| `client/Assets/DefaultVolumeProfile.asset.meta` | IMP-000 |
| `client/Assets/Localization/Settings/` | IMP-064 |
| `client/Assets/Localization/Tables/Core/` | IMP-064 |
| `client/Assets/Notices/THIRD_PARTY_ASSETS.txt` | IMP-076 |
| `client/Assets/Plugins/Google.Protobuf/` | IMP-000 |
| `client/Assets/Scenes/Bootstrap/` | IMP-067 |
| `client/Assets/Scenes/Collision/` | IMP-062 |
| `client/Assets/Scenes/Competitive/` | IMP-105 |
| `client/Assets/Scenes/Dungeons/` | IMP-105 |
| `client/Assets/Scenes/Finale/` | IMP-105 |
| `client/Assets/Scenes/Perf/` | IMP-095 |
| `client/Assets/Scenes/Review/` | IMP-070 |
| `client/Assets/Scenes/World/` | IMP-072 |
| `client/Assets/Scripts/App/` | IMP-067 |
| `client/Assets/Scripts/App/ThinhThan.App.asmdef` | IMP-000 |
| `client/Assets/Scripts/App/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Assets/` | IMP-063 |
| `client/Assets/Scripts/Core/Assets/Editor/AssetProduction/` | IMP-070 |
| `client/Assets/Scripts/Core/Assets/Editor/AssetProduction/ReleaseAssetAudit.cs` | IMP-076 |
| `client/Assets/Scripts/Core/Assets/Editor/GraphicsCapabilityProbe.cs` | IMP-000 |
| `client/Assets/Scripts/Core/Assets/Editor/ThinhThan.Core.Assets.Editor.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/Assets/Editor/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Assets/ThinhThan.Core.Assets.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/Assets/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Geometry/` | IMP-062 |
| `client/Assets/Scripts/Core/Geometry/Editor/ThinhThan.Core.Geometry.Editor.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/Geometry/Editor/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Input/` | IMP-066 |
| `client/Assets/Scripts/Core/Localization/` | IMP-064 |
| `client/Assets/Scripts/Core/Localization/Editor/ThinhThan.Core.Localization.Editor.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/Localization/Editor/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Localization/ThinhThan.Core.Localization.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/Localization/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Core/Performance/` | IMP-095 |
| `client/Assets/Scripts/Core/PerformanceDevice/` | IMP-096 |
| `client/Assets/Scripts/Core/Rendering/` | IMP-101 |
| `client/Assets/Scripts/Core/Runtime/` | IMP-065 |
| `client/Assets/Scripts/Core/Session/` | IMP-065 |
| `client/Assets/Scripts/Core/ThinhThan.Core.asmdef` | IMP-000 |
| `client/Assets/Scripts/Core/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Net/` | IMP-065 |
| `client/Assets/Scripts/Net/ThinhThan.Net.asmdef` | IMP-000 |
| `client/Assets/Scripts/Net/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Protocol/` | IMP-061 |
| `client/Assets/Scripts/Protocol/ThinhThan.Protocol.asmdef` | IMP-000 |
| `client/Assets/Scripts/Protocol/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/Systems/Atlas/` | IMP-060 |
| `client/Assets/Scripts/Systems/Auction/` | IMP-030 |
| `client/Assets/Scripts/Systems/Beasts/` | IMP-057 |
| `client/Assets/Scripts/Systems/Bosses/` | IMP-022 |
| `client/Assets/Scripts/Systems/Bosses/Relics/` | IMP-091 |
| `client/Assets/Scripts/Systems/Camera/` | IMP-066 |
| `client/Assets/Scripts/Systems/Character/` | IMP-065 |
| `client/Assets/Scripts/Systems/Classes/` | IMP-017 |
| `client/Assets/Scripts/Systems/Combat/` | IMP-014 |
| `client/Assets/Scripts/Systems/Cosmetics/` | IMP-038 |
| `client/Assets/Scripts/Systems/Crafting/` | IMP-027 |
| `client/Assets/Scripts/Systems/Death/` | IMP-084 |
| `client/Assets/Scripts/Systems/Dungeons/` | IMP-023 |
| `client/Assets/Scripts/Systems/Dungeons/Endgame/` | IMP-024 |
| `client/Assets/Scripts/Systems/Effects/` | IMP-016 |
| `client/Assets/Scripts/Systems/Equipment/` | IMP-012 |
| `client/Assets/Scripts/Systems/Formations/` | IMP-033 |
| `client/Assets/Scripts/Systems/Guild/` | IMP-036 |
| `client/Assets/Scripts/Systems/GuildStorage/` | IMP-037 |
| `client/Assets/Scripts/Systems/GuildWar/` | IMP-042 |
| `client/Assets/Scripts/Systems/Inventory/` | IMP-009 |
| `client/Assets/Scripts/Systems/LifeSkills/Cooking/` | IMP-059 |
| `client/Assets/Scripts/Systems/LifeSkills/Fishing/` | IMP-058 |
| `client/Assets/Scripts/Systems/Meridian/` | IMP-032 |
| `client/Assets/Scripts/Systems/Monsters/` | IMP-019 |
| `client/Assets/Scripts/Systems/Movement/` | IMP-013 |
| `client/Assets/Scripts/Systems/NpcServices/` | IMP-028 |
| `client/Assets/Scripts/Systems/Party/` | IMP-035 |
| `client/Assets/Scripts/Systems/Progression/` | IMP-011 |
| `client/Assets/Scripts/Systems/Pvp/` | IMP-039 |
| `client/Assets/Scripts/Systems/Pvp/Arena/` | IMP-041 |
| `client/Assets/Scripts/Systems/Pvp/Duel/` | IMP-040 |
| `client/Assets/Scripts/Systems/Pvp/Sparring/` | IMP-087 |
| `client/Assets/Scripts/Systems/Quests/` | IMP-021 |
| `client/Assets/Scripts/Systems/Replication/` | IMP-065 |
| `client/Assets/Scripts/Systems/Rewards/` | IMP-010 |
| `client/Assets/Scripts/Systems/Seasons/` | IMP-052 |
| `client/Assets/Scripts/Systems/Skills/` | IMP-015 |
| `client/Assets/Scripts/Systems/Social/` | IMP-034 |
| `client/Assets/Scripts/Systems/Souls/` | IMP-031 |
| `client/Assets/Scripts/Systems/Store/` | IMP-102 |
| `client/Assets/Scripts/Systems/ThinhThan.Systems.asmdef` | IMP-000 |
| `client/Assets/Scripts/Systems/Trade/` | IMP-029 |
| `client/Assets/Scripts/Systems/WeaponGlow/` | IMP-088 |
| `client/Assets/Scripts/Systems/World/` | IMP-018 |
| `client/Assets/Scripts/Systems/WorldEvents/` | IMP-025 |
| `client/Assets/Scripts/Systems/csc.rsp` | IMP-000 |
| `client/Assets/Scripts/UI/Account/` | IMP-103 |
| `client/Assets/Scripts/UI/Atlas/` | IMP-060 |
| `client/Assets/Scripts/UI/Auction/` | IMP-030 |
| `client/Assets/Scripts/UI/Beasts/` | IMP-057 |
| `client/Assets/Scripts/UI/Books/` | IMP-090 |
| `client/Assets/Scripts/UI/Bosses/` | IMP-022 |
| `client/Assets/Scripts/UI/Character/` | IMP-065 |
| `client/Assets/Scripts/UI/Chivalry/` | IMP-086 |
| `client/Assets/Scripts/UI/CoreHud/` | IMP-066 |
| `client/Assets/Scripts/UI/Cosmetics/` | IMP-038 |
| `client/Assets/Scripts/UI/Crafting/` | IMP-027 |
| `client/Assets/Scripts/UI/Discovery/` | IMP-020 |
| `client/Assets/Scripts/UI/Dungeons/` | IMP-023 |
| `client/Assets/Scripts/UI/Equipment/` | IMP-012 |
| `client/Assets/Scripts/UI/Feats/` | IMP-085 |
| `client/Assets/Scripts/UI/Formations/` | IMP-033 |
| `client/Assets/Scripts/UI/Guild/` | IMP-036 |
| `client/Assets/Scripts/UI/GuildStone/` | IMP-093 |
| `client/Assets/Scripts/UI/GuildStorage/` | IMP-037 |
| `client/Assets/Scripts/UI/GuildWar/` | IMP-042 |
| `client/Assets/Scripts/UI/Inventory/` | IMP-009 |
| `client/Assets/Scripts/UI/LifeSkills/Cooking/` | IMP-059 |
| `client/Assets/Scripts/UI/LifeSkills/Fishing/` | IMP-058 |
| `client/Assets/Scripts/UI/Meridian/` | IMP-032 |
| `client/Assets/Scripts/UI/Party/` | IMP-035 |
| `client/Assets/Scripts/UI/Progression/` | IMP-011 |
| `client/Assets/Scripts/UI/Pvp/` | IMP-039 |
| `client/Assets/Scripts/UI/Pvp/Arena/` | IMP-041 |
| `client/Assets/Scripts/UI/Pvp/Duel/` | IMP-040 |
| `client/Assets/Scripts/UI/Quests/` | IMP-021 |
| `client/Assets/Scripts/UI/Quests/Bounty/` | IMP-089 |
| `client/Assets/Scripts/UI/Rewards/` | IMP-010 |
| `client/Assets/Scripts/UI/Screens/` | IMP-099 |
| `client/Assets/Scripts/UI/Seasons/` | IMP-052 |
| `client/Assets/Scripts/UI/Shops/` | IMP-028 |
| `client/Assets/Scripts/UI/Skills/` | IMP-015 |
| `client/Assets/Scripts/UI/Social/` | IMP-034 |
| `client/Assets/Scripts/UI/Souls/` | IMP-031 |
| `client/Assets/Scripts/UI/StateMachine/` | IMP-066 |
| `client/Assets/Scripts/UI/Store/` | IMP-102 |
| `client/Assets/Scripts/UI/ThinhThan.UI.asmdef` | IMP-000 |
| `client/Assets/Scripts/UI/Trade/` | IMP-029 |
| `client/Assets/Scripts/UI/WorldEvents/` | IMP-025 |
| `client/Assets/Scripts/UI/csc.rsp` | IMP-000 |
| `client/Assets/Settings/Performance/` | IMP-095 |
| `client/Assets/Settings/Rendering/` | IMP-101 |
| `client/Assets/Tests/EditMode/AddressablesValidation/` | IMP-063 |
| `client/Assets/Tests/EditMode/AssemblyGraph/` | IMP-000 |
| `client/Assets/Tests/EditMode/AssetProvenance/` | IMP-070 |
| `client/Assets/Tests/EditMode/AudioAssetCoverage/` | IMP-075 |
| `client/Assets/Tests/EditMode/ClassPresentation/` | IMP-017 |
| `client/Assets/Tests/EditMode/CosmeticArtCoverage/` | IMP-074 |
| `client/Assets/Tests/EditMode/CreatureArtCoverage/` | IMP-104 |
| `client/Assets/Tests/EditMode/CutoutQualityGate/` | IMP-070 |
| `client/Assets/Tests/EditMode/FormationUi/` | IMP-033 |
| `client/Assets/Tests/EditMode/FrameRuntime/` | IMP-065 |
| `client/Assets/Tests/EditMode/GeometryExporter/` | IMP-062 |
| `client/Assets/Tests/EditMode/InstanceArtCoverage/` | IMP-105 |
| `client/Assets/Tests/EditMode/InterfaceArtCoverage/` | IMP-073 |
| `client/Assets/Tests/EditMode/LocalizationValidation/` | IMP-064 |
| `client/Assets/Tests/EditMode/MeridianUi/` | IMP-032 |
| `client/Assets/Tests/EditMode/PerformanceBudgets/` | IMP-095 |
| `client/Assets/Tests/EditMode/PlayerArtCoverage/` | IMP-071 |
| `client/Assets/Tests/EditMode/ProgressionPresentation/` | IMP-011 |
| `client/Assets/Tests/EditMode/ProtocolParity/` | IMP-061 |
| `client/Assets/Tests/EditMode/ReleaseAssetAudit/` | IMP-076 |
| `client/Assets/Tests/EditMode/RenderingSetup/` | IMP-101 |
| `client/Assets/Tests/EditMode/ThinhThan.Tests.EditMode.asmdef` | IMP-000 |
| `client/Assets/Tests/EditMode/VolumeDepthGate/` | IMP-070 |
| `client/Assets/Tests/EditMode/WeaponGlow/` | IMP-088 |
| `client/Assets/Tests/EditMode/WorldArtCoverage/` | IMP-072 |
| `client/Assets/Tests/EditMode/csc.rsp` | IMP-000 |
| `client/Assets/Tests/PlayMode/AccountUi/` | IMP-103 |
| `client/Assets/Tests/PlayMode/AppComposition/` | IMP-067 |
| `client/Assets/Tests/PlayMode/ArenaUi/` | IMP-041 |
| `client/Assets/Tests/PlayMode/AtlasUi/` | IMP-060 |
| `client/Assets/Tests/PlayMode/AuctionUi/` | IMP-030 |
| `client/Assets/Tests/PlayMode/BooksUi/` | IMP-090 |
| `client/Assets/Tests/PlayMode/BossPresentation/` | IMP-022 |
| `client/Assets/Tests/PlayMode/BountyUi/` | IMP-089 |
| `client/Assets/Tests/PlayMode/CameraFollow/` | IMP-066 |
| `client/Assets/Tests/PlayMode/CharacterLifecycleClient/` | IMP-065 |
| `client/Assets/Tests/PlayMode/ChivalryUi/` | IMP-086 |
| `client/Assets/Tests/PlayMode/CombatPresentation/` | IMP-014 |
| `client/Assets/Tests/PlayMode/CookingBonfirePresentation/` | IMP-059 |
| `client/Assets/Tests/PlayMode/CosmeticsUi/` | IMP-038 |
| `client/Assets/Tests/PlayMode/CraftingUi/` | IMP-027 |
| `client/Assets/Tests/PlayMode/DeathPresentation/` | IMP-084 |
| `client/Assets/Tests/PlayMode/DiscoveryPresentation/` | IMP-020 |
| `client/Assets/Tests/PlayMode/DuelUi/` | IMP-040 |
| `client/Assets/Tests/PlayMode/DungeonPresentation/` | IMP-023 |
| `client/Assets/Tests/PlayMode/EffectPresentation/` | IMP-016 |
| `client/Assets/Tests/PlayMode/EndgameDungeonPresentation/` | IMP-024 |
| `client/Assets/Tests/PlayMode/EquipmentUi/` | IMP-012 |
| `client/Assets/Tests/PlayMode/FeatsUi/` | IMP-085 |
| `client/Assets/Tests/PlayMode/FishingPresentation/` | IMP-058 |
| `client/Assets/Tests/PlayMode/GuildStoneUi/` | IMP-093 |
| `client/Assets/Tests/PlayMode/GuildStorageUi/` | IMP-037 |
| `client/Assets/Tests/PlayMode/GuildUi/` | IMP-036 |
| `client/Assets/Tests/PlayMode/GuildWarUi/` | IMP-042 |
| `client/Assets/Tests/PlayMode/Harness/` | IMP-065 |
| `client/Assets/Tests/PlayMode/InputHudStateMachine/` | IMP-066 |
| `client/Assets/Tests/PlayMode/InventoryPanel/` | IMP-009 |
| `client/Assets/Tests/PlayMode/MonsterPresentation/` | IMP-019 |
| `client/Assets/Tests/PlayMode/MovementPrediction/` | IMP-013 |
| `client/Assets/Tests/PlayMode/NetReceive/` | IMP-065 |
| `client/Assets/Tests/PlayMode/PartyUi/` | IMP-035 |
| `client/Assets/Tests/PlayMode/Performance/` | IMP-095 |
| `client/Assets/Tests/PlayMode/PvpBuildUi/` | IMP-039 |
| `client/Assets/Tests/PlayMode/QuestUi/` | IMP-021 |
| `client/Assets/Tests/PlayMode/RelicChestPresentation/` | IMP-091 |
| `client/Assets/Tests/PlayMode/RewardClaimUi/` | IMP-010 |
| `client/Assets/Tests/PlayMode/Screens/` | IMP-099 |
| `client/Assets/Tests/PlayMode/SeasonsUi/` | IMP-052 |
| `client/Assets/Tests/PlayMode/SessionTransport/` | IMP-065 |
| `client/Assets/Tests/PlayMode/ShopUi/` | IMP-028 |
| `client/Assets/Tests/PlayMode/SkillUi/` | IMP-015 |
| `client/Assets/Tests/PlayMode/SocialChatUi/` | IMP-034 |
| `client/Assets/Tests/PlayMode/SoulUi/` | IMP-031 |
| `client/Assets/Tests/PlayMode/SparringPresentation/` | IMP-087 |
| `client/Assets/Tests/PlayMode/SpiritBeastPresentation/` | IMP-057 |
| `client/Assets/Tests/PlayMode/SpiritSurgePresentation/` | IMP-025 |
| `client/Assets/Tests/PlayMode/StoreUi/` | IMP-102 |
| `client/Assets/Tests/PlayMode/ThinhThan.Tests.PlayMode.asmdef` | IMP-000 |
| `client/Assets/Tests/PlayMode/TradeUi/` | IMP-029 |
| `client/Assets/Tests/PlayMode/WorldTransferPresentation/` | IMP-018 |
| `client/Assets/Tests/PlayMode/csc.rsp` | IMP-000 |
| `client/Assets/UniversalRenderPipelineGlobalSettings.asset` | IMP-000 |
| `client/Assets/UniversalRenderPipelineGlobalSettings.asset.meta` | IMP-000 |
| `client/BuildProfiles/` | IMP-067 |
| `client/Packages/` | IMP-000 |
| `client/ProjectSettings/` | IMP-000 |
| `client/ProjectSettings/QualitySettings.asset` | IMP-095 |
| `deploy/load/` | IMP-046 |
| `deploy/prod/` | IMP-048 |
| `deploy/prod/runbooks/data_subject_requests.md` | IMP-103 |
| `docs/10_implementation/release/` | IMP-048 |
| `proto/testdata/golden/` | IMP-061 |
| `proto/thinhthan/internal/v1/durable_journal.proto` | IMP-061 |
| `proto/thinhthan/v1/` | IMP-061 |
| `scripts/codegen.ps1` | IMP-061 |
| `scripts/device_perf.ps1` | IMP-096 |
| `scripts/verify.ps1` | IMP-000, IMP-106 |
| `scripts/verify_client_build.ps1` | IMP-067 |
| `server/cmd/compiler/` | IMP-003 |
| `server/cmd/migrate/` | IMP-005 |
| `server/cmd/server/` | IMP-006, IMP-069 |
| `server/cmd/verify/` | IMP-000 |
| `server/go.mod` | IMP-000, IMP-003, IMP-005, IMP-006, IMP-081, IMP-098, IMP-100 |
| `server/go.sum` | IMP-000, IMP-003, IMP-005, IMP-006, IMP-081, IMP-098, IMP-100 |
| `server/internal/app/` | IMP-069 |
| `server/internal/config/` | IMP-003, IMP-004 |
| `server/internal/config/equipment/` | IMP-026 |
| `server/internal/config/validation/balance/` | IMP-049 |
| `server/internal/config/validation/beast/` | IMP-050 |
| `server/internal/config/validation/drop/` | IMP-051 |
| `server/internal/conformance/architecture/` | IMP-083 |
| `server/internal/conformance/caching/` | IMP-106 |
| `server/internal/conformance/deviceperf/` | IMP-096 |
| `server/internal/conformance/gates/` | IMP-000 |
| `server/internal/conformance/ratchet/` | IMP-068 |
| `server/internal/conformance/style/` | IMP-000 |
| `server/internal/conformance/taskgraph/` | IMP-083 |
| `server/internal/conformance/trusted/` | IMP-068 |
| `server/internal/core/id/` | IMP-001 |
| `server/internal/core/rng/` | IMP-002 |
| `server/internal/durable/account/` | IMP-006 |
| `server/internal/durable/anti_rmt/` | IMP-054 |
| `server/internal/durable/atlas/` | IMP-060 |
| `server/internal/durable/auction/` | IMP-030 |
| `server/internal/durable/beasts/` | IMP-057 |
| `server/internal/durable/character/` | IMP-100 |
| `server/internal/durable/chat/` | IMP-094 |
| `server/internal/durable/chivalry/` | IMP-086 |
| `server/internal/durable/cooking/` | IMP-059 |
| `server/internal/durable/cosmetics/` | IMP-038 |
| `server/internal/durable/crafting/` | IMP-027 |
| `server/internal/durable/currency/` | IMP-007 |
| `server/internal/durable/db/` | IMP-005 |
| `server/internal/durable/discovery/` | IMP-020 |
| `server/internal/durable/dungeons/` | IMP-023 |
| `server/internal/durable/equipment/` | IMP-012 |
| `server/internal/durable/feats/` | IMP-085 |
| `server/internal/durable/fishing/` | IMP-058 |
| `server/internal/durable/formations/` | IMP-033 |
| `server/internal/durable/guild/` | IMP-036 |
| `server/internal/durable/guild/stone/` | IMP-093 |
| `server/internal/durable/guild_storage/` | IMP-037 |
| `server/internal/durable/guild_war/` | IMP-042 |
| `server/internal/durable/idempotency/` | IMP-005 |
| `server/internal/durable/inventory/` | IMP-009 |
| `server/internal/durable/journal/v1/` | IMP-061 |
| `server/internal/durable/items/` | IMP-008 |
| `server/internal/durable/lockorder/` | IMP-097 |
| `server/internal/durable/meridian/` | IMP-032 |
| `server/internal/durable/monetization/` | IMP-053 |
| `server/internal/durable/monetization/claims/` | IMP-102 |
| `server/internal/durable/operator/` | IMP-077 |
| `server/internal/durable/privacy/` | IMP-056 |
| `server/internal/durable/progression/` | IMP-011 |
| `server/internal/durable/pvp/` | IMP-039 |
| `server/internal/durable/pvp/arena/` | IMP-041 |
| `server/internal/durable/pvp/duel/` | IMP-040 |
| `server/internal/durable/quests/` | IMP-021 |
| `server/internal/durable/queue/` | IMP-082 |
| `server/internal/durable/reward/` | IMP-010 |
| `server/internal/durable/schema/` | IMP-005 |
| `server/internal/durable/seasons/` | IMP-052 |
| `server/internal/durable/shops/` | IMP-028 |
| `server/internal/durable/social/` | IMP-034 |
| `server/internal/durable/souls/` | IMP-031 |
| `server/internal/durable/trade/` | IMP-029 |
| `server/internal/durable/world/` | IMP-018 |
| `server/internal/durable/worldconsequence/` | IMP-022 |
| `server/internal/edge/account/` | IMP-103 |
| `server/internal/edge/admin/` | IMP-077 |
| `server/internal/edge/auth/` | IMP-006 |
| `server/internal/edge/character/` | IMP-100 |
| `server/internal/edge/entitlement/` | IMP-102 |
| `server/internal/edge/heartbeat/` | IMP-081 |
| `server/internal/edge/iap/` | IMP-053 |
| `server/internal/edge/inventory/` | IMP-009 |
| `server/internal/edge/listener/` | IMP-081 |
| `server/internal/edge/progression/` | IMP-011 |
| `server/internal/edge/router/` | IMP-006 |
| `server/internal/edge/security/` | IMP-045 |
| `server/internal/edge/session/` | IMP-006 |
| `server/internal/edge/world/` | IMP-018 |
| `server/internal/global/bosses/` | IMP-022 |
| `server/internal/global/guild/` | IMP-036 |
| `server/internal/global/matchmaking/arena/` | IMP-041 |
| `server/internal/global/matchmaking/duel/` | IMP-040 |
| `server/internal/global/matchmaking/guild_war/` | IMP-042 |
| `server/internal/global/moderation/` | IMP-094 |
| `server/internal/global/party/` | IMP-035 |
| `server/internal/global/runtime/` | IMP-080 |
| `server/internal/global/seasons/` | IMP-052 |
| `server/internal/global/social/` | IMP-034 |
| `server/internal/global/spirit_surge/` | IMP-025 |
| `server/internal/observability/audit/` | IMP-043 |
| `server/internal/observability/core/` | IMP-098 |
| `server/internal/protocol/v1/` | IMP-061 |
| `server/internal/sim/aoi/` | IMP-079 |
| `server/internal/sim/atlas/` | IMP-060 |
| `server/internal/sim/beasts/` | IMP-057 |
| `server/internal/sim/books/` | IMP-090 |
| `server/internal/sim/bosses/` | IMP-022 |
| `server/internal/sim/bosses/chest/` | IMP-091 |
| `server/internal/sim/bosses/relics/` | IMP-091 |
| `server/internal/sim/classes/` | IMP-017 |
| `server/internal/sim/combat/` | IMP-014 |
| `server/internal/sim/cooking/` | IMP-059 |
| `server/internal/sim/crafting/` | IMP-027 |
| `server/internal/sim/death/` | IMP-084 |
| `server/internal/sim/discovery/` | IMP-020 |
| `server/internal/sim/dungeons/` | IMP-023 |
| `server/internal/sim/dungeons/endgame/` | IMP-024 |
| `server/internal/sim/effects/` | IMP-016 |
| `server/internal/sim/effects/maam/` | IMP-092 |
| `server/internal/sim/equipment/` | IMP-012 |
| `server/internal/sim/fishing/` | IMP-058 |
| `server/internal/sim/formations/` | IMP-033 |
| `server/internal/sim/guild_war/` | IMP-042 |
| `server/internal/sim/meridian/` | IMP-032 |
| `server/internal/sim/movement/` | IMP-013 |
| `server/internal/sim/progression/` | IMP-011 |
| `server/internal/sim/pvp/` | IMP-039 |
| `server/internal/sim/pvp/arena/` | IMP-041 |
| `server/internal/sim/pvp/duel/` | IMP-040 |
| `server/internal/sim/pvp/sparring/` | IMP-087 |
| `server/internal/sim/quests/` | IMP-021 |
| `server/internal/sim/quests/bounty/` | IMP-089 |
| `server/internal/sim/replication/` | IMP-079 |
| `server/internal/sim/runtime/` | IMP-079 |
| `server/internal/sim/shops/` | IMP-028 |
| `server/internal/sim/skills/` | IMP-015 |
| `server/internal/sim/souls/` | IMP-031 |
| `server/internal/sim/spatial/capacity/` | IMP-055 |
| `server/internal/sim/spatial/collision/` | IMP-078 |
| `server/internal/sim/spatial/geometry/` | IMP-078 |
| `server/internal/sim/spatial/maps/` | IMP-062 |
| `server/internal/sim/spatial/parity/` | IMP-062 |
| `server/internal/sim/spatial/testdata/` | IMP-062 |
| `server/internal/sim/spawning/` | IMP-019 |
| `server/internal/sim/trade/` | IMP-029 |
| `server/internal/sim/travel/` | IMP-020 |
| `server/internal/sim/world/` | IMP-018 |
| `server/internal/sim/world/surge/` | IMP-025 |
| `server/internal/stackpin/` | IMP-000 |
| `server/internal/testing/fault/` | IMP-044 |
| `server/internal/testing/load/` | IMP-046 |
| `server/internal/testing/migration/` | IMP-047 |
| `server/internal/testing/pgtest/` | IMP-005 |
| `server/internal/testing/protocol/` | IMP-061 |
| `server/internal/testing/release/` | IMP-048 |
| `server/migrations/` | IMP-005 |

## Invariants

```text
current implemented baseline = none (docs only)
planned path requires an IN_PROGRESS owner task
Go module root = server/go.mod, module thinhthan
one production main = server/cmd/server
one deployed binary = thinhthan-server
proto source = proto/thinhthan/v1
generated protocol is never hand-edited
one migration owner = IMP-005
no Redis / ORM / extra server binary / second Go module / container
```
