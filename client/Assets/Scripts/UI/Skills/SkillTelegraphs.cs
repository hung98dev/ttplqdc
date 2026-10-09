using System.Collections.Generic;
using ThinhThan.Systems.Skills;

namespace ThinhThan.UI.Skills
{
    /// <summary>
    /// Skill telegraph presenter (IMP-015): converts the resolved
    /// server-mirrored geometry into telegraph primitives for HUD/UI
    /// rendering. Telegraphs always present the resolved authoritative
    /// geometry — never sprite bounds — and never alter reach.
    /// </summary>
    public sealed class SkillTelegraphs
    {
        /// <summary>One telegraph primitive in millimeters.</summary>
        public struct TelegraphPrimitive
        {
            public SkillGeometry.ShapeKind Kind;
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

        /// <summary>Authored cost/cooldown metadata paired with the
        /// resolved shape for HUD presentation.</summary>
        public struct TelegraphDescriptor
        {
            public string SkillId;
            public bool Resolved;
            public TelegraphPrimitive Primitive;
            public int OuterReachMM;
        }

        private readonly List<TelegraphPrimitive> _primitives =
            new List<TelegraphPrimitive>();

        /// <summary>Current telegraph primitives (most recent Preview).</summary>
        public IReadOnlyList<TelegraphPrimitive> Primitives
        {
            get
            {
                return _primitives;
            }
        }

        /// <summary>Builds the telegraph descriptor for one cast
        /// intent: resolved shape + outer reach. Returns false when the
        /// skill id is not a compiled launch skill.</summary>
        public bool TryDescribe(
            string skillId,
            int anchorX,
            int anchorY,
            bool facingLeft,
            int castX,
            int castY,
            out TelegraphDescriptor descriptor)
        {
            descriptor = new TelegraphDescriptor
            {
                SkillId = skillId,
            };
            if (!SkillGeometry.TryResolve(
                    skillId, anchorX, anchorY, facingLeft, castX, castY,
                    out SkillGeometry.Shape shape))
            {
                return false;
            }
            descriptor.Resolved = true;
            descriptor.Primitive = new TelegraphPrimitive
            {
                Kind = shape.Kind,
                MinX = shape.MinX,
                MaxX = shape.MaxX,
                MinY = shape.MinY,
                MaxY = shape.MaxY,
                CenterX = shape.CenterX,
                CenterY = shape.CenterY,
                RadiusMM = shape.RadiusMM,
                PathMinX = shape.PathMinX,
                PathMaxX = shape.PathMaxX,
            };
            descriptor.OuterReachMM = OuterReach(shape);
            return true;
        }

        /// <summary>Stages the telegraph primitives for a cast preview;
        /// subsequent renders consume <see cref="Primitives"/>.</summary>
        public bool Preview(
            string skillId,
            int anchorX,
            int anchorY,
            bool facingLeft,
            int castX,
            int castY)
        {
            _primitives.Clear();
            if (!TryDescribe(
                    skillId, anchorX, anchorY, facingLeft, castX, castY,
                    out TelegraphDescriptor d))
            {
                return false;
            }
            _primitives.Add(d.Primitive);
            return true;
        }

        /// <summary>Clears the staged telegraph (intent dropped).</summary>
        public void Clear()
        {
            _primitives.Clear();
        }

        private static int OuterReach(SkillGeometry.Shape s)
        {
            switch (s.Kind)
            {
                case SkillGeometry.ShapeKind.AreaSelf:
                case SkillGeometry.ShapeKind.AreaPosition:
                case SkillGeometry.ShapeKind.SingleTargetRange:
                    return s.RadiusMM;
                case SkillGeometry.ShapeKind.MeleeBox:
                case SkillGeometry.ShapeKind.DirectionBox:
                    return s.MaxX - s.MinX;
                case SkillGeometry.ShapeKind.DashLine:
                case SkillGeometry.ShapeKind.MoveLine:
                case SkillGeometry.ShapeKind.MoveContactLine:
                    return s.PathMaxX - s.PathMinX;
                case SkillGeometry.ShapeKind.Projectile:
                    return s.MaxX - s.MinX + s.RadiusMM;
                case SkillGeometry.ShapeKind.BarrierPosition:
                    return s.MaxX - s.MinX;
                default:
                    return 0;
            }
        }
    }
}
