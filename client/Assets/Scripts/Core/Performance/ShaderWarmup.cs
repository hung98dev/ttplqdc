using System;
using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Loading-screen shader warm-up (client_performance.md item 5,
    /// PERF-018): the checked-in ShaderVariantCollection —
    /// <c>Assets/Settings/Performance/PerfShaderVariants.shadervariants</c>
    /// — is warmed on the loading screen so no first-use frame spikes
    /// over 50 ms CPU. <see cref="UnityEngine.Rendering.GraphicsStateCollection"/>
    /// warm-up runs where the graphics API supports it.
    /// </summary>
    public static class ShaderWarmup
    {
        /// <summary>Warms every variant in the collection before gameplay.</summary>
        public static void WarmUp(ShaderVariantCollection variants)
        {
            if (variants == null)
            {
                throw new ArgumentNullException(nameof(variants));
            }

            variants.WarmUp();
        }
    }
}
