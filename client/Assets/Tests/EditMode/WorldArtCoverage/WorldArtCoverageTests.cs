using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;
using VolumeGate = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate;

namespace ThinhThan.Tests.EditMode.WorldArtCoverage
{
    /// <summary>
    /// IMP-072 normal-world environment art coverage
    /// (presentation_asset_manifest.md §3.1-§3.11, §5-§6; world_route_catalog.md;
    /// physics_geometry_contract.md; client_assets.md § Stable Asset Keys;
    /// ADR-0068, ADR-0072, ADR-0076): the 24 visual scenes under
    /// Scenes/World must exist with unique stable Addressable keys, carry the
    /// exported geometry's bounds/anchors with no ServerGeometry, and be
    /// composed of layered tile/prop/parallax sprites rather than one bitmap;
    /// the world fragment must cover every shipped media file with valid
    /// provenance; each region keeps one style pack whose palette the shipped
    /// textures pass, tiles hold the seam rule, and every map carries a
    /// folklore_card.
    /// </summary>
    public class WorldArtCoverageTests
    {
        private const string FragmentPath =
            "client/Assets/Art/Provenance/fragments/world.json";
        private const string ArtRoot = "client/Assets/Art/World";
        private const string StyleRoot = "client/Assets/Art/StyleRef/world";
        private const string TermsRoot =
            "client/Assets/Art/Provenance/terms/world";
        private const string SceneRoot = "client/Assets/Scenes/World";
        private const string CollisionRoot = "client/Assets/Scenes/Collision";
        private const string GeomRoot = "server/internal/sim/spatial/maps";
        private const string GroupRoot =
            "client/Assets/AddressableAssetsData/AssetGroups";

        private static readonly string[] Zones =
        {
            "lang_da", "rung_u_minh", "ben_nuoc_den",
            "deo_may", "thanh_co", "nui_thieng",
        };

        private static readonly string[] MapSuffixes =
        {
            "dinh_lang", "ben_da", "bo_ruong", "go_ma",
            "loi_tram", "mieu_bo_hoang", "rung_sau", "xom_rung",
            "bai_lau", "ben_do_cu", "cho_ben", "duong_ngap",
            "ban_chan_deo", "duong_rung", "khe_da", "rung_cam",
            "cong_ngoai", "den_tran", "duong_da", "hao_can",
            "chan_nui", "cong_co", "rung_may", "suon_da",
        };

        private static readonly string[] TileKinds =
            { "ground", "fill", "platform", "wall", "water" };
        private static readonly string[] PropKinds =
            { "veg", "marker", "scatter" };
        private static readonly string[] ParallaxKinds =
            { "par_l2", "par_l3", "par_l4", "fg" };

        private static readonly string[] BannedMotifs =
        {
            "torii", "jiangshi", "kimono", "hanbok", "jade dragon",
            "chinese palace lantern", "qing dynasty",
        };

        // Region bundle budget: 60 MiB compressed per region
        // (client_performance.md region unload budget).
        private const long RegionBudgetBytes = 60L * 1024 * 1024;

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

        private static string Abs(string rel)
        {
            return Path.Combine(RepoRoot(),
                rel.Replace('/', Path.DirectorySeparatorChar));
        }

        private static string MapId(string zone, string suffix)
        {
            return "map." + zone + "." + suffix;
        }

        private static string ZoneOf(string suffix)
        {
            foreach (var z in Zones)
            {
                if (File.Exists(Abs(GeomRoot + "/map." + z + "." + suffix +
                                    ".geom.json")))
                {
                    return z;
                }
            }
            Assert.Fail("no zone owns map suffix " + suffix);
            return null!;
        }

        // ---------- fragment helpers ----------

        private static RegisterJson.Node Fragment()
        {
            string path = Abs(FragmentPath);
            Assert.IsTrue(File.Exists(path), "world fragment missing");
            var root = RegisterJson.Parse(File.ReadAllText(path));
            Assert.AreEqual(RegisterJson.Node.Kind.Obj, root.Type);
            Assert.AreEqual(1.0, root.Get("schema_version")!.Num);
            Assert.AreEqual(RegisterJson.Node.Kind.Arr,
                root.Get("assets")!.Type);
            return root;
        }

