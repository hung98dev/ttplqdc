# ADR-0009: Account Session and Gameplay Credential Model
status: ACCEPTED

## Context
The Unity client needs revocable authentication and fast gameplay reconnect without trusting long-lived secrets inside the realtime protocol. The Go backend must support PC/mobile, duplicate-login replacement, and one authoritative character session while keeping PostgreSQL as durable account/session truth.

## Decision
- `account_id` is the canonical identity and is independent of login provider.
- HTTPS authentication issues opaque high-entropy bearer credentials; token contents are not trusted client claims.
- Access credentials are short-lived; refresh credentials are rotating/revocable and stored/compared server-side in non-recoverable form.
- Gameplay WebSocket bootstrap uses a short-lived single-purpose gameplay ticket, not the refresh credential.
- Reconnect uses a short-lived resume credential bound to account/session/character lineage.
- One **account** has one active gameplay session epoch (ADR-0030). One character is attached at a time. Newer authenticated attach/reconnect invalidates older epochs, including another device playing a different character of the same account.
- External/OIDC/platform identities may link to the same account through explicit verified provider identity records. Provider choice is deployment/product configuration, not character identity.
- If first-party password authentication is enabled, passwords are stored only as modern salted password hashes; plaintext/reversible password storage is forbidden.

## Consequences
- Realtime packets do not carry long-lived refresh credentials.
- Revocation and duplicate-login semantics are explicit.
- Login-provider changes do not change persistent `account_id` or character ownership.
- Authentication/session rows add durable state, but gameplay packet validation uses the already-bound session epoch instead of querying PostgreSQL per message.
