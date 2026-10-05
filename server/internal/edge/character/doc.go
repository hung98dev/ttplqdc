// Package character is the edge/session side of the character
// lifecycle: the C2S_CHARACTER_CREATE (id 12) handler on the ADR-0081
// durable seam — session view -> JournalClientCommand -> Submit ->
// await terminal JournalOutcome -> deliver S2C_CHARACTER_CREATE_RESULT
// (13) then S2C_CHARACTER_LIST (14) -> Ack only after delivery.
package character
