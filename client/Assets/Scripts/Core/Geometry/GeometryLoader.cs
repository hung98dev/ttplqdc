using System;
using System.Collections.Generic;
using System.Text;

namespace ThinhThan.Core.Geometry
{
    /// <summary>
    /// Strict loader for committed <c>.geom.json</c> documents
    /// (physics_geometry_contract.md §7.3). Decodes the schema-v1 object and
    /// enforces every intrinsic rule the Go <c>geometry.Validate</c> applies
    /// that does not need the SpaceRecord: version, bounds positivity,
    /// sorted unique segment ids, kind slope rules, endpoint-in-bounds,
    /// camera-region size/coverage, anchor uniqueness/in-bounds/on-floor.
    /// The record-dependent checks (space_id/kind/profile/bounds/anchor set
    /// equality) belong to the server-side parity suite that produced the
    /// committed file.
    /// </summary>
    public static class GeometryLoader
    {
        /// <summary>Thrown for malformed JSON or schema-v1 violations.</summary>
        public sealed class GeometryParseException : Exception
        {
            public GeometryParseException(string message) : base(message)
            {
            }
        }

        /// <summary>Parse canonical bytes into a validated GeometryData.</summary>
        public static GeometryData Parse(string json)
        {
            object doc = new Reader(json).ReadDocument();
            var root = doc as Dictionary<string, object>;
            if (root == null)
            {
                throw Fail("root object expected");
            }

            long schema = ReqInt(root, "schema_version");
            if (schema != GeometryData.SchemaVersion)
            {
                throw Fail("schema_version: expected 1, got " + schema);
            }
            string spaceId = ReqStr(root, "space_id");
            string kind = ReqStr(root, "space_kind");
            string profile = ReqStr(root, "layout_profile");
            string revision = ReqStr(root, "content_revision");
            var bounds = ReqObj(root, "bounds_mm");
            long maxX = ReqInt(bounds, "max_x");
            long maxY = ReqInt(bounds, "max_y");
            if (maxX <= 0 || maxY <= 0)
            {
                throw Fail("bounds_mm: bounds must be positive");
            }
            if (!IsLowerHex64(revision))
            {
                throw Fail("content_revision: expected 64 lowercase hex chars");
            }

            var segs = new List<GeometryData.Segment>();
            foreach (object e in ReqArr(root, "segments"))
            {
                var o = ReqObj(e, "segments[]");
                long id = ReqInt(o, "id");
                string kt = ReqStr(o, "kind");
                GeometryData.SegmentKind k;
                if (!GeometryData.TryParseKind(kt, out k))
                {
                    throw Fail("segments[].kind: unknown kind " + kt);
                }
                segs.Add(new GeometryData.Segment(id, k, ReqInt(o, "x1"), ReqInt(o, "y1"), ReqInt(o, "x2"), ReqInt(o, "y2")));
            }
            var regs = new List<GeometryData.CameraRegion>();
            foreach (object e in ReqArr(root, "camera_regions"))
            {
                var o = ReqObj(e, "camera_regions[]");
                regs.Add(new GeometryData.CameraRegion(ReqInt(o, "id"), ReqInt(o, "min_x"), ReqInt(o, "min_y"), ReqInt(o, "max_x"), ReqInt(o, "max_y")));
            }
            var anchs = new List<GeometryData.Anchor>();
            foreach (object e in ReqArr(root, "anchors"))
            {
                var o = ReqObj(e, "anchors[]");
                anchs.Add(new GeometryData.Anchor(ReqStr(o, "id"), ReqInt(o, "x"), ReqInt(o, "y")));
            }

            var g = new GeometryData(spaceId, kind, profile, revision, maxX, maxY,
                segs.ToArray(), regs.ToArray(), anchs.ToArray());
            ValidateIntrinsic(g);
            return g;
        }

        // ---- intrinsic validation (mirrors geometry.go) ---------------------

        const long MinCameraRegionXMM = 25600;
        const long MinCameraRegionYMM = 14400;

