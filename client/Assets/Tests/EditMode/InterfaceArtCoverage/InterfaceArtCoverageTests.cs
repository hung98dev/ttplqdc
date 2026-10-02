using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.InterfaceArtCoverage
{
    /// <summary>
    /// IMP-073 interface-art coverage (presentation_asset_manifest.md §1-§3,
    /// §3.8-§3.9, §5-§6; client_assets.md § Stable Asset Keys): the interface
    /// fragment must carry a register row for every produced media file, keys
    /// must resolve through Addressables entries to on-disk files whose .meta
    /// GUID matches the entry, glyph coverage must include the Vietnamese set
    /// in NFC without stacked-diacritic clipping, nine-slice and VFX flipbook
    /// limits must be declared and respected, and group size estimates must
    /// fit the manifest §1 budgets.
    /// </summary>
    public class InterfaceArtCoverageTests
    {
        private const string FragmentPath =
            "client/Assets/Art/Provenance/fragments/interface.json";
        private const string ArtRoot = "client/Assets/Art";
        private const string GroupsDir =
            "client/Assets/AddressableAssetsData/AssetGroups";

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
            return Path.Combine(RepoRoot(), rel.Replace('/', Path.DirectorySeparatorChar));
        }

        // ---------- fragment ----------

        private static RegisterJson.Node Fragment()
        {
            string path = Abs(FragmentPath);
            Assert.IsTrue(File.Exists(path), "interface fragment missing");
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
            return v != null && v.Type == RegisterJson.Node.Kind.Str ? v.Str : null;
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
            Assert.Greater(rows.Count, 0, "interface fragment has no rows");
            string? prev = null;
            var seen = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in rows)
            {
                var path = Str(row, "file_path");
                Assert.IsNotNull(path, "row without file_path");
                Assert.IsTrue(File.Exists(Abs(path!)), "row file missing: " + path);
                if (prev != null)
                {
                    Assert.IsTrue(
                        string.CompareOrdinal(prev, path) < 0,
                        "assets array not sorted by file_path: " + path);
                }
                prev = path;
                Assert.IsTrue(seen.Add(path!), "duplicate file_path row: " + path);
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
                Assert.IsTrue(termsUri!.StartsWith("https://", StringComparison.Ordinal));
                var snap = Str(gen, "terms_snapshot_sha256");
                Assert.IsNotNull(snap, "terms_snapshot_sha256 missing: " + path);
                var pack = Str(row, "style_pack_id");
                Assert.IsNotNull(pack, "style_pack_id missing: " + path);
                var fragment = pack!.Split('/')[0];
                var snapPath = "client/Assets/Art/Provenance/terms/" + fragment
                    + "/" + snap + ".txt";
                Assert.IsTrue(File.Exists(Abs(snapPath)),
                    "terms snapshot missing: " + snapPath);
                Assert.AreEqual(snap, Sha256File(Abs(snapPath)),
                    "terms snapshot hash mismatch");
            }
            Assert.Greater(aiRows, 0, "no AI_CREATED rows in fragment");
        }

        // ---------- import manifests / gate scope ----------

        private sealed class ImportDir
        {
            public string Dir = string.Empty;
            public string AssetClass = string.Empty;
            public readonly Dictionary<string, RegisterJson.Node> Assets =
                new Dictionary<string, RegisterJson.Node>(StringComparer.Ordinal);

            public string DirAbs()
            {
                return Path.Combine(RepoRoot(),
                    Dir.Replace('/', Path.DirectorySeparatorChar));
            }
        }

        private static List<ImportDir> LoadImportJsons()
        {
            var list = new List<ImportDir>();
            var artAbs = Abs(ArtRoot);
            foreach (var file in Directory.GetFiles(artAbs, "import.json",
                SearchOption.AllDirectories))
            {
                var doc = RegisterJson.Parse(File.ReadAllText(file));
                var d = new ImportDir();
                d.Dir = Path.GetDirectoryName(file)!
                    .Substring(RepoRoot().Length + 1)
                    .Replace(Path.DirectorySeparatorChar, '/');
                d.AssetClass = Str(doc, "asset_class") ?? string.Empty;
                var assets = doc.Get("assets");
                if (assets != null && assets.Type == RegisterJson.Node.Kind.Obj)
                {
                    foreach (var kv in assets.Obj!)
                    {
                        d.Assets[kv.Key] = kv.Value;
                    }
                }
                list.Add(d);
            }
            return list;
        }

        [Test]
        public void TestAssetClassGateScope()
        {
            var dirs = LoadImportJsons();
            Assert.Greater(dirs.Count, 0, "no import.json manifests found");
            var allowed = new HashSet<string>(StringComparer.Ordinal)
            { "UI_ART", "FONT_ATLAS", "ITEM_ICON", "EQUIPMENT_ICON", "VFX_SOFT" };
            foreach (var d in dirs)
            {
                Assert.IsTrue(allowed.Contains(d.AssetClass),
                    "unknown asset_class in " + d.Dir + ": " + d.AssetClass);
                foreach (var file in Directory.GetFiles(d.DirAbs(), "*.png"))
                {
                    string name = Path.GetFileName(file);
                    Assert.IsTrue(d.Assets.ContainsKey(name),
                        "undocumented png " + name + " in " + d.Dir);
                }
            }
            // gate-scope spot rules from manifest §3.1a: FONT_ATLAS dirs hold
            // no cutout-scoped images; VFX_SOFT declares soft_edges.
            foreach (var d in dirs)
            {
                if (d.AssetClass == "FONT_ATLAS")
                {
                    Assert.AreEqual(0,
                        Directory.GetFiles(d.DirAbs(), "*.png").Length,
                        "FONT_ATLAS dir must not carry png media: " + d.Dir);
                }
                if (d.AssetClass == "VFX_SOFT")
                {
                    foreach (var kv in d.Assets)
                    {
                        var se = kv.Value.Get("soft_edges");
                        Assert.IsNotNull(se,
                            "VFX_SOFT row lacks soft_edges: " + kv.Key);
                        Assert.AreEqual(RegisterJson.Node.Kind.Bool, se!.Type);
                        Assert.IsTrue(se.Bool, kv.Key + " soft_edges must be true");
                    }
                }
            }
        }

        // ---------- nine-slice ----------

        private static double[] MetaBorder(string pngRel)
        {
            var metaPath = Abs(pngRel + ".meta");
            Assert.IsTrue(File.Exists(metaPath), "missing .meta: " + pngRel);
            var text = File.ReadAllText(metaPath);
            var idx = text.IndexOf("spriteBorder:", StringComparison.Ordinal);
            Assert.IsTrue(idx >= 0, "no spriteBorder in " + pngRel);
            var m = System.Text.RegularExpressions.Regex.Match(
                text.Substring(idx),
                @"spriteBorder:\s*\{x:\s*([\d.eE+-]+),\s*y:\s*([\d.eE+-]+),\s*z:\s*([\d.eE+-]+),\s*w:\s*([\d.eE+-]+)\}");
            Assert.IsTrue(m.Success, "unparseable spriteBorder in " + pngRel);
            var b = new double[4];
            for (int i = 0; i < 4; i++)
            {
                b[i] = double.Parse(m.Groups[i + 1].Value,
                    CultureInfo.InvariantCulture);
            }
            return b;
        }

        [Test]
        public void TestNineSliceBordersDeclared()
        {
            int n = 0;
            foreach (var d in LoadImportJsons())
            {
                foreach (var kv in d.Assets)
                {
                    var ns = kv.Value.Get("nine_slice");
                    if (ns == null || ns.Type != RegisterJson.Node.Kind.Arr)
                    {
                        continue;
                    }
                    n++;
                    Assert.AreEqual(4, ns.Arr!.Count, kv.Key + " nine_slice arity");
                    var border = MetaBorder(d.Dir + "/" + kv.Key);
                    for (int i = 0; i < 4; i++)
                    {
                        var want = ns.Arr[i].Num;
                        Assert.AreEqual(want, border[i], 0.001,
                            kv.Key + " spriteBorder mismatch at " + i);
                    }
                    // stretch band L* std <= 2 else the cell must be tiled
                    var rel = d.Dir + "/" + kv.Key;
                    var bytes = File.ReadAllBytes(Abs(rel));
                    var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                    Assert.IsTrue(tex.LoadImage(bytes), "decode failed " + rel);
                    var px = tex.GetPixels32();
                    int w = tex.width, h = tex.height;
                    var ls = new List<double>();
                    int bx = (int)border[0], by = (int)border[1];
                    int bz = (int)border[2], bw = (int)border[3];
                    int x0 = bx + 2, x1 = w - bz - 2;
                    int y0 = h - by - 2, y1 = bw + 2;
                    for (int y = y1; y < y0 && y < h; y++)
                    {
                        for (int x = x0; x < x1 && x < w; x++)
                        {
                            var p = px[y * w + x];
                            if (p.a == 0)
                            {
                                continue;
                            }
                            ls.Add(LabPixels.ToLab(p.r, p.g, p.b).L);
                        }
                    }
                    UnityEngine.Object.DestroyImmediate(tex);
                    if (ls.Count > 1)
                    {
                        double mean = 0;
                        foreach (var v in ls)
                        {
                            mean += v;
                        }
                        mean /= ls.Count;
                        double var_ = 0;
                        foreach (var v in ls)
                        {
                            var_ += (v - mean) * (v - mean);
                        }
                        double std = Math.Sqrt(var_ / ls.Count);
                        Assert.LessOrEqual(std, 2.0,
                            rel + " stretch-band L* std " + std.ToString("F2",
                            CultureInfo.InvariantCulture) + " > 2 (use Tiled)");
                    }
                }
            }
            Assert.Greater(n, 0, "no nine-slice assets declared");
        }

        // ---------- vfx ----------

        [Test]
        public void TestVfxFlipbookLimits()
        {
            int sheets = 0;
            foreach (var d in LoadImportJsons())
            {
                foreach (var kv in d.Assets)
                {
                    var fb = kv.Value.Get("flipbook");
                    if (fb == null || fb.Type != RegisterJson.Node.Kind.Obj)
                    {
                        continue;
                    }
                    sheets++;
                    var frames = fb.Get("frames")!.Num;
                    Assert.GreaterOrEqual(frames, 2, kv.Key + " frames");
                    Assert.LessOrEqual(frames, 16, kv.Key + " frames > 16");
                    var fps = fb.Get("fps")!.Num;
                    Assert.IsTrue(fps == 12.0 || fps == 24.0,
                        kv.Key + " fps must be 12 or 24");
                    var blend = Str(fb, "blend");
                    Assert.IsTrue(blend == "ADDITIVE" || blend == "ALPHA",
                        kv.Key + " blend invalid: " + blend);
                    var mi = fb.Get("max_instances");
                    Assert.IsNotNull(mi, kv.Key + " max_instances missing");
                    Assert.Greater(mi!.Num, 0, kv.Key + " max_instances");
                    var png = Abs(d.Dir + "/" + kv.Key);
                    var dim = PngSize(png);
                    Assert.LessOrEqual(dim.Item1, 1024, kv.Key + " sheet width");
                    Assert.LessOrEqual(dim.Item2, 1024, kv.Key + " sheet height");
                    var cell = kv.Value.Get("cell_ref");
                    Assert.IsNotNull(cell, kv.Key + " cell_ref missing");
                    Assert.AreEqual(0.0, cell!.Num % 16.0,
                        kv.Key + " cell_ref not multiple of 16");
                    Assert.LessOrEqual(cell.Num, 512.0, kv.Key + " cell_ref >512");
                    // texture = 2 x cell_ref
                    Assert.AreEqual(cell.Num * 2.0, (double)dim.Item1,
                        kv.Key + " sheet must be 2x cell_ref");
                }
            }
            Assert.Greater(sheets, 0, "no flipbook sheets declared");
        }

        private static (int, int) PngSize(string path)
        {
            var b = new byte[24];
            using (var fs = File.OpenRead(path))
            {
                Assert.AreEqual(24, fs.Read(b, 0, 24), "short png " + path);
            }
            Assert.AreEqual(0x89, b[0], "not png " + path);
            int w = (b[16] << 24) | (b[17] << 16) | (b[18] << 8) | b[19];
            int h = (b[20] << 24) | (b[21] << 16) | (b[22] << 8) | b[23];
            return (w, h);
        }

        // ---------- fonts ----------

        /// <summary>Minimal TTF reader: cmap format 4 + head/hhea/OS-2 metrics.</summary>
        private sealed class Ttf
        {
            public readonly HashSet<int> Glyphs = new HashSet<int>();
            public double UnitsPerEm;
            public double HeadYMax;
            public double WinAscent;
            private readonly byte[] _b;
            private readonly Dictionary<int, int> _gid = new Dictionary<int, int>();
            private readonly List<long> _loca = new List<long>();
            private long _glyf;

            private ushort U16(long o)
            {
                return (ushort)((_b[o] << 8) | _b[o + 1]);
            }
            private short S16(long o)
            {
                return (short)((_b[o] << 8) | _b[o + 1]);
            }
            private uint U32(long o)
            {
                return (uint)((_b[o] << 24) | (_b[o + 1] << 16)
                    | (_b[o + 2] << 8) | _b[o + 3]);
            }

            public Ttf(string path)
            {
                _b = File.ReadAllBytes(path);
                int numTables = U16(4);
                long cmap = 0, head = 0, hhea = 0, os2 = 0, maxp = 0;
                for (int i = 0; i < numTables; i++)
                {
                    long rec = 12 + i * 16;
                    var tag = Encoding.ASCII.GetString(_b, (int)rec, 4);
                    long off = U32(rec + 8);
                    if (tag == "cmap")
                    {
                        cmap = off;
                    }
                    if (tag == "head")
                    {
                        head = off;
                    }
                    if (tag == "hhea")
                    {
                        hhea = off;
                    }
                    if (tag == "OS/2")
                    {
                        os2 = off;
                    }
                    if (tag == "maxp")
                    {
                        maxp = off;
                    }
                    if (tag == "loca")
                    {
                        _loca.Clear();
                        _loca.Add(off);
                        _loca.Add(U32(rec + 12));
                    }
                    if (tag == "glyf")
                    {
                        _glyf = off;
                    }
                }
                Assert.AreNotEqual(0, cmap, "no cmap in " + path);
                UnitsPerEm = U16(head + 18);
                HeadYMax = S16(head + 42);
                WinAscent = os2 != 0 ? U16(os2 + 74) : 0;
                int indexToLocFormat = S16(head + 50);
                int numGlyphs = U16(maxp + 4);
                // cmap format 4 (platform 3 enc 1) or 12
                int subCount = U16(cmap + 2);
                long chosen = 0; int fmt = 0;
                for (int i = 0; i < subCount; i++)
                {
                    long e = cmap + 4 + i * 8;
                    int platform = U16(e); int enc = U16(e + 2);
                    long sub = cmap + U32(e + 4);
                    int f = U16(sub);
                    if (f == 12)
                    {
                        chosen = sub; fmt = 12;
                        break;
                    }
                    if (f == 4 && platform == 3 && (enc == 1 || enc == 10))
                    {
                        chosen = sub; fmt = 4;
                    }
                }
                Assert.AreNotEqual(0, chosen, "no usable cmap subtable");
                if (fmt == 4)
                {
                    ParseFmt4(chosen);
                }
                else
                {
                    ParseFmt12(chosen);
                }
                // loca offsets
                int count = numGlyphs + 1;
                _loca.Clear();
                // re-scan table dir for loca offset
                for (int i = 0; i < numTables; i++)
                {
                    long rec = 12 + i * 16;
                    var tag = Encoding.ASCII.GetString(_b, (int)rec, 4);
                    if (tag == "loca")
                    {
                        long off = U32(rec + 8);
                        for (int g = 0; g < count; g++)
                        {
                            if (indexToLocFormat == 0)
                            {
                                _loca.Add(U16(off + g * 2) * 2L);
                            }
                            else
                            {
                                _loca.Add(U32(off + g * 4));
                            }
                        }
                    }
                }
            }

            private void ParseFmt4(long s)
            {
                int segCount = U16(s + 6) / 2;
                long endCodes = s + 14;
                long startCodes = endCodes + segCount * 2 + 2;
                long idDelta = startCodes + segCount * 2;
                long idRange = idDelta + segCount * 2;
                for (int i = 0; i < segCount; i++)
                {
                    int end = U16(endCodes + i * 2);
                    int start = U16(startCodes + i * 2);
                    int delta = S16(idDelta + i * 2);
                    int ro = S16(idRange + i * 2);
                    if (start == 0xFFFF)
                    {
                        continue;
                    }
                    for (int c = start; c <= end; c++)
                    {
                        Glyphs.Add(c);
                        int g;
                        if (ro == 0)
                        {
                            g = (c + delta) & 0xFFFF;
                        }
                        else
                        {
                            long addr = idRange + i * 2 + ro + (c - start) * 2;
                            int v = U16(addr);
                            g = v == 0 ? 0 : (v + delta) & 0xFFFF;
                        }
                        if (g != 0 && !_gid.ContainsKey(c))
                        {
                            _gid[c] = g;
                        }
                    }
                }
            }

            private void ParseFmt12(long s)
            {
                int nGroups = (int)U32(s + 12);
                for (int i = 0; i < nGroups; i++)
                {
                    long g = s + 16 + i * 12;
                    uint start = U32(g); uint end = U32(g + 4); uint gid = U32(g + 8);
                    for (uint c = start; c <= end; c++)
                    {
                        Glyphs.Add((int)c);
                        if (!_gid.ContainsKey((int)c))
                        {
                            _gid[(int)c] = (int)(gid + c - start);
                        }
                    }
                }
            }

            /// <summary>Glyph yMax in font units (glyf table bbox).</summary>
            public int GlyphYMax(int codepoint)
            {
                if (!_gid.TryGetValue(codepoint, out int g))
                {
                    return int.MinValue;
                }
                if (g + 1 >= _loca.Count)
                {
                    return int.MinValue;
                }
                long start = _glyf + _loca[g];
                long end = _glyf + _loca[g + 1];
                if (end - start < 10)
                {
                    return int.MinValue;
                    // empty glyph
                }
                return S16(start + 8);
            }
        }

        private static readonly string[] FontFiles =
        {
            "client/Assets/Art/UI/Fonts/BeVietnamPro-Regular.ttf",
            "client/Assets/Art/UI/Fonts/BeVietnamPro-Bold.ttf",
        };

        private static int[] RequiredGlyphSet()
        {
            // presentation_asset_manifest.md §5 Vietnamese coverage set
            var set = new List<int>();
            for (int c = 0x0020; c <= 0x007E; c++)
            {
                set.Add(c);
            }
            for (int c = 0x00A0; c <= 0x00FF; c++)
            {
                set.Add(c);
            }
            for (int c = 0x0102; c <= 0x0103; c++)
            {
                set.Add(c);
            }
            for (int c = 0x0110; c <= 0x0111; c++)
            {
                set.Add(c);
            }
            for (int c = 0x0128; c <= 0x0129; c++)
            {
                set.Add(c);
            }
            for (int c = 0x0168; c <= 0x0169; c++)
            {
                set.Add(c);
            }
            for (int c = 0x01A0; c <= 0x01A1; c++)
            {
                set.Add(c);
            }
            for (int c = 0x01AF; c <= 0x01B0; c++)
            {
                set.Add(c);
            }
            for (int c = 0x1EA0; c <= 0x1EF9; c++)
            {
                set.Add(c);
            }
            set.Add(0x20AB);
            return set.ToArray();
        }

        [Test]
        public void TestVietnameseGlyphSetNfc()
        {
            var fonts = new Ttf[FontFiles.Length];
            for (int i = 0; i < FontFiles.Length; i++)
            {
                Assert.IsTrue(File.Exists(Abs(FontFiles[i])),
                    "font missing: " + FontFiles[i]);
                fonts[i] = new Ttf(Abs(FontFiles[i]));
            }
            foreach (var f in fonts)
            {
                foreach (int c in RequiredGlyphSet())
                {
                    Assert.IsTrue(f.Glyphs.Contains(c),
                        "glyph U+" + c.ToString("X4", CultureInfo.InvariantCulture)
                        + " missing from font");
                }
            }
            // NFC normalization: composed Vietnamese text must remain stable
            var samples = new[]
            {
                "Đảng viên", "Nguyễn Trãi", "thủy", "mộc", "hỏa", "thổ", "kim",
                "Ẳ", "Ẵ", "Ổ", "Ỗ", "Ẫ", "Ấ", "Ỡ", "Ữ", "vũ khí", "đũa tre",
                "Ngưỡng mộ", "Nghiệp chướng", "₫",
            };
            foreach (var s in samples)
            {
                var nfc = s.Normalize(NormalizationForm.FormC);
                Assert.AreEqual(s, nfc, "string not NFC: " + s);
                foreach (var f in fonts)
                {
                    foreach (var ch in nfc)
                    {
                        Assert.IsTrue(f.Glyphs.Contains(ch),
                            "font lacks U+" + ((int)ch).ToString("X4",
                            CultureInfo.InvariantCulture) + " from " + s);
                    }
                }
            }
        }

        [Test]
        public void TestStackedDiacriticsDoNotClip()
        {
            var stacked = new[]
            { 'Ẳ', 'Ẵ', 'Ổ', 'Ỗ', 'Ẫ', 'Ấ', 'Ỡ', 'Ữ' };
            foreach (var rel in FontFiles)
            {
                var f = new Ttf(Abs(rel));
                foreach (var ch in stacked)
                {
                    var ymax = f.GlyphYMax(ch);
                    Assert.AreNotEqual(int.MinValue, ymax,
                        rel + " no glyph for U+" + ((int)ch).ToString("X4",
                        CultureInfo.InvariantCulture));
                    Assert.LessOrEqual(ymax, f.HeadYMax,
                        rel + " glyph " + ch + " exceeds font bounding box");
                    if (f.WinAscent > 0)
                    {
                        Assert.LessOrEqual(ymax, f.WinAscent,
                            rel + " glyph " + ch + " exceeds usWinAscent");
                    }
                }
            }
        }

        // ---------- coverage / keys / budgets ----------

        [Test]
        public void TestCatalogIconCoverage()
        {
            // every fragment row key parses, routes, and lands in the group
            // file that carries its m_Address entry
            var entries = ReadGroupEntries();
            foreach (var row in Rows())
            {
                var key = Str(row, "asset_key");
                var path = Str(row, "file_path");
                Assert.IsNotNull(key, "row without asset_key: " + path);
                Assert.IsTrue(AssetKey.TryParse(key!, out var parsed),
                    "unparseable key: " + key);
                var want = KeyGroupRule.Assign(parsed);
                Assert.IsNotNull(want, "key routes to no group: " + key);
                Assert.IsTrue(entries.ContainsKey(want!),
                    "group file missing: " + want);
                Assert.IsTrue(entries[want!].ContainsKey(key!),
                    "key not in " + want + ": " + key);
                var fileGuid = MetaGuid(path!);
                Assert.AreEqual(fileGuid, entries[want!][key!],
                    key + " entry GUID does not match .meta of " + path);
            }
        }

        private static string MetaGuid(string rel)
        {
            var metaPath = Abs(rel + ".meta");
            Assert.IsTrue(File.Exists(metaPath), "missing .meta: " + rel);
            var m = System.Text.RegularExpressions.Regex.Match(
                File.ReadAllText(metaPath), @"guid:\s*([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + rel + ".meta");
            return m.Groups[1].Value;
        }

        private static Dictionary<string, Dictionary<string, string>> ReadGroupEntries()
        {
            var map = new Dictionary<string, Dictionary<string, string>>(
                StringComparer.Ordinal);
            var dir = Abs(GroupsDir);
            foreach (var file in Directory.GetFiles(dir, "*.asset"))
            {
                var name = Path.GetFileNameWithoutExtension(file);
                var entries = new Dictionary<string, string>(StringComparer.Ordinal);
                var text = File.ReadAllText(file);
                var rx = new System.Text.RegularExpressions.Regex(
                    @"-\s+m_GUID:\s*([0-9a-f]{32})\s*\r?\n\s+m_Address:\s*(\S+)");
                foreach (System.Text.RegularExpressions.Match m
                    in rx.Matches(text))
                {
                    entries[m.Groups[2].Value] = m.Groups[1].Value;
                }
                map[name] = entries;
            }
            return map;
        }

        [Test]
        public void TestVfxSkillPrefabsCoverSkills()
        {
            // every catalog skill key in the fragment must have a .vfx
            // GameObject prefab that references a declared flipbook material
            var prefabDir = Abs("client/Assets/Art/VFX/Skills");
            var matDir = Abs("client/Assets/Art/VFX/Materials");
            Assert.IsTrue(Directory.Exists(prefabDir));
            var entries = ReadGroupEntries();
            int skills = 0;
            var skillKeys = new List<string>();
            foreach (var key in entries["shared.local"].Keys)
            {
                if (key.StartsWith("asset.skill.", StringComparison.Ordinal)
                    && key.EndsWith(".vfx", StringComparison.Ordinal))
                {
                    skillKeys.Add(key);
                }
            }
            skillKeys.Sort(StringComparer.Ordinal);
            foreach (var key in skillKeys)
            {
                skills++;
                var name = key.Substring("asset.".Length);
                name = name.Substring(0, name.Length - ".vfx".Length)
                    .Replace('.', '_');
                var prefab = Path.Combine(prefabDir, name + ".prefab");
                Assert.IsTrue(File.Exists(prefab), "missing prefab " + prefab);
                var text = File.ReadAllText(prefab);
                Assert.IsTrue(text.Contains("tilesX: 2") && text.Contains("tilesY: 2"),
                    prefab + " lacks 2x2 texture-sheet animation");
                Assert.IsTrue(entries["shared.local"].ContainsKey(key),
                    key + " missing from shared.local");
            }
            Assert.AreEqual(60, skills, "expected 60 skill vfx prefabs");
            // every material references a declared sheet texture guid
            var sheets = new HashSet<string>(StringComparer.Ordinal);
            foreach (var d in LoadImportJsons())
            {
                foreach (var kv in d.Assets)
                {
                    if (kv.Value.Get("flipbook") != null)
                    {
                        sheets.Add(MetaGuid(d.Dir + "/" + kv.Key));
                    }
                }
            }
            foreach (var mat in Directory.GetFiles(matDir, "*.mat"))
            {
                var text = File.ReadAllText(mat);
                var m = System.Text.RegularExpressions.Regex.Match(text,
                    @"m_Texture:\s*\{fileID:\s*2800000,\s*guid:\s*([0-9a-f]{32})");
                Assert.IsTrue(m.Success, mat + " no base texture");
                Assert.IsTrue(sheets.Contains(m.Groups[1].Value),
                    mat + " texture is not a declared flipbook sheet");
            }
        }

        [Test]
        public void TestGroupBudgetEstimates()
        {
            // BC7 = 8bpp: decoded w*h bytes approximates compressed size
            var entries = ReadGroupEntries();
            var bytes = new Dictionary<string, long>(StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                var key = Str(row, "asset_key")!;
                var path = Str(row, "file_path")!;
                if (!AssetKey.TryParse(key, out var parsed))
                {
                    continue;
                }
                var group = KeyGroupRule.Assign(parsed);
                if (group == null)
                {
                    continue;
                }
                long sz;
                if (path.EndsWith(".png", StringComparison.Ordinal))
                {
                    var dim = PngSize(Abs(path));
                    sz = (long)dim.Item1 * dim.Item2; // BC7 8bpp
                }
                else
                {
                    sz = new FileInfo(Abs(path)).Length;
                }
                bytes[group] = (bytes.TryGetValue(group, out var v) ? v : 0) + sz;
            }
            foreach (var kv in bytes)
            {
                Assert.IsTrue(GroupBudgets.Budgets.ContainsKey(kv.Key),
                    "unknown group " + kv.Key);
                var budget = GroupBudgets.Budgets[kv.Key].CompressedMaxBytes;
                Assert.LessOrEqual(kv.Value, budget,
                    kv.Key + " BC7 estimate " + kv.Value + " > budget " + budget);
            }
            Assert.IsTrue(bytes.ContainsKey("icons.shared"));
            Assert.IsTrue(bytes.ContainsKey("bootstrap.local"));
            Assert.IsTrue(bytes.ContainsKey("shared.local"));
        }

        [Test]
        public void TestStylePackPaletteCoverage()
        {
            var packDir = "client/Assets/Art/StyleRef/interface/interface_chibi";
            Assert.IsTrue(File.Exists(Abs(packDir + "/palette.json")),
                "palette.json missing");
            Assert.IsTrue(File.Exists(Abs(packDir + "/style.md")),
                "style.md missing");
            int anchors = 0;
            foreach (var row in Rows())
            {
                var path = Str(row, "file_path")!;
                if (path.StartsWith(packDir + "/anchor_", StringComparison.Ordinal)
                    && path.EndsWith(".png", StringComparison.Ordinal))
                {
                    anchors++;
                    Assert.AreEqual("interface/interface_chibi",
                        Str(row, "style_pack_id"), path + " wrong style_pack_id");
                }
            }
            Assert.GreaterOrEqual(anchors, 6, "style pack needs 6-10 anchors");
            Assert.LessOrEqual(anchors, 10);
            // palette.json parses and carries Lab entries
            var pal = RegisterJson.Parse(
                File.ReadAllText(Abs(packDir + "/palette.json")));
            Assert.AreEqual(RegisterJson.Node.Kind.Arr, pal.Type);
            Assert.Greater(pal.Arr!.Count, 0);
        }
    }

}
