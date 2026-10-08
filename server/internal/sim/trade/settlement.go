package trade

import (
	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

const (
	// MaxOffersPerSide is the entry bound each side may offer.
	MaxOffersPerSide = 12
	// FeePermille is the DIRECT_TRADE_FEE rate on offered common:
	// fee = floor(offered_common * 0.05) (trading_auction.md § Direct
	// Trade Fees), charged to the offering side; the receiver gets the
	// rest and the fee is routed to the sink.
	FeePercentPermille = 50
	// SinkReason is the audit reason code for the burned fee fraction.
	SinkReason = "DIRECT_TRADE_FEE"
)

// equipmentTierFloors is the authored per-piece equipment floor schedule
// shared with listing floors (trading_auction.md § Direct Trade — an
// offered equipment piece may only be received against at least its
// tier floor in common).
var equipmentTierFloors = map[int]int64{1: 500, 2: 1500, 3: 4000, 4: 10000, 5: 25000, 6: 50000}

// Fee computes floor(offered * 0.05).
func Fee(offered int64) int64 {
	if offered <= 0 {
		return 0
	}
	return offered * FeePercentPermille / 1000
}

// EquipmentFloor returns the per-piece common floor for one offered
// equipment tier (0 when the item is not equipment).
func EquipmentFloor(tier int) int64 {
	return equipmentTierFloors[tier]
}

// EquipmentFloorSum sums the tier floors over offered items; tiers are
// resolved through the injected seam.
func EquipmentFloorSum(offers []Offer, tierOf func(itemID string) int) int64 {
	var sum int64
	for _, o := range offers {
		sum += equipmentTierFloors[tierOf(o.ItemID)]
	}
	return sum
}

// BuildJournal assembles the two-sided JournalTrade a LOCKED session
// embeds in the CLIENT708 record (save_rules § Closed Durable Queue
// Producer Registry — the journal is the committed offer snapshot the
// settlement executor replays).
func BuildJournal(s *Session, settlementID, opID id.UUID, finalizedAtMs int64,
	snapshot func(charID id.UUID, o Offer) *journalv1.JournalItem) *journalv1.JournalTrade {
	return &journalv1.JournalTrade{
		TradeId:                     s.TradeID[:],
		ExpectedRevision:            s.Revision,
		Initiator:                   journalSide(s.Initiator, snapshot),
		Counterpart:                 journalSide(s.Counterpart, snapshot),
		SettlementId:                settlementID[:],
		FeeCommon:                   totalFee(s),
		FinalizedAtMs:               finalizedAtMs,
		InitiatingClientOperationId: opID[:],
	}
}

func journalSide(side *Side, snapshot func(charID id.UUID, o Offer) *journalv1.JournalItem) *journalv1.JournalTradeSide {
	out := &journalv1.JournalTradeSide{
		CharacterId:  side.CharacterID[:],
		AccountId:    side.AccountID[:],
		CommonAmount: side.CommonAmount,
	}
	for _, o := range side.Offers {
		out.Items = append(out.Items, snapshot(side.CharacterID, o))
	}
	return out
}

// totalFee sums the per-side fee the offering side pays.
func totalFee(s *Session) int64 {
	return Fee(s.Initiator.CommonAmount) + Fee(s.Counterpart.CommonAmount)
}
