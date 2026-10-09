// Package moderation is the global-runtime communication-restriction
// and report-case surface (social.md § Moderation / § Reports): the
// send-admission consult behind social.Service's Moderation port —
// canonical NONE/MUTED restriction with channel subsets and
// server-authoritative expiry — plus the automated unsafe/spam filter
// that may reject but never sanctions, and the report case/evidence
// retention semantics on top of player_reports (PRIV-004).
//
// Restriction state is runtime-owned here: an operator-issued mute is
// an in-memory verdict with an explicit expiry (a server restart drops
// restrictions like every other global surface); persistence of the
// moderation log itself lives in durable/chat — never in this package.
package moderation
