// Package anti_rmt implements the economy behavioural signals of
// anti_cheat.md § Economy Behavioural Signals (ADR-0041, ADR-0065):
// three rolling-window predicates evaluated on the persistent
// economy_*_daily_rollups aggregation tables plus indexed raw
// settlement records for partial boundary days, an
// accounts.economy_review_flagged_at review flag set/cleared with one
// audit append per transition, and the exported settlement-emission
// entrypoint producers call after each qualifying economy commit.
//
// Windows are exact [T-W, T) UTC intervals (W = 7 or 30 elapsed
// 24-hour days): whole-day rollup rows strictly inside the window,
// indexed raw events for the oldest and current partial days. The
// result equals the raw interval query.
//
// The flag is detection-and-review only: it blocks nothing and never
// triggers enforcement.
package anti_rmt
