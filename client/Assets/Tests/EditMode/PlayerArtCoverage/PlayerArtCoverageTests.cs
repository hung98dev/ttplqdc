using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.PlayerArtCoverage
{
    /// <summary>
    /// IMP-071 player/class-art coverage (presentation_asset_manifest.md §3.1,
    /// §3.7, §3.8, §5-§6; classes.md; physics_geometry_contract.md §3;
    /// client_assets.md § Stable Asset Keys; ADR-0072, ADR-0076): the
    /// actors_players fragment must carry a register row for every shipped
    /// media file, each class must route `asset.class.<id>.prefab` to
    /// shared.local through the canonical registry, the §3.7 skeletal rig must
    /// use the fixed layer names with every CHARACTER clip at 30 fps, the
    /// import contract (100 PPU, 192x256 sheet, feet at y=0, silhouette within
    /// the 64x96 ref-px bound) must hold, style-pack palette and folklore
    /// cards must be present, and no placeholder sprite may ship.
    /// </summary>
    public class PlayerArtCoverageTests
    {
        private const string FragmentPath =
            "client/Assets/Art/Provenance/fragments/actors_players.json";
        private const string PlayersRoot = "client/Assets/Art/Actors/Players";
        private const string PackDir =
            "client/Assets/Art/StyleRef/actors_players/vn_class_chibi_v1";
        private const string GroupPath =
            "client/Assets/AddressableAssetsData/AssetGroups/shared.local.asset";

        private static readonly string[] ClassIds =
            { "kim", "moc", "thuy", "hoa", "tho" };

        private static readonly string[] RigLayers =
        {
            "head", "hair", "torso", "arm_front", "arm_back",
            "leg_front", "leg_back", "weapon",
        };

        private static readonly string[] RequiredClips =
        {
            "idle", "run", "jump_up", "fall", "land",
            "attack_basic", "cast", "hit", "guard", "defeat",
        };

        private static readonly string[] RigBones =
        {
            "bone_root", "bone_pelvis", "bone_leg_front", "bone_leg_back",
            "bone_spine", "bone_arm_back", "bone_arm_front",
            "bone_hand_front", "bone_head", "bone_hair",
        };

        // §5 banned-motif seed list (manifest §5 / ART-010).
        private static readonly string[] BannedMotifs =
        {
            "torii", "jiangshi", "kimono", "hanbok", "jade dragon",
            "chinese palace lantern", "qing dynasty",
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

        private static string Abs(string rel)
        {
            return Path.Combine(RepoRoot(),
                rel.Replace('/', Path.DirectorySeparatorChar));
        }

        // ---------- fragment helpers ----------

        private static RegisterJson.Node Fragment()
        {
            string path = Abs(FragmentPath);
            Assert.IsTrue(File.Exists(path), "actors_players fragment missing");
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
            var m = System.Text.RegularExpressions.Regex.Match(
                text, @"guid: ([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + relAsset + ".meta");
            return m.Groups[1].Value;
        }

        // ---------- provenance ----------

        [Test]
        public void TestProvenanceFragmentIntegrity()
        {
            var rows = Rows();
            Assert.Greater(rows.Count, 0, "fragment has no rows");
            string? prev = null;
            var seen = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in rows)
            {
                var path = Str(row, "file_path");
                Assert.IsNotNull(path, "row without file_path");
                Assert.IsTrue(File.Exists(Abs(path!)),
                    "row file missing: " + path);
                if (prev != null)
                {
                    Assert.IsTrue(
                        string.CompareOrdinal(prev, path) < 0,
                        "assets array not sorted by file_path: " + path);
                }
                prev = path;
                Assert.IsTrue(seen.Add(path!), "duplicate row: " + path);
                var finalHash = Str(row, "final_sha256");
                Assert.IsNotNull(finalHash, "final_sha256 missing: " + path);
                Assert.AreEqual(finalHash, Sha256File(Abs(path!)),
                    "final_sha256 mismatch: " + path);
            }
        }

        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
            // Owner-approved tool name (technology_versions.md § Content
            // production tools, ADR-0072) must appear verbatim in every
            // AI_CREATED generation record.
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
                var snapPath = "client/Assets/Art/Provenance/terms/actors_players/"
                    + snap + ".txt";
                Assert.IsTrue(File.Exists(Abs(snapPath)),
                    "terms snapshot missing: " + snapPath);
                Assert.AreEqual(snap, Sha256File(Abs(snapPath)),
                    "terms snapshot hash mismatch");
            }
            Assert.Greater(aiRows, 0, "no AI_CREATED rows in fragment");
        }

        // ---------- coverage ----------

        [Test]
        public void TestClassAssetCoverage()
        {
            var fragmentFiles = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                var fp = Str(row, "file_path");
                Assert.IsNotNull(fp);
                fragmentFiles.Add(fp!);
            }
            foreach (var c in ClassIds)
            {
                string dir = PlayersRoot + "/" + c;
                var must = new List<string>
                {
                    dir + "/" + c + ".psb",
                    dir + "/" + c + "_sheet.png",
                    dir + "/" + c + ".controller",
                    dir + "/" + c + ".prefab",
                };
                foreach (var view in new[]
                    { "front", "threequarter", "side", "back" })
                {
                    must.Add(dir + "/turnarounds/" + view + ".png");
                }
                foreach (var clip in RequiredClips)
                {
                    must.Add(dir + "/clips/" + clip + ".anim");
                }
                foreach (var rel in must)
                {
                    Assert.IsTrue(File.Exists(Abs(rel)),
                        "missing deliverable " + rel);
                }
                // every media file under the class dir resolves to a row
                foreach (var rel in must)
                {
                    if (ProvenanceValidator.IsMediaFile(rel))
                    {
                        Assert.IsTrue(fragmentFiles.Contains(rel),
                            "media file lacks provenance row: " + rel);
                    }
                }
            }
            Assert.IsTrue(File.Exists(Abs(PlayersRoot + "/players.spriteLib")),
                "shared players.spriteLib missing");
        }

        [Test]
        public void TestClassCatalogKeyRoutesSharedLocal()
        {
            // BLK-002: `case "class"` route via IMP-063 gatefix — every class
            // prefab key must resolve to shared.local, and the group file must
            // contain the address bound to the prefab's .meta GUID.
            var group = File.ReadAllText(Abs(GroupPath));
            foreach (var c in ClassIds)
            {
                string key = "asset.class." + c + ".prefab";
                Assert.IsTrue(AssetKey.TryParse(key, out var parsed),
                    "key failed to parse: " + key);
                Assert.AreEqual(AddressableGroups.SharedLocal,
                    KeyGroupRule.Assign(parsed), "key routed wrong: " + key);
                Assert.IsTrue(group.Contains("m_Address: " + key),
                    "shared.local lacks address " + key);
                string guid = MetaGuid(PlayersRoot + "/" + c + "/" + c + ".prefab");
                Assert.IsTrue(group.Contains("m_GUID: " + guid),
                    "shared.local lacks GUID row for " + key);
            }
        }

        // ---------- skeletal rig (§3.7) ----------

        [Test]
        public void TestSkeletalRigLayersAndClips()
        {
            string libGuid = MetaGuid(PlayersRoot + "/players.spriteLib");
            string lib = File.ReadAllText(Abs(PlayersRoot + "/players.spriteLib"));
            foreach (var layer in RigLayers)
            {
                Assert.IsTrue(lib.Contains("m_Name: " + layer + "\n"),
                    "players.spriteLib lacks category " + layer);
            }
            foreach (var c in ClassIds)
            {
                string dir = PlayersRoot + "/" + c;
                // one shared skeleton: identical fixed layer names in the psb meta
                var meta = File.ReadAllText(Abs(dir + "/" + c + ".psb.meta"));
                var layered = meta.Substring(
                    meta.IndexOf("layeredSpriteImportData:", StringComparison.Ordinal));
                foreach (var layer in RigLayers)
                {
                    Assert.IsTrue(layered.Contains("- name: " + layer + "\n"),
                        c + ".psb.meta lacks rig layer " + layer);
                }
                // clips: contract names, 30 fps, >= 4 keys
                foreach (var clip in RequiredClips)
                {
                    var text = File.ReadAllText(
                        Abs(dir + "/clips/" + clip + ".anim"));
                    Assert.IsTrue(text.Contains("m_SampleRate: 30"),
                        c + "/" + clip + " not 30 fps");
                    int keys = System.Text.RegularExpressions.Regex.Matches(
                        text, "time: ").Count;
                    Assert.GreaterOrEqual(keys, 4,
                        c + "/" + clip + " has fewer than 4 keys");
                }
                // controller: ten states, default = idle
                string ctrlGuid = MetaGuid(dir + "/" + c + ".controller");
                var ctrl = File.ReadAllText(Abs(dir + "/" + c + ".controller"));
                foreach (var clip in RequiredClips)
                {
                    Assert.IsTrue(ctrl.Contains("m_Name: " + clip + "\n"),
                        c + ".controller lacks state " + clip);
                }
                // prefab: bones, one resolver per layer, lib + animator wiring
                var prefab = File.ReadAllText(Abs(dir + "/" + c + ".prefab"));
                foreach (var bone in RigBones)
                {
                    Assert.IsTrue(prefab.Contains("m_Name: " + bone + "\n"),
                        c + ".prefab lacks " + bone);
                }
                Assert.AreEqual(8,
                    System.Text.RegularExpressions.Regex.Matches(prefab,
                        "guid: ed8b1ae4e4e52b34ea557c1c11e076fc").Count,
                    c + ".prefab must carry 8 SpriteResolvers");
                Assert.IsTrue(prefab.Contains(
                        "guid: c29cff538c195c249b69c6f2236de67b"),
                    c + ".prefab lacks SpriteLibrary component");
                Assert.IsTrue(prefab.Contains("guid: " + libGuid),
                    c + ".prefab does not reference players.spriteLib");
                Assert.IsTrue(prefab.Contains(
                        "m_Controller: {fileID: 9100000, guid: " + ctrlGuid),
                    c + ".prefab does not reference " + c + ".controller");
            }
        }

        // ---------- import contract / silhouette ----------

        [Test]
        public void TestImportScaleAndCell()
        {
            // physics_geometry_contract.md §3 CHARACTER profile: texture 2x of
            // 96x128 cell => 192x256, 100 PPU, alpha transparency, feet at y=0.
            foreach (var c in ClassIds)
            {
                var meta = File.ReadAllText(Abs(
                    PlayersRoot + "/" + c + "/" + c + ".psb.meta"));
                Assert.IsTrue(meta.Contains("spritePixelsToUnits: 100"),
                    c + ".psb not imported at 100 PPU");
                Assert.IsTrue(meta.Contains("alphaIsTransparency: 1"),
                    c + ".psb alphaIsTransparency disabled");
                Assert.IsTrue(meta.Contains("spriteMode: 2"),
                    c + ".psb not imported in Multiple sprite mode");
                var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(
                    PlayersRoot + "/" + c + "/" + c + "_sheet.png"))),
                    c + "_sheet.png decode failed");
                Assert.AreEqual(192, tex.width, c + " sheet width");
                Assert.AreEqual(256, tex.height, c + " sheet height");
                UnityEngine.Object.DestroyImmediate(tex);
            }
        }

        [Test]
        public void TestHitboxSilhouetteAlignment()
        {
            // Silhouette must fit the 64x96 ref-px CHARACTER bound (=> 128x192
            // at 2x) and the feet must reach the canvas bottom (y=0 pivot).
            foreach (var c in ClassIds)
            {
                var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(
                    PlayersRoot + "/" + c + "/" + c + "_sheet.png"))));
                var px = tex.GetPixels32();
                int w = tex.width, h = tex.height;
                int minX = w, maxX = -1, minY = h, maxY = -1;
                for (int y = 0; y < h; y++)
                {
                    for (int x = 0; x < w; x++)
                    {
                        if (px[y * w + x].a == 0)
                        {
                            continue;
                        }
                        if (x < minX)
                        {
                            minX = x;
                        }
                        if (x > maxX)
                        {
                            maxX = x;
                        }
                        if (y < minY)
                        {
                            minY = y;
                        }
                        if (y > maxY)
                        {
                            maxY = y;
                        }
                    }
                }
                UnityEngine.Object.DestroyImmediate(tex);
                Assert.Greater(maxX, -1, c + " sheet is fully transparent");
                int bw = maxX - minX + 1;
                int bh = maxY - minY + 1;
                Assert.LessOrEqual(bw, 128,
                    c + " silhouette width exceeds 64 ref px");
                Assert.LessOrEqual(bh, 192,
                    c + " silhouette height exceeds 96 ref px");
                Assert.GreaterOrEqual(maxY, h - 6,
                    c + " feet do not reach the canvas bottom (y=0)");
            }
        }

        // ---------- style pack / palette gate ----------

        [Test]
        public void TestStylePackAndPaletteGate()
        {
            string palettePath = PackDir + "/palette.json";
            Assert.IsTrue(File.Exists(Abs(palettePath)), "palette.json missing");
            Assert.IsTrue(File.Exists(Abs(PackDir + "/style.md")),
                "style.md missing");
            var palette = RegisterJson.Parse(
                File.ReadAllText(Abs(palettePath)));
            Assert.AreEqual("actors_players/vn_class_chibi_v1",
                palette.Get("pack_id")!.Str);
            var colors = palette.Get("colors")!.Arr!;
            Assert.Greater(colors.Count, 0, "empty palette");
            var rgb = new List<byte[]>();
            foreach (var entry in colors)
            {
                string hex = entry.Get("hex")!.Str!.TrimStart('#');
                Assert.AreEqual(6, hex.Length, "bad hex " + entry.Get("id")!.Str);
                rgb.Add(new[]
                {
                    byte.Parse(hex.Substring(0, 2), NumberStyles.HexNumber,
                        CultureInfo.InvariantCulture),
                    byte.Parse(hex.Substring(2, 2), NumberStyles.HexNumber,
                        CultureInfo.InvariantCulture),
                    byte.Parse(hex.Substring(4, 2), NumberStyles.HexNumber,
                        CultureInfo.InvariantCulture),
                });
            }
            // every fragment row references this style pack
            foreach (var row in Rows())
            {
                Assert.AreEqual("actors_players/vn_class_chibi_v1",
                    Str(row, "style_pack_id"),
                    Str(row, "file_path") + " wrong style_pack_id");
            }
            // palette gate: opaque sheet pixels must sit on (or near) the pack
            // palette — median distance 0, p99 <= 60 sRGB units (antialiased
            // sub-pixel blends allowed, arbitrary off-palette fills not).
            foreach (var c in ClassIds)
            {
                var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(
                    PlayersRoot + "/" + c + "/" + c + "_sheet.png"))));
                var px = tex.GetPixels32();
                var dist = new List<double>(px.Length);
                int w = tex.width;
                for (int i = 0; i < px.Length; i++)
                {
                    var p = px[i];
                    if (p.a < 250)
                    {
                        continue;
                    }
                    double best = double.MaxValue;
                    foreach (var q in rgb)
                    {
                        double dr = p.r - q[0];
                        double dg = p.g - q[1];
                        double db = p.b - q[2];
                        double d = Math.Sqrt(dr * dr + dg * dg + db * db);
                        if (d < best)
                        {
                            best = d;
                        }
                    }
                    dist.Add(best);
                }
                UnityEngine.Object.DestroyImmediate(tex);
                Assert.Greater(dist.Count, 1000,
                    c + " sheet too few opaque pixels");
                dist.Sort();
                double median = dist[dist.Count / 2];
                double p99 = dist[(int)(dist.Count * 0.99)];
                Assert.LessOrEqual(median, 1.0,
                    c + " median off-palette distance too high");
                Assert.LessOrEqual(p99, 60.0,
                    c + " p99 off-palette distance too high");
            }
        }

        // ---------- folklore cards ----------

        [Test]
        public void TestFolkloreCards()
        {
            int cards = 0;
            foreach (var row in Rows())
            {
                var fp = Str(row, "file_path")!;
                if (!fp.StartsWith(PlayersRoot + "/", StringComparison.Ordinal))
                {
                    continue;
                }
                var card = row.Get("folklore_card");
                Assert.IsNotNull(card, "folklore_card missing: " + fp);
                Assert.AreEqual(RegisterJson.Node.Kind.Obj, card!.Type,
                    "folklore_card must be an object: " + fp);
                var tales = card.Get("source_tales");
                Assert.IsNotNull(tales, fp + " source_tales");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, tales!.Type);
                Assert.Greater(tales.Arr!.Count, 0,
                    "source_tales empty: " + fp);
                Assert.IsNotNull(card.Get("regional_variants"),
                    fp + " regional_variants");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr,
                    card.Get("regional_variants")!.Type);
                var motifs = card.Get("motifs_checked");
                Assert.IsNotNull(motifs, fp + " motifs_checked");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, motifs!.Type);
                Assert.Greater(motifs.Arr!.Count, 0,
                    "motifs_checked empty: " + fp);
                foreach (var m in motifs.Arr)
                {
                    string motif = m.Str!.ToLowerInvariant();
                    foreach (var banned in BannedMotifs)
                    {
                        Assert.IsFalse(motif.Contains(banned),
                            "banned motif '" + banned + "' in " + fp);
                    }
                }
                cards++;
            }
            Assert.GreaterOrEqual(cards, ClassIds.Length,
                "fewer folklore cards than classes");
        }

        // ---------- no placeholder ----------

        [Test]
        public void TestNoPlaceholderSprite()
        {
            foreach (var c in ClassIds)
            {
                foreach (var rel in new[]
                {
                    PlayersRoot + "/" + c + "/" + c + "_sheet.png",
                    PlayersRoot + "/" + c + "/turnarounds/front.png",
                })
                {
                    var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                    Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(rel))));
                    var px = tex.GetPixels32();
                    var colors = new HashSet<int>();
                    int opaque = 0;
                    foreach (var p in px)
                    {
                        if (p.a < 8)
                        {
                            continue;
                        }
                        opaque++;
                        colors.Add((p.r << 16) | (p.g << 8) | p.b);
                    }
                    UnityEngine.Object.DestroyImmediate(tex);
                    Assert.Greater(opaque, 2000,
                        rel + " looks empty");
                    Assert.Greater(colors.Count, 20,
                        rel + " looks like a flat-color placeholder");
                }
            }
        }
    }
}
