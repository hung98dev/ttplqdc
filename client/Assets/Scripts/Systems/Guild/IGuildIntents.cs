using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Outbound guild intents (IMP-036 client surface). Each request
    /// produces exactly one S2C_GUILD_RESULT(649) server verdict.
    /// </summary>
    public interface IGuildIntents
    {
        Awaitable<ulong> RequestCreate(string guildName, CancellationToken cancel);
        Awaitable<ulong> RequestDisband(CancellationToken cancel);
        Awaitable<ulong> RequestInvite(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestAccept(byte[] guildId, CancellationToken cancel);
        Awaitable<ulong> RequestDecline(byte[] guildId, CancellationToken cancel);
        Awaitable<ulong> RequestInviteCancel(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestApply(byte[] guildId, CancellationToken cancel);
        Awaitable<ulong> RequestApplicationDecide(byte[] applicantCharacterId, GuildApplicationDecision decision, CancellationToken cancel);
        Awaitable<ulong> RequestApplicationCancel(byte[] guildId, CancellationToken cancel);
        Awaitable<ulong> RequestLeave(CancellationToken cancel);
        Awaitable<ulong> RequestKick(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestRoleUpdate(byte[] targetCharacterId, string newRole, CancellationToken cancel);
        Awaitable<ulong> RequestLeaderTransfer(byte[] targetCharacterId, CancellationToken cancel);
        Awaitable<ulong> RequestMotdSet(string motd, CancellationToken cancel);
        Awaitable<ulong> RequestLeadershipClaim(CancellationToken cancel);
        Awaitable<ulong> RequestSettingsSet(GuildRecruitmentMode mode, CancellationToken cancel);
        Awaitable<ulong> RequestBlessingVote(string cycleId, string blessingId, CancellationToken cancel);
    }
}
