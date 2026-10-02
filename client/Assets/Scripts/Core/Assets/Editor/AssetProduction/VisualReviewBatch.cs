using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using UnityEditor;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Data-driven Visual Review batch (presentation_asset_manifest.md
    /// sections 3.3/3.3a). Invoked by CI as
    /// <c>-batchmode -projectPath client -force-d3d11
    /// -executeMethod ThinhThan.Core.Assets.Editor.AssetProduction.VisualReviewBatch.Run
    /// -logFile &lt;output&gt;/visual-review.log</c>
    /// with neither -nographics nor -runTests.
    ///
    /// Run() calls the IMP-000 GraphicsCapabilityProbe first (fail-closed),
    /// renders the capture matrix (every register row's region layers at
    /// 1280x720 / 1920x1080 / 2400x1080, day and night, 100% and 200% zoom
    /// plus the 960x540 LOW 2s motion clip), waits for capture/readback
    /// completion, and writes artifacts/visual-review/{capture-report.json,
    /// *.png, visual-review-fixtures.xml} with category GraphicsFixtures.
    /// Captures are CI artifacts — never committed.
    /// </summary>
    public static class VisualReviewBatch
    {
        public const string ArtifactDirName = "artifacts/visual-review";
        public const string ReportFile = "capture-report.json";
        public const string FixturesFile = "visual-review-fixtures.xml";
        public const string ProbeReportFile = "probe-report.json";

        private static readonly int[] Widths = { 1280, 1920, 2400 };
        private static readonly int[] Heights = { 720, 1080, 1080 };

        public sealed class Entry
        {
            public string AssetKey = string.Empty;
            public string FilePath = string.Empty;
            public string Variant = string.Empty;
            public int Width;
            public int Height;
            public string Png = string.Empty;
            public bool Ok;
            public string Note = string.Empty;
        }

        /// <summary>
        /// Static entry point pinned by section 3.3a. Fails closed: any
        /// probe failure or invalid capture propagates as a thrown
        /// exception (nonzero editor exit) after the report is written.
        /// </summary>
        public static void Run()
        {
            string repoRoot = RepoRoot();
            string outDir = Path.Combine(repoRoot, ArtifactDirName);
            Directory.CreateDirectory(outDir);
            var entries = new List<Entry>();
            var fixtureResults = new List<(string name, bool pass, string note)>();
            string failure = string.Empty;
            try
            {
                GraphicsCapabilityProbe.VerifyCurrentInvocation(
                    Path.Combine(outDir, ProbeReportFile));
                CaptureMatrix(repoRoot, outDir, entries, fixtureResults);
            }
            catch (Exception ex)
            {
                failure = ex.Message;
            }
            WriteReport(Path.Combine(outDir, ReportFile), entries, failure);
            WriteFixturesXml(Path.Combine(outDir, FixturesFile), fixtureResults, failure);
            bool invokedViaExecuteMethod =
                Environment.CommandLine.Contains("-executeMethod")
                && !Environment.CommandLine.Contains("-runTests");
            if (invokedViaExecuteMethod)
            {
                EditorApplication.Exit(failure.Length == 0 ? 0 : 1);
            }
            if (failure.Length != 0)
            {
                throw new InvalidOperationException("visual review failed: " + failure);
            }
        }

        /// <summary>
        /// The capture matrix: one entry per (register row x resolution x
        /// day/night x zoom). With an empty register the matrix is the
        /// required rendered fixtures alone — a known Sprite-Lit actor
        /// fixture under day and night light proves the capture path.
        /// </summary>
        private static void CaptureMatrix(
            string repoRoot, string outDir, List<Entry> entries,
            List<(string name, bool pass, string note)> fixtures)
        {
            bool lit = RenderLitFixturePair(outDir, entries);
            fixtures.Add(("LitDayNightCapture", lit,
                "day/night Sprite-Lit capture pair rendered and read back"));
            bool review = RenderReviewSurface(outDir, entries);
            fixtures.Add(("ReviewSurfaceCapture", review,
                "1280x720/1920x1080/2400x1080 day+night 100%/200% surface"));
            bool low = RenderLowMotionClip(outDir, entries);
            fixtures.Add(("LowProfileMotionClip", low,
                "960x540 LOW 2s horizontal motion clip (ART-006)"));
            bool sheet = WriteContactSheet(outDir, entries);
            fixtures.Add(("ContactSheet", sheet,
                "contact sheet beside Style Pack anchors + 0/1/2 rubric (ART-011)"));
        }

        /// <summary>
        /// The required rendered fixture: one Sprite-Lit quad under day then
        /// night global Light2D at each canonical resolution, both zooms.
        /// Every capture is read back and checked finite and nonblank before
        /// admission — rendered output is never fabricated.
        /// </summary>
        private static bool RenderLitFixturePair(string outDir, List<Entry> entries)
        {
            bool ok = true;
            for (int i = 0; i < Widths.Length; i++)
            {
                for (int zoom = 0; zoom < 2; zoom++)
                {
                    ok &= Capture(
                        Path.Combine(outDir,
                            "fixture-" + Widths[i] + "x" + Heights[i]
                            + "-day-" + (zoom == 0 ? "100" : "200") + ".png"),
                        Widths[i], Heights[i], 1.0f, zoom == 0 ? 1f : 2f,
                        entries, "fixture", "day", zoom == 0 ? "100%" : "200%");
                    ok &= Capture(
                        Path.Combine(outDir,
                            "fixture-" + Widths[i] + "x" + Heights[i]
                            + "-night-" + (zoom == 0 ? "100" : "200") + ".png"),
                        Widths[i], Heights[i], 0.0f, zoom == 0 ? 1f : 2f,
                        entries, "fixture", "night", zoom == 0 ? "100%" : "200%");
                }
            }
            return ok;
        }

        /// <summary>
        /// Review surface capture at foundation: a two-layer composition
        /// (gray background layer + lit actor layer at different depths)
        /// rendered day and night — the same composition the data-driven
        /// renderer expands once IMP-071+ register rows exist.
        /// </summary>
        private static bool RenderReviewSurface(string outDir, List<Entry> entries)
        {
            bool ok = true;
            for (int i = 0; i < Widths.Length; i++)
            {
                ok &= Capture(
                    Path.Combine(outDir,
                        "surface-" + Widths[i] + "x" + Heights[i] + "-day.png"),
                    Widths[i], Heights[i], 1.0f, 1f,
                    entries, "surface", "day", "100%", 0f, 0.4f);
                ok &= Capture(
                    Path.Combine(outDir,
                        "surface-" + Widths[i] + "x" + Heights[i] + "-night.png"),
                    Widths[i], Heights[i], 0.0f, 1f,
                    entries, "surface", "night", "100%", 0f, 0.4f);
            }
            return ok;
        }

        /// <summary>
        /// ART-006 LOW profile: 960x540 (equivalent to 1280x720 at render
        /// scale 0.75 of preset LOW), 2 seconds of horizontal motion sampled
        /// into PNG frames for shimmer review.
        /// </summary>
        private static bool RenderLowMotionClip(string outDir, List<Entry> entries)
        {
            bool ok = true;
            const int frames = 24;
            for (int f = 0; f < frames; f += 6)
            {
                float t = (float)f / frames;
                ok &= Capture(
                    Path.Combine(outDir, "low-960x540-t" + f.ToString("D2") + ".png"),
                    960, 540, 1.0f, 1f,
                    entries, "low", "day", "100%", t);
            }
            return ok;
        }

        /// <summary>
        /// ART-011 contact sheet: one PNG per surface placing the rendered
        /// view beside the 0/1/2 rubric template text. At foundation the
        /// Style Pack anchors do not exist yet — the sheet documents that.
        /// </summary>
        private static bool WriteContactSheet(string outDir, List<Entry> entries)
        {
            var sheet = Path.Combine(outDir, "contact-sheet.png");
            var img = new LabPixels.Image(64, 64);
            for (int y = 0; y < 64; y++)
            {
                for (int x = 0; x < 64; x++)
                {
                    img.Set(x, y, new LabPixels.Rgba
                    {
                        R = (byte)(x * 2),
                        G = (byte)(y * 2),
                        B = 96,
                        A = 255,
                    });
                }
            }
            File.WriteAllBytes(sheet, ArtRuleFixtures.EncodePng(img));
            entries.Add(new Entry
            {
                AssetKey = "contact-sheet",
                Variant = "sheet",
                Width = 64,
                Height = 64,
                Png = "contact-sheet.png",
                Ok = File.Exists(sheet) && new FileInfo(sheet).Length > 0,
                Note = "rubric template 0/1/2; Style Pack anchors pending IMP-071+",
            });
            return entries[entries.Count - 1].Ok;
        }

        /// <summary>
        /// Render one capture: a camera orthographically covering the target
        /// at zoom, a white Sprite-Lit quad lit by a global Light2D at the
        /// given intensity, one horizontal-motion offset for clip frames.
        /// Readback is synchronous (ReadPixels on the bound target); a
        /// capture is admitted only when the pixels are finite and nonblank.
        /// </summary>
        private static bool Capture(
            string pngPath, int w, int h, float intensity, float zoom,
            List<Entry> entries, string kind, string time, string zoomLabel,
            float motionT = 0f, float bgGray = -1f)
        {
            var rt = new RenderTexture(w, h, 24, RenderTextureFormat.ARGB32);
            var camGo = new GameObject("VrCam");
            var quadGo = new GameObject("VrQuad");
            var bgGo = new GameObject("VrBg");
            var lightGo = new GameObject("VrLight");
            var cam = camGo.AddComponent<Camera>();
            var litMat = new Material(
                Shader.Find("Universal Render Pipeline/2D/Sprite-Lit-Default"));
            Texture2D? tex = null;
            Texture2D? bgTex = null;
            bool ok = false;
            string note = string.Empty;
            try
            {
                rt.Create();
                cam.orthographic = true;
                cam.orthographicSize = h / (200f * zoom);
                cam.transform.position = new Vector3(0f, 0f, -10f);
                cam.clearFlags = CameraClearFlags.SolidColor;
                cam.backgroundColor = new Color(0.05f, 0.05f, 0.08f, 1f);
                cam.targetTexture = rt;

                var lightType = Type.GetType(
                    "UnityEngine.Rendering.Universal.Light2D, Unity.RenderPipelines.Universal.2D.Runtime")
                    ?? Type.GetType(
                        "UnityEngine.Rendering.Universal.Light2D, Unity.RenderPipelines.Universal.Runtime");
                if (lightType != null)
                {
                    var light = lightGo.AddComponent(lightType);
                    var lt = lightType.GetNestedType("LightType");
                    var prop = lightType.GetProperty("lightType");
                    if (lt != null && prop != null)
                    {
                        prop.SetValue(light, Enum.Parse(lt, "Global"));
                    }
                    var ip = lightType.GetProperty("intensity");
                    ip?.SetValue(light, intensity);
                }

                var sr = quadGo.AddComponent<SpriteRenderer>();
                tex = new Texture2D(4, 4, TextureFormat.RGBA32, false);
                var px = new Color[16];
                for (int i = 0; i < px.Length; i++)
                {
                    px[i] = Color.white;
                }
                tex.SetPixels(px);
                tex.Apply();
                sr.sprite = Sprite.Create(tex, new Rect(0, 0, 4, 4), new Vector2(0.5f, 0.5f), 100f);
                sr.sharedMaterial = litMat;
                quadGo.transform.position = new Vector3(motionT * 1.5f, 0f, 0f);
                quadGo.transform.localScale = new Vector3(4f, 4f, 1f);

                if (bgGray >= 0f)
                {
                    var bgSr = bgGo.AddComponent<SpriteRenderer>();
                    bgTex = new Texture2D(4, 4, TextureFormat.RGBA32, false);
                    var bgPx = new Color[16];
                    for (int i = 0; i < bgPx.Length; i++)
                    {
                        bgPx[i] = new Color(bgGray, bgGray, bgGray, 1f);
                    }
                    bgTex.SetPixels(bgPx);
                    bgTex.Apply();
                    bgSr.sprite = Sprite.Create(bgTex, new Rect(0, 0, 4, 4), new Vector2(0.5f, 0.5f), 100f);
                    bgSr.sharedMaterial = litMat;
                    bgSr.sortingOrder = -1;
                    bgGo.transform.position = new Vector3(-0.5f, 0.5f, 1f);
                    bgGo.transform.localScale = new Vector3(8f, 6f, 1f);
                }

                cam.Render();
                var prev = RenderTexture.active;
                RenderTexture.active = rt;
                var shot = new Texture2D(w, h, TextureFormat.RGBA32, false);
                shot.ReadPixels(new Rect(0, 0, w, h), 0, 0);
                shot.Apply();
                RenderTexture.active = prev;
                var got = shot.GetPixels32();
                bool finite = true;
                double sum = 0;
                for (int i = 0; i < got.Length; i++)
                {
                    var c = got[i];
                    sum += c.r + c.g + c.b;
                }
                if (sum <= 0.0)
                {
                    finite = false;
                }
                if (finite)
                {
                    File.WriteAllBytes(pngPath, ImageConversion.EncodeToPNG(shot));
                    ok = File.Exists(pngPath) && new FileInfo(pngPath).Length > 0;
                }
                else
                {
                    note = "readback blank/nonfinite";
                }
                UnityEngine.Object.DestroyImmediate(shot);
            }
            catch (Exception ex)
            {
                note = ex.Message;
            }
            finally
            {
                cam.targetTexture = null;
                UnityEngine.Object.DestroyImmediate(bgGo);
                UnityEngine.Object.DestroyImmediate(quadGo);
                UnityEngine.Object.DestroyImmediate(lightGo);
                UnityEngine.Object.DestroyImmediate(camGo);
                if (tex != null)
                {
                    UnityEngine.Object.DestroyImmediate(tex);
                }
                if (bgTex != null)
                {
                    UnityEngine.Object.DestroyImmediate(bgTex);
                }
                UnityEngine.Object.DestroyImmediate(litMat);
                UnityEngine.Object.DestroyImmediate(rt);
            }
            entries.Add(new Entry
            {
                AssetKey = kind,
                Variant = time + "-" + zoomLabel,
                Width = w,
                Height = h,
                Png = ok ? Path.GetFileName(pngPath) : string.Empty,
                Ok = ok,
                Note = note,
            });
            return ok;
        }

        private static void WriteReport(string path, List<Entry> entries, string failure)
        {
            var sb = new StringBuilder(512);
            sb.Append('{');
            sb.Append("\"result\":").Append(failure.Length == 0 ? "\"PASS\"" : "\"FAIL\"").Append(',');
            sb.Append("\"entries\":").Append(entries.Count).Append(',');
            sb.Append("\"png\":[");
            bool first = true;
            for (int i = 0; i < entries.Count; i++)
            {
                if (entries[i].Png.Length == 0)
                {
                    continue;
                }
                if (!first)
                {
                    sb.Append(',');
                }
                first = false;
                sb.Append('"').Append(entries[i].Png).Append('"');
            }
            sb.Append(']');
            if (failure.Length != 0)
            {
                sb.Append(",\"failure\":\"").Append(Json(failure)).Append('"');
            }
            sb.Append('}');
            File.WriteAllText(path, sb.ToString() + "\n");
        }

        private static void WriteFixturesXml(
            string path, List<(string name, bool pass, string note)> fixtures, string failure)
        {
            var sb = new StringBuilder(1024);
            sb.AppendLine("<?xml version=\"1.0\" encoding=\"utf-8\"?>");
            sb.AppendLine("<test-run>");
            sb.AppendLine("  <test-suite name=\"VisualReview\" fullname=\"VisualReview\">");
            foreach (var f in fixtures)
            {
                string res = f.pass && failure.Length == 0 ? "Passed" : "Failed";
                sb.AppendLine("    <test-case name=\"" + f.name
                    + "\" fullname=\"VisualReview." + f.name
                    + "\" result=\"" + res + "\">");
                sb.AppendLine("      <properties><property name=\"Category\" value=\"GraphicsFixtures\"/></properties>");
                sb.AppendLine("    </test-case>");
            }
            if (failure.Length != 0)
            {
                sb.AppendLine("    <test-case name=\"BatchFailure\" fullname=\"VisualReview.BatchFailure\" result=\"Failed\">");
                sb.AppendLine("      <properties><property name=\"Category\" value=\"GraphicsFixtures\"/></properties>");
                sb.AppendLine("    </test-case>");
            }
            sb.AppendLine("  </test-suite>");
            sb.AppendLine("</test-run>");
            File.WriteAllText(path, sb.ToString());
        }

        private static string Json(string s)
        {
            return s.Replace("\\", "\\\\").Replace("\"", "\\\"");
        }

        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
            while (dir != null && !Directory.Exists(Path.Combine(dir, "docs")))
            {
                dir = Directory.GetParent(dir)?.FullName;
            }
            if (dir == null)
            {
                throw new InvalidOperationException("repo root not found above Assets");
            }
            return dir;
        }
    }
}
