package party

import (
	"time"

	"thinhthan/internal/core/id"
)

// Canonical constants (party.md).
const (
	MaxMembers          = 5
	InviteTTL           = 60 * time.Second
	LeaderGrace         = 120 * time.Second
	BoardPostTTL        = 120 * time.Second
	BoardRepostCooldown = 30 * time.Second
	BoardNoteMax        = 40
	inviteRateLimit     = 10
	inviteRateWindow    = 60 * time.Second
)

// Request message ids — messages.md § Party; every request yields
// exactly one Result carrying its id (ADR-0064).
const (
	ReqInvite         uint32 = 602
	ReqAccept         uint32 = 604
	ReqLeave          uint32 = 605
	ReqKick           uint32 = 606
	ReqDecline        uint32 = 620
	ReqInviteCancel   uint32 = 621
	ReqLeaderTransfer uint32 = 622
	ReqBoardPost      uint32 = 634
	ReqBoardCancel    uint32 = 635
)

// InviteState — party.md § Invitations.
type InviteState int32

const (
	InvitePending InviteState = iota + 1
	InviteAccepted
	InviteDeclined
	InviteCancelled
	InviteExpired
)

// Member is one roster row; JoinSequence is monotonic per party.
type Member struct {
	CharacterID    id.UUID
	DisplayName    string
	ClassID        string
	Level          uint32
	ZoneID         string
	JoinSequence   uint64
	Online         bool
	DisconnectedAt time.Time
}

// Party state (party.md: ACTIVE / DISBANDED; ids never reused).
type Party struct {
	ID       id.UUID
	Revision uint64
	Leader   id.UUID
	Members  []*Member
	Active   bool
}

// Invite is one 60 s invitation between two characters.
type Invite struct {
	PartyID   id.UUID
	Inviter   id.UUID
	Target    id.UUID
	ExpiresAt time.Time
	State     InviteState
}

// BoardPost is one live safe-anchor party-board entry.
type BoardPost struct {
	PostID    id.UUID
	Poster    id.UUID
	DungeonID string
	Size      uint32
	Note      string
	ExpiresAt time.Time
}
