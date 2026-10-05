using NUnit.Framework;
using ThinhThan.Core.Assets.Editor.AssetProduction;

namespace ThinhThan.Tests.EditMode.CutoutQualityGate
{
    /// <summary>
    /// DistanceTransform.NearestSource coverage: a naive O(n) full scan
    /// (Euclidean distance, row-major (y,x) ties) is the reference; the
    /// two-pass transform must agree on every mask.
    /// </summary>
    public class DistanceTransformTests
    {
        private static int[] Naive(bool[] src, int w, int h)
        {
            var outp = new int[w * h];
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    int best = -1;
                    int bx = -1;
                    int by = -1;
                    for (int yy = 0; yy < h; yy++)
                    {
                        for (int xx = 0; xx < w; xx++)
                        {
                            if (!src[yy * w + xx])
                            {
                                continue;
                            }
                            int d = (xx - x) * (xx - x) + (yy - y) * (yy - y);
                            if (best < 0 || d < best
                                || (d == best && (yy < by || (yy == by && xx < bx))))
                            {
                                best = d;
                                bx = xx;
                                by = yy;
                            }
                        }
                    }
                    outp[y * w + x] = bx < 0 ? -1 : by * w + bx;
                }
            }
            return outp;
        }

        private static void AssertAgrees(bool[] src, int w, int h, string tag)
        {
            var expected = Naive(src, w, h);
            var actual = DistanceTransform.NearestSource(src, w, h);
            for (int i = 0; i < expected.Length; i++)
            {
                Assert.AreEqual(expected[i], actual[i],
                    tag + ": pixel " + (i % w) + "," + (i / w));
            }
        }

        [Test]
        public void TestNearestSourceExhaustive3x3()
        {
            for (int mask = 0; mask < 512; mask++)
            {
                var src = new bool[9];
                for (int i = 0; i < 9; i++)
                {
                    src[i] = (mask & (1 << i)) != 0;
                }
                AssertAgrees(src, 3, 3, "mask " + mask);
            }
        }

        [Test]
        public void TestNearestSourceDirectedCases()
        {
            // Equal-distance sources above and below a query: the
            // smaller source row wins (row-major tie).
            var tie = new bool[25];
            tie[1 * 5 + 2] = true;
            tie[3 * 5 + 2] = true;
            AssertAgrees(tie, 5, 5, "vertical tie");
            Assert.AreEqual(1 * 5 + 2,
                DistanceTransform.NearestSource(tie, 5, 5)[2 * 5 + 2],
                "vertical tie must resolve to the smaller source row");

            // Three sources equidistant from one query.
            var three = new bool[25];
            three[2 * 5 + 0] = true;
            three[0 * 5 + 2] = true;
            three[2 * 5 + 4] = true;
            AssertAgrees(three, 5, 5, "three-way tie");
            Assert.AreEqual(0 * 5 + 2,
                DistanceTransform.NearestSource(three, 5, 5)[2 * 5 + 2],
                "three-way tie must resolve to smallest (row, col)");

            // Horizontal pair equidistant from a query on the same row.
            var horiz = new bool[9];
            horiz[1 * 3 + 0] = true;
            horiz[1 * 3 + 2] = true;
            Assert.AreEqual(1 * 3 + 0,
                DistanceTransform.NearestSource(horiz, 3, 3)[1 * 3 + 1],
                "horizontal tie must resolve to the smaller source column");

            // No sources at all.
            var empty = new bool[16];
            var near = DistanceTransform.NearestSource(empty, 4, 4);
            for (int i = 0; i < near.Length; i++)
            {
                Assert.AreEqual(-1, near[i]);
            }

            // Every pixel is a source: nearest is itself.
            var full = new bool[16];
            for (int i = 0; i < full.Length; i++)
            {
                full[i] = true;
            }
            AssertAgrees(full, 4, 4, "all sources");
        }

        [Test]
        public void TestNearestSourceSeededMasks()
        {
            // Deterministic LCG masks across sizes and densities; no RNG.
            uint state = 0x9E3779B9u;
            int[] sizes = { 4, 7, 5, 12, 8, 16, 6, 11 };
            for (int t = 0; t < 8; t++)
            {
                int w = sizes[t];
                int h = sizes[(t + 3) % sizes.Length];
                uint mod = (uint)(2 + (t % 5));
                var src = new bool[w * h];
                for (int i = 0; i < src.Length; i++)
                {
                    state = state * 1664525u + 1013904223u;
                    src[i] = (state >> 28) % mod == 0;
                }
                AssertAgrees(src, w, h, "lcg " + w + "x" + h);
            }
        }
    }
}
