using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;

namespace ThinhThan.Tests.EditMode.AudioAssetCoverage
{
    /// <summary>
    /// IMP-075 coverage for the produced SFX/BGM set: every release cue and
    /// scene BGM key resolves to a real licensed clip, import settings match
    /// the streaming/decompress contract of presentation_asset_manifest.md §1,
    /// per-group compressed sizes stay inside the 15 MB audio budgets, and the
    /// audio provenance fragment documents the FREE_LICENSED route with real
    /// sources, hashes and terms snapshots (ADR-0071/0072/0076).
    /// </summary>
    public class AudioAssetCoverageTests
    {
        private const string AudioRoot = "client/Assets/Audio";
        private const string FragmentPath =
            "client/Assets/Art/Provenance/fragments/audio.json";
        private const string TermsDir =
            "client/Assets/Art/Provenance/terms/audio";

        // The 14 release cues of presentation_asset_manifest.md §6.
        private static readonly string[] SfxCues =
        {
            "ui_confirm", "ui_cancel", "ui_error", "jump", "land",
            "basic_attack", "hit", "guard", "just_guard_success",
            "skill_cast", "boss_telegraph", "item_pickup",
            "quest_complete", "map_transfer",
        };

        private static readonly Dictionary<string, string[]> ZoneMapTable =
            new Dictionary<string, string[]>
            {
                { "lang_da", new[] { "dinh_lang", "bo_ruong", "ben_da", "go_ma" } },
                { "rung_u_minh", new[] { "xom_rung", "loi_tram", "rung_sau", "mieu_bo_hoang" } },
                { "ben_nuoc_den", new[] { "cho_ben", "bai_lau", "duong_ngap", "ben_do_cu" } },
                { "deo_may", new[] { "ban_chan_deo", "duong_rung", "khe_da", "rung_cam" } },
                { "thanh_co", new[] { "cong_ngoai", "duong_da", "hao_can", "den_tran" } },
                { "nui_thieng", new[] { "chan_nui", "rung_may", "suon_da", "cong_co" } },
            };

        // All 33 release spaces and their owning audio.bgm.* group.
        private static readonly (string space, string group)[] Scenes = BuildScenes();

        private static (string space, string group)[] BuildScenes()
        {
            var list = new List<(string, string)>();
            foreach (var zone in AddressableGroups.ZoneKeys)
            {
                foreach (var map in ZoneMaps(zone))
                {
                    list.Add(("map." + zone + "." + map, "audio.bgm." + zone));
                }
            }
            list.Add(("dungeon.dinh_lang_bo_hoang", "audio.bgm.lang_da"));
            list.Add(("dungeon.mieu_ba_trong_rung", "audio.bgm.rung_u_minh"));
            list.Add(("dungeon.xom_chim", "audio.bgm.ben_nuoc_den"));
            list.Add(("dungeon.hang_ma_tranh", "audio.bgm.deo_may"));
            list.Add(("dungeon.den_tran", "audio.bgm.thanh_co"));
            list.Add(("instance.finale.than_trung", "audio.bgm.nui_thieng"));
            list.Add(("map.pvp.duel_court", AddressableGroups.AudioBgmShared));
            list.Add(("map.pvp.five_element_arena", AddressableGroups.AudioBgmShared));
            list.Add(("map.guild_war.five_seal_conflict", AddressableGroups.AudioBgmShared));
            return list.ToArray();
        }

        private static string[] ZoneMaps(string zone)
        {
            return ZoneMapTable[zone];
        }

        // ---------- helpers ----------

        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(UnityEngine.Application.dataPath, ".."));
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

