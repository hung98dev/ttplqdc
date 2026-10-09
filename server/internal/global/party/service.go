package party

import (
	"time"

	"thinhthan/internal/core/id"
	v1 "thinhthan/internal/protocol/v1"
)

// Options injects clocks and cross-boundary queries. Online reports a
// character's live session presence (edge-owned); Blocked reports the
// social direct-interaction gate (IMP-034 domain); InSafeAnchor reports
// safe-anchor membership (world-owned). Fail-closed defaults: nothing
// is online, nothing is blocked, nothing is anchored.
type Options struct {
	Now          func() time.Time
	Online       func(charID id.UUID) bool
	Blocked      func(from, to id.UUID) bool
	InSafeAnchor func(charID id.UUID) bool
	// MemberProfile resolves the roster fields the board/state rows
	// carry (display name, class, level, zone). Missing -> zero values.
	MemberProfile func(charID id.UUID) (name, classID, zoneID string, level uint32, ok bool)
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Service owns party state. All methods must run on the single writer
// goroutine (global runtime) or be driven single-threaded by tests —
// the package takes no locks (service_boundaries.md).
type Service struct {
	opt Options

	parties map[id.UUID]*Party
	byChar  map[id.UUID]id.UUID // character -> party
	invites map[id.UUID]*Invite // pending invite keyed by target
	board   map[id.UUID]*BoardPost
	byPost  map[id.UUID]id.UUID // poster -> post
	repost  map[id.UUID]time.Time
	rate    map[id.UUID][]time.Time

	// Outbox accumulates fanout intents (603 invite delivery, 607 state
	// pushes, 636 board pushes) for the wiring layer; tests assert it.
	Outbox []Event
}

// EventKind tags outbound fanout kinds.
type EventKind int32

const (
	EventPartyInvite EventKind = iota + 1 // S2C_PARTY_INVITE (603)
	EventPartyState                       // S2C_PARTY_STATE (607)
	EventPartyBoard                       // S2C_PARTY_BOARD_STATE (636)
)

// Event is one fanout intent. Targets are characters to address.
type Event struct {
	Kind    EventKind
	Targets []id.UUID
	Invite  *v1.S2CPartyInvite
	State   *v1.S2CPartyState
	Board   *v1.S2CPartyBoardState
}

// New builds the empty service.
func New(opt Options) *Service {
	return &Service{
		opt:     opt,
		parties: map[id.UUID]*Party{},
		byChar:  map[id.UUID]id.UUID{},
		invites: map[id.UUID]*Invite{},
		board:   map[id.UUID]*BoardPost{},
		byPost:  map[id.UUID]id.UUID{},
		repost:  map[id.UUID]time.Time{},
		rate:    map[id.UUID][]time.Time{},
	}
}

func (s *Service) online(char id.UUID) bool {
	if s.opt.Online != nil {
		return s.opt.Online(char)
	}
	return false
}

func (s *Service) blocked(from, to id.UUID) bool {
	return s.opt.Blocked != nil && s.opt.Blocked(from, to)
}

func (s *Service) anchored(char id.UUID) bool {
	return s.opt.InSafeAnchor != nil && s.opt.InSafeAnchor(char)
}

func (s *Service) member(char id.UUID) (*Party, *Member) {
	pid, ok := s.byChar[char]
	if !ok {
		return nil, nil
	}
	p := s.parties[pid]
	if p == nil {
		return nil, nil
	}
	for _, m := range p.Members {
		if m.CharacterID == char {
			return p, m
		}
	}
	return p, nil
}

// Invite handles C2S_PARTY_INVITE (602). A partyless inviter atomically
// creates an ACTIVE party led by the inviter at join_sequence 1
// (ADR-0060/0062, party.md); a party member must be leader.
func (s *Service) Invite(opID, inviter, target id.UUID) Result {
	now := s.opt.now()
	res := Result{RequestMessageID: ReqInvite, OperationID: opID}

	p, inviterMember := s.member(inviter)
	if p != nil && (inviterMember == nil || p.Leader != inviter) {
		return res.fail(v1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	}
	if s.limited(inviter, now) {
		return res.fail(v1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}
	if s.blocked(inviter, target) {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_BLOCKED)
	}
	if _, memberOf := s.byChar[target]; memberOf {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if !s.online(target) {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if pend, ok := s.invites[target]; ok && pend.State == InvitePending && !pend.ExpiresAt.After(now) {
		pend.State = InviteExpired
		delete(s.invites, target)
	} else if ok && pend.State == InvitePending {
		return res.fail(v1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}

	if p == nil {
		p = s.createParty(inviter, now)
	}

	inv := &Invite{
		PartyID:   p.ID,
		Inviter:   inviter,
		Target:    target,
		ExpiresAt: now.Add(InviteTTL),
		State:     InvitePending,
	}
	s.invites[target] = inv
	s.bump(p)
	s.Outbox = append(s.Outbox, Event{
		Kind:    EventPartyInvite,
		Targets: []id.UUID{target},
		Invite: &v1.S2CPartyInvite{
			PartyId:            p.ID[:],
			InviterCharacterId: inviter[:],
			InviterName:        s.nameOf(inviter),
			ExpiresInSeconds:   uint32(InviteTTL / time.Second),
		},
	})
	s.emitState(p)
	return res.ok(p.ID)
}

// Accept handles C2S_PARTY_ACCEPT (604); capacity validated atomically.
func (s *Service) Accept(opID, char, partyID, inviter id.UUID) Result {
	now := s.opt.now()
	res := Result{RequestMessageID: ReqAccept, OperationID: opID}
	inv, ok := s.invites[char]
	if !ok || inv.State != InvitePending {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if !inv.ExpiresAt.After(now) {
		inv.State = InviteExpired
		delete(s.invites, char)
		return res.fail(v1.ErrorCode_ERROR_CODE_EXPIRED)
	}
	if inv.PartyID != partyID || inv.Inviter != inviter {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	p := s.parties[partyID]
	if p == nil || !p.Active {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if len(p.Members) >= MaxMembers {
		return res.fail(v1.ErrorCode_ERROR_CODE_CAPACITY_FULL)
	}
	inv.State = InviteAccepted
	delete(s.invites, char)
	s.addMember(p, char, now)
	s.emitState(p)
	return res.ok(p.ID)
}

// Decline handles C2S_PARTY_DECLINE (620); the solo party persists.
func (s *Service) Decline(opID, char, partyID, inviter id.UUID) Result {
	return s.endInvite(ReqDecline, opID, char, partyID, inviter, InviteDeclined)
}

// CancelInvite handles C2S_PARTY_INVITE_CANCEL (621).
func (s *Service) CancelInvite(opID, inviter, target id.UUID) Result {
	res := Result{RequestMessageID: ReqInviteCancel, OperationID: opID}
	inv, ok := s.invites[target]
	if !ok || inv.State != InvitePending || inv.Inviter != inviter {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	inv.State = InviteCancelled
	delete(s.invites, target)
	return res.ok(inv.PartyID)
}

func (s *Service) endInvite(req uint32, opID, char, partyID, inviter id.UUID, to InviteState) Result {
	now := s.opt.now()
	res := Result{RequestMessageID: req, OperationID: opID}
	inv, ok := s.invites[char]
	if !ok || inv.State != InvitePending || inv.PartyID != partyID || inv.Inviter != inviter {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if !inv.ExpiresAt.After(now) {
		inv.State = InviteExpired
		delete(s.invites, char)
		return res.fail(v1.ErrorCode_ERROR_CODE_EXPIRED)
	}
	inv.State = to
	delete(s.invites, char)
	return res.ok(inv.PartyID)
}

// Leave handles C2S_PARTY_LEAVE (605): lowest join_sequence inherits;
// the final leave disbands (party.md § Leave / Leadership).
func (s *Service) Leave(opID, char id.UUID) Result {
	res := Result{RequestMessageID: ReqLeave, OperationID: opID}
	p, m := s.member(char)
	if p == nil || m == nil {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	pid := p.ID
	s.removeMember(p, char)
	if len(p.Members) == 0 {
		p.Active = false
		s.bump(p)
		delete(s.parties, pid)
		s.emitState(p)
		return res.ok(pid)
	}
	if p.Leader == char {
		p.Leader = lowestSeq(p.Members).CharacterID
	}
	s.bump(p)
	s.emitState(p)
	return res.ok(pid)
}

// Kick handles C2S_PARTY_KICK (606); leader only.
func (s *Service) Kick(opID, leader, target id.UUID) Result {
	res := Result{RequestMessageID: ReqKick, OperationID: opID}
	p, m := s.member(leader)
	if p == nil || m == nil || p.Leader != leader {
		return res.fail(v1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	}
	tm := findMember(p, target)
	if tm == nil {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	pid := p.ID
	s.removeMember(p, target)
	s.bump(p)
	s.emitState(p)
	return res.ok(pid)
}

// TransferLeader handles C2S_PARTY_LEADER_TRANSFER (622).
func (s *Service) TransferLeader(opID, leader, target id.UUID) Result {
	res := Result{RequestMessageID: ReqLeaderTransfer, OperationID: opID}
	p, m := s.member(leader)
	if p == nil || m == nil || p.Leader != leader {
		return res.fail(v1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	}
	if findMember(p, target) == nil {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	p.Leader = target
	s.bump(p)
	s.emitState(p)
	return res.ok(p.ID)
}

// BoardPost handles C2S_PARTY_BOARD_POST (634): safe-anchor only,
// one live post per poster, 120 s TTL, 30 s repost cooldown.
func (s *Service) BoardPost(opID, poster id.UUID, dungeonID string, size uint32, note string) Result {
	now := s.opt.now()
	res := Result{RequestMessageID: ReqBoardPost, OperationID: opID}
	if !s.anchored(poster) {
		return res.fail(v1.ErrorCode_ERROR_CODE_NOT_IN_SAFE_ANCHOR)
	}
	if size < 2 || size > MaxMembers || dungeonID == "" || len(note) > BoardNoteMax {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if until, ok := s.repost[poster]; ok && now.Before(until) {
		return res.fail(v1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE)
	}
	if _, ok := s.byPost[poster]; ok {
		return res.fail(v1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}
	post := &BoardPost{
		PostID:    id.NewV4(),
		Poster:    poster,
		DungeonID: dungeonID,
		Size:      size,
		Note:      note,
		ExpiresAt: now.Add(BoardPostTTL),
	}
	s.board[post.PostID] = post
	s.byPost[poster] = post.PostID
	s.emitBoard()
	if p, _ := s.member(poster); p != nil {
		return res.ok(p.ID)
	}
	return res.ok(id.UUID{})
}

// BoardCancel handles C2S_PARTY_BOARD_CANCEL (635).
func (s *Service) BoardCancel(opID, poster id.UUID) Result {
	res := Result{RequestMessageID: ReqBoardCancel, OperationID: opID}
	postID, ok := s.byPost[poster]
	if !ok {
		return res.fail(v1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	s.closePost(postID, s.opt.now())
	if p, _ := s.member(poster); p != nil {
		return res.ok(p.ID)
	}
	return res.ok(id.UUID{})
}

// Attach marks a member online (session attach/resume).
func (s *Service) Attach(char id.UUID) {
	if _, m := s.member(char); m != nil {
		m.Online = true
		m.DisconnectedAt = time.Time{}
	}
}

// Detach marks a member offline and stamps the disconnect instant the
// leader grace clock needs (party.md § Disconnect).
func (s *Service) Detach(char id.UUID) {
	if _, m := s.member(char); m != nil {
		m.Online = false
		m.DisconnectedAt = s.opt.now()
	}
}

// Sweep expires invites, closes board posts and applies the leader
// disconnect grace. Safe to call at any cadence; idempotent.
func (s *Service) Sweep(now time.Time) {
	for char, inv := range s.invites {
		if inv.State == InvitePending && !inv.ExpiresAt.After(now) {
			inv.State = InviteExpired
			delete(s.invites, char)
		}
	}
	for pid, post := range s.board {
		if !post.ExpiresAt.After(now) {
			s.closePost(pid, now)
		}
	}
	for _, p := range s.parties {
		if !p.Active {
			continue
		}
		var leader *Member
		for _, m := range p.Members {
			if m.CharacterID == p.Leader {
				leader = m
			}
		}
		if leader == nil || leader.Online || now.Sub(leader.DisconnectedAt) < LeaderGrace {
			continue
		}
		if next := lowestOnline(p.Members); next != nil {
			p.Leader = next.CharacterID
			s.bump(p)
			s.emitState(p)
		}
	}
}

func (s *Service) createParty(leader id.UUID, _ time.Time) *Party {
	p := &Party{ID: id.NewV4(), Active: true}
	s.parties[p.ID] = p
	s.addMember(p, leader, s.opt.now())
	p.Leader = leader
	s.bump(p)
	return p
}

func (s *Service) addMember(p *Party, char id.UUID, _ time.Time) {
	name, classID, zoneID, level := s.profile(char)
	p.Members = append(p.Members, &Member{
		CharacterID:  char,
		DisplayName:  name,
		ClassID:      classID,
		Level:        level,
		ZoneID:       zoneID,
		JoinSequence: uint64(len(p.Members)) + 1,
		Online:       true,
	})
	s.byChar[char] = p.ID
	s.bump(p)
}

func (s *Service) removeMember(p *Party, char id.UUID) {
	for i, m := range p.Members {
		if m.CharacterID == char {
			p.Members = append(p.Members[:i], p.Members[i+1:]...)
			break
		}
	}
	delete(s.byChar, char)
}

func (s *Service) bump(p *Party) { p.Revision++ }

func (s *Service) profile(char id.UUID) (string, string, string, uint32) {
	if s.opt.MemberProfile != nil {
		n, c, z, l, _ := s.opt.MemberProfile(char)
		return n, c, z, l
	}
	return "", "", "", 0
}

func (s *Service) nameOf(char id.UUID) string {
	n, _, _, _ := s.profile(char)
	return n
}

func (s *Service) limited(char id.UUID, now time.Time) bool {
	window := now.Add(-inviteRateWindow)
	kept := s.rate[char][:0]
	for _, t := range s.rate[char] {
		if t.After(window) {
			kept = append(kept, t)
		}
	}
	s.rate[char] = kept
	if len(kept) >= inviteRateLimit {
		return true
	}
	s.rate[char] = append(s.rate[char], now)
	return false
}

func (s *Service) closePost(postID id.UUID, now time.Time) {
	post := s.board[postID]
	if post == nil {
		return
	}
	delete(s.board, postID)
	delete(s.byPost, post.Poster)
	s.repost[post.Poster] = now.Add(BoardRepostCooldown)
	s.emitBoard()
}

// StateView returns the live party view for a member (for 607 wiring).
func (s *Service) StateView(char id.UUID) *v1.S2CPartyState {
	p, _ := s.member(char)
	if p == nil || !p.Active {
		return nil
	}
	return stateProto(p)
}

// BoardView returns the live board snapshot (for 636 wiring).
func (s *Service) BoardView() *v1.S2CPartyBoardState {
	return boardProto(s.board)
}

// PendingInvite returns the pending invite addressed to char, if any.
func (s *Service) PendingInvite(char id.UUID) *Invite {
	if inv, ok := s.invites[char]; ok && inv.State == InvitePending {
		return inv
	}
	return nil
}

// Party returns the party of char (test/introspection helper).
func (s *Service) Party(char id.UUID) *Party {
	p, _ := s.member(char)
	return p
}

func (s *Service) emitState(p *Party) {
	if len(p.Members) == 0 {
		return
	}
	targets := make([]id.UUID, 0, len(p.Members))
	for _, m := range p.Members {
		targets = append(targets, m.CharacterID)
	}
	s.Outbox = append(s.Outbox, Event{
		Kind:    EventPartyState,
		Targets: targets,
		State:   stateProto(p),
	})
}

func (s *Service) emitBoard() {
	s.Outbox = append(s.Outbox, Event{
		Kind:  EventPartyBoard,
		Board: boardProto(s.board),
	})
}

func lowestSeq(members []*Member) *Member {
	var best *Member
	for _, m := range members {
		if best == nil || m.JoinSequence < best.JoinSequence {
			best = m
		}
	}
	return best
}

func lowestOnline(members []*Member) *Member {
	var best *Member
	for _, m := range members {
		if !m.Online {
			continue
		}
		if best == nil || m.JoinSequence < best.JoinSequence {
			best = m
		}
	}
	return best
}

func findMember(p *Party, char id.UUID) *Member {
	for _, m := range p.Members {
		if m.CharacterID == char {
			return m
		}
	}
	return nil
}

func stateProto(p *Party) *v1.S2CPartyState {
	out := &v1.S2CPartyState{
		PartyId:           p.ID[:],
		PartyRevision:     p.Revision,
		LeaderCharacterId: p.Leader[:],
	}
	for _, m := range p.Members {
		online := v1.OnlineState_ONLINE_STATE_OFFLINE
		if m.Online {
			online = v1.OnlineState_ONLINE_STATE_ONLINE
		}
		out.Members = append(out.Members, &v1.PartyMemberView{
			CharacterId: m.CharacterID[:],
			DisplayName: m.DisplayName,
			ClassId:     m.ClassID,
			Level:       m.Level,
			OnlineState: online,
			ZoneId:      m.ZoneID,
		})
	}
	return out
}

func boardProto(board map[id.UUID]*BoardPost) *v1.S2CPartyBoardState {
	out := &v1.S2CPartyBoardState{}
	for _, post := range board {
		out.Entries = append(out.Entries, &v1.PartyBoardEntry{
			PostId:            post.PostID[:],
			PosterCharacterId: post.Poster[:],
			DungeonId:         post.DungeonID,
			DesiredSize:       post.Size,
			ExpiresInSeconds:  uint32(time.Until(post.ExpiresAt) / time.Second),
		})
	}
	return out
}
