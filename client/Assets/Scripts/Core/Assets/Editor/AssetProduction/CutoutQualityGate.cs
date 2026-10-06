using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Cutout Quality Gate (presentation_asset_manifest.md section 3.2):
    /// per-file alpha measurements plus a report. Every row is measured on
    /// the final 2x texture; a single violation fails the file. The
    /// translucent mask (&lt;texture&gt;.translucent.png, same size, 1-bit)
    /// exempts exactly the rows that say "translucent" — the semi-transparent
    /// band and the interior-hole check — nothing else.
    /// </summary>
    public static class CutoutQualityGate
    {
        public enum AssetClass
        {
            Actor = 0,
            Prop = 1,
            UiArt = 2,
            FontAtlas = 3,
            Tile = 4,
            ParallaxNear = 5,
            ParallaxFar = 6,
            VfxSoft = 7,
        }

        /// <summary>Rows of section 3.2 applied per asset_class (section 3.1a).</summary>
        [Flags]
        public enum RuleSet
        {
            Format = 1,
            SemiBand = 2,
            Fringe = 4,
            TransparentRgb = 8,
            Speck = 16,
            Antialias = 32,
            Hole = 64,
            CellPadding = 128,
            CellSize = 256,
            Corners = 512,
            All = 1023,
        }

        public sealed class FileReport
        {
            public string Path = string.Empty;
            public AssetClass Class;
            public int Width;
            public int Height;
            public int CellW;
            public int CellH;
            public bool HasTranslucentMask;
            public double TranslucentFraction;
            public int[] CornerAlpha = new int[4];
            public int SemiBandViolations;
            public int FringeHueViolations;
            public int FringeDeltaLViolations;
            public int TransparentRgbViolations;
            public int SpeckComponents;
            public int SpeckPixels;
            public double AntialiasRatio;
            public int InteriorHoles;
            public int PadLeft = -1;
            public int PadRight = -1;
            public int PadTop = -1;
            public int PadBottom = -1;
            public int BboxW;
            public int BboxH;
            public int BboxHBody;
            public readonly List<string> Violations = new List<string>();

            public bool Passed
            {
                get
                {
                    return Violations.Count == 0;
                }
            }
        }

        /// <summary>Section 3.1a scope table for one asset class.</summary>
        public static RuleSet RulesFor(AssetClass cls)
        {
            switch (cls)
            {
                case AssetClass.Actor:
                    return RuleSet.All;
                case AssetClass.Prop:
                    return RuleSet.All;
                case AssetClass.UiArt:
                    return RuleSet.Format | RuleSet.SemiBand | RuleSet.Fringe | RuleSet.TransparentRgb;
                case AssetClass.FontAtlas:
                    return 0;
                case AssetClass.Tile:
                    return RuleSet.Format | RuleSet.Fringe | RuleSet.TransparentRgb;
                case AssetClass.ParallaxNear:
                    return RuleSet.All;
                case AssetClass.ParallaxFar:
                    return RuleSet.Format | RuleSet.Fringe;
                case AssetClass.VfxSoft:
                    return RuleSet.Format;
                default:
                    return 0;
            }
        }

        /// <summary>Classes carrying the 4-corner alpha=0 rule (ART-001).</summary>
        public static bool CornersApply(AssetClass cls)
        {
            return cls == AssetClass.Actor || cls == AssetClass.Prop
                || cls == AssetClass.ParallaxNear;
        }

        /// <summary>
        /// Measure one final 2x texture. img is the full texture; (cx,cy) is
        /// the cell index; cellW/cellH = 2 x cell_ref px (or the declared
        /// cell_ref for PROP/VFX). mask is the optional translucent mask
        /// buffer (same dimensions as the cell; nonzero = translucent).
        /// humanoidFigure states the file is a full-figure surface (sheet,
        /// turnarounds, preview/body) of a CHARACTER or NPC_HUMANOID
        /// size_profile — the only surfaces the 176..192 body band binds
        /// (section 3.1a); every other class/profile takes only the
        /// bbox/silhouette limit.
        /// Returns the per-file report; violations are strings naming the
        /// section-3.2 row that failed.
        /// </summary>
        public static FileReport Measure(
            string path, LabPixels.Image img, int cx, int cy, int cellW, int cellH,
            AssetClass cls, bool[]? translucentMask, bool detachedParts, bool pixelArt,
            bool humanoidFigure)
        {
            var r = new FileReport
            {
                Path = path,
                Class = cls,
                Width = img.Width,
                Height = img.Height,
                CellW = cellW,
                CellH = cellH,
                HasTranslucentMask = translucentMask != null,
            };
            var rules = RulesFor(cls);
            var cell = Crop(img, cx * cellW, cy * cellH, cellW, cellH);
            int n = cellW * cellH;
            if (translucentMask != null && translucentMask.Length != n)
            {
                r.Violations.Add("translucent mask size mismatch");
                translucentMask = null;
            }

            int sil = CountAlphaGE(cell, 128);
            if (translucentMask != null)
            {
                int m = 0;
                for (int i = 0; i < n; i++)
                {
                    if (translucentMask[i])
                    {
                        m++;
                    }
                }
                r.TranslucentFraction = sil == 0 ? 0.0 : (double)m / sil;
                if (sil > 0 && r.TranslucentFraction > 0.60)
                {
                    r.Violations.Add("translucent mask exceeds 60% of silhouette");
                }
            }

            if ((rules & RuleSet.Corners) != 0 && CornersApply(cls))
            {
                CheckCorners(cell, cellW, cellH, r);
            }
            int[] nearOpaque = Array.Empty<int>();
            if ((rules & (RuleSet.Fringe | RuleSet.TransparentRgb)) != 0)
            {
                var opaque = new bool[n];
                for (int i = 0; i < n; i++)
                {
                    opaque[i] = cell.Pixels[i].A == 255;
                }
                nearOpaque = DistanceTransform.NearestSource(opaque, cellW, cellH);
            }
            if ((rules & RuleSet.SemiBand) != 0)
            {
                CheckSemiBand(cell, cellW, cellH, translucentMask, r);
            }
            if ((rules & RuleSet.Fringe) != 0)
            {
                CheckFringe(cell, cellW, cellH, nearOpaque, r);
            }
            if ((rules & RuleSet.TransparentRgb) != 0)
            {
                CheckTransparentRgb(cell, cellW, cellH, nearOpaque, r);
            }
            if ((rules & RuleSet.Speck) != 0 && !detachedParts)
            {
                CheckSpecks(cell, cellW, cellH, r);
            }
            if ((rules & RuleSet.Antialias) != 0 && !pixelArt)
            {
                CheckAntialias(cell, cellW, cellH, r);
            }
            if ((rules & RuleSet.Hole) != 0)
            {
                CheckHoles(cell, cellW, cellH, translucentMask, r);
            }
            if ((rules & RuleSet.CellPadding) != 0)
            {
                CheckPadding(cell, cellW, cellH, r);
            }
            if ((rules & RuleSet.CellSize) != 0)
            {
                CheckCellSize(img, cellW, cellH, cls, humanoidFigure, r);
            }
            return r;
        }

        private static LabPixels.Image Crop(LabPixels.Image img, int x0, int y0, int w, int h)
        {
            var outp = new LabPixels.Image(w, h);
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    int sx = x0 + x;
                    int sy = y0 + y;
                    if (sx >= 0 && sx < img.Width && sy >= 0 && sy < img.Height)
                    {
                        outp.Set(x, y, img.At(sx, sy));
                    }
                }
            }
            return outp;
        }

        private static int CountAlpha(LabPixels.Image img, int a)
        {
            int n = 0;
            for (int i = 0; i < img.Pixels.Length; i++)
            {
                if (img.Pixels[i].A == a)
                {
                    n++;
                }
            }
            return n;
        }

        private static int CountAlphaGE(LabPixels.Image img, int a)
        {
            int n = 0;
            for (int i = 0; i < img.Pixels.Length; i++)
            {
                if (img.Pixels[i].A >= a)
                {
                    n++;
                }
            }
            return n;
        }

        /// <summary>ART-001: the 4x4 corners of every cell must be alpha=0.</summary>
        private static void CheckCorners(LabPixels.Image cell, int w, int h, FileReport r)
        {
            int[] sums = { 0, 0, 0, 0 };
            int[,] ox = { { 0, 0 }, { w - 4, 0 }, { 0, h - 4 }, { w - 4, h - 4 } };
            for (int c = 0; c < 4; c++)
            {
                for (int y = 0; y < 4; y++)
                {
                    for (int x = 0; x < 4; x++)
                    {
                        sums[c] += cell.At(ox[c, 0] + x, ox[c, 1] + y).A;
                    }
                }
            }
            for (int c = 0; c < 4; c++)
            {
                r.CornerAlpha[c] = sums[c];
                if (sums[c] != 0)
                {
                    r.Violations.Add("corner " + c + " alpha != 0 (baked checkerboard/solid bg)");
                }
            }
        }

        /// <summary>
        /// Semi-transparent band: every pixel with 1 &lt;= a &lt;= 254 must
        /// sit within Chebyshev 2 px of an a = 255 pixel, except pixels in
        /// the declared translucent mask.
        /// </summary>
        private static void CheckSemiBand(
            LabPixels.Image cell, int w, int h, bool[]? mask, FileReport r)
        {
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    byte a = cell.At(x, y).A;
                    if (a == 0 || a == 255)
                    {
                        continue;
                    }
                    if (mask != null && mask[y * w + x])
                    {
                        continue;
                    }
                    if (!HasOpaqueWithin(cell, w, h, x, y, 2))
                    {
                        r.SemiBandViolations++;
                    }
                }
            }
            if (r.SemiBandViolations != 0)
            {
                r.Violations.Add(r.SemiBandViolations + " semi-transparent pixels beyond 2px of opaque");
            }
        }

        private static bool HasOpaqueWithin(LabPixels.Image img, int w, int h, int x, int y, int d)
        {
            for (int dy = -d; dy <= d; dy++)
            {
                for (int dx = -d; dx <= d; dx++)
                {
                    int nx = x + dx;
                    int ny = y + dy;
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                    {
                        continue;
                    }
                    if (img.At(nx, ny).A == 255)
                    {
                        return true;
                    }
                }
            }
            return false;
        }

        /// <summary>
        /// Fringe: a fringe pixel is 1 &lt;= a &lt;= 254 adjacent
        /// (8-connectivity) to an a = 255 pixel. Zero fringe pixels may have
        /// hue within +/-15 degrees of the key color (#FF00FF) at
        /// saturation &gt; 0.30, or |dL*| &gt; 35 vs the nearest a = 255
        /// pixel (Euclidean; ties in row-major order).
        /// </summary>
        private static void CheckFringe(
            LabPixels.Image cell, int w, int h, int[] nearOpaque, FileReport r)
        {
            const double keyHue = 300.0;
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    var p = cell.At(x, y);
                    if (p.A == 0 || p.A == 255)
                    {
                        continue;
                    }
                    if (!AdjacentToOpaque(cell, w, h, x, y))
                    {
                        continue;
                    }
                    LabPixels.ToHsv(p.R, p.G, p.B, out double hue, out double sat);
                    double dh = Math.Abs(hue - keyHue);
                    if (dh > 180.0)
                    {
                        dh = 360.0 - dh;
                    }
                    if (dh <= 15.0 && sat > 0.30)
                    {
                        r.FringeHueViolations++;
                        continue;
                    }
                    var pl = LabPixels.ToLab(p.R, p.G, p.B);
                    var src = cell.Pixels[nearOpaque[y * w + x]];
                    var near = LabPixels.ToLab(src.R, src.G, src.B);
                    if (Math.Abs(pl.L - near.L) > 35.0)
                    {
                        r.FringeDeltaLViolations++;
                    }
                }
            }
            if (r.FringeHueViolations != 0)
            {
                r.Violations.Add(r.FringeHueViolations + " fringe pixels in key hue (+/-15 deg)");
            }
            if (r.FringeDeltaLViolations != 0)
            {
                r.Violations.Add(r.FringeDeltaLViolations + " fringe pixels |dL*| > 35 (white/black fringe)");
            }
        }

        private static bool AdjacentToOpaque(LabPixels.Image img, int w, int h, int x, int y)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                for (int dx = -1; dx <= 1; dx++)
                {
                    if (dx == 0 && dy == 0)
                    {
                        continue;
                    }
                    int nx = x + dx;
                    int ny = y + dy;
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                    {
                        continue;
                    }
                    if (img.At(nx, ny).A == 255)
                    {
                        return true;
                    }
                }
            }
            return false;
        }

        /// <summary>
        /// Nearest a = 255 pixel by Euclidean distance; ties resolve in
        /// row-major (y,x) order. Returns its Lab; NaN when none exists.
        /// </summary>
        public static LabPixels.Lab NearestOpaqueLab(LabPixels.Image img, int w, int h, int x, int y)
        {
            var opaque = new bool[w * h];
            for (int i = 0; i < opaque.Length; i++)
            {
                opaque[i] = img.Pixels[i].A == 255;
            }
            int s = DistanceTransform.NearestSource(opaque, w, h)[y * w + x];
            if (s < 0)
            {
                return new LabPixels.Lab { L = double.NaN, A = double.NaN, B = double.NaN };
            }
            var p = img.Pixels[s];
            return LabPixels.ToLab(p.R, p.G, p.B);
        }

        /// <summary>
        /// Transparent RGB: every a = 0 pixel within Chebyshev 4 px of an
        /// a &gt; 0 pixel must carry the RGB of the nearest a = 255 pixel
        /// (Euclidean; row-major ties) — the dilated halo that kills bilinear
        /// and atlas fringe.
        /// </summary>
        private static void CheckTransparentRgb(
            LabPixels.Image cell, int w, int h, int[] nearOpaque, FileReport r)
        {
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    var p = cell.At(x, y);
                    if (p.A != 0)
                    {
                        continue;
                    }
                    if (!HasNonZeroAlphaWithin(cell, w, h, x, y, 4))
                    {
                        continue;
                    }
                    int s = nearOpaque[y * w + x];
                    if (s < 0)
                    {
                        continue;
                    }
                    var src = cell.Pixels[s];
                    if (p.R != src.R || p.G != src.G || p.B != src.B)
                    {
                        r.TransparentRgbViolations++;
                    }
                }
            }
            if (r.TransparentRgbViolations != 0)
            {
                r.Violations.Add(r.TransparentRgbViolations + " transparent pixels without dilated RGB");
            }
        }

        private static bool HasNonZeroAlphaWithin(LabPixels.Image img, int w, int h, int x, int y, int d)
        {
            for (int dy = -d; dy <= d; dy++)
            {
                for (int dx = -d; dx <= d; dx++)
                {
                    int nx = x + dx;
                    int ny = y + dy;
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                    {
                        continue;
                    }
                    if (img.At(nx, ny).A > 0)
                    {
                        return true;
                    }
                }
            }
            return false;
        }

        /// <summary>
        /// Stray specks: every 8-connected component of a &gt;= 16 pixels
        /// detached from the main body must have area &gt;= 64 px.
        /// </summary>
        private static void CheckSpecks(LabPixels.Image cell, int w, int h, FileReport r)
        {
            int n = w * h;
            var comp = new int[n];
            for (int i = 0; i < n; i++)
            {
                comp[i] = -1;
            }
            int count = 0;
            var areas = new List<int>();
            var queue = new int[n];
            for (int i = 0; i < n; i++)
            {
                if (comp[i] != -1 || cell.Pixels[i].A < 16)
                {
                    continue;
                }
                int head = 0;
                int tail = 0;
                queue[tail++] = i;
                comp[i] = count;
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
                            if (comp[ni] == -1 && cell.Pixels[ni].A >= 16)
                            {
                                comp[ni] = count;
                                queue[tail++] = ni;
                            }
                        }
                    }
                }
                areas.Add(area);
                count++;
            }
            if (count <= 1)
            {
                return;
            }
            int largest = 0;
            for (int i = 1; i < count; i++)
            {
                if (areas[i] > areas[largest])
                {
                    largest = i;
                }
            }
            for (int i = 0; i < count; i++)
            {
                if (i == largest)
                {
                    continue;
                }
                if (areas[i] < 64)
                {
                    r.SpeckComponents++;
                    r.SpeckPixels += areas[i];
                }
            }
            if (r.SpeckComponents != 0)
            {
                r.Violations.Add(r.SpeckComponents + " detached components < 64px");
            }
        }

        /// <summary>
        /// Antialias: at least 50% of border pixels (a &gt; 0 adjacent to
        /// a = 0) must be semi-transparent (1..254). Binary 0/255 cutouts
        /// are rejected unless pixel_art is declared.
        /// </summary>
        private static void CheckAntialias(LabPixels.Image cell, int w, int h, FileReport r)
        {
            int border = 0;
            int semi = 0;
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    byte a = cell.At(x, y).A;
                    if (a == 0)
                    {
                        continue;
                    }
                    if (!AdjacentToTransparent(cell, w, h, x, y))
                    {
                        continue;
                    }
                    border++;
                    if (a < 255)
                    {
                        semi++;
                    }
                }
            }
            if (border == 0)
            {
                return;
            }
            r.AntialiasRatio = (double)semi / border;
            if (r.AntialiasRatio < 0.5)
            {
                r.Violations.Add("binary/jagged edge: " + (r.AntialiasRatio * 100).ToString("F1")
                    + "% border pixels antialiased (< 50%)");
            }
        }

        private static bool AdjacentToTransparent(LabPixels.Image img, int w, int h, int x, int y)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                for (int dx = -1; dx <= 1; dx++)
                {
                    if (dx == 0 && dy == 0)
                    {
                        continue;
                    }
                    int nx = x + dx;
                    int ny = y + dy;
                    if (nx < 0 || ny < 0 || nx >= w || ny >= h)
                    {
                        continue;
                    }
                    if (img.At(nx, ny).A == 0)
                    {
                        return true;
                    }
                }
            }
            return false;
        }

        /// <summary>
        /// Interior holes: zero pixels with a &lt; 250 fully enclosed inside
        /// the silhouette — flood fill from cell borders marks the outside;
        /// unmarked a &lt; 250 pixels are holes unless in the translucent mask.
        /// </summary>
        private static void CheckHoles(
            LabPixels.Image cell, int w, int h, bool[]? mask, FileReport r)
        {
            int n = w * h;
            var outside = new bool[n];
            var queue = new int[n];
            int head = 0;
            int tail = 0;
            for (int x = 0; x < w; x++)
            {
                TryMark(cell, w, h, x, 0, outside, queue, ref tail);
                TryMark(cell, w, h, x, h - 1, outside, queue, ref tail);
            }
            for (int y = 0; y < h; y++)
            {
                TryMark(cell, w, h, 0, y, outside, queue, ref tail);
                TryMark(cell, w, h, w - 1, y, outside, queue, ref tail);
            }
            while (head < tail)
            {
                int cur = queue[head++];
                int cx = cur % w;
                int cy = cur / w;
                if (cx > 0)
                {
                    TryMark(cell, w, h, cx - 1, cy, outside, queue, ref tail);
                }
                if (cx < w - 1)
                {
                    TryMark(cell, w, h, cx + 1, cy, outside, queue, ref tail);
                }
                if (cy > 0)
                {
                    TryMark(cell, w, h, cx, cy - 1, outside, queue, ref tail);
                }
                if (cy < h - 1)
                {
                    TryMark(cell, w, h, cx, cy + 1, outside, queue, ref tail);
                }
            }
            int holes = 0;
            for (int i = 0; i < n; i++)
            {
                if (outside[i] || cell.Pixels[i].A >= 250)
                {
                    continue;
                }
                if (mask != null && mask[i])
                {
                    continue;
                }
                holes++;
            }
            r.InteriorHoles = holes;
            if (holes != 0)
            {
                r.Violations.Add(holes + " enclosed pixels with a < 250 (interior hole)");
            }
        }

        private static void TryMark(
            LabPixels.Image cell, int w, int h, int x, int y,
            bool[] outside, int[] queue, ref int tail)
        {
            int i = y * w + x;
            if (outside[i] || cell.Pixels[i].A >= 250)
            {
                return;
            }
            outside[i] = true;
            queue[tail++] = i;
        }

        /// <summary>
        /// Cell padding: silhouette (a &gt;= 128) must be at least 4 texture
        /// px from the left/right/top cell edges; bottom allows 0..4 px
        /// (feet on ground).
        /// </summary>
        private static void CheckPadding(LabPixels.Image cell, int w, int h, FileReport r)
        {
            int left = w;
            int right = -1;
            int top = h;
            int bottom = -1;
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    if (cell.At(x, y).A < 128)
                    {
                        continue;
                    }
                    if (x < left)
                    {
                        left = x;
                    }
                    if (x > right)
                    {
                        right = x;
                    }
                    if (y < top)
                    {
                        top = y;
                    }
                    if (y > bottom)
                    {
                        bottom = y;
                    }
                }
            }
            if (right < 0)
            {
                return;
            }
            r.PadLeft = left;
            r.PadRight = w - 1 - right;
            r.PadTop = top;
            r.PadBottom = h - 1 - bottom;
            if (r.PadLeft < 4 || r.PadRight < 4 || r.PadTop < 4)
            {
                r.Violations.Add("silhouette within 4px of cell side/top edge");
            }
            if (r.PadBottom > 4)
            {
                r.Violations.Add("silhouette floats above cell bottom (> 4px gap)");
            }
        }

        /// <summary>
        /// Cell size: texture = exactly 2 x cell ref; silhouette bbox
        /// (a &gt;= 128) at most 2 x the silhouette limit; body height of a
        /// humanoid full-figure surface is 176..192 texture px.
        /// </summary>
        private static void CheckCellSize(
            LabPixels.Image img, int cellW, int cellH, AssetClass cls,
            bool humanoidFigure, FileReport r)
        {
            int minX = img.Width;
            int maxX = -1;
            int minY = img.Height;
            int maxY = -1;
            for (int y = 0; y < img.Height; y++)
            {
                for (int x = 0; x < img.Width; x++)
                {
                    if (img.At(x, y).A < 128)
                    {
                        continue;
                    }
                    if (x < minX)
                    {
                        minX = x;
                    }
                    if (x > maxX)
                    {
                        maxX = x;
                    }
                    if (y < minY)
                    {
                        minY = y;
                    }
                    if (y > maxY)
                    {
                        maxY = y;
                    }
                }
            }
            if (maxX < 0)
            {
                r.BboxW = 0;
                r.BboxH = 0;
                return;
            }
            r.BboxW = maxX - minX + 1;
            r.BboxH = maxY - minY + 1;
            if (r.BboxW > cellW * 2 || r.BboxH > cellH * 2)
            {
                r.Violations.Add("silhouette bbox exceeds 2x limit");
            }
            if (cls == AssetClass.Actor && humanoidFigure)
            {
                r.BboxHBody = r.BboxH;
                if (r.BboxHBody < 176 || r.BboxHBody > 192)
                {
                    r.Violations.Add("actor body height " + r.BboxHBody + " outside 176..192px");
                }
            }
        }

    }
}
