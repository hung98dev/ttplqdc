using System;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Deterministic pixel analysis shared by the Cutout (section 3.2) and
    /// Volume &amp; Depth (section 3.6) gates. Coordinates are image-origin
    /// top-left row-major (y,x); sRGB converts through IEC sRGB transfer to
    /// CIELAB D65; all arithmetic is binary64; NaN never passes a gate.
    /// </summary>
    public static class LabPixels
    {
        public struct Rgba
        {
            public byte R;
            public byte G;
            public byte B;
            public byte A;
        }

        /// <summary>Row-major RGBA8 buffer, origin top-left.</summary>
        public sealed class Image
        {
            public readonly int Width;
            public readonly int Height;
            public readonly Rgba[] Pixels;

            public Image(int width, int height)
            {
                Width = width;
                Height = height;
                Pixels = new Rgba[width * height];
            }

            public Rgba At(int x, int y)
            {
                return Pixels[y * Width + x];
            }

            public void Set(int x, int y, Rgba p)
            {
                Pixels[y * Width + x] = p;
            }
        }

        public struct Lab
        {
            public double L;
            public double A;
            public double B;
        }

        private static readonly double[] SrgbLinear = BuildSrgbTable();

        private static double[] BuildSrgbTable()
        {
            var t = new double[256];
            for (int i = 0; i < 256; i++)
            {
                double c = i / 255.0;
                t[i] = c <= 0.04045 ? c / 12.92 : Math.Pow((c + 0.055) / 1.055, 2.4);
            }
            return t;
        }

        /// <summary>sRGB 8-bit to CIELAB D65 (spec section 3.6 single measure).</summary>
        public static Lab ToLab(byte r8, byte g8, byte b8)
        {
            double r = SrgbLinear[r8];
            double g = SrgbLinear[g8];
            double b = SrgbLinear[b8];
            double x = r * 0.4124564 + g * 0.3575761 + b * 0.1804375;
            double y = r * 0.2126729 + g * 0.7151522 + b * 0.0721750;
            double z = r * 0.0193339 + g * 0.1191920 + b * 0.9503041;
            double fx = Fxyz(x / 0.95047);
            double fy = Fxyz(y / 1.0);
            double fz = Fxyz(z / 1.08883);
            var lab = new Lab();
            lab.L = 116.0 * fy - 16.0;
            lab.A = 500.0 * (fx - fy);
            lab.B = 200.0 * (fy - fz);
            return lab;
        }

        private static double Fxyz(double t)
        {
            const double e = 216.0 / 24389.0;
            const double k = 24389.0 / 27.0;
            return t > e ? Math.Pow(t, 1.0 / 3.0) : (k * t + 16.0) / 116.0;
        }

        /// <summary>
        /// CIE76-free section-3.6 flatness: Lab bin
        /// (floor(L*/3), floor(a*/6), floor(b*/6)).
        /// </summary>
        public static (int L, int A, int B) LabBin(Lab c)
        {
            return (FloorDiv(c.L, 3.0), FloorDiv(c.A, 6.0), FloorDiv(c.B, 6.0));
        }

        private static int FloorDiv(double v, double d)
        {
            double q = Math.Floor(v / d);
            if (q > int.MaxValue || q < int.MinValue || double.IsNaN(q))
            {
                return 0;
            }
            return (int)q;
        }

        /// <summary>CIEDE2000 color difference (used by ART-004/ART-005 gates).</summary>
        public static double DeltaE00(Lab a, Lab b)
        {
            double l1 = a.L, a1 = a.A, b1 = a.B;
            double l2 = b.L, a2 = b.A, b2 = b.B;
            double c1 = Math.Sqrt(a1 * a1 + b1 * b1);
            double c2 = Math.Sqrt(a2 * a2 + b2 * b2);
            double cBar = (c1 + c2) * 0.5;
            double g = 0.5 * (1.0 - Math.Sqrt(Math.Pow(cBar, 7.0) / (Math.Pow(cBar, 7.0) + Math.Pow(25.0, 7.0))));
            double ap1 = a1 * (1.0 + g);
            double ap2 = a2 * (1.0 + g);
            double cp1 = Math.Sqrt(ap1 * ap1 + b1 * b1);
            double cp2 = Math.Sqrt(ap2 * ap2 + b2 * b2);
            double hp1 = HueDeg(b1, ap1);
            double hp2 = HueDeg(b2, ap2);
            double dL = l2 - l1;
            double dC = cp2 - cp1;
            double dH;
            if (cp1 * cp2 == 0.0)
            {
                dH = 0.0;
            }
            else if (Math.Abs(hp2 - hp1) <= 180.0)
            {
                dH = hp2 - hp1;
            }
            else if (hp2 - hp1 > 180.0)
            {
                dH = hp2 - hp1 - 360.0;
            }
            else
            {
                dH = hp2 - hp1 + 360.0;
            }
            double dHp = 2.0 * Math.Sqrt(cp1 * cp2) * Math.Sin(dH * Math.PI / 360.0);
            double lBp = (l1 + l2) * 0.5;
            double cBp = (cp1 + cp2) * 0.5;
            double hBp;
            if (cp1 * cp2 == 0.0)
            {
                hBp = hp1 + hp2;
            }
            else if (Math.Abs(hp1 - hp2) <= 180.0)
            {
                hBp = (hp1 + hp2) * 0.5;
            }
            else if (hp1 + hp2 < 360.0)
            {
                hBp = (hp1 + hp2 + 360.0) * 0.5;
            }
            else
            {
                hBp = (hp1 + hp2 - 360.0) * 0.5;
            }
            double t = 1.0 - 0.17 * Math.Cos((hBp - 30.0) * Math.PI / 180.0)
                + 0.24 * Math.Cos(2.0 * hBp * Math.PI / 180.0)
                + 0.32 * Math.Cos((3.0 * hBp + 6.0) * Math.PI / 180.0)
                - 0.20 * Math.Cos((4.0 * hBp - 63.0) * Math.PI / 180.0);
            double sL = 1.0 + (0.015 * (lBp - 50.0) * (lBp - 50.0)) / Math.Sqrt(20.0 + (lBp - 50.0) * (lBp - 50.0));
            double sC = 1.0 + 0.045 * cBp;
            double sH = 1.0 + 0.015 * cBp * t;
            double dTheta = 30.0 * Math.Exp(-((hBp - 275.0) / 25.0) * ((hBp - 275.0) / 25.0));
            double rC = 2.0 * Math.Sqrt(Math.Pow(cBp, 7.0) / (Math.Pow(cBp, 7.0) + Math.Pow(25.0, 7.0)));
            double rT = -Math.Sin(2.0 * dTheta * Math.PI / 180.0) * rC;
            double vL = dL / sL;
            double vC = dC / sC;
            double vH = dHp / sH;
            return Math.Sqrt(vL * vL + vC * vC + rT * vC * vH + vH * vH);
        }

        private static double HueDeg(double b, double ap)
        {
            if (ap == 0.0 && b == 0.0)
            {
                return 0.0;
            }
            double h = Math.Atan2(b, ap) * 180.0 / Math.PI;
            return h < 0.0 ? h + 360.0 : h;
        }

        /// <summary>HSV hue/saturation for the section-3.2 fringe key check.</summary>
        public static void ToHsv(byte r, byte g, byte b, out double hue, out double sat)
        {
            double rf = r / 255.0, gf = g / 255.0, bf = b / 255.0;
            double max = Math.Max(rf, Math.Max(gf, bf));
            double min = Math.Min(rf, Math.Min(gf, bf));
            double d = max - min;
            hue = 0.0;
            if (d > 0.0)
            {
                if (max == rf)
                {
                    hue = 60.0 * (((gf - bf) / d) % 6.0);
                }
                else if (max == gf)
                {
                    hue = 60.0 * ((bf - rf) / d + 2.0);
                }
                else
                {
                    hue = 60.0 * ((rf - gf) / d + 4.0);
                }
                if (hue < 0.0)
                {
                    hue += 360.0;
                }
            }
            sat = max == 0.0 ? 0.0 : d / max;
        }

        /// <summary>
        /// Nearest-rank quantile over a sorted ascending array:
        /// max(0, ceil(q*N)-1), no interpolation (spec numeric completion).
        /// </summary>
        public static double Quantile(double[] sorted, double q)
        {
            if (sorted.Length == 0)
            {
                return double.NaN;
            }
            int i = (int)Math.Max(0, Math.Ceiling(q * sorted.Length) - 1);
            return sorted[i];
        }

        /// <summary>Chebyshev distance between two pixel coords.</summary>
        public static int Chebyshev(int x1, int y1, int x2, int y2)
        {
            return Math.Max(Math.Abs(x1 - x2), Math.Abs(y1 - y2));
        }
    }
}
