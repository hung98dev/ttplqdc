package heartbeat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestHeartbeatTimeout15s: the connection is declared lost after 15 s
// without valid traffic (protocol.md § Heartbeat).
func TestHeartbeatTimeout15s(t *testing.T) {
	var now atomic.Int64
	base := time.Now()
	now.Store(base.UnixNano())
	var lastValid atomic.Int64
	lastValid.Store(base.UnixMilli())
	lost := make(chan struct{}, 1)
	sent := make(chan uint64, 32)
	m := &Monitor{
		Interval: 5 * time.Second,
		Timeout:  15 * time.Second,
		Now:      func() time.Time { return time.Unix(0, now.Load()) },
		Send: func(ms uint64) error {
			sent <- ms
			return nil
		},
		LastValidMS: func() int64 { return lastValid.Load() },
		Lost:        func() { lost <- struct{}{} },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()

	// First heartbeat fires immediately at start.
	select {
	case <-sent:
	case <-time.After(2 * time.Second):
		t.Fatal("initial heartbeat missing")
	}
	// Advance the fake clock past the timeout without traffic.
	now.Add(int64(16 * time.Second))
	select {
	case <-lost:
	case <-time.After(3 * time.Second):
		t.Fatal("connection must be declared lost after 15 s without valid traffic")
	}
	cancel()
	<-done
}

func TestRttSampleFromEcho(t *testing.T) {
	recv := time.UnixMilli(10_000)
	s, ok := SampleFromEcho(recv, 0)
	if ok {
		t.Fatalf("echo 0 must not produce a sample (spec: only when != 0)")
	}
	s, ok = SampleFromEcho(recv, 9_900)
	if !ok {
		t.Fatal("echo != 0 must produce a sample")
	}
	if s.RTT != 100*time.Millisecond {
		t.Fatalf("RTT = server recv - echo_server_ms: got %v want 100ms", s.RTT)
	}
	if s.EchoServerMS != 9_900 || !s.ServerRecv.Equal(recv) {
		t.Fatalf("sample fields: %+v", s)
	}
}
