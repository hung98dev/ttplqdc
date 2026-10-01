using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using NUnit.Framework;
using ThinhThan.Core.Rendering;
using UnityEditor;
using UnityEngine;
using UnityEngine.Rendering.Universal;

namespace ThinhThan.Tests.EditMode.RenderingSetup
{
    /// <summary>
    /// IMP-101 acceptance surface: URP 2D renderer + pipeline assets,
    /// Sprite-Lit-Default materials, day/night evaluation, point-light
    /// budgets, per-profile contact shadows, UI PPU contract and the
    /// path-derived pipeline GUID baseline.
    /// </summary>
    public class RenderingSetupTests
    {
        private const string SettingsDir = "Assets/Settings/Rendering";
        private const string UrpAssetPath = SettingsDir + "/ThinhThanURP.asset";
        private const string RendererDataPath = SettingsDir + "/ThinhThan2DRenderer.asset";
        private const string UrpAssetMetaPath = SettingsDir + "/ThinhThanURP.asset.meta";
        private const string SpriteLitShaderName = "Universal Render Pipeline/2D/Sprite-Lit-Default";
        private const string UrpBaselineGuid = "7173cd4de770d04e9eb16b653173bef6";

        private static string DerivedGuid(string repoRelativePath)
        {
            using (var sha = SHA256.Create())
            {
                var bytes = sha.ComputeHash(Encoding.UTF8.GetBytes(repoRelativePath));
                var sb = new StringBuilder(32);
                for (int i = 0; i < 16; i++)
                {
                    sb.Append(bytes[i].ToString("x2"));
                }
                return sb.ToString();
            }
        }

        private static void AssertLight(
            (Color color, float intensity) result,
            Color wantColor,
            float wantIntensity,
            string context)
        {
            Assert.That(result.color.r, Is.EqualTo(wantColor.r).Within(1e-4f), "color.r " + context);
            Assert.That(result.color.g, Is.EqualTo(wantColor.g).Within(1e-4f), "color.g " + context);
            Assert.That(result.color.b, Is.EqualTo(wantColor.b).Within(1e-4f), "color.b " + context);
            Assert.That(result.color.a, Is.EqualTo(wantColor.a).Within(1e-4f), "color.a " + context);
            Assert.That(result.intensity, Is.EqualTo(wantIntensity).Within(1e-5f), "intensity " + context);
        }

        private static string ReadAssetText(string assetRelativePath)
        {
            var path = Path.GetFullPath(Path.Combine(Application.dataPath, "..", assetRelativePath));
            Assert.IsTrue(File.Exists(path), "missing " + path);
            return File.ReadAllText(path);
        }

        private static string ReadMetaGuid(string assetRelativePath)
        {
            var meta = ReadAssetText(assetRelativePath + ".meta");
            var match = Regex.Match(meta, "^guid: ([0-9a-f]{32})$", RegexOptions.Multiline);
            Assert.IsTrue(match.Success, "guid line in " + assetRelativePath + ".meta");
            return match.Groups[1].Value;
        }

        [Test]
        public void TestRendererAsset()
        {
            var pipeline = AssetDatabase.LoadAssetAtPath<UniversalRenderPipelineAsset>(UrpAssetPath);
            Assert.IsNotNull(pipeline, "ThinhThanURP.asset must deserialize as UniversalRenderPipelineAsset");
            var rendererData = AssetDatabase.LoadAssetAtPath<ScriptableRendererData>(RendererDataPath);
            Assert.IsNotNull(rendererData, "ThinhThan2DRenderer.asset must deserialize as ScriptableRendererData");
            Assert.IsInstanceOf<Renderer2DData>(rendererData, "renderer data must be the URP 2D renderer");

            // Renderer slot 0 binds to the renderer asset's own materialized GUID.
            string rendererGuid = ReadMetaGuid(RendererDataPath);
            string pipelineText = ReadAssetText(UrpAssetPath);
            StringAssert.Contains("m_RendererDataList:", pipelineText, "renderer list serialized");
            StringAssert.Contains(
                "- {fileID: 11400000, guid: " + rendererGuid + ", type: 2}",
                pipelineText,
                "renderer list entry 0 must be ThinhThan2DRenderer");
        }

