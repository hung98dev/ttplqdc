// Package atlas is the sim-side Atlas journal intake: authoritative
// source events (monster kills, soul acquisitions, boss relic witnesses,
// hidden-chest opens, fish catches, dishes cooked) resolve against the
// compiled roster and enqueue a JournalRewardCommand of kind "atlas" on
// the sim.atlas_settlement family. durable/atlas is the settlement
// authority: counters, tier promotion and reward auto-settle happen
// inside its transaction; the durable op ledger (one command per source
// event) is the once-only fence — this package never persists progress.
//
// CHEST_SPOTTED and QUEST_CLUE events never reach intake (atlas.md
// § Unlock Sources). The PHAT_HIEN first-Seen marker is page-scoped:
// fish pages ride the settlement's FISH_RARE peak and hidden-chest
// pages ride CHEST_HIDDEN; all other first-Seen events report
// ATLAS_SEEN (see PeakSource).
package atlas
