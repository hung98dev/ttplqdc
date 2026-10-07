using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Quest-tracker dock (§2 right edge): pinned objective lines fed by
    /// the quest composition seam; text-only, no layout group.
    /// </summary>
    public sealed class QuestTrackerWidget : MonoBehaviour
    {
        public TMP_Text? Body;

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
            HudText.Set(Body, m.QuestDockText);
        }
    }
}
