using System;
using System.Collections.Generic;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Five-second first-launch benchmark (client_performance.md item 1,
    /// PERF-001): samples frame times for five seconds, classifies the
    /// observed p95 into LOW/MEDIUM/HIGH and exposes the result for
    /// persistence. Runs only on first launch — a persisted or
    /// settings-chosen preset always wins.
    /// </summary>
    public sealed class QualityBenchmark
    {
        /// <summary>Session-storage key for the persisted preset.</summary>
        public const string PersistedKey = "quality.preset";

        /// <summary>Benchmark sampling duration (item 1).</summary>
        public const double DurationSeconds = 5.0;

        /// <summary>p95 at or below this selects HIGH (~120 FPS).</summary>
        public const double HighMaxFrameSeconds = 1.0 / 120.0;

        /// <summary>p95 at or below this selects MEDIUM (~60 FPS).</summary>
        public const double MediumMaxFrameSeconds = 1.0 / 60.0;

        private readonly IClock _clock;
        private readonly IFrameTimeSource _frameTime;
        private readonly double _duration;
        private readonly List<double> _samples = new List<double>(1024);
        private double _startedAt;
        private bool _running;

        /// <param name="durationSeconds">Sampling window; 5 s on first launch.</param>
        public QualityBenchmark(
            IClock clock,
            IFrameTimeSource frameTime,
            double durationSeconds = DurationSeconds)
        {
            _clock = clock ?? throw new ArgumentNullException(nameof(clock));
            _frameTime = frameTime ?? throw new ArgumentNullException(nameof(frameTime));
            if (!(durationSeconds > 0.0) || double.IsNaN(durationSeconds))
            {
                throw new ArgumentOutOfRangeException(nameof(durationSeconds));
            }

            _duration = durationSeconds;
            _startedAt = clock.NowSeconds;
            _running = true;
        }

        /// <summary>True after the sampling window completed.</summary>
        public bool Completed
        {
            get
            {
                return !_running;
            }
        }

        /// <summary>Feeds one frame; returns the selected preset when the window closes.</summary>
        public QualityPreset? ObserveFrame()
        {
            if (!_running)
            {
                return null;
            }

            double sample = _frameTime.FrameSeconds;
            if (sample > 0.0 && !double.IsNaN(sample) && !double.IsInfinity(sample))
            {
                _samples.Add(sample);
            }

            if (_clock.NowSeconds - _startedAt < _duration)
            {
                return null;
            }

            _running = false;
            if (_samples.Count == 0)
            {
                return QualityPreset.Medium;
            }

            return Classify(FrameIntervalProbe.Percentile(_samples, 0.95));
        }

        /// <summary>Maps a sampled p95 frame time to a preset (deterministic thresholds).</summary>
        public static QualityPreset Classify(double p95FrameSeconds)
        {
            if (p95FrameSeconds <= HighMaxFrameSeconds)
            {
                return QualityPreset.High;
            }

            if (p95FrameSeconds <= MediumMaxFrameSeconds)
            {
                return QualityPreset.Medium;
            }

            return QualityPreset.Low;
        }

        /// <summary>Reads a persisted preset if one exists.</summary>
        public static bool TryGetPersisted(
            ISessionStorage storage,
            out QualityPreset preset)
        {
            if (storage == null)
            {
                throw new ArgumentNullException(nameof(storage));
            }

            preset = QualityPreset.Medium;
            if (!storage.TryGet(PersistedKey, out string? raw) ||
                !int.TryParse(raw, out int ordinal) ||
                ordinal < 0 ||
                ordinal > (int)QualityPreset.High)
            {
                return false;
            }

            preset = (QualityPreset)ordinal;
            return true;
        }

        /// <summary>Persists a selected preset.</summary>
        public static void Persist(ISessionStorage storage, QualityPreset preset)
        {
            if (storage == null)
            {
                throw new ArgumentNullException(nameof(storage));
            }

            storage.Set(PersistedKey, ((int)preset).ToString());
        }
    }
}
