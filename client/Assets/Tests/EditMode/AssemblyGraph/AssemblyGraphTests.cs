using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using NUnit.Framework;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// Assembly-definition contract tests (repository_layout.md § Mandatory
    /// Assemblies): the ThinhThan.* reference graph is acyclic, Protocol stays
    /// a leaf, all 13 mandatory assemblies are declared, and the live graph
    /// matches the canonical layout table — parsed from the spec itself,
    /// including its "every …" shorthand rows.
    /// </summary>
    public class AssemblyGraphTests
    {
        private static readonly string AssetsRoot = Application.dataPath;

        [Serializable]
        private sealed class AsmDefData
        {
            public string name = string.Empty;
            public string[]? references;
        }

        private sealed class LayoutRow
        {
            public string Name = string.Empty;
            public bool Editor;
            public string RefsCell = string.Empty;
        }

        private static string RepoRoot()
        {
            var dir = new DirectoryInfo(Application.dataPath);
            while (dir != null && !Directory.Exists(Path.Combine(dir.FullName, "docs")))
            {
                dir = dir.Parent;
            }
            Assert.IsNotNull(dir, "repository root (dir containing docs/) not found above " + Application.dataPath);
            return dir!.FullName;
        }

        /// <summary>
        /// Parse the § Mandatory Assemblies table of repository_layout.md into
        /// ordered rows: `| `Name` | `Folder/` | platform | refs |`.
        /// </summary>
        private static List<LayoutRow> ParseLayoutTable()
        {
            var path = Path.Combine(RepoRoot(), "docs", "10_implementation", "repository_layout.md");
            Assert.IsTrue(File.Exists(path), "missing " + path);
            var text = File.ReadAllText(path);
            var start = text.IndexOf("## Mandatory Assemblies", StringComparison.Ordinal);
            Assert.GreaterOrEqual(start, 0, "Mandatory Assemblies section not found");
            var end = text.IndexOf("\n## ", start + 1, StringComparison.Ordinal);
            var section = end < 0 ? text.Substring(start) : text.Substring(start, end - start);
            var rows = new List<LayoutRow>();
            foreach (var line in section.Split('\n'))
            {
                var t = line.Trim();
                if (!t.StartsWith("| `ThinhThan.", StringComparison.Ordinal))
                {
                    continue;
                }
                var cells = t.Split('|').Select(c => c.Trim()).Where(c => c.Length > 0).ToArray();
                Assert.GreaterOrEqual(cells.Length, 4, "malformed layout row: " + t);
                rows.Add(new LayoutRow
                {
                    Name = StripTicks(cells[0]),
                    Editor = cells[2].IndexOf("Editor", StringComparison.Ordinal) >= 0,
                    RefsCell = cells[3],
                });
            }
            Assert.AreEqual(13, rows.Count, "layout must declare exactly 13 mandatory assemblies");
            return rows;
        }

        private static string StripTicks(string s)
        {
            return s.Replace("`", "").Trim();
        }

        private static bool IsThinhThan(string name)
        {
            return name.StartsWith("ThinhThan.", StringComparison.Ordinal);
        }

        /// <summary>
        /// Literal assembly names in a refs cell: backticked tokens that are
        /// not "precompiled" DLL entries and not an "every …" shorthand.
        /// </summary>
        private static IEnumerable<string> LiteralRefs(string refsCell)
        {
            foreach (var raw in refsCell.Split(',', ';'))
            {
                var tok = StripTicks(raw);
                if (tok.Length == 0
                    || tok.StartsWith("precompiled", StringComparison.Ordinal)
                    || tok.EndsWith(".dll", StringComparison.Ordinal)
                    || tok.StartsWith("every", StringComparison.Ordinal)
                    || tok == "only")
                {
                    continue;
                }
                yield return tok;
            }
        }

        /// <summary>
        /// Expand a refs cell: literal names plus the "every …" shorthand rows
        /// (except-clauses, "assembly above" scopes, Editor filtering), with
        /// self-reference always excluded.
        /// </summary>
        private static List<string> ExpandRefs(
            LayoutRow row,
            List<LayoutRow> allRows,
            List<LayoutRow> rowsAbove,
            HashSet<string> pkgsAbove)
        {
            var outp = new List<string>();
            var cell = row.RefsCell;
            var norm = StripTicks(cell);
            var thin = allRows.Where(r => IsThinhThan(r.Name));
            var thinAbove = rowsAbove.Where(r => IsThinhThan(r.Name));

            if (norm.IndexOf("every non-Editor ThinhThan.* assembly", StringComparison.Ordinal) >= 0)
            {
                var scope = norm.IndexOf("assembly above", StringComparison.Ordinal) >= 0 ? thinAbove : thin;
                outp.AddRange(scope.Where(r => !r.Editor).Select(r => r.Name));
            }
            else if (norm.IndexOf("every ThinhThan.* assembly", StringComparison.Ordinal) >= 0)
            {
                var excl = new HashSet<string>();
                var idx = norm.IndexOf("except ", StringComparison.Ordinal);
                if (idx >= 0)
                {
                    var tail = norm.Substring(idx + "except ".Length);
                    var cut = tail.IndexOf(',');
                    if (cut >= 0)
                    {
                        tail = tail.Substring(0, cut);
                    }
                    foreach (var ex in tail.Split(new[] { " and " }, StringSplitOptions.RemoveEmptyEntries))
                    {
                        excl.Add(ex.Trim());
                    }
                }
                outp.AddRange(thin.Where(r => !excl.Contains(r.Name)).Select(r => r.Name));
            }
            if (norm.IndexOf("Unity package assembly listed above", StringComparison.Ordinal) >= 0)
            {
                var nonEditor = norm.IndexOf("every non-Editor Unity package assembly", StringComparison.Ordinal) >= 0;
                outp.AddRange(pkgsAbove.Where(p => !nonEditor
                    || (!p.EndsWith(".Editor", StringComparison.Ordinal)
                        && !p.StartsWith("UnityEditor.", StringComparison.Ordinal))));
            }
            outp.AddRange(LiteralRefs(cell));
            return outp.Where(r => r != row.Name).Distinct().ToList();
        }

        /// <summary>
        /// The canonical reference map derived from repository_layout.md:
        /// literals plus expanded "every …" rows; Unity package names are
        /// collected row-by-row as they first appear.
        /// </summary>
        private static Dictionary<string, string[]> ExpectedReferences()
        {
            var rows = ParseLayoutTable();
            var map = new Dictionary<string, string[]>();
            var rowsAbove = new List<LayoutRow>();
            var pkgsAbove = new HashSet<string>();
            foreach (var row in rows)
            {
                map[row.Name] = ExpandRefs(row, rows, rowsAbove, pkgsAbove)
                    .OrderBy(r => r, StringComparer.Ordinal).ToArray();
                foreach (var lit in LiteralRefs(row.RefsCell))
                {
                    if (!IsThinhThan(lit))
                    {
                        pkgsAbove.Add(lit);
                    }
                }
                rowsAbove.Add(row);
            }
            return map;
        }

        private static Dictionary<string, AsmDefData> LoadAll()
        {
            var map = new Dictionary<string, AsmDefData>();
            foreach (var path in Directory.GetFiles(AssetsRoot, "*.asmdef", SearchOption.AllDirectories))
            {
                var data = JsonUtility.FromJson<AsmDefData>(File.ReadAllText(path));
                Assert.IsNotNull(data, path + ": unparseable asmdef");
                Assert.IsFalse(map.ContainsKey(data.name), "duplicate asmdef " + data.name);
                map[data.name] = data;
            }
            return map;
        }

        [Test]
        public void TestAsmdefReferencesAcyclic()
        {
            var map = LoadAll();
            var visiting = new HashSet<string>();
            var done = new HashSet<string>();
            foreach (var name in map.Keys.OrderBy(x => x, StringComparer.Ordinal))
            {
                AssertIsNull(FindCycle(name, map, visiting, done));
            }
        }

        private static string? FindCycle(
            string name,
            Dictionary<string, AsmDefData> map,
            HashSet<string> visiting,
            HashSet<string> done)
        {
            if (done.Contains(name))
            {
                return null;
            }
            if (!visiting.Add(name))
            {
                return name;
            }
            if (map.TryGetValue(name, out var data))
            {
                foreach (var r in data.references ?? new string[0])
                {
                    if (r.StartsWith("ThinhThan.", StringComparison.Ordinal))
                    {
                        var cyc = FindCycle(r, map, visiting, done);
                        if (cyc != null)
                        {
                            return name + " -> " + cyc;
                        }
                    }
                }
            }
            visiting.Remove(name);
            done.Add(name);
            return null;
        }

        private static void AssertIsNull(object? o)
        {
            Assert.IsNull(o);
        }

        [Test]
        public void TestProtocolReferencesNoProjectAssembly()
        {
            var map = LoadAll();
            Assert.IsTrue(map.ContainsKey("ThinhThan.Protocol"), "ThinhThan.Protocol asmdef missing");
            foreach (var r in map["ThinhThan.Protocol"].references ?? new string[0])
            {
                Assert.IsFalse(
                    r.StartsWith("ThinhThan.", StringComparison.Ordinal),
                    "Protocol references project assembly " + r);
            }
        }

        [Test]
        public void TestMandatoryAssembliesDeclared()
        {
            foreach (var row in ParseLayoutTable())
            {
                var rel = row.Name + ".asmdef";
                var hits = Directory.GetFiles(AssetsRoot, rel, SearchOption.AllDirectories);
                Assert.AreEqual(1, hits.Length, "expected exactly one " + rel + " under Assets/");
                var data = JsonUtility.FromJson<AsmDefData>(File.ReadAllText(hits[0]));
                Assert.AreEqual(row.Name, data.name, hits[0] + ": name mismatch");
            }
        }

        [Test]
        public void TestReferenceGraphMatchesLayout()
        {
            var map = LoadAll();
            var expected = ExpectedReferences();
            var extra = map.Keys.Where(k => !expected.ContainsKey(k)).ToList();
            Assert.IsEmpty(extra, "unlisted asmdefs: " + string.Join(",", extra));
            foreach (var kv in expected.OrderBy(k => k.Key, StringComparer.Ordinal))
            {
                Assert.IsTrue(map.ContainsKey(kv.Key), "missing asmdef " + kv.Key);
                var got = (map[kv.Key].references ?? new string[0]).OrderBy(r => r, StringComparer.Ordinal).ToArray();
                Assert.AreEqual(
                    string.Join(",", kv.Value),
                    string.Join(",", got),
                    kv.Key + ": reference list differs from layout");
            }
        }
    }
}
