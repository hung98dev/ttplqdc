using System;
using TMPro;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// 0-alloc numeric text for HUD widgets (PERF-022): formats into a
    /// per-widget char buffer and hands TMP the array slice — no string,
    /// no boxing.
    /// </summary>
    internal static class HudText
    {
        public static void SetNumber(TMP_Text? text, long value, char[] scratch)
        {
            if (text == null)
            {
                return;
            }

            if (!value.TryFormat(scratch, out int written))
            {
                text.SetText(scratch, 0, 0);
                return;
            }

            text.SetText(scratch, 0, written);
        }

        public static void Set(TMP_Text? text, string? value)
        {
            if (text != null)
            {
                text.SetText(value ?? string.Empty);
            }
        }

        /// <summary>"hp / max" compact readout into one scratch buffer.</summary>
        public static void SetPair(
            TMP_Text? text, long a, long b, char[] scratch)
        {
            if (text == null)
            {
                return;
            }

            int n = 0;
            if (!a.TryFormat(scratch.AsSpan(n), out int wA))
            {
                text.SetText(scratch, 0, 0);
                return;
            }

            n += wA;
            if (n + 3 < scratch.Length)
            {
                scratch[n] = '/';
                n += 1;
                if (b.TryFormat(scratch.AsSpan(n), out int wB))
                {
                    n += wB;
                }
            }

            text.SetText(scratch, 0, n);
        }
    }
}
