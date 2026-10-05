using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Geometry
{
    /// <summary>
    /// Shared integer-millimeter geometry/movement math — the C# port of
    /// server/internal/sim/spatial/collision (physics_geometry_contract.md
    /// §3-§5). <c>GeometryWorld.ResolveMove</c> mirrors
    /// <c>collision.World.ResolveMove</c> exactly: same phase order
    /// (horizontal sweep then vertical sweep), same 1 mm contact epsilon,
    /// same exact-rational ordering with lowest-segment-id tie-breaks.
    /// IMP-013 prediction consumes this; the committed
    /// <c>sim/spatial/testdata</c> vectors pin Go↔C# byte-identical outputs.
    /// </summary>
    public static class GeometryMath
    {
        /// <summary>
        /// RoundDiv: signed n over positive d, absolute quotient rounded to
        /// nearest, halves away from zero (contract §2.2). Identical to
        /// <c>geometry.RoundDiv</c>; d &lt;= 0 or n == long.MinValue is an
        /// impossible-state violation.
        /// </summary>
        public static long RoundDiv(long n, long d)
        {
            if (d <= 0)
            {
                throw new InvalidOperationException("GeometryMath.RoundDiv non-positive denominator");
            }
            if (n == long.MinValue)
            {
                throw new InvalidOperationException("GeometryMath.RoundDiv INT64_MIN numerator");
            }
            long a = n;
            bool neg = a < 0;
            if (neg)
            {
                a = -a;
            }
            long q = a / d;
            long r = a % d;
            if (r >= d - r)
            {
                q++;
            }
            return neg ? -q : q;
        }

        /// <summary>Axis-aligned box in millimeter coordinates (collision.AABB).</summary>
        public readonly struct Aabb
        {
            public readonly long MinX, MinY, MaxX, MaxY;

            public Aabb(long minX, long minY, long maxX, long maxY)
            {
                MinX = minX;
                MinY = minY;
                MaxX = maxX;
                MaxY = maxY;
            }

            public Aabb Translate(long dx, long dy)
            {
                return new Aabb(MinX + dx, MinY + dy, MaxX + dx, MaxY + dy);
            }
        }

        /// <summary>Per-move call-site parameters (collision.MoveOpts).</summary>
        public readonly struct MoveOpts
        {
            /// <summary>Ledge/slope rise a move may mount in one tick (300mm).</summary>
            public readonly long StepHeightMm;
            /// <summary>ONE_WAY_PLATFORM id being dropped through.</summary>
            public readonly long DropIgnorePlatformId;
            /// <summary>Tick (exclusive) at which the ignored platform resumes.</summary>
            public readonly ulong DropIgnoreUntilTick;
            /// <summary>Authoritative tick of the move.</summary>
            public readonly ulong Tick;

            public MoveOpts(long stepHeightMm, long dropIgnorePlatformId, ulong dropIgnoreUntilTick, ulong tick)
            {
                StepHeightMm = stepHeightMm;
                DropIgnorePlatformId = dropIgnorePlatformId;
                DropIgnoreUntilTick = dropIgnoreUntilTick;
                Tick = tick;
            }
        }

        /// <summary>One resolved surface contact (collision.Contact).</summary>
        public readonly struct Contact
        {
            public readonly long SegmentId;
            public readonly GeometryData.SegmentKind Kind;
            public readonly long X, Y;

            public Contact(long segmentId, GeometryData.SegmentKind kind, long x, long y)
            {
                SegmentId = segmentId;
                Kind = kind;
                X = x;
                Y = y;
            }
        }

        /// <summary>Deterministic outcome of ResolveMove (collision.Result).</summary>
        public sealed class Result
        {
            public Aabb Final;
            public bool VxZeroed;
            public bool VyZeroed;
            public bool Grounded;
            public long GroundSegment;
            public bool OnOneWay;
            public readonly List<Contact> Contacts = new List<Contact>();
            /// <summary>Input state already penetrated blocking geometry.</summary>
            public bool Illegal;
        }

        /// <summary>
        /// Read-only segment index over a validated GeometryData. The Go
        /// implementation builds a uniform grid; results are
        /// iteration-order-independent (every tie-break compares segment
        /// ids), so the port visits all segments in id order and applies the
        /// identical AABB-overlap prefilter.
        /// </summary>
        public sealed class GeometryWorld
        {
            readonly GeometryData g;

            public GeometryWorld(GeometryData geometry)
            {
                g = geometry;
            }

            public GeometryData Geometry
            {
                get { return g; }
            }

            // forEach equivalent: visit every segment whose bounding box
            // overlaps b (inclusive edges), in segment order.
            void ForEach(in Aabb b, Action<GeometryData.Segment> fn)
            {
                for (int i = 0; i < g.Segments.Length; i++)
                {
                    GeometryData.Segment s = g.Segments[i];
                    if (Math.Max(s.X1, s.X2) < b.MinX || Math.Min(s.X1, s.X2) > b.MaxX ||
                        Math.Max(s.Y1, s.Y2) < b.MinY || Math.Min(s.Y1, s.Y2) > b.MaxY)
                    {
                        continue;
                    }
                    fn(s);
                }
            }

            /// <summary>
            /// ResolveMove: horizontal sweep first (walls stop it, ledges
            /// within StepHeightMm are mounted), then the vertical sweep
            /// resolves floor/ceiling contact (contract §4.2).
            /// </summary>
            public Result ResolveMove(Aabb box, long dx, long dy, MoveOpts opts)
            {
                Result res = new Result { Final = box, GroundSegment = -1 };
                Aabb cur = box;

                if (dx != 0)
                {
                    bool stop;
                    cur = SweepX(cur, dx, opts, res, out stop);
                    if (stop)
                    {
                        res.VxZeroed = true;
                    }
                }

                cur = SweepY(cur, dy, opts, res);
                res.Final = cur;

                long gy;
                GeometryData.Segment gseg;
                if (GroundAtInternal(cur, opts.Tick, -1, out gy, out gseg))
                {
                    res.Grounded = true;
                    res.GroundSegment = gseg.Id;
                    res.OnOneWay = gseg.Kind == GeometryData.SegmentKind.OneWayPlatform;
                }
                return res;
            }

            /// <summary>GroundAt: id + surface height of the segment under the feet.</summary>
            public bool GroundAt(Aabb box, ulong tick, long ignoreId, out long y, out long segId)
            {
                GeometryData.Segment s;
                bool ok = GroundAtInternal(box, tick, ignoreId, out y, out s);
                segId = s.Id;
                return ok;
            }

            bool GroundAtInternal(Aabb box, ulong tick, long ignoreId, out long best, out GeometryData.Segment hit)
            {
                best = -1;
                hit = default(GeometryData.Segment);
                ForEach(FootSpan(box), delegate (GeometryData.Segment s)
                {
                    if (!GeometryData.IsWalkable(s.Kind) || s.X1 == s.X2)
                    {
                        return;
                    }
                    if (ignoreId >= 0 && s.Kind == GeometryData.SegmentKind.OneWayPlatform && s.Id == ignoreId)
                    {
                        return;
                    }
                    long top;
                    if (!GeometryData.SurfaceMax(s, box.MinX, box.MaxX, out top))
                    {
                        return;
                    }
                    if (box.MinY < top - 1 || box.MinY > top + 1)
                    {
                        return;
                    }
                    if (top > best || (top == best && s.Id < hit.Id))
                    {
                        best = top;
                        hit = s;
                    }
                });
                return best >= 0;
            }

            static Aabb FootSpan(Aabb b)
            {
                return new Aabb(b.MinX - 1, b.MinY - 1, b.MaxX + 1, b.MaxY + 1);
            }

            /// <summary>Horizontal phase alone (collision.SweepX).</summary>
            public Aabb SweepX(Aabb box, long dx, MoveOpts opts, out bool stop)
            {
                Result res = new Result();
                return SweepX(box, dx, opts, res, out stop);
            }

            /// <summary>Vertical phase alone (collision.SweepY).</summary>
            public Aabb SweepY(Aabb box, long dy, MoveOpts opts)
            {
                Result res = new Result();
                return SweepY(box, dy, opts, res);
            }

            Aabb SweepX(Aabb box, long dx, MoveOpts opts, Result res, out bool stopped)
            {
                Aabb cur = box;
                long remaining = dx;
                for (int i = 0; i < g.Segments.Length + 1 && remaining != 0; i++)
                {
                    long dist;
                    GeometryData.Segment wall;
                    bool hit, pen;
                    XContact(cur, remaining, out dist, out wall, out hit, out pen);
                    if (!hit)
                    {
                        stopped = false;
                        return cur.Translate(remaining, 0);
                    }
                    long move = dist * Sign(remaining);
                    cur = cur.Translate(move, 0);
                    remaining -= move;
                    if (pen)
                    {
                        res.Illegal = true;
                    }
                    long wallTop = Math.Max(wall.Y1, wall.Y2);
                    long step = wallTop - cur.MinY;
                    if (step > 0 && step <= opts.StepHeightMm && HasSupport(cur, wallTop))
                    {
                        cur = new Aabb(cur.MinX, cur.MinY + step, cur.MaxX, cur.MaxY + step);
                        res.Contacts.Add(new Contact(wall.Id, wall.Kind, EdgeX(cur, remaining), wallTop));
                        continue;
                    }
                    res.Contacts.Add(new Contact(wall.Id, wall.Kind, EdgeX(cur, remaining), cur.MinY));
                    stopped = true;
                    return cur;
                }
                if (remaining != 0)
                {
                    cur = cur.Translate(remaining, 0);
                }
                stopped = false;
                return cur;
            }

            void XContact(Aabb box, long dx, out long best, out GeometryData.Segment wall, out bool hit, out bool pen)
            {
                long adir = Math.Abs(dx);
                best = -1;
                wall = default(GeometryData.Segment);
                pen = false;
                Aabb probe = box.Translate(dx, 0);
                Aabb probeBox = new Aabb(
                    Math.Min(box.MinX, probe.MinX), box.MinY - 1,
                    Math.Max(box.MaxX, probe.MaxX), box.MaxY + 1);
                ForEach(probeBox, delegate (GeometryData.Segment s)
                {
                    if (s.Kind != GeometryData.SegmentKind.Wall)
                    {
                        return;
                    }
                    if (box.MinY >= Math.Max(s.Y1, s.Y2) || box.MaxY <= Math.Min(s.Y1, s.Y2))
                    {
                        return;
                    }
                    long xw;
                    if (s.X1 == s.X2)
                    {
                        xw = s.X1;
                    }
                    else
                    {
                        xw = WallXAtBoxSpan(s, box, dx);
                    }
                    long dist = dx > 0 ? xw - box.MaxX : box.MinX - xw;
                    if (dist < 0)
                    {
                        bool inside = xw > box.MinX && xw < box.MaxX;
                        if (inside && (best < 0 || s.Id < wall.Id))
                        {
                            best = 0;
                            wall = s;
                            pen = true;
                        }
                        return;
                    }
                    if (dist > adir)
                    {
                        return;
                    }
                    if (best < 0 || dist < best || (dist == best && s.Id < wall.Id))
                    {
                        best = dist;
                        wall = s;
                        pen = false;
                    }
                });
                hit = best >= 0;
                if (!hit)
                {
                    wall = default(GeometryData.Segment);
                    pen = false;
                }
            }

            static long WallXAtBoxSpan(GeometryData.Segment s, Aabb box, long dx)
            {
                long yLo = Math.Max(Math.Min(s.Y1, s.Y2), box.MinY);
                long yHi = Math.Min(Math.Max(s.Y1, s.Y2), box.MaxY);
                long dy = s.Y2 - s.Y1;
                Func<long, long> xAt = y => s.X1 + RoundDiv((y - s.Y1) * (s.X2 - s.X1), dy);
                if (dx > 0)
                {
                    return Math.Min(xAt(yLo), xAt(yHi));
                }
                return Math.Max(xAt(yLo), xAt(yHi));
            }

            bool HasSupport(Aabb box, long y)
            {
                bool found = false;
                ForEach(box, delegate (GeometryData.Segment s)
                {
                    if (found || !GeometryData.IsWalkable(s.Kind) || s.X1 == s.X2)
                    {
                        return;
                    }
                    long top;
                    if (GeometryData.SurfaceMax(s, box.MinX, box.MaxX, out top) && top >= y - 1 && top <= y + 1)
                    {
                        found = true;
                    }
                });
                return found;
            }

            static long EdgeX(Aabb b, long remaining)
            {
                return remaining > 0 ? b.MaxX : b.MinX;
            }

            Aabb SweepY(Aabb box, long dy, MoveOpts opts, Result res)
            {
                Aabb cur = box;

                long lift;
                GeometryData.Segment lseg;
                if (PenetratingSurface(cur, opts, out lift, out lseg))
                {
                    if (lift - cur.MinY > opts.StepHeightMm && lseg.Kind != GeometryData.SegmentKind.Slope)
                    {
                        res.Illegal = true;
                    }
                    long d = lift - cur.MinY;
                    cur = new Aabb(cur.MinX, cur.MinY + d, cur.MaxX, cur.MaxY + d);
                    res.Contacts.Add(new Contact(lseg.Id, lseg.Kind, cur.MinX, lift));
                }

                if (dy < 0)
                {
                    long fall = -dy;
                    long best = -1;
                    GeometryData.Segment seg = default(GeometryData.Segment);
                    ForEach(new Aabb(cur.MinX, cur.MinY - fall, cur.MaxX, cur.MaxY), delegate (GeometryData.Segment s)
                    {
                        if (!GeometryData.IsWalkable(s.Kind) || s.X1 == s.X2)
                        {
                            return;
                        }
                        if (OneWayIgnored(s, opts))
                        {
                            return;
                        }
                        long top;
                        if (!GeometryData.SurfaceMax(s, cur.MinX, cur.MaxX, out top) || top > cur.MinY + 1 || top < cur.MinY - fall)
                        {
                            return;
                        }
                        if (s.Kind == GeometryData.SegmentKind.OneWayPlatform && cur.MinY < top)
                        {
                            return;
                        }
                        if (top > best || (top == best && s.Id < seg.Id))
                        {
                            best = top;
                            seg = s;
                        }
                    });
                    if (best >= 0)
                    {
                        long d = cur.MinY - best;
                        cur = new Aabb(cur.MinX, cur.MinY - d, cur.MaxX, cur.MaxY - d);
                        res.VyZeroed = true;
                        res.Contacts.Add(new Contact(seg.Id, seg.Kind, cur.MinX, best));
                    }
                    else
                    {
                        cur = cur.Translate(0, -fall);
                    }
                }
                else if (dy > 0)
                {
                    long bestTop = -1;
                    GeometryData.Segment seg = default(GeometryData.Segment);
                    ForEach(new Aabb(cur.MinX, cur.MinY, cur.MaxX, cur.MaxY + dy), delegate (GeometryData.Segment s)
                    {
                        if (s.Kind != GeometryData.SegmentKind.Ceiling || s.X1 == s.X2)
                        {
                            return;
                        }
                        long c;
                        if (!GeometryData.SurfaceMin(s, cur.MinX, cur.MaxX, out c) || c < cur.MaxY || c > cur.MaxY + dy)
                        {
                            return;
                        }
                        if (bestTop < 0 || c < bestTop || (c == bestTop && s.Id < seg.Id))
                        {
                            bestTop = c;
                            seg = s;
                        }
                    });
                    if (bestTop >= 0)
                    {
                        long d = bestTop - cur.MaxY;
                        cur = new Aabb(cur.MinX, cur.MinY + d, cur.MaxX, cur.MaxY + d);
                        res.VyZeroed = true;
                        res.Contacts.Add(new Contact(seg.Id, seg.Kind, cur.MinX, bestTop));
                    }
                    else
                    {
                        cur = cur.Translate(0, dy);
                    }
                }
                return cur;
            }

            bool PenetratingSurface(Aabb box, MoveOpts opts, out long best, out GeometryData.Segment seg)
            {
                best = -1;
                seg = default(GeometryData.Segment);
                ForEach(box, delegate (GeometryData.Segment s)
                {
                    if (!GeometryData.IsWalkable(s.Kind) || s.X1 == s.X2)
                    {
                        return;
                    }
                    if (OneWayIgnored(s, opts))
                    {
                        return;
                    }
                    long top;
                    if (!GeometryData.SurfaceMax(s, box.MinX, box.MaxX, out top) || top <= box.MinY)
                    {
                        return;
                    }
                    if (s.Kind == GeometryData.SegmentKind.OneWayPlatform)
                    {
                        return;
                    }
                    if (best < 0 || top > best || (top == best && s.Id < seg.Id))
                    {
                        best = top;
                        seg = s;
                    }
                });
                return best >= 0;
            }

            static bool OneWayIgnored(GeometryData.Segment s, MoveOpts opts)
            {
                return s.Kind == GeometryData.SegmentKind.OneWayPlatform &&
                    s.Id == opts.DropIgnorePlatformId &&
                    opts.Tick < opts.DropIgnoreUntilTick;
            }

            static long Sign(long v)
            {
                if (v > 0)
                {
                    return 1;
                }
                if (v < 0)
                {
                    return -1;
                }
                return 0;
            }
        }
    }
}
