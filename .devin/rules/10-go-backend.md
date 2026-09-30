---
description: Go backend rules — toolchain, package design, errors, concurrency, SQL
trigger: glob
globs:
  - "server/**/*.go"
  - "server/go.mod"
  - "server/go.sum"
---

# Go Backend Rules

Canonical: `docs/10_implementation/engineering_conventions.md` §1, `architecture_conformance.md`. Toolchain Go `1.27.1` exactly.

## Format & packages

- `gofmt` clean (tabs); `go vet ./...` and `staticcheck` 2026.2.1 clean (Q4, `CODE-003`; `//lint:ignore` needs a reason, `//lint:file-ignore` is forbidden). Package names: short, lowercase, domain-named (`sim`, `durable`, `edge`, `global`, `core/id`, `core/rng`). No `utils`, `helpers`, `common`, `misc` packages.
- One module `thinhthan` rooted at `server/go.mod`. One production main: `cmd/server`. Only `cmd/compiler`, `cmd/migrate`, `cmd/verify` tools allowed besides it.
- No import cycles. Interfaces small and defined at the consumer; do not create an interface just to have one.
- Import fences (enforced by Q4/architecture tests — do not violate):
  - `sim/` — no `database/sql`, no `pgx`, no `edge/` import
  - `durable/` — no `sim/` import; sole owner of SQL gameplay mutation
  - `edge/` — routes intent only; no combat/value mutation
  - `internal/protocol/` — generated; imports no domain package
  - `global/` — in-process single writer; calls durable via typed interfaces

## Stdlib-first dependencies

- Logging: `log/slog` only (JSON handler in prod). Required attrs on business/error logs: `op`, `revision`, `actor_id`/`session_id` when in session context. Never log secrets or player PII.
- HTTP: `net/http`. Time: `time.Now().UTC()` — server is the only time authority; never trust client timestamps.
- IDs/revisions/operation identity: reuse `server/internal/core/id/` once IMP-001 creates it; never create a second UUID, revision, static-ID, runtime-entity-ID, or operation-conflict implementation.
- RNG: gameplay rolls via injected `core/rng` (`math/rand/v2` PCG-64); `crypto/rand` only through the canonical ID/security owners. `math/rand` v1 is forbidden.
- Forbidden: Gin/Chi/Echo/Fiber, gorilla/*, gRPC, GORM/sqlx/ORM, zap/logrus/zerolog, Redis/Kafka/NATS clients, `x/time/rate` (not pinned; the rate limiter is Go stdlib per `external_integrations.md`).

## Errors, context, concurrency

- `ctx context.Context` is the first parameter of every I/O, DB, network, or long-running function. Honor `ctx.Done()`; never store ctx in long-lived structs.
- Domain errors: sentinel errors + `errors.Is`/`errors.As`. Map to wire codes from `docs/05_network/errors.md` at the edge only. Do not double-log one error at multiple layers. Never swallow an error (`_ = err` needs a documented reason).
- No `panic` in request handling or sim tick — panic only for unrecoverable startup assertion.
- Every goroutine needs a defined lifecycle: ownership, cancellation, error propagation. Sim loop = single-writer goroutine per map channel; no shared mutable memory without channels or atomic snapshots. Run `go test -race` on concurrency-sensitive changes.

## Security & performance

- Treat every client field as untrusted intent. Authenticate, authorize ownership, validate size/range/state, and enforce applicable rate limits before mutation; never trust client prices, hits, rewards, coordinates, revisions, paths, or RNG outcomes.
- No unbounded tick scans, goroutine fan-out, lock hold, query loops, or network buffers. Hot-path optimization requires a benchmark/profile or a documented spec bound; avoid premature optimization elsewhere.
- Steady sim tick, delta build and envelope encode are 0 allocs/op, gated by `TestAllocs_*` (`!race`, `testing.AllocsPerRun`; `capacity.md` § Hot-Path Allocation Budgets). A new per-tick system adds its own `TestAllocs_<System>Tick`. Reuse caller-owned buffers; no `fmt`/`reflect`/per-tick maps/closures in steady state (`engineering_conventions.md` §1.7). ns/op is report-only.

## SQL / persistence (pgx/v5, `durable/` only)

- 100% parameterized SQL (`$1`, `$2`); never string-concatenate SQL.
- Multi-value mutations in one transaction; consistent lock ordering (sort IDs before `SELECT FOR UPDATE`).
- Every persistent mutation has stable operation identity / uniqueness constraint; retry returns committed result, never re-executes.
- Migrations: `server/migrations/` numbered `NNNNNN_name.up/down.sql` pairs, immutable once committed; `000001_baseline_schema` is created by IMP-005.

## Tests

- `*_test.go` colocated; name `Test<Feature>_<Scenario>`; happy path + error/boundary cases.
- Deterministic: fixed seeds via `core/rng`, frozen time injection; no wall-clock or map-iteration dependence. Fixtures in `testdata/`, immutable.