        [Test]
        public void TestSpriteLitMaterials()
        {
            var shader = Shader.Find(SpriteLitShaderName);
            Assert.IsNotNull(shader, "Sprite-Lit-Default shader must resolve by name");
            string[] guids = AssetDatabase.FindAssets("t:Material", new[] { SettingsDir });
            Assert.GreaterOrEqual(guids.Length, 2, "shared Sprite-Lit materials for gameplay and UI sprites");
            foreach (string guid in guids)
            {
                string path = AssetDatabase.GUIDToAssetPath(guid);
                var material = AssetDatabase.LoadAssetAtPath<Material>(path);
                Assert.IsNotNull(material, path);
                Assert.AreSame(shader, material!.shader, path + " must use Sprite-Lit-Default");
            }
        }

        [Test]
        public void TestDayNightInterpolation()
        {
            var day = new Color(1f, 0.95f, 0.8f, 1f);
            var night = new Color(0.1f, 0.15f, 0.35f, 1f);
            const float dayIntensity = 1f;
            const float nightIntensity = 0.25f;

            // Boundaries: dawn [0,300), day [300,4500), dusk [4500,4800), night [4800,7200), wrap at 7200.
            AssertLight(DayNightLighting.EvaluateGlobalLight(0, day, dayIntensity, night, nightIntensity), night, nightIntensity, "phase 0 = dawn start");
            AssertLight(DayNightLighting.EvaluateGlobalLight(300, day, dayIntensity, night, nightIntensity), day, dayIntensity, "phase 300 = day");
            AssertLight(DayNightLighting.EvaluateGlobalLight(4500, day, dayIntensity, night, nightIntensity), day, dayIntensity, "phase 4500 = day");
            AssertLight(DayNightLighting.EvaluateGlobalLight(4800, day, dayIntensity, night, nightIntensity), night, nightIntensity, "phase 4800 = night");
            AssertLight(DayNightLighting.EvaluateGlobalLight(6000, day, dayIntensity, night, nightIntensity), night, nightIntensity, "phase 6000 = night");
            AssertLight(DayNightLighting.EvaluateGlobalLight(7200, day, dayIntensity, night, nightIntensity), night, nightIntensity, "phase 7200 wraps to 0 = night");

            // Mid-dawn and mid-dusk interpolate halfway between night and day.
            var half = Color.Lerp(night, day, 0.5f);
            float halfIntensity = Mathf.Lerp(nightIntensity, dayIntensity, 0.5f);
            AssertLight(DayNightLighting.EvaluateGlobalLight(150, day, dayIntensity, night, nightIntensity), half, halfIntensity, "mid dawn");
            AssertLight(DayNightLighting.EvaluateGlobalLight(4650, day, dayIntensity, night, nightIntensity), half, halfIntensity, "mid dusk");

            // Phase wraps modulo WORLD_DAY_SECONDS, including negative input.
            AssertLight(DayNightLighting.EvaluateGlobalLight(9200, day, dayIntensity, night, nightIntensity), day, dayIntensity, "9200 wraps to 2000 = day");
            AssertLight(DayNightLighting.EvaluateGlobalLight(-6450, day, dayIntensity, night, nightIntensity), day, dayIntensity, "-6450 wraps to 750 = day");
            AssertLight(DayNightLighting.EvaluateGlobalLight(-100, day, dayIntensity, night, nightIntensity), night, nightIntensity, "-100 wraps to 7100 = night");
        }

