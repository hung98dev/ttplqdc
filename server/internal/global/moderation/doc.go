// Package moderation is the global-runtime communication-restriction
// surface (social.md § Moderation): the send-admission consult behind
// social.Service's Moderation port — canonical NONE/MUTED restriction
// with channel subsets and server-authoritative expiry — plus the
// automated unsafe/spam filter that may reject but never sanctions.
//
// Restriction state is runtime-owned here: an operator-issued mute is
// an in-memory verdict with an explicit expiry (a server restart drops
// restrictions like every other global surface). SQL never enters this
// package — the chat_messages log and player_reports case/evidence
// reads live in durable/chat (PRIV-004).
package moderation
