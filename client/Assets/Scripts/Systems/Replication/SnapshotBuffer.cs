using System;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// Per-entity interpolation buffer (client_performance.md § Network
    /// Smoothness): remote entities render at now minus the interpolation
    /// delay (200 ms nominal, adaptive 150-300 ms by jitter); once the
    /// newest sample is older than the extrapolation limit (250 ms) the
    /// entity freezes — it never dead-reckons indefinitely.
    /// </summary>
    public sealed class SnapshotBuffer
    {
        public const double BaseDelaySeconds = 0.2;

        public const double MinDelaySeconds = 0.15;

        public const double MaxDelaySeconds = 0.3;

        public const double ExtrapolationLimitSeconds = 0.25;

        private const int Capacity = 8;

        private readonly Sample[] _samples = new Sample[Capacity];
        private int _count;
        private int _next;
        private double _delaySeconds = BaseDelaySeconds;
        private double _lastArrivalSeconds;
        private double _emaGapSeconds;
        private double _emaJitterSeconds;

        /// <summary>Adaptive interpolation delay currently in force.</summary>
        public double DelaySeconds
        {
            get
            {
                return _delaySeconds;
            }
        }

        /// <summary>Adds a snapshot observed at <paramref name="nowSeconds"/>.</summary>
        public void Push(
            ulong serverTick, int xMm, int yMm, int vxMmS, int vyMmS,
            double nowSeconds)
        {
            if (_count > 0)
            {
                double gap = nowSeconds - _lastArrivalSeconds;
                double jitter = Math.Abs(
                    gap - (_emaGapSeconds > 0 ? _emaGapSeconds : gap));
                _emaGapSeconds = _emaGapSeconds <= 0
                    ? gap
                    : _emaGapSeconds * 0.875 + gap * 0.125;
                _emaJitterSeconds = _emaJitterSeconds <= 0
                    ? jitter
                    : _emaJitterSeconds * 0.875 + jitter * 0.125;
                double adaptive = BaseDelaySeconds + _emaJitterSeconds;
                _delaySeconds = adaptive < MinDelaySeconds
                    ? MinDelaySeconds
                    : adaptive > MaxDelaySeconds
                        ? MaxDelaySeconds
                        : adaptive;
            }

            _lastArrivalSeconds = nowSeconds;
            _samples[_next] = new Sample(
                serverTick, xMm, yMm, vxMmS, vyMmS, nowSeconds);
            _next = (_next + 1) % Capacity;
            if (_count < Capacity)
            {
                _count++;
            }
        }

        /// <summary>
        /// Interpolated render position at
        /// <paramref name="nowSeconds"/> - delay: lerps between the two
        /// samples bracketing the render time, extrapolates by velocity for
        /// at most <see cref="ExtrapolationLimitSeconds"/> past the newest
        /// sample, then freezes.
        /// </summary>
        public (double XMm, double YMm) Sample(double nowSeconds)
        {
            if (_count == 0)
            {
                return (0.0, 0.0);
            }

            int newest = NewestIndex();
            double renderTime = nowSeconds - _delaySeconds;
            if (renderTime >= _samples[newest].ReceivedSeconds)
            {
                double overshoot = renderTime - _samples[newest].ReceivedSeconds;
                if (overshoot > ExtrapolationLimitSeconds)
                {
                    overshoot = ExtrapolationLimitSeconds;
                }

                return (
                    _samples[newest].XMm +
                        _samples[newest].VxMmS * overshoot,
                    _samples[newest].YMm +
                        _samples[newest].VyMmS * overshoot);
            }

            for (int i = _count - 1; i > 0; i--)
            {
                int newer = ToIndex(i);
                int older = ToIndex(i - 1);
                if (_samples[older].ReceivedSeconds <= renderTime)
                {
                    double span =
                        _samples[newer].ReceivedSeconds -
                        _samples[older].ReceivedSeconds;
                    double t = span <= 0
                        ? 0
                        : (renderTime - _samples[older].ReceivedSeconds) / span;
                    if (t > 1)
                    {
                        t = 1;
                    }

                    return (
                        _samples[older].XMm +
                            (_samples[newer].XMm - _samples[older].XMm) * t,
                        _samples[older].YMm +
                            (_samples[newer].YMm - _samples[older].YMm) * t);
                }
            }

            int oldest = ToIndex(0);
            return (_samples[oldest].XMm, _samples[oldest].YMm);
        }

        public void Clear()
        {
            _count = 0;
            _next = 0;
            _delaySeconds = BaseDelaySeconds;
            _emaGapSeconds = 0;
            _emaJitterSeconds = 0;
            _lastArrivalSeconds = 0;
        }

        private int NewestIndex()
        {
            return (_next + Capacity - 1) % Capacity;
        }

        /// <summary>Index of the i-th oldest live sample (0.._count-1).</summary>
        private int ToIndex(int i)
        {
            return (_next + Capacity - _count + i) % Capacity;
        }

        private readonly struct Sample
        {
            public Sample(
                ulong serverTick, int xMm, int yMm, int vxMmS, int vyMmS,
                double receivedSeconds)
            {
                ServerTick = serverTick;
                XMm = xMm;
                YMm = yMm;
                VxMmS = vxMmS;
                VyMmS = vyMmS;
                ReceivedSeconds = receivedSeconds;
            }

            public ulong ServerTick
            {
                get;
            }

            public int XMm
            {
                get;
            }

            public int YMm
            {
                get;
            }

            public int VxMmS
            {
                get;
            }

            public int VyMmS
            {
                get;
            }

            public double ReceivedSeconds
            {
                get;
            }
        }
    }
}
