using ThinhThan.Core.Runtime;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// The Interpolation frame phase (client_performance.md § Smoothness by
    /// Construction): recomputes every remote entity's render position once
    /// per frame from the snapshot buffers so presentation reads are O(1).
    /// </summary>
    public sealed class InterpolationSystem : IFrameSystem
    {
        private readonly ReplicationApplier _applier;
        private (double XMm, double YMm)[] _positions =
            new (double, double)[EntityViewStore.InitialCapacity];

        public InterpolationSystem(ReplicationApplier applier)
        {
            _applier = applier;
        }

        /// <summary>Render positions aligned with EntityViewStore.Ids.</summary>
        public (double XMm, double YMm)[] Positions
        {
            get
            {
                return _positions;
            }
        }

        public void Tick(in FrameTime time)
        {
            int count = _applier.Store.Count;
            if (_positions.Length < count)
            {
                int next = _positions.Length;
                while (next < count)
                {
                    next *= 2;
                }

                System.Array.Resize(ref _positions, next);
            }

            for (int i = 0; i < count; i++)
            {
                _positions[i] = _applier.RenderPosition(i);
            }
        }
    }
}
