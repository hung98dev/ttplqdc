using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Target frame (§2 top-centre): name, level, HP bar — shows only the
    /// server-accepted target carried on the model.
    /// </summary>
    public sealed class TargetFrameWidget : MonoBehaviour
    {
        public GameObject? Root;
        public TMP_Text? NameText;
        public TMP_Text? LevelText;
        public Image? HpFill;

        private readonly char[] _scratch = new char[24];
        private bool _shown = true;

        public int ApplyCount
        {
            get;
            private set;
        }

        private void Awake()
        {
            foreach (Graphic g in GetComponentsInChildren<Graphic>(true))
            {
                g.raycastTarget = false;
            }
        }

        public void Apply(in HudDataModel m)
        {
            ApplyCount++;
            if (m.HasTarget != _shown)
            {
                _shown = m.HasTarget;
                if (Root != null)
                {
                    Root.SetActive(_shown);
                }
            }

            if (!m.HasTarget)
            {
                return;
            }

            HudText.Set(NameText, m.TargetName);
            HudText.SetNumber(LevelText, m.TargetLevel, _scratch);
            if (HpFill != null)
            {
                HpFill.fillAmount = m.TargetMaxHp > 0L
                    ? Mathf.Clamp01((float)m.TargetHp / m.TargetMaxHp)
                    : 0f;
            }
        }
    }
}
