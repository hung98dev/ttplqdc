using System.Globalization;
using System.IO;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AssetProvenance
{
    /// <summary>
    /// Provenance register validator (manifest §6): foundation mode on the
    /// clean empty register, row-level field/license/hash/approval checks
    /// with path-specific errors, the ART-012 extended generation record,
    /// upscale recording, and the ART-010 folklore_card requirement.
    /// </summary>
    public class AssetProvenanceTests
    {
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

        private static string RegisterPath()
        {
            return Path.Combine(RepoRoot(), ProvenanceValidator.RegisterRelativePath);
        }

        /// <summary>
        /// Unique synthetic repo root per call so release-mode media files
        /// never leak between tests.
        /// </summary>
        private static string NewRoot()
        {
            var dir = Path.Combine(RepoRoot(), "artifacts", "provenance-tests",
                System.Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(dir);
            return dir;
        }

        private static void WriteRegister(string root, string json)
        {
            var provenance = Path.Combine(root, "client", "Assets", "Art", "Provenance");
            Directory.CreateDirectory(provenance);
            File.WriteAllText(Path.Combine(provenance,
                "asset_source_register.json"), json);
        }

        private static string WriteTempRegister(string json)
        {
            var dir = NewRoot();
            WriteRegister(dir, json);
            return dir;
        }

        private static string MinimalRow(
            string filePath, string sourceKind, string licenseId, string reviewState,
            string attribution, string stylePack, string genRecord, string folkloreCard,
            string sourceUri)
        {
            return "{"
                + "\"asset_key\":\"asset.ui.test.sprite\","
                + "\"file_path\":\"" + filePath + "\","
                + "\"content_id\":null,"
                + "\"source_kind\":\"" + sourceKind + "\","
                + "\"creator\":\"fixture\","
                + "\"source_uri\":" + sourceUri + ","
                + "\"license_id\":\"" + licenseId + "\","
                + "\"license_uri\":\"https://example.com/lic\","
                + "\"acquired_at_utc\":\"2026-10-01T00:00:00Z\","
                + "\"source_sha256\":\"" + new string('a', 64) + "\","
                + "\"final_sha256\":\"" + new string('b', 64) + "\","
                + "\"changes\":\"none\","
                + "\"attribution\":" + attribution + ","
                + "\"style_pack_id\":" + stylePack + ","
                + "\"generation_record\":" + genRecord + ","
                + "\"folklore_card\":" + folkloreCard + ","
                + "\"inputs\":[],"
                + "\"review_state\":\"" + reviewState + "\""
                + "}";
        }

        private static string Doc(string rowsJson)
        {
            return "{\"schema_version\":1,\"assets\":[" + rowsJson + "]}";
        }

        [Test]
        public void TestExtendedGenerationRecordAndTermsSnapshot()
        {
            var report = ProvenanceValidator.ValidateFoundation(RepoRoot());
            Assert.IsTrue(report.Passed,
                "clean empty register must pass foundation: "
                + string.Join("; ", report.Errors.ConvertAll(e => e.ToString())));

            var gen = "{"
                + "\"tool\":\"gen-image\",\"version\":\"1.0\",\"model_id\":\"m1\","
                + "\"model_sha256\":\"" + new string('c', 64) + "\","
                + "\"terms_uri\":\"https://tool/terms\","
                + "\"terms_snapshot_sha256\":\"" + new string('d', 64) + "\","
                + "\"prompt\":\"p\",\"seed\":42,\"parameters\":null,"
                + "\"workflow_sha256\":\"" + new string('e', 64) + "\","
                + "\"reference_uris\":[],\"reference_sha256\":[],\"c2pa_present\":false}";
            var row = MinimalRow("client/Assets/Art/x.png", "AI_CREATED", "AI_TOOL_TERMS",
                "PENDING", "null", "\"actors_players/pack\"", gen, "null", "null");
            var doc = Doc(row);
            var tmp = WriteTempRegister(doc);
            var regPath = Path.Combine(tmp, "client", "Assets", "Art",
                "Provenance", "asset_source_register.json");
            Assert.IsTrue(File.Exists(regPath));
            var reg = RegisterJson.Parse(File.ReadAllText(regPath));
            var rowNode = reg.Get("assets")!.Arr![0];
            var gr = rowNode.Get("generation_record")!;
            Assert.AreEqual(RegisterJson.Node.Kind.Obj, gr.Type,
                "generation_record must parse as object");
            Assert.IsNotNull(gr.Get("terms_snapshot_sha256"));
            Assert.IsNotNull(gr.Get("model_sha256"));
            Assert.IsNotNull(gr.Get("workflow_sha256"));

            var rep2 = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep2.Passed,
                "row must fail when the terms snapshot is absent");
            Assert.IsTrue(rep2.Errors.Exists(e => e.Field.Contains("terms_snapshot_sha256")),
                "validator must resolve terms_snapshot_sha256 to terms/<fragment>/<sha>.txt");
        }

        [Test]
        public void TestUpscaleRecordedInChanges()
        {
            var rowNoUpscale = MinimalRow("client/Assets/Art/a.png", "AI_CREATED",
                "AI_TOOL_TERMS", "PENDING", "null", "null", "null", "null", "null");
            var rowUpscale = MinimalRow("client/Assets/Art/b.png", "AI_CREATED",
                "AI_TOOL_TERMS", "PENDING", "null", "null", "null", "null", "null")
                .Replace("\"changes\":\"none\"", "\"changes\":\"upscale 4x then shrink to 2x\"");
            var doc = Doc(rowNoUpscale + "," + rowUpscale);
            var reg = RegisterJson.Parse(doc);
            var rows = reg.Get("assets")!.Arr!;
            Assert.AreEqual("none", rows[0].Get("changes")!.Str);
            Assert.IsTrue(rows[1].Get("changes")!.Str.Contains("upscale"),
                "upscale step must be recorded in changes (ART-012)");
            Assert.Less(rows[0].Get("file_path")!.Str.CompareTo(rows[1].Get("file_path")!.Str), 0,
                "assets must sort ascending by file_path");
        }

        [Test]
        public void TestFolkloreCardRequiredForCulturalEntities()
        {
            var card = "{"
                + "\"source_tales\":[\"tale A\"],"
                + "\"regional_variants\":[\"north\"],"
                + "\"motifs_checked\":[\"torii_gate\"]}";
            var rowCultural = MinimalRow("client/Assets/Art/e.png", "FREE_LICENSED",
                "CC0-1.0", "APPROVED", "null", "\"actors_creatures/pack\"",
                "null", card, "\"https://author/p\"");
            var doc = Doc(rowCultural);
            var reg = RegisterJson.Parse(doc);
            var row = reg.Get("assets")!.Arr![0];
            var fc = row.Get("folklore_card")!;
            Assert.AreEqual(RegisterJson.Node.Kind.Obj, fc.Type);
            Assert.IsNotNull(fc.Get("source_tales"));
            Assert.IsNotNull(fc.Get("motifs_checked"));
            Assert.Greater(fc.Get("motifs_checked")!.Arr!.Count, 0,
                "motifs_checked must be nonempty for a cultural entity (ART-010)");

            var tmp = WriteTempRegister(doc);
            var foundationReport = ValidateOnCopy(tmp);
            Assert.IsTrue(foundationReport.Passed,
                "valid cultural row must pass: "
                + string.Join("; ", foundationReport.Errors.ConvertAll(e => e.ToString())));

            var noCardDoc = doc.Replace(",\"folklore_card\":" + card, string.Empty);
            var tmp2 = WriteTempRegister(noCardDoc);
            var noCardReport = ValidateOnCopy(tmp2);
            Assert.IsFalse(noCardReport.Passed,
                "row without the folklore_card key must fail (§6 required field)");
            Assert.IsTrue(noCardReport.Errors.Exists(e => e.Field.Contains("folklore_card")),
                "missing folklore_card must be reported on its own path");
        }

        private static ProvenanceValidator.Report ValidateOnCopy(string dir)
        {
            return ProvenanceValidator.ValidateFoundation(dir);
        }

        private static string Sha256Text(string s)
        {
            using (var sha = System.Security.Cryptography.SHA256.Create())
            {
                var bytes = sha.ComputeHash(Encoding.UTF8.GetBytes(s));
                var sb = new StringBuilder(64);
                foreach (var b in bytes)
                {
                    sb.Append(b.ToString("x2", CultureInfo.InvariantCulture));
                }
                return sb.ToString();
            }
        }

        /// <summary>
        /// Writes a terms snapshot under
        /// tmp/client/Assets/Art/Provenance/terms/&lt;fragment&gt;/&lt;sha&gt;.txt
        /// and returns its sha256.
        /// </summary>
        private static string WriteTermsFile(string tmp, string fragment, string content)
        {
            string sha = Sha256Text(content);
            var dir = Path.Combine(tmp, "client", "Assets", "Art",
                "Provenance", "terms", fragment);
            Directory.CreateDirectory(dir);
            File.WriteAllText(Path.Combine(dir, sha + ".txt"), content);
            return sha;
        }

        /// <summary>Writes a media file under tmp/client/... and returns its sha256.</summary>
        private static string WriteMediaFile(string tmp, string relPath, byte[] bytes)
        {
            var path = Path.Combine(tmp, relPath.Replace('/', Path.DirectorySeparatorChar));
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllBytes(path, bytes);
            return ProvenanceValidator.Sha256File(path);
        }

        private static string ValidGen(string termsSha)
        {
            return "{"
                + "\"tool\":\"gen-image\",\"version\":\"1.0\",\"model_id\":\"m1\","
                + "\"model_sha256\":\"" + new string('c', 64) + "\","
                + "\"terms_uri\":\"https://tool/terms\","
                + "\"terms_snapshot_sha256\":\"" + termsSha + "\","
                + "\"prompt\":\"p\",\"seed\":42,\"parameters\":null,"
                + "\"workflow_sha256\":\"" + new string('e', 64) + "\","
                + "\"reference_uris\":[],\"reference_sha256\":[],\"c2pa_present\":false}";
        }

        /// <summary>
        /// A row with a caller-supplied asset_key and file_path; all other
        /// fields valid so a single targeted check can be observed.
        /// </summary>
        private static string Row(
            string key, string filePath, string sourceKind, string licenseId,
            string reviewState, string attribution, string stylePack,
            string genRecord, string folkloreCard, string sourceUri,
            string inputs, string finalSha)
        {
            return "{"
                + "\"asset_key\":\"" + key + "\","
                + "\"file_path\":\"" + filePath + "\","
                + "\"content_id\":null,"
                + "\"source_kind\":\"" + sourceKind + "\","
                + "\"creator\":\"fixture\","
                + "\"source_uri\":" + sourceUri + ","
                + "\"license_id\":\"" + licenseId + "\","
                + "\"license_uri\":\"https://example.com/lic\","
                + "\"acquired_at_utc\":\"2026-10-01T00:00:00Z\","
                + "\"source_sha256\":\"" + new string('a', 64) + "\","
                + "\"final_sha256\":\"" + finalSha + "\","
                + "\"changes\":\"none\","
                + "\"attribution\":" + attribution + ","
                + "\"style_pack_id\":" + stylePack + ","
                + "\"generation_record\":" + genRecord + ","
                + "\"folklore_card\":" + folkloreCard + ","
                + "\"inputs\":" + inputs + ","
                + "\"review_state\":\"" + reviewState + "\""
                + "}";
        }

        [Test]
        public void TestValidLicenseKindsPass()
        {
            var tmp = NewRoot();
            var termsSha = WriteTermsFile(tmp, "actors_players", "tool terms text");
            var shaB = new string('b', 64);
            var rows = new string[]
            {
                Row("asset.actor.test_ai.prefab", "client/Assets/Art/actors/a.png",
                    "AI_CREATED", "AI_TOOL_TERMS", "PENDING", "null",
                    "\"actors_players/pack\"", ValidGen(termsSha), "null",
                    "null", "[]", shaB),
                Row("asset.ui.test_cc0.icon", "client/Assets/Art/b.png",
                    "FREE_LICENSED", "CC0-1.0", "APPROVED", "null",
                    "\"interface/pack\"", "null", "null",
                    "\"https://author/cc0\"", "[]", shaB),
                Row("asset.ui.test_ccby.icon", "client/Assets/Art/c.png",
                    "FREE_LICENSED", "CC-BY-4.0", "APPROVED",
                    "\"Sprite by Alice (CC-BY-4.0)\"", "\"interface/pack\"",
                    "null", "null", "\"https://author/ccby\"", "[]", shaB),
                Row("asset.ui.test_pending.icon", "client/Assets/Art/d.png",
                    "FREE_LICENSED", "CC-BY-4.0", "PENDING",
                    "\"Hidden until approved\"", "\"interface/pack\"",
                    "null", "null", "\"https://author/pending\"", "[]", shaB),
                Row("asset.font.test_ofl.font", "client/Assets/Art/fonts/f.ttf",
                    "FREE_LICENSED", "OFL-1.1", "APPROVED",
                    "\"Body font by Bob (OFL-1.1)\"", "null",
                    "null", "null", "\"https://author/ofl\"", "[]", shaB),
            };
            WriteRegister(tmp, Doc(string.Join(",", rows)));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsTrue(rep.Passed,
                "valid AI/CC0/CC-BY/OFL rows must pass foundation: "
                + string.Join("; ", rep.Errors.ConvertAll(e => e.ToString())));

            var reg = RegisterJson.Parse(File.ReadAllText(
                Path.Combine(tmp, ProvenanceValidator.RegisterRelativePath)));
            string credits = ProvenanceValidator.CreditsText(reg);
            Assert.IsTrue(credits.Contains("Sprite by Alice"),
                "approved CC-BY attribution must reach credits.txt");
            Assert.IsFalse(credits.Contains("Hidden until approved"),
                "PENDING rows must not emit credit lines");
            string notice = ProvenanceValidator.FontNotice(reg);
            Assert.IsTrue(notice.Contains("Body font by Bob"),
                "approved OFL attribution must reach the font notice");
            Assert.IsFalse(notice.Contains("Sprite by Alice"),
                "CC-BY image rows must not leak into the font notice");
        }

        [Test]
        public void TestMissingRowFailsRelease()
        {
            var tmp = NewRoot();
            var media = "client/Assets/Art/prov/covered.png";
            var sha = WriteMediaFile(tmp, media, new byte[] { 1, 2, 3, 4 });
            var row = Row("asset.prop.test_covered.prefab", media,
                "FREE_LICENSED", "CC0-1.0", "APPROVED", "null",
                "\"world/pack\"", "null", "null",
                "\"https://author/c\"", "[]", sha);
            WriteRegister(tmp, Doc(row));
            var rep = ProvenanceValidator.ValidateRelease(tmp);
            Assert.IsTrue(rep.Passed,
                "covered APPROVED media must pass release: "
                + string.Join("; ", rep.Errors.ConvertAll(e => e.ToString())));
            Assert.AreEqual(1, rep.CoveredFiles);

            WriteMediaFile(tmp, "client/Assets/Art/prov/orphan.png",
                new byte[] { 9, 9 });
            var rep2 = ProvenanceValidator.ValidateRelease(tmp);
            Assert.IsFalse(rep2.Passed,
                "media file without a register row must fail release");
            Assert.IsTrue(rep2.Errors.Exists(
                e => e.Path.Contains("orphan.png") && e.Field == "file_path"),
                "the missing row must be reported on the file's own path");
        }

        [Test]
        public void TestDuplicatePathAndInvalidKeyFail()
        {
            var shaB = new string('b', 64);
            var a = Row("asset.prop.dup_a.prefab", "client/Assets/Art/dup/a.png",
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/a\"", "[]", shaB);
            var b = Row("asset.prop.dup_b.prefab", "client/Assets/Art/dup/a.png",
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/b\"", "[]", shaB);
            var tmp = WriteTempRegister(Doc(a + "," + b));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed);
            Assert.IsTrue(rep.Errors.Exists(
                e => e.Field == "file_path" && e.Message.Contains("duplicate")),
                "duplicate file_path must be a path-specific error");

            var badKey = Row("bogus-key", "client/Assets/Art/k.png",
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/k\"", "[]", shaB);
            var tmp2 = WriteTempRegister(Doc(badKey));
            var rep2 = ProvenanceValidator.ValidateFoundation(tmp2);
            Assert.IsFalse(rep2.Passed);
            Assert.IsTrue(rep2.Errors.Exists(e => e.Field == "asset_key"),
                "a key outside the asset.<kind>.<name>.<facet> grammar must fail");

            var badPath = Row("asset.prop.badpath.prefab",
                "client\\\\Assets\\\\Art\\\\x.png",
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/p\"", "[]", shaB);
            var tmp3 = WriteTempRegister(Doc(badPath));
            var rep3 = ProvenanceValidator.ValidateFoundation(tmp3);
            Assert.IsFalse(rep3.Passed);
            Assert.IsTrue(rep3.Errors.Exists(
                e => e.Field == "file_path" && e.Message.Contains("normalized")),
                "a non-normalized file_path must fail");
        }

        [Test]
        public void TestChangedFileHashFails()
        {
            var tmp = NewRoot();
            var media = "client/Assets/Art/prov/hashed.png";
            var sha = WriteMediaFile(tmp, media, new byte[] { 7, 7, 7 });
            var wrong = Row("asset.prop.test_hash.prefab", media,
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/h\"", "[]",
                new string('b', 64));
            WriteRegister(tmp, Doc(wrong));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed,
                "final_sha256 must match the file on disk");
            Assert.IsTrue(rep.Errors.Exists(
                e => e.Field == "final_sha256" && e.Message.Contains("mismatch")),
                "hash drift must be a path-specific error");

            var right = Row("asset.prop.test_hash.prefab", media,
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/h\"", "[]", sha);
            WriteRegister(tmp, Doc(right));
            var rep2 = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsTrue(rep2.Passed,
                "the correct on-disk hash must pass: "
                + string.Join("; ", rep2.Errors.ConvertAll(e => e.ToString())));
        }

        [Test]
        public void TestDisallowedLicenseRejected()
        {
            var row = Row("asset.ui.test_nc.icon", "client/Assets/Art/nc.png",
                "FREE_LICENSED", "CC-BY-NC-4.0", "PENDING", "null",
                "\"interface/pack\"", "null", "null", "\"https://x/nc\"",
                "[]", new string('b', 64));
            var tmp = WriteTempRegister(Doc(row));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed);
            Assert.IsTrue(rep.Errors.Exists(
                e => e.Field == "license_id" && e.Message.Contains("unknown")),
                "non-allowlisted licenses (e.g. NC) must be rejected");
        }

        [Test]
        public void TestPendingRejectedBlockedInRelease()
        {
            var tmp = NewRoot();
            var media = "client/Assets/Art/prov/gated.png";
            var sha = WriteMediaFile(tmp, media, new byte[] { 5, 5 });
            var pending = Row("asset.prop.test_gate.prefab", media,
                "FREE_LICENSED", "CC0-1.0", "PENDING", "null",
                "\"world/pack\"", "null", "null", "\"https://x/g\"", "[]", sha);
            WriteRegister(tmp, Doc(pending));
            var rep = ProvenanceValidator.ValidateRelease(tmp);
            Assert.IsFalse(rep.Passed,
                "a PENDING row must not satisfy release coverage");
            Assert.IsTrue(rep.Errors.Exists(e => e.Field == "review_state"));

            var rejected = pending.Replace("\"review_state\":\"PENDING\"",
                "\"review_state\":\"REJECTED\"");
            WriteRegister(tmp, Doc(rejected));
            var rep2 = ProvenanceValidator.ValidateRelease(tmp);
            Assert.IsFalse(rep2.Passed,
                "a REJECTED row must not satisfy release coverage");

            var approved = pending.Replace("\"review_state\":\"PENDING\"",
                "\"review_state\":\"APPROVED\"");
            WriteRegister(tmp, Doc(approved));
            var rep3 = ProvenanceValidator.ValidateRelease(tmp);
            Assert.IsTrue(rep3.Passed,
                "APPROVED row + matching hash must pass release: "
                + string.Join("; ", rep3.Errors.ConvertAll(e => e.ToString())));
        }

        [Test]
        public void TestAttributionRequiredForCcBy()
        {
            var row = Row("asset.ui.test_noattr.icon", "client/Assets/Art/at.png",
                "FREE_LICENSED", "CC-BY-4.0", "PENDING", "null",
                "\"interface/pack\"", "null", "null", "\"https://x/at\"",
                "[]", new string('b', 64));
            var tmp = WriteTempRegister(Doc(row));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed);
            Assert.IsTrue(rep.Errors.Exists(e => e.Field == "attribution"),
                "CC-BY-4.0 without an attribution line must fail");
        }

        [Test]
        public void TestGeneratedInputsNotHidden()
        {
            var tmp = NewRoot();
            var termsSha = WriteTermsFile(tmp, "actors_players", "tool terms text");
            var incompleteInput = "[{\"creator\":\"x\","
                + "\"source_uri\":\"https://src/i\",\"license_id\":\"CC0-1.0\","
                + "\"license_uri\":\"https://src/lic\","
                + "\"acquired_at_utc\":\"2026-10-01T00:00:00Z\"}]";
            var hidden = Row("asset.actor.test_inputs.prefab",
                "client/Assets/Art/actors/in.png",
                "AI_CREATED", "AI_TOOL_TERMS", "PENDING", "null",
                "\"actors_players/pack\"", ValidGen(termsSha), "null",
                "null", incompleteInput, new string('b', 64));
            WriteRegister(tmp, Doc(hidden));
            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed,
                "a third-party input missing its provenance fields must fail");
            Assert.IsTrue(rep.Errors.Exists(
                e => e.Path.Contains("inputs[0]") && e.Field == "sha256"),
                "the missing input field must land on its own indexed path");

            var completeInput = "[{\"creator\":\"x\","
                + "\"source_uri\":\"https://src/i\",\"license_id\":\"CC0-1.0\","
                + "\"license_uri\":\"https://src/lic\","
                + "\"acquired_at_utc\":\"2026-10-01T00:00:00Z\","
                + "\"sha256\":\"" + new string('f', 64) + "\"}]";
            var declared = Row("asset.actor.test_inputs.prefab",
                "client/Assets/Art/actors/in.png",
                "AI_CREATED", "AI_TOOL_TERMS", "PENDING", "null",
                "\"actors_players/pack\"", ValidGen(termsSha), "null",
                "null", completeInput, new string('b', 64));
            WriteRegister(tmp, Doc(declared));
            var rep2 = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsTrue(rep2.Passed,
                "a fully attributed third-party input must pass: "
                + string.Join("; ", rep2.Errors.ConvertAll(e => e.ToString())));
        }
    }
}
