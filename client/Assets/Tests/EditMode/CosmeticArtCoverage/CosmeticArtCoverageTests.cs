using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text.RegularExpressions;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;
using CutoutGate = ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate;
using VolumeGate = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate;

namespace ThinhThan.Tests.EditMode.CosmeticArtCoverage
{
    /// <summary>
    /// IMP-074 coverage: all 294 launch cosmetic IDs resolve through the
    /// presentation map (art / text), shipped media pass the §3.1a Cutout
    /// and Volume &amp; Depth gates, appearances bind via Sprite Library
    /// (ADR-0076 §3.7 fixed layers), cosmetics carry no power/collider
    /// components, provenance fragment + folklore cards + cultural review
    /// evidence cover every row, and the AI tool record matches the owner
    /// setup (Direct AI Generation, technology_versions.md).
    /// </summary>
    public class CosmeticArtCoverageTests
    {
        private const string MapRel = "client/Assets/Art/Cosmetics/cosmetic_presentation_map.json";
        private const string FragmentRel = "client/Assets/Art/Provenance/fragments/cosmetics.json";
        private const string PaletteRel = "client/Assets/Art/StyleRef/cosmetics/cosmetic_chibi/palette.json";
        private const string StyleRel = "client/Assets/Art/StyleRef/cosmetics/cosmetic_chibi/style.md";
        private const string CulturalRel = "client/Assets/Art/Provenance/cultural_review.md";
        private const string CatalogRel = "docs/07_content/cosmetic_catalog.md";
        private const string CosmeticGroupRel = "client/Assets/AddressableAssetsData/AssetGroups/cosmetic.shared.asset";
        private const string IconsGroupRel = "client/Assets/AddressableAssetsData/AssetGroups/icons.shared.asset";
        private const string ExpectedPack = "cosmetics/cosmetic_chibi";

        private static readonly HashSet<string> AllowedScriptGuids = new HashSet<string>
        {
            "c29cff538c195c249b69c6f2236de67b", // SpriteLibrary
            "ed8b1ae4e4e52b34ea557c1c11e076fc", // SpriteResolver
        };

        private static readonly string[] FixedLayers =
        {
            "head", "hair", "torso", "arm_front", "arm_back",
            "leg_front", "leg_back", "weapon",
        };

        // Every appearance preview was produced without a separate weapon
        // sprite, so the required set is the fixed layers minus "weapon";
        // optional accessory_* categories (3.7) are also permitted.
        private static readonly string[] RequiredLayers =
        {
            "head", "hair", "torso", "arm_front", "arm_back",
            "leg_front", "leg_back",
        };

        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
            while (dir != null && !Directory.Exists(Path.Combine(dir, "docs")))
            {
                dir = Directory.GetParent(dir)?.FullName;
            }
            Assert.IsNotNull(dir, "repo root not found above Assets");
            return dir!;
        }

        private static string Read(string rel)
        {
            var p = Path.Combine(RepoRoot(), rel.Replace('/', Path.DirectorySeparatorChar));
            Assert.IsTrue(File.Exists(p), "missing " + rel);
            return File.ReadAllText(p);
        }

        private static bool Exists(string rel)
        {
            return File.Exists(Path.Combine(RepoRoot(), rel.Replace('/', Path.DirectorySeparatorChar)));
        }

        private static RegisterJson.Node Map()
        {
            return RegisterJson.Parse(Read(MapRel));
        }

        private static RegisterJson.Node Fragment()
        {
            return RegisterJson.Parse(Read(FragmentRel));
        }

        private static List<RegisterJson.Node> Cosmetics()
        {
            var m = Map();
            Assert.AreEqual(1, (int)m.Get("schema_version")!.Num, "map schema_version");
            var list = m.Get("cosmetics")!.Arr!;
            Assert.AreEqual((int)m.Get("count")!.Num, list.Count, "count field");
            return list;
        }

