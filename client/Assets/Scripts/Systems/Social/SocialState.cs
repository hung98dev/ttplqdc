using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Social
{
    /// <summary>
    /// Client-side projection of the friends / block / chat surface:
    /// the 616 friend state (full snapshot then deltas), the 619 block
    /// list, the rolling chat feed, and the last typed results
    /// (654/655/633). All mutation happens through SocialApplier so
    /// UI only ever reads.
    /// </summary>
    public sealed class SocialState
    {
        /// <summary>Own character id for whisper/party perspectives.</summary>
        public byte[] LocalCharacterId = new byte[0];

        /// <summary>Friend entries keyed by hex character id.</summary>
        public readonly Dictionary<string, FriendEntry> Friends =
            new Dictionary<string, FriendEntry>();

        /// <summary>Latest incoming pending requests (616).</summary>
        public readonly List<FriendRequestView> Incoming =
            new List<FriendRequestView>();

        /// <summary>Latest outgoing pending requests (616).</summary>
        public readonly List<FriendOutgoingView> Outgoing =
            new List<FriendOutgoingView>();

        /// <summary>Latest full block list (619 REPLACEABLE_STATE).</summary>
        public readonly List<BlockEntry> Blocks =
            new List<BlockEntry>();

        /// <summary>Rolling chat feed, capped at MaxChatMessages.</summary>
        public readonly List<S2CChatMessage> Chat =
            new List<S2CChatMessage>();

        public const int MaxChatMessages = 64;

        /// <summary>Latest 612 request push (for accept/decline UI).</summary>
        public S2CFriendRequest? PendingPush;

        public S2CChatSendResult? LastChatResult;
        public S2CSocialResult? LastSocialResult;
        public S2CReportPlayerResult? LastReportResult;

        public static string Key(Google.Protobuf.ByteString id)
        {
            return id == null ? "" : id.ToByteArray().Length == 0
                ? "" : System.BitConverter.ToString(id.ToByteArray());
        }

        /// <summary>Apply a 616: full snapshot replaces; delta applies
        /// entry changes and always refreshes the request lists.</summary>
        public void ApplyFriendState(S2CFriendState state)
        {
            if (state == null)
            {
                return;
            }
            if (state.FullSnapshot)
            {
                Friends.Clear();
            }
            foreach (FriendEntry e in state.Entries)
            {
                string k = Key(e.FriendCharacterId);
                if (k.Length == 0)
                {
                    continue;
                }
                if (e.Change == FriendChange.Removed)
                {
                    Friends.Remove(k);
                }
                else
                {
                    Friends[k] = e;
                }
            }
            Incoming.Clear();
            foreach (FriendRequestView v in state.IncomingRequests)
            {
                Incoming.Add(v);
            }
            Outgoing.Clear();
            foreach (FriendOutgoingView v in state.OutgoingRequests)
            {
                Outgoing.Add(v);
            }
        }

        /// <summary>Apply a 619: REPLACEABLE_STATE replaces the list.</summary>
        public void ApplyBlockState(S2CBlockState state)
        {
            if (state == null)
            {
                return;
            }
            Blocks.Clear();
            foreach (BlockEntry e in state.Blocked)
            {
                Blocks.Add(e);
            }
        }

        /// <summary>Append a 601 to the capped rolling feed.</summary>
        public void AppendChat(S2CChatMessage msg)
        {
            if (msg == null)
            {
                return;
            }
            Chat.Add(msg);
            while (Chat.Count > MaxChatMessages)
            {
                Chat.RemoveAt(0);
            }
        }

        /// <summary>A 612 inbound push primes the pending view.</summary>
        public void ApplyRequestPush(S2CFriendRequest req)
        {
            PendingPush = req;
        }
    }
}
