// Package guild is the durable owner of guild state (guild.md,
// guild_progression.md, ADR-0060/0062/0065): every guild C2S mutation
// commits inside its client.<ID> transaction — membership, roles,
// invites/applications, lifecycle, disband — and every Guild EXP /
// ritual / blessing input commits as a GUILD_EVENT journal command
// under family "guild.event" with the guild as receipt owner.
//
// Runtime-only surfaces (presence, gathering-slot tracking, fanout)
// live in global/guild; this package never imports sim, edge or
// global. Wire codes are produced only as journal outcomes — the edge
// renders S2C_GUILD_RESULT (649) from the committed outcome and pushes
// S2C_GUILD_STATE (628) snapshots built by SnapshotView.
package guild
