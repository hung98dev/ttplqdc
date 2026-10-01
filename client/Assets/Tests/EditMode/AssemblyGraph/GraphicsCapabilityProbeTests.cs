using NUnit.Framework;
using ThinhThan.Core.Assets.Editor;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// Probe seam tests (presentation_asset_manifest.md §3.3a): WARP
    /// detection, lit-fixture luminance delta, and RFloat overlap readback.
    /// </summary>
    public class GraphicsCapabilityProbeTests
    {
        [Test]
        public void TestProbeAdapterDetectionWarp()
        {
            var warp = new GraphicsCapabilityProbe.GraphicsAdapterInfo
            {
                Name = "Microsoft Basic Render Driver",
                VendorId = 0x1414,
                DeviceId = 0x008c,
                SoftwareFlag = true,
            };
            Assert.IsTrue(GraphicsCapabilityProbe.IsSoftwareAdapter(warp), "WARP ids");

            var warpByName = new GraphicsCapabilityProbe.GraphicsAdapterInfo
            {
                Name = "Microsoft Basic Render Driver",
                VendorId = 0x1414,
                DeviceId = 0x0000,
                SoftwareFlag = true,
            };
            Assert.IsTrue(GraphicsCapabilityProbe.IsSoftwareAdapter(warpByName), "flag + WARP name");

            var nameOnly = new GraphicsCapabilityProbe.GraphicsAdapterInfo
            {
                Name = "Microsoft Basic Render Driver",
                VendorId = 0x10de,
                DeviceId = 0x2204,
                SoftwareFlag = false,
            };
            Assert.IsFalse(
                GraphicsCapabilityProbe.IsSoftwareAdapter(nameOnly),
                "generic name without software flag is not proof");

            var hardware = new GraphicsCapabilityProbe.GraphicsAdapterInfo
            {
                Name = "NVIDIA GeForce RTX 3060",
                VendorId = 0x10de,
                DeviceId = 0x2504,
                SoftwareFlag = false,
            };
            Assert.IsFalse(GraphicsCapabilityProbe.IsSoftwareAdapter(hardware), "hardware adapter");

            var noName = new GraphicsCapabilityProbe.GraphicsAdapterInfo
            {
                Name = null,
                VendorId = 0x1414,
                DeviceId = 0x0000,
                SoftwareFlag = true,
            };
            Assert.IsFalse(GraphicsCapabilityProbe.IsSoftwareAdapter(noName), "ambiguous adapter");
        }

        [Test]
        public void TestProbeSpriteLitDayNightLuminanceChange()
        {
            Assert.IsTrue(
                GraphicsCapabilityProbe.LuminanceDeltaSufficient(80f, 0f),
                "lit delta >= 5");
            Assert.IsFalse(
                GraphicsCapabilityProbe.LuminanceDeltaSufficient(4.9f, 0f),
                "delta below 5");
            Assert.IsFalse(
                GraphicsCapabilityProbe.LuminanceDeltaSufficient(0f, 0f),
                "blank output rejected");
            Assert.IsFalse(
                GraphicsCapabilityProbe.LuminanceDeltaSufficient(float.NaN, 0f),
                "NaN day rejected");
            Assert.IsFalse(
                GraphicsCapabilityProbe.LuminanceDeltaSufficient(80f, float.NaN),
                "NaN night rejected");
        }

        [Test]
        public void TestProbeRFloatOverlapReadback()
        {
            var pixels = new float[16 * 16];
            for (int y = 0; y < 16; y++)
            {
                for (int x = 0; x < 16; x++)
                {
                    pixels[y * 16 + x] = x < 8 ? 2f : 1f;
                }
            }
            Assert.IsTrue(GraphicsCapabilityProbe.ValidateOverlapReadback(pixels), "exact fixture");

            // float32 cannot place a value exactly 0.001 under 2f — 1.999f
            // rounds to a diff slightly above the tolerance — so the inside
            // check uses a representable value comfortably within it.
            pixels[0] = 1.9995f;
            Assert.IsTrue(
                GraphicsCapabilityProbe.ValidateOverlapReadback(pixels),
                "error within tolerance");

            pixels[0] = 1.9f;
            Assert.IsFalse(
                GraphicsCapabilityProbe.ValidateOverlapReadback(pixels),
                "error beyond tolerance");

            pixels[0] = float.NaN;
            Assert.IsFalse(
                GraphicsCapabilityProbe.ValidateOverlapReadback(pixels),
                "NaN rejected");

            Assert.IsFalse(
                GraphicsCapabilityProbe.ValidateOverlapReadback(new float[16]),
                "wrong size rejected");
        }
    }
}
