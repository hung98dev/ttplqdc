// Package social is the ephemeral global-runtime surface for friends,
// blocks and chat (service_boundaries.md § Global): the wire handlers
// for the social intents, the per-channel rate limiters, presence
// lookups and fanout pushes. Durable mutations commit inside
// client.<ID> transactions through the durable queue; every persisted
// change is followed by the spec's 612/616/619 pushes.
package social

import (
	"context"
	"errors"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ErrNotMember: the sender is not in the channel's roster (PARTY/GUILD)
// — maps to PERMISSION_DENIED on the wire.
var ErrNotMember = errors.New("social: not a channel member")

// Presence is the runtime-visible state of one character.
type Presence struct {
	Online   bool
	ZoneID   string
	Activity protocolv1.FriendActivity
}

// Hub is the session/presence fanout surface the composition root
// (IMP-069) binds in production; tests drive a fake. Deliver targets
// every live session attached as the character.
type Hub interface {
	// Deliver pushes one S2C frame to every live session of
	// characterID. An unknown/offline character is a no-op.
	Deliver(ctx context.Context, characterID id.UUID, msgID uint32, m proto.Message) error
	// Presence reads the character's runtime presence — OFFLINE when
	// unattached or unknown.
	Presence(characterID id.UUID) Presence
	// Fanout resolves the recipient character set for one chat
	// channel: WORLD = every attached character, LOCAL = the sender's
	// zone, PARTY/GUILD = the sender's roster, WHISPER = {sender,
	// target}. ErrNotMember when the sender lacks the channel.
	Fanout(ctx context.Context, channel protocolv1.ChatChannel,
		senderID, targetID id.UUID) ([]id.UUID, error)
}

// Moderation is the communication-restriction consult the composition
// root binds in production (social.md § Moderation; send admission
// requires "moderation restrictions pass"). IMP-094 implements the
// real consult against global/moderation; a nil consult admits every
// send. CheckSend returns ok=false plus the wire error to fail the
// send with when the sender or message is restricted (e.g. MUTED on
// this channel or a rejected content filter).
type Moderation interface {
	CheckSend(ctx context.Context, senderID id.UUID,
		channel protocolv1.ChatChannel, targetID id.UUID,
		text string) (protocolv1.ErrorCode, bool)
}
