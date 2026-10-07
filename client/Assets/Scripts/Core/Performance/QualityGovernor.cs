using System;
using System.Collections.Generic;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Adaptive quality governor (client_performance.md item 7, PERF-017).
    /// Sliding 120-frame p95 window over injected frame times; steps down
    /// when p95 exceeds 110% of the tier frame target and steps up after
    /// p95 stays under 75% for 10 continuous seconds. Window resets after
    /// every step and at least 3 s pass between steps. Outputs are
    /// presentation-only: render scale and the particle budget level.
    /// </summary>
    public sealed class QualityGovernor
    {
        public const int WindowFrames = 120;
        public const double StepDownRatio = 1.10;
        public const double StepUpRatio = 0.75;
        public const double StepUpHoldSeconds = 10.0;
        public const double HysteresisSeconds = 3.0;
        public const float RenderScaleStep = 0.05f;
        public const float RenderScaleFloor = 0.6f;
        public const int ParticleLevelFull = 4;
        public const int ParticleLevelMin = 2;

        private const float ScaleEpsilon = 0.001f;

        /// <summary>Presentation-only governor output consumed by <see cref="PresetApplier"/>.</summary>
        public struct State
        {
            public float RenderScale;
            public int ParticleLevel;
            public int ParticleCeiling;
        }

        private readonly double[] _window = new double[WindowFrames];
        private int _windowCount;
        private int _windowIndex;

        private readonly IClock _clock;
        private readonly IFrameTimeSource _frameTime;
        private readonly double _frameTargetSeconds;

        private QualityPreset _preset;
        private float _presetRenderScale;
        private int _presetParticleCeiling;

        private float _renderScale;
        private int _particleLevel;
        private double _lastStepAt;
        private double _lowSince = double.NaN;
        private State _current;

        /// <param name="frameTargetSeconds">
        /// Tier frame target the ratios apply to (e.g. 1/60 for a 60 FPS tier).
        /// </param>
        public QualityGovernor(
            QualityPreset preset,
            double frameTargetSeconds,
            IClock clock,
            IFrameTimeSource frameTime)
        {
            _clock = clock ?? throw new ArgumentNullException(nameof(clock));
            _frameTime = frameTime ?? throw new ArgumentNullException(nameof(frameTime));
            if (!(frameTargetSeconds > 0.0) || double.IsNaN(frameTargetSeconds))
            {
                throw new ArgumentOutOfRangeException(nameof(frameTargetSeconds));
            }

            _frameTargetSeconds = frameTargetSeconds;
            ApplyPreset(preset);
        }

        /// <summary>Latest computed state — presentation-only outputs.</summary>
        public State Current
        {
            get { return _current; }
        }

        /// <summary>Raised whenever a step or preset change updates <see cref="Current"/>.</summary>
        public event Action<State>? Changed;

        /// <summary>
        /// Records one frame sample from the injected
        /// <see cref="IFrameTimeSource"/> and runs the step rules.
        /// </summary>
        public void ObserveFrame()
        {
            double sample = _frameTime.FrameSeconds;
            if (!(sample > 0.0) || double.IsNaN(sample) || double.IsInfinity(sample))
            {
                return;
            }

            _window[_windowIndex] = sample;
            _windowIndex = (_windowIndex + 1) % WindowFrames;
            if (_windowCount < WindowFrames)
            {
                _windowCount++;
            }

            double now = _clock.NowSeconds;
            double p95 = FrameIntervalProbe.Percentile(WindowContents(), 0.95);
            double downAt = _frameTargetSeconds * StepDownRatio;
            double upAt = _frameTargetSeconds * StepUpRatio;

            if (p95 > downAt)
            {
                _lowSince = double.NaN;
                if (now - _lastStepAt >= HysteresisSeconds && TryStepDown())
                {
                    FinishStep(now);
                }

                return;
            }

            if (p95 < upAt)
            {
                if (double.IsNaN(_lowSince))
                {
                    _lowSince = now;
                }
                else if (
                    now - _lowSince >= StepUpHoldSeconds &&
                    now - _lastStepAt >= HysteresisSeconds &&
                    TryStepUp())
                {
                    FinishStep(now);
                }

                return;
            }

            _lowSince = double.NaN;
        }

        /// <summary>
        /// Switches the governed preset: level resets to 100% and the window
        /// clears — a preset switch never preserves a higher old budget.
        /// </summary>
        public void SetPreset(QualityPreset preset)
        {
            ApplyPreset(preset);
        }

        private void ApplyPreset(QualityPreset preset)
        {
            _preset = preset;
            _presetRenderScale = PresetApplier.RenderScaleFor(preset);
            _presetParticleCeiling = PresetApplier.ParticleCeilingFor(preset);
            _renderScale = _presetRenderScale;
            _particleLevel = ParticleLevelFull;
            _windowCount = 0;
            _windowIndex = 0;
            _lowSince = double.NaN;
            _lastStepAt = double.MinValue;
            Publish();
        }

        private bool TryStepDown()
        {
            if (_renderScale > RenderScaleFloor + ScaleEpsilon)
            {
                _renderScale = Mathf.Max(RenderScaleFloor, _renderScale - RenderScaleStep);
                return true;
            }

            if (_particleLevel > ParticleLevelMin)
            {
                _particleLevel--;
                return true;
            }

            return false;
        }

        private bool TryStepUp()
        {
            if (_particleLevel < ParticleLevelFull)
            {
                _particleLevel++;
                return true;
            }

            if (_renderScale < _presetRenderScale - ScaleEpsilon)
            {
                _renderScale = Mathf.Min(_presetRenderScale, _renderScale + RenderScaleStep);
                return true;
            }

            return false;
        }

        private void FinishStep(double now)
        {
            _windowCount = 0;
            _windowIndex = 0;
            _lowSince = double.NaN;
            _lastStepAt = now;
            Publish();
        }

        private void Publish()
        {
            _current = new State
            {
                RenderScale = _renderScale,
                ParticleLevel = _particleLevel,
                ParticleCeiling = _presetParticleCeiling * _particleLevel / ParticleLevelFull,
            };
            Changed?.Invoke(_current);
        }

        private List<double> WindowContents()
        {
            var values = new List<double>(_windowCount);
            for (int i = 0; i < _windowCount; i++)
            {
                values.Add(_window[i]);
            }

            return values;
        }
    }
}
