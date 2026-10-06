using System;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Top-right cluster (§2): map name + channel, ping readout, and the
    /// menu button that opens the shell menu (event, not state — the
    /// shell screen handles the rest).
    /// </summary>
    public sealed class MenuButtonWidget : MonoBehaviour
    {
        public TMP_Text? MapText;
        public TMP_Text? PingText;
        public GameObject? PingWarning;

        private readonly char[] _scratch = new char[24];
        private bool _warningShown;

        /// <summary>Raised when the menu button is pressed.</summary>
        public event Action? MenuPressed;

        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>uGUI Button.onClick hook.</summary>
        public void OnPressed()
        {
            Action? handler = MenuPressed;
            if (handler != null)
            {
                handler();
            }
        }

        private void Awake()
        {
            // Only the menu Button graphic raycasts; labels don't.
            foreach (TMP_Text t in GetComponentsInChildren<TMP_Text>(true))
            {
                t.raycastTarget = false;
            }
        }

        public void Apply(in HudDataModel m)
        {
            ApplyCount++;
            if (m.MapDisplay != null && m.ChannelIndex > 0U)
            {
                HudText.Set(MapText, m.MapDisplay + " · Ch" + m.ChannelIndex);
            }
            else
            {
                HudText.Set(MapText, m.MapDisplay);
            }

            if (m.PingMs >= 0)
            {
                HudText.SetNumber(PingText, m.PingMs, _scratch);
            }
            else
            {
                HudText.Set(PingText, null);
            }

            bool warning = m.PingMs > 250;
            if (PingWarning != null && _warningShown != warning)
            {
                _warningShown = warning;
                PingWarning.SetActive(warning);
            }
        }
    }
}
