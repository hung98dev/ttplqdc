# Observability
status: LOCKED

## Scope
Defines mandatory launch logs, metrics, traces, correlation context, dashboards, and alert classes.

## Principles
Observability must explain authority/value failures without logging secrets or requiring packet capture.

Instrumentation is OpenTelemetry (vendor-neutral); the launch storage, dashboard and alerting stack is fixed below.

## Launch Telemetry Stack (ADR-0066)
Versions: `../00_context/technology_versions.md` § Production Operations.
```text
server metrics + traces  OTel SDK -> OTLP/HTTP -> Collector on the world host (127.0.0.1:4318, OTEL_EXPORTER_OTLP_ENDPOINT)
metrics                  Collector prometheus exporter -> Prometheus on the ops host (15 s scrape, 90-day retention)
host / DB metrics        node_exporter on every host, postgres_exporter on the PostgreSQL host -> Prometheus
traces                   Collector file exporter on the world host, JSON, rotated at 14 days
logs                     log/slog JSON to stdout -> journald on the world host (MaxRetentionSec=90day)
audit / security events  audit_events in PostgreSQL (../06_data/data_model.md), 3-year retention
dashboards               Grafana on the ops host, the seven § Required Dashboards provisioned from deploy/prod/grafana/
alerts                   Prometheus rules (deploy/prod/prometheus/rules/) -> Alertmanager
```
Alertmanager receivers: `ops-critical` (every § Alerts critical example; page the operator), `ops-warning` (warnings; daily digest) and `security-queue` (every § Alerts security alert; page the operator). Alert history for 3 years is kept by the world process, not Alertmanager: the Alertmanager webhook for `security-queue` posts to the server private admin listener (`ADMIN_BIND_ADDR`), which inserts one `audit_events` row per firing/resolved notification (`actor_kind = SYSTEM`, Category H, 3-year retention; ADR-0070). Receiver endpoints (email/webhook) are deploy-environment configuration injected at runtime, never committed. A telemetry pipeline outage never blocks the server: the SDK drops on a full export queue and counts the drops.

## Correlation Context
Where applicable include:
```text
trace_id
correlation_id
operation_id
account_id/character_id (policy-safe)
session_epoch
simulation_owner_id
partition_id
content_revision
protocol_version
server_build
```

Never log passwords, raw auth tokens, gameplay resume credentials, DB credentials, or full payment/provider secrets.

## Realtime Metrics
Per process/partition:
- tick runtime p50/p95/p99,
- late/catch-up tick count,
- entity/player counts,
- command queue depth/wait/reject,
- AI time,
- hit/effect resolution time,
- replication serialization time/bytes,
- outbound queue depth,
- reconnect/transfer rates.

Alert when realtime thresholds from `capacity.md`/realtime loop are sustained.

## Network Metrics
- active WSS connections,
- connect/handshake failures,
- bytes/sec up/down,
- message counts by family,
- malformed/rate-limited messages,
- stale input rejection,
- heartbeat timeout,
- slow-consumer disconnect,
- baseline/resync frequency,
- **dropped/late movement-edge counter** (`C2S_MOVEMENT_EDGE` ID 108 messages that are dropped or arrive out-of-order relative to send sequence): warn on any non-zero count; a dropped edge is a silent Just Guard breakage.

## PostgreSQL / Durable Metrics
- pool in-use/idle/wait,
- query and transaction latency,
- rows/locks waited,
- deadlock/serialization retry,
- transaction rollback rate,
- slow query count,
- idempotency conflict/replay,
- Reward Claim/proceeds backlog,
- migration status.

## Economy / Integrity Metrics
Track rates and reconciliation signals for:
- currency faucet/sink by source,
- enhancement attempts/results,
- crafting output,
- Auction trades/tax/proceeds,
- Reward Claim creation/claim,
- Soul/equipment rare acquisition,
- duplicate-operation prevention.

Unexpected value creation is a critical integrity signal.

## Required Dashboards
Launch dashboards:
1. CCU / connection health.
2. Simulation tick/partition health.
3. Network throughput/reconnect.
4. PostgreSQL/persistence.
5. Economy/reward integrity.
6. Content revision/errors.
7. Instance/public-boss health.

## Gameplay / World Health Metrics
Metric class covering simulation and world-system health:

