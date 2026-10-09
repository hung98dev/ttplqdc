// Package fishing implements the IMP-058 folk-fishing runtime inside
// the sim: the per-character FSM `IDLE -> CASTING -> HOOK_WINDOW ->
// RESOLVING -> IDLE`, hook-window timing and the interact admission
// consults (world_rules.md § Folk Fishing, ADR-0024).
//
// One Channel value is owned by the sim/world ChannelHost of one map
// channel; it is constructed with the map's committed `fishing_spot.*`
// geometry anchors, consulted through the ADR-0083 consult drain and
// advanced once per partition tick. The package never touches SQL —
// bait consumption, the keyed PCG-64 catch roll, the daily counter and
// the reward/claim writes commit inside durable/fishing's
// `interaction.cast` / `interaction.hook` executors on the other side
// of the flat DurableCommand queue.
package fishing
