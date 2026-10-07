using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// PERF-002 interval accounting (client_performance.md § PERF-002):
    /// main-thread gameplay CPU = duration of the frame's single complete
    /// PlayerLoop root minus the union of eligible exclusion intervals,
    /// clipped to the root and merged once. Exclusions are main-thread
    /// descendants only: Gfx.*, Camera.Render, Render.*, WaitForTargetFPS,
    /// and Semaphore.WaitForSignal only when its ancestor chain marks a
    /// rendering wait. A missing root, truncated list, invalid nesting,
    /// negative or non-finite duration, or a union exceeding the frame is
    /// an invalid measurement — it fails, never clamps.
    /// </summary>
    public static class FrameIntervalProbe
    {
        /// <summary>One profiler sample interval on the main thread.</summary>
        public readonly struct Sample
        {
            public Sample(
                int id,
                int parentId,
                string name,
                double startSeconds,
                double durationSeconds)
            {
                Id = id;
                ParentId = parentId;
                Name = name;
                StartSeconds = startSeconds;
                DurationSeconds = durationSeconds;
            }

            public int Id
            {
                get;
            }
            public int ParentId
            {
                get;
            }
            public string Name
            {
                get;
            }
            public double StartSeconds
            {
                get;
            }
            public double DurationSeconds
            {
                get;
            }
            public double EndSeconds
            {
                get
                {
                    return StartSeconds + DurationSeconds;
                }
            }
        }

        /// <summary>
        /// Computes main-thread gameplay CPU for one frame.
        /// <paramref name="samples"/> must contain the frame's complete
        /// sample intervals with parent links (root has ParentId &lt; 0).
        /// Returns false with <paramref name="error"/> on any invalid
        /// measurement; the caller fails the gate instead of clamping.
        /// </summary>
        public static bool TryComputeCpuSeconds(
            IReadOnlyList<Sample> samples,
            out double cpuSeconds,
            out string? error)
        {
            cpuSeconds = 0.0;
            error = null;
            if (samples == null || samples.Count == 0)
            {
                error = "missing PlayerLoop root: empty sample list";
                return false;
            }

            var byId = new Dictionary<int, int>(samples.Count);
            for (int i = 0; i < samples.Count; i++)
            {
                byId[samples[i].Id] = i;
            }

            int rootIndex = -1;
            for (int i = 0; i < samples.Count; i++)
            {
                Sample s = samples[i];
                if (!double.IsFinite(s.StartSeconds) ||
                    !double.IsFinite(s.DurationSeconds))
                {
                    error = "non-finite sample duration";
                    return false;
                }

                if (s.DurationSeconds < 0.0)
                {
                    error = "negative sample duration";
                    return false;
                }

                if (s.ParentId < 0 &&
                    s.Name == PerfMarkers.PlayerLoopMarker)
                {
                    if (rootIndex >= 0)
                    {
                        error = "ambiguous PlayerLoop root";
                        return false;
                    }

                    rootIndex = i;
                }
            }

            if (rootIndex < 0)
            {
                error = "missing PlayerLoop root";
                return false;
            }

            Sample root = samples[rootIndex];
            double pStart = root.StartSeconds;
            double pEnd = root.EndSeconds;
            if (pEnd <= pStart)
            {
                error = "invalid PlayerLoop duration";
                return false;
            }

            var excluded = new List<(double start, double end)>();
            for (int i = 0; i < samples.Count; i++)
            {
                if (i == rootIndex || !IsDescendant(samples, byId, i, rootIndex))
                {
                    continue;
                }

                Sample s = samples[i];
                if (s.StartSeconds < pStart || s.EndSeconds > pEnd)
                {
                    error = "invalid nesting: sample outside PlayerLoop";
                    return false;
                }

                bool eligible = PerfMarkers.IsAlwaysExcludedMarker(s.Name) ||
                    (s.Name == PerfMarkers.SemaphoreWaitMarker &&
                        HasRenderAncestor(samples, byId, i, rootIndex));
                if (eligible && s.DurationSeconds > 0.0)
                {
                    excluded.Add((s.StartSeconds, s.EndSeconds));
                }
            }

            excluded.Sort(
                (a, b) => a.start != b.start
                    ? a.start.CompareTo(b.start)
                    : a.end.CompareTo(b.end));

            double union = 0.0;
            double cursor = double.NegativeInfinity;
            double cursorEnd = double.NegativeInfinity;
            for (int i = 0; i < excluded.Count; i++)
            {
                double start = excluded[i].start;
                double end = excluded[i].end;
                if (start > cursorEnd)
                {
                    if (cursorEnd > double.NegativeInfinity)
                    {
                        union += cursorEnd - cursor;
                    }

                    cursor = start;
                    cursorEnd = end;
                }
                else if (end > cursorEnd)
                {
                    cursorEnd = end;
                }
            }

            if (excluded.Count > 0)
            {
                union += cursorEnd - cursor;
            }

            double frameDuration = pEnd - pStart;
            if (union > frameDuration + 1e-9)
            {
                error = "exclusion union exceeds frame duration";
                return false;
            }

            cpuSeconds = frameDuration - union;
            return true;
        }

        /// <summary>
        /// Nearest-rank percentile over <paramref name="values"/>:
        /// index <c>ceil(q · N) − 1</c> (zero-based) into the sorted copy.
        /// </summary>
        public static double Percentile(IReadOnlyList<double> values, double q)
        {
            if (values == null || values.Count == 0)
            {
                throw new ArgumentException("values must be non-empty", nameof(values));
            }

            if (q <= 0.0 || q > 1.0 || double.IsNaN(q))
            {
                throw new ArgumentOutOfRangeException(nameof(q));
            }

            var sorted = new double[values.Count];
            for (int i = 0; i < values.Count; i++)
            {
                sorted[i] = values[i];
            }

            Array.Sort(sorted);
            int index = (int)Math.Ceiling(q * sorted.Length) - 1;
            return sorted[Math.Max(0, index)];
        }

        /// <summary>Median of a non-empty sample set (timing gates use 3 reps).</summary>
        public static double Median(IReadOnlyList<double> values)
        {
            if (values == null || values.Count == 0)
            {
                throw new ArgumentException("values must be non-empty", nameof(values));
            }

            var sorted = new double[values.Count];
            for (int i = 0; i < values.Count; i++)
            {
                sorted[i] = values[i];
            }

            Array.Sort(sorted);
            int mid = sorted.Length / 2;
            if (sorted.Length % 2 == 1)
            {
                return sorted[mid];
            }

            return (sorted[mid - 1] + sorted[mid]) / 2.0;
        }

        private static bool IsDescendant(
            IReadOnlyList<Sample> samples,
            Dictionary<int, int> byId,
            int index,
            int rootIndex)
        {
            int cursor = index;
            int guard = samples.Count + 1;
            while (guard-- > 0)
            {
                int parentId = samples[cursor].ParentId;
                if (parentId < 0)
                {
                    return false;
                }

                if (!byId.TryGetValue(parentId, out int parentIndex))
                {
                    return false;
                }

                if (parentIndex == rootIndex)
                {
                    return true;
                }

                cursor = parentIndex;
            }

            return false;
        }

        private static bool HasRenderAncestor(
            IReadOnlyList<Sample> samples,
            Dictionary<int, int> byId,
            int index,
            int rootIndex)
        {
            int cursor = index;
            int guard = samples.Count + 1;
            while (guard-- > 0)
            {
                int parentId = samples[cursor].ParentId;
                if (parentId < 0 || !byId.TryGetValue(parentId, out int parentIndex))
                {
                    return false;
                }

                if (parentIndex == rootIndex)
                {
                    return false;
                }

                if (PerfMarkers.IsRenderAncestorMarker(samples[parentIndex].Name))
                {
                    return true;
                }

                cursor = parentIndex;
            }

            return false;
        }
    }
}
