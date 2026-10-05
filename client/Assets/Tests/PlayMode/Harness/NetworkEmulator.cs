using System;
using System.Threading;
using System.Threading.Tasks;
using ThinhThan.Core.Runtime;

namespace ThinhThan.Tests.PlayMode.Harness
{
    /// <summary>
    /// Seeded outbound-shaping for the fake server (packet § Harness):
    /// base latency + jitter + drop probability applied to every S2C send so
    /// scenario tests run on a deterministic latency path. Both endpoints
    /// are loopback, so shaping only ever delays server-to-client frames.
    /// </summary>
    public sealed class NetworkEmulator
    {
        private readonly PresentationRandom _random;
        private int _latencyMs;
        private int _jitterMs;
        private double _dropProbability;

        public NetworkEmulator(ulong seed)
        {
            _random = new PresentationRandom(seed);
        }

        /// <summary>Constant one-way delay applied before each send.</summary>
        public int LatencyMs
        {
            get
            {
                return _latencyMs;
            }
            set
            {
                _latencyMs = Math.Max(0, value);
            }
        }

        /// <summary>Uniform extra delay 0..JitterMs-1.</summary>
        public int JitterMs
        {
            get
            {
                return _jitterMs;
            }
            set
            {
                _jitterMs = Math.Max(0, value);
            }
        }

        /// <summary>Probability 0..1 that a given S2C send is dropped.</summary>
        public double DropProbability
        {
            get
            {
                return _dropProbability;
            }
            set
            {
                _dropProbability = value < 0 ? 0 : value > 1 ? 1 : value;
            }
        }

        /// <summary>True when the next send should be skipped.</summary>
        public bool ShouldDrop()
        {
            return _dropProbability > 0 && _random.NextFloat() < _dropProbability;
        }

        /// <summary>Delay for the next send (latency + jitter).</summary>
        public int NextDelayMs()
        {
            return _latencyMs +
                (_jitterMs > 0 ? _random.NextInt(_jitterMs) : 0);
        }

        /// <summary>Delay + drop decision in one call.</summary>
        public async Task<bool> PassAsync(CancellationToken cancel)
        {
            if (ShouldDrop())
            {
                return false;
            }

            int delay = NextDelayMs();
            if (delay > 0)
            {
                await Task.Delay(delay, cancel).ConfigureAwait(false);
            }

            return true;
        }
    }
}
