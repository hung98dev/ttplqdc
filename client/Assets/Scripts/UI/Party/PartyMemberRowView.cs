using ThinhThan.Systems.Party;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Party
{
    /// <summary>One member row: name/class/level + leader + online mark
    /// + kick button (leader-only affordance).</summary>
    public sealed class PartyMemberRowView : MonoBehaviour
    {
        public TMP_Text? NameText;
        public TMP_Text? MetaText;
        public GameObject? LeaderMark;
        public Button? KickButton;
        private byte[] _characterId = new byte[0];

        public byte[] CharacterId
        {
            get
            {
                return _characterId;
            }
        }

        public void Bind(PartyMemberModel m, byte[] leaderId)
        {
            _characterId = m.CharacterId;
            if (NameText != null)
            {
                NameText.text = m.DisplayName;
            }
            if (MetaText != null)
            {
                MetaText.text = m.ClassId + " Lv" + m.Level +
                    (m.OnlineState ==
                        global::ThinhThan.Protocol.V1
                        .OnlineState.Online ? " • online" : " • offline");
            }
            if (LeaderMark != null)
            {
                LeaderMark.SetActive(SameId(m.CharacterId, leaderId));
            }
        }

        private static bool SameId(byte[] a, byte[] b)
        {
            if (a.Length != b.Length)
            {
                return false;
            }
            for (int i = 0; i < a.Length; i++)
            {
                if (a[i] != b[i])
                {
                    return false;
                }
            }
            return true;
        }
    }
}
