using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Progression
{
    /// <summary>
    /// Progression panel root (client_experience_contract.md character
    /// screen): level + EXP bar, unspent skill/potential pools, allocated
    /// potential readout, skill list, loadout slots and the latest
    /// mutation verdict. <see cref="Apply"/> runs at most once per frame
    /// from the UI stage and writes only dirty model sections.
    /// </summary>
    public sealed class ProgressionPanel : MonoBehaviour
    {
        /// <summary>Level readout.</summary>
        public TMP_Text? LevelText;

        /// <summary>EXP bar fill (fraction = current_exp/exp_required).</summary>
        public Image? ExpFill;

        /// <summary>"exp / required" readout.</summary>
        public TMP_Text? ExpText;

        /// <summary>Unspent skill-point pool.</summary>
        public TMP_Text? SkillPointsText;

        /// <summary>Unspent potential pool.</summary>
        public TMP_Text? PotentialPointsText;

        /// <summary>Allocated potential readouts {STR, VIT, INT, AGI}.</summary>
        public TMP_Text?[] PotentialLabels = new TMP_Text?[4];

        /// <summary>Active loadout slot labels 1..5.</summary>
        public TMP_Text?[] LoadoutLabels = new TMP_Text?[5];

        /// <summary>Basic-skill label.</summary>
        public TMP_Text? BasicLabel;

        /// <summary>Latest mutation verdict line.</summary>
        public TMP_Text? ResultText;

        /// <summary>Skill rows widget.</summary>
        public SkillListView? SkillList;

        /// <summary>Total Apply calls — once-per-frame proof for tests.</summary>
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

        /// <summary>Applies dirty model sections. Not called by Unity —
        /// the UI stage owns cadence; call ClearDirty after this.</summary>
        public void Apply(in ProgressionPresenter.Model m)
        {
            ApplyCount++;
            ProgressionPresenter.Dirty dirty = m.Dirty;

            if ((dirty & ProgressionPresenter.Dirty.Vitals) != 0)
            {
                if (LevelText != null)
                {
                    LevelText.SetText(m.Level.ToString());
                }
                if (ExpFill != null)
                {
                    ExpFill.fillAmount = m.ExpToNext > 0
                        ? (float)((double)m.CurrentExp / m.ExpToNext)
                        : 1f;
                }
                if (ExpText != null)
                {
                    ExpText.SetText(
                        m.CurrentExp + " / " + m.ExpToNext);
                }
            }
            if ((dirty & ProgressionPresenter.Dirty.Points) != 0)
            {
                if (SkillPointsText != null)
                {
                    SkillPointsText.SetText(
                        m.UnspentSkillPoints.ToString());
                }
                if (PotentialPointsText != null)
                {
                    PotentialPointsText.SetText(
                        m.UnspentPotentialPoints.ToString());
                }
                int[] alloc =
                {
                    m.PotentialStr, m.PotentialVit,
                    m.PotentialInt, m.PotentialAgi,
                };
                for (int i = 0; i < PotentialLabels.Length && i < 4; i++)
                {
                    TMP_Text? label = PotentialLabels[i];
                    if (label != null)
                    {
                        label.SetText(alloc[i].ToString());
                    }
                }
            }
            if ((dirty & ProgressionPresenter.Dirty.Skills) != 0)
            {
                if (BasicLabel != null)
                {
                    BasicLabel.SetText(m.BasicSkillId ?? string.Empty);
                }
                for (int i = 0; i < LoadoutLabels.Length; i++)
                {
                    TMP_Text? label = LoadoutLabels[i];
                    if (label != null)
                    {
                        string? id = i < m.ActiveSlots.Length
                            ? m.ActiveSlots[i]
                            : null;
                        label.SetText(id ?? string.Empty);
                    }
                }
                if (SkillList != null)
                {
                    SkillList.Apply(in m);
                }
            }
            if ((dirty & ProgressionPresenter.Dirty.Result) != 0 &&
                ResultText != null)
            {
                ResultText.SetText(m.ResultText);
            }
        }
    }
}
