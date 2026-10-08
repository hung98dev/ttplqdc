using System;
using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Localization;
using ThinhThan.Protocol.V1;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Screens
{

    /// <summary>
    /// Thin view helpers shared by the IMP-099 screens: TMP/UGUI refs are
    /// serialized nullable and every setter null-guards — headless PlayMode
    /// has no TMP Essentials (IMP-066 precedent), so tests exercise the
    /// presenters and never instantiate views with components.
    /// Buttons keep a ≥44 pt touch target per § Accessibility.
    /// </summary>
    public abstract class ScreenView : MonoBehaviour
    {
        /// <summary>Minimum touch-target points (client_experience_contract §6).</summary>
        public const float MinTouchTargetPoints = 44f;

        /// <summary>Loc-aware TMP setter; null-safe.</summary>
        protected static void SetLabel(TMP_Text? label, string key)
        {
            if (label != null)
            {
                label.text = Loc.Get(key);
            }
        }

        /// <summary>Loc-aware formatted TMP setter; null-safe.</summary>
        protected static void SetLabel(
            TMP_Text? label, string key, IReadOnlyDictionary<string, object> args)
        {
            if (label != null)
            {
                label.text = Loc.Get(key, args);
            }
        }

        /// <summary>Plain TMP setter; null-safe.</summary>
        protected static void SetText(TMP_Text? label, string text)
        {
            if (label != null)
            {
                label.text = text;
            }
        }
    }
}
