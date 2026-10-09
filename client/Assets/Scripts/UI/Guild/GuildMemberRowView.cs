using ThinhThan.Systems.Guild;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Guild
{
    /// <summary>
    /// One roster row: name, class/level meta, role badge, online mark,
    /// kick button (shown when the receiver has kick permission).
    /// </summary>
    public sealed class GuildMemberRowView : MonoBehaviour
    {
        public TMP_Text? NameText;
        public TMP_Text? MetaText;
        public TMP_Text? RoleText;
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

        public void Bind(GuildMemberModel m, bool receiverCanKick)
        {
            _characterId = m.CharacterId;
            if (NameText != null)
            {
                NameText.text = m.DisplayName;
            }
            if (MetaText != null)
            {
                MetaText.text = m.ClassId + " Lv" + m.Level +
                    (m.Online ? " • online" : " • offline");
            }
            if (RoleText != null)
            {
                RoleText.text = m.Role;
            }
            if (LeaderMark != null)
            {
                LeaderMark.SetActive(m.Role == "guild.role.leader");
            }
            if (KickButton != null)
            {
                KickButton.gameObject.SetActive(
                    receiverCanKick && m.Role != "guild.role.leader");
            }
        }
    }
}
