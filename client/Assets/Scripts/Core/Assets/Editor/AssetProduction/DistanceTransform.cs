namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Exact Euclidean nearest-source map in O(w*h): the two-phase
    /// distance transform (per-column 1-D transform, then a per-row
    /// lower envelope of parabolas). The result maps every pixel index
    /// to the index of its nearest source pixel; equal distances resolve
    /// in row-major (y,x) order — the semantics the naive full scans in
    /// the quality gates implement. -1 marks "no source pixel exists".
    /// </summary>
    public static class DistanceTransform
    {
        private const int Inf = int.MaxValue / 4;

        /// <summary>
        /// Build the nearest-source index map for a boolean source mask
        /// of w*h cells (row-major).
        /// </summary>
        public static int[] NearestSource(bool[] src, int w, int h)
        {
            int n = w * h;
            var g = new int[n];
            var sy = new int[n];

            // Column pass: g[i] = squared distance to the nearest source
            // inside the same column; sy[i] = that source's row. Forward
            // sweep tracks the nearest source above; backward sweep
            // replaces only when strictly nearer, so equal distance
            // keeps the smaller source row (row-major order).
            for (int x = 0; x < w; x++)
            {
                int dist = Inf;
                int srcY = -1;
                for (int y = 0; y < h; y++)
                {
                    int i = y * w + x;
                    if (src[i])
                    {
                        dist = 0;
                        srcY = y;
                    }
                    else if (srcY >= 0)
                    {
                        dist++;
                    }
                    g[i] = srcY < 0 ? Inf : dist * dist;
                    sy[i] = srcY;
                }
                dist = Inf;
                srcY = -1;
                for (int y = h - 1; y >= 0; y--)
                {
                    int i = y * w + x;
                    if (src[i])
                    {
                        dist = 0;
                        srcY = y;
                    }
                    else if (srcY >= 0)
                    {
                        dist++;
                    }
                    int below = srcY < 0 ? Inf : dist * dist;
                    if (below < g[i])
                    {
                        g[i] = below;
                        sy[i] = srcY;
                    }
                }
            }

            // Row pass: each column c contributes the parabola
            // p_c(x) = (x-c)^2 + g[c,y]; the lower envelope gives the
            // winning column per x. The hull keeps parabolas whose
            // intersection lands exactly on an existing boundary (zero
            // width) instead of popping them — they still tie at that
            // integer x. At a query point the tie set is the winning
            // segment plus the contiguous run of segments ending at that
            // boundary, resolved to the smallest (source row, source
            // column).
            var near = new int[n];
            var v = new int[w];
            var z = new double[w + 1];
            for (int y = 0; y < h; y++)
            {
                int row = y * w;
                int m = 0;
                for (int c = 0; c < w; c++)
                {
                    if (g[row + c] >= Inf)
                    {
                        continue;
                    }
                    double s = double.NegativeInfinity;
                    while (m > 0)
                    {
                        int u = v[m - 1];
                        s = ((g[row + c] + (double)c * c) - (g[row + u] + (double)u * u))
                            / (2.0 * (c - u));
                        if (s >= z[m - 1])
                        {
                            break;
                        }
                        m--;
                    }
                    v[m] = c;
                    z[m] = m == 0 ? double.NegativeInfinity : s;
                    z[m + 1] = double.PositiveInfinity;
                    m++;
                }
                if (m == 0)
                {
                    for (int x = 0; x < w; x++)
                    {
                        near[row + x] = -1;
                    }
                    continue;
                }
                int p = 0;
                for (int x = 0; x < w; x++)
                {
                    while (x >= z[p + 1])
                    {
                        p++;
                    }
                    int best = v[p];
                    int bestVal = g[row + best] + (x - best) * (x - best);
                    int j = p - 1;
                    while (j >= 0 && z[j + 1] == x)
                    {
                        int cj = v[j];
                        if (g[row + cj] + (x - cj) * (x - cj) == bestVal)
                        {
                            int ry = sy[row + cj];
                            int by = sy[row + best];
                            if (ry < by || (ry == by && cj < best))
                            {
                                best = cj;
                            }
                        }
                        j--;
                    }
                    near[row + x] = sy[row + best] * w + best;
                }
            }
            return near;
        }
    }
}
