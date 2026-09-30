# NPCs
status: LOCKED

## Scope
Defines NPC identity, placement, interaction, dialogue, shops/services access, movement, schedule, and server authority. Quest/economy logic stays in owning specs.

## Identity / Capabilities
Stable `npc_id`; runtime `npc_instance_id`; localized display names. Capabilities: DIALOGUE, QUEST, SHOP, SERVICE, STORY, DECORATIVE.

## Interaction
Client requests runtime NPC; server validates same instance, state/range, resolves allowed actions; every gameplay-changing follow-up is revalidated. Default range = `2.5m` equivalent. Normal interaction rejected while `in_combat`.

Gameplay-capable sessions expire after `30s` and close on range exit, transfer, despawn, death, conflicting transaction, or expiry.

## Dialogue / Quest / Shop
Dialogue uses stable nodes/responses and localization keys. NPCs reference quest IDs rather than duplicate quest logic. Shop references `shop_id`; server validates every price/stock/balance/capacity transaction.

## Service NPC
Initial service catalog:
```text
service.set_checkpoint
service.travel
service.crafting
service.enhancement
service.respec
service.storage.account
service.auction
```
`service.storage.account` opens the account-level IAP entitlement panel in `../03_systems/account_storage.md` under ADR-0029. No gameplay item vault or alt bank exists.
There is no repair service.

Wire mapping is canonical in `../05_network/messages.md`: dialogue responses are `C2S_INTERACT` `TALK` with `service_param` = dialogue option; `service.set_checkpoint` and `service.travel` are `C2S_INTERACT` `NPC_SERVICE` (`service_param` = travel destination); the other services and quest accept/turn-in use their dedicated messages carrying `npc_id`, validated against the open NPC session and range. A service or option outside the NPC's allowed set, or an expired session, is rejected.

## Travel / Respec
Travel destinations are predefined map+spawn pairs; access/cost/combat state revalidated. Respec only opens the canonical progression action.

## Movement / Schedule
Movement: STATIC default, PATROL, SCRIPTED. Optional schedule DAY_ONLY/NIGHT_ONLY/EVENT_CONDITION/QUEST_CONDITION. Critical progression services must have an always-available equivalent.

## Shared State / Concurrency
Character-specific by default. Shared NPC state must reference authoritative world/event key. Multiple players may interact simultaneously; underlying transaction systems own limited resources, not a global NPC mutex.

## Dynamic / One-Time
Dynamic NPCs belong to map/event context; despawn invalidates sessions. One-time rewards/progression are idempotent.

## Required Data
Definition: npc_id, capabilities, visual_id, interaction_range, interactions. Placement: npc_id, map_id, spawn/position, facing, movement_mode, conditions.

## Invariants
```text
npc_id != npc_instance_id
normal interaction while in_combat = rejected
personal general-purpose storage = disabled
account storage service opens IAP panel only (ADR-0029)
client never authorizes reward/price/teleport result
```
