# ADR-0083: Edge Admission Consults and Durable-Op World Effects over the Partition Seam
status: ACCEPTED

## Context

ADR-0081 makes the edge durable handler thin: validate with the session view, submit a `JournalClientCommand`, await the terminal outcome, deliver the result. Several durable ops additionally need **live partition/world state at admission** that neither the session view nor catalog data can supply:

- `C2S_INVENTORY_MUTATE` USE emits `use_effect.cooldown_ends_at_tick` — an absolute tick **in the character's owning partition** (`messages.md` Movement Checkpoint rule); the edge needs the partition's current tick.
- `C2S_RESPEC` (513) and `C2S_ENTITLEMENT_CLAIM` (418) carry `npc_id` that must be validated against the **open NPC session**, the NPC's service allowed-set and the 2.5m interaction range (`npcs.md`); session state lives only in the `sim/world` tracker.
- The durable world ids 103/104/109 (`set_checkpoint`, portal, channel switch) need the same session/range/`in_combat` checks plus Director/channel capacity — and their placement effect is partition-owned.

ADR-0082 lands an edge→partition **command** mailbox for non-durable ids but defines no synchronous read surface, and `protobuf_conventions.md` §7 keeps the durable ids on the journal route (`codec/receipt applies whenever their validated intent crosses Durable for placement/checkpoint/membership`).

Fences that bound the design: `durable/` may not import `sim/`; `edge/` routes intent and never owns mutation; the partition cannot originate a `JournalClientCommand` (`runtime.DurableKind` is a closed set with no client-command kind); placement/respawn authority lives in the owning map/channel partition (ADR-0082).

Alternatives weighed: an edge-side published replica of partition/session state (rejected — a second seam with weaker ordering and an unbounded staleness contract; a FIFO consult on the command path gives a partition-consistent answer at no new transport); catalog-position-only admission for the NPC checks (rejected — drops `npcs.md`'s open-session + range requirement, a degraded reading of spec); partition-side admission (rejected — the partition cannot originate the `JournalClientCommand`, so moving admission past the edge still leaves the journal origination question unsolved and inverts ADR-0081's thin-handler shape).

## Decision

- **Consult entries on the world-command mailbox.** The `sim/world` typed mailbox (ADR-0082) carries a second closed entry kind: typed **admission consults** — request fields plus a single-shot in-process reply channel. The world runtime routes a consult to the character's owning partition (or answers Director/world-scoped consults itself); it is drained in the same pass as commands, FIFO — a consult observes every command enqueued before it. Replies come from the authoritative site only (the owning partition's drain for partition state; the world runtime for Director/channel state). No state replica is published edge-side.
- **Read-only and bounded.** A consult never mutates, never emits and never originates a journal record. The edge caller awaits with a bounded wait; on timeout or no live owning partition the admission fails with the op's session/state-class error (`INVALID_STATE`/`OUT_OF_RANGE` per the message contract) — never a fabricated verdict, never a silent hang.
- **Initial consult set** (registered like commands by their owning packets): `PartitionTick(characterID)`; `NpcServiceValid(characterID, npcID, serviceID)` — session open, NPC offers the service, within interaction range, not `in_combat`; placement consults for 104/109 admission (portal validity/range and destination capacity; channel target capacity and switch cooldown).
- **Durable-id admission route.** For a durable op that needs live world state, the edge handler: admission consult → builds the `JournalClientCommand` → `queue.Submit` → `AwaitClientOutcome` (ADR-0081) → the owning `durable/` executor commits the placement/checkpoint/membership write and produces the recorded `client_result` → **for ops with a partition-side effect** (104/109 transfer/placement, later placement ops) the edge handler then posts the corresponding world command preserving the client `operation_id` into the same mailbox → delivers the recorded result on the connection → `Ack`. The partition applies the effect through its ADR-0082 drain and settles further writes via `QueueCommand`/`EMIT_DURABLE_COMMANDS`. The partition never emits a `JournalClientCommand`; the edge never performs the world mutation itself; a failed effect post after a committed outcome is reconciled by world recovery like any other partition-side settlement.
- **Ownership.** `sim/world` produces the consult surface; `edge/world` exposes it as a read-only port that sibling `edge/*` admission handlers consume — IMP-018 itself for 103/104/109 (producer and first consumer), `edge/inventory` (IMP-009) for `PartitionTick`, `edge/progression` (IMP-011) and `edge/entitlement` (IMP-102) for `NpcServiceValid`. Composition wires the port in `app/` only.

Canonical text: `service_boundaries.md` § Edge / Session.

## Consequences

- No `sim/runtime` change: the consult is a feature-owned entry kind on the existing `sim/world` mailbox, not new runtime machinery.
- Consults preserve ordering: an admission consult observes the effects of every world command posted before it, so admission cannot validate against pre-drain state.
- The NPC-session requirement of `npcs.md` stays mandatory for every `npc_id`-carrying dedicated message (513, 418, and the existing 404/406/420/426 set); catalog-only fallback is not a compliant substitute.
- Each new `npc_id`-carrying service message, and each new durable op needing live admission state, registers its consult entries in the packet that implements it — the consult set grows exactly like the command registry, never as an open-ended query API.
