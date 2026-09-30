# ADR-0049 — Guild Storage Same-Account Transfer Prohibition

status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: Error codes are `GUILD_STORAGE_SAME_ACCOUNT` and `GUILD_MEMBERSHIP_TOO_NEW` (`../05_network/errors.md`). Every stored item carries `depositor_character_id`/`depositor_account_id` (ADR-0053), so the "depositor unknown" fallback never applies. Added rule: withdrawing another character's deposit requires 72h continuous guild membership, and such withdrawals feed the item-transfer anti-RMT signal. Canonical: `../03_systems/guild_storage.md`; enforcement task IMP-037.

## Context

`guild.md:42` allows multiple characters from the same account to be guild members simultaneously. `guild_storage.md` defines deposit/withdraw/Reserve-claim permissions by role only, with no check on whether the depositing and receiving characters share an `account_id`.

This creates a bypass of the inter-character transfer restrictions established by:
- **ADR-0029** (Character Resource Isolation): currency and items are isolated per character; no direct character-to-character transfer on the same account.
- **ADR-0041** (Anti-RMT Trade Gates): 24-hour character age gate, 5% fee, equipment floor, and anti-cheat rollup are bypassed if Guild Storage can relay items between same-account characters without those checks.
- **`non_goals.md:18`**: same-account item transfer is an explicit non-goal at launch.

Without an explicit account check, a player can deposit on Character A and withdraw on Character B of the same account, achieving the same effect as a direct item transfer while bypassing all ADR-0041 gates.

## Decision

Guild Storage deposit and withdraw (including Reserve claim delivery) are prohibited between characters sharing an `account_id`.

The server must:
1. At deposit time: verify the depositing character's `account_id` is not the same as any pending Reserve claimant on the same item/quantity.
2. At withdraw/delivery time: verify the withdrawing/receiving character's `account_id` differs from the depositing character's `account_id` that last put the item in storage. If the depositor's account identity cannot be determined (e.g., item pre-dates this rule), the restriction applies to the current transaction only (the withdrawing character's account must differ from any same-session depositor).
3. Emit `GUILD_STORAGE_SAME_ACCOUNT_REJECTED` on violation.

Note: the server already enforces that direct trade cannot occur between same-account characters (`trading_auction.md`). This ADR extends the same principle to Guild Storage.

## Consequences

- `guild_storage.md` §Eligibility updated to document the prohibition.
- `messages.md` will need `GUILD_STORAGE_SAME_ACCOUNT_REJECTED` registered as a domain error code when the wire message section for Guild Storage is defined.
- IMP task needed for server enforcement of the account check.

## Specs changed

- `docs/03_systems/guild_storage.md` (Eligibility section)
