# Dungeons
status: LOCKED

## Scope
Defines dungeon identity, membership, entry, stages, checkpoints, wipe, reconnect, completion, rewards, cleanup, recovery.

## Identity / Types
Stable `dungeon_id`/`stage_id`; runtime `dungeon_instance_id`/`map_instance_id`. Types SOLO and PARTY; PARTY `1..5` according to content.

Each launch dungeon uses its `dungeon_id` as geometry `space_id` and has exact bounds plus a distinct `layout_profile` in `../07_content/dungeon_catalog.md` (ADR-0046). Bounds are an outer envelope; stage topology and exported collision define the walkable shape.

## Difficulty
Launch supports exactly `NORMAL`. There is no generic Challenge/Hard difficulty at launch. Max-level PvE uses configured Level-60 NORMAL dungeons/bosses with endgame reward tables and mechanics. A future challenge mode requires a separate explicit rules update and meaningful mechanics, not stat inflation.

## Entry / Membership
Validate unlock/alive/not in combat/requirements/player count/transition. Membership snapshots approved members at creation and is independent of later party mutation. No new member after ACTIVE; original snapshot members may first-enter until final encounter engagement.

Entry flow (fields, prompt lifetime and rejections canonical in `../05_network/messages.md` `C2S_DUNGEON_ENTER_REQUEST` / `C2S_DUNGEON_ENTRY_RESPOND` / `C2S_DUNGEON_ENTRY_CANCEL` / `C2S_DUNGEON_LEAVE`): the requester (partyless character or party leader) at the entrance sends the request with its `run_tag` (`ENDGAME_L60` only as allowed by `../07_content/dungeon_catalog.md`); party members on the same map instance approve or decline the pending entry; the approved set (requester included) becomes the snapshot. The instance records the requester's `source_map_id` and `source_channel_id` at creation (runtime only; not durable): return transfers and relic spawns use them.

Entry validation order for `C2S_DUNGEON_ENTER_REQUEST` (ADR-0062); the first failing step rejects:
```text
1 re-entry   sender is a non-ABANDONED snapshot member of a non-terminal instance of this dungeon_id
             -> re-enter that instance; only alive, not in combat, not transferring are checked
             (party role, story gate and claim cap are skipped)
2 role       sender is partyless or the party leader, else PERMISSION_DENIED
3 sender     range (OUT_OF_RANGE), level (LEVEL_TOO_LOW), alive / not transferring / no pending entry
             (STATE_CONFLICT), not in combat (IN_COMBAT), story gate (STORY_CHOICE_REQUIRED,
             ../07_content/quest_catalog.md), pending Reward Claims >= 100 (CLAIM_CAP_REACHED)
4 create     SOLO dungeon or partyless sender -> instance now; otherwise one pending entry (30s prompt)
```
`C2S_DUNGEON_ENTRY_RESPOND` ACCEPT revalidates the member with step 3 (range excluded; same map instance as the requester instead). At instance creation (all prompted members responded or the prompt expired) every ACCEPTED member is revalidated: online, alive, not in combat, not transferring, still in the requester's party and on the requester's map instance; a failing member becomes `INELIGIBLE` and is excluded from the snapshot. A leader change, the requester leaving the party, disconnecting, dying or changing map instance while the prompt is pending cancels it (`outcome = CANCELLED`); nothing is created.

The Act-VI finale `instance.finale.than_trung` (`../07_content/world_route_catalog.md`) is a PARTY `1..5` instance that follows every rule of this file (entry, membership, states, scaling, checkpoints, wipe, completion, re-entry, cleanup, restart) with its `space_id` in place of `dungeon_id` and `boss.than_trung` as its final encounter (ADR-0061).

## States / Stages
```text
CREATING -> READY -> ACTIVE -> COMPLETED/FAILED -> CLOSING -> CLOSED
```
Ordered main path with optional short rejoining side routes. Encounters may be combat/objective/survival/boss.

