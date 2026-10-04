package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestCorrelationContext(t *testing.T) {
	rev := strings.Repeat("a", 64)
	ctx := With(context.Background(), Context{
		Operation: "reward_claim",
		Source:    "partition/7",
		Revision:  rev,
		Retry:     2,
		Extra:     map[string]string{"account_id": "acct-9"},
	})
	c := From(ctx)
	if c.Operation != "reward_claim" || c.Source != "partition/7" || c.Revision != rev || c.Retry != 2 {
		t.Fatalf("From(ctx) = %+v", c)
	}
	if got := From(NextRetry(ctx)); got.Retry != 3 {
		t.Fatalf("NextRetry retry = %d, want 3", got.Retry)
	}
	if got := From(WithOperation(context.Background(), "login")); got.Operation != "login" {
		t.Fatalf("WithOperation op = %q", got.Operation)
	}

	var buf bytes.Buffer
	WithContext(NewLogger(&buf, slog.LevelInfo, false), ctx).Info("claim.ok")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line not JSON: %v", err)
	}
	for k, want := range map[string]any{
		"op":         "reward_claim",
		"revision":   rev,
		"source":     "partition/7",
		"retry":      float64(2),
		"account_id": "acct-9",
	} {
		if line[k] != want {
			t.Fatalf("log attr %q = %v, want %v (line %v)", k, line[k], want, line)
		}
	}
	if got := SpanAttrs(ctx); len(got) < 5 {
		t.Fatalf("SpanAttrs len = %d, want >=5", len(got))
	}
}

func TestSecretRedaction(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, slog.LevelInfo, false)
	l.Info("login.ok",
		"user", "u1",
		"password", "hunter2",
		"auth_token", "tok-abc",
		slog.Group("db", "host", "pg1", "password", "pg-secret", "dsn", "x"),
		slog.Group("resume", "session_resume", "r9"),
	)
	out := buf.String()
	for _, want := range []string{
		`"password":"***"`, `"auth_token":"***"`, `"session_resume":"***"`,
		`"user":"u1"`, `"host":"pg1"`, `"dsn":"x"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log line missing %s: %s", want, out)
		}
	}
	for _, leak := range []string{"hunter2", "tok-abc", "pg-secret", "r9"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log line leaked %q: %s", leak, out)
		}
	}
}

func TestBoundedCardinality(t *testing.T) {
	r := NewRegistry(sdkmetric.NewMeterProvider())
	if _, err := r.Register(Descriptor{Kind: Counter}); err == nil {
		t.Fatal("empty name registered")
	}
	if _, err := r.Register(Descriptor{
		Name: "unbounded", Kind: Counter,
		Labels: []LabelDomain{{Key: "free"}},
	}); err == nil {
		t.Fatal("unbounded label set registered")
	}
	inst, err := r.Register(Descriptor{
		Name: "requests_total", Kind: Counter,
		Labels: []LabelDomain{{Key: "map_id", Values: []string{"m1", "m2"}}},
	})
	if err != nil {
		t.Fatalf("bounded register: %v", err)
	}
	ctx := context.Background()
	inst.Add(ctx, 1, map[string]string{"map_id": "m1"})
	inst.Add(ctx, 1, map[string]string{"map_id": "not-declared-value"}) // folds to __other__
	inst.Add(ctx, 1, nil)
	inst.Add(ctx, 1, map[string]string{"undeclared_key": "x"}) // dropped: unknown key
	if attrs, ok := inst.bind(map[string]string{"undeclared_key": "x"}); ok {
		t.Fatalf("undeclared label bound: %v", attrs)
	}

	capped, err := r.Register(Descriptor{
		Name: "per_map_total", Kind: Counter,
		Labels: []LabelDomain{{Key: "map_id", MaxDistinct: 2}},
	})
	if err != nil {
		t.Fatalf("MaxDistinct register: %v", err)
	}
	capped.Add(ctx, 1, map[string]string{"map_id": "a"})
	capped.Add(ctx, 1, map[string]string{"map_id": "b"})
	capped.Add(ctx, 1, map[string]string{"map_id": "c"})
	if got := len(capped.seen["map_id"]); got != 2 {
		t.Fatalf("distinct values kept = %d, want capped at 2", got)
	}
}

type failHandler struct{}

func (failHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (failHandler) Handle(context.Context, slog.Record) error { return errors.New("sink down") }
func (h failHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h failHandler) WithGroup(string) slog.Handler           { return h }

func TestSinkFailureNonBlocking(t *testing.T) {
	// A permanently failing log sink: calls return fast, nothing
	// propagates (slog drops handler errors by design).
	bad := slog.New(RedactHandler(failHandler{}, nil))
	for i := 0; i < 50; i++ {
		bad.Info("tick.ok", "i", i)
	}

	// A failing export sink: emit calls stay fast against an
	// unreachable Collector.
	s, err := Setup(context.Background(), SetupOptions{
		Endpoint: "http://127.0.0.1:1",
		Timeout:  200 * time.Millisecond,
		Logger:   NewLogger(io.Discard, slog.LevelError, false),
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	tr := s.NewTracer("test")
	start := time.Now()
	for i := 0; i < 20; i++ {
		_, sp := tr.Start(context.Background(), "op")
		sp.End()
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("emit loop blocked %v against dead sink", took)
	}
	shCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(shCtx); err != nil && shCtx.Err() == nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestOtlpExportToLocalEndpoint(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()
	s, err := Setup(ctx, SetupOptions{
		Endpoint:       srv.URL,
		BatchTimeout:   20 * time.Millisecond,
		MetricInterval: 50 * time.Millisecond,
		Timeout:        2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shCtx)
	}()

	_, sp := s.NewTracer("probe").Start(ctx, "probe.op")
	sp.End()
	inst, err := s.Registry.Register(Descriptor{Name: "probe_total", Kind: Counter})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	inst.Add(ctx, 3, nil)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := hits["/v1/traces"] > 0 && hits["/v1/metrics"] > 0
		mu.Unlock()
		if ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("OTLP payloads missing after 10s: %v", hits)
}

func TestCollectorDownDropsAndCounts(t *testing.T) {
	// Saturated export path: a Collector that accepts but stalls every
	// POST 300 ms makes the bounded queue fill; enqueue-full jobs are
	// dropped and counted.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	ctx := context.Background()
	s, err := Setup(ctx, SetupOptions{
		Endpoint:     slow.URL,
		QueueSize:    4,
		Timeout:      2 * time.Second,
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    1,
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shCtx)
	}()

	tr := s.NewTracer("t")
	for i := 0; i < 20; i++ {
		_, sp := tr.Start(ctx, "op")
		sp.End()
	}
	if err := s.TracerProvider.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for s.Drops() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := s.Drops(); got == 0 {
		t.Fatal("full export queue never dropped a job")
	}
}
