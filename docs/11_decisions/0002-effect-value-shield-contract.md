# ADR-0002: Typed Heal, Shield, and Resource Effects
status: ACCEPTED

## Context
Launch content already uses percent-MAX_HP healing, percent-MAX_HP shields, percent-MAX_MP restoration, incoming-heal modifiers, shield-strength modifiers, and shield-break triggers. The previous generic stat/combat specs only defined a flat/source-stat healing formula and did not define shield lifecycle or absorption order. Different implementations could therefore produce incompatible results.

## Decision
- Keep HP and MP as the only character combat resources.
- Model heal/shield/resource results with explicit typed components such as `flat`, `source_stat_coefficient`, and `target_max_*_ratio`; never encode percent effects as ambiguous flat decimals.
- Resolve damage through defense/reduction, then shields, then HP.
- Give shields explicit runtime identity/amount/expiry and deterministic multi-shield absorption order.
- Distinguish `SHIELD_BROKEN`, `SHIELD_EXPIRED`, and `SHIELD_REMOVED`.
- Treat `takes hostile damage` as positive post-mitigation damage before shield absorption; use `takes HP damage` when a trigger specifically requires HP loss.
- Keep targeting identifiers synchronized with `skills.md`.

## Consequences
- Existing skill/set/Soul/Meridian/Formation content has one implementation interpretation.
- Shield-break effects no longer fire on natural expiry or replacement.
- Percent healing/resource effects remain easy to balance without adding new character stats/currencies.
- Static validation must reject ambiguous effect-value encoding.
