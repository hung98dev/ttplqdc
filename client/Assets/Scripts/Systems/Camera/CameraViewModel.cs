using UnityEngine;

namespace ThinhThan.Systems.Camera
{
    /// <summary>
    /// World-view geometry (physics.md §6.2): orthographic half-height
    /// 7.2 m → 25.6 × 14.4 m view at 16:9. Wider screens pillarbox, capped
    /// at 21:9; narrower letterbox, capped at 16:9 width. Pure math — no
    /// Unity camera required so PlayMode tests exercise it directly.
    /// </summary>
    public sealed class CameraViewModel
    {
        /// <summary>Orthographic half-height in meters (always 7.2).</summary>
        public const float OrthoHalfHeight = 7.2f;

        /// <summary>Visible height at the reference 16:9 aspect.</summary>
        public const float ReferenceViewHeight = 14.4f;

        /// <summary>Visible width at the reference 16:9 aspect.</summary>
        public const float ReferenceViewWidth = 25.6f;

        public const float ReferenceAspect = 16f / 9f;
        public const float PillarboxMaxAspect = 21f / 9f;
        public const float MaxViewWidth = ReferenceViewHeight * PillarboxMaxAspect;

        /// <summary>Visible world width in meters at the current aspect.</summary>
        public float ViewWidth
        {
            get;
            private set;
        }

        /// <summary>Visible world height in meters at the current aspect.</summary>
        public float ViewHeight
        {
            get;
            private set;
        }

        /// <summary>
        /// Normalized viewport rect for the render camera's viewport —
        /// narrower than 1 on the pillar/letterboxed axis.
        /// </summary>
        public Rect Viewport
        {
            get;
            private set;
        }

        /// <summary>View dims at the reference aspect before the first tick.</summary>
        public CameraViewModel()
        {
            ViewWidth = ReferenceViewWidth;
            ViewHeight = ReferenceViewHeight;
            Viewport = new Rect(0f, 0f, 1f, 1f);
        }

        /// <summary>Recomputes view dims for the given screen aspect.</summary>
        public void Resize(float aspect)
        {
            if (aspect >= PillarboxMaxAspect)
            {
                // Ultra-wide: horizontal reveal capped at 21:9 → pillarbox.
                ViewWidth = MaxViewWidth;
                ViewHeight = ReferenceViewHeight;
                float w = PillarboxMaxAspect / aspect;
                Viewport = new Rect((1f - w) * 0.5f, 0f, w, 1f);
            }
            else if (aspect >= ReferenceAspect)
            {
                ViewWidth = ReferenceViewHeight * aspect;
                ViewHeight = ReferenceViewHeight;
                Viewport = new Rect(0f, 0f, 1f, 1f);
            }
            else
            {
                // Narrow: horizontal reveal capped at 16:9 → letterbox.
                ViewWidth = ReferenceViewWidth;
                ViewHeight = ReferenceViewWidth / aspect;
                float h = ReferenceAspect / aspect;
                Viewport = new Rect(0f, (1f - h) * 0.5f, 1f, h);
            }
        }

        /// <summary>Half view width in integer millimeters.</summary>
        public long HalfWidthMm
        {
            get
            {
                return (long)(ViewWidth * 500.0);
            }
        }

        /// <summary>Half view height in integer millimeters.</summary>
        public long HalfHeightMm
        {
            get
            {
                return (long)(ViewHeight * 500.0);
            }
        }
    }
}
