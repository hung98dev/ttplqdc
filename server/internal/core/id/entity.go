package id

// RuntimeEntityID is the owner/epoch scope of transient simulation entity
// identities: unsigned 64-bit entity values are unique per
// simulation_owner_id + ownership_epoch lifetime, never substitute durable
// identity, and every client reference is revalidated against the live scope
// (ids.md § Runtime Entity IDs).
type RuntimeEntityID struct {
	Owner uint64
	Epoch uint64
}
