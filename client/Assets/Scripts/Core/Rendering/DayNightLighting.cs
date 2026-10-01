using UnityEngine;

namespace ThinhThan.Core.Rendering
{
    /// <summary>
    /// Day/night evaluation for the per-map Global Light2D
    /// (world_rules.md § Day and Night; client.md § Rendering). Pure math over
    /// the day phase — the FrameLoop Presentation phase evaluates it and writes
    /// the result onto the map's Global Light2D; no frame callbacks live here.
    /// </summary>
    public static class DayNightLighting
    {
        /// <summary>WORLD_DAY_SECONDS: full day/night cycle in world seconds.</summary>
        public const int WorldDaySeconds = 7200;

        /// <summary>Night covers [NightStart, WorldDaySeconds).</summary>
        public const int NightStart = 4800;

        /// <summary>Dusk transition covers [DuskStart, NightStart).</summary>
        public const int DuskStart = 4500;

        /// <summary>Dawn transition covers [0, DawnEnd).</summary>
        public const int DawnEnd = 300;

        /// <summary>
        /// phase_seconds = floor_mod(utc - epoch, WORLD_DAY_SECONDS) per
        /// world_rules.md; negative inputs wrap into [0, 7200).
        /// </summary>
        public static int PhaseSeconds(long utcSeconds)
        {
            var phase = (int)(utcSeconds % WorldDaySeconds);
            if (phase < 0)
            {
                phase += WorldDaySeconds;
            }
            return phase;
        }

        /// <summary>Day factor: 1 = full day, 0 = full night, linear across dawn/dusk.</summary>
        public static float Dayness(int phaseSeconds)
        {
            int phase = PhaseSeconds(phaseSeconds);
            if (phase >= NightStart)
            {
                return 0f;
            }
            if (phase >= DuskStart)
            {
                return (float)(NightStart - phase) / (NightStart - DuskStart);
            }
            if (phase < DawnEnd)
            {
                return (float)phase / DawnEnd;
            }
            return 1f;
        }

        /// <summary>
        /// Evaluates the piecewise rule: DAY [0,4800) holds the authored day
        /// values, NIGHT [4800,7200) the authored night values, dusk
        /// [4500,4800) and dawn [0,300) linearly interpolate between them.
        /// </summary>
        public static (Color color, float intensity) EvaluateGlobalLight(
            int phaseSeconds,
            Color dayColor,
            float dayIntensity,
            Color nightColor,
            float nightIntensity)
        {
            float t = Dayness(phaseSeconds);
            Color color = Color.Lerp(nightColor, dayColor, t);
            float intensity = Mathf.Lerp(nightIntensity, dayIntensity, t);
            return (color, intensity);
        }
    }
}
