using System;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Five-skill hotbar (§3 K, L, U, I, O): cooldown radial + key label;
    /// fill is a fraction of the armed slot's end tick.
    /// </summary>
    public sealed class SkillHotbarWidget : MonoBehaviour
    {
        /// <summary>Cooldown sweep images, slots 1..5.</summary>
        public Image?[] CooldownFills = new Image?[5];

        /// <summary>Skill key labels K/L/U/I/O.</summary>
        public TMP_Text?[] KeyLabels = new TMP_Text?[5];

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

        public void Apply(in HudDataModel m, ulong nowTick)
        {
            ApplyCount++;
            // Zero every sweep, then fill armed slots — a cooled skill's
            // sweep must clear even when its row was purged.
            for (int i = 0; i < CooldownFills.Length; i++)
            {
                Image? fill = CooldownFills[i];
                if (fill != null)
                {
                    fill.fillAmount = 0f;
                }
            }

            ReadOnlySpan<HudCooldownSlot> rows = m.Cooldowns;
            for (int i = 0; i < rows.Length; i++)
            {
                int slot = rows[i].Slot;
                if (slot < 1 || slot > CooldownFills.Length)
                {
                    continue;
                }

                Image? fill = CooldownFills[slot - 1];
                if (fill != null)
                {
                    fill.fillAmount = rows[i].Fill(nowTick);
                }
            }
        }
    }
}