- **AOI entity shedding rate**: count of clients per channel that have hit `MAX_ENTITIES_IN_AOI_PER_CLIENT = 40` and are receiving a reduced entity set; shed entities are invisible to those clients. Alert condition: any client in a contested channel sustaining the cap for >10s is a warning; the shed count per channel tick is a standing metric. This indicates the density-increase has saturated the AOI budget.
- **WorldConsequence load delay**: time taken to load the WorldConsequence aggregate before a partition accepts its first player. Warn when this exceeds 500ms; partition-start rejection due to `WORLD_CONSEQUENCE_LOAD_TIMEOUT = 5 s`, an unreadable table or a quarantined row (`../06_data/data_model.md` § Boss Aftermath Relic) is the critical alert `world_consequence_load_failed` for that partition only; zero rows is valid and never alerts.
- **Spirit Surge 3-region coordination failure**: count of UTC hours where fewer than 3 regions activated (coordinator failure, region eligibility exhaustion, or deterministic assignment fault). Any non-zero count in a production hour is a warning; two consecutive hours is critical.
- **Anti-RMT rolling-window query rate/latency**: query rate and p95/p99 latency for the anti-RMT rolling-window aggregation query. Warn when p95 exceeds 50ms or query rate exceeds a configured threshold indicating it is becoming a database hotspot; alert when p99 exceeds 200ms or a single query causes lock waits.

## Alerts
Critical examples:
- sustained p95 tick >=50ms,
- partition crash/restart loop,
- DB unavailable or pool exhaustion,
- durable mutation error spike,
- dual-ownership invariant violation,
- reward/economy reconciliation mismatch,
- backup/PITR failure,
- `world_consequence_load_failed`: WorldConsequence rows of a partition unreadable, quarantined (unknown content ID) or over `WORLD_CONSEQUENCE_LOAD_TIMEOUT` at partition start (zero rows is valid; only that partition stays closed),
- `shutdown_flush_timeout` (durable outbox journal written) and `durable_outbox_corrupt` (startup stopped; `deployment.md` § Durable Outbox Journal),
- `erasure_ledger_backlog`: a `pending_erasure_ledger` row older than 24 h (`../06_data/data_model.md` § Account Erasure),
- Spirit Surge coordination failure for 2+ consecutive hours,
- error budget burn rate > 10x over 1 hour for any SLO below,
- `DURABLE_BACKPRESSURE` active on any partition > 60s.

Security alerts (Alertmanager receiver `security-queue`):
- password/login failures per IP or `username_key` above 10x the 7-day baseline,
- `SESSION_REPLACED` rate per account > 10/hour,
- refresh-credential reuse detection (any),
- new `ECONOMY_REVIEW`, `AUTOMATION_REVIEW` or `ACCOUNT_SECURITY_REVIEW` flag,
- operator (admin) value-affecting action (always notified).

Warning examples:
- p95 tick >=35ms,
- rising reconnect/resync,
- slow-consumer rate,
- elevated DB lock waits,
- queue depth approaching capacity,
- any dropped/late `C2S_MOVEMENT_EDGE` (ID 108),
- client AOI cap sustained >10s in contested channel,
- WorldConsequence partition-start load >500ms,
- anti-RMT rolling-window query p95 >50ms,
- `di_tich_sweep_failed`: relic expiry sweep run failed (retried every 60 s; reads stay correct, ADR-0070).

## SLOs
Measured monthly, excluding announced maintenance restarts (ADR-0052):
```text
availability      : successful login + character attach >= 99.5% of attempts  (error budget 0.5%)
simulation        : p95 tick < 35 ms in >= 99% of partition-minutes
durable ops       : p95 commit latency < 150 ms; error rate < 0.1%
reconnect         : >= 99% of drops reconnect within 30 s (normal world)
```

## Logs
Structured logs only in production.

Repeated high-frequency gameplay success events are sampled/aggregated rather than logged individually when metrics suffice.

Security/audit/value-changing events retain required audit detail according to policy.

## Tracing
Trace cross-boundary operations that can block or mutate durable state.

Do not create one expensive distributed trace span for every simulation entity every tick.

Sampling must preserve error traces and value-changing failures.

## Retention
Retention is configured by data class:
- operational high-volume telemetry: shorter,
- audit/security/value-changing logs: longer according to policy/legal requirements.

Launch retention:
```text
operational logs/metrics with IP or account_id : 90 days (data_protection.md Category D)
traces                                          : 14 days
gameplay event logs                             : 180 days (Category G)
audit/security logs                             : 3 years (Category H)
```
Deletion must be intentional and testable.

## Invariants
- every value-changing retry can be correlated,
- every partition exposes tick/queue/load metrics,
- secrets are redacted,
- observability failure never changes gameplay authority,
- alerts cover realtime, DB, ownership, and economy integrity,
- AOI entity shedding, WorldConsequence load delay, Spirit Surge coordination, anti-RMT query hotspot, and dropped movement-edge counts are standing metrics with defined alert thresholds.