        static void ValidateIntrinsic(GeometryData g)
        {
            var seen = new HashSet<string>();
            for (int i = 0; i < g.Segments.Length; i++)
            {
                GeometryData.Segment s = g.Segments[i];
                if (i > 0 && g.Segments[i - 1].Id >= s.Id)
                {
                    throw Fail("segments[" + i + "].id: not sorted ascending");
                }
                CheckPoint(g, s.X1, s.Y1, "segments[" + i + "]");
                CheckPoint(g, s.X2, s.Y2, "segments[" + i + "]");
                CheckSlopeRule(s, "segments[" + i + "]");
                string key = GeometryData.KindName(s.Kind) + ":" + s.X1 + "," + s.Y1 + "," + s.X2 + "," + s.Y2;
                if (!seen.Add(key))
                {
                    throw Fail("segments[" + i + "]: duplicate segment");
                }
            }

            if (g.CameraRegions.Length == 0)
            {
                throw Fail("camera_regions: at least one region required");
            }
            var rseen = new HashSet<long>();
            for (int i = 0; i < g.CameraRegions.Length; i++)
            {
                GeometryData.CameraRegion r = g.CameraRegions[i];
                if (!rseen.Add(r.Id))
                {
                    throw Fail("camera_regions[" + i + "].id: duplicate");
                }
                if (r.MinX >= r.MaxX || r.MinY >= r.MaxY)
                {
                    throw Fail("camera_regions[" + i + "]: degenerate region");
                }
                if (r.MinX < 0 || r.MaxX > g.BoundsMaxX || r.MinY < 0 || r.MaxY > g.BoundsMaxY)
                {
                    throw Fail("camera_regions[" + i + "]: outside bounds");
                }
                if (r.MaxX - r.MinX < MinCameraRegionXMM || r.MaxY - r.MinY < MinCameraRegionYMM)
                {
                    throw Fail("camera_regions[" + i + "]: smaller than 25600x14400mm");
                }
            }
            for (int i = 0; i < g.Segments.Length; i++)
            {
                GeometryData.Segment s = g.Segments[i];
                if (GeometryData.IsWalkable(s.Kind) && !CoveredByRegions(s, g.CameraRegions))
                {
                    throw Fail("segments[" + i + "]: walkable segment not covered by camera regions");
                }
            }

            if (g.Anchors.Length == 0)
            {
                throw Fail("anchors: at least one anchor required");
            }
            var aseen = new HashSet<string>();
            for (int i = 0; i < g.Anchors.Length; i++)
            {
                GeometryData.Anchor a = g.Anchors[i];
                if (!aseen.Add(a.Id))
                {
                    throw Fail("anchors[" + i + "].id: duplicate");
                }
                if (a.X < 0 || a.X > g.BoundsMaxX || a.Y < 0 || a.Y > g.BoundsMaxY)
                {
                    throw Fail("anchors[" + i + "]: outside bounds");
                }
                if (!AnchorOnFloor(g, a))
                {
                    throw Fail("anchors[" + i + "]: does not stand on a walkable surface");
                }
            }
        }

        static void CheckPoint(GeometryData g, long x, long y, string path)
        {
            if (x < 0 || x > g.BoundsMaxX || y < 0 || y > g.BoundsMaxY)
            {
                throw Fail(path + ": segment endpoint outside bounds");
            }
        }

        static void CheckSlopeRule(GeometryData.Segment s, string path)
        {
            long dx = s.X2 - s.X1;
            long dy = s.Y2 - s.Y1;
            long adx = Math.Abs(dx);
            long ady = Math.Abs(dy);
            bool le5 = adx > 0 && ady * 1000000 <= adx * 87489;
            bool le45 = ady <= adx;
            if (s.X1 == s.X2)
            {
                if (s.Kind != GeometryData.SegmentKind.Wall || s.Y1 >= s.Y2)
                {
                    throw Fail(path + ": vertical segment must be WALL with y1 < y2");
                }
                return;
            }
            if (s.X1 >= s.X2)
            {
                throw Fail(path + ": non-vertical segment requires x1 < x2");
            }
            switch (s.Kind)
            {
                case GeometryData.SegmentKind.SolidGround:
                case GeometryData.SegmentKind.OneWayPlatform:
                    if (!le5)
                    {
                        throw Fail(path + ": kind requires |slope| <= 5 degrees");
                    }
                    break;
                case GeometryData.SegmentKind.Slope:
                    if (le5 || !le45)
                    {
                        throw Fail(path + ": SLOPE requires 5 < |slope| <= 45 degrees");
                    }
                    break;
                case GeometryData.SegmentKind.Wall:
                    if (le45)
                    {
                        throw Fail(path + ": WALL requires |slope| > 45 degrees");
                    }
                    break;
                case GeometryData.SegmentKind.Ceiling:
                    if (!le45)
                    {
                        throw Fail(path + ": CEILING requires |slope| <= 45 degrees");
                    }
                    break;
            }
        }

