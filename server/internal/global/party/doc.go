// Package party is the in-process Global party host: max-5 membership,
// leadership, 60 s invitations, the 120 s leader-disconnect grace and the
// safe-anchor party board (party.md; service_boundaries.md § Ephemeral
// Global Runtime). Ordinary world parties are ephemeral — a full restart
// restores nothing (party.md § Disconnect), so Restore is a no-op.
//
// Boundaries: the package owns party domain state only. Presence,
// social direct-interaction gates and safe-anchor membership are
// injected (Options) — party never imports edge/sim/durable. Every
// request (602, 604..606, 620..622, 634, 635) yields exactly one Result
// carrying request_message_id + operation_id (ADR-0064); roster/board
// fanout rides the Outbox for the wiring layer to translate to
// 603/607/636 frames.
package party
