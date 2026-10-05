using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;
using CutoutGate = ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate;
using VolumeGate = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate;

namespace ThinhThan.Tests.EditMode.InstanceArtCoverage
{
    /// <summary>
    /// IMP-105 coverage: the nine instance scenes (5 dungeons + finale +
    /// 3 competitive) exist with geometry-faithful layout markers, unique
    /// Addressable keys in the right groups, mirror parity for competitive
    /// spaces, group byte budgets, provenance rows for every shipped
    /// texture, style packs + tile seams (ADR-0076), folklore cards
    /// (ART-010), cutout/volume gates, and the AI tool record matching the
    /// owner setup (ADR-0072).
    /// </summary>
    public class InstanceArtCoverageTests
    {
        private const string FragmentRel =
            "client/Assets/Art/Provenance/fragments/instances.json";
        private const string ArtRootRel = "client/Assets/Art/Instances";
        private const string GeomRootRel = "server/internal/sim/spatial/maps";
        private const string GroupsRel =
            "client/Assets/AddressableAssetsData/AssetGroups";
        private const string StyleRootRel = "client/Assets/Art/StyleRef/instances";

        private const double MirrorToleranceMeters = 0.001;
        private const long GroupBudgetBytes = 25L * 1024 * 1024;

        private sealed class SpaceSpec
        {
            public string SpaceId = string.Empty;
            public string SceneRel = string.Empty;
            public string GroupFile = string.Empty;
            public string Pack = string.Empty;
        }

