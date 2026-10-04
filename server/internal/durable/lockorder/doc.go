// Package lockorder owns the canonical aggregate lock acquisition of
// database.md § Lock Order. Every multi-aggregate durable transaction
// acquires its lock set through this helper in canonical order —
// aggregate-type priority, listed table order within a priority, stable
// key byte order — before any mutation. An owning feature may define a
// stricter deterministic order; every other caller must use this order.
package lockorder
