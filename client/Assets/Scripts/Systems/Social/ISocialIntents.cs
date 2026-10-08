using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Social
{
    /// <summary>Client intents for the social C2S set.</summary>
    public interface ISocialIntents
    {
        Awaitable<byte[]> SendChat(ChatChannel channel,
            byte[] targetCharacterId, string text,
            CancellationToken cancel);
        Awaitable<byte[]> RequestFriend(byte[] targetCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> AcceptFriend(byte[] requesterCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> DeclineFriend(byte[] requesterCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> RemoveFriend(byte[] targetCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> AddBlock(byte[] targetCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> RemoveBlock(byte[] targetCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> ReportPlayer(byte[] targetCharacterId,
            ReportReason reason, byte[] chatMessageId, string notes,
            CancellationToken cancel);
    }
}