        private static readonly SpaceSpec[] Roster =
        {
            new SpaceSpec
            {
                SpaceId = "dungeon.den_tran",
                SceneRel = "client/Assets/Scenes/Dungeons/dungeon.den_tran.unity",
                GroupFile = "dungeon.den_tran.asset",
                Pack = "instances/dungeon_ruins",
            },
            new SpaceSpec
            {
                SpaceId = "dungeon.dinh_lang_bo_hoang",
                SceneRel = "client/Assets/Scenes/Dungeons/dungeon.dinh_lang_bo_hoang.unity",
                GroupFile = "dungeon.dinh_lang_bo_hoang.asset",
                Pack = "instances/dungeon_ruins",
            },
            new SpaceSpec
            {
                SpaceId = "dungeon.hang_ma_tranh",
                SceneRel = "client/Assets/Scenes/Dungeons/dungeon.hang_ma_tranh.unity",
                GroupFile = "dungeon.hang_ma_tranh.asset",
                Pack = "instances/dungeon_ruins",
            },
            new SpaceSpec
            {
                SpaceId = "dungeon.mieu_ba_trong_rung",
                SceneRel = "client/Assets/Scenes/Dungeons/dungeon.mieu_ba_trong_rung.unity",
                GroupFile = "dungeon.mieu_ba_trong_rung.asset",
                Pack = "instances/dungeon_ruins",
            },
            new SpaceSpec
            {
                SpaceId = "dungeon.xom_chim",
                SceneRel = "client/Assets/Scenes/Dungeons/dungeon.xom_chim.unity",
                GroupFile = "dungeon.xom_chim.asset",
                Pack = "instances/dungeon_ruins",
            },
            new SpaceSpec
            {
                SpaceId = "instance.finale.than_trung",
                SceneRel = "client/Assets/Scenes/Finale/instance.finale.than_trung.unity",
                GroupFile = "dungeon.finale.asset",
                Pack = "instances/finale_omen",
            },
            new SpaceSpec
            {
                SpaceId = "map.pvp.duel_court",
                SceneRel = "client/Assets/Scenes/Competitive/map.pvp.duel_court.unity",
                GroupFile = "pvp.shared.asset",
                Pack = "instances/competitive_arena",
            },
            new SpaceSpec
            {
                SpaceId = "map.pvp.five_element_arena",
                SceneRel = "client/Assets/Scenes/Competitive/map.pvp.five_element_arena.unity",
                GroupFile = "pvp.shared.asset",
                Pack = "instances/competitive_arena",
            },
            new SpaceSpec
            {
                SpaceId = "map.guild_war.five_seal_conflict",
                SceneRel = "client/Assets/Scenes/Competitive/map.guild_war.five_seal_conflict.unity",
                GroupFile = "pvp.shared.asset",
                Pack = "instances/competitive_arena",
            },
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

        private static RegisterJson.Node Fragment()
        {
            return RegisterJson.Parse(Read(FragmentRel));
        }

        /// <summary>fileID of GameObject -> name; fileID of Transform ->
        /// (gameObject fileID, x, y) parsed from a Unity scene YAML.</summary>
        private sealed class SceneObjects
        {
            public readonly Dictionary<long, string> Names =
                new Dictionary<long, string>();
            public readonly Dictionary<long, long> TransformOwner =
                new Dictionary<long, long>();
            public readonly Dictionary<long, (float x, float y)> TransformPos =
                new Dictionary<long, (float x, float y)>();
            public readonly List<(long go, float x, float y)> Positions =
                new List<(long go, float x, float y)>();
            public int SpriteRenderers;
            public int ColliderLike;
            public string SpaceId = string.Empty;
            public float BoundsMaxX;
            public float BoundsMaxY;
        }

        private static SceneObjects ParseScene(string rel)
        {
            var text = Read(rel);
            var s = new SceneObjects();
            var goBlocks = Regex.Matches(text,
                @"--- !u!1 &(\d+)\nGameObject:(.*?)(?=--- !u!|\z)",
                RegexOptions.Singleline);
            foreach (Match m in goBlocks)
            {
                var id = long.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
                var nm = Regex.Match(m.Groups[2].Value, @"m_Name: ([^\n]*)");
                s.Names[id] = nm.Success ? nm.Groups[1].Value.Trim() : string.Empty;
            }
            var trBlocks = Regex.Matches(text,
                @"--- !u!4 &(\d+)\nTransform:(.*?)(?=--- !u!|\z)",
                RegexOptions.Singleline);
            foreach (Match m in trBlocks)
            {
                var id = long.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
                var body = m.Groups[2].Value;
                var owner = Regex.Match(body, @"m_GameObject: \{fileID: (\d+)\}");
                var pos = Regex.Match(body,
                    @"m_LocalPosition: \{x: ([-\d.eE+]+), y: ([-\d.eE+]+), z: [-\d.eE+]+\}");
                Assert.IsTrue(owner.Success && pos.Success, "malformed transform in " + rel);
                var goId = long.Parse(owner.Groups[1].Value, CultureInfo.InvariantCulture);
                var x = float.Parse(pos.Groups[1].Value, CultureInfo.InvariantCulture);
                var y = float.Parse(pos.Groups[2].Value, CultureInfo.InvariantCulture);
                s.TransformOwner[id] = goId;
                s.TransformPos[id] = (x, y);
                s.Positions.Add((goId, x, y));
            }
            s.SpriteRenderers = Regex.Matches(text, @"--- !u!212 &").Count;
            s.ColliderLike = Regex.Matches(text,
                @"!u!(?:56|58|60|61|64|65|66|68|136|197|205|213) &|ServerGeometry").Count;
            var meta = Regex.Match(text,
                @"spaceId: ([^\n]+)\s*\n\s*spaceKind: [^\n]+\n\s*layoutProfile: [^\n]+\n\s*boundsMaxX: ([-\d.eE+]+)\s*\n\s*boundsMaxY: ([-\d.eE+]+)");
            if (meta.Success)
            {
                s.SpaceId = meta.Groups[1].Value.Trim();
                s.BoundsMaxX = float.Parse(meta.Groups[2].Value, CultureInfo.InvariantCulture);
                s.BoundsMaxY = float.Parse(meta.Groups[3].Value, CultureInfo.InvariantCulture);
            }
            return s;
        }

        private static string NameOf(SceneObjects s, long goId)
        {
            return s.Names.TryGetValue(goId, out var n) ? n : string.Empty;
        }

        private static RegisterJson.Node Geom(string spaceId)
        {
            return RegisterJson.Parse(
                Read(GeomRootRel + "/" + spaceId + ".geom.json"));
        }

        private static List<string> TextureFiles()
        {
            var abs = Path.Combine(RepoRoot(),
                ArtRootRel.Replace('/', Path.DirectorySeparatorChar));
            Assert.IsTrue(Directory.Exists(abs), "Art/Instances missing");
            var rel = new List<string>();
            foreach (var f in Directory.GetFiles(abs, "*.png",
                SearchOption.AllDirectories))
            {
                rel.Add(Path.GetFullPath(f)
                    .Replace(RepoRoot() + Path.DirectorySeparatorChar, string.Empty)
                    .Replace(Path.DirectorySeparatorChar, '/'));
            }
            rel.Sort(StringComparer.Ordinal);
            return rel;
        }

        private static string SpaceOf(string rel)
        {
            var dir = Path.GetDirectoryName(rel)!.Replace('\\', '/');
            var leaf = dir.Substring(dir.LastIndexOf('/') + 1);
            return leaf == "competitive" ? CompetitiveOwner(leaf, rel) : leaf;
        }

        private static string CompetitiveOwner(string leaf, string rel)
        {
            var f = Path.GetFileName(rel);
            if (f.Contains("banner.duel"))
            {
                return "map.pvp.duel_court";
            }
            if (f.Contains("drum.guild"))
            {
                return "map.guild_war.five_seal_conflict";
            }
            return "map.pvp.five_element_arena";
        }

        private static CutoutGate.AssetClass ClassOf(string file)
        {
            var f = Path.GetFileName(file);
            if (f.StartsWith("tile.", StringComparison.Ordinal))
            {
                return CutoutGate.AssetClass.Tile;
            }
            if (f.StartsWith("bg.far.", StringComparison.Ordinal))
            {
                return CutoutGate.AssetClass.ParallaxFar;
            }
            if (f.StartsWith("bg.mid.", StringComparison.Ordinal))
            {
                return CutoutGate.AssetClass.ParallaxNear;
            }
            Assert.IsTrue(f.StartsWith("prop.", StringComparison.Ordinal),
                "unexpected texture naming " + f);
            return CutoutGate.AssetClass.Prop;
        }

        private static bool VolumeApplies(string file)
        {
            var cls = ClassOf(file);
            return cls == CutoutGate.AssetClass.Prop
                || cls == CutoutGate.AssetClass.ParallaxNear;
        }

        private static List<LabPixels.Lab> Palette(string pack)
        {
            var pid = pack.Substring(pack.IndexOf('/') + 1);
            var p = RegisterJson.Parse(
                Read(StyleRootRel + "/" + pid + "/palette.json"));
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

        private static string PackOf(string rel)
        {
            var sp = SpaceOf(rel);
            foreach (var s in Roster)
            {
                if (s.SpaceId == sp)
                {
                    return s.Pack;
                }
            }
            // shared competitive props live under competitive/ but carry
            // the competitive pack id.
            return "instances/competitive_arena";
        }

        [Test]
        public void TestNineSceneRoster()
        {
            Assert.AreEqual(9, Roster.Length, "roster covers exactly nine spaces");
            var ids = new HashSet<string>();
            foreach (var s in Roster)
            {
                Assert.IsTrue(ids.Add(s.SpaceId), "duplicate " + s.SpaceId);
                Assert.IsTrue(File.Exists(Path.Combine(RepoRoot(),
                    s.SceneRel.Replace('/', Path.DirectorySeparatorChar))),
                    "scene missing " + s.SceneRel);
                Assert.IsTrue(File.Exists(Path.Combine(RepoRoot(),
                    s.SceneRel.Replace('/', Path.DirectorySeparatorChar) + ".meta")),
                    "scene .meta missing " + s.SceneRel);
                var sc = ParseScene(s.SceneRel);
                Assert.AreEqual(s.SpaceId, sc.SpaceId,
                    s.SceneRel + " GeometryMeta spaceId");
                Assert.AreEqual(0, sc.ColliderLike,
                    s.SceneRel + " must not carry colliders/ServerGeometry");
                Assert.GreaterOrEqual(sc.SpriteRenderers, 4,
                    s.SceneRel + " single-bitmap scenes forbidden");
            }
        }

        [Test]
        public void TestInstanceAddressableKeys()
        {
            var seen = new HashSet<string>();
            foreach (var s in Roster)
            {
                var key = "asset." + s.SpaceId + ".scene";
                Assert.IsTrue(seen.Add(key), "duplicate key " + key);
                var group = Read(GroupsRel + "/" + s.GroupFile);
                var sceneGuid = Regex.Match(
                    Read(s.SceneRel + ".meta"), @"guid: ([0-9a-f]{32})");
                Assert.IsTrue(sceneGuid.Success, "no scene guid " + s.SceneRel);
                var entry = Regex.Escape(
                    "- m_GUID: " + sceneGuid.Groups[1].Value + "\n"
                    + "    m_Address: " + key);
                Assert.IsTrue(Regex.IsMatch(group, entry),
                    key + " must be registered in " + s.GroupFile
                    + " pointing at the scene asset");
            }
        }

        [Test]
        public void TestInstanceGeometryConsistency()
        {
            foreach (var s in Roster)
            {
                var geom = Geom(s.SpaceId);
                var sc = ParseScene(s.SceneRel);
                var gBounds = geom.Get("bounds_mm")!;
                Assert.AreEqual(
                    gBounds.Get("max_x")!.Num / 1000.0, sc.BoundsMaxX, 1e-4,
                    s.SpaceId + " boundsMaxX");
                Assert.AreEqual(
                    gBounds.Get("max_y")!.Num / 1000.0, sc.BoundsMaxY, 1e-4,
                    s.SpaceId + " boundsMaxY");
                Assert.AreEqual(geom.Get("space_id")!.Str, sc.SpaceId);

                var byName = new Dictionary<string, (float x, float y)>();
                foreach (var (go, x, y) in sc.Positions)
                {
                    var n = NameOf(sc, go);
                    if (n.Length > 0)
                    {
                        byName[n] = (x, y);
                    }
                }
                foreach (var seg in geom.Get("segments")!.Arr!)
                {
                    var prefix = "seg."
                        + seg.Get("kind")!.Str.ToLowerInvariant() + "."
                        + ((long)seg.Get("id")!.Num).ToString(
                            CultureInfo.InvariantCulture);
                    var a = prefix + ".a";
                    var b = prefix + ".b";
                    Assert.IsTrue(byName.ContainsKey(a), s.SpaceId + " missing " + a);
                    Assert.IsTrue(byName.ContainsKey(b), s.SpaceId + " missing " + b);
                    Assert.AreEqual(seg.Get("x1")!.Num / 1000.0,
                        byName[a].x, 1e-3, a + ".x");
                    Assert.AreEqual(seg.Get("y1")!.Num / 1000.0,
                        byName[a].y, 1e-3, a + ".y");
                    Assert.AreEqual(seg.Get("x2")!.Num / 1000.0,
                        byName[b].x, 1e-3, b + ".x");
                    Assert.AreEqual(seg.Get("y2")!.Num / 1000.0,
                        byName[b].y, 1e-3, b + ".y");
                }
                foreach (var an in geom.Get("anchors")!.Arr!)
                {
                    var id = an.Get("id")!.Str;
                    Assert.IsTrue(byName.ContainsKey(id),
                        s.SpaceId + " missing anchor " + id);
                    Assert.AreEqual(an.Get("x")!.Num / 1000.0,
                        byName[id].x, 1e-3, id + ".x");
                    Assert.AreEqual(an.Get("y")!.Num / 1000.0,
                        byName[id].y, 1e-3, id + ".y");
                }
                var regs = geom.Get("camera_regions")!.Arr!;
                for (var i = 0; i < regs.Count; i++)
                {
                    var mn = $"region.{i + 1}.min";
                    var mx = $"region.{i + 1}.max";
                    Assert.IsTrue(byName.ContainsKey(mn), s.SpaceId + " missing " + mn);
                    Assert.IsTrue(byName.ContainsKey(mx), s.SpaceId + " missing " + mx);
                    Assert.AreEqual(regs[i].Get("min_x")!.Num / 1000.0,
                        byName[mn].x, 1e-3);
                    Assert.AreEqual(regs[i].Get("min_y")!.Num / 1000.0,
                        byName[mn].y, 1e-3);
                    Assert.AreEqual(regs[i].Get("max_x")!.Num / 1000.0,
                        byName[mx].x, 1e-3);
                    Assert.AreEqual(regs[i].Get("max_y")!.Num / 1000.0,
                        byName[mx].y, 1e-3);
                }
            }
        }

        [Test]
        public void TestCompetitiveMirrorParity()
        {
            foreach (var s in Roster)
            {
                if (!s.SpaceId.StartsWith("map.", StringComparison.Ordinal))
                {
                    continue;
                }
                var sc = ParseScene(s.SceneRel);
                var axis = sc.BoundsMaxX / 2f;
                var arts = new List<(float x, float y)>();
                foreach (var (go, x, y) in sc.Positions)
                {
                    if (NameOf(sc, go).StartsWith("art.", StringComparison.Ordinal))
                    {
                        arts.Add((x, y));
                    }
                }
                Assert.Greater(arts.Count, 0, s.SpaceId + " has no art objects");
                var unpaired = new List<string>();
                foreach (var (x, y) in arts)
                {
                    if (Math.Abs(x - axis) <= MirrorToleranceMeters)
                    {
                        continue; // on-axis objects are self-mirrored
                    }
                    var mx = 2 * axis - x;
                    var ok = false;
                    foreach (var (x2, y2) in arts)
                    {
                        if (Math.Abs(x2 - mx) <= MirrorToleranceMeters
                            && Math.Abs(y2 - y) <= MirrorToleranceMeters)
                        {
                            ok = true;
                            break;
                        }
                    }
                    if (!ok)
                    {
                        unpaired.Add($"({x:F3},{y:F3})");
                    }
                }
                Assert.AreEqual(0, unpaired.Count,
                    s.SpaceId + " unpaired art: " + string.Join(", ", unpaired));
            }
        }

        [Test]
        public void TestInstanceBundleBudget()
        {
            var byGroup = new Dictionary<string, long>();
            foreach (var s in Roster)
            {
                var dir = Path.Combine(RepoRoot(),
                    (ArtRootRel + "/" + s.SpaceId)
                    .Replace('/', Path.DirectorySeparatorChar));
                if (!Directory.Exists(dir))
                {
                    // shared competitive props live under competitive/
                    continue;
                }
                long sum = 0;
                foreach (var f in Directory.GetFiles(dir, "*.png"))
                {
                    sum += new FileInfo(f).Length;
                }
                byGroup[s.GroupFile] = (byGroup.TryGetValue(s.GroupFile,
                    out var v) ? v : 0) + sum;
            }
            var shared = Path.Combine(RepoRoot(),
                (ArtRootRel + "/competitive")
                .Replace('/', Path.DirectorySeparatorChar));
            if (Directory.Exists(shared))
            {
                long sum = 0;
                foreach (var f in Directory.GetFiles(shared, "*.png"))
                {
                    sum += new FileInfo(f).Length;
                }
                byGroup["pvp.shared.asset"] += sum;
            }
            foreach (var kv in byGroup)
            {
                Assert.LessOrEqual(kv.Value, GroupBudgetBytes,
                    kv.Key + " texture bytes over 25MB budget: " + kv.Value);
            }
        }

        [Test]
        public void TestInstanceProvenanceCoverage()
        {
            var rows = Fragment().Get("assets")!.Arr!;
            var byFile = new Dictionary<string, RegisterJson.Node>();
            foreach (var r in rows)
            {
                var fp = r.Get("file_path")!.Str;
                Assert.IsFalse(byFile.ContainsKey(fp), "duplicate row " + fp);
                byFile[fp] = r;
            }
            var files = TextureFiles();
            Assert.AreEqual(files.Count, rows.Count,
                "every shipped texture needs exactly one provenance row");
            foreach (var rel in files)
            {
                Assert.IsTrue(byFile.ContainsKey(rel),
                    rel + " has no provenance row");
                var r = byFile[rel];
                Assert.AreEqual("AI_CREATED", r.Get("source_kind")!.Str);
                Assert.AreEqual("PENDING", r.Get("review_state")!.Str);
                Assert.IsTrue(r.Get("style_pack_id")!.Str
                    .StartsWith("instances/", StringComparison.Ordinal),
                    rel + " style_pack_id fragment must be instances/");
                var abs = Path.Combine(RepoRoot(),
                    rel.Replace('/', Path.DirectorySeparatorChar));
                Assert.AreEqual(ProvenanceValidator.Sha256File(abs),
                    r.Get("final_sha256")!.Str,
                    rel + " final_sha256 mismatch");
                var terms = "client/Assets/Art/Provenance/terms/instances/"
                    + r.Get("generation_record")!.Get("terms_snapshot_sha256")!.Str
                    + ".txt";
                Assert.IsTrue(File.Exists(Path.Combine(RepoRoot(),
                    terms.Replace('/', Path.DirectorySeparatorChar))),
                    "terms snapshot missing for " + rel);
            }
        }

        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
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
                Assert.IsTrue(gr.Get("model_id")!.Str.Length > 0,
                    "model_id must be named");
                tools.Add(gr.Get("tool")!.Str);
                modelIds.Add(gr.Get("model_id")!.Str);
                Assert.AreEqual("https://openai.com/policies/terms-of-use/",
                    gr.Get("terms_uri")!.Str, "terms_uri must be the OpenAI terms");
            }
            Assert.AreEqual(1, tools.Count,
                "all instance textures must name the single owner-approved tool");
            Assert.AreEqual(1, modelIds.Count,
                "all instance textures must name the single owner-approved model");
        }

