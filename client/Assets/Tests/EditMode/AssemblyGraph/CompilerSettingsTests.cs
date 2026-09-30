using System.IO;
using NUnit.Framework;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// Compiler settings contract (CODE-001): warnings are errors, nullable
    /// is enabled, and no source file suppresses warnings.
    /// </summary>
    public class CompilerSettingsTests
    {
        private const string AssetsRoot = "Assets";

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
                var lines = File.ReadAllLines(cs);
                for (int i = 0; i < lines.Length; i++)
                {
                    var line = lines[i];
                    StringAssert.DoesNotContain(
                        "#pragma warning disable",
                        line,
                        cs + ":" + (i + 1));
                    StringAssert.DoesNotContain(
                        "SuppressMessage",
                        line,
                        cs + ":" + (i + 1));
                }
            }
        }
    }
}
