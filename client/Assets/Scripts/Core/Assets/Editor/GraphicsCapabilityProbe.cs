using System;
using System.Diagnostics;
using System.IO;
using System.Text;
using UnityEngine;
using UnityEngine.Rendering;

namespace ThinhThan.Core.Assets.Editor
{
    /// <summary>
    /// Observed graphics capability probe (presentation_asset_manifest.md
    /// §3.3a). Every graphical editor invocation calls
    /// <see cref="VerifyCurrentInvocation"/> before its review or performance
    /// results are admitted. The hosted CI path renders on the D3D11 WARP
    /// software adapter; a null device, an ambiguous match or a hardware
    /// adapter all fail. The probe never fabricates adapter metadata and
    /// always writes its report, including on failure.
    /// </summary>
    public static class GraphicsCapabilityProbe
    {
        /// <summary>Observed adapter identity (SystemInfo + platform
        /// diagnostics).</summary>
        public struct GraphicsAdapterInfo
        {
            public string? Name;
            public int VendorId;
            public int DeviceId;
            public string? PnpDeviceId;
            public bool SoftwareFlag;
        }

        /// <summary>Probe run failure — the job fails closed.</summary>
        public sealed class GraphicsProbeException : Exception
        {
            public GraphicsProbeException(string message) : base(message)
            {
            }
        }

        /// <summary>JSON-serializable probe report written to reportPath.</summary>
        public sealed class Report
        {
            public string EditorVersion = string.Empty;
            public string GraphicsDeviceType = string.Empty;
            public string DeviceName = string.Empty;
            public string DeviceVendor = string.Empty;
            public int VendorId;
            public int DeviceId;
            public string AdapterPnpId = string.Empty;
            public bool Warp;
            public bool LitFixtureOk;
            public bool RFloatReadbackOk;
            public string Failure = string.Empty;
        }

        private const float LitDeltaMin = 5f;
        private const float OverlapTolerance = 0.001f;
        private const int OverlapWidth = 16;
        private const int OverlapHeight = 16;

        /// <summary>
        /// The adapter must match the Microsoft Basic Render Driver / WARP
        /// software adapter. A generic device name alone is not proof: the
        /// vendor/device IDs of the initialized device must match the
        /// enumerated software adapter (Microsoft 0x1414 / MBRD 0x008c) or
        /// carry the DXGI software flag with a WARP-named description.
        /// </summary>
        public static bool IsSoftwareAdapter(GraphicsAdapterInfo adapter)
        {
            if (adapter.VendorId == 0x1414 && adapter.DeviceId == 0x008c)
            {
                return true;
            }
            if (!adapter.SoftwareFlag || adapter.Name == null)
            {
                return false;
            }
            return adapter.Name.IndexOf("Microsoft Basic Render Driver", StringComparison.OrdinalIgnoreCase) >= 0
                || adapter.Name.IndexOf("WARP", StringComparison.OrdinalIgnoreCase) >= 0;
        }

        /// <summary>
        /// The lit fixture must show a real luminance change: day mean L*
        /// exceeds night by at least 5, both finite, day non-zero.
        /// </summary>
        public static bool LuminanceDeltaSufficient(float dayMean, float nightMean)
        {
            return float.IsFinite(dayMean) && float.IsFinite(nightMean)
                && dayMean > 0f
                && dayMean - nightMean >= LitDeltaMin;
        }

        /// <summary>
        /// 16x16 RFloat overlap fixture: clear=0, one full layer=1, a second
        /// layer over the left 8 columns → left=2, right=1, tolerance 0.001.
        /// pixels is row-major, y*width+x.
        /// </summary>
        public static bool ValidateOverlapReadback(float[] pixels)
        {
            if (pixels == null || pixels.Length != OverlapWidth * OverlapHeight)
            {
                return false;
            }
            for (int y = 0; y < OverlapHeight; y++)
            {
                for (int x = 0; x < OverlapWidth; x++)
                {
                    float v = pixels[y * OverlapWidth + x];
                    if (!float.IsFinite(v))
                    {
                        return false;
                    }
                    float want = x < OverlapWidth / 2 ? 2f : 1f;
                    if (Math.Abs(v - want) > OverlapTolerance)
                    {
                        return false;
                    }
                }
            }
            return true;
        }

