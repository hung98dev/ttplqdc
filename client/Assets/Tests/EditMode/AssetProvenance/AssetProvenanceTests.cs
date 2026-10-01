using System.IO;
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

        private static string WriteTempRegister(string json)
        {
            var dir = Path.Combine(RepoRoot(), "artifacts", "provenance-tests");
            var provenance = Path.Combine(dir, "client", "Assets", "Art", "Provenance");
            Directory.CreateDirectory(provenance);
            var path = Path.Combine(provenance, "asset_source_register.json");
            File.WriteAllText(path, json);
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

            var rep = ProvenanceValidator.ValidateFoundation(tmp);
            Assert.IsFalse(rep.Passed,
                "row must fail when the terms snapshot is absent");
            Assert.IsTrue(rep.Errors.Exists(e => e.Field.Contains("terms_snapshot_sha256")),
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
    }
}