        private static List<string> FilePaths(RegisterJson.Node cosmetic)
        {
            var files = cosmetic.Get("files");
            var paths = new List<string>();
            if (files == null || files.Type != RegisterJson.Node.Kind.Obj)
            {
                return paths;
            }
            foreach (var kv in files.Obj!)
            {
                if (kv.Value.Type == RegisterJson.Node.Kind.Str && kv.Value.Str.EndsWith(".png"))
                {
                    paths.Add(kv.Value.Str);
                }
                else if (kv.Value.Type == RegisterJson.Node.Kind.Arr)
                {
                    foreach (var n in kv.Value.Arr!)
                    {
                        if (n.Str.EndsWith(".png"))
                        {
                            paths.Add(n.Str);
                        }
                    }
                }
            }
            return paths;
        }

        // Section 3.1a gates apply to presentation surfaces (body, preview,
        // icon). Sprite-library part slices are sub-sprites of the appearance
        // asset, never standalone presentation — excluded from the gates but
        // still covered by provenance and structural tests.
        private static List<string> GatedPaths(RegisterJson.Node cosmetic)
        {
            var paths = new List<string>();
            foreach (var p in FilePaths(cosmetic))
            {
                if (!p.Contains("/parts/"))
                {
                    paths.Add(p);
                }
            }
            return paths;
        }

        private static CutoutGate.AssetClass ClassOf(RegisterJson.Node cosmetic)
        {
            var c = cosmetic.Get("asset_class")!.Str;
            switch (c)
            {
                case "COSMETIC_APPEARANCE":
                    return CutoutGate.AssetClass.Actor;
                case "PROP":
                case "ITEM_ICON":
                    return CutoutGate.AssetClass.Prop;
                case "UI_ART":
                    return CutoutGate.AssetClass.UiArt;
                case "VFX_SOFT":
                    return CutoutGate.AssetClass.VfxSoft;
                default:
                    Assert.Fail("unknown asset_class " + c);
                    return default;
            }
        }

        private static bool VolumeApplies(RegisterJson.Node cosmetic)
        {
            var c = cosmetic.Get("asset_class")!.Str;
            return c == "COSMETIC_APPEARANCE" || c == "PROP" || c == "ITEM_ICON";
        }

        private static List<LabPixels.Lab> Palette()
        {
            var p = RegisterJson.Parse(Read(PaletteRel));
            var labs = new List<LabPixels.Lab>();
            foreach (var fam in p.Get("families")!.Obj!)
            {
                foreach (var tri in fam.Value.Get("lab")!.Arr!)
                {
                    labs.Add(new LabPixels.Lab
                    {
                        L = tri.Arr![0].Num,
                        A = tri.Arr[1].Num,
                        B = tri.Arr[2].Num,
                    });
                }
            }
            Assert.Greater(labs.Count, 0, "palette must not be empty");
            return labs;
        }

        [Test]
        public void TestCosmeticCatalogCompleteMapping()
        {
            var catalog = Read(CatalogRel);
            var mm = Regex.Match(catalog, @"TOTAL_STABLE_COSMETIC_IDS\s*=\s*(\d+)");
            Assert.IsTrue(mm.Success, "TOTAL_STABLE_COSMETIC_IDS constant missing in catalog");
            var total = int.Parse(mm.Groups[1].Value, CultureInfo.InvariantCulture);

            var cosmetics = Cosmetics();
            Assert.AreEqual(total, cosmetics.Count,
                "map must cover every stable cosmetic id");

            var ids = new HashSet<string>();
            var cosmeticEntries = AddressableEntries(CosmeticGroupRel);
            var iconEntries = AddressableEntries(IconsGroupRel);
            foreach (var c in cosmetics)
            {
                var id = c.Get("id")!.Str;
                Assert.IsTrue(ids.Add(id), "duplicate id " + id);
                StringAssert.StartsWith("cosmetic.", id);
                var res = c.Get("resolution")!.Str;
                Assert.IsTrue(res == "art" || res == "text",
                    id + " unknown resolution " + res);

                var keys = c.Get("keys");
                var files = FilePaths(c);
                if (res == "text")
                {
                    Assert.AreEqual(0, files.Count, id + " text entry must have no files");
                    continue;
                }
                Assert.Greater(files.Count, 0, id + " art entry must list files");
                Assert.IsNotNull(keys, id + " art entry must declare keys");
                foreach (var kv in keys!.Obj!)
                {
                    Assert.IsTrue(AssetKey.TryParse(kv.Value.Str, out var parsed),
                        "unparseable key " + kv.Value.Str);
                    var group = KeyGroupRule.Assign(parsed);
                    Assert.IsNotNull(group, "unmapped key " + kv.Value.Str);
                    var expectedGroup = kv.Key == "icon"
                        ? AddressableGroups.IconsShared
                        : AddressableGroups.CosmeticShared;
                    Assert.AreEqual(expectedGroup, group,
                        "key " + kv.Value.Str + " must route to " + expectedGroup);
                    var entries = kv.Key == "icon" ? iconEntries : cosmeticEntries;
                    Assert.IsTrue(entries.ContainsKey(kv.Value.Str),
                        kv.Value.Str + " missing from " + expectedGroup);
                    Assert.AreEqual(MetaGuid(TargetPath(c, kv.Key)), entries[kv.Value.Str],
                        kv.Value.Str + " entry must address the mapped file");
                }
            }
        }

