using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using UnityEditor;
using UnityEditor.SceneManagement;
using UnityEngine;
using UnityEngine.SceneManagement;

namespace ThinhThan.Core.Geometry.Editor
{
    /// <summary>
    /// Collision-scene → canonical <c>.geom.json</c> exporter
    /// (physics_geometry_contract.md §7). Scans the scene convention
    /// (GeometryMeta on `meta`; EdgeCollider2D on `ServerGeometry` objects
    /// named <c>seg.&lt;kind&gt;</c> under `segments`; empty GameObjects
    /// named by anchor id under `anchors`; BoxCollider2D regions under
    /// `camera_regions`), applies §2.5 binary32-rational quantization once
    /// per coordinate, classifies kinds against the §7.3 slope rules, sorts
    /// arrays by id, and writes the schema-v1 document with the exact
    /// canonical byte format the Go parity package reproduces.
    /// </summary>
    public static class GeometryExporter
    {
        /// <summary>Collective result of one export; Json is null when !Ok.</summary>
        public sealed class Result
        {
            public bool Ok;
            public string Json;
            public string SpaceId;
            public readonly List<string> Errors = new List<string>();
        }

        /// <summary>Export the loaded scene at <paramref name="scenePath"/> to canonical JSON.</summary>
        public static Result ExportSceneFile(string scenePath)
        {
            Scene scene = EditorSceneManager.OpenScene(scenePath, OpenSceneMode.Single);
            return ExportScene(scene);
        }

        /// <summary>Export every scene under Assets/Scenes/Collision to maps/.</summary>
        public static List<Result> ExportAll()
        {
            var results = new List<Result>();
            string dir = Path.Combine(Application.dataPath, "Scenes", "Collision");
            foreach (string file in Directory.GetFiles(dir, "*.unity", SearchOption.TopDirectoryOnly))
            {
                string rel = "Assets/Scenes/Collision/" + Path.GetFileName(file);
                Result r = ExportSceneFile(rel);
                if (r.Ok)
                {
                    string outPath = Path.GetFullPath(Path.Combine(
                        Application.dataPath, "..", "..", "server", "internal", "sim", "spatial", "maps",
                        r.SpaceId + ".geom.json"));
                    File.WriteAllText(outPath, r.Json, new UTF8Encoding(false));
                }
                results.Add(r);
            }
            return results;
        }