        private static List<RegisterJson.Node> Rows()
        {
            return Fragment().Get("assets")!.Arr!;
        }

        private static string? Str(RegisterJson.Node n, string f)
        {
            var v = n.Get(f);
            return v != null && v.Type == RegisterJson.Node.Kind.Str
                ? v.Str : null;
        }

        private static string Sha256File(string path)
        {
            using (var sha = System.Security.Cryptography.SHA256.Create())
            {
                var bytes = sha.ComputeHash(File.ReadAllBytes(path));
                var sb = new StringBuilder(64);
                for (int i = 0; i < bytes.Length; i++)
                {
                    sb.Append(bytes[i].ToString("x2", CultureInfo.InvariantCulture));
                }
                return sb.ToString();
            }
        }

        private static string MetaGuid(string relAsset)
        {
            var text = File.ReadAllText(Abs(relAsset + ".meta"));
            var m = Regex.Match(text, @"guid: ([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + relAsset + ".meta");
            return m.Groups[1].Value;
        }

        // ---------- geom.json parsing (minimal, field-scoped) ----------

        private static string GeomText(string mapId)
        {
            string p = Abs(GeomRoot + "/" + mapId + ".geom.json");
            Assert.IsTrue(File.Exists(p), "geom.json missing for " + mapId);
            return File.ReadAllText(p);
        }

        private static double GeomNumber(string text, string key)
        {
            var m = Regex.Match(text, "\"" + key + "\"\\s*:\\s*(-?[0-9.]+)");
            Assert.IsTrue(m.Success, "geom missing " + key);
            return double.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
        }

        private static string GeomString(string text, string key)
        {
            var m = Regex.Match(text, "\"" + key + "\"\\s*:\\s*\"([^\"]+)\"");
            Assert.IsTrue(m.Success, "geom missing " + key);
            return m.Groups[1].Value;
        }

        /// <summary>All (id,x,y) triples from the geom anchors array.</summary>
        private static List<(string Id, double X, double Y)> GeomAnchors(
            string text)
        {
            var list = new List<(string, double, double)>();
            var anchorsPart = text.Substring(text.IndexOf(
                "\"anchors\"", StringComparison.Ordinal));
            foreach (Match m in Regex.Matches(
                        anchorsPart,
                        "\\{\\s*\"id\"\\s*:\\s*\"([^\"]+)\"\\s*,\\s*\"x\"\\s*:" +
                        "\\s*(-?[0-9.]+)\\s*,\\s*\"y\"\\s*:\\s*(-?[0-9.]+)"))
            {
                list.Add((m.Groups[1].Value,
                        double.Parse(m.Groups[2].Value,
                                    CultureInfo.InvariantCulture),
                        double.Parse(m.Groups[3].Value,
                                    CultureInfo.InvariantCulture)));
            }
            Assert.Greater(list.Count, 0, "geom has no anchors");
            return list;
        }

        // ---------- scene YAML helpers ----------

        /// <summary>
        /// Extract (name -> (x,y)) for GameObjects whose Transform immediately
        /// follows them in the authored scene (composer emits GO+Transform
        /// pairs in order).
        /// </summary>
        private static Dictionary<string, (double X, double Y)> SceneObjects(
            string sceneText)
        {
            var map = new Dictionary<string, (double, double)>(
                StringComparer.Ordinal);
            foreach (Match m in Regex.Matches(
                        sceneText,
                        "m_Name: ([^\\n]+)\\n[\\s\\S]*?m_LocalPosition: " +
                        "\\{x: (-?[0-9.]+), y: (-?[0-9.]+)"))
            {
                var name = m.Groups[1].Value.Trim();
                if (!map.ContainsKey(name))
                {
                    map[name] = (
                        double.Parse(m.Groups[2].Value,
                                    CultureInfo.InvariantCulture),
                        double.Parse(m.Groups[3].Value,
                                    CultureInfo.InvariantCulture));
                }
            }
            return map;
        }

        // ---------- tests ----------

        [Test]
        public void TestWorldSceneRoster()
        {
            var groups = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var z in Zones)
            {
                groups[z] = File.ReadAllText(
                    Abs(GroupRoot + "/region." + z + ".asset"));
            }
            var seenKeys = new HashSet<string>(StringComparer.Ordinal);
            foreach (var z in Zones)
            {
                for (int i = 0; i < 4; i++)
                {
                    string suffix = MapSuffixes[
                        Array.IndexOf(Zones, z) * 4 + i];
                    string mapId = MapId(z, suffix);
                    string rel = SceneRoot + "/" + mapId + ".unity";
                    Assert.IsTrue(File.Exists(Abs(rel)),
                        "visual scene missing: " + rel);
                    string key = "asset." + mapId + ".scene";
                    Assert.IsTrue(seenKeys.Add(key),
                        "duplicate scene key " + key);
                    Assert.IsTrue(AssetKey.TryParse(key, out var parsed),
                        "scene key failed to parse: " + key);
                    Assert.AreEqual("region." + z, KeyGroupRule.Assign(parsed),
                        "scene key routed wrong: " + key);
                    string guid = MetaGuid(rel);
                    Assert.IsTrue(groups[z].Contains(
                            "m_GUID: " + guid + "\n    m_Address: " + key),
                        key + " bound to wrong GUID in region." + z);
                }
            }
        }