        [Test]
        public void TestInstanceStylePacksAndTileSeams()
        {
            foreach (var pack in new[]
            {
                "dungeon_ruins", "finale_omen", "competitive_arena",
            })
            {
                var dir = StyleRootRel + "/" + pack;
                var style = Read(dir + "/style.md");
                Assert.Greater(style.Length, 200,
                    pack + " style.md must document the pack");
                var p = RegisterJson.Parse(Read(dir + "/palette.json"));
                Assert.AreEqual("instances/" + pack, p.Get("pack_id")!.Str);
                Assert.LessOrEqual(p.Get("delta_e00_max")!.Num, 8.0);
                Assert.GreaterOrEqual(p.Get("gate_min_coverage")!.Num, 0.85);
                Assert.Greater(p.Get("families")!.Obj!.Count, 0,
                    pack + " palette families");
            }
            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                var sp = r.Get("style_pack_id")!.Str;
                Assert.IsTrue(
                    sp == "instances/dungeon_ruins"
                        || sp == "instances/finale_omen"
                        || sp == "instances/competitive_arena",
                    sp + " must name a declared instances pack");
            }

            // ART-007 tile seam: mean deltaE00 between opposing edges <= 2.
            foreach (var rel in TextureFiles())
            {
                if (!Path.GetFileName(rel).StartsWith("tile.",
                    StringComparison.Ordinal))
                {
                    continue;
                }
                var img = ArtRuleFixtures.LoadPng(Path.Combine(RepoRoot(),
                    rel.Replace('/', Path.DirectorySeparatorChar)));
                var seams = TileSeams(img);
                Assert.LessOrEqual(seams.vert, 2.0,
                    rel + " left/right seam " + seams.vert.ToString("F3",
                        CultureInfo.InvariantCulture));
                Assert.LessOrEqual(seams.horiz, 2.0,
                    rel + " top/bottom seam " + seams.horiz.ToString("F3",
                        CultureInfo.InvariantCulture));
            }
        }

