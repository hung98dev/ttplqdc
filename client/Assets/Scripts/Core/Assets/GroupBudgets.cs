using System.Collections.Generic;

namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// Per-group compressed/RAM budgets and the deterministic RAM estimator of
    /// presentation_asset_manifest.md §1: RAM = texture format x size x mips +
    /// mesh + decompressed audio, computed from import settings — never
    /// measured from a process. Totals: resident steady &lt;= 450 MB, transfer
    /// peak &lt;= 570 MB, base install &lt;= 92 MB.
    /// </summary>
    public static class GroupBudgets
    {
        /// <summary>Texture/compression formats the estimator prices.</summary>
        public enum TexFormat
        {
            Rgba32 = 0,
            Rgba16 = 1,
            Dxt5 = 2,
            Bc7 = 3,
            Astc4X4 = 4,
            Astc6X6 = 5,
            Astc8X8 = 6,
            Astc10X10 = 7,
            Astc12X12 = 8,
            Etc2Rgba8 = 9,
        }

        /// <summary>One canonical group's download + RAM budgets.</summary>
        public sealed class GroupBudget
        {
            public string Group
            {
                get;
            }
            public long CompressedMaxBytes
            {
                get;
            }
            public long RamMaxBytes
            {
                get;
            }
            public AddressableGroups.GroupKind Kind
            {
                get;
            }

            public GroupBudget(string group, long compressedMaxBytes, long ramMaxBytes)
            {
                Group = group;
                CompressedMaxBytes = compressedMaxBytes;
                RamMaxBytes = ramMaxBytes;
                Kind = AddressableGroups.KindOf(group);
            }
        }

        private const long Mib = 1024L * 1024L;

        public const long ResidentSteadyMaxBytes = 450L * Mib;
        public const long TransferPeakMaxBytes = 570L * Mib;
        public const long BaseInstallMaxBytes = 92L * Mib;

        private static readonly Dictionary<string, GroupBudget> _budgets = BuildBudgets();

        private static Dictionary<string, GroupBudget> BuildBudgets()
        {
            var budgets = new Dictionary<string, GroupBudget>();
            Add(budgets, AddressableGroups.BootstrapLocal, 12, 24);
            Add(budgets, AddressableGroups.SharedLocal, 70, 110);
            Add(budgets, AddressableGroups.IconsShared, 20, 32);
            Add(budgets, AddressableGroups.BeastShared, 20, 36);
            Add(budgets, AddressableGroups.CosmeticShared, 60, 48);
            Add(budgets, AddressableGroups.PvpShared, 25, 60);
            Add(budgets, AddressableGroups.AudioBgmShared, 15, 4);
            foreach (var zone in AddressableGroups.ZoneKeys)
            {
                Add(budgets, "audio.bgm." + zone, 15, 4);
                Add(budgets, "region." + zone, 60, 120);
            }
            foreach (var dungeon in AddressableGroups.DungeonKeys)
            {
                Add(budgets, "dungeon." + dungeon, 25, 60);
            }
            Add(budgets, AddressableGroups.DungeonFinale, 25, 60);
            Add(budgets, AddressableGroups.LocalizationLocales, 1, 2);
            Add(budgets, AddressableGroups.LocalizationShared, 1, 4);
            Add(budgets, AddressableGroups.LocalizationStringsViVn, 4, 8);
            Add(budgets, AddressableGroups.LocalizationStringsEnUs, 4, 8);
            return budgets;
        }

        private static void Add(Dictionary<string, GroupBudget> budgets, string group, long compressedMib, long ramMib)
        {
            budgets[group] = new GroupBudget(group, compressedMib * Mib, ramMib * Mib);
        }

        /// <summary>Budget table covering every canonical group.</summary>
        public static IReadOnlyDictionary<string, GroupBudget> Budgets
        {
            get
            {
                return _budgets;
            }
        }

        /// <summary>
        /// Texture RAM in bytes for one mip level chain: format x size x mips.
        /// Block formats round each level up to whole blocks (16 bytes/block);
        /// uncompressed formats use bytes-per-pixel. mipCount 0/1 = base level.
        /// </summary>
        public static long EstimateTextureBytes(int width, int height, TexFormat format, int mipCount)
        {
            if (width <= 0 || height <= 0 || mipCount < 0)
            {
                return 0;
            }
            var levels = mipCount <= 0 ? 1 : mipCount;
            long total = 0;
            for (var level = 0; level < levels; level++)
            {
                var w = width >> level;
                var h = height >> level;
                if (w < 1)
                {
                    w = 1;
                }
                if (h < 1)
                {
                    h = 1;
                }
                total += LevelBytes(w, h, format);
            }
            return total;
        }

        private static long LevelBytes(int width, int height, TexFormat format)
        {
            switch (format)
            {
                case TexFormat.Rgba32:
                    return (long)width * height * 4L;
                case TexFormat.Rgba16:
                    return (long)width * height * 2L;
                case TexFormat.Astc6X6:
                    return Blocks(width, height, 6) * 16L;
                case TexFormat.Astc8X8:
                    return Blocks(width, height, 8) * 16L;
                case TexFormat.Astc10X10:
                    return Blocks(width, height, 10) * 16L;
                case TexFormat.Astc12X12:
                    return Blocks(width, height, 12) * 16L;
                case TexFormat.Dxt5:
                case TexFormat.Bc7:
                case TexFormat.Astc4X4:
                case TexFormat.Etc2Rgba8:
                default:
                    return Blocks(width, height, 4) * 16L;
            }
        }

        private static long Blocks(int width, int height, int dim)
        {
            return ((long)(width + dim - 1) / dim) * ((long)(height + dim - 1) / dim);
        }

        /// <summary>Mesh RAM in bytes: vertex buffer + index buffer.</summary>
        public static long EstimateMeshBytes(int vertexCount, int vertexStrideBytes, int indexCount, int indexStrideBytes)
        {
            if (vertexCount <= 0 && indexCount <= 0)
            {
                return 0;
            }
            return (long)vertexCount * vertexStrideBytes + (long)indexCount * indexStrideBytes;
        }

        /// <summary>Decompressed PCM16 audio RAM in bytes.</summary>
        public static long EstimateAudioBytes(double seconds, int channels, int sampleRate)
        {
            if (seconds <= 0 || channels <= 0 || sampleRate <= 0)
            {
                return 0;
            }
            return (long)(seconds * sampleRate) * channels * 2L;
        }

        /// <summary>
        /// Worst-case resident steady RAM (manifest §1): every in-player group
        /// plus icons/beast/cosmetic plus the single largest region/dungeon/pvp
        /// group plus one zone BGM stream and the shared BGM stream.
        /// </summary>
        public static long ResidentSteadyWorstCaseBytes()
        {
            long total = 0;
            foreach (var kv in _budgets)
            {
                if (kv.Value.Kind == AddressableGroups.GroupKind.Local)
                {
                    total += kv.Value.RamMaxBytes;
                }
            }
            total += _budgets[AddressableGroups.IconsShared].RamMaxBytes;
            total += _budgets[AddressableGroups.BeastShared].RamMaxBytes;
            total += _budgets[AddressableGroups.CosmeticShared].RamMaxBytes;
            total += LargestGroupRam();
            total += _budgets["audio.bgm." + AddressableGroups.ZoneKeys[0]].RamMaxBytes;
            total += _budgets[AddressableGroups.AudioBgmShared].RamMaxBytes;
            return total;
        }

        /// <summary>
        /// Worst-case transfer peak (manifest §1): resident steady while the
        /// destination group and its zone BGM stream load before the source
        /// region releases.
        /// </summary>
        public static long TransferPeakWorstCaseBytes()
        {
            return ResidentSteadyWorstCaseBytes()
                + LargestGroupRam()
                + _budgets["audio.bgm." + AddressableGroups.ZoneKeys[0]].RamMaxBytes;
        }

        /// <summary>Compressed bytes shipped inside the player (manifest §1).</summary>
        public static long BaseInstallCompressedBytes()
        {
            long total = 0;
            foreach (var kv in _budgets)
            {
                if (kv.Value.Kind == AddressableGroups.GroupKind.Local)
                {
                    total += kv.Value.CompressedMaxBytes;
                }
            }
            return total;
        }

        private static long LargestGroupRam()
        {
            long max = 0;
            foreach (var kv in _budgets)
            {
                if (kv.Value.RamMaxBytes > max)
                {
                    max = kv.Value.RamMaxBytes;
                }
            }
            return max;
        }
    }
}
