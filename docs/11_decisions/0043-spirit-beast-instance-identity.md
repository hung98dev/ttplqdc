# ADR-0043: Spirit Beast Instance Identity
status: ACCEPTED

## Context
ADR-0019 defined Linh Thú as character-scoped companions with `beast_grant` already 1:1 per `(character_id, beast_id)` (duplicates become Linh Đan). A persistence sketch used `beast_instance_id` UUID for equipment locations, implying multiple live copies of the same `beast_id`. That UUID is unnecessary and would leak a second identity beside the content `beast_id`.

## Decision
No new UUID. Ownership is composite `(character_id, beast_id)`.

```text
character_beasts PK = (character_id, beast_id)
beast_equipment_locations PK = (character_id, beast_id, slot_id)
FK beast_equipment_locations -> character_beasts
item_locations.kind includes BEAST_EQUIPMENT_SLOT
```

Unequipped `BEAST_EQUIPMENT` lives in `CHARACTER_INVENTORY` (ADR-0019 inventory-storable). Equipped occupies `BEAST_EQUIPMENT_SLOT` only.

## Consequences
- Specs: `03_systems/spirit_beasts.md`, `03_systems/items.md`, `06_data/data_model.md`, `06_data/ids.md`.
- Implementers must not introduce `beast_instance_id`.
- Beast ops (`C2S_BEAST_*`) identify the companion by `beast_id` under the attached `character_id`.
