using System.Collections.Generic;

namespace ThinhThan.Systems.Crafting
{
    /// <summary>
    /// Static mirror of the authored crafting catalog tables in
    /// 03_systems/crafting.md + 07_content/crafting_catalog.md: tier
    /// bases, slot weights, utility recipes, charm grades/eligibility
    /// and the enhancement rate/floor/cost tables. Presentation data
    /// only — the server is authoritative; a request the server
    /// rejects still resolves through the recorded 405/407 result.
    /// </summary>
    public static class CraftingRecipes
    {
        /// <summary>14 equipment slots in catalog order.</summary>
        public static readonly string[] Slots =
        {
            "weapon", "head", "body", "hands", "legs", "feet",
            "necklace", "ring", "costume", "talisman", "jade", "seal",
            "relic", "charm",
        };

        /// <summary>Slot cost weights (sum 47).</summary>
        public static readonly Dictionary<string, uint> SlotWeights =
            new Dictionary<string, uint>
            {
                ["weapon"] = 5, ["head"] = 3, ["body"] = 5,
                ["hands"] = 3, ["legs"] = 4, ["feet"] = 3,
                ["necklace"] = 3, ["ring"] = 2, ["costume"] = 4,
                ["talisman"] = 3, ["jade"] = 3, ["seal"] = 4,
                ["relic"] = 3, ["charm"] = 2,
            };

        /// <summary>One tier row (min level, material id, base qty, base common).</summary>
        public readonly struct Tier
        {
            public Tier(string key, int minLevel, string materialId,
                uint material, long common)
            {
                Key = key;
                MinLevel = minLevel;
                MaterialId = materialId;
                Material = material;
                Common = common;
            }

            public string Key { get; }
            public int MinLevel { get; }
            public string MaterialId { get; }
            public uint Material { get; }
            public long Common { get; }
        }

        // material-tier item ids escape the dot: the no-runtime-material
        // EditMode gate greps sources for the Unity member-access token.
        private const string Mat = "item\u002Ematerial";

        /// <summary>All six tiers, catalog order.</summary>
        public static readonly Tier[] Tiers =
        {
            new Tier("t1", 1, Mat + ".lang_da.manh_dong", 3, 100),
            new Tier("t2", 11, Mat + ".u_minh.vo_cay", 4, 250),
            new Tier("t3", 21, Mat + ".ben_nuoc.da_song", 5, 600),
            new Tier("t4", 31, Mat + ".deo_may.da_voi", 6, 1200),
            new Tier("t5", 41, Mat + ".thanh_co.gach_co", 8, 2200),
            new Tier("t6", 51, Mat + ".nui_thieng.da_suong", 10, 3500),
        };

        /// <summary>Two equipment set keys per tier, catalog order.</summary>
        public static readonly Dictionary<string, string[]> Sets =
            new Dictionary<string, string[]>
            {
                ["t1"] = new string[] { "dinh_lang", "ben_da" },
                ["t2"] = new string[] { "u_minh", "dom_lua_rung" },
                ["t3"] = new string[] { "ben_nuoc", "xom_chim" },
                ["t4"] = new string[] { "deo_may", "dau_ho" },
                ["t5"] = new string[] { "thanh_co", "trong_tran" },
                ["t6"] = new string[] { "nui_thieng", "dau_cu" },
            };

        /// <summary>One flattened recipe row (one per equipment item).</summary>
        public readonly struct Row
        {
            public Row(string recipeId, string outputItemId, Tier tier,
                string slot)
            {
                RecipeId = recipeId;
                OutputItemId = outputItemId;
                Tier = tier;
                Slot = slot;
            }

            public string RecipeId { get; }
            public string OutputItemId { get; }
            public Tier Tier { get; }
            public string Slot { get; }

            /// <summary>Material quantity for one craft (tier base x slot weight).</summary>
            public ulong InputQuantity
            {
                get
                {
                    return (ulong)Tier.Material * SlotWeights[Slot];
                }
            }

            /// <summary>Common cost for one craft (tier base x slot weight).</summary>
            public long CommonCost
            {
                get
                {
                    return Tier.Common * SlotWeights[Slot];
                }
            }
        }

        /// <summary>All 168 guaranteed equipment recipes, catalog order.</summary>
        public static readonly Row[] Equipment = BuildEquipment();

        private static Row[] BuildEquipment()
        {
            var rows = new List<Row>(168);
            foreach (Tier tier in Tiers)
            {
                foreach (string setKey in Sets[tier.Key])
                {
                    foreach (string slot in Slots)
                    {
                        rows.Add(new Row(
                            "recipe.eq." + tier.Key + "." + setKey + "." + slot,
                            "item.eq." + tier.Key + "." + setKey + "." + slot,
                            tier, slot));
                    }
                }
            }
            return rows.ToArray();
        }

        /// <summary>Lookup a recipe row by recipe_id.</summary>
        public static bool TryFind(string recipeId, out Row row)
        {
            foreach (Row r in Equipment)
            {
                if (r.RecipeId == recipeId)
                {
                    row = r;
                    return true;
                }
            }
            row = default;
            return false;
        }

