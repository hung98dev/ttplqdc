package core

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// cloneResourceMetrics deep-copies rm so the queued export owns its
// payload. The SDK pools metricdata between PeriodicReader exports —
// retaining the argument would race the next collect.
func cloneResourceMetrics(rm *metricdata.ResourceMetrics) *metricdata.ResourceMetrics {
	out := &metricdata.ResourceMetrics{
		Resource:     rm.Resource,
		ScopeMetrics: make([]metricdata.ScopeMetrics, len(rm.ScopeMetrics)),
	}
	for i, sm := range rm.ScopeMetrics {
		out.ScopeMetrics[i].Scope = sm.Scope
		out.ScopeMetrics[i].Metrics = cloneMetrics(sm.Metrics)
	}
	return out
}

func cloneMetrics(in []metricdata.Metrics) []metricdata.Metrics {
	out := make([]metricdata.Metrics, len(in))
	for i, m := range in {
		out[i] = metricdata.Metrics{
			Name:        m.Name,
			Description: m.Description,
			Unit:        m.Unit,
			Data:        cloneAggregation(m.Data),
		}
	}
	return out
}

func cloneAggregation(a metricdata.Aggregation) metricdata.Aggregation {
	switch d := a.(type) {
	case metricdata.Sum[int64]:
		d.DataPoints = cloneDataPoints(d.DataPoints)
		return d
	case metricdata.Sum[float64]:
		d.DataPoints = cloneDataPoints(d.DataPoints)
		return d
	case metricdata.Gauge[int64]:
		d.DataPoints = cloneDataPoints(d.DataPoints)
		return d
	case metricdata.Gauge[float64]:
		d.DataPoints = cloneDataPoints(d.DataPoints)
		return d
	case metricdata.Histogram[int64]:
		d.DataPoints = cloneHistogramDataPoints(d.DataPoints)
		return d
	case metricdata.Histogram[float64]:
		d.DataPoints = cloneHistogramDataPoints(d.DataPoints)
		return d
	case metricdata.ExponentialHistogram[int64]:
		d.DataPoints = cloneExponentialHistogramDataPoints(d.DataPoints)
		return d
	case metricdata.ExponentialHistogram[float64]:
		d.DataPoints = cloneExponentialHistogramDataPoints(d.DataPoints)
		return d
	case metricdata.Summary:
		d.DataPoints = cloneSummaryDataPoints(d.DataPoints)
		return d
	default:
		return a
	}
}

func cloneDataPoints[N int64 | float64](in []metricdata.DataPoint[N]) []metricdata.DataPoint[N] {
	out := make([]metricdata.DataPoint[N], len(in))
	for i, p := range in {
		out[i] = p
		out[i].Exemplars = cloneExemplars(p.Exemplars)
	}
	return out
}

func cloneHistogramDataPoints[N int64 | float64](in []metricdata.HistogramDataPoint[N]) []metricdata.HistogramDataPoint[N] {
	out := make([]metricdata.HistogramDataPoint[N], len(in))
	for i, p := range in {
		out[i] = p
		out[i].Bounds = append([]float64{}, p.Bounds...)
		out[i].BucketCounts = append([]uint64{}, p.BucketCounts...)
		out[i].Exemplars = cloneExemplars(p.Exemplars)
	}
	return out
}

func cloneExponentialHistogramDataPoints[N int64 | float64](in []metricdata.ExponentialHistogramDataPoint[N]) []metricdata.ExponentialHistogramDataPoint[N] {
	out := make([]metricdata.ExponentialHistogramDataPoint[N], len(in))
	for i, p := range in {
		out[i] = p
		out[i].PositiveBucket = cloneBuckets(p.PositiveBucket)
		out[i].NegativeBucket = cloneBuckets(p.NegativeBucket)
		out[i].Exemplars = cloneExemplars(p.Exemplars)
	}
	return out
}

func cloneBuckets(b metricdata.ExponentialBucket) metricdata.ExponentialBucket {
	b.Counts = append([]uint64{}, b.Counts...)
	return b
}

func cloneSummaryDataPoints(in []metricdata.SummaryDataPoint) []metricdata.SummaryDataPoint {
	out := make([]metricdata.SummaryDataPoint, len(in))
	for i, p := range in {
		out[i] = p
		out[i].QuantileValues = append([]metricdata.QuantileValue{}, p.QuantileValues...)
	}
	return out
}

func cloneExemplars[N int64 | float64](in []metricdata.Exemplar[N]) []metricdata.Exemplar[N] {
	out := make([]metricdata.Exemplar[N], len(in))
	for i, e := range in {
		out[i] = e
		out[i].FilteredAttributes = append([]attribute.KeyValue{}, e.FilteredAttributes...)
	}
	return out
}
