package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// defaultEndpoint is the world-host Collector address
// (observability.md § Launch Telemetry Stack).
const defaultEndpoint = "http://127.0.0.1:4318"

// SetupOptions configures Setup.
type SetupOptions struct {
	// Endpoint is the OTLP/HTTP Collector base URL. Empty means the
	// OTEL_EXPORTER_OTLP_ENDPOINT environment value, else the
	// world-host default http://127.0.0.1:4318.
	Endpoint string
	// QueueSize bounds the in-process export queue fronting the OTLP
	// exporters; enqueues past it drop and count. Default 1024.
	QueueSize int
	// Timeout bounds one queued export attempt; default 10 s.
	Timeout time.Duration
	// BatchTimeout is the span processor's batch window; default is the
	// SDK's 5 s.
	BatchTimeout time.Duration
	// BatchSize caps spans per export job; zero uses the SDK default
	// (512). One is useful to test queue-drop behavior.
	BatchSize int
	// MetricInterval is the PeriodicReader cadence; default 15 s
	// (observability.md scrape interval).
	MetricInterval time.Duration
	// Logger receives the single warn line on the first export
	// failure; default slog.Default().
	Logger *slog.Logger
}

// Stack is the wired telemetry stack: providers export through one
// bounded queue so a telemetry outage never blocks the server — it
// drops on a full export queue and counts the drops
// (observability.md § Launch Telemetry Stack invariant, ADR-0066).
type Stack struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Registry       *Registry
	queue          *exportQueue
}

// NewTracer returns a named tracer on the stack's provider.
func (s *Stack) NewTracer(name string) oteltrace.Tracer {
	return s.TracerProvider.Tracer(name)
}

// NewMeter returns a named meter on the stack's provider.
func (s *Stack) NewMeter(name string) otelmetric.Meter {
	return s.MeterProvider.Meter(name)
}

// Drops returns the number of export jobs rejected by a full queue.
func (s *Stack) Drops() int64 {
	return s.queue.drops.Load()
}

// Shutdown shuts the providers down (flushing pending jobs) then
// drains the export queue best-effort within ctx.
func (s *Stack) Shutdown(ctx context.Context) error {
	err := s.TracerProvider.Shutdown(ctx)
	if merr := s.MeterProvider.Shutdown(ctx); err == nil && merr != nil {
		err = merr
	}
	s.queue.close(ctx)
	return err
}

// Setup builds the launch telemetry stack from opts. Exporters are the
// pinned OTLP/HTTP modules only (otlptracehttp/otlpmetrichttp; the base
// otlptrace module is never imported — technology_versions.md).
func Setup(ctx context.Context, opts SetupOptions) (*Stack, error) {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("core: invalid OTLP endpoint %q", endpoint)
	}
	insecure := u.Scheme != "https"

	size := opts.QueueSize
	if size <= 0 {
		size = 1024
	}
	lg := opts.Logger
	if lg == nil {
		lg = slog.Default()
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	interval := opts.MetricInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	q := newExportQueue(size, lg, timeout)

	texp, err := otlptracehttp.New(ctx, traceOpts(u.Host, insecure)...)
	if err != nil {
		return nil, fmt.Errorf("core: otlp trace exporter: %w", err)
	}
	mexp, err := otlpmetrichttp.New(ctx, metricOpts(u.Host, insecure)...)
	if err != nil {
		return nil, fmt.Errorf("core: otlp metric exporter: %w", err)
	}

	bopts := []sdktrace.BatchSpanProcessorOption{}
	if opts.BatchTimeout > 0 {
		bopts = append(bopts, sdktrace.WithBatchTimeout(opts.BatchTimeout))
	}
	if opts.BatchSize > 0 {
		bopts = append(bopts, sdktrace.WithMaxExportBatchSize(opts.BatchSize))
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(
			sdktrace.NewBatchSpanProcessor(queueSpanExporter{inner: texp, q: q}, bopts...),
		),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(queueMetricExporter{inner: mexp, q: q}, sdkmetric.WithInterval(interval)),
		),
	)

	// observability.export_drops: the standing drop counter the
	// telemetry-outage invariant requires.
	meter := mp.Meter("thinhthan.observability")
	drops, err := meter.Int64ObservableCounter("observability.export_drops")
	if err != nil {
		return nil, fmt.Errorf("core: export_drops instrument: %w", err)
	}
	if _, err = meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		o.ObserveInt64(drops, q.drops.Load())
		return nil
	}, drops); err != nil {
		return nil, fmt.Errorf("core: export_drops callback: %w", err)
	}

	return &Stack{
		TracerProvider: tp,
		MeterProvider:  mp,
		Registry:       NewRegistry(mp),
		queue:          q,
	}, nil
}