        private static (double vert, double horiz) TileSeams(LabPixels.Image img)
        {
            int w = img.Width, h = img.Height;
            double vSum = 0, hSum = 0;
            for (var y = 0; y < h; y++)
            {
                var a = img.At(0, y);
                var b = img.At(w - 1, y);
                vSum += LabPixels.DeltaE00(
                    LabPixels.ToLab(a.R, a.G, a.B),
                    LabPixels.ToLab(b.R, b.G, b.B));
            }
            for (var x = 0; x < w; x++)
            {
                var a = img.At(x, 0);
                var b = img.At(x, h - 1);
                hSum += LabPixels.DeltaE00(
                    LabPixels.ToLab(a.R, a.G, a.B),
                    LabPixels.ToLab(b.R, b.G, b.B));
            }
            return (vSum / h, hSum / w);
        }

        [Test]
        public void TestInstanceFolkloreCards()
        {
            foreach (var r in Fragment().Get("assets")!.Arr!)
            {
                var card = r.Get("folklore_card");
                Assert.IsNotNull(card,
                    r.Get("file_path")!.Str + " requires folklore_card (ART-010)");
                Assert.Greater(card!.Get("source_tales")!.Arr!.Count, 0,
                    "source_tales");
                Assert.Greater(card.Get("regional_variants")!.Arr!.Count, 0,
                    "regional_variants");
                Assert.Greater(card.Get("motifs_checked")!.Arr!.Count, 0,
                    "motifs_checked");
            }
        }

