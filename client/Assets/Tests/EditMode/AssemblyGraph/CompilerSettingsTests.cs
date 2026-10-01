using System;
using System.IO;
using NUnit.Framework;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// Compiler settings contract (CODE-001): warnings are errors, nullable
    /// is enabled, and no source file suppresses warnings.
    /// </summary>
    public class CompilerSettingsTests
    {
        private static readonly string AssetsRoot = Application.dataPath;

        // The generated-only Protocol tree is exempt: CODE-004 mandates the
        // verbatim `#pragma warning disable` header on its .cs outputs
        // (engineering_conventions.md §2.7). Same exclusion the Go verifier
        // applies in style.generatedCSharpDir.
        private static readonly string GeneratedProtocolDir =
            Path.GetFullPath(Path.Combine(Application.dataPath, "Scripts", "Protocol"))
                + Path.DirectorySeparatorChar;

        [Test]
        public void TestCscRspWarnAsErrorNullable()
        {
            var asmdefs = Directory.GetFiles(AssetsRoot, "*.asmdef", SearchOption.AllDirectories);
            Assert.IsNotEmpty(asmdefs, "no asmdefs under Assets/");
            foreach (var asmdef in asmdefs)
            {
                var rsp = Path.Combine(Path.GetDirectoryName(asmdef) ?? ".", "csc.rsp");
                Assert.IsTrue(File.Exists(rsp), "missing sibling csc.rsp for " + asmdef);
                var text = File.ReadAllText(rsp);
                StringAssert.Contains("-warnaserror+", text, rsp);
                StringAssert.Contains("-nullable:enable", text, rsp);
            }
        }

        [Test]
        public void TestZeroCompilerWarnings()
        {
            // With -warnaserror+ the compile fails on any warning; the only
            // remaining escape is a suppression directive, which is banned.
            foreach (var cs in Directory.GetFiles(AssetsRoot, "*.cs", SearchOption.AllDirectories))
            {
                if (Path.GetFullPath(cs).StartsWith(GeneratedProtocolDir, StringComparison.Ordinal))
                {
                    continue;
                }
                var lines = File.ReadAllLines(cs);
                for (int i = 0; i < lines.Length; i++)
                {
                    // Match the directive/attribute itself, not the literal
                    // in this test's own source: a trimmed line that starts
                    // with the pragma or an attribute bracket.
                    var t = lines[i].TrimStart();
                    Assert.IsFalse(
                        t.StartsWith("#pragma warning disable", StringComparison.Ordinal),
                        cs + ":" + (i + 1));
                    Assert.IsFalse(
                        t.StartsWith("[", StringComparison.Ordinal) &&
                            t.Contains("SuppressMessage", StringComparison.Ordinal),
                        cs + ":" + (i + 1));
                }
            }
        }
    }
}
