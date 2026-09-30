# ADR-0030: One Account, One Live Session
status: ACCEPTED

## Context
An account may own three isolated characters (ADR-0029). ADR-0009 and `session.md` allowed one controlling epoch **per character**, and left open account-level multiple sessions if they did not control the same character. That would permit two devices (or two clients) to play two characters of one account at once.

Product rule: one account is logged in on **one machine at a time**. Multiple characters do not allow concurrent play.

## Decision
1. Exactly one live authenticated **gameplay** session exists per `account_id`.
2. At most one character is attached (`current_character_id`) on that session.
3. A newer successful account login / gameplay attach / resume on another connection increments `account_session_epoch`, sends `S2C_SESSION_REPLACED` to the old connection when feasible, and rejects all old-epoch intents.
4. Attaching a second character while another is attached is rejected. Switching character requires detach (or return to character select) on the same live session, then attach the other character. The previous character goes `OFFLINE`.
5. Same-account characters still cannot share gameplay resources (ADR-0029). This ADR only constrains **presence**, not items.
6. HTTPS token refresh without a gameplay attach does not place a character in the world. Creating a second gameplay WebSocket for the same account is duplicate login (rule 3).

## Consequences
- No dual-box of two owned characters.
- Relog on a new device kicks the old device.
- Session epoch is account-scoped; character attach is a field on that epoch, not a second live epoch.
