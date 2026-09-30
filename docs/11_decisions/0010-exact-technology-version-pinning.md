# ADR-0010: Exact Technology Version Pinning
status: ACCEPTED

## Context
This repository is implemented primarily by AI agents. If framework, compiler, database, code-generator, or core-library versions are left implicit, separate agents can choose different versions and produce incompatible code, migrations, generated files, or Unity assets.

"Latest" is also not a reproducible build input and may point to a preview/prerelease that is newer numerically but less appropriate for production.

## Decision
- `docs/00_context/technology_versions.md` is the canonical launch technology-version matrix.
- Foundation toolchains and approved core dependencies are pinned to exact stable production versions.
- Build manifests/CI must use the pinned versions; `latest`, wildcards, floating ranges, preview/beta/RC/nightly versions, and agent-selected substitutes are forbidden unless the canonical matrix explicitly allows them.
- Prefer current production LTS/stable releases over preview/prerelease releases for launch.
- An AI agent may not introduce a new runtime/framework/infrastructure dependency merely because it is convenient. It must first add an explicit approved version to the matrix and update the owning architecture spec.
- A change that alters a foundational architecture/data-contract choice still requires its own ADR; a patch/minor dependency refresh that preserves the architecture may update the version matrix with compatibility/test evidence.
- Dependency updates are intentional repository changes. They never happen implicitly because a package manager resolves a newer version.

## Reproducibility
Implementation pins are expected in native lock points:
- Unity editor: `ProjectSettings/ProjectVersion.txt`.
- Unity packages: exact `Packages/manifest.json` entries plus `Packages/packages-lock.json`.
- Go toolchain/modules: `go.mod` / `go.sum` and CI toolchain.
- Protocol compiler/generators: pinned tool bootstrap/checksum or pinned CI artifact.
- PostgreSQL: exact production image/package major+patch policy in deployment configuration.
- Migration and build tools: exact version in bootstrap/CI.

Generated Protocol Buffer sources are produced by pinned generators and checked/verified for drift.

## Update Rule
A version update must:
1. update `technology_versions.md`,
2. update native lockfiles/toolchain declarations,
3. regenerate affected generated code,
4. run relevant compatibility, migration, gameplay/network/backend tests,
5. record any migration/behavior change in the owning spec/ADR,
6. never silently alter production data/content semantics.

## Consequences
- Agents have one deterministic stack target.
- Reproducibility is favored over opportunistic package upgrades.
- Security/bugfix updates remain possible, but are reviewed and tested explicitly.
- A numerically newer beta (for example a database beta) is not selected over a current stable release merely because it has a higher major number.

## Invariants
```text
one canonical version matrix
exact stable pins
no latest/*
no prerelease unless explicitly approved
no unapproved core dependency
native lockfiles must agree with the matrix
```
