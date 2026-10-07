// Package inventory owns the durable inventory subsystem: the
// character_inventories aggregate (capacity/revision), the two client
// command families inventory.mutate (wire 400) and inventory.expand
// (wire 428), the consumable shared-cooldown tracker, and the three
// post-attach/post-commit state-push payload builders (S2C_WALLET_STATE
// 432, S2C_INVENTORY_STATE 433, S2C_ENTITLEMENT_PANEL_STATE 435).
//
// Scope boundaries (task packet + wave-13 plan): entitlement.claim
// (418/419) execution is IMP-102's edge/entitlement grant; this package
// only reads account_iap_entitlements/account_entitlement_claims for
// the 435 projection. character_loadouts is IMP-012's write domain and
// is read here only for the 433 loadout projection. Items custody
// mutations go through durable/items; currency through durable/currency;
// progression points through a characters row update (515 revision
// contract).
package inventory
