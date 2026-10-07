using UnityEngine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Fits a HUD RectTransform into <see cref="Screen.safeArea"/> plus a
    /// minimum edge padding (§4: touch-reachable zones keep ≥48 px from
    /// the hardware edge). Composition calls <see cref="ApplySafeArea"/>
    /// on resolution/orientation change — never per frame.
    /// </summary>
    public sealed class SafeAreaFitter : MonoBehaviour
    {
        /// <summary>Minimum padding from the safe-area edge, in pixels.</summary>
        public const float MinPaddingPx = 48f;

        public RectTransform? Target;

        /// <summary>Current applied inset (test-visible).</summary>
        public Vector2 LastInset
        {
            get;
            private set;
        }

        /// <summary>
        /// Recomputes anchors for the given safe area and screen size.
        /// Testable without a device: pass the rect + resolution directly.
        /// </summary>
        public void ApplySafeArea(Rect safeArea, Vector2Int screenSize)
        {
            RectTransform? target = Target != null
                ? Target
                : transform as RectTransform;
            if (target == null || screenSize.x <= 0 || screenSize.y <= 0)
            {
                return;
            }

            float padX = Mathf.Min(MinPaddingPx, safeArea.width * 0.25f);
            float padY = Mathf.Min(MinPaddingPx, safeArea.height * 0.25f);
            Vector2 min = safeArea.position + new Vector2(padX, padY);
            Vector2 max = safeArea.position + safeArea.size -
                new Vector2(padX, padY);
            min.x /= screenSize.x;
            min.y /= screenSize.y;
            max.x /= screenSize.x;
            max.y /= screenSize.y;
            target.anchorMin = min;
            target.anchorMax = max;
            target.offsetMin = Vector2.zero;
            target.offsetMax = Vector2.zero;
            LastInset = new Vector2(padX, padY);
        }
    }
}
