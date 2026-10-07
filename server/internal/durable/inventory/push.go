package inventory

import (
	"context"
	"encoding/json"
	"sort"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Push builders produce the REPLACEABLE_STATE payloads for 432/433/435.
// They run post-attach and after every committed inventory/wallet/
// loadout/entitlement change (plan § contract map) and read the rows
// under the caller's transaction (a pgx.Tx in executor scope or a
// consistent read tx the edge opens),
// never mutating.

// WalletPush builds S2C_WALLET_STATE (432): balances + SUM(revision).
func (s *Store) WalletPush(ctx context.Context, charID id.UUID) (*protocolv1.S2CWalletState, error) {
	w, err := s.wallet(ctx, s.pool, charID)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CWalletState{WalletRevision: uint64(w.Revision)}
	for _, b := range w.Balances {
		out.Balances = append(out.Balances, &protocolv1.WalletBalance{
			CurrencyId: b.CurrencyID,
			Amount:     b.Amount,
			Cap:        b.Cap,
		})
	}
	return out, nil
}

// InventoryPush builds S2C_INVENTORY_STATE (433): capacity, revision,
// slots (locked_quantity merged from the live trade-lock ledger per
// ADR-0064), all 3 loadouts, loadout_revision = SUM(loadout revisions).
func (s *Store) InventoryPush(ctx context.Context, charID id.UUID,
	ledgerLocked func(instanceID id.UUID) int) (*protocolv1.S2CInventoryState, error) {
	st, err := s.loadState(ctx, s.pool, charID)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CInventoryState{
		Capacity:          uint32(st.Capacity),
		InventoryRevision: uint64(st.Revision),
	}
	for _, sv := range st.Slots {
		iv := &protocolv1.InventorySlotView{Slot: sv.Slot}
		if sv.Item != nil {
			iv.Item = itemView(sv.Item)
			if ledgerLocked != nil {
				iv.LockedQuantity = uint32(ledgerLocked(sv.Item.InstanceID))
			}
		}
		out.Slots = append(out.Slots, iv)
	}
	var loadoutRev int64
	for _, l := range st.Loadouts {
		loadoutRev += l.Revision
		lv := &protocolv1.LoadoutView{LoadoutId: l.LoadoutID, IsActive: l.IsActive}
		for _, ls := range l.Slots {
			lsv := &protocolv1.LoadoutSlotView{SlotId: ls.SlotID}
			if ls.Item != nil {
				lsv.Item = itemView(ls.Item)
			}
			lv.Slots = append(lv.Slots, lsv)
		}
		out.Loadouts = append(out.Loadouts, lv)
	}
	out.LoadoutRevision = uint64(loadoutRev)
	return out, nil
}

// EntitlementPush builds S2C_ENTITLEMENT_PANEL_STATE (435): every
// account-level entitlement row with its per-character claimed tiers;
// claimable tier ids are the GRANTED rows' configured tiers not yet
// claimed for this character (monetization.md § panel). The push is a
// pure projection — claim execution stays with edge/entitlement.
func (s *Store) EntitlementPush(ctx context.Context,
	accountID, charID id.UUID, claimableTiers func(entitlementID string) []string) (*protocolv1.S2CEntitlementPanelState, error) {
	rows, err := s.entitlementPanel(ctx, s.pool, accountID, charID)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CEntitlementPanelState{}
	for _, r := range rows {
		v := &protocolv1.EntitlementView{
			EntitlementId:   r.EntitlementID[:],
			ProductId:       r.ProductID,
			EntitlementType: entitlementTypeOf(r.EntitlementType),
			GrantState:      grantStateOf(r.GrantState),
			SeasonNumber:    uint32(r.SeasonNumber),
			ClaimDeadlineAt: r.ClaimDeadlineAtMs,
			ClaimedTierIds:  r.ClaimedTierIDs,
		}
		if claimableTiers != nil {
			v.ClaimableTierIds = claimableTiers(r.EntitlementID.String())
		}
		out.Entitlements = append(out.Entitlements, v)
	}
	return out, nil
}

// itemView projects one ItemRow to the wire ItemInstanceView
// (messages.md § item views): stats arrays come from the ADR-0012
// schema-versioned item_state JSONB, sorted by stat_id.
func itemView(it *ItemRow) *protocolv1.ItemInstanceView {
	v := &protocolv1.ItemInstanceView{
		ItemInstanceId:   it.InstanceID[:],
		ItemId:           it.ItemID,
		Quantity:         uint32(it.Quantity),
		ContentRevision:  it.ContentRevision,
		EffectiveBinding: bindingOf(it.EffectiveBinding),
		EnhancementLevel: uint32(it.EnhancementLevel),
	}
	st := decodeItemState(it.ItemState)
	v.RolledBaseStats = st.RolledBase
	v.RolledSecondaryStats = st.RolledSecondary
	v.EffectiveStats = st.Effective
	return v
}

// itemState is the decoded ADR-0012 item_state JSONB. Version 1 shape:
// {"version":1,"rolled_base_stats":[{"stat_id":"...","value":N}],
// "rolled_secondary_stats":[...],"effective_stats":[...]} — equipment
// only; '{}' and unknown versions decode to empty lists (future
// writers bump version and extend this decoder, never decode fuzzily).
type itemState struct {
	RolledBase      []*protocolv1.StatValue
	RolledSecondary []*protocolv1.StatValue
	Effective       []*protocolv1.StatValue
}

func decodeItemState(raw []byte) itemState {
	var out itemState
	if len(raw) == 0 || string(raw) == "{}" {
		return out
	}
	var wire struct {
		Version         int `json:"version"`
		RolledBaseStats []struct {
			StatID string `json:"stat_id"`
			Value  int64  `json:"value"`
		} `json:"rolled_base_stats"`
		RolledSecondaryStats []struct {
			StatID string `json:"stat_id"`
			Value  int64  `json:"value"`
		} `json:"rolled_secondary_stats"`
		EffectiveStats []struct {
			StatID string `json:"stat_id"`
			Value  int64  `json:"value"`
		} `json:"effective_stats"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || wire.Version != 1 {
		return out
	}
	toStats := func(in []struct {
		StatID string `json:"stat_id"`
		Value  int64  `json:"value"`
	}) []*protocolv1.StatValue {
		if len(in) == 0 {
			return nil
		}
		stats := make([]*protocolv1.StatValue, 0, len(in))
		for _, s := range in {
			stats = append(stats, &protocolv1.StatValue{StatId: s.StatID, Value: s.Value})
		}
		sort.Slice(stats, func(i, j int) bool { return stats[i].StatId < stats[j].StatId })
		if len(stats) > 32 {
			stats = stats[:32]
		}
		return stats
	}
	out.RolledBase = toStats(wire.RolledBaseStats)
	out.RolledSecondary = toStats(wire.RolledSecondaryStats)
	out.Effective = toStats(wire.EffectiveStats)
	return out
}

// bindingOf maps the committed effective_binding VARCHAR to the wire
// enum.
func bindingOf(b string) protocolv1.ItemBinding {
	switch b {
	case "ACCOUNT_BOUND":
		return protocolv1.ItemBinding_ITEM_BINDING_ACCOUNT_BOUND
	case "CHARACTER_BOUND":
		return protocolv1.ItemBinding_ITEM_BINDING_CHARACTER_BOUND
	case "UNBOUND":
		return protocolv1.ItemBinding_ITEM_BINDING_UNBOUND
	}
	return protocolv1.ItemBinding_ITEM_BINDING_UNSPECIFIED
}

func entitlementTypeOf(t string) protocolv1.EntitlementType {
	switch t {
	case "ONE_SHOT":
		return protocolv1.EntitlementType_ENTITLEMENT_TYPE_ONE_SHOT
	case "ACCOUNT_SCOPED_ACCESS":
		return protocolv1.EntitlementType_ENTITLEMENT_TYPE_ACCOUNT_SCOPED_ACCESS
	case "DIRECT_ACCOUNT_COSMETIC":
		return protocolv1.EntitlementType_ENTITLEMENT_TYPE_DIRECT_ACCOUNT_COSMETIC
	}
	return protocolv1.EntitlementType_ENTITLEMENT_TYPE_UNSPECIFIED
}

func grantStateOf(g string) protocolv1.EntitlementGrantState {
	switch g {
	case "PENDING":
		return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_PENDING
	case "GRANTED":
		return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_GRANTED
	case "REJECTED":
		return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_REJECTED
	case "REFUNDED":
		return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_REFUNDED
	case "REFUNDED_CONSUMED":
		return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_REFUNDED_CONSUMED
	}
	return protocolv1.EntitlementGrantState_ENTITLEMENT_GRANT_STATE_UNSPECIFIED
}
