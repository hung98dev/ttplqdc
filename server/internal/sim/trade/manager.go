package trade

import (
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

var (
	// ErrNoSession covers requests addressing a trade the instance does
	// not track (unknown id, wrong participant, already closed).
	ErrNoSession = errors.New("trade: no session")
	// ErrBusy covers one-active-session-per-character conflicts.
	ErrBusy = errors.New("trade: participant already in a session")
	// ErrCommitted blocks every mutation once COMMITTING started —
	// cancel is only valid precommit (trading_auction.md § Direct Trade).
	ErrCommitted = errors.New("trade: session already committing")
)

const (
	defaultInviteTTL = 60 * time.Second
	defaultIdleTTL   = 120 * time.Second
)

// Emit is one outbound S2C the manager wants delivered; the edge layer
// turns each into a wire frame for the addressed character.
type Emit struct {
	To  id.UUID
	Msg proto.Message
}

// StackReader resolves an offered instance to (owner, item_id, stack
// quantity); the session only offers what the side currently owns.
type StackReader func(instanceID id.UUID) (owner id.UUID, itemID string, qty int, ok bool)

// SnapshotReader builds the committed JournalItem for one offer — the
// composition root resolves catalog-backed fields; tests return stubs.
type SnapshotReader func(charID id.UUID, o Offer) *journalv1.JournalItem

// TierReader resolves an item's authored equipment tier (0 = not
// equipment) for the price-floor check.
type TierReader func(itemID string) int

// Manager owns every direct-trade session on its map instance. It is
// single-writer: the owning simulation loop calls it from tick context.
type Manager struct {
	sessions  map[id.UUID]*Session
	pending   map[id.UUID]*Session
	byChar    map[id.UUID]id.UUID
	pres      Presence
	stacks    StackReader
	snap      SnapshotReader
	tier      TierReader
	now       func() time.Time
	inviteTTL time.Duration
	idleTTL   time.Duration
	newID     func() id.UUID
}

// Deps are the seams composition injects.
type Deps struct {
	Presence  Presence
	Stacks    StackReader
	Snapshots SnapshotReader
	Tier      TierReader
}

// New builds a manager; nil seams admit everything (tests).
func New(d Deps) *Manager {
	if d.Presence == nil {
		d.Presence = PresenceFunc(func(id.UUID) (ParticipantView, bool) {
			return ParticipantView{Level: 60, Age: MinAge, MapID: "map.test", Online: true}, true
		})
	}
	if d.Stacks == nil {
		d.Stacks = func(id.UUID) (id.UUID, string, int, bool) { return id.UUID{}, "", 0, false }
	}
	if d.Tier == nil {
		d.Tier = func(string) int { return 0 }
	}
	return &Manager{
		sessions:  map[id.UUID]*Session{},
		pending:   map[id.UUID]*Session{},
		byChar:    map[id.UUID]id.UUID{},
		pres:      d.Presence,
		stacks:    d.Stacks,
		snap:      d.Snapshots,
		tier:      d.Tier,
		now:       time.Now,
		inviteTTL: defaultInviteTTL,
		idleTTL:   defaultIdleTTL,
		newID:     id.NewV4,
	}
}

// WithClock overrides the clock and timeouts (tests).
func (m *Manager) WithClock(now func() time.Time) *Manager {
	c := *m
	c.now = now
	return &c
}

// WithTTLs overrides invite/idle deadlines (tests).
func (m *Manager) WithTTLs(invite, idle time.Duration) *Manager {
	c := *m
	c.inviteTTL, c.idleTTL = invite, idle
	return &c
}

func (m *Manager) ms() int64 { return m.now().UnixMilli() }

func opResult(opID id.UUID, code protocolv1.ErrorCode) *protocolv1.OperationResult {
	st := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		st = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	return &protocolv1.OperationResult{OperationId: opID[:], Status: st, ErrorCode: code}
}

// requestResult builds the single S2C_TRADE_REQUEST_RESULT (710) every
// 700/702/703/705/707 request resolves to.
func requestResult(req uint32, opID id.UUID, tradeID id.UUID, code protocolv1.ErrorCode) *protocolv1.S2CTradeRequestResult {
	return &protocolv1.S2CTradeRequestResult{
		Result:           opResult(opID, code),
		RequestMessageId: req,
		TradeId:          tradeID[:],
	}
}

// offerState builds the 706 broadcast both sides observe.
func (m *Manager) offerState(s *Session) *protocolv1.S2CTradeOfferState {
	return &protocolv1.S2CTradeOfferState{
		TradeId:    s.TradeID[:],
		Revision:   s.Revision,
		State:      stateProto(s.State),
		Sides:      []*protocolv1.TradeSide{sideProto(s.Initiator), sideProto(s.Counterpart)},
		FeePreview: totalFee(s),
	}
}

func stateProto(s State) protocolv1.TradeOfferState {
	switch s {
	case StateOpen:
		return protocolv1.TradeOfferState_TRADE_OFFER_STATE_OPEN
	case StateLocked:
		return protocolv1.TradeOfferState_TRADE_OFFER_STATE_LOCKED
	case StateCommitting:
		return protocolv1.TradeOfferState_TRADE_OFFER_STATE_COMMITTING
	}
	return protocolv1.TradeOfferState_TRADE_OFFER_STATE_UNSPECIFIED
}

func sideProto(s *Side) *protocolv1.TradeSide {
	out := &protocolv1.TradeSide{
		CharacterId:  s.CharacterID[:],
		CommonAmount: s.CommonAmount,
		Confirmed:    s.Confirmed,
	}
	for _, o := range s.Offers {
		out.Items = append(out.Items, &protocolv1.ItemInstanceView{
			ItemInstanceId: o.ItemInstanceID[:],
			ItemId:         o.ItemID,
			Quantity:       uint32(o.Quantity),
		})
	}
	return out
}

// cancelled builds the 704 broadcast.
func cancelled(s *Session, reason protocolv1.TradeCancelReason, code protocolv1.ErrorCode) *protocolv1.S2CTradeCancelled {
	return &protocolv1.S2CTradeCancelled{TradeId: s.TradeID[:], Reason: reason, ErrorCode: code}
}

func (m *Manager) both(s *Session, msg proto.Message) []Emit {
	return []Emit{
		{To: s.Initiator.CharacterID, Msg: msg},
		{To: s.Counterpart.CharacterID, Msg: msg},
	}
}

// inviteLookup resolves a participant or fails the invite with the
// mapped code.
func (m *Manager) inviteLookup(charID id.UUID) (ParticipantView, protocolv1.ErrorCode, bool) {
	v, ok := m.pres.Participant(charID)
	if !ok || !v.Online {
		return ParticipantView{}, protocolv1.ErrorCode_ERROR_CODE_TRADE_PARTNER_DISCONNECTED, false
	}
	return v, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, true
}

// Invite admits a 700 invitation.
func (m *Manager) Invite(opID, inviter, target id.UUID) (*protocolv1.S2CTradeRequestResult, []Emit) {
	if inviter == target {
		return requestResult(700, opID, id.UUID{}, protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN), nil
	}
	if _, busy := m.byChar[inviter]; busy {
		return requestResult(700, opID, id.UUID{}, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if _, busy := m.byChar[target]; busy {
		return requestResult(700, opID, id.UUID{}, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	a, code, ok := m.inviteLookup(inviter)
	if !ok {
		return requestResult(700, opID, id.UUID{}, code), nil
	}
	b, code, ok := m.inviteLookup(target)
	if !ok {
		return requestResult(700, opID, id.UUID{}, code), nil
	}
	for _, v := range []ParticipantView{a, b} {
		if code, pass := CheckEligibility(v); !pass {
			return requestResult(700, opID, id.UUID{}, code), nil
		}
	}
	if a.AccountID == b.AccountID {
		return requestResult(700, opID, id.UUID{}, protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN), nil
	}
	if err := m.pres.CanDirectInteract(inviter, target); err != nil {
		return requestResult(700, opID, id.UUID{}, protocolv1.ErrorCode_ERROR_CODE_TARGET_BLOCKED), nil
	}
	if code, pass := CheckProximity(a, b); !pass {
		return requestResult(700, opID, id.UUID{}, code), nil
	}
	tradeID := m.newID()
	s := &Session{
		TradeID:      tradeID,
		Initiator:    &Side{CharacterID: inviter, Ledger: items.NewTradeLockLedger(inviter)},
		Counterpart:  &Side{CharacterID: target, Ledger: items.NewTradeLockLedger(target)},
		LastActivity: m.ms() - m.inviteTTL.Milliseconds(), // pending deadline, see Tick
	}
	m.pending[tradeID] = s
	m.byChar[inviter], m.byChar[target] = tradeID, tradeID
	emits := []Emit{{To: target, Msg: &protocolv1.S2CTradeInvite{
		TradeId:            tradeID[:],
		InviterCharacterId: inviter[:],
		ExpiresInSeconds:   uint32(m.inviteTTL / time.Second),
	}}}
	return requestResult(700, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// Accept admits a 702 acceptance — the session goes OPEN and both
// ledgers start tracking locks.
func (m *Manager) Accept(opID, accepter, tradeID id.UUID) (*protocolv1.S2CTradeRequestResult, []Emit) {
	s := m.pending[tradeID]
	if s == nil || s.Counterpart.CharacterID != accepter {
		return requestResult(702, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	delete(m.pending, tradeID)
	s.State = StateOpen
	s.LastActivity = m.ms()
	m.sessions[tradeID] = s
	emits := m.both(s, m.offerState(s))
	return requestResult(702, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// Cancel resolves a 703 cancel/decline.
func (m *Manager) Cancel(opID, charID, tradeID id.UUID) (*protocolv1.S2CTradeRequestResult, []Emit) {
	if s := m.pending[tradeID]; s != nil && s.has(charID) {
		reason := protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_DECLINED
		if s.Initiator.CharacterID == charID {
			reason = protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_CANCELLED
		}
		m.dropPending(s)
		emits := m.both(s, cancelled(s, reason, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED))
		return requestResult(703, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
	}
	s := m.sessions[tradeID]
	if s == nil || !s.has(charID) {
		return requestResult(703, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if s.State == StateCommitting {
		return requestResult(703, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	m.drop(s)
	emits := m.both(s, cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_CANCELLED, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED))
	return requestResult(703, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// OfferUpdate applies a 705 full-replacement offer for one side.
func (m *Manager) OfferUpdate(opID, charID, tradeID id.UUID, expectedRevision uint64,
	offers []*protocolv1.TradeOfferItem, common int64) (*protocolv1.S2CTradeRequestResult, []Emit) {
	s := m.sessions[tradeID]
	if s == nil || !s.has(charID) {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if s.State == StateCommitting {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if expectedRevision != s.Revision {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if len(offers) > MaxOffersPerSide {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	side := s.side(charID)
	if common < 0 {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if common > 0 && s.other(charID).CommonAmount > 0 {
		return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_TRADE_COMMON_BOTH_SIDES), nil
	}
	ledger := items.NewTradeLockLedger(charID)
	seen := map[id.UUID]bool{}
	built := make([]Offer, 0, len(offers))
	for _, o := range offers {
		var iid id.UUID
		copy(iid[:], o.GetItemInstanceId())
		if seen[iid] {
			return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
		}
		seen[iid] = true
		owner, itemID, qty, ok := m.stacks(iid)
		if !ok || owner != charID {
			return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
		}
		q := int(o.GetQuantity())
		if q < 1 || q > qty {
			return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_ITEM), nil
		}
		if err := ledger.Offer(iid, q, qty); err != nil {
			return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED), nil
		}
		built = append(built, Offer{ItemInstanceID: iid, ItemID: itemID, Quantity: q})
	}
	side.Ledger.Cancel()
	side.Ledger = ledger
	side.Offers = built
	side.CommonAmount = common
	s.Initiator.Confirmed, s.Counterpart.Confirmed = false, false
	s.State = StateOpen
	s.Revision++
	s.LastActivity = m.ms()
	emits := m.both(s, m.offerState(s))
	return requestResult(705, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// Confirm applies a 707 side confirmation; both confirmed → LOCKED.
func (m *Manager) Confirm(opID, charID, tradeID id.UUID, expectedRevision uint64) (*protocolv1.S2CTradeRequestResult, []Emit) {
	s := m.sessions[tradeID]
	if s == nil || !s.has(charID) {
		return requestResult(707, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if s.State == StateCommitting || expectedRevision != s.Revision {
		return requestResult(707, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	s.side(charID).Confirmed = true
	if s.Initiator.Confirmed && s.Counterpart.Confirmed {
		s.State = StateLocked
	}
	s.LastActivity = m.ms()
	emits := m.both(s, m.offerState(s))
	return requestResult(707, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// Finalise admits the 708 commit: the session must be LOCKED, every
// admission gate is re-validated (COMMITTING revalidation), and the
// returned JournalTrade is embedded in the durable CLIENT record by
// the caller — the session is consumed, never replayed (save_rules §
// no requestless TRADE replay).
func (m *Manager) Finalise(opID, charID, tradeID id.UUID, expectedRevision uint64) (*journalv1.JournalTrade, *protocolv1.S2CTradeRequestResult, []Emit) {
	s := m.sessions[tradeID]
	if s == nil || !s.has(charID) {
		return nil, requestResult(708, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	if s.State != StateLocked || expectedRevision != s.Revision {
		return nil, requestResult(708, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT), nil
	}
	// COMMITTING revalidation: eligibility and floors are checked again
	// under the same gates the invite applied (trading_auction.md §
	// re-validate at COMMITTING).
	if code := m.revalidate(s); code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		m.drop(s)
		emits := m.both(s, cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_VALIDATION_FAILED, code))
		return nil, requestResult(708, opID, tradeID, code), emits
	}
	s.State = StateCommitting
	s.CommitActor = charID
	s.LastActivity = m.ms()
	settlementID := m.newID()
	j := BuildJournal(s, settlementID, opID, m.ms(), m.snap)
	emits := m.both(s, m.offerState(s))
	return j, requestResult(708, opID, tradeID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED), emits
}

// revalidate runs the COMMITTING gates; the durable executor repeats
// them authoritatively inside the settlement transaction.
func (m *Manager) revalidate(s *Session) protocolv1.ErrorCode {
	for _, side := range []*Side{s.Initiator, s.Counterpart} {
		v, code, ok := m.inviteLookup(side.CharacterID)
		if !ok {
			return code
		}
		if code, pass := CheckEligibility(v); !pass {
			return code
		}
	}
	if s.Initiator.CommonAmount > 0 && s.Counterpart.CommonAmount > 0 {
		return protocolv1.ErrorCode_ERROR_CODE_TRADE_COMMON_BOTH_SIDES
	}
	if EquipmentFloorSum(s.Initiator.Offers, m.tier) > s.Counterpart.CommonAmount ||
		EquipmentFloorSum(s.Counterpart.Offers, m.tier) > s.Initiator.CommonAmount {
		return protocolv1.ErrorCode_ERROR_CODE_TRADE_PRICE_FLOOR_NOT_MET
	}
	return protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
}

// Settled completes a COMMITTING session after the durable outcome
// landed: the partner's 709 (empty operation_id — partner-triggered
// per messages.md § 709) goes out and the session is dropped. The
// caller relays the committed result status/code; the partner view is
// derived from the frozen session (they received what the actor
// offered).
func (m *Manager) Settled(tradeID, settlementID id.UUID, status protocolv1.ResultStatus,
	code protocolv1.ErrorCode) []Emit {
	s := m.sessions[tradeID]
	if s == nil {
		return nil
	}
	defer m.drop(s)
	partner := s.other(s.CommitActor)
	if partner == nil {
		partner = s.Counterpart
	}
	actor := s.other(partner.CharacterID)
	var received []*protocolv1.ItemGrant
	var commonReceived, fee int64
	if status == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		for _, o := range actor.Offers {
			received = append(received, &protocolv1.ItemGrant{
				ItemInstanceId: o.ItemInstanceID[:], ItemId: o.ItemID, Quantity: uint32(o.Quantity),
			})
		}
		fee = Fee(actor.CommonAmount)
		commonReceived = actor.CommonAmount - fee
	}
	msg := &protocolv1.S2CTradeResult{
		Result:         &protocolv1.OperationResult{Status: status, ErrorCode: code},
		TradeId:        s.TradeID[:],
		SettlementId:   settlementID[:],
		Received:       received,
		CommonReceived: commonReceived,
		Fee:            fee,
	}
	return []Emit{{To: partner.CharacterID, Msg: msg}}
}

// drop removes a live session and releases every lock.
func (m *Manager) drop(s *Session) {
	delete(m.sessions, s.TradeID)
	delete(m.byChar, s.Initiator.CharacterID)
	delete(m.byChar, s.Counterpart.CharacterID)
	s.release()
}

// dropPending removes an unaccepted invite (no locks held yet).
func (m *Manager) dropPending(s *Session) {
	delete(m.pending, s.TradeID)
	delete(m.byChar, s.Initiator.CharacterID)
	delete(m.byChar, s.Counterpart.CharacterID)
}

// Tick sweeps deadlines: pending invites expire (EXPIRED); idle open
// sessions time out (INACTIVE_TIMEOUT, 120s per spec).
func (m *Manager) Tick() []Emit {
	now := m.ms()
	var out []Emit
	for _, s := range m.pending {
		if now-s.LastActivity >= m.inviteTTL.Milliseconds() {
			m.dropPending(s)
			out = append(out, m.both(s, cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_EXPIRED, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED))...)
		}
	}
	for _, s := range m.sessions {
		if s.State != StateCommitting && now-s.LastActivity >= m.idleTTL.Milliseconds() {
			m.drop(s)
			out = append(out, m.both(s, cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_INACTIVE_TIMEOUT, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED))...)
		}
	}
	return out
}

// Disconnected cancels every session the character participates in
// (PARTNER_DISCONNECTED on the partner's side — the disconnecting
// actor gets nothing back).
func (m *Manager) Disconnected(charID id.UUID) []Emit {
	var out []Emit
	for _, s := range append(live(m.sessions), live(m.pending)...) {
		if !s.has(charID) {
			continue
		}
		m.dropSessionAny(s)
		out = append(out, Emit{
			To:  s.other(charID).CharacterID,
			Msg: cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_PARTNER_DISCONNECTED, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
		})
	}
	return out
}

// CancelOnParticipantEvent applies the release-only cancel paths:
// map transfer, respawn, instance change or death of a participant.
func (m *Manager) CancelOnParticipantEvent(charID id.UUID) []Emit {
	var out []Emit
	for _, s := range append(live(m.sessions), live(m.pending)...) {
		if !s.has(charID) {
			continue
		}
		m.dropSessionAny(s)
		out = append(out, m.both(s, cancelled(s, protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_CANCELLED, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED))...)
	}
	return out
}

// Restart drops every session — the process restart path (ADR-0060):
// sessions are runtime-only, cancellation is value-free (nothing has
// moved; every lock releases).
func (m *Manager) Restart() {
	for _, s := range m.sessions {
		m.drop(s)
	}
	for _, s := range m.pending {
		m.dropPending(s)
	}
	m.sessions, m.pending = map[id.UUID]*Session{}, map[id.UUID]*Session{}
}

// Ledger resolves the participant's runtime lock ledger — the seam
// Deps.Locks and the 433 locked_quantity view consume.
func (m *Manager) Ledger(charID id.UUID) *items.TradeLockLedger {
	if tid, ok := m.byChar[charID]; ok {
		if s := m.sessions[tid]; s != nil {
			return s.side(charID).Ledger
		}
		if s := m.pending[tid]; s != nil {
			return s.side(charID).Ledger
		}
	}
	return nil
}

// LockedQuantity reports the session-locked units of one stack.
func (m *Manager) LockedQuantity(charID, instanceID id.UUID) int {
	if l := m.Ledger(charID); l != nil {
		return l.LockedQty(instanceID)
	}
	return 0
}

// SessionFor resolves a participant's live session (tests).
func (m *Manager) SessionFor(charID id.UUID) *Session {
	if tid, ok := m.byChar[charID]; ok {
		if s := m.sessions[tid]; s != nil {
			return s
		}
		return m.pending[tid]
	}
	return nil
}

// Len reports live+pending session count (tests).
func (m *Manager) Len() int { return len(m.sessions) + len(m.pending) }

// live snapshots map values (iteration during drop-safe range).
func live(set map[id.UUID]*Session) []*Session {
	out := make([]*Session, 0, len(set))
	for _, s := range set {
		out = append(out, s)
	}
	return out
}

func (m *Manager) dropSessionAny(s *Session) {
	if _, ok := m.pending[s.TradeID]; ok {
		m.dropPending(s)
		return
	}
	m.drop(s)
}
