package reward

import (
	"errors"
	"fmt"
	"math/big"

	"thinhthan/internal/core/id"
)

// Durable family name (save_rules.md producer registry; router's
// durableFamiliesExact binds wire id 408 to it).
const ClaimFamily = "reward.claim"

// Closed enums (reward_claims.md § Claim Creation + baseline schema).
const (
	ClaimKindSingle            = "SINGLE"
	ClaimKindItemConsolidated  = "ITEM_CONSOLIDATED"
	ClaimKindCurrencyAggregate = "CURRENCY_AGGREGATE"
	StatePending               = "PENDING"
	StateClaiming              = "CLAIMING"
	StateClaimed               = "CLAIMED"
	StateExpired               = "EXPIRED"
	lineItem                   = "ITEM"
	lineCurrency               = "CURRENCY"
	softCap                    = 100
	hardCeiling                = 500
	pageLimit                  = 50
	statePushCap               = 100
	restDailyCap               = 180
)

// Sentinel errors; wire codes map at the executor boundary only.
var (
	ErrUnknownSourceType  = errors.New("reward: unknown source_type")
	ErrClaimCapReached    = errors.New("reward: claim cap reached")
	ErrNotOwner           = errors.New("reward: claim owned by another character")
	ErrClaimNotFound      = errors.New("reward: claim not found")
	ErrStateConflict      = errors.New("reward: claim state conflict")
	ErrClaimExpired       = errors.New("reward: claim expired")
	ErrInventoryFull      = errors.New("reward: inventory capacity exhausted")
	ErrCurrencyCapReached = errors.New("reward: currency cap reached")
	ErrMalformed          = errors.New("reward: malformed input")
)

// SourceTypes is the canonical CHECK set (reward_claims.md, ADR-0060).
var SourceTypes = map[string]struct{}{
	"MONSTER": {}, "BOSS": {}, "BOSS_CHEST": {}, "DUNGEON": {},
	"QUEST": {}, "WORLD_EVENT": {}, "ATLAS": {}, "FEAT": {},
	"LEVEL_MILESTONE": {}, "PVP": {}, "GUILD_WAR": {}, "GUILD": {},
	"FISHING": {}, "HIDDEN_CHEST": {}, "AUCTION_ESCROW_EXPIRY": {},
	"ADMIN_COMPENSATION": {},
}

// dec38 is the exact NUMERIC(38,0) representation used for claim
// totals and delivered counters (reward_claims.md § Delivery).
var dec38Limit, _ = new(big.Int).SetString("99999999999999999999999999999999999999", 10)

// dec parses a NUMERIC(38,0) text projection into big.Int.
func dec(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return new(big.Int)
	}
	return n
}

// decString renders an exact decimal for a NUMERIC parameter.
func decString(n *big.Int) string { return n.String() }

// LineInput is one contributed reward line: an ITEM payload carries the
// complete immutable item-creation state (ADR-0012); a CURRENCY line
// carries the earned amount.
type LineInput struct {
	Kind             string // lineItem | lineCurrency
	ItemID           string
	Quantity         *big.Int
	EffectiveBinding string
	// ItemState carries the finalized per-instance payload (enhancement,
	// roll, provenance) as authored JSONB — the typed-table schema keeps
	// per-instance state out of scalar columns (ADR-0065).
	ItemState       []byte
	ContentRevision string
	CurrencyID      string
	Amount          *big.Int
	// PerInstance marks payloads whose per-instance state (equipment
	// rolls, soul state) forbids consolidation (ADR-0063).
	PerInstance bool
}

// Input is one claim contribution: the idempotent settled reward for
// one source slot.
type Input struct {
	OwnerCharacterID        id.UUID
	SourceType              string
	SourceReference         string
	RewardSlot              string
	SourceRewardOperationID id.UUID
	Lines                   []LineInput
	// Preventable marks player-initiated sources gated at action start
	// (ADR-0062 soft cap).
	Preventable bool
	// CreatedAt stamps the row deterministically; zero => store clock.
	CreatedAtUnixMs int64
}

// Created reports the committed (or replayed) claim.
type Created struct {
	ClaimID  id.UUID
	Kind     string
	Inserted bool // false => contribution key deduped, nothing mutated
	// Skipped reports a hard-ceiling item/equipment roll skip
	// (ADR-0062): the claim is intentionally absent.
	Skipped bool
}

// validate checks enum membership, line typing and the exact-decimal
// bounds before any row is read (reward_claims.md, ADR-0060).
func (in *Input) validate() error {
	if in.OwnerCharacterID == (id.UUID{}) || in.SourceRewardOperationID == (id.UUID{}) {
		return fmt.Errorf("%w: identity", ErrMalformed)
	}
	if _, ok := SourceTypes[in.SourceType]; !ok {
		return fmt.Errorf("%w: %q", ErrUnknownSourceType, in.SourceType)
	}
	if len(in.SourceType) > 32 || len(in.SourceReference) > 160 || len(in.RewardSlot) == 0 || len(in.RewardSlot) > 64 {
		return fmt.Errorf("%w: field bounds", ErrMalformed)
	}
	if len(in.Lines) == 0 {
		return fmt.Errorf("%w: empty claim", ErrMalformed)
	}
	for i := range in.Lines {
		l := &in.Lines[i]
		switch l.Kind {
		case lineItem:
			if len(l.ItemID) == 0 || len(l.ItemID) > 64 || len(l.EffectiveBinding) == 0 || len(l.EffectiveBinding) > 24 ||
				l.Quantity == nil || l.Quantity.Sign() <= 0 || l.Quantity.Cmp(dec38Limit) > 0 ||
				len(l.ContentRevision) != 64 || len(l.CurrencyID) != 0 || l.Amount != nil {
				return fmt.Errorf("%w: item line %d", ErrMalformed, i)
			}
		case lineCurrency:
			if len(l.CurrencyID) == 0 || len(l.CurrencyID) > 32 || l.Amount == nil || l.Amount.Sign() <= 0 ||
				l.Amount.Cmp(dec38Limit) > 0 || len(l.ItemID) != 0 || l.Quantity != nil {
				return fmt.Errorf("%w: currency line %d", ErrMalformed, i)
			}
		default:
			return fmt.Errorf("%w: line kind %q", ErrMalformed, l.Kind)
		}
	}
	return nil
}