        static bool CoveredByRegions(GeometryData.Segment s, GeometryData.CameraRegion[] regions)
        {
            var ivs = new List<long[]>();
            long dx = s.X2 - s.X1;
            for (int i = 0; i < regions.Length; i++)
            {
                GeometryData.CameraRegion r = regions[i];
                long lo = Math.Max(s.X1, r.MinX);
                long hi = Math.Min(s.X2, r.MaxX);
                if (lo > hi)
                {
                    continue;
                }
                long xLo = lo;
                long xHi = hi;
                ClipLinear(s.X1, s.Y1, dx, s.Y2 - s.Y1, ref xLo, ref xHi, r.MinY, true);
                ClipLinear(s.X1, s.Y1, dx, s.Y2 - s.Y1, ref xLo, ref xHi, r.MaxY, false);
                if (xLo <= xHi)
                {
                    ivs.Add(new[] { xLo, xHi });
                }
            }
            if (ivs.Count == 0)
            {
                return false;
            }
            ivs.Sort((a, b) => a[0].CompareTo(b[0]));
            long cur = ivs[0][0];
            if (cur > s.X1)
            {
                return false;
            }
            long maxHi = ivs[0][1];
            for (int i = 1; i < ivs.Count; i++)
            {
                if (ivs[i][0] > maxHi + 1)
                {
                    return false;
                }
                if (ivs[i][1] > maxHi)
                {
                    maxHi = ivs[i][1];
                }
            }
            return maxHi >= s.X2;
        }

        // clipLinear narrows [xLo,xHi] to where y1 + (x-x1)*dy/dx satisfies the
        // bound (needLower: y(x) >= yb; else y(x) <= yb), exact rational only.
        static void ClipLinear(long x1, long y1, long dx, long dy, ref long xLo, ref long xHi, long yb, bool needLower)
        {
            Func<long, long> eval = x => y1 * dx + (x - x1) * dy;
            Func<long, bool> ok = x => needLower ? eval(x) >= yb * dx : eval(x) <= yb * dx;
            bool loOK = ok(xLo);
            bool hiOK = ok(xHi);
            if (!loOK && !hiOK)
            {
                xLo = 1;
                xHi = 0;
                return;
            }
            if (loOK && hiOK)
            {
                return;
            }
            long a = xLo;
            long b = xHi;
            if (!loOK)
            {
                a = xHi;
                b = xLo;
            }
            long lo = Math.Min(a, b);
            long hi = Math.Max(a, b);
            if (a == lo)
            {
                long p = lo;
                long q = hi;
                while (p < q)
                {
                    long m = p + (q - p + 1) / 2;
                    if (ok(m))
                    {
                        p = m;
                    }
                    else
                    {
                        q = m - 1;
                    }
                }
                xLo = lo;
                xHi = p;
                return;
            }
            {
                long p = lo;
                long q = hi;
                while (p < q)
                {
                    long m = p + (q - p) / 2;
                    if (ok(m))
                    {
                        q = m;
                    }
                    else
                    {
                        p = m + 1;
                    }
                }
                xLo = p;
                xHi = hi;
            }
        }

        static bool AnchorOnFloor(GeometryData g, GeometryData.Anchor a)
        {
            for (int i = 0; i < g.Segments.Length; i++)
            {
                GeometryData.Segment s = g.Segments[i];
                if (!GeometryData.IsWalkable(s.Kind) || s.X1 == s.X2)
                {
                    continue;
                }
                if (a.X < s.X1 || a.X > s.X2)
                {
                    continue;
                }
                if (GeometryData.SurfaceHeight(s, a.X) == a.Y)
                {
                    return true;
                }
            }
            return false;
        }

        static bool IsLowerHex64(string s)
        {
            if (s.Length != 64)
            {
                return false;
            }
            for (int i = 0; i < s.Length; i++)
            {
                char c = s[i];
                if (!(c >= '0' && c <= '9' || c >= 'a' && c <= 'f'))
                {
                    return false;
                }
            }
            return true;
        }

        // ---- strict JSON reader --------------------------------------------

        static GeometryParseException Fail(string msg)
        {
            return new GeometryParseException("geometry: " + msg);
        }

        static string ReqStr(Dictionary<string, object> o, string k)
        {
            object v;
            if (!o.TryGetValue(k, out v) || !(v is string))
            {
                throw Fail(k + ": missing or not a string");
            }
            return (string)v;
        }

        static long ReqInt(Dictionary<string, object> o, string k)
        {
            object v;
            if (!o.TryGetValue(k, out v) || !(v is long))
            {
                throw Fail(k + ": missing or not an integer");
            }
            return (long)v;
        }

        static Dictionary<string, object> ReqObj(Dictionary<string, object> o, string k)
        {
            object v;
            if (!o.TryGetValue(k, out v))
            {
                throw Fail(k + ": missing");
            }
            return ReqObj(v, k);
        }

        static Dictionary<string, object> ReqObj(object v, string path)
        {
            var o = v as Dictionary<string, object>;
            if (o == null)
            {
                throw Fail(path + ": expected object");
            }
            return o;
        }

        static List<object> ReqArr(Dictionary<string, object> o, string k)
        {
            object v;
            if (!o.TryGetValue(k, out v) || !(v is List<object>))
            {
                throw Fail(k + ": missing or not an array");
            }
            return (List<object>)v;
        }

        // Minimal recursive-descent JSON reader: numbers must be integers,
        // duplicate keys and trailing input are rejected.
        sealed class Reader
        {
            readonly string s;
            int p;

