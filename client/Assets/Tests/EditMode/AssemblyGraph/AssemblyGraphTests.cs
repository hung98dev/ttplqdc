using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using NUnit.Framework;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// Assembly-definition contract tests (repository_layout.md): the
    /// ThinhThan.* reference graph is acyclic, Protocol stays a leaf, all 13
    /// mandatory assemblies are declared, and the live graph matches the
    /// canonical layout table.
    /// </summary>
    public class AssemblyGraphTests
    {
        private const string AssetsRoot = "Assets";

        private static readonly Dictionary<string, string[]> ExpectedReferences =
            new Dictionary<string, string[]>
            {
                ["ThinhThan.Protocol"] = new string[] { },
                ["ThinhThan.Core"] = new[]
                {
                    "Unity.InputSystem",
                    "Unity.RenderPipelines.Core.Runtime",
                    "Unity.RenderPipelines.Universal.Runtime",
                    "Unity.RenderPipelines.Universal.2D.Runtime",
                },
                ["ThinhThan.Core.Assets"] = new[]
                {
                    "ThinhThan.Core",
                    "Unity.Addressables",
                    "Unity.ResourceManager",
                },
                ["ThinhThan.Core.Assets.Editor"] = new[]
                {
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "Unity.Addressables",
                    "Unity.Addressables.Editor",
                    "Unity.ResourceManager",
                },
                ["ThinhThan.Core.Localization"] = new[]
                {
                    "ThinhThan.Core",
                    "Unity.Localization",
                    "Unity.Addressables",
                    "Unity.ResourceManager",
                },
                ["ThinhThan.Core.Localization.Editor"] = new[]
                {
                    "ThinhThan.Core",
                    "ThinhThan.Core.Localization",
                    "Unity.Localization",
                    "Unity.Localization.Editor",
                },
                ["ThinhThan.Core.Geometry.Editor"] = new[]
                {
                    "ThinhThan.Core",
                },
                ["ThinhThan.Net"] = new[]
                {
                    "ThinhThan.Core",
                    "ThinhThan.Protocol",
                },
                ["ThinhThan.Systems"] = new[]
                {
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "ThinhThan.Core.Localization",
                    "ThinhThan.Net",
                    "ThinhThan.Protocol",
                    "Unity.InputSystem",
                    "Unity.RenderPipelines.Core.Runtime",
                    "Unity.RenderPipelines.Universal.Runtime",
                    "Unity.2D.Animation.Runtime",
                },
                ["ThinhThan.UI"] = new[]
                {
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "ThinhThan.Core.Localization",
                    "ThinhThan.Net",
                    "ThinhThan.Protocol",
                    "ThinhThan.Systems",
                    "Unity.InputSystem",
                    "UnityEngine.UI",
                    "Unity.TextMeshPro",
                },
                ["ThinhThan.App"] = new[]
                {
                    "ThinhThan.Protocol",
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "ThinhThan.Core.Localization",
                    "ThinhThan.Net",
                    "ThinhThan.Systems",
                    "ThinhThan.UI",
                    "Unity.InputSystem",
                    "Unity.RenderPipelines.Universal.Runtime",
                    "Unity.Addressables",
                    "Unity.ResourceManager",
                    "Unity.Localization",
                },
                ["ThinhThan.Tests.EditMode"] = new[]
                {
                    "ThinhThan.Protocol",
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "ThinhThan.Core.Assets.Editor",
                    "ThinhThan.Core.Localization",
                    "ThinhThan.Core.Localization.Editor",
                    "ThinhThan.Core.Geometry.Editor",
                    "ThinhThan.Net",
                    "ThinhThan.Systems",
                    "ThinhThan.UI",
                    "Unity.InputSystem",
                    "Unity.RenderPipelines.Core.Runtime",
                    "Unity.RenderPipelines.Universal.Runtime",
                    "Unity.RenderPipelines.Universal.2D.Runtime",
                    "Unity.Addressables",
                    "Unity.Addressables.Editor",
                    "Unity.ResourceManager",
                    "Unity.Localization",
                    "Unity.Localization.Editor",
                    "Unity.2D.Animation.Runtime",
                    "UnityEngine.UI",
                    "Unity.TextMeshPro",
                    "UnityEngine.TestRunner",
                    "UnityEditor.TestRunner",
                    "Unity.PerformanceTesting",
                },
                ["ThinhThan.Tests.PlayMode"] = new[]
                {
                    "ThinhThan.Protocol",
                    "ThinhThan.Core",
                    "ThinhThan.Core.Assets",
                    "ThinhThan.Core.Localization",
                    "ThinhThan.Net",
                    "ThinhThan.Systems",
                    "ThinhThan.UI",
                    "ThinhThan.App",
                    "Unity.InputSystem",
                    "Unity.RenderPipelines.Core.Runtime",
                    "Unity.RenderPipelines.Universal.Runtime",
                    "Unity.RenderPipelines.Universal.2D.Runtime",
                    "Unity.Addressables",
                    "Unity.ResourceManager",
                    "Unity.Localization",
                    "Unity.2D.Animation.Runtime",
                    "UnityEngine.UI",
                    "Unity.TextMeshPro",
                    "UnityEngine.TestRunner",
                    "Unity.PerformanceTesting",
                },
            };

        [Serializable]
        private sealed class AsmDefData
        {
            public string name;
            public string[] references = new string[0];
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

        private static string FindCycle(
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

        private static void AssertIsNull(object o)
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
            string[] expected =
            {
                "Assets/Scripts/Protocol/ThinhThan.Protocol.asmdef",
                "Assets/Scripts/Core/ThinhThan.Core.asmdef",
                "Assets/Scripts/Core/Assets/ThinhThan.Core.Assets.asmdef",
                "Assets/Scripts/Core/Assets/Editor/ThinhThan.Core.Assets.Editor.asmdef",
                "Assets/Scripts/Core/Localization/ThinhThan.Core.Localization.asmdef",
                "Assets/Scripts/Core/Localization/Editor/ThinhThan.Core.Localization.Editor.asmdef",
                "Assets/Scripts/Core/Geometry/Editor/ThinhThan.Core.Geometry.Editor.asmdef",
                "Assets/Scripts/Net/ThinhThan.Net.asmdef",
                "Assets/Scripts/Systems/ThinhThan.Systems.asmdef",
                "Assets/Scripts/UI/ThinhThan.UI.asmdef",
                "Assets/Scripts/App/ThinhThan.App.asmdef",
                "Assets/Tests/EditMode/ThinhThan.Tests.EditMode.asmdef",
                "Assets/Tests/PlayMode/ThinhThan.Tests.PlayMode.asmdef",
            };
            foreach (var rel in expected)
            {
                Assert.IsTrue(File.Exists(rel), "missing asmdef " + rel);
                var data = JsonUtility.FromJson<AsmDefData>(File.ReadAllText(rel));
                var want = Path.GetFileNameWithoutExtension(rel);
                Assert.AreEqual(want, data.name, rel + ": name mismatch");
            }
        }

        [Test]
        public void TestReferenceGraphMatchesLayout()
        {
            var map = LoadAll();
            var extra = map.Keys.Where(k => !ExpectedReferences.ContainsKey(k)).ToList();
            Assert.IsEmpty(extra, "unlisted asmdefs: " + string.Join(",", extra));
            foreach (var kv in ExpectedReferences.OrderBy(k => k.Key, StringComparer.Ordinal))
            {
                Assert.IsTrue(map.ContainsKey(kv.Key), "missing asmdef " + kv.Key);
                var got = (map[kv.Key].references ?? new string[0]).OrderBy(r => r, StringComparer.Ordinal).ToArray();
                var want = kv.Value.OrderBy(r => r, StringComparer.Ordinal).ToArray();
                Assert.AreEqual(
                    string.Join(",", want),
                    string.Join(",", got),
                    kv.Key + ": reference list differs from layout");
            }
        }
    }
}