        [Test]
        public void TestPresetLightBudget()
        {
            Assert.AreEqual(4, PointLightBudget.ActivePointLightLimit(QualityPreset.Low), "LOW budget");
            Assert.AreEqual(8, PointLightBudget.ActivePointLightLimit(QualityPreset.Medium), "MEDIUM budget");
            Assert.AreEqual(16, PointLightBudget.ActivePointLightLimit(QualityPreset.High), "HIGH budget");

            var budget = new PointLightBudget();
            var lights = new List<Light2D>(12);
            var root = new GameObject("PointLightBudgetFixture");
            try
            {
                for (int i = 0; i < 12; i++)
                {
                    var go = new GameObject("Light" + i);
                    go.transform.SetParent(root.transform, false);
                    go.transform.position = new Vector3(i % 4 * 10f, i / 4 * 10f, 0f);
                    var light = go.AddComponent<Light2D>();
                    light.enabled = false;
                    Assert.IsTrue(budget.Register(light), "register " + i);
                    Assert.IsFalse(budget.Register(light), "duplicate register rejected");
                    lights.Add(light);
                }
                Assert.AreEqual(12, budget.Count);

                // LOW 4: nearest four to the origin activate; rest deactivate.
                Assert.AreEqual(4, budget.ApplyBudget(QualityPreset.Low, Vector2.zero), "LOW active count");
                int[] enabled = { 0, 1, 4, 5 };
                for (int i = 0; i < 12; i++)
                {
                    bool want = Array.IndexOf(enabled, i) >= 0;
                    Assert.AreEqual(want, lights[i].enabled, "LOW light " + i);
                }

                // MEDIUM 8 extends the set to the eight nearest.
                Assert.AreEqual(8, budget.ApplyBudget(QualityPreset.Medium, Vector2.zero), "MEDIUM active count");
                int[] enabled8 = { 0, 1, 2, 4, 5, 6, 8, 9 };
                for (int i = 0; i < 12; i++)
                {
                    bool want = Array.IndexOf(enabled8, i) >= 0;
                    Assert.AreEqual(want, lights[i].enabled, "MEDIUM light " + i);
                }

                // HIGH 16 covers all twelve registered lights.
                Assert.AreEqual(12, budget.ApplyBudget(QualityPreset.High, Vector2.zero), "HIGH active count");
                for (int i = 0; i < 12; i++)
                {
                    Assert.IsTrue(lights[i].enabled, "HIGH light " + i);
                }

                // Unregistered lights are untouched by the budget pass.
                Assert.IsTrue(budget.Unregister(lights[11]));
                Assert.IsFalse(budget.Unregister(lights[11]), "double unregister rejected");
                lights[11].enabled = false;
                Assert.AreEqual(11, budget.ApplyBudget(QualityPreset.High, Vector2.zero));
                Assert.IsFalse(lights[11].enabled, "unregistered light not re-enabled");
                Assert.AreEqual(11, budget.Count);

                // Destroyed lights are pruned without breaking the pass.
                UnityEngine.Object.DestroyImmediate(lights[10].gameObject);
                Assert.AreEqual(10, budget.ApplyBudget(QualityPreset.High, Vector2.zero));
                Assert.AreEqual(10, budget.Count, "destroyed light pruned");
            }
            finally
            {
                UnityEngine.Object.DestroyImmediate(root);
            }
        }

        [Test]
        public void TestContactShadowPerActorProfile()
        {
            var material = AssetDatabase.LoadAssetAtPath<Material>(SettingsDir + "/SpriteLitDefault.mat");
            Assert.IsNotNull(material, "shared Sprite-Lit material for shadows");
            var profiles = (ContactShadow.ActorProfile[])Enum.GetValues(typeof(ContactShadow.ActorProfile));
            Assert.AreEqual(8, profiles.Length, "8 actor profiles: 6 size_profile + SPIRIT_BEAST + NPC_HUMANOID");

            // The component must not own any Unity frame callbacks (PERF-020).
            foreach (MethodInfo method in typeof(ContactShadow).GetMethods(
                BindingFlags.NonPublic | BindingFlags.Instance | BindingFlags.DeclaredOnly))
            {
                Assert.IsFalse(
                    method.Name == "Update" || method.Name == "FixedUpdate"
                        || method.Name == "LateUpdate" || method.Name == "OnGUI",
                    method.Name + " must not be a frame callback");
            }

            var root = new GameObject("ContactShadowFixture");
            try
            {
                foreach (ContactShadow.ActorProfile actor in profiles)
                {
                    ContactShadow.Profile profile = ContactShadow.ProfileFor(actor);
                    string ctx = actor.ToString();
                    Assert.Greater(profile.Scale.x, 0f, ctx + " scale.x");
                    Assert.Greater(profile.Scale.y, 0f, ctx + " scale.y");
                    Assert.That(profile.Softness, Is.GreaterThan(0f).And.LessThanOrEqualTo(1f), ctx + " softness");

                    var actorObject = new GameObject(ctx);
                    actorObject.transform.SetParent(root.transform, false);
                    var shadow = actorObject.AddComponent<ContactShadow>();
                    Sprite sprite = ContactShadow.CreateShadowSprite(profile.Softness);
                    try
                    {
                        SpriteRenderer renderer = shadow.Configure(profile, sprite, material!);
                        Transform? child = actorObject.transform.Find(ContactShadow.ChildName);
                        Assert.IsNotNull(child, ctx + " shadow child exists");
                        Assert.AreSame(renderer, child!.GetComponent<SpriteRenderer>(), ctx + " renderer");
                        Assert.AreEqual(sprite, renderer.sprite, ctx + " sprite");
                        Assert.AreSame(material, renderer.sharedMaterial, ctx + " shared material");
                        Assert.AreEqual(
                            RenderingContract.ContactShadowOrderInLayer,
                            renderer.sortingOrder,
                            ctx + " order band under actor");
                        Assert.AreEqual(
                            new Vector3(profile.Offset.x, profile.Offset.y, 0f),
                            child!.localPosition,
                            ctx + " anchored at profile offset");
                        Assert.AreEqual(
                            new Vector3(profile.Scale.x, profile.Scale.y, 1f),
                            child!.localScale,
                            ctx + " scaled to profile");
                    }
                    finally
                    {
                        UnityEngine.Object.DestroyImmediate(sprite.texture);
                        UnityEngine.Object.DestroyImmediate(sprite);
                    }
                }
            }
            finally
            {
                UnityEngine.Object.DestroyImmediate(root);
            }
        }

