// Package crafting owns sim-side admission verdicts for C2S_CRAFT
// (404) and C2S_ENHANCE (406) at SERVICE(crafting|enhancement) station
// NPCs — the runtime gates that run before a durable record exists
// (crafting.md § Atomic Craft, npcs.md § services, ADR-0083 open NPC
// session + 2.5 m range). Durable settlement replay lives in
// internal/durable/crafting; this package never imports SQL.
package crafting
