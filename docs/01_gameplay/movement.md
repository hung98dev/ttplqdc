# Movement
status: LOCKED

## States

```text
IDLE
RUN
JUMP
FALL
KNOCKBACK
```

Status effects are not movement states.

## Canonical Locomotion
- horizontal left/right movement
- air control
- double jump: maximum `2` jump actions before valid ground contact
- intentional drop through one-way platforms
- portal/map transfer

Not enabled as base locomotion:
- sprint
- wall jump
- ladder/climb
- swimming
- fall damage
- mount movement

Skills may define explicit dash/teleport/forced movement without adding a global locomotion state.

## Baseline Physics
Content/engine config uses these canonical gameplay targets:

```text
BASE_RUN_SPEED = 6.0 units/s
FIRST_JUMP_IMPULSE = 11.0 units/s
SECOND_JUMP_IMPULSE = 10.0 units/s
GRAVITY_MAGNITUDE = 28.0 units/s^2
MAX_FALL_SPEED = 20.0 units/s
AIR_CONTROL = 0.85
```

`MOVE_SPEED` multiplies base run speed according to `stats.md`.

## Transitions
- `IDLE -> RUN`: horizontal input.
- `RUN -> IDLE`: horizontal input released.
- `IDLE|RUN -> JUMP`: first jump while grounded.
- `JUMP|FALL -> JUMP`: second jump while airborne and available.
- `JUMP -> FALL`: upward velocity ends.
- `FALL -> IDLE|RUN`: valid ground contact; reset jump count.
- `ANY -> KNOCKBACK`: accepted displacement.
- `KNOCKBACK -> IDLE|RUN|FALL`: displacement ends.

## Platforms
- Solid platforms collide normally.
- One-way platforms can be passed from below.
- Drop-through requires down input + jump/action command and ignores the selected platform briefly.

## Controls
PC defaults:

```text
A / Left Arrow  -> left
D / Right Arrow -> right
Space           -> jump
S + Space       -> drop through one-way platform
```

Mobile uses a virtual horizontal control plus jump/action buttons. PC/mobile gameplay capability is identical.

## Forced Movement
- Forced movement is server-authoritative.
- ROOT/FREEZE interaction follows `status_effects.md`.
- Knockback cannot move a character outside valid map geometry.

## Out-of-Bounds Recovery
If authoritative simulation detects invalid world position:
1. return to last server-recorded safe ground position
2. if unavailable/invalid, use current map fallback spawn
3. if still invalid, respawn at active world checkpoint

Out-of-bounds recovery does not create a death penalty or reward.

## Authority and Prediction
- Client may predict local movement.
- Server validates speed, acceleration envelope, jump count, map geometry, and transitions.
- Invalid movement is corrected to authoritative state.
- Client coordinates never become persistent authority.
