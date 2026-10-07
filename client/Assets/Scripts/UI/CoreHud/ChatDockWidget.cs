using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Chat dock (§2 bottom-left): last lines pre-formatted by the chat
    /// composition seam (Enter/Back opens the real input there, not here).
    /// </summary>
    public sealed class ChatDockWidget : MonoBehaviour
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
            HudText.Set(Body, m.ChatDockText);
        }
    }
}