        /// <summary>
        /// Run the full probe and write reportPath (always written, including
        /// on failure). Throws GraphicsProbeException on any failed
        /// observation.
        /// </summary>
        public static void VerifyCurrentInvocation(string reportPath)
        {
            var report = new Report
            {
                EditorVersion = Application.unityVersion,
                GraphicsDeviceType = SystemInfo.graphicsDeviceType.ToString(),
                DeviceName = SystemInfo.graphicsDeviceName,
                DeviceVendor = SystemInfo.graphicsDeviceVendor,
                VendorId = SystemInfo.graphicsDeviceVendorID,
                DeviceId = SystemInfo.graphicsDeviceID,
            };
            try
            {
                if (SystemInfo.graphicsDeviceType == GraphicsDeviceType.Null)
                {
                    throw new GraphicsProbeException("null graphics device");
                }
                var adapter = ObserveAdapter();
                report.AdapterPnpId = adapter.PnpDeviceId ?? string.Empty;
                report.Warp = IsSoftwareAdapter(adapter);
                if (!report.Warp)
                {
                    throw new GraphicsProbeException(
                        "initialized device does not match the software WARP adapter");
                }
                RenderLitFixture(report);
                RenderOverlapFixture(report);
            }
            catch (Exception ex)
            {
                report.Failure = ex.Message;
                WriteReport(reportPath, report);
                if (ex is GraphicsProbeException)
                {
                    throw;
                }
                throw new GraphicsProbeException(ex.Message);
            }
            WriteReport(reportPath, report);
            if (report.Failure.Length != 0)
            {
                throw new GraphicsProbeException(report.Failure);
            }
        }

        /// <summary>
        /// Windows platform diagnostics: enumerate the initialized adapter's
        /// PNP ID (which encodes VEN_/DEV_) and confirm it is the software
        /// Basic Render Driver. Non-Windows or ambiguous output fails.
        /// </summary>
        private static GraphicsAdapterInfo ObserveAdapter()
        {
            if (!Application.platform.ToString().Contains("Windows"))
            {
                throw new GraphicsProbeException("probe requires the native Windows editor");
            }
            var info = new GraphicsAdapterInfo();
            string output = RunDiagnostics(
                "powershell.exe",
                "-NoProfile -Command \"Get-CimInstance Win32_VideoController | " +
                "ForEach-Object { $_.Name + '|' + $_.PNPDeviceID }\"");
            foreach (var line in output.Split('\n'))
            {
                var parts = line.Trim().Split('|');
                if (parts.Length != 2)
                {
                    continue;
                }
                var parsed = ParsePnpVendorDevice(parts[1]);
                if (parsed.vendor == SystemInfo.graphicsDeviceVendorID
                    && parsed.device == SystemInfo.graphicsDeviceID)
                {
                    info.Name = parts[0];
                    info.VendorId = parsed.vendor;
                    info.DeviceId = parsed.device;
                    info.PnpDeviceId = parts[1];
                    info.SoftwareFlag = info.VendorId == 0x1414;
                    return info;
                }
            }
            throw new GraphicsProbeException("no enumerated adapter matches the initialized device");
        }

        private static (int vendor, int device) ParsePnpVendorDevice(string pnp)
        {
            int vendor = ParseHexField(pnp, "VEN_");
            int device = ParseHexField(pnp, "DEV_");
            return (vendor, device);
        }

        private static int ParseHexField(string s, string marker)
        {
            int i = s.IndexOf(marker, StringComparison.OrdinalIgnoreCase);
            if (i < 0)
            {
                return -1;
            }
            int j = i + marker.Length;
            int k = j;
            while (k < s.Length && Uri.IsHexDigit(s[k]))
            {
                k++;
            }
            return Convert.ToInt32(s.Substring(j, k - j), 16);
        }

