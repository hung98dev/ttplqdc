using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Guild intent sender: builds each C2S request with a fresh
    /// operation_id and hands it to the send seam. Wire ids follow
    /// messages.md; every request commits exactly one 649 verdict.
    /// </summary>
    public sealed class GuildIntents : IGuildIntents
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SGuildInvite = 608;
        public const uint C2SGuildAccept = 610;
        public const uint C2SGuildDecline = 623;
        public const uint C2SGuildLeave = 624;
        public const uint C2SGuildKick = 625;
        public const uint C2SGuildRoleUpdate = 626;
        public const uint C2SGuildLeaderTransfer = 627;
        public const uint C2SGuildCreate = 637;
        public const uint C2SGuildDisband = 638;
        public const uint C2SGuildApply = 639;
        public const uint C2SGuildApplicationDecide = 640;
        public const uint C2SGuildMotdSet = 642;
        public const uint C2SGuildLeadershipClaim = 643;
        public const uint C2SGuildBlessingVote = 648;
        public const uint C2SGuildSettingsSet = 650;
        public const uint C2SGuildInviteCancel = 651;
        public const uint C2SGuildApplicationCancel = 652;

        private readonly IGuildSender _sender;

        public GuildIntents(IGuildSender sender)
        {
            _sender = sender;
        }

        public async Awaitable<ulong> RequestCreate(
            string guildName, CancellationToken cancel)
        {
            var req = new C2SGuildCreate
            {
                OperationId = OpId(),
                GuildName = guildName,
            };
            return await _sender.SendAsync(C2SGuildCreate, req, cancel);
        }

        public async Awaitable<ulong> RequestDisband(CancellationToken cancel)
        {
            var req = new C2SGuildDisband
            {
                OperationId = OpId(),
            };
            return await _sender.SendAsync(C2SGuildDisband, req, cancel);
        }

        public async Awaitable<ulong> RequestInvite(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SGuildInvite
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SGuildInvite, req, cancel);
        }

        public async Awaitable<ulong> RequestAccept(
            byte[] guildId, CancellationToken cancel)
        {
            var req = new C2SGuildAccept
            {
                OperationId = OpId(),
                GuildId = ByteString.CopyFrom(guildId),
            };
            return await _sender.SendAsync(C2SGuildAccept, req, cancel);
        }

        public async Awaitable<ulong> RequestDecline(
            byte[] guildId, CancellationToken cancel)
        {
            var req = new C2SGuildDecline
            {
                OperationId = OpId(),
                GuildId = ByteString.CopyFrom(guildId),
            };
            return await _sender.SendAsync(C2SGuildDecline, req, cancel);
        }

        public async Awaitable<ulong> RequestInviteCancel(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SGuildInviteCancel
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SGuildInviteCancel, req, cancel);
        }

        public async Awaitable<ulong> RequestApply(
            byte[] guildId, CancellationToken cancel)
        {
            var req = new C2SGuildApply
            {
                OperationId = OpId(),
                GuildId = ByteString.CopyFrom(guildId),
            };
            return await _sender.SendAsync(C2SGuildApply, req, cancel);
        }

        public async Awaitable<ulong> RequestApplicationDecide(
            byte[] applicantCharacterId, GuildApplicationDecision decision,
            CancellationToken cancel)
        {
            var req = new C2SGuildApplicationDecide
            {
                OperationId = OpId(),
                ApplicantCharacterId = ByteString.CopyFrom(applicantCharacterId),
                Decision = decision,
            };
            return await _sender.SendAsync(C2SGuildApplicationDecide, req, cancel);
        }

        public async Awaitable<ulong> RequestApplicationCancel(
            byte[] guildId, CancellationToken cancel)
        {
            var req = new C2SGuildApplicationCancel
            {
                OperationId = OpId(),
                GuildId = ByteString.CopyFrom(guildId),
            };
            return await _sender.SendAsync(C2SGuildApplicationCancel, req, cancel);
        }

        public async Awaitable<ulong> RequestLeave(CancellationToken cancel)
        {
            var req = new C2SGuildLeave
            {
                OperationId = OpId(),
            };
            return await _sender.SendAsync(C2SGuildLeave, req, cancel);
        }

        public async Awaitable<ulong> RequestKick(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SGuildKick
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SGuildKick, req, cancel);
        }

        public async Awaitable<ulong> RequestRoleUpdate(
            byte[] targetCharacterId, string newRole, CancellationToken cancel)
        {
            var req = new C2SGuildRoleUpdate
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
                NewRole = newRole,
            };
            return await _sender.SendAsync(C2SGuildRoleUpdate, req, cancel);
        }

        public async Awaitable<ulong> RequestLeaderTransfer(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SGuildLeaderTransfer
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SGuildLeaderTransfer, req, cancel);
        }

        public async Awaitable<ulong> RequestMotdSet(
            string motd, CancellationToken cancel)
        {
            var req = new C2SGuildMotdSet
            {
                OperationId = OpId(),
                Motd = motd,
            };
            return await _sender.SendAsync(C2SGuildMotdSet, req, cancel);
        }

        public async Awaitable<ulong> RequestLeadershipClaim(
            CancellationToken cancel)
        {
            var req = new C2SGuildLeadershipClaim
            {
                OperationId = OpId(),
            };
            return await _sender.SendAsync(C2SGuildLeadershipClaim, req, cancel);
        }

        public async Awaitable<ulong> RequestSettingsSet(
            GuildRecruitmentMode mode, CancellationToken cancel)
        {
            var req = new C2SGuildSettingsSet
            {
                OperationId = OpId(),
                RecruitmentMode = mode,
            };
            return await _sender.SendAsync(C2SGuildSettingsSet, req, cancel);
        }

        public async Awaitable<ulong> RequestBlessingVote(
            string cycleId, string blessingId, CancellationToken cancel)
        {
            var req = new C2SGuildBlessingVote
            {
                OperationId = OpId(),
                CycleId = cycleId,
                BlessingId = blessingId,
            };
            return await _sender.SendAsync(C2SGuildBlessingVote, req, cancel);
        }

        private static ByteString OpId()
        {
            return ByteString.CopyFrom(Guid.NewGuid().ToByteArray());
        }
    }
}
