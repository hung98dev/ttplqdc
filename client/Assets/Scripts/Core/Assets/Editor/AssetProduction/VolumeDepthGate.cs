using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Volume &amp; Depth Gate (presentation_asset_manifest.md section 3.6).
    /// All measures run on the final 2x texture in silhouette S = pixels with
    /// alpha &gt;= 128 (declared translucent-mask pixels are excluded from S).
    /// One luminance measure everywhere: CIELAB L*. Deterministic numeric
    /// completion per spec: top-left origin, row-major ties, binary64,
    /// nearest-rank quantiles, seeded Lloyd k-means with frozen empty
    /// clusters and the DEEPEST core-ring fallback.
    /// </summary>
    public static class VolumeDepthGate
    {
        public const double ValueRangeMin = 40.0;
        public const double TopLightMin = 4.0;
        public const double RimShareMin = 0.60;
        public const double RimDeltaLMin = 12.0;
        public const double FlatRegionMaxShare = 0.20;
        public const double ActorBgDeltaLMin = 20.0;
        public const double PaletteDeltaE00Max = 8.0;
        public const double PaletteCoverageMin = 0.85;
        public const int EdgeBandWidth = 3;
        public const int CoreRingMin = 5;
        public const int CoreRingMax = 8;
        public const int VfxFlipbookMaxFrames = 16;
        public const int VfxFlipbookMaxSheetPx = 1024;
        public const int VfxFlipbookFpsLow = 12;
        public const int VfxFlipbookFpsHigh = 24;

        public sealed class Report
        {
            public string Path = string.Empty;
            public int SilhouettePixels;
            public int EdgeBandPixels;
            public string CoreMode = "RING";
            public double ValueRangeL;
            public int ValueTiers;
            public int SilhouetteHeight;
            public double TopLightMedian;
            public int HueClustersEligible;
            public double RimSeparatedShare;
            public double LargestFlatShare;
            public double ActorBgDeltaL;
            public readonly List<string> Violations = new List<string>();

            public bool Passed
            {
                get
                {
                    return Violations.Count == 0;
                }
            }
        }

        /// <summary>Per-rule result for the fixture sweep.</summary>
        public sealed class RuleCheck
        {
            public string Rule = string.Empty;
            public bool Passed;
            public string Detail = string.Empty;
        }

        /// <summary>
        /// Extract silhouette membership + Lab values. mask excludes pixels
        /// from S. Returns lab[y*w+x] for S pixels; labValid marks them.
        /// </summary>
        public static void Silhouette(
            LabPixels.Image img, bool[]? mask,
            out bool[] inS, out LabPixels.Lab[] lab)
        {
            int n = img.Width * img.Height;
            inS = new bool[n];
            lab = new LabPixels.Lab[n];
            for (int i = 0; i < n; i++)
            {
                var p = img.Pixels[i];
                if (p.A >= 128 && (mask == null || !mask[i]))
                {
                    inS[i] = true;
                    lab[i] = LabPixels.ToLab(p.R, p.G, p.B);
                }
            }
        }

        /// <summary>
        /// Chebyshev distance transform to outside-S. dist[y*w+x] = Chebyshev
        /// distance in px from each pixel to the nearest pixel NOT in S
        /// (outside the cell counts as outside S). Two-pass chamfer with
        /// row-major tie order — deterministic and exact for integer
        /// Chebyshev distance.
        /// </summary>
        public static int[] OutsideDistance(bool[] inS, int w, int h)
        {
            int n = w * h;
            var dist = new int[n];
            const int inf = int.MaxValue / 4;
            for (int i = 0; i < n; i++)
            {
                dist[i] = inS[i] ? inf : 0;
            }
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    int i = y * w + x;
                    if (dist[i] == 0)
                    {
                        continue;
                    }
                    for (int dy = -1; dy <= 0; dy++)
                    {
                        // Forward pass covers the row above (dx -1..1) and
                        // the same-row left neighbour (dx -1).
                        int lo = -1;
                        int hi = dy == -1 ? 1 : -1;
                        for (int dx = lo; dx <= hi; dx++)
                        {
                            int nx = x + dx;
                            int ny = y + dy;
                            if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                            {
                                continue;
                            }
                            int v = dist[ny * w + nx] + 1;
                            if (v < dist[i])
                            {
                                dist[i] = v;
                            }
                        }
                    }
                }
            }
            for (int y = h - 1; y >= 0; y--)
            {
                for (int x = w - 1; x >= 0; x--)
                {
                    int i = y * w + x;
                    if (dist[i] == 0)
                    {
                        continue;
                    }
                    for (int dy = 0; dy <= 1; dy++)
                    {
                        // Backward pass covers the row below (dx -1..1)
                        // and the same-row right neighbour (dx 1).
                        int lo = dy == 0 ? 1 : -1;
                        int hi = 1;
                        for (int dx = lo; dx <= hi; dx++)
                        {
                            int nx = x + dx;
                            int ny = y + dy;
                            if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                            {
                                continue;
                            }
                            int v = dist[ny * w + nx] + 1;
                            if (v < dist[i])
                            {
                                dist[i] = v;
                            }
                        }
                    }
                }
            }
            for (int i = 0; i < n; i++)
            {
                if (dist[i] == int.MaxValue / 4)
                {
                    dist[i] = Math.Max(w, h);
                }
            }
            return dist;
        }

        /// <summary>
        /// Core ring K(p): for each edge-band pixel p (Chebyshev &lt;= 3 to
        /// outside S), the nearest pixel of S at Chebyshev 5..8 px to outside
        /// S (Euclidean distance; row-major tie). Empty ring falls back to
        /// the deepest pixels of S (core_mode=DEEPEST). K must be nonempty
        /// and disjoint from B; a one-band-thick silhouette fails the rim
        /// gate rather than auto-passing.
        /// </summary>
        public static void EdgeBandAndCore(
            bool[] inS, int[] dist, int w, int h,
            out List<int> band, out List<int> coreRing, out List<int> deepestCore,
            out string coreMode)
        {
            band = new List<int>();
            coreRing = new List<int>();
            deepestCore = new List<int>();
            int maxDist = 0;
            for (int i = 0; i < w * h; i++)
            {
                if (!inS[i])
                {
                    continue;
                }
                int d = dist[i];
                if (d <= EdgeBandWidth)
                {
                    band.Add(i);
                }
                else if (d >= CoreRingMin && d <= CoreRingMax)
                {
                    coreRing.Add(i);
                }
                if (d > maxDist)
                {
                    maxDist = d;
                }
            }
            if (coreRing.Count != 0)
            {
                coreMode = "RING";
                return;
            }
            for (int i = 0; i < w * h; i++)
            {
                if (inS[i] && dist[i] == maxDist && dist[i] > EdgeBandWidth)
                {
                    deepestCore.Add(i);
                }
            }
            coreMode = "DEEPEST";
        }

        /// <summary>
        /// Rim separation: share of band pixels with |L*(p) - L*(K(p))| &gt;=
        /// 12 where K(p) is the nearest core pixel (Euclidean; row-major
        /// tie). The core set is the 5..8px ring, or the deepest pixels
        /// (DEEPEST mode) when the ring is empty.
        /// </summary>
        public static double RimSeparation(
            List<int> band, List<int> core, LabPixels.Lab[] lab, int w)
        {
            if (band.Count == 0)
            {
                return double.NaN;
            }
            if (core.Count == 0)
            {
                return 0.0;
            }
            int hit = 0;
            for (int i = 0; i < band.Count; i++)
            {
                int p = band[i];
                int px = p % w;
                int py = p / w;
                double best = double.MaxValue;
                int bj = -1;
                for (int j = 0; j < core.Count; j++)
                {
                    int q = core[j];
                    int qx = q % w;
                    int qy = q / w;
                    double d = (qx - px) * (double)(qx - px) + (qy - py) * (double)(qy - py);
                    if (d < best)
                    {
                        best = d;
                        bj = j;
                    }
                }
                if (bj < 0)
                {
                    continue;
                }
                if (Math.Abs(lab[p].L - lab[core[bj]].L) >= RimDeltaLMin)
                {
                    hit++;
                }
            }
            return (double)hit / band.Count;
        }

        /// <summary>
        /// Value range: L*(p95) - L*(p5) over S, nearest-rank quantiles on
        /// the (L*,y,x) sorted order.
        /// </summary>
        public static double ValueRange(List<int> sPixels, LabPixels.Lab[] lab)
        {
            if (sPixels.Count == 0)
            {
                return double.NaN;
            }
            var ls = new double[sPixels.Count];
            for (int i = 0; i < sPixels.Count; i++)
            {
                ls[i] = lab[sPixels[i]].L;
            }
            Array.Sort(ls);
            return LabPixels.Quantile(ls, 0.95) - LabPixels.Quantile(ls, 0.05);
        }

        /// <summary>
        /// Value tiers: 1-D k-means k=5 on L*, initial centers at L* of the
        /// p10/p30/p50/p70/p90 pixels, Lloyd until identical assignment or
        /// 100 iterations. At least 3 clusters each covering &gt;= 5% of S.
        /// </summary>
        public static int ValueTiers(List<int> sPixels, LabPixels.Lab[] lab)
        {
            if (sPixels.Count == 0)
            {
                return 0;
            }
            var ls = new double[sPixels.Count];
            for (int i = 0; i < sPixels.Count; i++)
            {
                ls[i] = lab[sPixels[i]].L;
            }
            var sorted = (double[])ls.Clone();
            Array.Sort(sorted);
            var centers = new double[5];
            double[] qs = { 0.10, 0.30, 0.50, 0.70, 0.90 };
            for (int k = 0; k < 5; k++)
            {
                centers[k] = LabPixels.Quantile(sorted, qs[k]);
            }
            var assign = new int[ls.Length];
            var prev = new int[ls.Length];
            for (int i = 0; i < ls.Length; i++)
            {
                prev[i] = -1;
            }
            int areasFinal;
            for (int iter = 0; iter < 100; iter++)
            {
                for (int i = 0; i < ls.Length; i++)
                {
                    double best = double.MaxValue;
                    int bj = 0;
                    for (int k = 0; k < 5; k++)
                    {
                        double d = ls[i] - centers[k];
                        double dd = d * d;
                        if (dd < best)
                        {
                            best = dd;
                            bj = k;
                        }
                    }
                    assign[i] = bj;
                }
                if (ArraysEqual(assign, prev))
                {
                    break;
                }
                Array.Copy(assign, prev, ls.Length);
                var sums = new double[5];
                var counts = new int[5];
                for (int i = 0; i < ls.Length; i++)
                {
                    sums[assign[i]] += ls[i];
                    counts[assign[i]]++;
                }
                for (int k = 0; k < 5; k++)
                {
                    if (counts[k] != 0)
                    {
                        centers[k] = sums[k] / counts[k];
                    }
                }
            }
            areasFinal = 0;
            var area = new int[5];
            for (int i = 0; i < ls.Length; i++)
            {
                area[assign[i]]++;
            }
            int min = Math.Max(1, (int)Math.Ceiling(ls.Length * 0.05));
            for (int k = 0; k < 5; k++)
            {
                if (area[k] >= min)
                {
                    areasFinal++;
                }
            }
            return areasFinal;
        }

        /// <summary>
        /// Hue clusters: 2-D k-means k=4 on (a*,b*) of S, seeded at the
        /// (a*,b*) of the pixels at L* rank p12.5/p37.5/p62.5/p87.5, Lloyd
        /// until identical assignment or 100 iterations; equal squared
        /// distances pick the lower original index; empty clusters keep
        /// their previous finite center and count area 0.
        /// </summary>
        public static List<int>[] HueClusters(List<int> sPixels, LabPixels.Lab[] lab, int w)
        {
            var empties = new List<int>[4];
            for (int k = 0; k < 4; k++)
            {
                empties[k] = new List<int>();
            }
            if (sPixels.Count == 0)
            {
                return empties;
            }
            var order = sPixels.ToArray();
            Array.Sort(order, (a, b) =>
            {
                int c = lab[a].L.CompareTo(lab[b].L);
                if (c != 0)
                {
                    return c;
                }
                int ya = a / w;
                int xa = a % w;
                int yb = b / w;
                int xb = b % w;
                c = ya.CompareTo(yb);
                return c != 0 ? c : xa.CompareTo(xb);
            });
            double[] qs = { 0.125, 0.375, 0.625, 0.875 };
            var centers = new (double a, double b)[4];
            for (int k = 0; k < 4; k++)
            {
                int i = (int)Math.Max(0, Math.Ceiling(qs[k] * order.Length) - 1);
                var l = lab[order[i]];
                centers[k] = (l.A, l.B);
            }
            var assign = new int[sPixels.Count];
            var prev = new int[sPixels.Count];
            for (int i = 0; i < assign.Length; i++)
            {
                prev[i] = -1;
            }
            for (int iter = 0; iter < 100; iter++)
            {
                for (int i = 0; i < sPixels.Count; i++)
                {
                    var l = lab[sPixels[i]];
                    double best = double.MaxValue;
                    int bj = 0;
                    for (int k = 0; k < 4; k++)
                    {
                        double da = l.A - centers[k].a;
                        double db = l.B - centers[k].b;
                        double dd = da * da + db * db;
                        if (dd < best)
                        {
                            best = dd;
                            bj = k;
                        }
                    }
                    assign[i] = bj;
                }
                if (ArraysEqual(assign, prev))
                {
                    break;
                }
                Array.Copy(assign, prev, assign.Length);
                var sa = new double[4];
                var sb = new double[4];
                var counts = new int[4];
                for (int i = 0; i < sPixels.Count; i++)
                {
                    var l = lab[sPixels[i]];
                    sa[assign[i]] += l.A;
                    sb[assign[i]] += l.B;
                    counts[assign[i]]++;
                }
                for (int k = 0; k < 4; k++)
                {
                    if (counts[k] != 0)
                    {
                        centers[k] = (sa[k] / counts[k], sb[k] / counts[k]);
                    }
                }
            }
            for (int i = 0; i < sPixels.Count; i++)
            {
                empties[assign[i]].Add(sPixels[i]);
            }
            return empties;
        }

        /// <summary>
        /// Top light (ART-003): for each hue cluster covering &gt;= 5% of S,
        /// d(C) = mean L* of the top third of its bbox minus the bottom
        /// third (thirds = max(1, ceil(h/3)) rows each end). The gate is the
        /// area-weighted median of d(C) over eligible clusters &gt;= 4.
        /// Weighted median sorts (d, cluster_index) and takes the first d
        /// whose doubled cumulative pixel area reaches the total eligible
        /// area (lower median on an exact half). A cluster whose third is
        /// empty of C pixels fails its measurement.
        /// </summary>
        public static double TopLight(
            List<int>[] clusters, int clusterCount, int sTotal, LabPixels.Lab[] lab, int w)
        {
            var eligible = new List<(double d, int idx, int area)>();
            for (int k = 0; k < clusterCount; k++)
            {
                var c = clusters[k];
                if (c.Count == 0 || c.Count * 20 < sTotal)
                {
                    continue;
                }
                int minY = int.MaxValue;
                int maxY = -1;
                int minX = int.MaxValue;
                int maxX = -1;
                for (int i = 0; i < c.Count; i++)
                {
                    int x = c[i] % w;
                    int y = c[i] / w;
                    if (y < minY)
                    {
                        minY = y;
                    }
                    if (y > maxY)
                    {
                        maxY = y;
                    }
                    if (x < minX)
                    {
                        minX = x;
                    }
                    if (x > maxX)
                    {
                        maxX = x;
                    }
                }
                int hh = maxY - minY + 1;
                int third = Math.Max(1, (int)Math.Ceiling(hh / 3.0));
                double topSum = 0;
                int topN = 0;
                double botSum = 0;
                int botN = 0;
                for (int i = 0; i < c.Count; i++)
                {
                    int y = c[i] / w;
                    if (y < minY + third)
                    {
                        topSum += lab[c[i]].L;
                        topN++;
                    }
                    if (y > maxY - third)
                    {
                        botSum += lab[c[i]].L;
                        botN++;
                    }
                }
                if (topN == 0 || botN == 0)
                {
                    return double.NaN;
                }
                eligible.Add((topSum / topN - botSum / botN, k, c.Count));
            }
            if (eligible.Count == 0)
            {
                return double.NaN;
            }
            eligible.Sort((a, b) =>
            {
                int c = a.d.CompareTo(b.d);
                return c != 0 ? c : a.idx.CompareTo(b.idx);
            });
            long totalArea = 0;
            for (int i = 0; i < eligible.Count; i++)
            {
                totalArea += eligible[i].area;
            }
            long cum = 0;
            for (int i = 0; i < eligible.Count; i++)
            {
                cum += eligible[i].area;
                if (cum * 2 >= totalArea)
                {
                    return eligible[i].d;
                }
            }
            return eligible[eligible.Count - 1].d;
        }

        /// <summary>
        /// Flat regions (ART-002, spec-governing per F-5): 8-connected
        /// components of S pixels sharing one Lab bin
        /// (floor(L*/3), floor(a*/6), floor(b*/6)); a smooth gradient spans
        /// many bins so it never coalesces. No component may exceed 20% of S.
        /// </summary>
        public static double LargestFlatShare(
            List<int> sPixels, LabPixels.Lab[] lab, bool[] inS, int w, int h)
        {
            int n = w * h;
            var seen = new bool[n];
            var keyL = new int[n];
            var keyA = new int[n];
            var keyB = new int[n];
            for (int i = 0; i < n; i++)
            {
                if (!inS[i])
                {
                    continue;
                }
                var b = LabPixels.LabBin(lab[i]);
                keyL[i] = b.L;
                keyA[i] = b.A;
                keyB[i] = b.B;
            }
            int largest = 0;
            var queue = new int[n];
            for (int i = 0; i < n; i++)
            {
                if (!inS[i] || seen[i])
                {
                    continue;
                }
                int head = 0;
                int tail = 0;
                queue[tail++] = i;
                seen[i] = true;
                int area = 0;
                while (head < tail)
                {
                    int cur = queue[head++];
                    area++;
                    int cx = cur % w;
                    int cy = cur / w;
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        for (int dx = -1; dx <= 1; dx++)
                        {
                            if (dx == 0 && dy == 0)
                            {
                                continue;
                            }
                            int nx = cx + dx;
                            int ny = cy + dy;
                            if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                            {
                                continue;
                            }
                            int ni = ny * w + nx;
                            if (!inS[ni] || seen[ni])
                            {
                                continue;
                            }
                            if (keyL[ni] == keyL[cur] && keyA[ni] == keyA[cur] && keyB[ni] == keyB[cur])
                            {
                                seen[ni] = true;
                                queue[tail++] = ni;
                            }
                        }
                    }
                }
                if (area > largest)
                {
                    largest = area;
                }
            }
            if (sPixels.Count == 0)
            {
                return double.NaN;
            }
            return (double)largest / sPixels.Count;
        }

        /// <summary>
        /// Actor-on-background: |mean L*(edge band B) - mean L*(background
        /// ring 4..12 px outside S)| &gt;= 20 on the review render.
        /// </summary>
        public static double ActorBgDeltaL(
            bool[] inS, List<int> band, LabPixels.Lab[] lab, int[] dist, int w, int h)
        {
            if (band.Count == 0)
            {
                return double.NaN;
            }
            double bSum = 0;
            for (int i = 0; i < band.Count; i++)
            {
                bSum += lab[band[i]].L;
            }
            double bgSum = 0;
            int bgN = 0;
            for (int i = 0; i < w * h; i++)
            {
                if (inS[i])
                {
                    continue;
                }
                int d = dist[i];
                if (d >= 4 && d <= 12)
                {
                    bgSum += lab[i].L;
                    bgN++;
                }
            }
            if (bgN == 0)
            {
                return double.NaN;
            }
            return Math.Abs(bSum / band.Count - bgSum / bgN);
        }

        /// <summary>
        /// Full section-3.6 measurement of one texture cell. Returns the
        /// report with every numeric measure plus the applicable violations.
        /// </summary>
        public static Report Measure(
            string path, LabPixels.Image img, bool[]? mask,
            int minBodyH, int maxBodyH)
        {
            var r = new Report { Path = path };
            Silhouette(img, mask, out var inS, out var lab);
            int w = img.Width;
            int h = img.Height;
            var sPixels = new List<int>();
            for (int i = 0; i < w * h; i++)
            {
                if (inS[i])
                {
                    sPixels.Add(i);
                }
            }
            r.SilhouettePixels = sPixels.Count;
            if (sPixels.Count == 0)
            {
                r.Violations.Add("empty silhouette");
                return r;
            }
            var dist = OutsideDistance(inS, w, h);
            EdgeBandAndCore(inS, dist, w, h, out var band, out var ring, out var deepest, out var mode);
            r.CoreMode = mode;
            r.EdgeBandPixels = band.Count;
            var core = ring.Count != 0 ? ring : deepest;
            if (band.Count == 0)
            {
                r.Violations.Add("empty edge band");
            }
            if (core.Count == 0 && band.Count != 0)
            {
                r.Violations.Add("silhouette is one edge band thick; rim gate fails");
            }
            r.ValueRangeL = ValueRange(sPixels, lab);
            if (double.IsNaN(r.ValueRangeL) || r.ValueRangeL < ValueRangeMin)
            {
                r.Violations.Add("value range L*(p95)-L*(p5) < 40");
            }
            r.ValueTiers = ValueTiers(sPixels, lab);
            if (r.ValueTiers < 3)
            {
                r.Violations.Add("fewer than 3 value tiers of >=5% S");
            }
            var clusters = HueClusters(sPixels, lab, w);
            int eligible = 0;
            for (int k = 0; k < clusters.Length; k++)
            {
                if (clusters[k].Count != 0 && clusters[k].Count * 20 >= sPixels.Count)
                {
                    eligible++;
                }
            }
            r.HueClustersEligible = eligible;
            r.TopLightMedian = eligible == 0 ? double.NaN
                : TopLight(clusters, clusters.Length, sPixels.Count, lab, w);
            if (double.IsNaN(r.TopLightMedian) || r.TopLightMedian < TopLightMin)
            {
                r.Violations.Add("top-light weighted median < 4");
            }
            r.RimSeparatedShare = RimSeparation(band, core, lab, w);
            if (double.IsNaN(r.RimSeparatedShare) || r.RimSeparatedShare < RimShareMin)
            {
                r.Violations.Add("rim separation share < 60%");
            }
            r.LargestFlatShare = LargestFlatShare(sPixels, lab, inS, w, h);
            if (double.IsNaN(r.LargestFlatShare) || r.LargestFlatShare > FlatRegionMaxShare)
            {
                r.Violations.Add("flat region exceeds 20% of S");
            }
            int minY = h;
            int maxY = -1;
            foreach (var px in sPixels)
            {
                int py = px / w;
                if (py < minY)
                {
                    minY = py;
                }
                if (py > maxY)
                {
                    maxY = py;
                }
            }
            r.SilhouetteHeight = maxY - minY + 1;
            if (minBodyH > 0 &&
                (r.SilhouetteHeight < minBodyH || r.SilhouetteHeight > maxBodyH))
            {
                r.Violations.Add("silhouette height outside declared body band");
            }
            return r;
        }

        /// <summary>
        /// Environment layer rule (section 3.6): on the isolated 1280x720
        /// day review render of each layer alone, contrast L*(p95-p5) must
        /// satisfy L1 &gt;= L2 &gt;= L3 &gt;= L4 and L4 &lt;= 0.5 * L1, with
        /// mean saturation C*ab decreasing L1 -&gt; L4.
        /// </summary>
        public static List<string> CheckEnvironmentLayers(List<LabPixels.Lab[]> layerLabs)
        {
            var errs = new List<string>();
            if (layerLabs.Count < 4)
            {
                errs.Add("fewer than 4 environment layers supplied");
                return errs;
            }
            var ranges = new double[layerLabs.Count];
            var saturations = new double[layerLabs.Count];
            for (int i = 0; i < layerLabs.Count; i++)
            {
                var ll = layerLabs[i];
                if (ll.Length == 0)
                {
                    errs.Add("layer " + i + " has no opaque pixels");
                    continue;
                }
                var ls = new double[ll.Length];
                double cs = 0;
                for (int j = 0; j < ll.Length; j++)
                {
                    ls[j] = ll[j].L;
                    cs += Math.Sqrt(ll[j].A * ll[j].A + ll[j].B * ll[j].B);
                }
                Array.Sort(ls);
                ranges[i] = LabPixels.Quantile(ls, 0.95) - LabPixels.Quantile(ls, 0.05);
                saturations[i] = cs / ll.Length;
            }
            for (int i = 1; i < ranges.Length; i++)
            {
                if (!(ranges[i] <= ranges[i - 1] + 1e-9))
                {
                    errs.Add("contrast order violated: L" + (i + 1) + " > L" + i);
                }
            }
            if (ranges[0] > 0 && !(ranges[ranges.Length - 1] <= 0.5 * ranges[0] + 1e-9))
            {
                errs.Add("far layer contrast exceeds 0.5 x L1");
            }
            for (int i = 1; i < saturations.Length; i++)
            {
                if (!(saturations[i] <= saturations[i - 1] + 1e-9))
                {
                    errs.Add("saturation order violated: C*ab L" + (i + 1) + " > L" + i);
                }
            }
            return errs;
        }

        /// <summary>
        /// Palette gate (ART-005): at least 85% of S pixels must lie within
        /// DeltaE00 &lt;= 8 of the nearest palette color.
        /// </summary>
        public static double PaletteCoverage(
            List<int> sPixels, LabPixels.Lab[] lab, List<LabPixels.Lab> palette)
        {
            if (sPixels.Count == 0 || palette.Count == 0)
            {
                return double.NaN;
            }
            int hit = 0;
            for (int i = 0; i < sPixels.Count; i++)
            {
                var l = lab[sPixels[i]];
                double best = double.MaxValue;
                for (int k = 0; k < palette.Count; k++)
                {
                    double d = LabPixels.DeltaE00(l, palette[k]);
                    if (d < best)
                    {
                        best = d;
                    }
                }
                if (best <= PaletteDeltaE00Max)
                {
                    hit++;
                }
            }
            return (double)hit / sPixels.Count;
        }

        /// <summary>
        /// Frame consistency (ART-004): every frame's pixels assign to the
        /// idle_0 hue centers by nearest (a*,b*) with lower-index ties; each
        /// candidate frame cluster must be nonempty and its mean Lab within
        /// DeltaE00 &lt;= 3 of the corresponding idle_0 mean. Empty frame
        /// silhouettes fail.
        /// </summary>
        public static List<string> CheckFrameConsistency(
            List<int> idlePixels, LabPixels.Lab[] idleLab, int w,
            List<int> framePixels, LabPixels.Lab[] frameLab)
        {
            var errs = new List<string>();
            if (framePixels.Count == 0)
            {
                errs.Add("empty frame silhouette");
                return errs;
            }
            var centers = HueClusters(idlePixels, idleLab, w);
            var centersList = new List<(double a, double b, LabPixels.Lab mean)>();
            for (int k = 0; k < centers.Length; k++)
            {
                if (centers[k].Count == 0)
                {
                    continue;
                }
                double sa = 0;
                double sb = 0;
                double sl = 0;
                for (int i = 0; i < centers[k].Count; i++)
                {
                    var l = idleLab[centers[k][i]];
                    sa += l.A;
                    sb += l.B;
                    sl += l.L;
                }
                int n = centers[k].Count;
                centersList.Add((sa / n, sb / n,
                    new LabPixels.Lab { L = sl / n, A = sa / n, B = sb / n }));
            }
            var meanSums = new double[centersList.Count, 3];
            var counts = new int[centersList.Count];
            for (int i = 0; i < framePixels.Count; i++)
            {
                var l = frameLab[framePixels[i]];
                double best = double.MaxValue;
                int bj = -1;
                for (int k = 0; k < centersList.Count; k++)
                {
                    double da = l.A - centersList[k].a;
                    double db = l.B - centersList[k].b;
                    double dd = da * da + db * db;
                    if (dd < best)
                    {
                        best = dd;
                        bj = k;
                    }
                }
                if (bj < 0)
                {
                    continue;
                }
                meanSums[bj, 0] += l.L;
                meanSums[bj, 1] += l.A;
                meanSums[bj, 2] += l.B;
                counts[bj]++;
            }
            for (int k = 0; k < centersList.Count; k++)
            {
                if (counts[k] == 0)
                {
                    errs.Add("frame has no pixels in idle_0 cluster " + k);
                    continue;
                }
                var mean = new LabPixels.Lab
                {
                    L = meanSums[k, 0] / counts[k],
                    A = meanSums[k, 1] / counts[k],
                    B = meanSums[k, 2] / counts[k],
                };
                double de = LabPixels.DeltaE00(mean, centersList[k].mean);
                if (!double.IsFinite(de) || de > 3.0)
                {
                    errs.Add("frame cluster " + k + " DeltaE00 " + de.ToString("F2") + " > 3");
                }
            }
            return errs;
        }

        private static bool ArraysEqual(int[] a, int[] b)
        {
            for (int i = 0; i < a.Length; i++)
            {
                if (a[i] != b[i])
                {
                    return false;
                }
            }
            return true;
        }
    }
}
