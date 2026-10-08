// Package cooking implements the IMP-059 hearth-cooking and bonfire
// durable surface: the `interaction.kindle`, `interaction.cook` and
// `interaction.bonfire_rest` ProducerClient executors behind
// S2C_INTERACT_RESULT (116).
//
// KINDLE consumes exactly one `item.material.cui_lua_trai` instance
// selected at execution (world_rules.md § Kindling: consume iff the
// consult admission proved the bonfire can accept kindling).
//
// COOK freezes a JournalCraftSnapshot at admission — the resolved hearth
// recipe, every input material instance selected, every created item
// UUID (dish + guaranteed `item.material.cui_lua_trai` extra_output) and
// the authored per-act LIFE_SKILL character EXP — per
// protobuf_conventions.md §7 ("103 COOK also requires `craft`"). The
// executor revalidates and commits the frozen snapshot; replay resolves
// the original operation and never rerolls UUIDs, inputs or EXP.
//
// BONFIRE_REST is a recorded admission whose only durable effect is the
// exactly-once 116 result; the rest session and its 10 s/300 s
// settlement intents live in sim/cooking.
//
// The package never writes `characters` rest/beast tables directly —
// `character_rest_daily` belongs to durable/reward (IMP-010) and beast
// bond tables to durable/beasts (IMP-057).
package cooking
