package auth

import (
	"context"
	"errors"
	"math"
	"time"

	"thinhthan/internal/durable/account"
)

// L2 limiter (external_integrations.md §3 + rate_limits.md §
// Authentication): Postgres-bucketed sliding window keyed on HMAC'd
// action:scope:value tuples, plus progressive auth-failure backoff.
const (
	ScopeAccount  = "ACCOUNT"
	ScopeUsername = "USERNAME"
	ScopeIP       = "IP"
	ScopeIPDevice = "IP_DEVICE"
)

// Action names from the rate_limits.md table.
const (
	ActionFederatedLogin = "auth.federated.login"
	ActionPasswordLogin  = "auth.password.login"
	ActionRegister       = "auth.password.register"
	ActionRefresh        = "auth.refresh"
	ActionPasswordChange = "auth.password.change"
	ActionLinkUnlink     = "auth.link_unlink"
	ActionGameplayTicket = "gameplay.ticket"
	ActionDeleteCancel   = "account.delete_cancel"
)

// Backoff schedule (auth_failure_backoff): from the 5th consecutive
// verified failure, locked_until = now + min(30s * 2^(n-5), 900s); rows
// decay one failure per 10 minutes without a new failure.
const (
	backoffFirstLockedAt = 5
	backoffBaseDelay     = 30 * time.Second
	backoffMaxDelay      = 15 * time.Minute
	backoffDecay         = 10 * time.Minute
)

// Limiter is the L2 limiter over the durable counter/backoff tables.
type Limiter struct {
	store *account.Store
	salt  account.Salt
	now   func() time.Time
}

// NewLimiter binds the limiter to the store + signal salt.
func NewLimiter(store *account.Store, salt account.Salt, now func() time.Time) *Limiter {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Limiter{store: store, salt: salt, now: now}
}

// Check consumes one unit of the action/scope bucket. It returns nil or
// ErrRateLimited carrying the spec retry_after (time until the effective
// count decays under the limit).
func (l *Limiter) Check(ctx context.Context, action, scopeKind, scopeValue string, limit int32, window time.Duration) error {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	now := l.now()
	aligned := windowStart(now, window)
	current, previous, _, err := l.store.BumpCounter(ctx, key[:], int32(window/time.Second), aligned)
	if err != nil {
		return err
	}
	effective := float64(current) + float64(previous)*(1.0-float64(now.Sub(aligned))/float64(window))
	if int64(math.Ceil(effective)) <= int64(limit) {
		return nil
	}
	// Decay time until effective <= limit: previous weight needs to drop by
	// (effective - limit) below previous_count. Solve on the window edge.
	retry := retryAfter(now, aligned, window, float64(limit)-float64(current), float64(previous))
	if uerr := l.store.UnbumpCounter(ctx, key[:]); uerr != nil {
		return uerr
	}
	return ErrRateLimited(retry.Milliseconds())
}

