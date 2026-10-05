// Package router classifies inbound C2S messages against the closed
// durable-intent set of protobuf_conventions.md §7 (CLIENT_EXPANSION)
// and dispatches them to registered per-ID handlers. Later tasks
// register handlers; the classification table here is complete and
// read-only for them.
package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Route is one classified durable intent.
type Route struct {
	// MsgID is the C2S wire id.
	MsgID uint32
	// Family is the §7 durable family (e.g. "inventory.mutate",
	// "interaction.cook", "client.608").
	Family string
	// Inbound is the listener's decoded intent.
	Inbound listener.Inbound
}

// Handler processes one durable intent. Registered per MsgID.
type Handler func(ctx context.Context, c *listener.Conn, r Route) error

var (
	// ErrNotDurableIntent: MsgID is outside the closed §7 set.
	ErrNotDurableIntent = errors.New("router: not a durable intent")
	// ErrDuplicateID: a handler is already registered for MsgID.
	ErrDuplicateID = errors.New("router: duplicate message id")
	// ErrUnregistered: durable MsgID with no registered handler.
	ErrUnregistered = errors.New("router: no handler registered")
)

// RejectError carries the wire code a handler wants sent back as
// S2C_ERROR (non-closing; adapter maps it).
type RejectError struct {
	Code protocolv1.ErrorCode
}

func (e *RejectError) Error() string { return fmt.Sprintf("router: reject %v", e.Code) }

// durableFamiliesExact is the §7 client-expansion table minus the
// interaction.<kind> (103) and client.<ID> (social) patterns.
var durableFamiliesExact = map[uint32]string{
	12:  "character.create",
	104: "placement.portal",
	109: "placement.channel",
	111: "instance.enter",
	113: "instance.respond",
	114: "instance.leave",
	117: "instance.cancel",
	400: "inventory.mutate",
	402: "loadout.change",
	404: "craft.create",
	406: "enhance.apply",
	408: "reward.claim",
	410: "beast.active",
	412: "beast.equip",
	414: "beast.unequip",
	416: "beast.feed",
	418: "entitlement.claim",
	420: "shop.buy",
	422: "cosmetic.redeem",
	424: "cosmetic.equip",
	426: "shop.sell",
	428: "inventory.expand",
	430: "beast.level_up",
	500: "quest.accept",
	502: "quest.turn_in",
	504: "atlas.acknowledge",
	507: "quest.abandon",
	509: "story.choose",
	511: "skill.upgrade",
	512: "potential.allocate",
	513: "progression.respec",
	708: "trade.finalise",
	730: "auction.list",
	732: "auction.buy",
	734: "auction.cancel",
	740: "auction.reclaim",
	742: "auction.proceeds",
}

// clientIDSet is the §7 social expansion mapping to `client.<ID>`.
var clientIDSet = map[uint32]struct{}{
	608: {}, 610: {}, 611: {}, 613: {}, 614: {}, 615: {}, 617: {}, 618: {},
	623: {}, 624: {}, 625: {}, 626: {}, 627: {}, 629: {}, 630: {}, 632: {},
	637: {}, 638: {}, 639: {}, 640: {}, 642: {}, 643: {}, 644: {}, 645: {},
	646: {}, 648: {}, 650: {}, 651: {}, 652: {}, 656: {},
}

// msgIDInteract is C2S_INTERACT — its family is
// interaction.<interact_kind lowercase> resolved from the payload.
const msgIDInteract = 103

// IsDurableIntent reports whether the wire id belongs to the closed §7
// durable-intent set.
func IsDurableIntent(msgID uint32) bool {
	if _, ok := durableFamiliesExact[msgID]; ok {
		return true
	}
	if msgID == msgIDInteract {
		return true
	}
	_, ok := clientIDSet[msgID]
	return ok
}

// FamilyFor resolves the §7 family for a durable intent. For id 103 the
// interact_kind is read from the decoded *C2SInteract payload; other ids
// need no payload.
func FamilyFor(msgID uint32, payload any) (string, bool) {
	if msgID == msgIDInteract {
		m, ok := payload.(*protocolv1.C2SInteract)
		if !ok {
			return "", false
		}
		name := m.GetInteractKind().String()
		kind, ok := strings.CutPrefix(name, "INTERACT_KIND_")
		if !ok || m.GetInteractKind() == protocolv1.InteractKind_INTERACT_KIND_UNSPECIFIED {
			return "", false
		}
		return "interaction." + strings.ToLower(kind), true
	}
	if fam, ok := durableFamiliesExact[msgID]; ok {
		return fam, true
	}
	if _, ok := clientIDSet[msgID]; ok {
		return fmt.Sprintf("client.%d", msgID), true
	}
	return "", false
}

// Registry is the router: closed classification + per-id handler table.
type Registry struct {
	mu       sync.RWMutex
	handlers map[uint32]Handler
}

// New builds an empty router.
func New() *Registry {
	return &Registry{handlers: make(map[uint32]Handler)}
}

// Register installs the handler for a durable msgID; duplicates are an
// error (handler table is closed at startup).
func (r *Registry) Register(msgID uint32, h Handler) error {
	if !IsDurableIntent(msgID) {
		return fmt.Errorf("%w: %d", ErrNotDurableIntent, msgID)
	}
	if h == nil {
		return fmt.Errorf("%w: nil handler for %d", ErrUnregistered, msgID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.handlers[msgID]; dup {
		return fmt.Errorf("%w: %d", ErrDuplicateID, msgID)
	}
	r.handlers[msgID] = h
	return nil
}

// Registered returns the sorted registered id set (tests/audits).
func (r *Registry) Registered() []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]uint32, 0, len(r.handlers))
	for id := range r.handlers {
		out = append(out, id)
	}
	return out
}

// Handles implements the session adapter's router check: every durable
// intent routes through Dispatch (unregistered ids reject there).
func (r *Registry) Handles(msgID uint32) bool { return IsDurableIntent(msgID) }

// Dispatch classifies the inbound intent and calls its handler. Unknown
// durable intents are rejected with MESSAGE_NOT_ALLOWED_IN_STATE; a
// non-durable id returns ErrNotDurableIntent for the caller to ignore.
func (r *Registry) Dispatch(ctx context.Context, c *listener.Conn, in listener.Inbound) error {
	fam, ok := FamilyFor(in.MessageID, in.Payload)
	if !ok {
		if IsDurableIntent(in.MessageID) {
			return &RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
		}
		return fmt.Errorf("%w: %d", ErrNotDurableIntent, in.MessageID)
	}
	r.mu.RLock()
	h, ok := r.handlers[in.MessageID]
	r.mu.RUnlock()
	if !ok {
		return &RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_OPERATION_REJECTED}
	}
	return h(ctx, c, Route{MsgID: in.MessageID, Family: fam, Inbound: in})
}