        /// <summary>Scan a loaded scene and emit canonical JSON.</summary>
        public static Result ExportScene(Scene scene)
        {
            var res = new Result();
            GameObject metaGo = null;
            GameObject segRoot = null;
            GameObject anchRoot = null;
            GameObject regRoot = null;
            foreach (GameObject root in scene.GetRootGameObjects())
            {
                switch (root.name)
                {
                    case "meta": metaGo = root; break;
                    case "segments": segRoot = root; break;
                    case "anchors": anchRoot = root; break;
                    case "camera_regions": regRoot = root; break;
                }
            }
            if (metaGo == null)
            {
                res.Errors.Add("missing root GameObject 'meta'");
                return res;
            }
            if (segRoot == null)
            {
                res.Errors.Add("missing root GameObject 'segments'");
                return res;
            }
            GeometryMeta meta = metaGo.GetComponent<GeometryMeta>();
            if (meta == null)
            {
                res.Errors.Add("'meta' lacks GeometryMeta component");
                return res;
            }
            if (meta.spaceId == "" || meta.spaceKind == "" || meta.layoutProfile == "")
            {
                res.Errors.Add("GeometryMeta missing spaceId/spaceKind/layoutProfile");
                return res;
            }
            res.SpaceId = meta.spaceId;

            var segs = new List<QSegment>();
            foreach (Transform child in segRoot.transform)
            {
                GameObject go = child.gameObject;
                if (!go.CompareTag("ServerGeometry"))
                {
                    res.Errors.Add(go.name + ": segment object not tagged ServerGeometry");
                    continue;
                }
                var ec = go.GetComponent<EdgeCollider2D>();
                if (ec == null)
                {
                    res.Errors.Add(go.name + ": missing EdgeCollider2D");
                    continue;
                }
                if (ec.points.Length != 2)
                {
                    res.Errors.Add(go.name + ": EdgeCollider2D must have exactly 2 points");
                    continue;
                }
                if (child.localRotation != Quaternion.identity || child.localScale != Vector3.one)
                {
                    res.Errors.Add(go.name + ": rotated/scaled segment colliders are not supported");
                    continue;
                }
                string token = go.name.StartsWith("seg.") ? go.name.Substring(4) : "";
                string kindName = KindForToken(token);
                if (kindName == null)
                {
                    res.Errors.Add(go.name + ": unknown kind token in name");
                    continue;
                }
                double w1x;
                double w1y;
                double w2x;
                double w2y;
                WorldPoint(child, ec.offset, ec.points[0], out w1x, out w1y);
                WorldPoint(child, ec.offset, ec.points[1], out w2x, out w2y);
                var q = new QSegment { Kind = kindName };
                q.X1 = Quantize(w1x, res, go.name, "x1");
                q.Y1 = Quantize(w1y, res, go.name, "y1");
                q.X2 = Quantize(w2x, res, go.name, "x2");
                q.Y2 = Quantize(w2y, res, go.name, "y2");
                if (q.X1 > q.X2 || (q.X1 == q.X2 && q.Y1 > q.Y2))
                {
                    long t = q.X1; q.X1 = q.X2; q.X2 = t;
                    t = q.Y1; q.Y1 = q.Y2; q.Y2 = t;
                }
                string slopeErr = CheckKindSlope(kindName, q.X1, q.Y1, q.X2, q.Y2);
                if (slopeErr != null)
                {
                    res.Errors.Add(go.name + ": " + slopeErr);
                    continue;
                }
                segs.Add(q);
            }

            var regs = new List<long[]>();
            if (regRoot != null)
            {
                foreach (Transform child in regRoot.transform)
                {
                    var bc = child.GetComponent<BoxCollider2D>();
                    if (bc == null)
                    {
                        res.Errors.Add(child.name + ": region missing BoxCollider2D");
                        continue;
                    }
                    Vector3 wp = child.position;
                    double cxf = (double)wp.x + (double)bc.offset.x;
                    double cyf = (double)wp.y + (double)bc.offset.y;
                    double sxf = bc.size.x;
                    double syf = bc.size.y;
                    long minX = Quantize(cxf - sxf / 2.0, res, child.name, "minX");
                    long minY = Quantize(cyf - syf / 2.0, res, child.name, "minY");
                    long maxX = Quantize(cxf + sxf / 2.0, res, child.name, "maxX");
                    long maxY = Quantize(cyf + syf / 2.0, res, child.name, "maxY");
                    regs.Add(new[] { minX, minY, maxX, maxY });
                }
            }

            var anchs = new List<QAnchor>();
            if (anchRoot != null)
            {
                foreach (Transform child in anchRoot.transform)
                {
                    Vector3 wp = child.position;
                    anchs.Add(new QAnchor
                    {
                        Id = child.name,
                        X = Quantize(wp.x, res, child.name, "x"),
                        Y = Quantize(wp.y, res, child.name, "y"),
                    });
                }
            }

            if (res.Errors.Count > 0)
            {
                return res;
            }

            long maxX = Quantize(meta.boundsMaxX, res, "meta", "boundsMaxX");
            long maxY = Quantize(meta.boundsMaxY, res, "meta", "boundsMaxY");
            if (res.Errors.Count > 0)
            {
                return res;
            }

            segs.Sort((a, b) => a.X1 != b.X1 ? a.X1.CompareTo(b.X1)
                : a.Y1 != b.Y1 ? a.Y1.CompareTo(b.Y1)
                : a.X2 != b.X2 ? a.X2.CompareTo(b.X2)
                : a.Y2.CompareTo(b.Y2));
            for (int i = 0; i < segs.Count; i++)
            {
                segs[i].Id = i + 1;
            }
            regs.Sort((a, b) => a[0] != b[0] ? a[0].CompareTo(b[0]) : a[1].CompareTo(b[1]));
            anchs.Sort((a, b) => string.CompareOrdinal(a.Id, b.Id));

            string revision = ContentRevision(meta.spaceId, meta.spaceKind, meta.layoutProfile, maxX, maxY, segs, regs, anchs);
            res.Json = Render(meta.spaceId, meta.spaceKind, meta.layoutProfile, revision, maxX, maxY, segs, regs, anchs);
            res.Ok = true;
            return res;
        }

