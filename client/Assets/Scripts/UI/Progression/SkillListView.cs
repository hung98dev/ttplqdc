using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Progression
{
    /// <summary>
    /// Skill rows of the progression panel (skills.md § Skills tab):
    /// one row per learned skill in catalog order — name label, level,
    /// upgrade affordance gated on unspent skill points. The presenter
    /// writes via <see cref="Apply"/>; the panel owns cadence.
    /// </summary>
    public sealed class SkillListView : MonoBehaviour
    {
        /// <summary>Row name labels, in model order.</summary>
        public TMP_Text?[] NameLabels = new TMP_Text?[0];

        /// <summary>Row level readouts, in model order.</summary>
        public TMP_Text?[] LevelLabels = new TMP_Text?[0];

        /// <summary>Row upgrade buttons, in model order (nullable).</summary>
        public Button?[] UpgradeButtons = new Button?[0];

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

        /// <summary>Writes the model's skill rows. Skill ids ride on the
        /// buttons' own bound listeners — this view only paints.</summary>
        public void Apply(in ProgressionPresenter.Model m)
        {
            ApplyCount++;
            int rows = m.Skills.Length;
            for (int i = 0; i < NameLabels.Length; i++)
            {
                TMP_Text? name = NameLabels[i];
                TMP_Text? level = i < LevelLabels.Length ? LevelLabels[i] : null;
                Button? upgrade =
                    i < UpgradeButtons.Length ? UpgradeButtons[i] : null;
                bool has = i < rows;
                if (name != null)
                {
                    name.gameObject.SetActive(has);
                    if (has)
                    {
                        name.SetText(m.Skills[i].SkillId);
                    }
                }
                if (level != null)
                {
                    level.gameObject.SetActive(has);
                    if (has)
                    {
                        level.SetText(m.Skills[i].Level.ToString());
                    }
                }
                if (upgrade != null)
                {
                    upgrade.gameObject.SetActive(has);
                    upgrade.interactable =
                        has && m.UnspentSkillPoints > 0;
                }
            }
        }
    }
}