        private sealed class GateJob
        {
            public string Rel = string.Empty;
            public LabPixels.Image Img = null!;
            public CutoutGate.AssetClass Cls;
        }

        [Test]
        [Timeout(1200000)]
        public void TestInstanceCutoutGateZeroViolations()
        {
            var fails = new ConcurrentBag<string>();
            var jobs = new List<GateJob>();
            foreach (var rel in TextureFiles())
            {
                jobs.Add(new GateJob
                {
                    Rel = rel,
                    Img = ArtRuleFixtures.LoadPng(Path.Combine(RepoRoot(),
                        rel.Replace('/', Path.DirectorySeparatorChar))),
                    Cls = ClassOf(rel),
                });
            }
            Parallel.ForEach(jobs, j =>
            {
                var rep = CutoutGate.Measure(
                    j.Rel, j.Img, 0, 0, j.Img.Width, j.Img.Height, j.Cls, null,
                    false, false);
                if (rep.Violations.Count != 0)
                {
                    fails.Add(j.Rel + ": " + string.Join("; ", rep.Violations));
                }
            });
            var ordered = new List<string>(fails);
            ordered.Sort(StringComparer.Ordinal);
            Assert.AreEqual(0, fails.Count,
                "cutout violations:\n" + string.Join("\n", ordered));
        }