        [Test]
        public void TestUiImport200Ppu()
        {
            Assert.AreEqual(200, RenderingContract.UiSpritePixelsPerUnit, "UI sprites import at 200 PPU");
            Assert.AreEqual(100, RenderingContract.GameplaySpritePixelsPerUnit, "gameplay sprites import at 100 PPU");
            Assert.AreEqual(50, RenderingContract.ParallaxFarSpritePixelsPerUnit, "parallax-far sprites import at 50 PPU");
        }

        [Test]
        public void TestSortAxisAndSrpBatcher()
        {
            var pipeline = AssetDatabase.LoadAssetAtPath<UniversalRenderPipelineAsset>(UrpAssetPath);
            Assert.IsNotNull(pipeline, "pipeline asset");
            string pipelineText = ReadAssetText(UrpAssetPath);
            StringAssert.Contains("m_UseSRPBatcher: 1", pipelineText, "SRP Batcher must be enabled");

            var rendererData = AssetDatabase.LoadAssetAtPath<ScriptableRendererData>(RendererDataPath);
            Assert.IsNotNull(rendererData, "renderer data");
            Assert.IsInstanceOf<Renderer2DData>(rendererData, "URP 2D renderer");
            string rendererText = ReadAssetText(RendererDataPath);
            StringAssert.Contains("m_TransparencySortMode: 3", rendererText, "transparency sort mode = CustomAxis");
            StringAssert.Contains(
                "m_TransparencySortAxis: {x: 0, y: 1, z: 0}",
                rendererText,
                "sort axis (0,1,0)");
        }

        [Test]
        public void TestPipelineAssetMatchesBaselineGuid()
        {
            string metaPath = Path.GetFullPath(Path.Combine(Application.dataPath, "..", UrpAssetMetaPath));
            Assert.IsTrue(File.Exists(metaPath), "missing " + metaPath);
            string meta = File.ReadAllText(metaPath);
            StringAssert.Contains(
                "guid: " + DerivedGuid("client/Assets/Settings/Rendering/ThinhThanURP.asset"),
                meta,
                "baseline path-derived GUID");
            StringAssert.Contains("guid: " + UrpBaselineGuid, meta, "URP slot GUID pinned in GraphicsSettings");

            var pipeline = AssetDatabase.LoadAssetAtPath<UniversalRenderPipelineAsset>(UrpAssetPath);
            Assert.IsNotNull(pipeline, "pipeline asset loads");
            Assert.AreSame(
                pipeline,
                UnityEngine.Rendering.GraphicsSettings.currentRenderPipeline,
                "project render pipeline resolves to ThinhThanURP");
        }
    }
}
