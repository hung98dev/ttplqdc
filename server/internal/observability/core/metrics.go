package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// otherValue bounds recorded label cardinality: values outside a
// declared domain are folded into this bucket instead of creating new
// time series.
const otherValue = "__other__"

// InstrumentKind selects the OTel instrument family a Descriptor
// produces.
type InstrumentKind int

const (
	// Counter is a monotonically increasing Int64Counter.
	Counter InstrumentKind = iota
	// Histogram is an Int64Histogram.
	Histogram
	// Gauge is an Int64Gauge recording the last observed value.
	Gauge
)

// LabelDomain declares the finite domain of one label key. A label is
// bounded either by an explicit value list (Values) or by a distinct-
// value cap (MaxDistinct); at least one must be set or registration
// fails — metric registration rejects unbounded label sets.
type LabelDomain struct {
	Key         string
	Values      []string
	MaxDistinct int
}

// Descriptor describes one metric to register.
type Descriptor struct {
	Name   string
	Kind   InstrumentKind
	Unit   string
	Labels []LabelDomain
}

// Registry is the bounded-cardinality metric registry
// (observability.md metric classes; packet contract_outputs). It owns
// one OTel meter and validates every descriptor's label domains before
// an instrument exists.
type Registry struct {
	meter otelmetric.Meter
}

// NewRegistry returns a Registry recording through mp's "thinhthan"
// meter. mp may be nil for logging-only callers; registration then
// still validates domains but emits nowhere.
func NewRegistry(mp *sdkmetric.MeterProvider) *Registry {
	if mp == nil {
		mp = sdkmetric.NewMeterProvider()
	}
	return &Registry{meter: mp.Meter("thinhthan")}
}

// Register validates d and creates the backing OTel instrument. It
// returns an error — and creates nothing — when the name is empty or
// any label lacks a finite declared domain.
func (r *Registry) Register(d Descriptor) (*Instrument, error) {
	if d.Name == "" {
		return nil, errors.New("core: metric name required")
	}
	domains := make(map[string]LabelDomain, len(d.Labels))
	for _, l := range d.Labels {
		if l.Key == "" {
			return nil, fmt.Errorf("core: metric %q: empty label key", d.Name)
		}
		if len(l.Values) == 0 && l.MaxDistinct <= 0 {
			return nil, fmt.Errorf("core: metric %q: label %q has unbounded cardinality", d.Name, l.Key)
		}
		domains[l.Key] = l
	}
	inst := &Instrument{domains: domains, seen: make(map[string]map[string]struct{})}
	var err error
	switch d.Kind {
	case Counter:
		opts := []otelmetric.Int64CounterOption{}
		if d.Unit != "" {
			opts = append(opts, otelmetric.WithUnit(d.Unit))
		}
		var c otelmetric.Int64Counter
		if c, err = r.meter.Int64Counter(d.Name, opts...); err == nil {
			inst.add = c.Add
		}
	case Histogram:
		opts := []otelmetric.Int64HistogramOption{}
		if d.Unit != "" {
			opts = append(opts, otelmetric.WithUnit(d.Unit))
		}
		var h otelmetric.Int64Histogram
		if h, err = r.meter.Int64Histogram(d.Name, opts...); err == nil {
			inst.record = h.Record
		}
	case Gauge:
		opts := []otelmetric.Int64GaugeOption{}
		if d.Unit != "" {
			opts = append(opts, otelmetric.WithUnit(d.Unit))
		}
		var g otelmetric.Int64Gauge
		if g, err = r.meter.Int64Gauge(d.Name, opts...); err == nil {
			inst.record = g.Record
		}
	default:
		return nil, fmt.Errorf("core: metric %q: unknown kind %d", d.Name, d.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("core: metric %q: %w", d.Name, err)
	}
	return inst, nil
}

// Instrument records values against its declared label domains. Emit
// never blocks and never returns an error to the caller: an undeclared
// label key drops the record, and out-of-domain values fold into
// "__other__".
type Instrument struct {
	domains map[string]LabelDomain
	add     func(context.Context, int64, ...otelmetric.AddOption)
	record  func(context.Context, int64, ...otelmetric.RecordOption)
	mu      sync.Mutex
	seen    map[string]map[string]struct{}
}

// Add increments a Counter instrument by v.
func (i *Instrument) Add(ctx context.Context, v int64, labels map[string]string) {
	attrs, ok := i.bind(labels)
	if !ok || i.add == nil {
		return
	}
	i.add(ctx, v, otelmetric.WithAttributes(attrs...))
}

// Record observes v on a Histogram or Gauge instrument.
func (i *Instrument) Record(ctx context.Context, v int64, labels map[string]string) {
	attrs, ok := i.bind(labels)
	if !ok || i.record == nil {
		return
	}
	i.record(ctx, v, otelmetric.WithAttributes(attrs...))
}

// bind maps caller labels onto declared domains, sorted for
// determinism. ok is false when a label key was never declared.
func (i *Instrument) bind(labels map[string]string) ([]attribute.KeyValue, bool) {
	if len(labels) == 0 {
		return nil, true
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		if _, ok := i.domains[k]; !ok {
			return nil, false
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attrs := make([]attribute.KeyValue, 0, len(keys))
	for _, k := range keys {
		attrs = append(attrs, attribute.String(k, i.bindValue(k, labels[k], i.domains[k])))
	}
	return attrs, true
}

// bindValue enforces one label's finite domain under i's lock.
func (i *Instrument) bindValue(key, v string, d LabelDomain) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(d.Values) > 0 {
		for _, allowed := range d.Values {
			if allowed == v {
				return v
			}
		}
		return otherValue
	}
	set, ok := i.seen[key]
	if !ok {
		set = make(map[string]struct{})
		i.seen[key] = set
	}
	if _, ok := set[v]; ok {
		return v
	}
	if len(set) >= d.MaxDistinct {
		return otherValue
	}
	set[v] = struct{}{}
	return v
}