func traceOpts(host string, insecure bool) []otlptracehttp.Option {
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(host)}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return opts
}

func metricOpts(host string, insecure bool) []otlpmetrichttp.Option {
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(host)}
	if insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	return opts
}

// exportQueue is the bounded in-process queue fronting both OTLP
// exporters. enqueue never blocks: past capacity the job is dropped and
// counted. The worker serializes exports and swallows every sink error
// after one slog record (suppressed repeats until the next success).
type exportQueue struct {
	jobs     chan func(context.Context) error
	done     chan struct{}
	lg       *slog.Logger
	timeout  time.Duration
	drops    atomic.Int64
	warnOnce atomic.Int32
}

func newExportQueue(size int, lg *slog.Logger, timeout time.Duration) *exportQueue {
	q := &exportQueue{
		jobs:    make(chan func(context.Context) error, size),
		done:    make(chan struct{}),
		lg:      lg,
		timeout: timeout,
	}
	go q.run()
	return q
}

func (q *exportQueue) enqueue(j func(context.Context) error) {
	select {
	case q.jobs <- j:
	default:
		q.drops.Add(1)
	}
}

func (q *exportQueue) run() {
	defer close(q.done)
	for j := range q.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
		err := j(ctx)
		cancel()
		if err != nil {
			if q.warnOnce.CompareAndSwap(0, 1) {
				q.lg.Warn("telemetry export failing; repeats suppressed until next success", "err", err.Error())
			}
		} else {
			q.warnOnce.Store(0)
		}
	}
}

func (q *exportQueue) close(ctx context.Context) {
	close(q.jobs)
	select {
	case <-q.done:
	case <-ctx.Done():
	}
}

// queueSpanExporter adapts the real OTLP trace exporter to
// sdktrace.SpanExporter, fronting it with the bounded queue.
type queueSpanExporter struct {
	inner sdktrace.SpanExporter
	q     *exportQueue
}

func (e queueSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	spans = append([]sdktrace.ReadOnlySpan{}, spans...)
	e.q.enqueue(func(jobCtx context.Context) error {
		return e.inner.ExportSpans(jobCtx, spans)
	})
	return nil
}

func (e queueSpanExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}

// queueMetricExporter adapts the real OTLP metric exporter to
// sdkmetric.Exporter, fronting it with the bounded queue.
type queueMetricExporter struct {
	inner sdkmetric.Exporter
	q     *exportQueue
}

func (e queueMetricExporter) Temporality(k sdkmetric.InstrumentKind) metricdata.Temporality {
	return e.inner.Temporality(k)
}

func (e queueMetricExporter) Aggregation(k sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return e.inner.Aggregation(k)
}

func (e queueMetricExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	rm = cloneResourceMetrics(rm)
	e.q.enqueue(func(jobCtx context.Context) error {
		return e.inner.Export(jobCtx, rm)
	})
	return nil
}

func (e queueMetricExporter) ForceFlush(ctx context.Context) error {
	return e.inner.ForceFlush(ctx)
}

func (e queueMetricExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}
