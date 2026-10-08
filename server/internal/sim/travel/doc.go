// Package travel owns IMP-020's travel NPC service surface: the
// destination-tier fee table (npc_shop_catalog.md § Travel), the
// service_id registration seam used by composition (spec #215 F2:
// Runtime.Dispatcher().RegisterService is IMP-069's wiring — this
// package exports ServiceID and the handler; sim/world/interact.go
// stays IMP-018-owned and is never edited), and the admission helpers
// the consult and post-commit effect paths call.
package travel
