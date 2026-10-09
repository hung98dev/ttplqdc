// Package fishing implements the IMP-058 folk-fishing durable surface:
// the `interaction.cast` and `interaction.hook` ProducerClient
// executors behind S2C_INTERACT_RESULT (116).
//
// CAST revalidates rod (`item.tool.can_cau_tre`) and bait
// (`item.consumable.moi_cau`) ownership plus the 50-success daily cap
// inside the transaction, then consumes exactly one bait — the durable
// sequence of the accepted cast is its receipt ordinal for the UTC day
// (reward_claims.md § Fishing source identity). HOOK rolls the spot's
// catch table exactly once under the PCG-64 stream keyed
// `fishing.<character_id>.<utc_date>.<cast_sequence>`, increments the
// character's daily counter atomically with settlement, grants the
// catch (overflow -> Reward Claims source_type/source_family FISHING,
// source_ref (spot_id, character_id, utc_date, cast_sequence)) and
// grants the authored per-act LIFE_SKILL EXP. Replay resolves the
// recorded outcome — never rerolls, never refunds bait.
package fishing
