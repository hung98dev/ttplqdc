using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Asset provenance register validator (presentation_asset_manifest.md
    /// section 6). Foundation mode validates the on-disk register itself —
    /// a clean empty register must pass. Release mode additionally requires
    /// every media file under client/Assets/Art (excluding the Provenance
    /// tree, fragments and terms snapshots) to resolve to exactly one
    /// register row with a correct file hash and approval.
    /// </summary>
    public static class ProvenanceValidator
    {
        public const string RegisterRelativePath = "client/Assets/Art/Provenance/asset_source_register.json";
        public const string SchemaRelativePath = "client/Assets/Art/Provenance/register.schema.json";

        private static readonly string[] RequiredRowFields =
        {
            "asset_key", "file_path", "content_id", "source_kind", "creator",
            "source_uri", "license_id", "license_uri", "acquired_at_utc",
            "source_sha256", "final_sha256", "changes", "attribution",
            "style_pack_id", "generation_record", "folklore_card", "inputs",
            "review_state",
        };

        private static readonly string[] RequiredGenFields =
        {
            "tool", "version", "model_id", "model_sha256", "terms_uri",
            "terms_snapshot_sha256", "prompt", "seed", "parameters",
            "workflow_sha256", "reference_uris", "reference_sha256",
            "c2pa_present",
        };

        private static readonly string[] RequiredInputFields =
        {
            "creator", "source_uri", "license_id", "license_uri",
            "acquired_at_utc", "sha256",
        };

        private static readonly string[] RequiredCardFields =
        {
            "source_tales", "regional_variants", "motifs_checked",
        };

        private static readonly string[] ImageExtensions =
        {
            ".png", ".jpg", ".jpeg", ".tga", ".psd", ".psb", ".tif", ".tiff",
        };

        private static readonly string[] AudioExtensions =
        {
            ".wav", ".ogg", ".mp3", ".aiff", ".aif",
        };

        private static readonly string[] FontExtensions =
        {
            ".ttf", ".otf", ".fnt", ".fontsettings",
        };

        public sealed class Error
        {
            public string Path = string.Empty;
            public string Field = string.Empty;
            public string Message = string.Empty;

            public override string ToString()
            {
                return Path + " :: " + Field + " :: " + Message;
            }
        }

        public sealed class Report
        {
            public readonly List<Error> Errors = new List<Error>();
            public int RowCount;
            public int CoveredFiles;
            public string CreditsText = string.Empty;
            public string FontNotice = string.Empty;

            public bool Passed
            {
                get
                {
                    return Errors.Count == 0;
                }
            }
        }

        /// <summary>Foundation gate: validate the register document alone.</summary>
        public static Report ValidateFoundation(string repoRoot)
        {
            var report = new Report();
            string path = Path.Combine(repoRoot, RegisterRelativePath);
            RegisterJson.Node? root = LoadRegister(path, report);
            if (root == null)
            {
                return report;
            }
            CheckRows(root, repoRoot, verifyHashes: true, report);
            return report;
        }

        /// <summary>
        /// Release gate: foundation checks plus coverage — every media file
        /// under client/Assets/Art must resolve to exactly one register row.
        /// </summary>
        public static Report ValidateRelease(string repoRoot)
        {
            var report = new Report();
            string path = Path.Combine(repoRoot, RegisterRelativePath);
            RegisterJson.Node? root = LoadRegister(path, report);
            if (root == null)
            {
                return report;
            }
            CheckRows(root, repoRoot, verifyHashes: true, report);
            string artRoot = Path.Combine(repoRoot, "client", "Assets", "Art");
            var seen = new HashSet<string>(StringComparer.Ordinal);
            var states = new Dictionary<string, string?>(StringComparer.Ordinal);
            if (root.Get("assets") != null && root.Get("assets")!.Type == RegisterJson.Node.Kind.Arr)
            {
                foreach (var row in root.Get("assets")!.Arr!)
                {
                    var fp = row.Get("file_path");
                    if (fp != null && fp.Type == RegisterJson.Node.Kind.Str)
                    {
                        seen.Add(NormalizePath(fp.Str));
                        states[NormalizePath(fp.Str)] = StrField(row, "review_state");
                    }
                }
            }
            if (Directory.Exists(artRoot))
            {
                var files = new List<string>(Directory.GetFiles(artRoot, "*", SearchOption.AllDirectories));
                files.Sort(StringComparer.Ordinal);
                foreach (var file in files)
                {
                    string rel = NormalizePath(file.Substring(repoRoot.Length + 1));
                    if (!IsMediaFile(rel) || IsProvenanceMetadata(rel))
                    {
                        continue;
                    }
                    if (!seen.Contains(rel))
                    {
                        Fail(report, rel, "file_path", "release media file has no register row");
                        continue;
                    }
                    if (states[rel] != "APPROVED")
                    {
                        Fail(report, rel, "review_state",
                            "release media file requires an APPROVED row");
                        continue;
                    }
                    report.CoveredFiles++;
                }
            }
            return report;
        }

        private static RegisterJson.Node? LoadRegister(string path, Report report)
        {
            if (!File.Exists(path))
            {
                Fail(report, RegisterRelativePath, "register", "register file missing");
                return null;
            }
            try
            {
                return RegisterJson.Parse(File.ReadAllText(path));
            }
            catch (RegisterJson.ParseException ex)
            {
                Fail(report, RegisterRelativePath, "register", "invalid JSON: " + ex.Message);
                return null;
            }
        }

        private static void CheckRows(
            RegisterJson.Node root, string repoRoot, bool verifyHashes, Report report)
        {
            var ver = root.Get("schema_version");
            if (ver == null || ver.Type != RegisterJson.Node.Kind.Num || ver.Num != 1.0)
            {
                Fail(report, RegisterRelativePath, "schema_version", "must equal 1");
                return;
            }
            var assets = root.Get("assets");
            if (assets == null || assets.Type != RegisterJson.Node.Kind.Arr)
            {
                Fail(report, RegisterRelativePath, "assets", "must be an array");
                return;
            }
            var rows = assets.Arr!;
            report.RowCount = rows.Count;
            var paths = new HashSet<string>(StringComparer.Ordinal);
            string? prev = null;
            for (int i = 0; i < rows.Count; i++)
            {
                var row = rows[i];
                string where = "assets[" + i.ToString(CultureInfo.InvariantCulture) + "]";
                if (row.Type != RegisterJson.Node.Kind.Obj)
                {
                    Fail(report, where, "row", "row must be an object");
                    continue;
                }
                CheckRow(row, where, repoRoot, verifyHashes, report);
                var fp = row.Get("file_path");
                if (fp != null && fp.Type == RegisterJson.Node.Kind.Str)
                {
                    string norm = NormalizePath(fp.Str);
                    if (!paths.Add(norm))
                    {
                        Fail(report, where, "file_path", "duplicate file_path " + fp.Str);
                    }
                    if (prev != null && string.CompareOrdinal(prev, fp.Str) > 0)
                    {
                        Fail(report, where, "file_path", "assets must sort ascending by file_path");
                    }
                    prev = fp.Str;
                }
            }
        }

        private static void CheckRow(
            RegisterJson.Node row, string where, string repoRoot, bool verifyHashes, Report report)
        {
            foreach (var f in RequiredRowFields)
            {
                if (row.Get(f) == null)
                {
                    Fail(report, where, f, "missing required field");
                }
            }
            if (row.Type != RegisterJson.Node.Kind.Obj || row.Obj == null)
            {
                return;
            }
            var allowed = new HashSet<string>(RequiredRowFields, StringComparer.Ordinal);
            foreach (var k in row.Obj.Keys)
            {
                if (!allowed.Contains(k))
                {
                    Fail(report, where, k, "unknown field (spec section 6 field list is exact)");
                }
            }
            var kind = StrField(row, "source_kind");
            if (kind != null && kind != "AI_CREATED" && kind != "FREE_LICENSED")
            {
                Fail(report, where, "source_kind", "must be AI_CREATED or FREE_LICENSED");
            }
            var lic = StrField(row, "license_id");
            if (lic != null && lic != "CC0-1.0" && lic != "CC-BY-4.0"
                && lic != "OFL-1.1" && lic != "AI_TOOL_TERMS")
            {
                Fail(report, where, "license_id", "unknown license_id");
            }
            CheckSourceLicensePair(row, where, kind, lic, report);
            var state = StrField(row, "review_state");
            if (state != null && state != "PENDING" && state != "APPROVED" && state != "REJECTED")
            {
                Fail(report, where, "review_state", "must be PENDING, APPROVED or REJECTED");
            }
            CheckAssetKey(row, where, report);
            CheckFilePathShape(row, where, report);
            CheckAcquired(row, where, report);
            CheckHashes(row, where, repoRoot, verifyHashes, report);
            CheckStylePack(row, where, report);
            CheckGenerationRecord(row, where, repoRoot, report);
            CheckFolkloreCard(row, where, report);
            CheckInputs(row, where, report);
        }

        private static void CheckSourceLicensePair(
            RegisterJson.Node row, string where, string? kind, string? lic, Report report)
        {
            if (kind == "AI_CREATED")
            {
                var uri = row.Get("source_uri");
                if (uri != null && uri.Type == RegisterJson.Node.Kind.Str && uri.Str.Length != 0)
                {
                    Fail(report, where, "source_uri", "must be null for AI_CREATED");
                }
                if (lic != null && lic != "AI_TOOL_TERMS")
                {
                    Fail(report, where, "license_id", "AI_CREATED rows must use AI_TOOL_TERMS");
                }
            }
            if (kind == "FREE_LICENSED")
            {
                var uri = row.Get("source_uri");
                if (uri == null || uri.Type != RegisterJson.Node.Kind.Str || uri.Str.Length == 0)
                {
                    Fail(report, where, "source_uri", "FREE_LICENSED requires the original source URL");
                }
                if (lic == "AI_TOOL_TERMS")
                {
                    Fail(report, where, "license_id", "FREE_LICENSED cannot use AI_TOOL_TERMS");
                }
            }
            if (lic == "CC-BY-4.0")
            {
                var attr = row.Get("attribution");
                if (attr == null || attr.Type != RegisterJson.Node.Kind.Str || attr.Str.Length == 0)
                {
                    Fail(report, where, "attribution", "CC-BY-4.0 requires an attribution credit line");
                }
            }
            var licUri = row.Get("license_uri");
            if (licUri == null || licUri.Type != RegisterJson.Node.Kind.Str || licUri.Str.Length == 0)
            {
                Fail(report, where, "license_uri", "exact license/terms URL required");
            }
        }

        /// <summary>
        /// asset_key follows the client_assets.md grammar:
        /// asset.&lt;catalog_id|kind&gt;.&lt;name&gt;.&lt;facet&gt; — lowercase
        /// segments, at least three after the asset prefix.
        /// </summary>
        private static void CheckAssetKey(RegisterJson.Node row, string where, Report report)
        {
            var k = row.Get("asset_key");
            if (k == null || k.Type != RegisterJson.Node.Kind.Str)
            {
                return;
            }
            if (!System.Text.RegularExpressions.Regex.IsMatch(
                k.Str, "^asset\\.[a-z0-9_]+(\\.[a-z0-9_]+){2,}$"))
            {
                Fail(report, where, "asset_key",
                    "must match the asset.<kind>.<name>.<facet> grammar");
            }
        }

        /// <summary>
        /// file_path is stored normalized: forward slashes, no leading
        /// slash, no .. or empty segments, repo-relative only.
        /// </summary>
        private static void CheckFilePathShape(RegisterJson.Node row, string where, Report report)
        {
            var fp = row.Get("file_path");
            if (fp == null || fp.Type != RegisterJson.Node.Kind.Str)
            {
                return;
            }
            if (fp.Str.Length == 0 || fp.Str != NormalizePath(fp.Str)
                || fp.Str.Contains("..") || fp.Str.Contains("//"))
            {
                Fail(report, where, "file_path",
                    "must be a normalized repo-relative path");
            }
        }

        private static void CheckAcquired(RegisterJson.Node row, string where, Report report)
        {
            var a = row.Get("acquired_at_utc");
            if (a == null)
            {
                return;
            }
            if (a.Type != RegisterJson.Node.Kind.Str || a.Str.Length == 0)
            {
                Fail(report, where, "acquired_at_utc", "ISO 8601 UTC timestamp required");
                return;
            }
            if (!DateTimeOffset.TryParse(a.Str, CultureInfo.InvariantCulture,
                DateTimeStyles.AssumeUniversal, out var ts) || ts.Offset != TimeSpan.Zero)
            {
                Fail(report, where, "acquired_at_utc", "must parse as ISO 8601 UTC");
            }
        }

        private static void CheckHashes(
            RegisterJson.Node row, string where, string repoRoot, bool verifyHashes, Report report)
        {
            foreach (var f in new[] { "source_sha256", "final_sha256" })
            {
                var h = row.Get(f);
                if (h == null)
                {
                    continue;
                }
                if (h.Type != RegisterJson.Node.Kind.Str || !IsSha256(h.Str))
                {
                    Fail(report, where, f, "must be a 64-hex lowercase SHA-256");
                }
            }
            if (!verifyHashes)
            {
                return;
            }
            var fp = row.Get("file_path");
            var fh = row.Get("final_sha256");
            if (fp == null || fp.Type != RegisterJson.Node.Kind.Str
                || fh == null || fh.Type != RegisterJson.Node.Kind.Str || !IsSha256(fh.Str))
            {
                return;
            }
            string disk = Path.Combine(repoRoot, NormalizePath(fp.Str).Replace('/', Path.DirectorySeparatorChar));
            if (!File.Exists(disk))
            {
                return;
            }
            string actual = Sha256File(disk);
            if (!string.Equals(actual, fh.Str, StringComparison.Ordinal))
            {
                Fail(report, where, "final_sha256", "file hash mismatch for " + fp.Str);
            }
        }

        private static void CheckStylePack(RegisterJson.Node row, string where, Report report)
        {
            var fp = row.Get("file_path");
            var pack = row.Get("style_pack_id");
            if (pack == null)
            {
                return;
            }
            bool image = fp != null && fp.Type == RegisterJson.Node.Kind.Str
                && HasAnyExtension(fp.Str, ImageExtensions);
            if (!image)
            {
                if (pack.Type != RegisterJson.Node.Kind.Null)
                {
                    Fail(report, where, "style_pack_id", "null allowed only for non-image media");
                }
                return;
            }
            if (pack.Type != RegisterJson.Node.Kind.Str
                || !System.Text.RegularExpressions.Regex.IsMatch(pack.Str, "^[a-z0-9_]+/[a-z0-9_]+$"))
            {
                Fail(report, where, "style_pack_id", "image rows require <fragment>/<pack_id>");
            }
        }

        private static void CheckGenerationRecord(
            RegisterJson.Node row, string where, string repoRoot, Report report)
        {
            var gen = row.Get("generation_record");
            var kind = StrField(row, "source_kind");
            if (gen == null)
            {
                return;
            }
            if (kind == "AI_CREATED" && gen.Type == RegisterJson.Node.Kind.Null)
            {
                Fail(report, where, "generation_record", "AI_CREATED requires generation_record");
                return;
            }
            if (gen.Type == RegisterJson.Node.Kind.Null)
            {
                return;
            }
            if (gen.Type != RegisterJson.Node.Kind.Obj || gen.Obj == null)
            {
                Fail(report, where, "generation_record", "must be an object or null");
                return;
            }
            foreach (var f in RequiredGenFields)
            {
                if (gen.Get(f) == null)
                {
                    Fail(report, where, "generation_record." + f, "missing required field");
                }
            }
            var allowed = new HashSet<string>(RequiredGenFields, StringComparer.Ordinal);
            foreach (var k in gen.Obj.Keys)
            {
                if (k == "style_pack_id")
                {
                    Fail(report, where, "generation_record.style_pack_id",
                        "legacy nested field removed; style_pack_id lives on the row");
                    continue;
                }
                if (!allowed.Contains(k))
                {
                    Fail(report, where, "generation_record." + k, "unknown field");
                }
            }
            foreach (var f in new[] { "tool", "version", "model_id", "terms_uri", "prompt" })
            {
                var v = gen.Get(f);
                if (v != null && (v.Type != RegisterJson.Node.Kind.Str || v.Str.Length == 0))
                {
                    Fail(report, where, "generation_record." + f, "non-empty string required");
                }
            }
            foreach (var f in new[] { "model_sha256", "workflow_sha256" })
            {
                var v = gen.Get(f);
                if (v != null && v.Type != RegisterJson.Node.Kind.Null
                    && (v.Type != RegisterJson.Node.Kind.Str || !IsSha256(v.Str)))
                {
                    Fail(report, where, "generation_record." + f,
                        "null or 64-hex SHA-256 required");
                }
            }
            var terms = gen.Get("terms_snapshot_sha256");
            if (terms != null)
            {
                if (terms.Type != RegisterJson.Node.Kind.Str || !IsSha256(terms.Str))
                {
                    Fail(report, where, "generation_record.terms_snapshot_sha256",
                        "64-hex SHA-256 of the stored terms copy required");
                }
                else
                {
                    CheckTermsSnapshot(row, where, terms.Str, repoRoot, report);
                }
            }
            var refs = gen.Get("reference_uris");
            if (refs != null && refs.Type != RegisterJson.Node.Kind.Arr)
            {
                Fail(report, where, "generation_record.reference_uris", "array required");
            }
            var refh = gen.Get("reference_sha256");
            if (refh != null)
            {
                if (refh.Type != RegisterJson.Node.Kind.Arr)
                {
                    Fail(report, where, "generation_record.reference_sha256", "array required");
                }
                else
                {
                    for (int i = 0; i < refh.Arr!.Count; i++)
                    {
                        var h = refh.Arr[i];
                        if (h.Type != RegisterJson.Node.Kind.Str || !IsSha256(h.Str))
                        {
                            Fail(report, where, "generation_record.reference_sha256[" + i + "]",
                                "64-hex SHA-256 required");
                        }
                    }
                }
            }
            var c2pa = gen.Get("c2pa_present");
            if (c2pa != null && c2pa.Type != RegisterJson.Node.Kind.Bool)
            {
                Fail(report, where, "generation_record.c2pa_present", "boolean required");
            }
        }

        /// <summary>
        /// terms_snapshot_sha256 must resolve under
        /// client/Assets/Art/Provenance/terms/&lt;fragment&gt;/&lt;sha256&gt;.txt
        /// and hash-match the stored copy. The fragment comes from
        /// style_pack_id (&lt;fragment&gt;/&lt;pack_id&gt;); the validator
        /// never creates terms directories — fragment owners write them.
        /// </summary>
        private static void CheckTermsSnapshot(
            RegisterJson.Node row, string where, string sha, string repoRoot, Report report)
        {
            var pack = row.Get("style_pack_id");
            if (pack == null || pack.Type != RegisterJson.Node.Kind.Str
                || !pack.Str.Contains("/"))
            {
                Fail(report, where, "generation_record.terms_snapshot_sha256",
                    "cannot resolve terms fragment without a valid style_pack_id");
                return;
            }
            string fragment = pack.Str.Substring(0, pack.Str.IndexOf('/'));
            string termsPath = Path.Combine(repoRoot, "client", "Assets", "Art",
                "Provenance", "terms", fragment, sha + ".txt");
            if (!File.Exists(termsPath))
            {
                Fail(report, where, "generation_record.terms_snapshot_sha256",
                    "terms snapshot missing at terms/" + fragment + "/" + sha + ".txt");
                return;
            }
            if (!string.Equals(Sha256File(termsPath), sha, StringComparison.Ordinal))
            {
                Fail(report, where, "generation_record.terms_snapshot_sha256",
                    "terms snapshot content hash mismatch");
            }
        }

        private static void CheckFolkloreCard(RegisterJson.Node row, string where, Report report)
        {
            var card = row.Get("folklore_card");
            if (card == null || card.Type == RegisterJson.Node.Kind.Null)
            {
                return;
            }
            if (card.Type != RegisterJson.Node.Kind.Obj)
            {
                Fail(report, where, "folklore_card", "must be an object or null");
                return;
            }
            foreach (var f in RequiredCardFields)
            {
                var v = card.Get(f);
                if (v == null)
                {
                    Fail(report, where, "folklore_card." + f, "missing required field");
                    continue;
                }
                if (v.Type != RegisterJson.Node.Kind.Arr)
                {
                    Fail(report, where, "folklore_card." + f, "array required");
                    continue;
                }
                if ((f == "source_tales" || f == "motifs_checked") && v.Arr!.Count == 0)
                {
                    Fail(report, where, "folklore_card." + f, "must not be empty");
                }
            }
        }

        private static void CheckInputs(RegisterJson.Node row, string where, Report report)
        {
            var inputs = row.Get("inputs");
            if (inputs == null)
            {
                return;
            }
            if (inputs.Type != RegisterJson.Node.Kind.Arr)
            {
                Fail(report, where, "inputs", "array required");
                return;
            }
            for (int i = 0; i < inputs.Arr!.Count; i++)
            {
                var inp = inputs.Arr[i];
                string iw = where + ".inputs[" + i.ToString(CultureInfo.InvariantCulture) + "]";
                if (inp.Type != RegisterJson.Node.Kind.Obj)
                {
                    Fail(report, iw, "input", "must be an object");
                    continue;
                }
                foreach (var f in RequiredInputFields)
                {
                    if (inp.Get(f) == null)
                    {
                        Fail(report, iw, f, "missing required field");
                    }
                }
                var sha = inp.Get("sha256");
                if (sha != null && (sha.Type != RegisterJson.Node.Kind.Str || !IsSha256(sha.Str)))
                {
                    Fail(report, iw, "sha256", "64-hex SHA-256 required");
                }
                var lic = inp.Get("license_id");
                if (lic != null && lic.Type == RegisterJson.Node.Kind.Str
                    && lic.Str != "CC0-1.0" && lic.Str != "CC-BY-4.0"
                    && lic.Str != "OFL-1.1" && lic.Str != "AI_TOOL_TERMS")
                {
                    Fail(report, iw, "license_id", "unknown license_id");
                }
            }
        }

        /// <summary>Deterministic credits text from approved CC-BY-4.0 rows.</summary>
        public static string CreditsText(RegisterJson.Node root)
        {
            var sb = new StringBuilder();
            var rows = RowsOf(root);
            for (int i = 0; i < rows.Count; i++)
            {
                var row = rows[i];
                if (StrField(row, "review_state") != "APPROVED"
                    || StrField(row, "license_id") != "CC-BY-4.0")
                {
                    continue;
                }
                var attr = StrField(row, "attribution");
                if (attr != null)
                {
                    sb.AppendLine(attr);
                }
            }
            return sb.ToString();
        }

        /// <summary>Deterministic OFL-1.1 font notice from approved rows.</summary>
        public static string FontNotice(RegisterJson.Node root)
        {
            var sb = new StringBuilder();
            var rows = RowsOf(root);
            for (int i = 0; i < rows.Count; i++)
            {
                var row = rows[i];
                if (StrField(row, "review_state") != "APPROVED"
                    || StrField(row, "license_id") != "OFL-1.1")
                {
                    continue;
                }
                var attr = StrField(row, "attribution");
                if (attr != null)
                {
                    sb.AppendLine(attr);
                }
            }
            return sb.ToString();
        }

        private static List<RegisterJson.Node> RowsOf(RegisterJson.Node root)
        {
            var assets = root.Get("assets");
            if (assets == null || assets.Type != RegisterJson.Node.Kind.Arr)
            {
                return new List<RegisterJson.Node>();
            }
            return assets.Arr!;
        }

        private static string? StrField(RegisterJson.Node row, string field)
        {
            var n = row.Get(field);
            return n == null || n.Type != RegisterJson.Node.Kind.Str ? null : n.Str;
        }

        public static string Sha256File(string path)
        {
            using (var sha = SHA256.Create())
            using (var fs = File.OpenRead(path))
            {
                var bytes = sha.ComputeHash(fs);
                var sb = new StringBuilder(64);
                for (int i = 0; i < bytes.Length; i++)
                {
                    sb.Append(bytes[i].ToString("x2", CultureInfo.InvariantCulture));
                }
                return sb.ToString();
            }
        }

        public static bool IsSha256(string s)
        {
            if (s.Length != 64)
            {
                return false;
            }
            for (int i = 0; i < s.Length; i++)
            {
                char c = s[i];
                bool hex = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f');
                if (!hex)
                {
                    return false;
                }
            }
            return true;
        }

        public static string NormalizePath(string p)
        {
            return p.Replace('\\', '/').TrimStart('/');
        }

        private static bool HasAnyExtension(string path, string[] exts)
        {
            string ext = Path.GetExtension(path).ToLowerInvariant();
            for (int i = 0; i < exts.Length; i++)
            {
                if (ext == exts[i])
                {
                    return true;
                }
            }
            return false;
        }

        public static bool IsMediaFile(string rel)
        {
            return HasAnyExtension(rel, ImageExtensions)
                || HasAnyExtension(rel, AudioExtensions)
                || HasAnyExtension(rel, FontExtensions);
        }

        private static bool IsProvenanceMetadata(string rel)
        {
            return rel.StartsWith("client/Assets/Art/Provenance/", StringComparison.Ordinal)
                || rel.EndsWith(".meta", StringComparison.Ordinal);
        }

        private static void Fail(Report report, string path, string field, string message)
        {
            report.Errors.Add(new Error { Path = path, Field = field, Message = message });
        }
    }
}
