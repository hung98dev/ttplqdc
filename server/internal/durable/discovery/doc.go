// Package discovery owns the durable half of IMP-020's progression-source
// events: the character_discoveries once-only record store, the
// sim.discovery_settlement REWARD executor committing the 24 authored
// first-discovery EXP grants, and the travel NPC_SERVICE executor charging
// the destination-tier fee inside the operation's commit. Wire contract:
// docs/05_network/messages.md §103/§105/§116/§160; content:
// docs/07_content/world_route_catalog.md (discovery EXP),
// docs/07_content/npc_shop_catalog.md (travel tiers).
package discovery