        [Test]
        public void TestSceneGeometryConsistency()
        {
            foreach (var suffix in MapSuffixes)
            {
                var z = ZoneOf(suffix);
                string mapId = MapId(z, suffix);
                string geom = GeomText(mapId);
                string scene = File.ReadAllText(
                    Abs(SceneRoot + "/" + mapId + ".unity"));
                // no server geometry may ride in the visual scene
                Assert.IsFalse(scene.Contains("ServerGeometry"),
                    mapId + " carries ServerGeometry");
                Assert.IsFalse(scene.Contains("Collider"),
                    mapId + " carries a collider component");
                // GeometryMeta mirrors the exported space
                Assert.IsTrue(scene.Contains("spaceId: " + mapId),
                    mapId + " spaceId mismatch");
                Assert.IsTrue(scene.Contains(
                        "spaceKind: " + GeomString(geom, "space_kind")),
                    mapId + " spaceKind mismatch");
                Assert.IsTrue(scene.Contains(
                        "layoutProfile: " + GeomString(geom, "layout_profile")),
                    mapId + " layoutProfile mismatch");
                double bx = GeomNumber(geom, "max_x") / 1000.0;
                double by = GeomNumber(geom, "max_y") / 1000.0;
                Assert.IsTrue(scene.Contains(
                        "boundsMaxX: " + bx.ToString(
                            "0.########", CultureInfo.InvariantCulture)),
                    mapId + " boundsMaxX mismatch: " + bx);
                Assert.IsTrue(scene.Contains(
                        "boundsMaxY: " + by.ToString(
                            "0.########", CultureInfo.InvariantCulture)),
                    mapId + " boundsMaxY mismatch: " + by);
                // every exported anchor is a marker GO at the same position
                var objects = SceneObjects(scene);
                foreach (var a in GeomAnchors(geom))
                {
                    Assert.IsTrue(objects.TryGetValue(a.Id, out var pos),
                        mapId + " lacks anchor GO " + a.Id);
                    double ax = a.X / 1000.0, ay = a.Y / 1000.0;
                    Assert.LessOrEqual(Math.Abs(pos.X - ax), 0.01,
                        mapId + " anchor " + a.Id + " x off");
                    Assert.LessOrEqual(Math.Abs(pos.Y - ay), 0.01,
                        mapId + " anchor " + a.Id + " y off");
                }
                // layered sprites — a real composition, not one bitmap
                int renderers = Regex.Matches(scene, "SpriteRenderer:").Count;
                Assert.GreaterOrEqual(renderers, 24,
                    mapId + " too few sprites — single-bitmap substitute?");
                var guids = new HashSet<string>(StringComparer.Ordinal);
                foreach (Match m in Regex.Matches(
                            scene, "m_Sprite: \\{fileID: -?[0-9]+, " +
                            "guid: ([0-9a-f]{32})"))
                {
                    guids.Add(m.Groups[1].Value);
                }
                Assert.GreaterOrEqual(guids.Count, 8,
                    mapId + " uses fewer than 8 distinct textures");
            }
        }

