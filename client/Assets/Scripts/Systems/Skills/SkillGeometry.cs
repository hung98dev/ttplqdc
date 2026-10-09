using System.Collections.Generic;

namespace ThinhThan.Systems.Skills
{
    /// <summary>
    /// Client mirror of the resolved skill geometry (IMP-015):
    /// SKILL_ORIGIN_Y = caster anchor + 0.9m, int-mm world space.
    /// Telegraph primitives derive from this resolved geometry only —
    /// the client never derives reach from sprite bounds and never
    /// changes the authoritative value.
    /// </summary>
    public static class SkillGeometry
    {
        /// <summary>SKILL_ORIGIN_Y offset in millimeters.</summary>
        public const int OriginYOffsetMM = 900;

        /// <summary>Resolved-geometry kinds, mirroring the catalog.</summary>
        public enum ShapeKind
        {
            Self,
            MeleeBox,
            DirectionBox,
            Projectile,
            AreaSelf,
            AreaPosition,
            SingleTargetRange,
            DashLine,
            MoveLine,
            MoveContactLine,
            BarrierPosition,
        }

        /// <summary>Authored geometry parameters (mm / ms).</summary>
        public struct Spec
        {
            public ShapeKind Kind;

            /// <summary>Reach / cast range / range / distance / cast (primary extent, mm).</summary>
            public int Extent;

            /// <summary>Half-height / area radius / hit radius (secondary extent, mm).</summary>
            public int Extent2;

            /// <summary>Projectile speed mm/s or barrier height mm.</summary>
            public int Extent3;

            /// <summary>Line duration ms / barrier duration ms.</summary>
            public int DurationMs;

            /// <summary>Barrier thickness mm.</summary>
            public int ThicknessMM;
        }

        /// <summary>World-space resolved shape (mm).</summary>
        public struct Shape
        {
            public ShapeKind Kind;
            public int MinX;
            public int MaxX;
            public int MinY;
            public int MaxY;
            public int CenterX;
            public int CenterY;
            public int RadiusMM;
            public int PathMinX;
            public int PathMaxX;
        }

        // Compiled launch catalog — mirrors
        // server/internal/sim/skills/registry.go for telegraph
        // presentation only (no authority).
        private static readonly Dictionary<string, Spec> Specs =
            new Dictionary<string, Spec>
            {
                { "skill.kim.basic.kiem_thuc", S(ShapeKind.MeleeBox, 1900, 900) },
                { "skill.kim.basic.truy_phong_kiem", L(ShapeKind.DashLine, 2400, 180, 900) },
                { "skill.kim.basic.pha_khong_kiem", S(ShapeKind.DirectionBox, 3200, 900) },
                { "skill.kim.basic.vo_song_kiem", S(ShapeKind.MeleeBox, 2400, 1000) },
                { "skill.kim.active.xuyen_phong", L(ShapeKind.DashLine, 4200, 300, 900) },
                { "skill.kim.active.hoi_kiem", S(ShapeKind.AreaSelf, 2800, 0) },
                { "skill.kim.active.pha_giap", S(ShapeKind.MeleeBox, 2500, 900) },
                { "skill.kim.active.kiem_tran", S(ShapeKind.AreaSelf, 3200, 0) },
                { "skill.kim.active.nhat_kiem_dinh_hon", S(ShapeKind.SingleTargetRange, 2600, 0) },
                { "skill.moc.basic.linh_diep", P(7500, 11000, 250) },
                { "skill.moc.basic.thao_kich", P(8000, 11500, 250) },
                { "skill.moc.basic.truc_phi_tieu", P(8500, 12500, 220) },
                { "skill.moc.basic.co_thu_kich", P(8000, 10500, 300) },
                { "skill.moc.active.moc_bo", S(ShapeKind.AreaPosition, 7000, 2500) },
                { "skill.moc.active.hoi_xuan", S(ShapeKind.SingleTargetRange, 7500, 0) },
                { "skill.moc.active.thanh_dang", S(ShapeKind.AreaPosition, 7000, 3000) },
                { "skill.moc.active.van_doc", S(ShapeKind.AreaPosition, 7500, 2800) },
                { "skill.moc.active.van_moc_hoi_sinh", S(ShapeKind.AreaPosition, 7000, 3500) },
                { "skill.thuy.basic.thuy_tien", P(8000, 12000, 230) },
                { "skill.thuy.basic.bang_phien", P(8200, 12500, 240) },
                { "skill.thuy.basic.am_luu", P(7500, 10000, 300) },
                { "skill.thuy.basic.huyen_bang_kich", P(8500, 13000, 250) },
                { "skill.thuy.active.luu_bo", L(ShapeKind.MoveContactLine, 4500, 280, 1000) },
                { "skill.thuy.active.trieu_quyen", S(ShapeKind.AreaPosition, 7000, 2600) },
                { "skill.thuy.active.thuy_kinh", S(ShapeKind.Self, 0, 0) },
                { "skill.thuy.active.han_trieu", S(ShapeKind.DirectionBox, 5500, 1400) },
                { "skill.thuy.active.thien_ha", S(ShapeKind.AreaPosition, 7500, 3500) },
                { "skill.hoa.basic.hoa_phu", P(7500, 11000, 250) },
                { "skill.hoa.basic.viem_dan", P(7800, 11000, 280) },
                { "skill.hoa.basic.hoa_xa", P(8200, 13000, 220) },
                { "skill.hoa.basic.lua_tao_quan", P(8000, 12000, 260) },
                { "skill.hoa.active.boc_bo", L(ShapeKind.DashLine, 4200, 300, 1000) },
                { "skill.hoa.active.lien_bao", S(ShapeKind.AreaPosition, 7000, 2800) },
                { "skill.hoa.active.hoa_giap", S(ShapeKind.Self, 0, 0) },
                { "skill.hoa.active.hoa_vuc", S(ShapeKind.AreaPosition, 7000, 3000) },
                { "skill.hoa.active.cuu_hoa_lien", S(ShapeKind.AreaPosition, 7500, 3500) },
                { "skill.tho.basic.tran_quyen", S(ShapeKind.MeleeBox, 2000, 1000) },
                { "skill.tho.basic.pha_thach_kich", S(ShapeKind.MeleeBox, 2200, 1000) },
                { "skill.tho.basic.dia_liet_kich", S(ShapeKind.DirectionBox, 3000, 1000) },
                { "skill.tho.basic.kim_cang_quyen", S(ShapeKind.MeleeBox, 2300, 1100) },
                { "skill.tho.active.thach_kich", S(ShapeKind.MeleeBox, 2400, 1000) },
                { "skill.tho.active.tho_giap", S(ShapeKind.AreaSelf, 3000, 0) },
                { "skill.tho.active.dia_chan", S(ShapeKind.AreaSelf, 3200, 0) },
                { "skill.tho.active.son_bich", B(6500, 800, 4000, 5000) },
                { "skill.tho.active.thien_son_tran", S(ShapeKind.AreaSelf, 3800, 0) },
            };

