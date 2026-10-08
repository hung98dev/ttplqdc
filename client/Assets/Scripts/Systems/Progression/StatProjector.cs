using System.Collections.Generic;

namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// Client mirror of the stats.md pipeline for PREVIEW only —
    /// allocation/respec tooltips and the panel's "after" column. It is
    /// never authoritative: server commits remain the truth and arrive
    /// via 515. Mirrors sim/progression/stats.go exactly; a spec change
    /// there must land here through the contract-change rule.
    /// </summary>
    public static class StatProjector
    {
        /// <summary>The 21 final stats of stats.md, same order as the
        /// server enum.</summary>
        public enum Stat
        {
            MaxHP,
            MaxMP,
            Attack,
            Defense,
            HPRegen,
            MPRegen,
            CritChance,
            CritDamage,
            DamageBonus,
            DamageReduction,
            DodgeChance,
            Accuracy,
            MoveSpeed,
            AttackSpeed,
            CastSpeed,
            CooldownReduction,
            Lifesteal,
            HealReduction,
            Reflect,
            Absorb,
            HealingReceived,
            Count,
        }

        /// <summary>One pipeline contribution.</summary>
        public readonly struct Modifier
        {
            public readonly Stat Stat;
            public readonly double Value;

            public Modifier(Stat stat, double value)
            {
                Stat = stat;
                Value = value;
            }
        }

        private const int StatCount = (int)Stat.Count;

        // stats.md § Level 1 base table.
        private static readonly double[] LevelOneBase =
        {
            500.0, 200.0, 40.0, 20.0, 2.0, 3.0, 0.05, 1.50,
            0.0, 0.0, 0.03, 0.0, 1.00, 0.0, 0.0, 0.0,
            0.0, 0.0, 0.0, 0.0, 1.00,
        };

        // stats.md § Class Growth — {HP, MP, ATK, DEF, HP_REGEN, MP_REGEN}
        // per level; the regen columns are identical for every class.
        private static double[] ClassGrowth(string? classId)
        {
            switch (classId)
            {
                case "class.kim":
                    return new[] { 32.0, 8.0, 5.5, 2.0, 0.12, 0.10 };
                case "class.moc":
                    return new[] { 36.0, 12.0, 4.8, 2.2, 0.12, 0.10 };
                case "class.thuy":
                    return new[] { 32.0, 12.0, 5.0, 1.9, 0.12, 0.10 };
                case "class.hoa":
                    return new[] { 30.0, 14.0, 5.5, 1.8, 0.12, 0.10 };
                case "class.tho":
                    return new[] { 44.0, 8.0, 4.4, 3.0, 0.12, 0.10 };
                default:
                    return new double[6];
            }
        }

        // stats.md CLAMP table — {min, max}; stats not listed are
        // unclamped.
        private static readonly Dictionary<Stat, (double Min, double Max)> Caps =
            new Dictionary<Stat, (double, double)>
            {
                { Stat.CritChance, (0.0, 0.60) },
                { Stat.CritDamage, (0.0, 2.50) },
                { Stat.DodgeChance, (0.0, 0.40) },
                { Stat.Accuracy, (0.0, 0.40) },
                { Stat.DamageReduction, (0.0, 0.40) },
                { Stat.Lifesteal, (0.0, 0.08) },
                { Stat.HealReduction, (0.0, 0.30) },
                { Stat.Reflect, (0.0, 0.15) },
                { Stat.Absorb, (0.0, 0.10) },
                { Stat.AttackSpeed, (0.0, 0.50) },
                { Stat.CastSpeed, (0.0, 0.50) },
                { Stat.CooldownReduction, (0.0, 0.35) },
                { Stat.MoveSpeed, (0.40, 1.50) },
            };

        /// <summary>Level-adjusted base: level-1 base + class growth ×
        /// (level-1). Unknown classes keep the level-1 base.</summary>
        public static double[] ComputeBase(string? classId, int level)
        {
            var out_ = (double[])LevelOneBase.Clone();
            if (level <= 1)
            {
                return out_;
            }
            double[] g = ClassGrowth(classId);
            double n = level - 1;
            out_[(int)Stat.MaxHP] += g[0] * n;
            out_[(int)Stat.MaxMP] += g[1] * n;
            out_[(int)Stat.Attack] += g[2] * n;
            out_[(int)Stat.Defense] += g[3] * n;
            out_[(int)Stat.HPRegen] += g[4] * n;
            out_[(int)Stat.MPRegen] += g[5] * n;
            return out_;
        }

        /// <summary>stats.md § Potential Stats — flat contributions of a
        /// candidate allocation (all FLAT_ADD here).</summary>
        public static List<Modifier> PotentialModifiers(
            string? classId, int str, int vit, int int_, int agi)
        {
            double strPer = 0.25;
            double intPer = 0.25;
            if (classId == "class.kim" || classId == "class.tho")
            {
                strPer = 0.75;
            }
            else if (classId == "class.moc" || classId == "class.thuy" ||
                classId == "class.hoa")
            {
                intPer = 0.75;
            }
            var mods = new List<Modifier>(9)
            {
                new Modifier(Stat.Attack, strPer * str),
                new Modifier(Stat.Attack, intPer * int_),
                new Modifier(Stat.MaxMP, 1.0 * int_),
                new Modifier(Stat.MaxHP, 6.0 * vit),
                new Modifier(Stat.Defense, 0.20 * vit),
                new Modifier(Stat.CritChance, 0.0005 * agi),
                new Modifier(Stat.DodgeChance, 0.0004 * agi),
                new Modifier(Stat.CooldownReduction, 0.0002 * agi),
            };
            double ms = 0.0008 * agi;
            if (ms > 0.15)
            {
                ms = 0.15; // stats.md § MOVE_SPEED AGI-alone sub-cap.
            }
            mods.Add(new Modifier(Stat.MoveSpeed, ms));
            return mods;
        }

        /// <summary>Canonical order: BASE → FLAT_ADD → PERCENT_ADD →
        /// FINAL_MULTIPLY → per-stat CLAMP → floor on integral stats.</summary>
        public static double[] ComputeFinal(
            double[] baseStats, IReadOnlyList<Modifier> flat,
            bool pvp = false)
        {
            var flatSum = new double[StatCount];
            foreach (Modifier m in flat)
            {
                flatSum[(int)m.Stat] += m.Value;
            }
            var out_ = new double[StatCount];
            for (int s = 0; s < StatCount; s++)
            {
                double v = baseStats[s] + flatSum[s];
                if (Caps.TryGetValue((Stat)s, out (double Min, double Max) c))
                {
                    // PvP overrides (stats.md § PvP Caps).
                    if (pvp)
                    {
                        c = s switch
                        {
                            (int)Stat.Lifesteal => (0.0, 0.05),
                            (int)Stat.HealReduction => (0.0, 0.25),
                            (int)Stat.Reflect => (0.0, 0.08),
                            (int)Stat.Absorb => (0.0, 0.06),
                            _ => c,
                        };
                    }
                    if (v < c.Min)
                    {
                        v = c.Min;
                    }
                    else if (v > c.Max)
                    {
                        v = c.Max;
                    }
                }
                out_[s] = v;
            }
            // Integral stats floor at the end (stats.md).
            out_[(int)Stat.MaxHP] = System.Math.Floor(out_[(int)Stat.MaxHP]);
            out_[(int)Stat.MaxMP] = System.Math.Floor(out_[(int)Stat.MaxMP]);
            out_[(int)Stat.Attack] = System.Math.Floor(out_[(int)Stat.Attack]);
            out_[(int)Stat.Defense] = System.Math.Floor(out_[(int)Stat.Defense]);
            out_[(int)Stat.HPRegen] = System.Math.Floor(out_[(int)Stat.HPRegen]);
            out_[(int)Stat.MPRegen] = System.Math.Floor(out_[(int)Stat.MPRegen]);
            return out_;
        }

        /// <summary>Preview helper: current allocation + candidate deltas
        /// resolved to final stats for the tooltip.</summary>
        public static double[] Preview(ProgressionState state,
            int addStr, int addVit, int addInt, int addAgi)
        {
            int totalStr = state.PotentialStr + addStr;
            int totalVit = state.PotentialVit + addVit;
            int totalInt = state.PotentialInt + addInt;
            int totalAgi = state.PotentialAgi + addAgi;
            double[] baseStats = ComputeBase(state.ClassId, state.Level);
            List<Modifier> mods = PotentialModifiers(
                state.ClassId, totalStr, totalVit, totalInt, totalAgi);
            return ComputeFinal(baseStats, mods);
        }
    }
}