        private static string MetaGuid(string relAsset)
        {
            string p = Abs(relAsset + ".meta");
            Assert.IsTrue(File.Exists(p), relAsset + ".meta missing");
            var m = System.Text.RegularExpressions.Regex.Match(
                File.ReadAllText(p), @"guid: ([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + relAsset + ".meta");
            return m.Groups[1].Value;
        }

        /// <summary>Reads a group .asset and returns address -> m_GUID map.</summary>
        private static Dictionary<string, string> GroupEntries(string groupFile)
        {
            string path = Abs("client/Assets/AddressableAssetsData/AssetGroups/"
                + groupFile + ".asset");
            Assert.IsTrue(File.Exists(path), groupFile + ".asset missing");
            var map = new Dictionary<string, string>(StringComparer.Ordinal);
            var guidRe = new System.Text.RegularExpressions.Regex(
                @"- m_GUID: ([0-9a-f]{32})\s*\r?\n\s*m_Address: (\S+)");
            foreach (System.Text.RegularExpressions.Match m in
                guidRe.Matches(File.ReadAllText(path)))
            {
                map[m.Groups[2].Value] = m.Groups[1].Value;
            }
            return map;
        }

        private static string MetaSetting(string relAsset, string key)
        {
            var text = File.ReadAllText(Abs(relAsset + ".meta"));
            var m = System.Text.RegularExpressions.Regex.Match(
                text, @"^\s*" + key + @": (\S+)",
                System.Text.RegularExpressions.RegexOptions.Multiline);
            Assert.IsTrue(m.Success, key + " not found in " + relAsset + ".meta");
            return m.Groups[1].Value;
        }

        /// <summary>
        /// Duration of an Ogg file from the last Vorbis page granule position
        /// and the stream sample rate — no decode needed.
        /// </summary>
        private static double OggSeconds(string path)
        {
            var bytes = File.ReadAllBytes(path);
            Assert.GreaterOrEqual(bytes.Length, 58, "tiny ogg: " + path);
            // sample rate: bytes 40..43 of the identification header page.
            uint rate = BitConverter.ToUInt32(bytes, 40);
            Assert.Greater(rate, 0, "bad vorbis sample rate in " + path);
            // scan backwards for a real "OggS" page header with a sane granule
            // (a "OggS" byte pattern inside compressed payload is rejected by
            // the version byte and the duration sanity window).
            for (int i = bytes.Length - 28; i >= 0; i--)
            {
                if (bytes[i] != (byte)'O' || bytes[i + 1] != (byte)'g'
                    || bytes[i + 2] != (byte)'g' || bytes[i + 3] != (byte)'S'
                    || bytes[i + 4] != 0)
                {
                    continue;
                }
                ulong granule = BitConverter.ToUInt64(bytes, i + 6);
                if (granule == ulong.MaxValue)
                {
                    continue;
                }
                double seconds = granule / (double)rate;
                if (seconds > 0.005 && seconds < 7200)
                {
                    return seconds;
                }
            }
            Assert.Fail("no usable OggS page found in " + path);
            return 0;
        }

        // ---------- coverage ----------

        [Test]
        public void TestSfxCueRegistry()
        {
            var entries = GroupEntries("shared.local");
            foreach (var cue in SfxCues)
            {
                string key = "asset.sfx." + cue + ".clip";
                Assert.IsTrue(entries.ContainsKey(key),
                    "no shared.local entry for " + key);
                string rel = AudioRoot + "/Sfx/" + cue + ".ogg";
                Assert.AreEqual(MetaGuid(rel), entries[key],
                    key + " bound to wrong guid");
                Assert.IsTrue(AssetKey.TryParse(key, out var parsed),
                    "key failed to parse: " + key);
                Assert.AreEqual(AddressableGroups.SharedLocal,
                    KeyGroupRule.Assign(parsed), "key routed to wrong group: " + key);
            }
            Assert.AreEqual(SfxCues.Length, CountSfxEntries(entries),
                "unexpected extra asset.sfx.* entries in shared.local");
        }

        private static int CountSfxEntries(Dictionary<string, string> entries)
        {
            int n = 0;
            foreach (var k in entries.Keys)
            {
                if (k.StartsWith("asset.sfx.", StringComparison.Ordinal))
                {
                    n++;
                }
            }
            return n;
        }

        [Test]
        public void TestBgmSpaceCoverage()
        {
            Assert.AreEqual(33, Scenes.Length, "release scene count drifted");
            var groupCache = new Dictionary<string, Dictionary<string, string>>();
            foreach (var (space, group) in Scenes)
            {
                Assert.AreEqual(group, AddressableGroups.SpaceBgmGroup(space),
                    "space routed to wrong bgm group: " + space);
                string key = "asset." + space + ".bgm";
                Assert.IsTrue(AssetKey.TryParse(key, out var parsed),
                    "key failed to parse: " + key);
                Assert.AreEqual(group, KeyGroupRule.Assign(parsed),
                    "key routed to wrong group: " + key);
                if (!groupCache.TryGetValue(group, out var entries))
                {
                    entries = GroupEntries(group);
                    groupCache[group] = entries;
                }
                Assert.IsTrue(entries!.ContainsKey(key),
                    group + " has no entry for " + key);
                AssertResolvesToClip(entries[key], key, groupCache);
            }
        }

        /// <summary>
        /// Follows an entry guid to a real .ogg — directly, or through exactly
        /// one PresentationAlias hop whose target key is a clip entry.
        /// </summary>
        private void AssertResolvesToClip(string guid, string key,
            Dictionary<string, Dictionary<string, string>> groupCache)
        {
            foreach (var relOgg in AllOggUnder(AudioRoot))
            {
                if (MetaGuid(relOgg) == guid)
                {
                    return;
                }
            }
            // must be an alias asset
            var aliases = Directory.GetFiles(Abs(AudioRoot + "/Aliases"), "*.asset");
            foreach (var aliasPath in aliases)
            {
                string rel = Rel(aliasPath);
                if (MetaGuid(rel) != guid)
                {
                    continue;
                }
                var target = System.Text.RegularExpressions.Regex.Match(
                    File.ReadAllText(aliasPath), @"target_key: (\S+)");
                Assert.IsTrue(target.Success, "alias without target_key: " + rel);
                string tkey = target.Groups[1].Value;
                Assert.IsTrue(AssetKey.TryParse(tkey, out var tparsed),
                    "alias target failed to parse: " + tkey);
                string? tgroup = KeyGroupRule.Assign(tparsed);
                Assert.IsNotNull(tgroup, "alias target routed nowhere: " + tkey);
                if (!groupCache.TryGetValue(tgroup!, out var tent))
                {
                    tent = GroupEntries(tgroup!);
                    groupCache[tgroup!] = tent;
                }
                Assert.IsTrue(tent!.ContainsKey(tkey),
                    "alias target not registered: " + tkey + " -> " + tgroup);
                foreach (var relOgg2 in AllOggUnder(AudioRoot))
                {
                    if (MetaGuid(relOgg2) == tent[tkey])
                    {
                        return;
                    }
                }
                Assert.Fail("alias " + rel + " target " + tkey + " is not a clip");
            }
            Assert.Fail("key " + key + " bound to unknown guid " + guid);
        }

        private static string Rel(string abs)
        {
            return abs.Substring(RepoRoot().Length + 1)
                .Replace(Path.DirectorySeparatorChar, '/');
        }

        private static List<string> AllOggUnder(string relDir)
        {
            var list = new List<string>();
            var dir = Abs(relDir);
            if (!Directory.Exists(dir))
            {
                return list;
            }
            foreach (var f in Directory.GetFiles(dir, "*.ogg",
                SearchOption.AllDirectories))
            {
                list.Add(Rel(f));
            }
            list.Sort(StringComparer.Ordinal);
            return list;
        }

        // ---------- import settings / budgets ----------

        [Test]
        public void TestBgmStreamsNeverResident()
        {
            var oggs = AllOggUnder(AudioRoot + "/Bgm");
            Assert.Greater(oggs.Count, 0, "no bgm clips produced");
            foreach (var rel in oggs)
            {
                Assert.AreEqual("2", MetaSetting(rel, "loadType"),
                    rel + " must stream (loadType 2)");
                Assert.AreEqual("0", MetaSetting(rel, "preloadAudioData"),
                    rel + " must not preload");
                Assert.GreaterOrEqual(OggSeconds(Abs(rel)), 30.0,
                    rel + " too short to serve as looped scene BGM");
            }
        }

        [Test]
        public void TestSfxDecompressedInMemory()
        {
            var oggs = AllOggUnder(AudioRoot + "/Sfx");
            Assert.Greater(oggs.Count, 0, "no sfx clips produced");
            foreach (var rel in oggs)
            {
                Assert.AreEqual("0", MetaSetting(rel, "loadType"),
                    rel + " must decompress on load (loadType 0)");
                Assert.Less(OggSeconds(Abs(rel)), 10.0,
                    rel + " is longer than a one-shot cue");
            }
        }

        [Test]
        public void TestAudioGroupCompressedBudgets()
        {
            var sizes = new Dictionary<string, long>(StringComparer.Ordinal);
            foreach (var rel in AllOggUnder(AudioRoot))
            {
                string group = GuessOwnerGroup(rel);
                sizes.TryGetValue(group, out long cur);
                sizes[group] = cur + new FileInfo(Abs(rel)).Length;
            }
            foreach (var kv in sizes)
            {
                var budget = GroupBudgets.Budgets[kv.Key];
                Assert.LessOrEqual(kv.Value, budget.CompressedMaxBytes,
                    kv.Key + " compressed audio exceeds budget: " + kv.Value);
            }
        }

        private static string GuessOwnerGroup(string relOgg)
        {
            // Mirrors the registration map produced in the asset files.
            var part = relOgg.Split('/');
            if (part[3] == "Sfx")
            {
                return AddressableGroups.SharedLocal;
            }
            return part[4] == "shared" ? AddressableGroups.AudioBgmShared
                : "audio.bgm." + part[4];
        }

        [Test]
        public void TestNoPlaceholderAudio()
        {
            var oggs = AllOggUnder(AudioRoot);
            Assert.AreEqual(36, oggs.Count, "expected 22 bgm + 14 sfx clips");
            foreach (var rel in oggs)
            {
                var info = new FileInfo(Abs(rel));
                long floor = rel.Contains("/Bgm/") ? 150_000 : 3_000;
                Assert.GreaterOrEqual(info.Length, floor,
                    rel + " below real-content size floor");
                var head = new byte[4];
                using (var fs = File.OpenRead(Abs(rel)))
                {
                    fs.Read(head, 0, 4);
                }
                Assert.AreEqual("OggS", Encoding.ASCII.GetString(head),
                    rel + " is not a real ogg container");
            }
        }

        // ---------- provenance ----------

        private static RegisterJson.Node Fragment()
        {
            string path = Abs(FragmentPath);
            Assert.IsTrue(File.Exists(path), "audio provenance fragment missing");
            var root = RegisterJson.Parse(File.ReadAllText(path));
            Assert.AreEqual(RegisterJson.Node.Kind.Obj, root.Type);
            Assert.AreEqual(1.0, root.Get("schema_version")!.Num);
            Assert.AreEqual(RegisterJson.Node.Kind.Arr, root.Get("assets")!.Type);
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

        [Test]
        public void TestProvenanceFragmentIntegrity()
        {
            var rows = Rows();
            Assert.AreEqual(AllOggUnder(AudioRoot).Count, rows.Count,
                "row count must equal shipped clip count");
            string? prev = null;
            var seen = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in rows)
            {
                string fp = Str(row, "file_path")!;
                Assert.IsNotNull(fp, "row without file_path");
                if (prev != null)
                {
                    Assert.IsTrue(
                        string.CompareOrdinal(prev, fp) < 0,
                        "rows not sorted by file_path");
                }
                prev = fp;
                Assert.IsTrue(seen.Add(fp), "duplicate file_path " + fp);
                Assert.IsTrue(File.Exists(Abs(fp)),
                    "row file missing: " + fp);
                Assert.AreEqual(Sha256File(Abs(fp)), Str(row, "final_sha256"),
                    "final_sha256 mismatch for " + fp);
                Assert.AreEqual("sha256:" + Str(row, "final_sha256"),
                    Str(row, "content_id"), "content_id mismatch for " + fp);
                Assert.IsTrue(fp.StartsWith("client/Assets/Audio/",
                    StringComparison.Ordinal), "row outside Audio/: " + fp);
                Assert.IsFalse(fp.EndsWith(".meta"), "meta row " + fp);
            }
        }

        [Test]
        public void TestAudioLicenseAndSourceRows()
        {
            foreach (var row in Rows())
            {
                string fp = Str(row, "file_path")!;
                string kind = Str(row, "source_kind")!;
                Assert.AreEqual("FREE_LICENSED", kind,
                    fp + ": audio route is FREE_LICENSED only");
                string lic = Str(row, "license_id")!;
                Assert.Contains(lic, new[] { "CC0-1.0", "CC-BY-4.0" },
                    fp + ": license not in the permitted set");
                var src = Str(row, "source_uri");
                Assert.IsNotNull(src, fp + " missing source_uri");
                StringAssert.StartsWith("https://", src!);
                var sha = Str(row, "source_sha256");
                Assert.IsTrue(
                    System.Text.RegularExpressions.Regex.IsMatch(
                        sha ?? "", "^[0-9a-f]{64}$"),
                    fp + " bad source_sha256");
                if (lic == "CC-BY-4.0")
                {
                    Assert.IsNotNull(Str(row, "attribution"),
                        fp + ": CC-BY-4.0 requires attribution text");
                }
                var changes = row.Get("changes")!;
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, changes.Type,
                    fp + " changes must be an array");
                Assert.Greater(changes.Arr!.Count, 0,
                    fp + " shipped file differs from source; changes must document the edits");
                Assert.AreEqual("PENDING", Str(row, "review_state"),
                    fp + " must enter review as PENDING");
            }
        }

