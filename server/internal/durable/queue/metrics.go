package queue

import (
	"context"

	obscore "thinhthan/internal/observability/core"
)

// queueMetrics holds the queue's bounded-cardinality instruments
// (concurrency.md § Queue Rules: depth/drop/reject metrics;
// capacity.md durable SLOs are reported through the latency histograms).
type queueMetrics struct {
	depth    *obscore.Instrument
	submits  *obscore.Instrument
	inflight *obscore.Instrument
	commitMs *obscore.Instrument
	waitMs   *obscore.Instrument
}

var (
	kindLabels   = obscore.LabelDomain{Key: "kind", MaxDistinct: 16}
	resultLabels = obscore.LabelDomain{Key: "result", MaxDistinct: 8}
)

func newQueueMetrics(reg *obscore.Registry) *queueMetrics {
	m := &queueMetrics{}
	if reg == nil {
		return m
	}
	m.depth, _ = reg.Register(obscore.Descriptor{Name: "durable_queue_depth", Kind: obscore.Gauge})
	m.submits, _ = reg.Register(obscore.Descriptor{
		Name: "durable_queue_submit_total", Kind: obscore.Counter,
		Labels: []obscore.LabelDomain{kindLabels, resultLabels}})
	m.inflight, _ = reg.Register(obscore.Descriptor{Name: "durable_queue_inflight", Kind: obscore.Gauge})
	m.commitMs, _ = reg.Register(obscore.Descriptor{
		Name: "durable_queue_commit_ms", Kind: obscore.Histogram, Unit: "ms"})
	m.waitMs, _ = reg.Register(obscore.Descriptor{
		Name: "durable_queue_wait_ms", Kind: obscore.Histogram, Unit: "ms"})
	return m
}

func (m *queueMetrics) submit(ctx context.Context, kind, result string) {
	if m == nil || m.submits == nil {
		return
	}
	m.submits.Add(ctx, 1, map[string]string{"kind": kind, "result": result})
}

func (m *queueMetrics) setDepth(ctx context.Context, n int64) {
	if m == nil || m.depth == nil {
		return
	}
	m.depth.Record(ctx, n, nil)
}

func (m *queueMetrics) commit(ctx context.Context, ms int64) {
	if m == nil || m.commitMs == nil {
		return
	}
	m.commitMs.Record(ctx, ms, nil)
}

func (m *queueMetrics) wait(ctx context.Context, ms int64) {
	if m == nil || m.waitMs == nil {
		return
	}
	m.waitMs.Record(ctx, ms, nil)
}

func (q *Queue) emitDepth(ctx context.Context) {
	q.mu.Lock()
	n := int64(len(q.records))
	q.mu.Unlock()
	q.metrics.setDepth(ctx, n)
}
