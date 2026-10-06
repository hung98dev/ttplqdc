using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using ReleaseAudit = ThinhThan.Core.Assets.Editor.AssetProduction.ReleaseAssetAudit;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.ReleaseAssetAudit
{
    /// <summary>
    /// IMP-076 coverage: the release audit reproduces the merged register,
    /// rights review and packaged notice deterministically, resolves every
    /// release catalog key through the canonical groups, and reports every
    /// failure mode as a named finding — no suppression shortcuts.
    /// </summary>
    public class ReleaseAssetAuditTests
    {
        private static string RepoRoot()
        {
            var dir = new DirectoryInfo(Application.dataPath);
            while (dir != null && !Directory.Exists(
                Path.Combine(dir.FullName, "docs")))
            {
                dir = dir.Parent;
            }
            Assert.IsNotNull(dir, "repo root with docs/ not found");
            return dir!.FullName;
        }

        private static List<ReleaseAudit.Finding> Domain(
            string repoRoot, string domain)
        {
            var report = ReleaseAudit.Run(repoRoot, runArtGates: false);
            return report.OfDomain(domain);
        }

        // ---------------- deterministic artifacts ----------------

        [Test]
        public void TestRegisterIsDeterministicMergeOfFragments()
        {
            string root = RepoRoot();
            string merged = ReleaseAudit.MergeRegisterText(root);
            string committed = File.ReadAllText(Path.Combine(root,
                ReleaseAudit.RegisterRelativePath.Replace('/',
                    Path.DirectorySeparatorChar)));
            // Byte-stable across clean invocations.
            Assert.AreEqual(merged,
                ReleaseAudit.MergeRegisterText(root),
                "merge must be byte-stable across invocations");
            Assert.AreEqual(merged, committed,
                "committed register differs from deterministic merge");
            var node = RegisterJson.Parse(merged);
            var assets = node.Get("assets");
            Assert.IsNotNull(assets);
            int expected = 0;
            foreach (var f in Directory.GetFiles(Path.Combine(root,
                ReleaseAudit.FragmentsDir.Replace('/',
                    Path.DirectorySeparatorChar)), "*.json"))
            {
                expected += RegisterJson.Parse(File.ReadAllText(f))
                    .Get("assets")!.Arr!.Count;
            }
            Assert.AreEqual(expected, assets!.Arr!.Count,
                "merged register must carry every fragment row");
            // Sorted by file_path ascending, unique.
            var prev = string.Empty;
            var seen = new HashSet<string>();
            foreach (var r in assets.Arr)
            {
                var fp = r.Get("file_path")!.Str;
                Assert.IsTrue(string.CompareOrdinal(prev, fp) < 0,
                    "rows not sorted by file_path");
                Assert.IsTrue(seen.Add(fp), "duplicate file_path " + fp);
                prev = fp;
            }
        }

        [Test]
        public void TestRightsReviewMatchesAudit()
        {
            string root = RepoRoot();
            string p = Path.Combine(root,
                ReleaseAudit.RightsReviewRelativePath.Replace('/',
                    Path.DirectorySeparatorChar));
            Assert.IsTrue(File.Exists(p), "rights review record missing");
            Assert.AreEqual(ReleaseAudit.RightsReviewText(root),
                File.ReadAllText(p));
        }

        [Test]
        public void TestNoticeIsDeterministicAndRegistered()
        {
            string root = RepoRoot();
            string p = Path.Combine(root,
                ReleaseAudit.NoticeRelativePath.Replace('/',
                    Path.DirectorySeparatorChar));
            Assert.IsTrue(File.Exists(p),
                "THIRD_PARTY_ASSETS.txt missing");
            Assert.AreEqual(ReleaseAudit.NoticeText(root),
                File.ReadAllText(p));
            var groups = ReleaseAudit.LoadGroupEntries(root);
            Assert.IsTrue(groups[AddressableGroups.SharedLocal]
                .ContainsKey(ReleaseAudit.CreditsTextKey),
                "asset.ui.credits.text not registered in shared.local");
        }

        // ---------------- release-scope walk ----------------

        [Test]
        public void TestReleaseScopeResolvesAllReleaseKeys()
        {
            var fails = Domain(RepoRoot(), "coverage");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString()).GetRange(0,
                    System.Math.Min(fails.Count, 50))));
        }

        [Test]
        public void TestEveryRowApprovedWithLaunchRights()
        {
            var fails = Domain(RepoRoot(), "rights");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString()).GetRange(0,
                    System.Math.Min(fails.Count, 50))));
        }

        [Test]
        public void TestReleaseScopeArtGatesAndReviewRecords()
        {
            var report = ReleaseAudit.Run(RepoRoot());
            Assert.IsEmpty(report.Findings,
                "release audit findings:\n" + report.Summary());
        }

        [Test]
        public void TestBudgetsRespected()
        {
            var fails = Domain(RepoRoot(), "budget");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString())));
        }

        [Test]
        public void TestGlyphCoverageOnFonts()
        {
            var fails = Domain(RepoRoot(), "glyph");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString())));
        }

        [Test]
        public void TestNoPlaceholdersInReleaseSet()
        {
            var fails = Domain(RepoRoot(), "placeholder");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString())));
        }

        [Test]
        public void TestProvenanceValidatorClean()
        {
            var fails = Domain(RepoRoot(), "provenance");
            Assert.IsEmpty(fails, string.Join("\n",
                fails.ConvertAll(f => f.ToString()).GetRange(0,
                    System.Math.Min(fails.Count, 50))));
        }

        // ---------------- negatives (synthetic repo) ----------------

        private static string NewRoot()
        {
            string root = Path.Combine(Path.GetTempPath(),
                "imp076_" + Path.GetRandomFileName());
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/Provenance/fragments"));
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/Provenance/terms/testfrag"));
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/StyleRef/testfrag/testpack"));
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/StyleRef/testfrag/testpack/style.md"),
                "# test pack\n");
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/StyleRef/testfrag/testpack/palette.json"),
                "{\"pack_id\":\"testfrag/testpack\",\"families\":{}}\n");
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/Provenance/cultural_review.md"),
                "| cosmetic.id | CATEGORY | assessment | verdict |\n");
            return root;
        }

        private static string Sha256(string path)
        {
            using (var sha = SHA256.Create())
            using (var fs = File.OpenRead(path))
            {
                var b = sha.ComputeHash(fs);
                var sb = new StringBuilder(64);
                foreach (var x in b)
                {
                    sb.Append(x.ToString("x2", CultureInfo.InvariantCulture));
                }
                return sb.ToString();
            }
        }

        private static string RowJson(string rel, string filePath,
            string sha, string license, string review, string genJson)
        {
            return "{\n"
                + "\"asset_key\":\"asset.test.icon.sprite\",\n"
                + "\"file_path\":\"" + filePath + "\",\n"
                + "\"content_id\":\"sha256:" + sha + "\",\n"
                + "\"source_kind\":\"" + rel + "\",\n"
                + "\"creator\":\"t\",\"source_uri\":\"https://x\",\n"
                + "\"license_id\":\"" + license + "\","
                + "\"license_uri\":\"https://x\",\n"
                + "\"acquired_at_utc\":\"2026-01-01T00:00:00Z\",\n"
                + "\"source_sha256\":\"" + sha + "\",\n"
                + "\"final_sha256\":\"" + sha + "\",\n"
                + "\"changes\":\"\",\"attribution\":\"attr\",\n"
                + "\"style_pack_id\":\"testfrag/testpack\",\n"
                + "\"generation_record\":"
                + (genJson.Length == 0 ? "null" : genJson)
                + ",\"folklore_card\":null,\n"
                + "\"inputs\":[],\"review_state\":\"" + review + "\""
                + "\n}";
        }

        private static void WriteFragment(string root, string name,
            string assetsJson)
        {
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/Provenance/fragments/" + name + ".json"),
                "{\"schema_version\":1,\"assets\":[" + assetsJson + "]}\n");
        }

        private static void WriteRegister(string root, string assetsJson)
        {
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/Provenance/asset_source_register.json"),
                "{\"schema_version\":1,\"assets\":[" + assetsJson + "]}\n");
        }

        private static List<ReleaseAudit.Finding> WithSubject(
            ReleaseAudit.Report report, string subject)
        {
            var list = new List<ReleaseAudit.Finding>();
            foreach (var f in report.Findings)
            {
                if (f.Subject == subject)
                {
                    list.Add(f);
                }
            }
            return list;
        }

        private static List<ReleaseAudit.Finding> WithSubjectPrefix(
            ReleaseAudit.Report report, string prefix)
        {
            var list = new List<ReleaseAudit.Finding>();
            foreach (var f in report.Findings)
            {
                if (f.Subject.StartsWith(prefix,
                    System.StringComparison.Ordinal))
                {
                    list.Add(f);
                }
            }
            return list;
        }

        private static bool HasDetail(
            List<ReleaseAudit.Finding> findings, string needle)
        {
            foreach (var f in findings)
            {
                if (f.Detail.IndexOf(needle,
                    System.StringComparison.Ordinal) >= 0)
                {
                    return true;
                }
            }
            return false;
        }

        private static string ReadReviewState(string mergedText,
            string filePath)
        {
            var node = RegisterJson.Parse(mergedText);
            foreach (var r in node.Get("assets")!.Arr!)
            {
                if (r.Get("file_path")!.Str == filePath)
                {
                    return r.Get("review_state")!.Str;
                }
            }
            return string.Empty;
        }

        [Test]
        public void TestNegative_RowWithBadFinalShaReported()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/x.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string bad = new string('0', 64);
            WriteRegister(root, RowJson("FREE_LICENSED", rel, bad,
                "CC0-1.0", "APPROVED", ""));
            var report = ReleaseAudit.AuditRows(root);
            var hits = WithSubject(report, rel);
            Assert.IsTrue(HasDetail(hits, "does not match"),
                "bad final_sha256 not reported: " + report.Summary());
        }

        [Test]
        public void TestNegative_RowWithMissingFileReported()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/missing.png";
            WriteRegister(root, RowJson("FREE_LICENSED", rel,
                new string('a', 64), "CC0-1.0", "APPROVED", ""));
            var report = ReleaseAudit.AuditRows(root);
            var hits = WithSubject(report, rel);
            Assert.IsTrue(HasDetail(hits, "file missing"),
                "missing file not reported: " + report.Summary());
        }

        [Test]
        public void TestNegative_BannedLicenseStaysPending()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/y.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            WriteFragment(root, "testfrag", RowJson("FREE_LICENSED", rel,
                sha, "CC-BY-NC-4.0", "PENDING", ""));
            string state = ReadReviewState(
                ReleaseAudit.MergeRegisterText(root), rel);
            Assert.AreEqual("PENDING", state,
                "NC-licensed row must not be approved");
        }

        [Test]
        public void TestNegative_MissingFolkloreCardStaysPending()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/Actors/Creatures/z/m/a.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/Actors/Creatures/z/m"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            // AI_CREATED needs a complete generation_record + terms file;
            // missing folklore_card must independently block approval.
            string gen = "{\"tool\":\"t\","
                + "\"version\":\"v\",\"model_id\":\"m\",\"model_sha256\":null,"
                + "\"terms_uri\":\"https://x\",\"terms_snapshot_sha256\":\""
                + sha + "\",\"prompt\":\"p\",\"seed\":null,"
                + "\"parameters\":{},\"workflow_sha256\":null,"
                + "\"reference_uris\":[],\"reference_sha256\":[],"
                + "\"c2pa_present\":false}";
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Art/Provenance/terms/testfrag/" + sha + ".txt"),
                "px");
            WriteFragment(root, "testfrag", RowJson("AI_CREATED", rel, sha,
                "AI_TOOL_TERMS", "PENDING", gen));
            string state = ReadReviewState(
                ReleaseAudit.MergeRegisterText(root), rel);
            Assert.AreEqual("PENDING", state,
                "cultural entity without folklore_card must not be approved");
        }

        [Test]
        public void TestNegative_UnresolvableStylePackStaysPending()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/z.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            string row = RowJson("FREE_LICENSED", rel, sha,
                "CC0-1.0", "PENDING", "")
                .Replace("testfrag/testpack", "testfrag/no_such_pack");
            WriteFragment(root, "testfrag", row);
            string state = ReadReviewState(
                ReleaseAudit.MergeRegisterText(root), rel);
            Assert.AreEqual("PENDING", state,
                "image row with unresolvable style pack must stay pending");
        }

        [Test]
        public void TestNegative_ApprovedRowWithoutRightsFlagged()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/w.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            // APPROVED row with an unapproved license is a finding.
            WriteRegister(root, RowJson("FREE_LICENSED", rel,
                new string('b', 64), "CC-BY-NC-4.0", "APPROVED", ""));
            var report = ReleaseAudit.AuditRows(root);
            var hits = WithSubject(report, rel);
            Assert.IsTrue(HasDetail(hits, "approved despite rights"),
                "APPROVED row failing rights not flagged: "
                + report.Summary());
        }

        [Test]
        public void TestNegative_NoticeDriftReported()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/n.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Notices"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            WriteFragment(root, "testfrag", RowJson("FREE_LICENSED", rel,
                sha, "CC0-1.0", "PENDING", ""));
            File.WriteAllText(Path.Combine(root,
                "client/Assets/Notices/THIRD_PARTY_ASSETS.txt"),
                "tampered notice\n");
            var report = ReleaseAudit.AuditNotice(root);
            var hits = WithSubject(report,
                ReleaseAudit.NoticeRelativePath);
            Assert.IsTrue(HasDetail(hits, "differs"),
                "notice drift not reported: " + report.Summary());
        }

        [Test]
        public void TestNegative_PlaceholderDetected()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/flat.png";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI"));
            File.WriteAllText(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)), "px");
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            // Placeholder is a declared state: a row labelled placeholder
            // must be flagged regardless of pixel content.
            string row = RowJson("FREE_LICENSED", rel, sha,
                "CC0-1.0", "APPROVED", "");
            row = row.Insert(row.IndexOf('\n') + 1,
                "\"placeholder\":true,\n");
            WriteRegister(root, row);
            var report = ReleaseAudit.AuditMedia(root);
            var hits = WithSubject(report, rel);
            Assert.IsTrue(HasDetail(hits, "declared placeholder"),
                "labelled placeholder not detected: " + report.Summary());
        }

        [Test]
        public void TestGateScopeFollowsDeclaredClassAndProfile()
        {
            string root = NewRoot();
            var png = new LabPixels.Image(192, 256);
            var solid = new LabPixels.Rgba
            {
                R = 80, G = 40, B = 20, A = 255,
            };
            // Silhouette ~60px tall, feet at the bottom: a clear band miss if
            // the 176..192 humanoid band were misapplied.
            for (int y = 190; y < 250; y++)
            {
                for (int x = 70; x < 120; x++)
                {
                    png.Set(x, y, solid);
                }
            }
            byte[] bytes = ArtRuleFixtures.EncodePng(png);

            // Creature with a non-humanoid profile: only its own bbox limit
            // binds, never the humanoid band.
            string cdir = "client/Assets/Art/Actors/Creatures/z/m";
            Directory.CreateDirectory(Path.Combine(root,
                cdir.Replace('/', Path.DirectorySeparatorChar)));
            File.WriteAllText(Path.Combine(root,
                (cdir + "/actor_import.json").Replace('/',
                    Path.DirectorySeparatorChar)),
                "{\"asset_class\":\"ACTOR\",\"profile\":\"MONSTER_SMALL\","
                    + "\"technique\":\"skeletal\",\"cell_ref\":[96,128],"
                    + "\"texture_ref\":[192,256],\"assets\":{}}");
            string creature = cdir + "/m.png";
            File.WriteAllBytes(Path.Combine(root,
                creature.Replace('/', Path.DirectorySeparatorChar)), bytes);

            // UI_ART flat fill: volume/silhouette-exempt class, no pixel
            // heuristic may mark it placeholder.
            string udir = "client/Assets/Art/UI";
            Directory.CreateDirectory(Path.Combine(root, udir));
            File.WriteAllText(Path.Combine(root,
                udir.Replace('/', Path.DirectorySeparatorChar)
                    + "/import.json"),
                "{\"asset_class\":\"UI_ART\",\"assets\":{}}");
            string fill = udir + "/fill.png";
            File.WriteAllBytes(Path.Combine(root,
                fill.Replace('/', Path.DirectorySeparatorChar)), bytes);

            // *_parts.png composite: never band, never placeholder.
            string parts = cdir + "/m_parts.png";
            File.WriteAllBytes(Path.Combine(root,
                parts.Replace('/', Path.DirectorySeparatorChar)), bytes);

            // PARALLAX_FAR under World: exempt-class flat render.
            string wdir = "client/Assets/Art/World/z/parallax";
            Directory.CreateDirectory(Path.Combine(root,
                wdir.Replace('/', Path.DirectorySeparatorChar)));
            string far = wdir + "/bg.far.png";
            File.WriteAllBytes(Path.Combine(root,
                far.Replace('/', Path.DirectorySeparatorChar)), bytes);

            // Humanoid figure surface positive control: player sheet with
            // the same 60px silhouette must still flag the band.
            string pdir = "client/Assets/Art/Actors/Players/x";
            Directory.CreateDirectory(Path.Combine(root,
                pdir.Replace('/', Path.DirectorySeparatorChar)));
            string sheet = pdir + "/x_sheet.png";
            File.WriteAllBytes(Path.Combine(root,
                sheet.Replace('/', Path.DirectorySeparatorChar)), bytes);

            var rows = new StringBuilder();
            foreach (var rel in new[] { creature, fill, parts, far, sheet })
            {
                string sha = Sha256(Path.Combine(root,
                    rel.Replace('/', Path.DirectorySeparatorChar)));
                if (rows.Length != 0)
                {
                    rows.Append(',');
                }
                rows.Append(RowJson("FREE_LICENSED", rel, sha,
                    "CC0-1.0", "APPROVED", ""));
            }
            WriteRegister(root, rows.ToString());

            var report = ReleaseAudit.Run(root, runArtGates: true);
            foreach (var rel in new[] { creature, parts })
            {
                var hits = WithSubjectPrefix(report, rel);
                Assert.IsFalse(HasDetail(hits, "body height"),
                    "non-humanoid file hit by body band: " + rel);
                Assert.IsFalse(HasDetail(hits, "body band"),
                    "non-humanoid file hit by body band: " + rel);
            }
            var placeholder = report.OfDomain("placeholder");
            Assert.IsEmpty(placeholder,
                "placeholder finding without declared label: "
                    + report.Summary());
            var sheetHits = WithSubjectPrefix(report, sheet);
            Assert.IsTrue(HasDetail(sheetHits, "body height")
                || HasDetail(sheetHits, "body band"),
                "humanoid figure surface lost its band check: " + sheet);
        }

        [Test]
        public void TestNegative_UnreadableFontReported()
        {
            string root = NewRoot();
            string rel = "client/Assets/Art/UI/Fonts/bad.ttf";
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/Art/UI/Fonts"));
            File.WriteAllBytes(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)),
                new byte[32]);
            string sha = Sha256(Path.Combine(root,
                rel.Replace('/', Path.DirectorySeparatorChar)));
            WriteRegister(root, RowJson("FREE_LICENSED", rel, sha,
                "OFL-1.1", "APPROVED", ""));
            var report = ReleaseAudit.AuditMedia(root);
            var hits = WithSubject(report, rel);
            Assert.IsTrue(HasDetail(hits, "no readable cmap"),
                "unreadable font not reported: " + report.Summary());
        }

        [Test]
        public void TestNegative_UnregisteredGroupKeyReported()
        {
            string root = NewRoot();
            // A group file holding a key that does not parse must surface.
            Directory.CreateDirectory(Path.Combine(root,
                "client/Assets/AddressableAssetsData/AssetGroups"));
            foreach (var name in AddressableGroups.CanonicalNames())
            {
                string content = "m_Entries:\n";
                if (name == AddressableGroups.SharedLocal)
                {
                    content += "  - m_GUID: 11111111111111111111111111111111\n"
                        + "    m_Address: not.an.asset.key\n"
                        + "    m_ReadOnly: 0\n"
                        + "    m_SerializedLabels: []\n"
                        + "    FlaggedDuringContentUpdateRestriction: 0\n";
                }
                File.WriteAllText(Path.Combine(root,
                    "client/Assets/AddressableAssetsData/AssetGroups/"
                    + name + ".asset"), content);
            }
            var report = ReleaseAudit.AuditGroups(root);
            var hits = WithSubject(report, "shared.local not.an.asset.key");
            Assert.IsTrue(HasDetail(hits, "not a valid asset.* key"),
                "invalid group address not reported: " + report.Summary());
        }
    }
}
