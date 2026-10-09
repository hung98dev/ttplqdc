using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Party
{
    /// <summary>Outbound party intents — fire-and-forget; the server
    /// adjudicates every request with one 653 (ADR-0064).</summary>
    public interface IPartyIntents
    {
        Awaitable<ulong> RequestInvite(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestAccept(byte[] partyId, byte[] inviterCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestDecline(byte[] partyId, byte[] inviterCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestLeave(CancellationToken cancel);
        Awaitable<ulong> RequestKick(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestInviteCancel(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestLeaderTransfer(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestBoardPost(string dungeonId, uint desiredSize, string note, CancellationToken cancel);
        Awaitable<ulong> RequestBoardCancel(CancellationToken cancel);
    }
}
