package social

import (
	"sync"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// window is a fixed-interval limiter per social.md § Rate Limits.
type window struct {
	limit int
	span  time.Duration
}

// rateWindows are the per-channel send budgets plus the social-invite
// budget (messages.md: friend request 10/60s).
var rateWindows = map[protocolv1.ChatChannel]window{
	protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL:   {5, 10 * time.Second},
	protocolv1.ChatChannel_CHAT_CHANNEL_WORLD:   {2, 10 * time.Second},
	protocolv1.ChatChannel_CHAT_CHANNEL_PARTY:   {10, 10 * time.Second},
	protocolv1.ChatChannel_CHAT_CHANNEL_GUILD:   {10, 10 * time.Second},
	protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER: {5, 10 * time.Second},
}

const (
	inviteLimit = 10
	inviteSpan  = 60 * time.Second
)

// limiter is the runtime's rolling-window rate budget: per key it keeps
// the accepted timestamps inside the trailing span.
type limiter struct {
	mu    sync.Mutex
	times map[string][]time.Time
}

func newLimiter() *limiter {
	return &limiter{times: map[string][]time.Time{}}
}

// allow records one attempt at now; it returns false once limit events
// already sit inside the trailing span.
func (l *limiter) allow(key string, limit int, span time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := now.Add(-span)
	ts := l.times[key]
	kept := ts[:0]
	for _, t := range ts {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.times[key] = kept
		return false
	}
	l.times[key] = append(kept, now)
	return true
}

// allowChat applies the channel window; WHISPER budgets per
// (sender, target) pair per social.md.
func (l *limiter) allowChat(channel protocolv1.ChatChannel,
	sender, target id.UUID, now time.Time) bool {
	w, ok := rateWindows[channel]
	if !ok {
		return true
	}
	key := channel.String() + ":" + sender.String()
	if channel == protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER {
		key += ":" + target.String()
	}
	return l.allow(key, w.limit, w.span, now)
}

// allowInvite applies the social-invite budget (friend request 10/60s).
func (l *limiter) allowInvite(sender id.UUID, now time.Time) bool {
	return l.allow("invite:"+sender.String(), inviteLimit, inviteSpan, now)
}
