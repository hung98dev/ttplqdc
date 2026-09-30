# Death and Respawn
status: LOCKED

## Death
A character dies when:

```text
current_hp <= 0
```

Server atomically:
1. clamps HP to `0`
2. enters `DEAD`
3. cancels invalid unresolved actions
4. stops voluntary movement
5. clears target
6. applies status cleanup
7. records death context

Death processing is idempotent.

## Dead State
Dead characters cannot:
- move/jump voluntarily
- attack/cast/use combat items
- interact with normal NPCs/portals
- receive normal healing

They may use UI/chat and request respawn when available: in the normal world the client sends `C2S_RESPAWN_REQUEST` once `RESPAWN_DELAY` has elapsed since death (earlier or non-dead requests reject with `INVALID_STATE`; `../05_network/messages.md`). A character that never requests stays `DEAD`, including across reconnect. Dungeon, PvP and Guild War respawn is server-driven by that content's rules below.

## Status and Cooldown Cleanup
- Temporary statuses are removed according to `status_effects.md`.
- Normal skill cooldowns continue counting down while dead.
- Normal summons are removed on owner death.

## World Respawn
Canonical normal-world delay:

```text
RESPAWN_DELAY = 3 seconds
```

Flow:

```text
DEAD -> RESPAWNING -> ACTIVE
```

Server resolves the character's marked checkpoint and restores:

```text
current_hp = floor(MAX_HP * 0.40)
current_mp = floor(MAX_MP * 0.40)
```

## Checkpoint
- Each character always has exactly one valid world checkpoint.
- Initial checkpoint: `checkpoint.lang_da.dinh_lang`.
- Interacting with an enabled world shrine while not `in_combat` may replace the checkpoint.
- Client cannot submit arbitrary respawn coordinates.
- Invalid stored checkpoint falls back to `checkpoint.lang_da.dinh_lang`.

## Respawn Protection (Invulnerability)
Respawn grants:

```text
duration = 3 seconds
```

of invulnerability.

During invulnerability:
- Incoming hostile player, monster, and boss damage is reduced to 0 and harmful statuses are ignored.
- Character may move, use skills, and attack freely; offensive actions do not cancel invulnerability early.
- Outgoing damage dealt by the character during invulnerability is `0`:
  ```text
  outgoing_damage = 0
  ```
- Both invulnerability and the zero outgoing damage expire simultaneously after exactly 3.0 seconds.

## Penalty
Normal death causes no loss of:
- EXP/level
- skill or potential points
- inventory/equipment
- enhancement
- currency
- Guild contribution

There is no equipment durability and no item/currency drop on death.

## Content Overrides
Dungeons, bosses, and PvP may override:
- respawn location
- respawn delay
- encounter reset/wipe behavior
- whether respawn occurs inside the content

They may not silently introduce permanent item/currency/EXP loss without updating this specification.

## Player Revival
Player-to-player revival is not enabled initially.

## Disconnect While Dead
Reconnect restores the authoritative dead/content state. Disconnect cannot produce free healing, checkpoint movement, or a duplicate respawn.

## Persistence
Persist the active checkpoint and any content-specific death state required for reconnect/recovery. Client state is never sufficient authority.

## Invariants
```text
respawn_invulnerability_duration = 3s
respawn restores 40% MAX_HP and 40% MAX_MP
incoming hostile damage = 0, harmful statuses ignored
outgoing damage = 0 during invulnerability
offensive action does not cancel invulnerability early
```
