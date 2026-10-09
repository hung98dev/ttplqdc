// Package equipment materializes the compiled equipment catalog into
// typed runtime definitions and independently re-checks the finite
// expansion invariants at activation time (IMP-026).
//
// equipment_catalog.md §Compiler Source Schema is a named finite
// expansion: 12 set headings x 14 canonical slots = 168 immutable
// `item.eq.<tier>.<set_key>.<slot>` definitions plus tier budgets, fixed
// stats, the closed 12-roll secondary pool, element Layout A/B, 2/4/6pc
// set effects, 2pc-derived support signatures and enhancement base
// costs. The content compiler emits the families; this package loads
// them into the typed Catalog the runtime consumes (IMP-012+) and
// re-derives the spec's Validation reject list inside the activation
// gate (config.md § Activation Checks). Checks never mutate and never
// trust the expansion to have run — every invariant is re-derived from
// the emitted records themselves.
package equipment
