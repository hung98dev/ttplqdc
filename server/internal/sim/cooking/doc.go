// Package cooking implements the IMP-059 hearth/bonfire runtime inside
// the sim: bonfire kindle state and expiry, hearth interact admission
// and the per-character bonfire-rest sessions whose completed 10 s
// rest ticks and 300 s bond intervals emit `sim.rest_settlement` /
// `sim.beast_settlement` DurableCommands (world_rules.md § Hearth,
// § Bonfire Rest; save_rules.md § Closed Producer Registry).
//
// One Channel value is owned by the sim/world ChannelHost of one map
// channel; it is constructed with the map's committed geometry anchors
// (`bonfire.<map_id>` and `cooking_hearth.<map_id>`), consulted through
// the ADR-0083 consult drain and advanced once per partition tick.
// The package never touches SQL — durable writes ride the flat
// DurableCommand queue and are applied by durable/cooking,
// durable/reward (REST settlement) and durable/beasts (BOND
// settlement) on the other side.
package cooking
