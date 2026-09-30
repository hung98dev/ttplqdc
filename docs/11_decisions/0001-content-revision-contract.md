# ADR-0001: Versioned Static Content Bundles
status: ACCEPTED

## Context
Gameplay, world, and system rules are increasingly data-driven. Persistent IDs, reward settlement, combat encounters, and live operations must not silently reinterpret already-created state when content changes.

`docs/06_data/config.md` already defines stable IDs, content revisions, atomic activation, validation, rollback, and restricted hot reload. This materially affects the data contract and implementation architecture, so the decision requires an ADR.

## Decision
- Treat launch/static content as a versioned dependency graph identified by `schema_version` and `content_revision`.
- Persistent definition IDs are immutable; display/localization text is not identity.
- Validate required references, enums/ranges, ownership/currency references, probability data, and effect/stat references before activation.
- Activate a validated revision atomically.
- Encounters/operations that require deterministic interpretation may pin the revision they started with.
- Static content is not hot-reloaded by default. Only explicitly `hot_reload_safe` tuning fields may change without controlled activation/restart.
- Rollback changes the active definitions but never rewinds committed player state or transactions.
- Breaking interpretation changes require an explicit migration.

## Consequences
- Content tooling needs dependency validation and revision packaging before production activation.
- Runtime services must carry enough revision context to settle in-flight combat/rewards deterministically.
- Designers can tune explicitly safe fields without making the client authoritative.
- Content deployment has more ceremony, but avoids mixed-revision rewards, dangling IDs, and silent corruption of persistent state.