        sealed class QSegment
        {
            public long Id;
            public string Kind;
            public long X1, Y1, X2, Y2;
        }

        sealed class QAnchor
        {
            public string Id;
            public long X, Y;
        }

        // WorldPoint: world position + collider offset + point, computed in
        // float64 exactly like the Go §2.5 pipeline (identical result for
        // binary32 inputs, single rounding at the end).
        static void WorldPoint(Transform t, Vector2 offset, Vector2 local, out double x, out double y)
        {
            Vector3 p = t.position;
            x = (double)p.x + (double)offset.x + (double)local.x;
            y = (double)p.y + (double)offset.y + (double)local.y;
        }

        static string KindForToken(string token)
        {
            string baseTok = token;
            int dot = baseTok.IndexOf('.');
            if (dot >= 0)
            {
                baseTok = baseTok.Substring(0, dot);
            }
            switch (baseTok)
            {
                case "ground": return "SOLID_GROUND";
                case "slope": return "SLOPE";
                case "wall": return "WALL";
                case "ceiling": return "CEILING";
                case "oneway": return "ONE_WAY_PLATFORM";
                default: return null;
            }
        }

        // CheckKindSlope mirrors geometry.checkSlopeRule (§7.3).
        static string CheckKindSlope(string kind, long x1, long y1, long x2, long y2)
        {
            long dx = x2 - x1;
            long dy = y2 - y1;
            long adx = Math.Abs(dx);
            long ady = Math.Abs(dy);
            bool le5 = adx > 0 && ady * 1000000 <= adx * 87489;
            bool le45 = ady <= adx;
            if (x1 == x2)
            {
                if (kind != "WALL" || y1 >= y2)
                {
                    return "vertical segment must be WALL with y1 < y2";
                }
                return null;
            }
            if (x1 >= x2)
            {
                return "non-vertical segment requires x1 < x2";
            }
            switch (kind)
            {
                case "SOLID_GROUND":
                case "ONE_WAY_PLATFORM":
                    return le5 ? null : "kind requires |slope| <= 5 degrees";
                case "SLOPE":
                    return !le5 && le45 ? null : "SLOPE requires 5 < |slope| <= 45 degrees";
                case "WALL":
                    return !le45 ? null : "WALL requires |slope| > 45 degrees";
                case "CEILING":
                    return le45 ? null : "CEILING requires |slope| <= 45 degrees";
            }
            return null;
        }

        // ---- §2.5 binary32-rational quantization ---------------------------

        static long Quantize(double v, Result res, string owner, string field)
        {
            float f = (float)v;
            if (float.IsNaN(f) || float.IsInfinity(f))
            {
                res.Errors.Add(owner + "." + field + ": non-finite coordinate");
                return 0;
            }
            if (Math.Abs(f) > 9.0e15f)
            {
                res.Errors.Add(owner + "." + field + ": coordinate overflows mm range");
                return 0;
            }
            long num;
            long exp2;
            Binary32Rational(f, out num, out exp2);
            if (exp2 >= 0)
            {
                return (num << (int)exp2) * 1000;
            }
            return RoundDivPow2(num * 1000, -exp2);
        }

        static void Binary32Rational(float f, out long num, out long exp2)
        {
            byte[] bytes = BitConverter.GetBytes(f);
            uint bits = BitConverter.ToUInt32(bytes, 0);
            long sign = (bits >> 31) != 0 ? -1 : 1;
            int exp = (int)((bits >> 23) & 0xff);
            long mant = bits & 0x7fffff;
            if (exp == 0)
            {
                num = sign * mant;
                exp2 = -149;
                return;
            }
            num = sign * (mant + (1 << 23));
            exp2 = exp - 150;
        }

