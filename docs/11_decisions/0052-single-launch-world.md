# ADR-0052: Single Launch World
status: ACCEPTED

## Context
ADR-0044 allowed "additional independent worlds, each with its own PostgreSQL" when one process is full. No spec defined world selection, cross-world account/IAP visibility, character-count or name uniqueness across databases, while `glossary.md` defines one persistent logical world. Owner decision (2026-09-24): the game has exactly one world; the world has many maps and each map has many channels ("Khu").

## Decision
- Launch runs exactly **one logical world**: one `thinhthan-server` process and one PostgreSQL database. Additional worlds are not supported; adding them later requires a new ADR.
- Capacity inside the world: 24 open-world maps × 30 channels × 18 players (ADR-0020/0035) plus instances.
- `WORLD_CCU_CAP` (runtime config) = the CCU measured by the 10k release gate on production hardware. When authenticated sessions reach it, new logins wait in a FIFO login queue; the client retries with the `retry_after_ms` returned with `SERVER_OVERLOADED`. Connected players are never kicked for capacity.
- If the measured cap is below 10,000, the release gate fails and the fix is performance work or bigger hardware, never a second world.
- Deploys restart the single process in a maintenance window: `S2C_SERVER_DRAINING`, stop new logins, drain transfers/instances within budget, restart, clients reconnect through checkpoint recovery.

## Consequences
- ADR-0044 §3 "additional world processes" is superseded.
- `../04_architecture/system_overview.md`, `../08_scale_ops/capacity.md`, `sharding.md`, `deployment.md`, `../09_testing/load.md`, `../07_security/session.md` describe one process, the CCU cap and the login queue.
- Account, character-count, `name_key`, `username_key`/`email_key` and IAP entitlements live in the single database; no cross-world rule is needed.
