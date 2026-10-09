using ThinhThan.Systems.Atlas;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Atlas
{
    /// <summary>
    /// One journal-grid cell: page id label, tier badge
    /// (Locked/Seen/Studied/Mastered), progress text and the "new"
    /// marker shown while any reached tier stays unacknowledged.
    /// </summary>
    public sealed class AtlasPageCellView : MonoBehaviour
    {
        public TMP_Text? NameText;
        public TMP_Text? TierText;
        public TMP_Text? ProgressText;
        public GameObject? NewMarker;

        public string PageId = "";

        public void Bind(AtlasPageModel m)
        {
            PageId = m.PageId;
            if (NameText != null)
            {
                NameText.text = m.PageId;
            }
            if (TierText != null)
            {
                TierText.text = m.TierName.ToString();
            }
            if (ProgressText != null)
            {
                ProgressText.text = m.ProgressText();
            }
            if (NewMarker != null)
            {
                NewMarker.SetActive(m.HasUnacknowledgedTier());
            }
        }
    }
}
