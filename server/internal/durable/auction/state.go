package auction

import (
	"errors"
	"time"
)

// ListingState is the auction_listings.state CHECK enum (data_model.md §
// auction_listings). SETTLING exists only inside the purchase transaction
// that holds the listing row lock and is never committed.
type ListingState string

const (
	StateActive       ListingState = "ACTIVE"
	StateSold         ListingState = "SOLD"
	StateCancelled    ListingState = "CANCELLED"
	StateExpired      ListingState = "EXPIRED"
	StateReclaimed    ListingState = "RECLAIMED"
	StateMovedToClaim ListingState = "MOVED_TO_CLAIM"
)

// ProceedsState is the auction_proceeds.state CHECK enum.
type ProceedsState string

const (
	ProceedsPending ProceedsState = "PENDING"
	ProceedsClaimed ProceedsState = "CLAIMED"
)

// Domain rejections, each mapping to one wire ErrorCode.
var (
	ErrNotActive          = errors.New("auction: listing not active")
	ErrNotReclaimable     = errors.New("auction: listing not cancelled/expired")
	ErrNotSeller          = errors.New("auction: caller is not the seller")
	ErrSameAccount        = errors.New("auction: same-account purchase forbidden")
	ErrBelowFloor         = errors.New("auction: price below listing floor")
	ErrOutOfRange         = errors.New("auction: price out of bounds")
	ErrCapacityFull       = errors.New("auction: 20 active listing cap")
	ErrInventoryFull      = errors.New("auction: buyer/seller inventory full")
	ErrInsufficient       = errors.New("auction: insufficient currency")
	ErrCapExceeded        = errors.New("auction: currency cap exceeded")
	ErrLevelGate          = errors.New("auction: character level < 15")
	ErrAgeGate            = errors.New("auction: character age < 24h")
	ErrProceedsNotPending = errors.New("auction: proceeds not pending")
	ErrStateConflict      = errors.New("auction: expected price mismatch")
	ErrItemLocked         = errors.New("auction: item locked")
	ErrNotEscrowed        = errors.New("auction: asset not in escrow")
	ErrUntradable         = errors.New("auction: item not tradable")
)

const (
	// LevelGate is the level-15 service unlock every auction operation
	// requires (trading_auction.md § Auction Eligibility Gates).
	LevelGate = 15
	// AgeGateHours is the listing-only character-age gate.
	AgeGateHours = 24
	// MaxActiveListings is the per-character ACTIVE cap.
	MaxActiveListings = 20
	// MaxPriceCommon is the absolute listing-price ceiling.
	MaxPriceCommon int64 = 2_000_000_000
	// ListingDuration is the fixed 24 h listing lifetime.
	ListingDuration = 24 * time.Hour
	// EscrowClaimDelay is the 7-day window after which an unreclaimed
	// cancelled/expired asset moves into a Reward Claim (escrow is never
	// long-term storage).
	EscrowClaimDelay = 7 * 24 * time.Hour
	// maxPageSize bounds keyset search pages (messages.md 738).
	maxPageSize = 50
)

// equipmentTierFloors is the authored per-piece equipment floor schedule
// (trading_auction.md § Listing Price Floor — same schedule as direct
// trade).
var equipmentTierFloors = map[int]int64{1: 500, 2: 1500, 3: 4000, 4: 10000, 5: 25000, 6: 50000}

// unitFloor is max(100, npc_base_buy_price); the listing floor is
// unit_floor × quantity, then the maximum of all applicable rules
// (equipment tier floor per piece).
func unitFloor(npcBaseBuyPrice int64) int64 {
	if npcBaseBuyPrice > 100 {
		return npcBaseBuyPrice
	}
	return 100
}

func listingFloor(item *LookupItem, quantity int64) int64 {
	floor := unitFloor(item.NPCBaseBuyPrice) * quantity
	if item.IsEquipment() {
		if tf, ok := equipmentTierFloors[item.Tier]; ok && tf > floor {
			floor = tf
		}
	}
	return floor
}

// listingFee is max(10, floor(price*1%)), charged ON_LIST and never
// refunded.
func listingFee(price int64) int64 {
	if fee := price / 100; fee > 10 {
		return fee
	}
	return 10
}

// hcm is the Asia/Ho_Chi_Minh zone of the Morning Market window.
var hcm = func() *time.Location {
	l, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	}
	return l
}()

// taxRate returns the sale-tax fraction for a purchase committed at
// commit: 3% inside 06:00-08:00 Asia/Ho_Chi_Minh, else 5%. The UTC date
// may cross at midnight; the authoritative instant converts to HCM first
// (trading_auction.md § Morning Market).
func taxRate(commit time.Time) (num, den int64) {
	h := commit.In(hcm).Hour()
	if h >= 6 && h < 8 {
		return 3, 100
	}
	return 5, 100
}

// tax computes floor(price*rate) — exact integer arithmetic.
func tax(price int64, commit time.Time) int64 {
	n, d := taxRate(commit)
	return price * n / d
}
