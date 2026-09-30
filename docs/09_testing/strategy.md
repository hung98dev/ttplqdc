# Test Strategy
status: LOCKED

## Scope
Defines mandatory test layers, deterministic fixtures, environment ownership, and release gates for Unity client + Go backend + PostgreSQL.

## Layers
1. fast unit/formula tests,
2. content compiler/static validation,
3. Go + PostgreSQL integration tests,
4. Unity protocol/client-state integration tests,
5. deterministic gameplay simulation tests,
6. network/fault/reconnect tests,
7. security tests,
8. load/soak tests,
9. staging release smoke,
10. backup/restore drill evidence.

## Determinism
Gameplay/content fixtures pin:
```text
content_revision
RNG seed/stream
input sequence
server tick count
protocol/schema version
```

A deterministic test may not depend on wall-clock scheduling, Go map iteration, or random seed from test runtime.

UTC schedule tests use injected/frozen time.

## Test Ownership
Canonical specialized suites:
- `gameplay.md`
- `backend.md`
- `network.md`
- `load.md`
- security specs/tests

A behavior change updates the owning suite in the same change.

CI must also fail when the implementation resolves a toolchain/direct dependency different from `../00_context/technology_versions.md`, even if compilation succeeds.

## Real Dependencies
Use real PostgreSQL for transaction/migration/locking integration tests.

External identity/provider integrations may use controlled test providers/stubs at lower layers, plus staging end-to-end verification.

Gameplay simulation tests do not require production third-party services.

## Unity Contract Tests
Generated protobuf C# and Go schemas share golden fixtures.

Unity tests cover:
- pinned Unity Localization tables, required `vi-VN` and `en-US` coverage, glyph/layout and Smart String argument validation,
- pinned Addressables catalog/bundle build and compatibility metadata,
- remote/local asset load, cache/retry/hash-failure, destination preload, and handle-release regressions,
- encode/decode compatibility,
- baseline/delta lifecycle,
- prediction/reconciliation,
- stale session/error behavior,
- object-pool reset,
- content/version gate,
- mobile/PC input mapping to identical gameplay intents.

## Fault Injection
Mandatory fault cases include:
- duplicate request,
- packet delay/disconnect,
- process crash,
- DB latency/unavailability,
- commit-before-response loss,
- transfer interruption,
- queue saturation,
- scheduler duplicate,
- full inventory/currency cap.

## Release Gates
Pull request/CI:
- canonical technology-version/lockfile drift check,
- protobuf regeneration drift check with pinned compiler/generators,
- unit/static/compiler,
- gameplay deterministic,
- backend integration,
- network schema/protocol,
- security static/unit where applicable.

Pre-production:
- migration rehearsal,
- full staging smoke,
- fault/reconnect,
- performance regression.

Launch/release candidate:
- 10k load/soak gate,
- content integration + balance validation,
- no critical security finding,
- backup/PITR verification,
- rollback compatibility known.

## Flaky Tests
Do not normalize flaky authoritative/state tests.

A flaky deterministic/concurrency test is treated as a bug until root cause is fixed or the test is proven invalid.

Performance tests may use statistical thresholds but still record reproducible environment/context.

## Test Data
Fixtures use synthetic/non-production personal data.

Production secrets and player credential dumps are never copied into CI fixtures.

## Evidence
Release evidence records build, schema, content revision, test suite version, infrastructure shape for load tests, and results.

A green local build without this evidence is not launch readiness.

## Invariants
- deterministic rules have deterministic tests,
- real PostgreSQL verifies DB semantics,
- Unity/Go protocol has shared golden fixtures,
- fault injection is mandatory for value/ownership flows,
- 10k readiness requires load/soak evidence,
- no critical known correctness/security regression ships.
