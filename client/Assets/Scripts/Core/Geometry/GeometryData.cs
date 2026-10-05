using System;

namespace ThinhThan.Core.Geometry
{
    /// <summary>
    /// Schema-v1 geometry document in canonical integer-millimeter form
    /// (physics_geometry_contract.md §7.3). Immutable mirror of a committed
    /// <c>.geom.json</c>: same key order, same array ordering rules, same
    /// value domains as the Go <c>sim/spatial/geometry</c> types. IMP-013
    /// consumes this for client-side prediction; <c>GeometryLoader</c>
    /// produces it.
    /// </summary>
    public sealed class GeometryData
    {
        /// <summary>Segment kind, identical tokens to the Go SegmentKind.</summary>
        public enum SegmentKind
        {
            SolidGround,
            Slope,
            Wall,
            Ceiling,
            OneWayPlatform,
        }

        /// <summary>One collision segment in integer millimeters.</summary>
        public readonly struct Segment
        {
            public readonly long Id;
            public readonly SegmentKind Kind;
            public readonly long X1, Y1, X2, Y2;

            public Segment(long id, SegmentKind kind, long x1, long y1, long x2, long y2)
            {
                Id = id;
                Kind = kind;
                X1 = x1;
                Y1 = y1;
                X2 = x2;
                Y2 = y2;
            }
        }

        /// <summary>Axis-aligned camera region in integer millimeters.</summary>
        public readonly struct CameraRegion
        {
            public readonly long Id;
            public readonly long MinX, MinY, MaxX, MaxY;

            public CameraRegion(long id, long minX, long minY, long maxX, long maxY)
            {
                Id = id;
                MinX = minX;
                MinY = minY;
                MaxX = maxX;
                MaxY = maxY;
            }
        }

        /// <summary>Logical content anchor (feet point) in integer millimeters.</summary>
        public readonly struct Anchor
        {
            public readonly string Id;
            public readonly long X, Y;

            public Anchor(string id, long x, long y)
            {
                Id = id;
                X = x;
                Y = y;
            }
        }

        public const int SchemaVersion = 1;

        public readonly int SchemaVersionValue;
        public readonly string SpaceId;
        public readonly string SpaceKind;
        public readonly string LayoutProfile;
        public readonly string ContentRevision;
        public readonly long BoundsMaxX, BoundsMaxY;
        public readonly Segment[] Segments;
        public readonly CameraRegion[] CameraRegions;
        public readonly Anchor[] Anchors;

        public GeometryData(
            string spaceId,
            string spaceKind,
            string layoutProfile,
            string contentRevision,
            long boundsMaxX,
            long boundsMaxY,
            Segment[] segments,
            CameraRegion[] cameraRegions,
            Anchor[] anchors)
        {
            SchemaVersionValue = SchemaVersion;
            SpaceId = spaceId;
            SpaceKind = spaceKind;
            LayoutProfile = layoutProfile;
            ContentRevision = contentRevision;
            BoundsMaxX = boundsMaxX;
            BoundsMaxY = boundsMaxY;
            Segments = segments;
            CameraRegions = cameraRegions;
            Anchors = anchors;
        }

        /// <summary>
        /// Highest quantized surface height of a non-vertical segment over
        /// the x-interval [xa, xb] (geometry.go surfaceMax). Returns false
        /// when there is no overlap.
        /// </summary>
        public static bool SurfaceMax(Segment s, long xa, long xb, out long top)
        {
            long lo = Math.Max(s.X1, xa);
            long hi = Math.Min(s.X2, xb);
            if (lo > hi)
            {
                top = 0;
                return false;
            }
            long ya = SurfaceHeight(s, lo);
            long yb = SurfaceHeight(s, hi);
            top = Math.Max(ya, yb);
            return true;
        }

        /// <summary>Mirror of surfaceMax for ceilings (lowest height).</summary>
        public static bool SurfaceMin(Segment s, long xa, long xb, out long bottom)
        {
            long lo = Math.Max(s.X1, xa);
            long hi = Math.Min(s.X2, xb);
            if (lo > hi)
            {
                bottom = 0;
                return false;
            }
            long ya = SurfaceHeight(s, lo);
            long yb = SurfaceHeight(s, hi);
            bottom = Math.Min(ya, yb);
            return true;
        }

        /// <summary>
        /// Quantized floor height of a non-vertical segment at x
        /// (contract §2.3): y1 + RoundDiv((x-x1)*(y2-y1), x2-x1).
        /// </summary>
        public static long SurfaceHeight(Segment s, long x)
        {
            return s.Y1 + GeometryMath.RoundDiv((x - s.X1) * (s.Y2 - s.Y1), s.X2 - s.X1);
        }

        /// <summary>Whether the kind forms a surface a character can stand on.</summary>
        public static bool IsWalkable(SegmentKind kind)
        {
            return kind == SegmentKind.SolidGround || kind == SegmentKind.Slope || kind == SegmentKind.OneWayPlatform;
        }

        /// <summary>Parse a wire kind token; false when unknown.</summary>
        public static bool TryParseKind(string s, out SegmentKind kind)
        {
            switch (s)
            {
                case "SOLID_GROUND": kind = SegmentKind.SolidGround; return true;
                case "SLOPE": kind = SegmentKind.Slope; return true;
                case "WALL": kind = SegmentKind.Wall; return true;
                case "CEILING": kind = SegmentKind.Ceiling; return true;
                case "ONE_WAY_PLATFORM": kind = SegmentKind.OneWayPlatform; return true;
                default: kind = SegmentKind.SolidGround; return false;
            }
        }

        /// <summary>Wire token for a kind (geometry.go SegmentKind.String).</summary>
        public static string KindName(SegmentKind kind)
        {
            switch (kind)
            {
                case SegmentKind.SolidGround: return "SOLID_GROUND";
                case SegmentKind.Slope: return "SLOPE";
                case SegmentKind.Wall: return "WALL";
                case SegmentKind.Ceiling: return "CEILING";
                case SegmentKind.OneWayPlatform: return "ONE_WAY_PLATFORM";
                default: throw new ArgumentOutOfRangeException(nameof(kind));
            }
        }
    }
}
