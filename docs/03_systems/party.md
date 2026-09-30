# Party
status: LOCKED

## Scope
Defines party identity, membership, leadership, invitations, disconnect behavior, cooperative reward hooks, UI, and dungeon/PvP integration.

## Identity / Capacity
Immutable `party_id`; `PARTY_MAX_MEMBERS = 5`; one character at most one party. Solo party valid. States ACTIVE/DISBANDED; IDs never reused.

## Membership / Leader
Each member stores character_id, monotonic join_sequence, online_state. Exactly one leader. Leader invites, kicks, transfers leadership, and initiates party-controlled content when required; no reward/combat privilege.

## Invitations
Lifetime `60s`; states PENDING/ACCEPTED/DECLINED/CANCELLED/EXPIRED. Only leader invites. A partyless character may also invite: the invite commit atomically creates a new ACTIVE party with the inviter as leader (join_sequence 1) and carries its `party_id`; if that invite then declines/expires/cancels the solo party remains (valid) until the leader leaves. There is no separate create message. Target online, partyless, passes social direct-interaction gate; capacity validated atomically at acceptance. Pending invites do not reserve slots.

## Leave / Leadership
Leave/kick removes party membership but not established dungeon membership or committed rewards. If leader leaves, lowest join_sequence remaining member becomes leader. Final leave disbands. Explicit transfer allowed.

## Disconnect
Disconnect does not remove membership. If leader disconnected for `120s` and another member online, leadership transfers to online member with lowest join_sequence. Full server restart does not persist ordinary world parties. Live membership is the ephemeral global runtime in `../04_architecture/service_boundaries.md`.


## Combat / Maps / Dungeon
Friendly-fire rules belong to combat/PvP. Party membership is global across maps/channels. Dungeon membership is an independent snapshot after creation; party mutation never silently changes it.

## EXP Sharing
Ordinary monster EXP formula and all eligibility conditions are canonical in `../02_world/monsters.md`; the 30m proximity boundary is defined there.
Boss/dungeon EXP follows owning content.

## Currency / Loot
No generic party currency split or wallet. Loot mode = PERSONAL. No round-robin/need-greed/FFA initially.

## Matchmaking
No generic PvE dungeon finder / auto-fill matchmaker initially. PvP owns its matchmaking.

## Safe-Anchor Party Board
Each safe anchor may show a **party board** (presentation + invite only):
- A character not in combat may post `dungeon_id` + desired size `2..5` for `120s`.
- Another character may send the existing party **invite** (`60s` lifetime) to the poster or accept if the poster is leader.
- The board never creates a match, teleport, or lock. It does not bypass `social.md` block/direct-interact gates.
- Max one live post per character. After a post expires or is cancelled, a `30s` repost cooldown applies before a new post can be created. WORLD chat is unchanged.

## UI
May show character ID, display name, class, level, leader, online state, allowed map ID, revision, board post. Exact coordinates are never party state.
## Revision / Idempotency
Every mutation increments `party_revision`. Concurrent operations preserve max 5, one party/character, exactly one leader. Retries cannot duplicate invite/join/remove/transfer.

## Invariants
```text
PARTY_MAX_MEMBERS = 5
loot = PERSONAL
party currency split = disabled
party membership != dungeon membership
EXP formula and reward range = see monsters.md
```
