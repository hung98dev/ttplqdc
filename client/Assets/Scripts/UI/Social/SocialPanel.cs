using ThinhThan.Systems.Social;
using UnityEngine;

namespace ThinhThan.UI.Social
{
    /// <summary>
    /// Minimal social panel projection: friend/request/block counts
    /// plus the chat-feed line count. Headless-CI friendly — view refs
    /// are optional and every Apply is null-guarded (IMP-066 pattern).
    /// </summary>
    public sealed class SocialPanel : MonoBehaviour
    {
        /// <summary>Last applied counts (assertable in tests).</summary>
        public int FriendCount
        {
            get;
            private set;
        }

        public int IncomingCount
        {
            get;
            private set;
        }

        public int OutgoingCount
        {
            get;
            private set;
        }

        public int BlockCount
        {
            get;
            private set;
        }

        public int ChatCount
        {
            get;
            private set;
        }

        /// <summary>Total Apply calls (test instrumentation).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Fold the projection into panel counters.</summary>
        public void Apply(SocialState state)
        {
            if (state == null)
            {
                return;
            }
            ApplyCount++;
            FriendCount = state.Friends.Count;
            IncomingCount = state.Incoming.Count;
            OutgoingCount = state.Outgoing.Count;
            BlockCount = state.Blocks.Count;
            ChatCount = state.Chat.Count;
        }
    }
}