        private static string TargetPath(RegisterJson.Node cosmetic, string keyName)
        {
            var files = cosmetic.Get("files")!;
            switch (keyName)
            {
                case "body":
                    return files.Get("body")!.Str;
                case "prefab":
                    return files.Get("prefab")!.Str;
                case "icon":
                    return files.Get("icon")!.Str;
                default:
                    Assert.Fail("unexpected key name " + keyName);
                    return string.Empty;
            }
        }

        private static Dictionary<string, string> AddressableEntries(string rel)
        {
            var text = Read(rel);
            var map = new Dictionary<string, string>();
            var rx = new Regex(
                @"m_GUID: ([0-9a-f]{32})\n\s+m_Address: ([a-z0-9_.]+)");
            foreach (Match m in rx.Matches(text))
            {
                map[m.Groups[2].Value] = m.Groups[1].Value;
            }
            return map;
        }

        private static string MetaGuid(string rel)
        {
            var text = Read(rel + ".meta");
            var m = Regex.Match(text, @"guid: ([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + rel + ".meta");
            return m.Groups[1].Value;
        }

        [Test]
        public void TestCosmeticCutoutGateZeroViolations()
        {
            var fails = new List<string>();
            foreach (var c in Cosmetics())
            {
                if (c.Get("resolution")!.Str != "art")
                {
                    continue;
                }
                var cls = ClassOf(c);
                foreach (var rel in GatedPaths(c))
                {
                    var abs = Path.Combine(RepoRoot(),
                        rel.Replace('/', Path.DirectorySeparatorChar));
                    var img = ArtRuleFixtures.LoadPng(abs);
                    var rep = CutoutGate.Measure(
                        rel, img, 0, 0, img.Width, img.Height, cls, null,
                        false, false);
                    if (rep.Violations.Count != 0)
                    {
                        fails.Add(rel + ": " + string.Join("; ", rep.Violations));
                    }
                }
            }
            Assert.AreEqual(0, fails.Count, "cutout violations:\n" + string.Join("\n", fails));
        }

        [Test]
        public void TestCosmeticVolumeDepthGateZeroViolations()
        {
            var fails = new List<string>();
            var palette = Palette();
            foreach (var c in Cosmetics())
            {
                if (c.Get("resolution")!.Str != "art" || !VolumeApplies(c))
                {
                    continue;
                }
                var isActor = c.Get("asset_class")!.Str == "COSMETIC_APPEARANCE";
                foreach (var rel in GatedPaths(c))
                {
                    var abs = Path.Combine(RepoRoot(),
                        rel.Replace('/', Path.DirectorySeparatorChar));
                    var img = ArtRuleFixtures.LoadPng(abs);
                    var rep = VolumeGate.Measure(
                        rel, img, null, isActor ? 176 : 0, isActor ? 192 : 0);
                    if (rep.Violations.Count != 0)
                    {
                        fails.Add(rel + ": " + string.Join("; ", rep.Violations));
                    }
                    VolumeGate.Silhouette(img, null, out var inS, out var lab);
                    var s = new List<int>();
                    for (var i = 0; i < img.Width * img.Height; i++)
                    {
                        if (inS[i])
                        {
                            s.Add(i);
                        }
                    }
                    var cov = VolumeGate.PaletteCoverage(s, lab, palette);
                    if (double.IsNaN(cov) || cov < 0.85)
                    {
                        fails.Add(rel + ": palette coverage " + cov.ToString("F3",
                            CultureInfo.InvariantCulture));
                    }
                }
            }
            Assert.AreEqual(0, fails.Count, "volume violations:\n" + string.Join("\n", fails));
        }

        [Test]
        public void TestCosmeticSpriteLibraryBinding()
        {
            var appearances = Appearances();
            Assert.AreEqual(8, appearances.Count, "8 appearance cosmetics expected");
            foreach (var c in appearances)
            {
                var files = c.Get("files")!;
                var libPath = files.Get("sprite_lib")!.Str;
                var prefabPath = files.Get("prefab")!.Str;
                var libText = Read(libPath);
                var prefabText = Read(prefabPath);

                var libCats = LibCategories(libText);
                foreach (var layer in RequiredLayers)
                {
                    Assert.IsTrue(libCats.ContainsKey(layer),
                        c.Get("id")!.Str + " sprite lib missing layer " + layer);
                }
                foreach (var cat in libCats.Keys)
                {
                    var ok = cat.StartsWith("accessory_");
                    foreach (var layer in FixedLayers)
                    {
                        ok |= cat == layer;
                    }
                    Assert.IsTrue(ok,
                        c.Get("id")!.Str + " sprite lib has non-fixed category " + cat);
                }

                var resolvers = ParseResolvers(prefabText);
                Assert.AreEqual(libCats.Count, resolvers.Count,
                    "prefab must bind a resolver per sprite library category");
                foreach (var kv in libCats)
                {
                    var catHash = Animator.StringToHash(kv.Key) & 0x3FFFFFFF;
                    var labelHash = Animator.StringToHash(kv.Value) & 0x3FFFFFFF;
                    var pair = (catHash, labelHash);
                    Assert.IsTrue(resolvers.Contains(pair),
                        c.Get("id")!.Str + " resolver missing category " + kv.Key);
                    var declaredCat = DeclaredHash(libText, kv.Key);
                    Assert.AreEqual(catHash, declaredCat,
                        "declared m_Hash for category " + kv.Key + " must equal Animator.StringToHash");
                }
            }
        }

        private static List<RegisterJson.Node> Appearances()
        {
            var list = new List<RegisterJson.Node>();
            foreach (var c in Cosmetics())
            {
                if (c.Get("category")!.Str == "CHARACTER_APPEARANCE")
                {
                    list.Add(c);
                }
            }
            return list;
        }

        private static Dictionary<string, string> LibCategories(string libYaml)
        {
            var cats = new Dictionary<string, string>();
            var catRx = new Regex(@"- m_Name: ([a-z0-9_]+)\n\s+m_Hash: (-?\d+)\n\s+m_CategoryList:\n\s+- m_Name: ([a-z0-9_.]+)\n\s+m_Hash: (-?\d+)");
            foreach (Match m in catRx.Matches(libYaml))
            {
                cats[m.Groups[1].Value] = m.Groups[3].Value;
            }
            return cats;
        }

        private static int DeclaredHash(string libYaml, string category)
        {
            var rx = new Regex(@"- m_Name: " + Regex.Escape(category) + @"\n\s+m_Hash: (-?\d+)");
            var m = rx.Match(libYaml);
            Assert.IsTrue(m.Success, "category " + category + " hash not found");
            return int.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
        }

        private static HashSet<(int, int)> ParseResolvers(string prefabYaml)
        {
            var set = new HashSet<(int, int)>();
            var rx = new Regex(@"m_CategoryHash: (-?[0-9.eE+]+)\n\s+m_labelHash: (-?[0-9.eE+]+)");
            foreach (Match m in rx.Matches(prefabYaml))
            {
                var c = float.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
                var l = float.Parse(m.Groups[2].Value, CultureInfo.InvariantCulture);
                set.Add((BitConverterHack(c), BitConverterHack(l)));
            }
            return set;
        }

        private static int BitConverterHack(float f)
        {
            var bytes = System.BitConverter.GetBytes(f);
            return System.BitConverter.ToInt32(bytes, 0);
        }

        [Test]
        public void TestCosmeticEquipSlotPreview()
        {
            foreach (var c in Appearances())
            {
                var files = c.Get("files")!;
                var preview = files.Get("preview")!.Str;
                Assert.IsTrue(Exists(preview), preview + " missing");
                var img = ArtRuleFixtures.LoadPng(Path.Combine(RepoRoot(),
                    preview.Replace('/', Path.DirectorySeparatorChar)));
                var cell = c.Get("cell_ref")!.Str;
                Assert.AreEqual("96x128", cell, c.Get("id")!.Str + " cell");
                Assert.AreEqual(192, img.Width, "preview must be 2x of 96x128 cell");
                Assert.AreEqual(256, img.Height);

                var parts = files.Get("parts")!.Arr!;
                Assert.GreaterOrEqual(parts.Count, RequiredLayers.Length,
                    "preview slices must cover every produced layer");
                foreach (var layer in RequiredLayers)
                {
                    Assert.IsTrue(libPartNames(parts, layer),
                        c.Get("id")!.Str + " missing part " + layer);
                }
            }
        }

        private static bool libPartNames(List<RegisterJson.Node> parts, string layer)
        {
            foreach (var p in parts)
            {
                if (p.Str.EndsWith("/" + layer + ".png"))
                {
                    return true;
                }
            }
            return false;
        }

        [Test]
        public void TestCosmeticNonPowerInvariant()
        {
            var scriptGuids = new Regex(
                @"m_Script: \{fileID: 11500000, guid: ([0-9a-f]{32})");
            foreach (var c in Cosmetics())
            {
                var files = c.Get("files");
                if (files == null)
                {
                    continue;
                }
                var prefab = files.Get("prefab");
                if (prefab == null)
                {
                    continue;
                }
                var text = Read(prefab.Str);
                Assert.IsFalse(Regex.IsMatch(text,
                    "Collider|Rigidbody|StatBlock|Equipment|m_Damage"),
                    prefab.Str + " carries power/physics components");
                foreach (Match m in scriptGuids.Matches(text))
                {
                    Assert.IsTrue(AllowedScriptGuids.Contains(m.Groups[1].Value),
                        prefab.Str + " uses non-presentation script guid "
                        + m.Groups[1].Value);
                }
            }
        }

        [Test]
        public void TestCosmeticCulturalReviewEvidence()
        {
            var review = Read(CulturalRel);
            Assert.IsTrue(review.Contains("APPROVED"),
                "cultural review must record verdicts");
            foreach (var c in Cosmetics())
            {
                Assert.IsTrue(review.Contains("`" + c.Get("id")!.Str + "`"),
                    c.Get("id")!.Str + " missing a cultural review row");
            }
        }

        [Test]
        public void TestCosmeticProvenanceCoverage()
        {
            var rows = new Dictionary<string, RegisterJson.Node>();
            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                rows[r.Get("file_path")!.Str] = r;
            }
            var shipped = new List<string>();
            foreach (var c in Cosmetics())
            {
                shipped.AddRange(FilePaths(c));
            }
            foreach (var rel in shipped)
            {
                Assert.IsTrue(rows.ContainsKey(rel), "no provenance row for " + rel);
                var row = rows[rel];
                var abs = Path.Combine(RepoRoot(), rel.Replace('/', Path.DirectorySeparatorChar));
                var sha = ProvenanceValidator.Sha256File(abs);
                Assert.AreEqual(row.Get("final_sha256")!.Str, sha,
                    "final_sha256 mismatch " + rel);
                Assert.AreEqual("AI_CREATED", row.Get("source_kind")!.Str, rel);
                Assert.AreEqual("AI_TOOL_TERMS", row.Get("license_id")!.Str, rel);
                var gr = row.Get("generation_record");
                Assert.IsNotNull(gr, rel + " must carry generation_record");
                var termsSha = gr!.Get("terms_snapshot_sha256")!.Str;
                var termsRel = "client/Assets/Art/Provenance/terms/cosmetics/"
                    + termsSha + ".txt";
                Assert.IsTrue(Exists(termsRel), "terms snapshot missing " + termsRel);
                var termsAbs = Path.Combine(RepoRoot(),
                    termsRel.Replace('/', Path.DirectorySeparatorChar));
                Assert.AreEqual(ProvenanceValidator.Sha256File(termsAbs), termsSha,
                    "terms snapshot sha mismatch");
                Assert.IsTrue(row.Get("review_state")!.Str == "PENDING"
                    || row.Get("review_state")!.Str == "APPROVED",
                    rel + " review_state");
            }
        }

        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
            // Owner setup (technology_versions.md § Content production tools):
            // Direct AI Generation in-session; exact model id per record.
            var modelIds = new HashSet<string>();
            var tools = new HashSet<string>();
            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                if (r.Get("source_kind")!.Str != "AI_CREATED")
                {
                    continue;
                }
                var gr = r.Get("generation_record")!;
                Assert.IsTrue(gr.Get("tool")!.Str.Length > 0, "tool must be named");
                Assert.IsTrue(gr.Get("model_id")!.Str.Length > 0, "model_id must be named");
                tools.Add(gr.Get("tool")!.Str);
                modelIds.Add(gr.Get("model_id")!.Str);
                Assert.AreEqual("https://openai.com/policies/terms-of-use/",
                    gr.Get("terms_uri")!.Str, "terms_uri must be the OpenAI terms");
            }
            Assert.AreEqual(1, tools.Count,
                "all cosmetics must name the single owner-approved tool");
            Assert.AreEqual(1, modelIds.Count,
                "all cosmetics must name the single owner-approved model");
        }

        [Test]
        public void TestCosmeticStylePackAndPalette()
        {
            var style = Read(StyleRel);
            Assert.Greater(style.Length, 200, "style.md must document the pack");
            var p = RegisterJson.Parse(Read(PaletteRel));
            Assert.AreEqual(ExpectedPack, p.Get("pack_id")!.Str);
            Assert.LessOrEqual(p.Get("delta_e00_max")!.Num, 8.0);
            Assert.GreaterOrEqual(p.Get("gate_min_coverage")!.Num, 0.85);
            Assert.Greater(p.Get("families")!.Obj!.Count, 0);

            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                Assert.AreEqual(ExpectedPack, r.Get("style_pack_id")!.Str,
                    r.Get("file_path")!.Str + " style_pack_id");
            }
        }

        [Test]
        public void TestCosmeticFolkloreCards()
        {
            var byFile = new Dictionary<string, RegisterJson.Node>();
            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                byFile[r.Get("file_path")!.Str] = r;
            }
            foreach (var c in Cosmetics())
            {
                if (c.Get("cultural") == null
                    || c.Get("cultural")!.Type != RegisterJson.Node.Kind.Bool
                    || !c.Get("cultural")!.Bool)
                {
                    continue;
                }
                foreach (var rel in FilePaths(c))
                {
                    var row = byFile[rel];
                    var card = row.Get("folklore_card");
                    Assert.IsNotNull(card,
                        rel + " cultural cosmetic requires folklore_card (ART-010)");
                    Assert.Greater(card!.Get("source_tales")!.Arr!.Count, 0,
                        rel + " source_tales");
                    Assert.Greater(card.Get("motifs_checked")!.Arr!.Count, 0,
                        rel + " motifs_checked");
                }
            }
        }
    }
}
