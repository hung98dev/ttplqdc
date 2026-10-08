// Package discovery owns the sim half of IMP-020: the 24-slot
// first-discovery catalog binding (world_route_catalog.md § Map
// Metadata), first-entry detection, and the sim.discovery_settlement
// REWARD emission that durable/discovery commits once per
// reward.discovery.<map_id>.<character_id>. The emission is queued
// through the partition's EmitPort (CmdReward); who invokes the emit on
// successful admission is IMP-069's composition wiring — this package
// exports the seam, it never mutates world internals.
package discovery
