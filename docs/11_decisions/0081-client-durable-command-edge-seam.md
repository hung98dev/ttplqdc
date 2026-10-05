# ADR-0081: Client Durable Command Edge Seam
status: ACCEPTED

## Context

Every durable C2S intent in `protobuf_conventions.md` §7 `CLIENT_EXPANSION` (~40 message IDs across ~40 task packets) is dispatched by `server/internal/edge/router/` to a per-ID handler, but two seams were undeclared (wave-9 plan F-1):

1. **Session identity.** A handler receives the classified `Route` only. `JournalClientCommand` requires `account_id`, optional `character_id`, `session_epoch`, optional `ownership_epoch`; these live in `session.Registry`'s private session record, resolved internally at `Enqueue` and never exported. The router table cannot pass session state, and an exported epoch→session accessor would re-resolve what the adapter already resolved.
2. **Outcome delivery.** `queue.Submit` enqueues asynchronously; `queue.Ack` waits for resolution but returns only `error` and marks receipt disposition (releasing the retained outcome to purge). The typed `JournalOutcome` — including `client_result` at field `2000+n` — is persisted in `durable_command_receipts`, with no declared read path to the live connection.

Alternatives weighed: adapter-internal dispatch of CLIENT ids inside `session/` (rejected — every handler would have to live in the session package, contradicting "later tasks register their handler from their own package"); an exported `session.Registry` lookup (rejected — redundant second resolution and a new mutable-state accessor); deferring the seam to the IMP-069 composition root (rejected — IMP-069 depends transitively on IMP-100, which needs the seam now); reading the receipt row from each feature's own store (rejected — duplicates queue-owned receipt SQL in every package and races retention).

## Decision

- **Session view injection.** The session adapter passes an immutable session view — bound connection, `account_id`, `session_epoch`, `character_id`/`ownership_epoch` absent while unattached — into `router.Dispatch`. The view type lives in `server/internal/edge/router/` (session already imports router; no cycle). Handler signature takes the view; handlers never re-resolve session state. Attach/detach (6/10) remain session-internal and unchanged.
- **Client outcome await seam.** `server/internal/durable/queue/` owns a declared accessor that waits for a submitted CLIENT command's terminal resolution and returns the retained schema-v1 `JournalOutcome` (typed `client_result` member for client-visible results, or the conclusively recorded terminal nonexecution). A retried `operation_id` returns the retained committed outcome without re-execution — reconnect/replay safe. Handlers never poll `durable_command_receipts` themselves.
- **Deliver-then-ack.** The handler delivers the typed `S2C` result — plus any declared follow-up push on the session connection (e.g. `S2C_CHARACTER_LIST` after a successful create) — and only then calls `Ack`, because disposition acknowledgement makes the retained outcome purge-eligible.
- **Out-of-set rejections.** A domain rejection outside a result message's closed `error_code` set is sent as `S2C_ERROR` (id 3) through the router's reject path (e.g. `ACCOUNT_SUSPENDED` on `C2S_CHARACTER_CREATE` — its closed set at `messages.md` id 13 covers only name/slot/class codes).

Canonical text: `service_boundaries.md` § Edge / Session and `protobuf_conventions.md` §7.

## Consequences

- Edits land in IMP-006-owned `edge/session` + `edge/router` and IMP-082-owned `durable/queue` — routed as coordinator gatefixes to those owners; the seam spec itself is implementer-independent and unblocks IMP-100.
- Every future CLIENT-producer packet gets the same uniform flow: session view → build `JournalClientCommand` → submit → await outcome → deliver `client_result` on the connection → `Ack`. No new per-task plumbing or invented session lookup.
