using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Party
{
    /// <summary>
    /// <see cref="IPartyIntents"/> over <see cref="IPartySender"/>: each
    /// request stamps a fresh operation UUID v4 (client op ids, ids.md)
    /// and sends the wire payload — never adjudicates locally.
    /// </summary>
    public sealed class PartyIntents : IPartyIntents
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SPartyInvite = 602;
        public const uint C2SPartyAccept = 604;
        public const uint C2SPartyLeave = 605;
        public const uint C2SPartyKick = 606;
        public const uint C2SPartyDecline = 620;
        public const uint C2SPartyInviteCancel = 621;
        public const uint C2SPartyLeaderTransfer = 622;
        public const uint C2SPartyBoardPost = 634;
        public const uint C2SPartyBoardCancel = 635;

        private readonly IPartySender _sender;

        public PartyIntents(IPartySender sender)
        {
            _sender = sender;
        }

        public async Awaitable<ulong> RequestInvite(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SPartyInvite
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SPartyInvite, req, cancel);
        }

        public async Awaitable<ulong> RequestAccept(
            byte[] partyId, byte[] inviterCharacterId,
            CancellationToken cancel)
        {
            var req = new C2SPartyAccept
            {
                OperationId = OpId(),
                PartyId = ByteString.CopyFrom(partyId),
                InviterCharacterId = ByteString.CopyFrom(inviterCharacterId),
            };
            return await _sender.SendAsync(C2SPartyAccept, req, cancel);
        }

        public async Awaitable<ulong> RequestDecline(
            byte[] partyId, byte[] inviterCharacterId,
            CancellationToken cancel)
        {
            var req = new C2SPartyDecline
            {
                OperationId = OpId(),
                PartyId = ByteString.CopyFrom(partyId),
                InviterCharacterId = ByteString.CopyFrom(inviterCharacterId),
            };
            return await _sender.SendAsync(C2SPartyDecline, req, cancel);
        }

        public async Awaitable<ulong> RequestLeave(CancellationToken cancel)
        {
            var req = new C2SPartyLeave
            {
                OperationId = OpId(),
            };
            return await _sender.SendAsync(C2SPartyLeave, req, cancel);
        }

        public async Awaitable<ulong> RequestKick(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SPartyKick
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SPartyKick, req, cancel);
        }

        public async Awaitable<ulong> RequestInviteCancel(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SPartyInviteCancel
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SPartyInviteCancel, req, cancel);
        }

        public async Awaitable<ulong> RequestLeaderTransfer(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SPartyLeaderTransfer
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            };
            return await _sender.SendAsync(C2SPartyLeaderTransfer, req, cancel);
        }

        public async Awaitable<ulong> RequestBoardPost(
            string dungeonId, uint desiredSize, string note,
            CancellationToken cancel)
        {
            var req = new C2SPartyBoardPost
            {
                OperationId = OpId(),
                DungeonId = dungeonId,
                DesiredSize = desiredSize,
                Note = note ?? string.Empty,
            };
            return await _sender.SendAsync(C2SPartyBoardPost, req, cancel);
        }

        public async Awaitable<ulong> RequestBoardCancel(
            CancellationToken cancel)
        {
            var req = new C2SPartyBoardCancel
            {
                OperationId = OpId(),
            };
            return await _sender.SendAsync(C2SPartyBoardCancel, req, cancel);
        }

        private static ByteString OpId()
        {
            return ByteString.CopyFrom(Guid.NewGuid().ToByteArray());
        }
    }
}