// Peek reports whether one more unit would fit without consuming —
// capacity buckets need no consume-on-success semantics.
func (l *Limiter) Peek(ctx context.Context, action, scopeKind, scopeValue string, limit int32, window time.Duration) error {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	now := l.now()
	c, err := l.store.GetCounter(ctx, nil, key[:])
	if errors.Is(err, account.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	aligned := windowStart(now, window)
	var effective float64
	switch {
	case !c.WindowStart.Before(aligned):
		effective = float64(c.CurrentCount) + float64(c.PreviousCount)*(1.0-float64(now.Sub(aligned))/float64(window))
	case c.WindowStart.Add(window).After(aligned):
		effective = float64(c.CurrentCount) * (1.0 - float64(now.Sub(aligned))/float64(window))
	default:
		effective = 0
	}
	if int64(math.Ceil(effective))+1 > int64(limit) {
		retry := retryAfter(now, aligned, window, float64(limit), float64(c.CurrentCount))
		return ErrRateLimited(retry.Milliseconds())
	}
	return nil
}

// Consume records one request unconditionally (used for capacity buckets
// checked before expensive work; the request still counted even when a
// later gate rejects it).
func (l *Limiter) Consume(ctx context.Context, action, scopeKind, scopeValue string, window time.Duration) error {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	now := l.now()
	_, _, _, err := l.store.BumpCounter(ctx, key[:], int32(window/time.Second), windowStart(now, window))
	return err
}

// Clear removes one counter row (a correct password clears the USERNAME
// failure budget per rate_limits.md).
func (l *Limiter) Clear(ctx context.Context, action, scopeKind, scopeValue string) error {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	return l.store.DeleteCounter(ctx, key[:])
}

// ---------------------------------------------------------------------------
// Progressive failure backoff (auth_failure_backoff)

// BackoffLocked reports the lock for one backoff key (decay applied on
// read; lock applies even when the row has decayed under threshold).
func (l *Limiter) BackoffLocked(ctx context.Context, action, scopeKind, scopeValue string) (locked bool, retryAfter time.Duration, err error) {
	row, ok, err := l.backoff(ctx, action, scopeKind, scopeValue)
	if err != nil || !ok {
		return false, 0, err
	}
	now := l.now()
	if row.LockedUntil != nil && row.LockedUntil.After(now) {
		return true, row.LockedUntil.Sub(now), nil
	}
	return false, 0, nil
}

// backoff loads one row and applies the 10-minute decay.
func (l *Limiter) backoff(ctx context.Context, action, scopeKind, scopeValue string) (account.BackoffRow, bool, error) {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	row, err := l.store.GetBackoff(ctx, nil, key[:])
	if errors.Is(err, account.ErrNotFound) {
		return account.BackoffRow{}, false, nil
	}
	if err != nil {
		return account.BackoffRow{}, false, err
	}
	now := l.now()
	if decay := int32(now.Sub(row.LastFailureAt) / backoffDecay); decay > 0 {
		row.ConsecutiveFailures -= decay
		if row.ConsecutiveFailures < 0 {
			row.ConsecutiveFailures = 0
		}
	}
	return row, true, nil
}

// RecordFailure adds one verified failure and (re)arms the lock.
func (l *Limiter) RecordFailure(ctx context.Context, action, scopeKind, scopeValue string) error {
	row, _, err := l.backoff(ctx, action, scopeKind, scopeValue)
	if err != nil {
		return err
	}
	now := l.now()
	n := row.ConsecutiveFailures + 1
	out := account.BackoffRow{
		KeyHash:             l.rateKey(action, scopeKind, scopeValue),
		ConsecutiveFailures: n,
		LastFailureAt:       now,
	}
	if n >= backoffFirstLockedAt {
		delay := backoffBaseDelay << (n - backoffFirstLockedAt)
		if delay > backoffMaxDelay || delay < 0 {
			delay = backoffMaxDelay
		}
		until := now.Add(delay)
		out.LockedUntil = &until
	}
	return l.store.UpsertBackoff(ctx, out)
}

// ClearBackoff deletes one backoff row (login success clears USERNAME and
// source-IP backoff rows, not the IP capacity counter).
func (l *Limiter) ClearBackoff(ctx context.Context, action, scopeKind, scopeValue string) error {
	key := l.salt.RateKey(action, scopeKind, scopeValue)
	return l.store.DeleteBackoff(ctx, key[:])
}

func (l *Limiter) rateKey(action, scopeKind, scopeValue string) []byte {
	k := l.salt.RateKey(action, scopeKind, scopeValue)
	return k[:]
}

// windowStart is date_bin(window, now, epoch): the aligned bucket start.
func windowStart(now time.Time, window time.Duration) time.Time {
	return now.Truncate(window)
}

// retryAfter solves the time for effective to decay to room: effective =
// current + previous·(1 − elapsed/window) ≤ limit ⇒ elapsed/window ≥
// 1 − (limit−current)/previous ⇒ t = window·(1 − (limit−current)/previous)
// beyond the aligned start.
func retryAfter(now, aligned time.Time, window time.Duration, room, previous float64) time.Duration {
	if previous <= 0 {
		return 0
	}
	frac := 1.0 - room/previous
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	target := aligned.Add(time.Duration(frac * float64(window)))
	if d := target.Sub(now); d > 0 {
		return d + time.Millisecond
	}
	return 0
}