        private static string RunDiagnostics(string exe, string args)
        {
            var psi = new ProcessStartInfo(exe, args)
            {
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            using (var p = Process.Start(psi)
                ?? throw new GraphicsProbeException("failed to start platform diagnostics"))
            {
                string outp = p.StandardOutput.ReadToEnd();
                if (!p.WaitForExit(30000))
                {
                    throw new GraphicsProbeException("platform diagnostics timed out");
                }
                if (p.ExitCode != 0)
                {
                    throw new GraphicsProbeException("platform diagnostics exit " + p.ExitCode);
                }
                return outp;
            }
        }

        /// <summary>
        /// Light2D lives in the URP 2D package assembly, which this
        /// assembly does not reference (the Mandatory Assemblies table is
        /// exact); the fixture binds the component late and fails closed if
        /// the type or its members are absent.
        /// </summary>
        private static Type FindLight2DType()
        {
            var t = Type.GetType(
                    "UnityEngine.Rendering.Universal.Light2D, Unity.RenderPipelines.Universal.2D.Runtime")
                ?? Type.GetType(
                    "UnityEngine.Rendering.Universal.Light2D, Unity.RenderPipelines.Universal.Runtime");
            if (t == null)
            {
                throw new GraphicsProbeException("Light2D type not found");
            }
            return t;
        }

        private static void SetLight2DGlobal(Component light)
        {
            var t = light.GetType();
            var lt = t.GetNestedType("LightType")
                ?? throw new GraphicsProbeException("Light2D.LightType not found");
            var prop = t.GetProperty("lightType")
                ?? throw new GraphicsProbeException("Light2D.lightType not found");
            prop.SetValue(light, Enum.Parse(lt, "Global"));
        }

        /// <summary>
        /// Render the Sprite-Lit fixture under day then night global Light2D
        /// and require the day mean luminance to exceed night by >= 5.
        /// </summary>
        private static void RenderLitFixture(Report report)
        {
            var rt = new RenderTexture(OverlapWidth, OverlapHeight, 24, RenderTextureFormat.RFloat);
            var camGo = new GameObject("ProbeCam");
            var lightGo = new GameObject("ProbeLight");
            var quadGo = new GameObject("ProbeQuad");
            var cam = camGo.AddComponent<Camera>();
            try
            {
                rt.Create();
                cam.orthographic = true;
                cam.orthographicSize = 1f;
                cam.transform.position = new Vector3(0f, 0f, -10f);
                cam.clearFlags = CameraClearFlags.SolidColor;
                cam.backgroundColor = Color.black;
                cam.targetTexture = rt;

                var light2dType = FindLight2DType();
                var light = lightGo.AddComponent(light2dType);
                SetLight2DGlobal(light);
                var intensity = light2dType.GetProperty("intensity")
                    ?? throw new GraphicsProbeException("Light2D.intensity property not found");

                var sr = quadGo.AddComponent<SpriteRenderer>();
                sr.sprite = WhiteSprite();
                sr.material = new Material(Shader.Find("Universal Render Pipeline/2D/Sprite-Lit-Default"));

                intensity.SetValue(light, 1f);
                float day = RenderAndMeanLuminance(cam, rt);
                intensity.SetValue(light, 0f);
                float night = RenderAndMeanLuminance(cam, rt);
                report.LitFixtureOk = LuminanceDeltaSufficient(day, night);
                if (!report.LitFixtureOk)
                {
                    throw new GraphicsProbeException(
                        $"lit fixture delta insufficient: day={day} night={night}");
                }
            }
            finally
            {
                cam.targetTexture = null;
                UnityEngine.Object.DestroyImmediate(quadGo);
                UnityEngine.Object.DestroyImmediate(lightGo);
                UnityEngine.Object.DestroyImmediate(camGo);
                UnityEngine.Object.DestroyImmediate(rt);
            }
        }

        /// <summary>
        /// 16x16 RFloat overlap fixture: clear=0, additive layer1=1 across the
        /// target, layer2=1 over the left 8 columns; readback left=2/right=1.
        /// </summary>
        private static void RenderOverlapFixture(Report report)
        {
            var rt = new RenderTexture(OverlapWidth, OverlapHeight, 0, RenderTextureFormat.RFloat);
            var camGo = new GameObject("ProbeCamR");
            var full = new GameObject("ProbeFull");
            var half = new GameObject("ProbeHalf");
            var cam = camGo.AddComponent<Camera>();
            try
            {
                rt.Create();
                cam.orthographic = true;
                cam.orthographicSize = 1f;
                cam.transform.position = new Vector3(0f, 0f, -10f);
                cam.clearFlags = CameraClearFlags.SolidColor;
                cam.backgroundColor = Color.black;
                cam.targetTexture = rt;

                var mat = AdditiveWhiteMaterial();
                var quad = MeshQuad();
                SetupQuad(full, quad, mat, new Vector3(0f, 0f, 0f), new Vector3(2f, 2f, 1f));
                SetupQuad(half, quad, mat, new Vector3(-0.5f, 0f, 0f), new Vector3(1f, 2f, 1f));

                cam.Render();
                float[] pixels = ReadbackFloatPixels(rt);
                report.RFloatReadbackOk = ValidateOverlapReadback(pixels);
                if (!report.RFloatReadbackOk)
                {
                    throw new GraphicsProbeException("RFloat overlap readback failed");
                }
            }
            finally
            {
                cam.targetTexture = null;
                UnityEngine.Object.DestroyImmediate(half);
                UnityEngine.Object.DestroyImmediate(full);
                UnityEngine.Object.DestroyImmediate(camGo);
                UnityEngine.Object.DestroyImmediate(rt);
            }
        }

        private static void SetupQuad(GameObject go, Mesh quad, Material mat, Vector3 pos, Vector3 scale)
        {
            var mf = go.AddComponent<MeshFilter>();
            mf.sharedMesh = quad;
            var mr = go.AddComponent<MeshRenderer>();
            mr.sharedMaterial = mat;
            go.transform.position = pos;
            go.transform.localScale = scale;
        }

        private static Material AdditiveWhiteMaterial()
        {
            var mat = new Material(Shader.Find("Universal Render Pipeline/Unlit"));
            mat.SetFloat("_Surface", 0f);
            mat.SetFloat("_SrcBlend", (float)BlendMode.One);
            mat.SetFloat("_DstBlend", (float)BlendMode.One);
            mat.SetColor("_BaseColor", Color.white);
            return mat;
        }

        private static Mesh? _quad;
        private static Mesh MeshQuad()
        {
            if (_quad == null)
            {
                var go = GameObject.CreatePrimitive(PrimitiveType.Quad);
                var mesh = go.GetComponent<MeshFilter>()?.sharedMesh;
                UnityEngine.Object.DestroyImmediate(go);
                if (mesh == null)
                {
                    throw new GraphicsProbeException("quad mesh unavailable");
                }
                _quad = mesh;
            }
            return _quad;
        }

        private static Sprite? _whiteSprite;
        private static Sprite WhiteSprite()
        {
            if (_whiteSprite == null)
            {
                var tex = new Texture2D(4, 4, TextureFormat.RGBA32, false);
                var px = new Color[16];
                for (int i = 0; i < px.Length; i++)
                {
                    px[i] = Color.white;
                }
                tex.SetPixels(px);
                tex.Apply();
                _whiteSprite = Sprite.Create(tex, new Rect(0, 0, 4, 4), new Vector2(0.5f, 0.5f));
            }
            return _whiteSprite;
        }

        private static float RenderAndMeanLuminance(Camera cam, RenderTexture rt)
        {
            cam.Render();
            var prev = RenderTexture.active;
            RenderTexture.active = rt;
            var tex = new Texture2D(rt.width, rt.height, TextureFormat.RFloat, false);
            tex.ReadPixels(new Rect(0, 0, rt.width, rt.height), 0, 0);
            tex.Apply();
            RenderTexture.active = prev;
            var px = tex.GetPixels();
            UnityEngine.Object.DestroyImmediate(tex);
            double sum = 0;
            for (int i = 0; i < px.Length; i++)
            {
                sum += 0.2126 * px[i].r + 0.7152 * px[i].g + 0.0722 * px[i].b;
            }
            return (float)(100.0 * sum / px.Length);
        }

        private static float[] ReadbackFloatPixels(RenderTexture rt)
        {
            var prev = RenderTexture.active;
            RenderTexture.active = rt;
            var tex = new Texture2D(rt.width, rt.height, TextureFormat.RFloat, false);
            tex.ReadPixels(new Rect(0, 0, rt.width, rt.height), 0, 0);
            tex.Apply();
            RenderTexture.active = prev;
            var px = tex.GetPixels();
            var outp = new float[px.Length];
            for (int i = 0; i < px.Length; i++)
            {
                outp[i] = px[i].r;
            }
            UnityEngine.Object.DestroyImmediate(tex);
            return outp;
        }

        private static void WriteReport(string path, Report report)
        {
            var sb = new StringBuilder(256);
            sb.Append('{')
                .Append("\"editorVersion\":").Append(Json(report.EditorVersion)).Append(',')
                .Append("\"graphicsDeviceType\":").Append(Json(report.GraphicsDeviceType)).Append(',')
                .Append("\"deviceName\":").Append(Json(report.DeviceName)).Append(',')
                .Append("\"deviceVendor\":").Append(Json(report.DeviceVendor)).Append(',')
                .Append("\"vendorId\":").Append(report.VendorId).Append(',')
                .Append("\"deviceId\":").Append(report.DeviceId).Append(',')
                .Append("\"adapterPnpId\":").Append(Json(report.AdapterPnpId)).Append(',')
                .Append("\"warp\":").Append(report.Warp ? "true" : "false").Append(',')
                .Append("\"litFixtureOk\":").Append(report.LitFixtureOk ? "true" : "false").Append(',')
                .Append("\"rfloatReadbackOk\":").Append(report.RFloatReadbackOk ? "true" : "false").Append(',')
                .Append("\"failure\":").Append(Json(report.Failure))
                .Append('}');
            var dir = Path.GetDirectoryName(path);
            if (!string.IsNullOrEmpty(dir))
            {
                Directory.CreateDirectory(dir);
            }
            File.WriteAllText(path, sb.ToString() + "\n");
        }

        private static string Json(string? s)
        {
            if (s == null)
            {
                return "null";
            }
            return "\"" + s.Replace("\\", "\\\\").Replace("\"", "\\\"") + "\"";
        }
    }
}