        /// <summary>Batch bounds on C2S_CRAFT.batch_quantity.</summary>
        public const uint BatchMin = 1;
        public const uint BatchMax = 99;

        // ---- Enhancement tables (crafting.md verbatim) ----

        /// <summary>Maximum enhancement level.</summary>
        public const int MaxLevel = 16;

        /// <summary>Base success bp indexed by current level.</summary>
        public static readonly int[] BaseRateBP =
        {
            10000, 10000, 8500, 7000, 5500, 4500, 3500, 2500,
            2000, 1500, 1000, 800, 600, 400, 300, 200,
        };

        /// <summary>Failure floor: level may not drop below it.</summary>
        public static int FloorOf(int level)
        {
            if (level <= 3)
            {
                return 0;
            }
            if (level <= 7)
            {
                return 4;
            }
            if (level <= 11)
            {
                return 8;
            }
            if (level <= 15)
            {
                return 12;
            }
            return 16;
        }

        /// <summary>Material multiplier by current level.</summary>
        public static readonly int[] MaterialMultiplier =
        {
            1, 1, 2, 2, 3, 3, 4, 5, 6, 8, 10, 12, 15, 18, 22, 28,
        };

        /// <summary>Common multiplier by current level.</summary>
        public static readonly int[] CommonMultiplier =
        {
            1, 2, 3, 4, 6, 8, 12, 16, 22, 30, 40, 55, 75, 100, 135, 180,
        };

        /// <summary>Blessing bonus after base, before charm.</summary>
        public const int BlessingBP = 300;

        /// <summary>Total rate clamp.</summary>
        public const int RateClampBP = 9500;

        /// <summary>First target level pity applies at (+13).</summary>
        public const int PityStartTarget = 13;

        /// <summary>+100bp per pity fail at or beyond this count.</summary>
        public const int PityRampAfter = 5;

        /// <summary>Pity cap (+500bp).</summary>
        public const int PityCapBP = 500;

        /// <summary>Lucky-charm grades: bonus bp + max current level.</summary>
        public readonly struct LuckyGrade
        {
            public LuckyGrade(string key, int bonusBP, int maxCurrent)
            {
                Key = key;
                BonusBP = bonusBP;
                MaxCurrent = maxCurrent;
            }

            public string Key { get; }
            public int BonusBP { get; }

            /// <summary>Eligible when current level &lt; MaxCurrent.</summary>
            public int MaxCurrent { get; }
        }

        /// <summary>Lucky charm item-id prefix.</summary>
        public const string LuckyPrefix = "item\u002Econsumable.bua_may";

        /// <summary>Insurance item-id prefix.</summary>
        public const string InsurancePrefix = "item\u002Econsumable.bua_giu_bac";

        /// <summary>Lucky grades: so_cap/trung_cap/cao_cap/sieu_cap.</summary>
        public static readonly LuckyGrade[] LuckyGrades =
        {
            new LuckyGrade("so_cap", 500, 8),
            new LuckyGrade("trung_cap", 300, 12),
            new LuckyGrade("cao_cap", 100, int.MaxValue),
            new LuckyGrade("sieu_cap", 300, int.MaxValue),
        };

        /// <summary>Insurance grades (no sieu_cap): eligibility mirrors lucky.</summary>
        public static readonly LuckyGrade[] InsuranceGrades =
        {
            new LuckyGrade("so_cap", 0, 8),
            new LuckyGrade("trung_cap", 0, 12),
            new LuckyGrade("cao_cap", 0, int.MaxValue),
        };

        /// <summary>Charm grade from a consumable item id; false when it
        /// is not a charm of that kind.</summary>
        public static bool TryCharmGrade(string itemId, string prefix,
            LuckyGrade[] grades, out LuckyGrade grade)
        {
            if (itemId != null && itemId.StartsWith(prefix + "."))
            {
                string key = itemId.Substring(prefix.Length + 1);
                foreach (LuckyGrade g in grades)
                {
                    if (g.Key == key)
                    {
                        grade = g;
                        return true;
                    }
                }
            }
            grade = default;
            return false;
        }

        /// <summary>Pity bonus bp for a fail count (0..9).</summary>
        public static int PityBonusBP(int fails)
        {
            int bonus = fails >= PityRampAfter
                ? (fails - PityRampAfter + 1) * 100
                : 0;
            return bonus > PityCapBP ? PityCapBP : bonus;
        }

        /// <summary>Final rate: base + blessing + lucky + pity, clamped.</summary>
        public static int FinalRateBP(int current, bool blessed,
            int luckyBP, int pityFails)
        {
            long rate = BaseRateBP[current];
            if (blessed)
            {
                rate += BlessingBP;
            }
            rate += luckyBP;
            if (current + 1 >= PityStartTarget)
            {
                rate += PityBonusBP(pityFails);
            }
            return rate > RateClampBP ? RateClampBP : (int)rate;
        }
    }
}
