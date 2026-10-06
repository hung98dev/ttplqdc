using System;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Buff/debuff icon row (§2 vitals cluster, §6): buff = rounded frame,
    /// debuff = triangle-with-skull frame — differentiated by SHAPE, never
    /// by colour alone. Fixed pooled slots (≤ <see cref=
    /// "HudDataModel.StatusCapacity"/>), no per-frame allocation.
    /// </summary>
    public sealed class BuffIconRow : MonoBehaviour
    {
        /// <summary>Pooled icon roots in display order.</summary>
        public GameObject?[] Icons = new GameObject?[HudDataModel.StatusCapacity];

        /// <summary>Frame images; sprite picks the shape.</summary>
        public Image?[] Frames = new Image?[HudDataModel.StatusCapacity];

        /// <summary>Stack count labels on each slot.</summary>
        public TMP_Text?[] Stacks = new TMP_Text?[HudDataModel.StatusCapacity];

        /// <summary>Buff frame sprite (rounded + up-arrow silhouette).</summary>
        public Sprite? BuffSprite;

        /// <summary>Debuff frame sprite (triangle + skull/down silhouette).</summary>
        public Sprite? DebuffSprite;

        private readonly char[] _scratch = new char[8];

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
            ReadOnlySpan<HudStatusSlot> rows = m.Statuses;
            for (int i = 0; i < Icons.Length; i++)
            {
                bool active = i < rows.Length;
                GameObject? icon = Icons[i];
                if (icon != null && icon.activeSelf != active)
                {
                    icon.SetActive(active);
                }

                if (!active)
                {
                    continue;
                }

                HudStatusSlot row = rows[i];
                Image? frame = i < Frames.Length ? Frames[i] : null;
                if (frame != null)
                {
                    frame.sprite = IsDebuff(row.StatusKind)
                        ? DebuffSprite
                        : BuffSprite;
                }

                TMP_Text? stacks = i < Stacks.Length ? Stacks[i] : null;
                if (stacks != null)
                {
                    HudText.SetNumber(
                        stacks, row.Stacks > 1 ? row.Stacks : 0L, _scratch);
                }
            }
        }

        /// <summary>status_kind from the wire; anything but "buff" is a debuff shape.</summary>
        private static bool IsDebuff(string kind)
        {
            return !string.Equals(kind, "buff", StringComparison.Ordinal);
        }
    }
}
