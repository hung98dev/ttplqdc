# Guild Storage
status: LOCKED

## Scope
Defines two shared Guild vault sections, ownership, capacity, permissions, claims, quotas, audit, and anti-duplication.

## Ownership / Sections
Stored ownership context = GUILD_STORAGE; depositor retains no personal claim. Sections exactly COMMON and RESERVE. COMMON unlock Lv1; RESERVE Lv10.

## Capacity
COMMON: 60/80/100/120/140/160/180 across Lv1-4/5-9/10-14/15-19/20-24/25-29/30.
RESERVE: 0/0/40/50/60/70/80 on the same bands. Capacity reduction never deletes items; deposits stop if over capacity.

## Eligibility
Only UNBOUND + transfer-compatible + storage-allowed items. CHARACTER_BOUND/ACCOUNT_BOUND forbidden. Equipment must be unequipped; Soul Contract resolved; transaction-locked items forbidden.

**Same-account transfer prohibition (ADR-0049)**: A character may not withdraw (or receive a Reserve claim delivery of) an item that was deposited by a **different character on the same account**. Each stored item keeps `depositor_character_id` and `depositor_account_id`; the check compares them with the receiver. A character may withdraw another character's deposit only after `72h` of continuous membership in the guild, and every such withdrawal is counted in the item-transfer signal (`../07_security/anti_cheat.md`), so storage cannot bypass the ADR-0041 trade gates as a free anonymous channel. At deposit time the server also rejects a deposit into an item line that has a pending Reserve claim by a different character on the depositor's account. Rejections: `GUILD_STORAGE_SAME_ACCOUNT`, `GUILD_MEMBERSHIP_TOO_NEW` (`../05_network/errors.md`). The server must reject cross-character guild storage operations where both source and destination characters share an `account_id`. This prohibition applies to both direct withdraw and Reserve claim delivery. Bypassing it would circumvent the ADR-0041 character isolation, age-gate, and anti-RMT fee requirements.

## Permissions
Deposit: Leader/Vice/Officer COMMON+RESERVE; Member COMMON.
COMMON withdraw: Leader/Vice unlimited operation count; Officer 20/day; Member 5/day; reset 00:00 UTC.
RESERVE direct withdraw: Leader/Vice only.

## Claims
Officer/Member may request Reserve item. States PENDING/APPROVED/REJECTED/CANCELLED/EXPIRED/COMPLETED.
PENDING lifetime = `72h`. Approvers Leader/Vice.
Approval reserves exact item/quantity.

APPROVED lifetime = `7 days` from approval. If not delivered before expiry:
```text
APPROVED -> EXPIRED
reservation released
item remains in GUILD_STORAGE
```
Full recipient inventory leaves claim APPROVED until delivery/cancel/expiry. Requester/Leader/Vice may cancel. Leaving/kick cancels pending/approved claims.

## Section Move
Leader/Vice/Officer may move COMMON<->RESERVE when destination capacity exists and quantity not reserved. Member may not.

## Item State / Failure
Storage never rerolls/resets item instance state. Withdrawal/delivery all-or-nothing. No mailbox/ground/conversion.

## Audit
Record operation, guild, actor, action, section, item, quantity, receiver, before/after, timestamp. Actions include deposit/withdraw/move/claim lifecycle. Officer+ see full player-facing log; Member own actions + public summary. Gameplay retention target 180 days.

## Revision / Idempotency
Every commit increments guild_storage_revision and uses stable operation ID. Retry cannot duplicate transfer/reservation/delivery.

## Disband
Rejected unless occupancy=0 and no approved reservation.

## Invariants
```text
only UNBOUND accepted
COMMON Lv1; RESERVE Lv10
Member common withdraw 5/day
Officer 20/day
Reserve direct = Leader/Vice
PENDING expires 72h
APPROVED expires 7d and releases reservation
non-empty storage blocks disband
```
