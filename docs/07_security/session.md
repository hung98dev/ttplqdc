# Session Security
status: LOCKED

## Scope
Defines authenticated session lifecycle, character control ownership, duplicate login, resume, revocation, and stale-session rejection.

Related: `../04_architecture/authority.md`, `../05_network/reconnect.md`, ADR-0009, ADR-0030.

## Session Identity
A live authenticated gameplay session has:
```text
session_id
account_id
session_epoch
authenticated_at
expires_at / credential lineage
current_character_id when attached
simulation_owner_id when attached
ownership_epoch when attached
```

IDs are server-generated.

## Account Control (ADR-0030)
Exactly one live authenticated gameplay session exists per `account_id`.

At most one character is attached on that session. Two characters of the same account cannot be in the world at the same time, including on two devices or two client processes.

A gameplay command is accepted only when:
- connection is authenticated,
- session is not revoked/expired,
- `account_session_epoch` matches current account authority,
- attached character belongs to account,
- no other character of the account is attached,
- simulation ownership epoch matches current routing authority.

## Login Queue (ADR-0052)
```text
trigger        attached sessions >= WORLD_CCU_CAP (runtime config, ../08_scale_ops/capacity.md)
order          FIFO by first gameplay-ticket request time; one queue entry per account
response       SERVER_OVERLOADED with retry_after_ms and queue_position (ticket endpoint only)
admission      when a slot frees, the oldest entry's next ticket request succeeds; it must request it within 60 s of
               becoming position 1 or it loses its place
reconnect      an account with a character live or inside its grace window (reconnect.md) bypasses the queue on both the
               resume path and the ticket path
reservation    the queue slot is reserved when POST /api/v1/gameplay/ticket succeeds (ticket issued = admitted); the
               admission window is 60 s from ticket issue until S2C_HELLO_OK, then the slot is held while the session is on
               character select (no timeout) and is released on detach-to-logout, disconnect without grace, or logout
retry          retry_after_ms = 5000 when queue_position <= 10, else min(30000, 5000 + 1000 * floor(queue_position / 10))
attach         C2S_CHARACTER_ATTACH never returns SERVER_OVERLOADED; the ticket is the only admission gate
```
The queue lives in memory in the single world process; a restart clears it and clients simply retry.

## Duplicate Login
A newer successful **account** gameplay login, attach, or resume supersedes the older account session epoch. When a character of the account is live (in the world or inside reconnect grace) the new session re-attaches that same character exactly like a resume, whether the HELLO carried a ticket or a resume credential (`../05_network/messages.md` § Connection / Session, ADR-0069).

Old session:
- stops receiving gameplay authority,
- receives `SESSION_REPLACED` when possible,
- has future gameplay intents rejected,
- cannot reclaim authority through late packets.

Account-level parallel gameplay sessions are **not** allowed, even if they would control different characters.

Character switch: send `C2S_CHARACTER_DETACH` (10) or return to character select on the current session, then attach another owned character. The previous character is `OFFLINE` before the next attach commits. Success is `S2C_CHARACTER_DETACH_OK` (11).

## Resume
Resume credential is bound to the session lineage and, when attached, character identity. It is issued in every `S2C_HELLO_OK` and re-issued by `S2C_RESUME_CREDENTIAL` (16) every 300 s while the session is live (sliding rotation, ADR-0069); TTL 10 minutes from issue (`auth.md` § Credential Types), so a disconnect always leaves a credential valid for at least 5 minutes, longer than every grace window. At most two credentials of a session are valid at once (the newest and its predecessor); presenting the newest invalidates the predecessor. It is presented only in `C2S_HELLO` (`../05_network/protocol.md` § Handshake) and single-use: each successful HELLO rotates it. A resume inside the character's reconnect grace re-attaches that character without `C2S_CHARACTER_ATTACH` and bypasses the login queue.

Resume cannot:
- attach a different account,
- switch to a character not owned by the account,
- attach a second character while another of the account is still attached,
- restore an old superseded epoch,
- recreate authority from client-provided position/state.

Resume always resolves current server routing and sends a fresh authoritative baseline.

## Expiry
Credential expiry and transport disconnect are separate.

A bound gameplay connection may be allowed to continue until its session/security policy requires renewal, but a revoked session is terminated promptly.

Do not allow an expired refresh credential to create a new gameplay session.

## Logout
Explicit logout (`POST /api/v1/auth/logout`, `auth.md` § HTTPS Endpoints):
- revokes the intended session/refresh family,
- invalidates gameplay/resume tickets for that family,
- closes or deauthorizes the live gameplay connection,
- does not delete characters/account state.

## Security Events
Force session invalidation on events such as:
- password change (`auth.md` § Password Provider) or privileged account recovery,
- explicit revoke-all-sessions,
- confirmed credential compromise,
- account disable/ban,
- administrative security action.

## Device Metadata
Device/platform metadata may be recorded for security/risk signals but is not proof of identity.

Do not permanently bind account access to a device fingerprint without a separate recovery/product decision.

## Session Storage / Cache
Persistent session/revocation metadata may live in PostgreSQL.

Live Edge/session routing may cache current session state in memory for performance, but stale cache cannot resurrect a revoked/superseded session.

## Audit
Security-relevant events record:
- account/session IDs,
- event category,
- server timestamp,
- actor/provider context,
- reason/source,
- correlation/audit ID.

Secrets/tokens are redacted.

## Invariants
- one account -> one live gameplay session (ADR-0030)
- at most one attached character per account at a time
- newer account session epoch invalidates older control
- resume cannot change account ownership or dual-attach a second character
- logout/revocation removes gameplay authority
- device metadata is a signal, not identity proof
- stale session cache cannot restore authority