        private static Spec S(ShapeKind k, int extent, int extent2)
        {
            return new Spec { Kind = k, Extent = extent, Extent2 = extent2 };
        }

        private static Spec P(int range, int speed, int radius)
        {
            return new Spec { Kind = ShapeKind.Projectile, Extent = range, Extent2 = radius, Extent3 = speed };
        }

        private static Spec L(ShapeKind k, int dist, int dur, int hh)
        {
            return new Spec { Kind = k, Extent = dist, Extent2 = hh, DurationMs = dur };
        }

        private static Spec B(int cast, int thickness, int height, int dur)
        {
            return new Spec { Kind = ShapeKind.BarrierPosition, Extent = cast, ThicknessMM = thickness, Extent3 = height, DurationMs = dur };
        }

        /// <summary>True when the skill id is a compiled launch skill.</summary>
        public static bool IsCompiled(string skillId)
        {
            return skillId != null && Specs.ContainsKey(skillId);
        }

        /// <summary>Resolves the world-space shape of a cast from the
        /// caster anchor (mm). origin = (anchorX, anchorY + 900);
        /// castX/castY are the requested AREA_POSITION center (clamped
        /// to authored cast range by the caller contract).</summary>
        public static bool TryResolve(
            string skillId,
            int anchorX,
            int anchorY,
            bool facingLeft,
            int castX,
            int castY,
            out Shape shape)
        {
            shape = default;
            if (skillId == null || !Specs.TryGetValue(skillId, out Spec spec))
            {
                return false;
            }
            int ox = anchorX;
            int oy = anchorY + OriginYOffsetMM;
            shape.Kind = spec.Kind;
            switch (spec.Kind)
            {
                case ShapeKind.MeleeBox:
                case ShapeKind.DirectionBox:
                    shape.MinY = oy - spec.Extent2;
                    shape.MaxY = oy + spec.Extent2;
                    if (facingLeft)
                    {
                        shape.MinX = ox - spec.Extent;
                        shape.MaxX = ox;
                    }
                    else
                    {
                        shape.MinX = ox;
                        shape.MaxX = ox + spec.Extent;
                    }
                    break;
                case ShapeKind.Projectile:
                    shape.CenterX = ox;
                    shape.CenterY = oy;
                    shape.RadiusMM = spec.Extent2;
                    shape.MinX = facingLeft ? ox - spec.Extent : ox;
                    shape.MaxX = facingLeft ? ox : ox + spec.Extent;
                    shape.MinY = oy - spec.Extent2;
                    shape.MaxY = oy + spec.Extent2;
                    break;
                case ShapeKind.AreaSelf:
                    shape.CenterX = ox;
                    shape.CenterY = oy;
                    shape.RadiusMM = spec.Extent;
                    break;
                case ShapeKind.AreaPosition:
                    shape.CenterX = castX;
                    shape.CenterY = castY;
                    shape.RadiusMM = spec.Extent2;
                    break;
                case ShapeKind.SingleTargetRange:
                    shape.CenterX = ox;
                    shape.CenterY = oy;
                    shape.RadiusMM = spec.Extent;
                    break;
                case ShapeKind.DashLine:
                case ShapeKind.MoveLine:
                case ShapeKind.MoveContactLine:
                    int endX = facingLeft ? ox - spec.Extent : ox + spec.Extent;
                    shape.PathMinX = facingLeft ? endX : ox;
                    shape.PathMaxX = facingLeft ? ox : endX;
                    shape.MinY = oy - spec.Extent2;
                    shape.MaxY = oy + spec.Extent2;
                    break;
                case ShapeKind.BarrierPosition:
                    shape.CenterX = castX;
                    shape.MinX = castX - spec.ThicknessMM / 2;
                    shape.MaxX = castX + spec.ThicknessMM / 2;
                    shape.MinY = castY;
                    shape.MaxY = castY + spec.Extent3;
                    break;
            }
            return true;
        }
    }
}
