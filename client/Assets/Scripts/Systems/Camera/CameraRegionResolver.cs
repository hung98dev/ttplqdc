using ThinhThan.Core.Geometry;

namespace ThinhThan.Systems.Camera
{
    /// <summary>
    /// Active camera region (physics.md §6.2): the region containing the
    /// predicted anchor wins — smallest region id on overlap; when the
    /// anchor sits in no region the previous region is kept. The view
    /// rect clamps inside the active region, axis-centred when the view
    /// exceeds the region on that axis.
    /// </summary>
    public sealed class CameraRegionResolver
    {
        private readonly GeometryData.CameraRegion[] _regions;
        private int _activeIndex = -1;

        public CameraRegionResolver(GeometryData? geometry)
        {
            _regions = geometry?.CameraRegions ??
                System.Array.Empty<GeometryData.CameraRegion>();
        }

        /// <summary>Currently active region, or null when none applies.</summary>
        public GeometryData.CameraRegion? ActiveRegion
        {
            get
            {
                return _activeIndex >= 0 ? _regions[_activeIndex] : null;
            }
        }

        /// <summary>
        /// Resolves the active region for the anchor point; keeps the
        /// previous region when the anchor leaves every region.
        /// </summary>
        public GeometryData.CameraRegion? Resolve(
            long anchorXMm, long anchorYMm)
        {
            int best = -1;
            long bestId = long.MaxValue;
            for (int i = 0; i < _regions.Length; i++)
            {
                GeometryData.CameraRegion r = _regions[i];
                if (anchorXMm >= r.MinX && anchorXMm <= r.MaxX &&
                    anchorYMm >= r.MinY && anchorYMm <= r.MaxY &&
                    r.Id < bestId)
                {
                    best = i;
                    bestId = r.Id;
                }
            }

            if (best >= 0)
            {
                _activeIndex = best;
            }

            return ActiveRegion;
        }

        /// <summary>
        /// Clamps a desired camera centre (integer mm) inside the active
        /// region given the view half-extents; unclamped when no region.
        /// </summary>
        public void ClampCenter(
            long desiredXMm, long desiredYMm,
            long viewHalfWMm, long viewHalfHMm,
            out long centerXMm, out long centerYMm)
        {
            GeometryData.CameraRegion? region = ActiveRegion;
            if (region == null)
            {
                centerXMm = desiredXMm;
                centerYMm = desiredYMm;
                return;
            }

            GeometryData.CameraRegion r = region.Value;
            centerXMm = ClampAxis(
                desiredXMm, r.MinX, r.MaxX, viewHalfWMm);
            centerYMm = ClampAxis(
                desiredYMm, r.MinY, r.MaxY, viewHalfHMm);
        }

        private static long ClampAxis(
            long desired, long min, long max, long halfView)
        {
            if (halfView * 2L >= max - min)
            {
                return (min + max) / 2L;
            }

            if (desired < min + halfView)
            {
                return min + halfView;
            }

            if (desired > max - halfView)
            {
                return max - halfView;
            }

            return desired;
        }

        /// <summary>Clears the kept region (new space / baseline).</summary>
        public void Reset()
        {
            _activeIndex = -1;
        }
    }
}
