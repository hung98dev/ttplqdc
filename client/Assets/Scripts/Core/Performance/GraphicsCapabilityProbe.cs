using System;
using UnityEngine;
using UnityEngine.Rendering;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Graphics capability probe for the performance category
    /// (client_performance.md Measurement and Gates): a failed prerequisite
    /// (no graphics device, unsupported RFloat readback) is a hard failure,
    /// never a skip. The Unity job must run with a real device.
    /// </summary>
    public static class GraphicsCapabilityProbe
    {
        /// <summary>
        /// Throws <see cref="InvalidOperationException"/> when the current
        /// invocation cannot run the performance category.
        /// </summary>
        public static void VerifyCurrentInvocation()
        {
            GraphicsDeviceType deviceType = SystemInfo.graphicsDeviceType;
            if (deviceType == GraphicsDeviceType.Null)
            {
                throw new InvalidOperationException(
                    "Performance gates require a graphics device: " +
                    "run Unity with -force-d3d11 (WARP/URP capability per " +
                    "presentation_asset_manifest.md §3.3a)");
            }

            if (!SystemInfo.SupportsRenderTextureFormat(
                UnityEngine.RenderTextureFormat.RFloat))
            {
                throw new InvalidOperationException(
                    "graphics device lacks RFloat readback — PERF-016 " +
                    "overdraw measurement cannot run");
            }
        }
    }
}
