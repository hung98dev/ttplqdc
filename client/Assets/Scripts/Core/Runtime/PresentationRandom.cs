using System;

namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// Seeded presentation-only PRNG (xorshift64*) — the single allowed random
    /// source for client visuals (engineering_conventions.md §2.5 bans
    /// UnityEngine.Random and System.Random). Deterministic for a given seed;
    /// never used for gameplay authority.
    /// </summary>
    public sealed class PresentationRandom
    {
        private ulong _state;

        public PresentationRandom(ulong seed)
        {
            _state = seed == 0UL ? 0x9E3779B97F4A7C15UL : seed;
        }

        /// <summary>Next value in [0, 1).</summary>
        public float NextFloat()
        {
            return (NextBits() >> 40) * (1.0f / 16777216.0f);
        }

        /// <summary>Next value in [0, maxExclusive).</summary>
        public int NextInt(int maxExclusive)
        {
            if (maxExclusive <= 0)
            {
                throw new ArgumentOutOfRangeException(nameof(maxExclusive));
            }

            return (int)(NextBits() % (ulong)maxExclusive);
        }

        /// <summary>Next value in [minInclusive, maxExclusive).</summary>
        public int NextRange(int minInclusive, int maxExclusive)
        {
            return minInclusive + NextInt(maxExclusive - minInclusive);
        }

        /// <summary>Next value in [0, maxExclusive).</summary>
        public float NextFloat(float maxExclusive)
        {
            return NextFloat() * maxExclusive;
        }

        private ulong NextBits()
        {
            _state ^= _state >> 12;
            _state ^= _state << 25;
            _state ^= _state >> 27;
            return _state * 0x2545F4914F6CDD1DUL;
        }
    }
}
