using System;
using ThinhThan.Core.Rendering;
using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Applies a <see cref="QualityPreset"/> to the presentation sinks only
    /// (client_performance.md items 1–2, PERF-001): quality level, URP
    /// render scale, point-light budget, aggregate particle budget,
    /// parallax L3 visibility and bloom. Gameplay code, simulation state
    /// and costs are untouched — presets are presentation-only.
    /// </summary>
    public static class PresetApplier
    {
        /// <summary>Render scale on the LOW tier (PERF-001).</summary>
        public const float LowRenderScale = 0.75f;

        /// <summary>Render scale on MEDIUM and HIGH (PERF-001).</summary>
        public const float StandardRenderScale = 1.0f;

        /// <summary>
        /// Presentation sinks the preset drives. Each sink is an injectable
        /// delegate so tests verify application without engine objects.
        /// </summary>
        public sealed class Sinks
        {
            /// <summary>Applies <see cref="QualitySettings.SetQualityLevel"/> (preset ordinal).</summary>
            public Action<int>? SetQualityLevel;

            /// <summary>Applies the URP asset render scale.</summary>
            public Action<float>? SetRenderScale;

            /// <summary>Applies <see cref="PointLightBudget.ApplyBudget"/> with limit + view center.</summary>
            public Action<int, Vector2>? ApplyLightBudget;

            /// <summary>Applies the preset particle ceiling to <see cref="ParticleBudget"/>.</summary>
            public Action<int>? SetParticleCeiling;

            /// <summary>Shows/hides the parallax L3 layer.</summary>
            public Action<bool>? SetParallaxL3Visible;

            /// <summary>Enables/disables the bloom override.</summary>
            public Action<bool>? SetBloomEnabled;
        }

        /// <summary>Render scale for a preset: 0.75 on LOW, else 1.0.</summary>
        public static float RenderScaleFor(QualityPreset preset)
        {
            return preset == QualityPreset.Low ? LowRenderScale : StandardRenderScale;
        }

        /// <summary>Active point-light budget for a preset: 4/8/16.</summary>
        public static int PointLightLimitFor(QualityPreset preset)
        {
            switch (preset)
            {
                case QualityPreset.Low:
                    return 4;
                case QualityPreset.Medium:
                    return 8;
                default:
                    return 16;
            }
        }

        /// <summary>Aggregate live-particle ceiling for a preset: 512/1024/2048.</summary>
        public static int ParticleCeilingFor(QualityPreset preset)
        {
            switch (preset)
            {
                case QualityPreset.Low:
                    return 512;
                case QualityPreset.Medium:
                    return 1024;
                default:
                    return 2048;
            }
        }

        /// <summary>
        /// Applies the full preset table: quality level ordinal, render
        /// scale, light budget at the current view center, particle
        /// ceiling, parallax L3 hidden on LOW and bloom only on HIGH.
        /// </summary>
        public static void Apply(QualityPreset preset, Sinks sinks, Vector2 viewCenter)
        {
            if (sinks == null)
            {
                throw new ArgumentNullException(nameof(sinks));
            }

            Action<int>? setQualityLevel = sinks.SetQualityLevel;
            if (setQualityLevel != null)
            {
                setQualityLevel((int)preset);
            }

            Action<float>? setRenderScale = sinks.SetRenderScale;
            if (setRenderScale != null)
            {
                setRenderScale(RenderScaleFor(preset));
            }

            Action<int, Vector2>? applyLightBudget = sinks.ApplyLightBudget;
            if (applyLightBudget != null)
            {
                applyLightBudget(PointLightLimitFor(preset), viewCenter);
            }

            Action<int>? setParticleCeiling = sinks.SetParticleCeiling;
            if (setParticleCeiling != null)
            {
                setParticleCeiling(ParticleCeilingFor(preset));
            }

            Action<bool>? setParallaxL3 = sinks.SetParallaxL3Visible;
            if (setParallaxL3 != null)
            {
                setParallaxL3(preset != QualityPreset.Low);
            }

            Action<bool>? setBloom = sinks.SetBloomEnabled;
            if (setBloom != null)
            {
                setBloom(preset == QualityPreset.High);
            }
        }
    }
}