        [Test]
        [Timeout(1200000)]
        public void TestInstanceVolumeDepthGateZeroViolations()
        {
            var fails = new ConcurrentBag<string>();
            var jobs = new List<GateJob>();
            foreach (var rel in TextureFiles())
            {
                if (!VolumeApplies(rel))
                {
                    continue;
                }
                jobs.Add(new GateJob
                {
                    Rel = rel,
                    Img = ArtRuleFixtures.LoadPng(Path.Combine(RepoRoot(),
                        rel.Replace('/', Path.DirectorySeparatorChar))),
                    Cls = ClassOf(rel),
                });
            }
            Parallel.ForEach(jobs, j =>
            {
                var rep = VolumeGate.Measure(j.Rel, j.Img, null, 0, 0);
                if (rep.Violations.Count != 0)
                {
                    fails.Add(j.Rel + ": " + string.Join("; ", rep.Violations));
                }
                VolumeGate.Silhouette(j.Img, null, out var inS, out var lab);
                var s = new List<int>();
                for (var i = 0; i < j.Img.Width * j.Img.Height; i++)
                {
                    if (inS[i])
                    {
                        s.Add(i);
                    }
                }
                var cov = VolumeGate.PaletteCoverage(s, lab, Palette(PackOf(j.Rel)));
                if (double.IsNaN(cov) || cov < VolumeGate.PaletteCoverageMin)
                {
                    fails.Add(j.Rel + ": palette coverage " + cov.ToString("F3",
                        CultureInfo.InvariantCulture));
                }
            });
            var ordered = new List<string>(fails);
            ordered.Sort(StringComparer.Ordinal);
            Assert.AreEqual(0, fails.Count,
                "volume violations:\n" + string.Join("\n", ordered));
        }
    }
}