        [Test]
        public void TestWorldArtCoverage()
        {
            var fragmentFiles = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                var fp = Str(row, "file_path");
                Assert.IsNotNull(fp);
                fragmentFiles.Add(fp!);
            }
            // every shipped media file resolves a row
            foreach (var dir in new[] { ArtRoot, StyleRoot })
            {
                foreach (var f in Directory.GetFiles(
                            Abs(dir), "*", SearchOption.AllDirectories))
                {
                    var rel = Path.GetRelativePath(
                        RepoRoot(), f).Replace('\\', '/');
                    if (ProvenanceValidator.IsMediaFile(rel))
                    {
                        Assert.IsTrue(fragmentFiles.Contains(rel),
                            "media file lacks provenance row: " + rel);
                    }
                }
            }
            // roster completeness: every zone ships the full texture set
            foreach (var z in Zones)
            {
                var expected = new List<string>();
                foreach (var k in TileKinds)
                {
                    expected.Add(ArtRoot + "/" + z + "/tiles/" + k + ".png");
                }
                foreach (var k in PropKinds)
                {
                    expected.Add(ArtRoot + "/" + z + "/props/" + k + ".png");
                }
                foreach (var k in ParallaxKinds)
                {
                    expected.Add(ArtRoot + "/" + z + "/parallax/" + k + ".png");
                }
                for (int i = 0; i < 4; i++)
                {
                    string suffix = MapSuffixes[
                        Array.IndexOf(Zones, z) * 4 + i];
                    expected.Add(ArtRoot + "/" + z + "/sig/" + suffix + ".png");
                }
                foreach (var rel in expected)
                {
                    Assert.IsTrue(File.Exists(Abs(rel)),
                        "missing deliverable " + rel);
                    Assert.IsTrue(fragmentFiles.Contains(rel),
                        "no provenance row for " + rel);
                }
            }
            // fragment integrity: sorted, files exist, hashes match
            string? prev = null;
            foreach (var row in Rows())
            {
                var path = Str(row, "file_path")!;
                if (prev != null)
                {
                    Assert.IsTrue(string.CompareOrdinal(prev, path) < 0,
                        "assets not sorted: " + path);
                }
                prev = path;
                Assert.IsTrue(File.Exists(Abs(path)), "row file missing: " + path);
                Assert.AreEqual(Str(row, "final_sha256"), Sha256File(Abs(path)),
                    "final_sha256 mismatch: " + path);
            }
        }

        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
            var techDoc = File.ReadAllText(
                Abs("docs/00_context/technology_versions.md"));
            Assert.IsTrue(techDoc.Contains("Direct AI Generation"),
                "technology_versions.md no longer names the AI tool");
            int aiRows = 0;
            foreach (var row in Rows())
            {
                var path = Str(row, "file_path");
                if (Str(row, "source_kind") != "AI_CREATED")
                {
                    continue;
                }
                aiRows++;
                var gen = row.Get("generation_record");
                Assert.IsNotNull(gen, "generation_record missing: " + path);
                Assert.AreEqual(RegisterJson.Node.Kind.Obj, gen!.Type, path);
                Assert.AreEqual(
                    "Direct AI Generation (In-Session Multimodal)",
                    Str(gen, "tool"), "tool name drift: " + path);
                var model = Str(gen, "model_id");
                Assert.IsNotNull(model, "model_id missing: " + path);
                Assert.IsTrue(model!.Length > 0, path);
                var termsUri = Str(gen, "terms_uri");
                Assert.IsNotNull(termsUri, "terms_uri missing: " + path);
                Assert.IsTrue(termsUri!.StartsWith("https://",
                    StringComparison.Ordinal));
                var snap = Str(gen, "terms_snapshot_sha256");
                Assert.IsNotNull(snap,
                    "terms_snapshot_sha256 missing: " + path);
                var snapPath = TermsRoot + "/" + snap + ".txt";
                Assert.IsTrue(File.Exists(Abs(snapPath)),
                    "terms snapshot missing: " + snapPath);
                Assert.AreEqual(snap, Sha256File(Abs(snapPath)),
                    "terms snapshot hash mismatch");
            }
            Assert.Greater(aiRows, 0, "no AI_CREATED rows in fragment");
        }

        [Test]
        public void TestRegionStylePacksAndTileSeams()
        {
            foreach (var z in Zones)
            {
                string packDir = StyleRoot + "/vn_env_" + z + "_v1";
                Assert.IsTrue(Directory.Exists(Abs(packDir)),
                    "style pack missing: " + packDir);
                Assert.IsTrue(File.Exists(Abs(packDir + "/palette.json")),
                    "palette.json missing in " + packDir);
                Assert.IsTrue(File.Exists(Abs(packDir + "/style.md")),
                    "style.md missing in " + packDir);
                var anchors = Directory.GetFiles(
                    Abs(packDir + "/anchors"), "*.png");
                Assert.GreaterOrEqual(anchors.Length, 6,
                    packDir + " has fewer than 6 anchors");
                Assert.LessOrEqual(anchors.Length, 10,
                    packDir + " has more than 10 anchors");
                // palette.json declares the pack id + a color list
                var pal = File.ReadAllText(Abs(packDir + "/palette.json"));
                Assert.IsTrue(pal.Contains("vn_env_" + z + "_v1"),
                    packDir + " palette.json lacks pack_id");
                Assert.GreaterOrEqual(
                    Regex.Matches(pal, "\"hex\"").Count, 8,
                    packDir + " palette.json has too few colors");
                // TILE seam rule: mean |ΔE00| between left/right columns and
                // top/bottom rows <= 2 on every tile
                foreach (var k in TileKinds)
                {
                    var img = ArtRuleFixtures.LoadPng(
                        Abs(ArtRoot + "/" + z + "/tiles/" + k + ".png"));
                    int w = img.Width, h = img.Height;
                    double sumX = 0, sumY = 0;
                    for (int y = 0; y < h; y++)
                    {
                        var pl = img.At(0, y);
                        var pr = img.At(w - 1, y);
                        sumX += Math.Abs(LabPixels.DeltaE00(
                            LabPixels.ToLab(pl.R, pl.G, pl.B),
                            LabPixels.ToLab(pr.R, pr.G, pr.B)));
                    }
                    for (int x = 0; x < w; x++)
                    {
                        var pt = img.At(x, 0);
                        var pb = img.At(x, h - 1);
                        sumY += Math.Abs(LabPixels.DeltaE00(
                            LabPixels.ToLab(pt.R, pt.G, pt.B),
                            LabPixels.ToLab(pb.R, pb.G, pb.B)));
                    }
                    Assert.LessOrEqual(sumX / h, 2.0,
                        z + "/" + k + " tile seam x = " +
                        (sumX / h).ToString("0.##", CultureInfo.InvariantCulture));
                    Assert.LessOrEqual(sumY / w, 2.0,
                        z + "/" + k + " tile seam y = " +
                        (sumY / w).ToString("0.##", CultureInfo.InvariantCulture));
                }
            }
        }

        [Test]
        public void TestMapFolkloreCards()
        {
            // every map carries a folklore_card with the required shape on at
            // least one row keyed to its content_id
            var cards = new Dictionary<string, RegisterJson.Node>(
                StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                var cid = Str(row, "content_id");
                var fc = row.Get("folklore_card");
                if (cid != null && fc != null &&
                    fc.Type == RegisterJson.Node.Kind.Obj)
                {
                    cards[cid] = fc;
                }
            }
            foreach (var suffix in MapSuffixes)
            {
                var z = ZoneOf(suffix);
                string mapId = MapId(z, suffix);
                Assert.IsTrue(cards.TryGetValue(mapId, out var fc),
                    "no folklore_card for " + mapId);
                var tales = fc!.Get("source_tales");
                Assert.IsNotNull(tales, mapId + " card lacks source_tales");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, tales!.Type,
                    mapId + " source_tales not array");
                Assert.Greater(tales.Arr!.Count, 0,
                    mapId + " source_tales empty");
                var variants = fc.Get("regional_variants");
                Assert.IsNotNull(variants,
                    mapId + " card lacks regional_variants");
                var motifs = fc.Get("motifs_checked");
                Assert.IsNotNull(motifs, mapId + " card lacks motifs_checked");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, motifs!.Type,
                    mapId + " motifs_checked not array");
                Assert.Greater(motifs.Arr!.Count, 0,
                    mapId + " motifs_checked empty");
                foreach (var t in tales.Arr)
                {
                    foreach (var banned in BannedMotifs)
                    {
                        Assert.IsFalse(
                            t.Str!.IndexOf(banned,
                                StringComparison.OrdinalIgnoreCase) >= 0,
                            mapId + " tale mentions banned motif " + banned);
                    }
                }
            }
        }

        [Test]
        public void TestRegionBundleBudget()
        {
            foreach (var z in Zones)
            {
                long bytes = 0;
                foreach (var f in Directory.GetFiles(
                            Abs(ArtRoot + "/" + z), "*.png",
                            SearchOption.AllDirectories))
                {
                    bytes += new FileInfo(f).Length;
                }
                Assert.LessOrEqual(bytes, RegionBudgetBytes,
                    "region." + z + " bundle " +
                    (bytes / (1024.0 * 1024)).ToString(
                        "0.##", CultureInfo.InvariantCulture) +
                    " MiB exceeds 60 MiB budget");
            }
        }

        [Test]
        [Timeout(600000)]
        public void TestPaletteGateOnShippedTextures()
        {
            // ≥85% of silhouette pixels (a ≥ 128) within ΔE00 ≤ 8 of the
            // zone palette (manifest §3.8).
            foreach (var z in Zones)
            {
                var palJson = File.ReadAllText(
                    Abs(StyleRoot + "/vn_env_" + z + "_v1/palette.json"));
                var palette = new List<LabPixels.Lab>();
                foreach (Match m in Regex.Matches(
                            palJson, "\"hex\"\\s*:\\s*\"#([0-9a-fA-F]{6})\""))
                {
                    var hex = m.Groups[1].Value;
                    palette.Add(LabPixels.ToLab(
                        Convert.ToByte(hex.Substring(0, 2), 16),
                        Convert.ToByte(hex.Substring(2, 2), 16),
                        Convert.ToByte(hex.Substring(4, 2), 16)));
                }
                Assert.Greater(palette.Count, 0,
                    "no colors parsed for " + z);
                var files = new List<string>();
                files.AddRange(Directory.GetFiles(
                    Abs(ArtRoot + "/" + z + "/tiles"), "*.png"));
                files.AddRange(Directory.GetFiles(
                    Abs(ArtRoot + "/" + z + "/props"), "*.png"));
                files.AddRange(Directory.GetFiles(
                    Abs(ArtRoot + "/" + z + "/sig"), "*.png"));
                files.AddRange(Directory.GetFiles(
                    Abs(ArtRoot + "/" + z + "/parallax"), "*.png"));
                // LoadPng is main-thread only — decode on the main thread,
                // fan out the DeltaE00 gate math.
                var imgs = new List<(string Name, LabPixels.Image Img)>();
                foreach (var f in files)
                {
                    imgs.Add((Path.GetFileName(f),
                        ArtRuleFixtures.LoadPng(f)));
                }
                var fails = new ConcurrentBag<string>();
                Parallel.ForEach(imgs, j =>
                {
                    VolumeGate.Silhouette(
                        j.Img, null, out var inS, out var lab);
                    var s = new List<int>();
                    for (var i = 0; i < j.Img.Width * j.Img.Height; i++)
                    {
                        if (inS[i])
                        {
                            s.Add(i);
                        }
                    }
                    var cov = VolumeGate.PaletteCoverage(s, lab, palette);
                    if (double.IsNaN(cov) ||
                        cov < VolumeGate.PaletteCoverageMin)
                    {
                        fails.Add(j.Name + " palette coverage " +
                            cov.ToString("0.###",
                                CultureInfo.InvariantCulture));
                    }
                });
                var ordered = new List<string>(fails);
                ordered.Sort(StringComparer.Ordinal);
                Assert.AreEqual(0, fails.Count,
                    z + " palette violations:\n" +
                    string.Join("\n", ordered));
            }
        }
    }
}
