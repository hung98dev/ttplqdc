// Package items is the item custody & binding primitive (items.md,
// data_model.md § Inventory / Item Ownership, physical_schema_contract.md,
// ADR-0029, ADR-0043, ADR-0063, ADR-0065). It owns item_instances +
// item_locations + beast_equipment_locations writes: every live instance has
// exactly one location row, binding is tighten-only, and direct-trade locking
// is runtime-only session state (never persisted).
package items
