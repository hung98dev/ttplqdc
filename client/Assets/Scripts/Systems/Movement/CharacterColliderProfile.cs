using ThinhThan.Core.Geometry;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// CHARACTER collider profile: 800 mm wide × 1800 mm tall, anchored at
    /// mid-feet (contract §3, ADR-0046). The box is scale- and
    /// sprite-independent.
    /// </summary>
    public static class CharacterColliderProfile
    {
        public const long HalfWidthMm = 400;
        public const long HeightMm = 1800;

        /// <summary>Builds the mid-feet-anchored AABB around feet (x, y).</summary>
        public static GeometryMath.Aabb FeetBox(long xMm, long yMm)
        {
            return new GeometryMath.Aabb(
                xMm - HalfWidthMm,
                yMm,
                xMm + HalfWidthMm,
                yMm + HeightMm);
        }
    }
}
