package runtime

import (
	"context"
	"fmt"
)

// Host is one registered global subsystem (party, chat, matchmaking,
// Spirit Surge, boss generation, guild, moderation). Host state lives on
// the single writer goroutine: Restore runs at Start before any command
// is accepted; OnCommand runs after each routed command's Execute on that
// same goroutine (service_boundaries.md § Ephemeral Global Runtime).
type Host interface {
	// Name identifies the host in the registry.
	Name() string
	// Restore rebuilds durable-backed state at Start, before intake opens.
	// Ephemeral state (party/chat/matchmaking) starts empty per spec.
	Restore(ctx context.Context, view *DurableView) error
	// OnCommand observes a committed command routed to this host — a host
	// callback on the writer goroutine (never blocks the mailbox).
	OnCommand(ctx context.Context, cmd Command) error
}

// RegisterHost installs a host and routes the listed command families to
// it. Hosts must be registered before Start; an unknown family is
// rejected at Submit with ErrUnknownCommandFamily.
func (r *Runtime) RegisterHost(h Host, families ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != stateNew {
		return ErrNotRunning
	}
	r.hosts = append(r.hosts, h)
	for _, f := range families {
		r.routes[f] = h
	}
	return nil
}

// restoreAll runs every registered host's Restore in registration order
// before the writer goroutine accepts commands — mirrors the
// load-all-rows-before-partitions-accept-players rule (bosses.md).
func (r *Runtime) restoreAll(ctx context.Context) error {
	for _, h := range r.hosts {
		if err := h.Restore(ctx, r.view); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrHostRestore, h.Name(), err)
		}
	}
	return nil
}
