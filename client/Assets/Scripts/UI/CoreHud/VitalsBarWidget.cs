using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// HP/MP/shield bars + numeric readout (§2 bottom-centre cluster).
    /// Apply only writes fill + text — PERF-022: no layout work, 0-alloc.
    /// </summary>
    public sealed class VitalsBarWidget : MonoBehaviour
    {
        public Image? HpFill;
        public Image? MpFill;
        public Image? ShieldFill;
        public TMP_Text? HpText;
        public TMP_Text? MpText;

        private readonly char[] _scratch = new char[24];

        /// <summary>Apply-call count (once-per-frame proof for tests).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        private void Awake()
        {
            // PERF-022: bars/text are display-only — never raycast targets.
            foreach (Graphic g in GetComponentsInChildren<Graphic>(true))
            {
                g.raycastTarget = false;
            }
        }

        public void Apply(in HudDataModel m)
        {
            ApplyCount++;
            SetFill(HpFill, m.Hp, m.MaxHp);
            SetFill(MpFill, m.Mp, m.MaxMp);
            SetFill(ShieldFill, m.Shield, m.MaxHp);
            HudText.SetPair(HpText, m.Hp, m.MaxHp, _scratch);
            HudText.SetPair(MpText, m.Mp, m.MaxMp, _scratch);
        }

        private static void SetFill(Image? fill, long value, long max)
        {
            if (fill == null)
            {
                return;
            }

            fill.fillAmount = max > 0L
                ? Mathf.Clamp01((float)value / max)
                : 0f;
        }
    }
}
