using ThinhThan.Core.Input;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Virtual joystick (§4): left bottom-half activation zone + nub. The
    /// zone itself is geometry (<see cref="VirtualJoystickZone"/>); this
    /// widget only renders active/offset state pushed by the input
    /// composition — no per-frame work, apply-on-change only.
    /// </summary>
    public sealed class VirtualJoystickWidget : MonoBehaviour
    {
        /// <summary>Joystick cluster root (shown while touching).</summary>
        public GameObject? Root;

        /// <summary>Movable nub inside the pad ring.</summary>
        public RectTransform? Nub;

        /// <summary>Nub travel radius in local units.</summary>
        public float NubRadius = 48f;

        private bool _shown;

        /// <summary>
        /// Shows/hides the pad and moves the nub; called on touch change,
        /// not per frame.
        /// </summary>
        public void SetState(bool active, Vector2 offset)
        {
            if (Root != null && _shown != active)
            {
                _shown = active;
                Root.SetActive(active);
            }

            if (Nub != null && active)
            {
                Nub.anchoredPosition = offset * NubRadius;
            }
        }

        private void Awake()
        {
            // The zone reads pointer events on its own hit area; the pad
            // visuals never raycast.
            foreach (Graphic g in GetComponentsInChildren<Graphic>(true))
            {
                g.raycastTarget = false;
            }
        }
    }
}
