# Protocol Versioning
status: LOCKED

## Scope
Defines gameplay protocol compatibility, schema evolution, client build gates, feature negotiation, and forced-upgrade behavior.

## Version Tuple
Connection handshake declares:
```text
protocol_major
protocol_minor
client_build
content_schema_version
client_content_revision
```

The server returns accepted/required values and feature flags.

## Major Version
A protocol-major change is required for incompatible wire/semantic changes such as:
- reusing/changing meaning of an existing message ID,
- changing required field interpretation incompatibly,
- changing operation/idempotency semantics incompatibly,
- replacing envelope semantics.

Server does not silently translate arbitrary incompatible majors.

Launch policy supports exactly the current major plus an explicitly configured previous major only when compatibility tests exist. Default is current major only.

## Minor Version
Minor versions are additive/backward-compatible changes:
- new optional protobuf fields,
- new message IDs not required by older clients,
- new feature flags with safe absence behavior.

Unknown optional fields are ignored by protobuf-compatible readers.

Removing/changing required semantics is not a minor change.

## Client Build Gate
Server bootstrap maintains:
```text
minimum_supported_build
recommended_build
current_build
```

If build < minimum:
- login/gameplay attach is rejected with `CLIENT_UPDATE_REQUIRED`.

A connected client is not force-disconnected solely because a newer optional build became available. Security/compatibility emergency policy may require reconnect/update explicitly.

## Content Compatibility
Static content activation follows `../06_data/config.md`.

A client may connect only when its bundled schema/assets can interpret the active server content revision under declared compatibility rules.

Stable IDs remain server truth. Client content cannot introduce an entity/reward/skill unknown to server authority.

## Feature Flags
Handshake may negotiate bounded protocol features.

Rules:
- flag names/IDs are stable,
- server never enables a feature the client did not declare support for,
- disabled feature has a defined fallback,
- combat/economy authority never shifts to client through a feature flag.

Feature flags are not a substitute for major-version change when semantics are incompatible.

## Server Deploy
A server deploy is a maintenance restart of the single world process (ADR-0052):
- the new build must accept the configured client compatibility window,
- clients outside that window receive `CLIENT_UPDATE_REQUIRED` on reconnect,
- content revision activation remains atomic and independent from process deployment.

## Schema Discipline
Generated protobuf schema changes require:
- no field-number reuse,
- removed fields remain reserved,
- enum numeric value reuse is forbidden,
- regression fixtures for old supported version,
- explicit migration for persisted data when relevant.

## ADR-0037 Field Additions and Older Clients

ADR-0037 added:
- Three secondary-result fields to `S2C_COMBAT_EVENT`: `reflect_damage_instance`, `lifesteal_heal_amount`, `absorb_shield_amount`.
- Five stat-delta fields to `S2C_STATE_DELTA`: `stat_lifesteal`, `stat_reflect`, `stat_absorb`, `stat_heal_reduction`, `stat_healing_received`.

These are new optional protobuf fields. They do not change existing field semantics. This is a **minor-version** change; `protocol_minor` increments and `content_schema_version` increments to reflect the expanded replication schema. `protocol_major` does not change.

Older connected clients that have not been upgraded:
- Receive the ADR-0037 fields silently per protobuf unknown-field rules (ignored).
- Render **nothing** (not zero) for absent secondary-result fields. A missing `reflect_damage_instance`, `lifesteal_heal_amount`, or `absorb_shield_amount` must not produce a zero floating-number VFX or any UI output. Absence means the effect did not trigger, and the client must treat it as no-event.
- Receive no upgrade prompt solely due to this minor version change; the existing build-gate policy applies.
- If the server requires these fields for correct gameplay presentation (e.g., a future mandatory mechanic), that constitutes a required-field semantic change and must be handled as a major-version increment at that time.

## Forced Upgrade
Forced upgrade is allowed for:
- incompatible protocol major,
- critical security fix,
- client data/assets incapable of rendering/interpreting active required content.

Error includes a stable reason code, not a free-text-only rejection.

## Invariants
- incompatible semantics -> major version,
- additive optional fields -> minor version,
- protobuf field/enum numbers are never reused,
- unsupported build cannot attach,
- server content is authoritative,
- feature negotiation never transfers authority to Unity.