        // RoundDivPow2: n / 2^k, halves away from zero (contract §2.2).
        internal static long RoundDivPow2(long n, long k)
        {
            bool neg = n < 0;
            long a = neg ? -n : n;
            long q;
            if (k == 0)
            {
                q = a;
            }
            else if (k >= 64)
            {
                q = 0;
            }
            else
            {
                q = a >> (int)k;
                long r = a - (q << (int)k);
                if (r >= 1L << (int)(k - 1))
                {
                    q++;
                }
            }
            return neg ? -q : q;
        }

        // ---- canonical writer (mirrors parity.renderJSON byte-for-byte) ----

        static string ContentRevision(string spaceId, string kind, string profile, long maxX, long maxY,
            List<QSegment> segs, List<long[]> regs, List<QAnchor> anchs)
        {
            string doc = Render(spaceId, kind, profile, "", maxX, maxY, segs, regs, anchs);
            using (var sha = SHA256.Create())
            {
                byte[] hash = sha.ComputeHash(Encoding.UTF8.GetBytes(doc));
                var b = new StringBuilder(64);
                for (int i = 0; i < hash.Length; i++)
                {
                    b.Append(hash[i].ToString("x2"));
                }
                return b.ToString();
            }
        }

        static string Json(string s)
        {
            var b = new StringBuilder(s.Length + 2);
            b.Append('"');
            foreach (char c in s)
            {
                if (c == '"' || c == '\\')
                {
                    b.Append('\\');
                    b.Append(c);
                }
                else if (c < 0x20)
                {
                    b.Append("\\u");
                    b.Append(((int)c).ToString("x4"));
                }
                else
                {
                    b.Append(c);
                }
            }
            b.Append('"');
            return b.ToString();
        }

        static string Render(string spaceId, string kind, string profile, string rev, long maxX, long maxY,
            List<QSegment> segs, List<long[]> regs, List<QAnchor> anchs)
        {
            var b = new StringBuilder();
            b.Append("{\n");
            b.Append("  \"schema_version\": ").Append(GeometryData.SchemaVersion).Append(",\n");
            b.Append("  \"space_id\": ").Append(Json(spaceId)).Append(",\n");
            b.Append("  \"space_kind\": ").Append(Json(kind)).Append(",\n");
            b.Append("  \"layout_profile\": ").Append(Json(profile)).Append(",\n");
            b.Append("  \"content_revision\": ").Append(Json(rev)).Append(",\n");
            b.Append("  \"bounds_mm\": { \"max_x\": ").Append(maxX).Append(", \"max_y\": ").Append(maxY).Append(" },\n");
            b.Append("  \"segments\": [\n");
            for (int i = 0; i < segs.Count; i++)
            {
                QSegment s = segs[i];
                b.Append("    { \"id\": ").Append(s.Id)
                    .Append(", \"kind\": ").Append(Json(s.Kind))
                    .Append(", \"x1\": ").Append(s.X1)
                    .Append(", \"y1\": ").Append(s.Y1)
                    .Append(", \"x2\": ").Append(s.X2)
                    .Append(", \"y2\": ").Append(s.Y2).Append(" }");
                b.Append(i == segs.Count - 1 ? "\n" : ",\n");
            }
            b.Append("  ],\n");
            b.Append("  \"camera_regions\": [\n");
            for (int i = 0; i < regs.Count; i++)
            {
                long[] r = regs[i];
                b.Append("    { \"id\": ").Append(i + 1)
                    .Append(", \"min_x\": ").Append(r[0])
                    .Append(", \"min_y\": ").Append(r[1])
                    .Append(", \"max_x\": ").Append(r[2])
                    .Append(", \"max_y\": ").Append(r[3]).Append(" }");
                b.Append(i == regs.Count - 1 ? "\n" : ",\n");
            }
            b.Append("  ],\n");
            b.Append("  \"anchors\": [\n");
            for (int i = 0; i < anchs.Count; i++)
            {
                QAnchor a = anchs[i];
                b.Append("    { \"id\": ").Append(Json(a.Id))
                    .Append(", \"x\": ").Append(a.X)
                    .Append(", \"y\": ").Append(a.Y).Append(" }");
                b.Append(i == anchs.Count - 1 ? "\n" : ",\n");
            }
            b.Append("  ]\n");
            b.Append("}\n");
            return b.ToString();
        }
    }
}
