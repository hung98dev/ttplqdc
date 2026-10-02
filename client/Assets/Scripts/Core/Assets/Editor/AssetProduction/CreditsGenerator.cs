using System.IO;
using System.Text;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Deterministic credits/font-notice generation from the provenance
    /// register (presentation_asset_manifest.md section 6): the CC-BY-4.0
    /// attribution list and the OFL-1.1 font notice derive from the register
    /// alone and are packaged so players can reach them in credits.
    /// </summary>
    public static class CreditsGenerator
    {
        /// <summary>Credits block derived from approved CC-BY-4.0 rows.</summary>
        public static string BuildCreditsText(string repoRoot)
        {
            var root = Load(repoRoot);
            if (root == null)
            {
                return string.Empty;
            }
            return ProvenanceValidator.CreditsText(root);
        }

        /// <summary>Font notice block derived from approved OFL-1.1 rows.</summary>
        public static string BuildFontNotice(string repoRoot)
        {
            var root = Load(repoRoot);
            if (root == null)
            {
                return string.Empty;
            }
            return ProvenanceValidator.FontNotice(root);
        }

        /// <summary>
        /// Write both deterministic outputs beside the build artifacts —
        /// temporary credits output that cleanup_obligations removes after
        /// packaging.
        /// </summary>
        public static void WriteOutputs(string repoRoot, string outDir)
        {
            Directory.CreateDirectory(outDir);
            File.WriteAllText(Path.Combine(outDir, "credits.txt"), BuildCreditsText(repoRoot));
            File.WriteAllText(Path.Combine(outDir, "ofl-notice.txt"), BuildFontNotice(repoRoot));
        }

        private static RegisterJson.Node? Load(string repoRoot)
        {
            string path = Path.Combine(repoRoot, ProvenanceValidator.RegisterRelativePath);
            if (!File.Exists(path))
            {
                return null;
            }
            try
            {
                return RegisterJson.Parse(File.ReadAllText(path));
            }
            catch (RegisterJson.ParseException)
            {
                return null;
            }
        }
    }
}