## PARTY Encounter Scaling
A PARTY dungeon remains playable with `1..5` members without requiring separate difficulty modes.

At the start of each configured major combat encounter/final boss:
```text
n = count of snapshot members who are currently inside the dungeon,
    alive or eligible to respawn, and not ABANDONED
n = clamp(n, 1, 5)
```
Freeze `n` for that encounter attempt.

Default launch scaling:
```text
hp_multiplier     = 1 + 0.55 * (n - 1)
damage_multiplier = 1 + 0.04 * (n - 1)
```
Therefore:
```text
n=1 -> HP 1.00x, damage 1.00x
n=5 -> HP 3.20x, damage 1.16x
```

Content may explicitly opt out or use a different authored multiplier only when the encounter definition declares it.

Rules:
- current HP percentage is preserved if an encounter explicitly resamples before activation; active attempts do not live-rescale
- a reconnect during an active attempt does not change `n`
- after a wipe/reset, the next attempt samples `n` again
- disconnecting/leaving cannot reduce the already-active boss HP multiplier
- add count/mechanics do not automatically multiply with `n`; encounter content owns them

This scaling applies to combat durability, not reward count. Reward eligibility remains per character.

## Checkpoints / Death / Wipe
Internal checkpoints never overwrite world checkpoint. Default death: `5s` then latest dungeon checkpoint full HP/MP, no progression/item/currency loss. Wipe resets current encounter, preserves prior completed stages, no default life counter.

## Completion / Eligibility
Completion requires mandatory stages/final objective. Reward key = `dungeon_instance_id + character_id`. Eligible member entered, contributed to mandatory content, and did not abandon. Disconnect grace `120s`.

## Reward Slots
Canonical reward-slot meanings:
```text
REPEAT      -> every eligible completion; includes character EXP (see dungeon_catalog.md dungeon_repeat_exp table)
FIRST_CLEAR -> once per character for that configured content definition, lifetime until migrated/reset explicitly
DAILY_FIRST -> once per character per UTC day
```

Repeat completion of a dungeon grants character EXP via the `DUNGEON_REPEAT` channel (18% of the act EXP budget). The per-act `dungeon_repeat_exp` values are derived in `dungeon_catalog.md` and must be used by the settlement system.

`FIRST_CLEAR` and `DAILY_FIRST` are reward gates only; neither is an entry lockout.

A dungeon may expose any subset of these slots. Retry/reconnect/restart must be idempotent per slot.

## Rewards
Personal configured EXP/common/bound/material/consumable/equipment/progression rewards. Boss rewards are separate unless content explicitly combines them. Earned bundles that cannot fit use `../03_systems/reward_claims.md`.

Default lockout = `NONE`.

## Re-entry / Cleanup
Membership survives disconnect; guaranteed same-instance grace `120s`. Voluntary exit (`C2S_DUNGEON_LEAVE` `EXIT`) may re-enter through the entrance before terminal lock; `ABANDON` removes completion eligibility and cannot re-enter. Empty instance `10m` -> FAILED. Completion closing = `120s` then transfer out. Every transfer out (exit, abandon, FAILED/CLOSING) goes to `spawn.return.<region>.<dungeon_key>` in the recorded `source_channel_id` under forced placement (`world_rules.md` § Forced Placement).

## Restart
Active runtime instances are not reconstructed initially; incomplete run fails/closes, committed rewards remain, no duplicate settlement.

## Target Session
`15-25m`; normally `2-4` meaningful stages + final boss + optional short secret. Avoid trash corridors/backtracking.

## Invariants
```text
PARTY capacity <= 5
launch difficulty = NORMAL only
each launch dungeon resolves exact bounds + one layout_profile
PARTY encounter scaling samples once per attempt
Lv60 endgame may use NORMAL endgame-tagged dungeons
party membership != dungeon membership
lockout = NONE
FIRST_CLEAR != DAILY_FIRST
first/daily reward gate != entry lockout
REPEAT includes character EXP (DUNGEON_REPEAT channel)
earned overflow -> reward_claims
```
