// Package inventory is the edge admission surface for the inventory
// durable families (400 inventory.mutate, 428 inventory.expand) plus
// the post-attach/post-commit REPLACEABLE_STATE pushes 432/433/435.
// Claim intent 418 and entitlement execution belong to
// edge/entitlement (IMP-102); this package only projects the 435
// panel state.
package inventory