        /// <summary>
        /// ADR-0072 provenance contract: the fragment ships FREE_LICENSED rows
        /// (the audio tool grant is unavailable in-session, manifest §5), so
        /// AI_CREATED must not appear. If an AI_CREATED row is ever added it
        /// must carry the full in-session generation record — same gate shape
        /// as the art fragments.
        /// </summary>
        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
            foreach (var row in Rows())
            {
                if (Str(row, "source_kind") != "AI_CREATED")
                {
                    continue;
                }
                var rec = row.Get("generation_record");
                Assert.IsNotNull(rec,
                    "AI_CREATED audio requires generation_record");
                Assert.AreEqual("Direct AI Generation (In-Session Multimodal)",
                    Str(rec!, "tool"),
                    "generation tool must match owner-setup capability doc");
                Assert.IsNotNull(Str(rec!, "model_id"));
                Assert.IsNotNull(Str(rec!, "terms_snapshot_sha256"));
            }
        }

        /// <summary>
        /// ADR-0076 extended contract for audio: every AI_CREATED row would
        /// need generation_record + terms_snapshot_sha256 resolving under
        /// terms/audio/&lt;sha&gt;.txt. The shipped FREE_LICENSED set instead
        /// requires license-text snapshots to exist under terms/audio/.
        /// </summary>
        [Test]
        public void TestAudioGenerationRecordAndTermsSnapshot()
        {
            string termsDir = Abs(TermsDir);
            Assert.IsTrue(Directory.Exists(termsDir),
                "terms/audio snapshot dir missing");
            var licenses = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                if (Str(row, "source_kind") == "AI_CREATED")
                {
                    var rec = row.Get("generation_record")!;
                    string sha = Str(rec, "terms_snapshot_sha256")!;
                    string p = Path.Combine(termsDir, sha + ".txt");
                    Assert.IsTrue(File.Exists(p),
                        "missing terms snapshot " + sha);
                    Assert.AreEqual(sha, Sha256File(p),
                        "terms snapshot content hash mismatch");
                }
                else
                {
                    licenses.Add(Str(row, "license_id")!);
                }
            }
            Assert.GreaterOrEqual(licenses.Count, 1);
            int snapshots = Directory.GetFiles(termsDir, "*.txt").Length;
            Assert.GreaterOrEqual(snapshots, licenses.Count,
                "every distinct license_id needs a terms/audio snapshot");
            foreach (var f in Directory.GetFiles(termsDir, "*.txt"))
            {
                Assert.AreEqual(Path.GetFileNameWithoutExtension(f),
                    Sha256File(f), "terms file name must equal its sha256");
            }
        }

        /// <summary>
        /// Zone/dungeon/finale BGM rows are tied to cultural entities and must
        /// carry a folklore card (manifest §5 motif audit).
        /// </summary>
        [Test]
        public void TestZoneBgmFolkloreCards()
        {
            var zonePat = new System.Text.RegularExpressions.Regex(
                @"asset\.(bgm|map|dungeon|instance)\.("
                + string.Join("|", AddressableGroups.ZoneKeys)
                + @"|dinh_lang_bo_hoang|mieu_ba_trong_rung|xom_chim|hang_ma_tranh|den_tran|finale)");
            foreach (var row in Rows())
            {
                string fp = Str(row, "file_path")!;
                if (!fp.Contains("/Bgm/"))
                {
                    continue;
                }
                string key = Str(row, "asset_key")!;
                var card = row.Get("folklore_card");
                if (zonePat.IsMatch(key))
                {
                    Assert.IsNotNull(card,
                        key + " is a cultural-space track, folklore_card required");
                    Assert.AreEqual(RegisterJson.Node.Kind.Obj, card!.Type);
                    foreach (var f in new[]
                        { "source_tales", "regional_variants", "motifs_checked" })
                    {
                        var v = card.Get(f);
                        Assert.IsNotNull(v, key + " card missing " + f);
                        Assert.AreEqual(RegisterJson.Node.Kind.Arr, v!.Type);
                        Assert.Greater(v!.Arr!.Count, 0,
                            key + " card " + f + " empty");
                    }
                }
            }
        }
    }
}
