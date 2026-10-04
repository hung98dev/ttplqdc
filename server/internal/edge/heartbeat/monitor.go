package heartbeat

import (
	"context"
	"time"
)

// Monitor drives the per-connection heartbeat: one S2C_HEARTBEAT every
// Interval, connection lost after Timeout without valid traffic
// (protocol.md § Heartbeat, defaults 5 s / 15 s, runtime-tunable).
type Monitor struct {
	Interval time.Duration
	Timeout  time.Duration

	// Send emits S2C_HEARTBEAT{server_ms} on the connection. serverMS is a
	// Unix-millis timestamp taken from Now.
	Send func(serverMS uint64) error
	// LastValidMS reports the Unix-millis receive time of the last frame
	// that counted as valid traffic.
	LastValidMS func() int64
	// Lost is invoked once when the connection is declared lost.
	Lost func()
	// Now is the clock; injectable for tests.
	Now func() time.Time
}

// Run ticks until ctx is done or the connection is declared lost. The check
// cadence is the smaller of Interval/4 and one second so detection lands
// well inside Timeout; sends fire exactly on Interval boundaries.
func (m *Monitor) Run(ctx context.Context) {
	now := m.Now
	if now == nil {
		now = time.Now
	}
	interval := m.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	gran := interval / 4
	if gran > time.Second {
		gran = time.Second
	}
	if gran <= 0 {
		gran = time.Millisecond
	}
	t := now()
	tk := time.NewTicker(gran)
	defer tk.Stop()
	lastSend := t.Add(-interval) // first heartbeat fires on the first tick
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			t = now()
			if m.LastValidMS != nil {
				lv := time.UnixMilli(m.LastValidMS())
				if t.Sub(lv) > timeout {
					if m.Lost != nil {
						m.Lost()
					}
					return
				}
			}
			if t.Sub(lastSend) >= interval {
				lastSend = t
				if m.Send != nil {
					if err := m.Send(uint64(t.UnixMilli())); err != nil {
						return
					}
				}
			}
		}
	}
}