            public Reader(string text)
            {
                s = text;
            }

            public object ReadDocument()
            {
                SkipWs();
                object v = Value();
                SkipWs();
                if (p != s.Length)
                {
                    throw Fail("trailing input at offset " + p);
                }
                return v;
            }

            void SkipWs()
            {
                while (p < s.Length && (s[p] == ' ' || s[p] == '\t' || s[p] == '\n' || s[p] == '\r'))
                {
                    p++;
                }
            }

            char Peek()
            {
                if (p >= s.Length)
                {
                    throw Fail("unexpected end of input");
                }
                return s[p];
            }

            object Value()
            {
                SkipWs();
                char c = Peek();
                if (c == '{')
                {
                    return Obj();
                }
                if (c == '[')
                {
                    return Arr();
                }
                if (c == '"')
                {
                    return Str();
                }
                if (c == 't' || c == 'f')
                {
                    return Bool();
                }
                if (c == 'n')
                {
                    return Null();
                }
                return Num();
            }

            Dictionary<string, object> Obj()
            {
                var o = new Dictionary<string, object>();
                p++;
                SkipWs();
                if (Peek() == '}')
                {
                    p++;
                    return o;
                }
                while (true)
                {
                    SkipWs();
                    string k = Str();
                    SkipWs();
                    if (Peek() != ':')
                    {
                        throw Fail("expected ':' at offset " + p);
                    }
                    p++;
                    object v = Value();
                    if (o.ContainsKey(k))
                    {
                        throw Fail("duplicate key " + k);
                    }
                    o[k] = v;
                    SkipWs();
                    char c = Peek();
                    if (c == '}')
                    {
                        p++;
                        return o;
                    }
                    if (c != ',')
                    {
                        throw Fail("expected ',' at offset " + p);
                    }
                    p++;
                }
            }

            List<object> Arr()
            {
                var l = new List<object>();
                p++;
                SkipWs();
                if (Peek() == ']')
                {
                    p++;
                    return l;
                }
                while (true)
                {
                    l.Add(Value());
                    SkipWs();
                    char c = Peek();
                    if (c == ']')
                    {
                        p++;
                        return l;
                    }
                    if (c != ',')
                    {
                        throw Fail("expected ',' at offset " + p);
                    }
                    p++;
                }
            }

            string Str()
            {
                if (Peek() != '"')
                {
                    throw Fail("expected string at offset " + p);
                }
                p++;
                var b = new StringBuilder();
                while (true)
                {
                    if (p >= s.Length)
                    {
                        throw Fail("unterminated string");
                    }
                    char c = s[p++];
                    if (c == '"')
                    {
                        return b.ToString();
                    }
                    if (c == '\\')
                    {
                        if (p >= s.Length)
                        {
                            throw Fail("unterminated escape");
                        }
                        char e = s[p++];
                        switch (e)
                        {
                            case '"': b.Append('"'); break;
                            case '\\': b.Append('\\'); break;
                            case '/': b.Append('/'); break;
                            case 'n': b.Append('\n'); break;
                            case 't': b.Append('\t'); break;
                            case 'r': b.Append('\r'); break;
                            case 'b': b.Append('\b'); break;
                            case 'f': b.Append('\f'); break;
                            case 'u':
                                if (p + 4 > s.Length)
                                {
                                    throw Fail("bad \\u escape");
                                }
                                b.Append((char)Convert.ToInt32(s.Substring(p, 4), 16));
                                p += 4;
                                break;
                            default:
                                throw Fail("bad escape \\" + e);
                        }
                    }
                    else
                    {
                        b.Append(c);
                    }
                }
            }

            object Num()
            {
                int start = p;
                if (Peek() == '-')
                {
                    p++;
                }
                while (p < s.Length && s[p] >= '0' && s[p] <= '9')
                {
                    p++;
                }
                if (p < s.Length && (s[p] == '.' || s[p] == 'e' || s[p] == 'E' || s[p] == '+' || s[p] == '-'))
                {
                    throw Fail("non-integer number at offset " + start);
                }
                if (p == start || (p == start + 1 && s[start] == '-'))
                {
                    throw Fail("expected value at offset " + p);
                }
                long v;
                if (!long.TryParse(s.Substring(start, p - start), out v))
                {
                    throw Fail("bad integer at offset " + start);
                }
                return v;
            }

            object Bool()
            {
                if (s.Substring(p).StartsWith("true"))
                {
                    p += 4;
                    return true;
                }
                if (s.Substring(p).StartsWith("false"))
                {
                    p += 5;
                    return false;
                }
                throw Fail("bad literal at offset " + p);
            }

            object Null()
            {
                if (s.Substring(p).StartsWith("null"))
                {
                    p += 4;
                    return null;
                }
                throw Fail("bad literal at offset " + p);
            }
        }
    }
}
