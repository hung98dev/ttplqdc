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

        public Awaitable<ulong> RequestInvite(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            return _sender.SendAsync(C2SPartyInvite, new C2SPartyInvite
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            }, cancel);
        }

        public Awaitable<ulong> RequestAccept(
            byte[] partyId, byte[] inviterCharacterId,
            CancellationToken cancel)
        {
            return _sender.SendAsync(C2SPartyAccept, new C2SPartyAccept
            {
                OperationId = OpId(),
                PartyId = ByteString.CopyFrom(partyId),
                InviterCharacterId = ByteString.CopyFrom(inviterCharacterId),
            }, cancel);
        }

        public Awaitable<ulong> RequestDecline(
            byte[] partyId, byte[] inviterCharacterId,
            CancellationToken cancel)
        {
            return _sender.SendAsync(C2SPartyDecline, new C2SPartyDecline
            {
                OperationId = OpId(),
                PartyId = ByteString.CopyFrom(partyId),
                InviterCharacterId = ByteString.CopyFrom(inviterCharacterId),
            }, cancel);
        }

        public Awaitable<ulong> RequestLeave(CancellationToken cancel)
        {
            return _sender.SendAsync(C2SPartyLeave, new C2SPartyLeave
            {
                OperationId = OpId(),
            }, cancel);
        }

        public Awaitable<ulong> RequestKick(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            return _sender.SendAsync(C2SPartyKick, new C2SPartyKick
            {
                OperationId = OpId(),
                TargetCharacterId = ByteString.CopyFrom(targetCharacterId),
            }, cancel);
        }

        public Awaitable<ulong> RequestInviteCancel(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            return _sender.SendAsync(
                C2SPartyInviteCancel, new C2SPartyInviteCancel
                {
                    OperationId = OpId(),
                    TargetCharacterId =
                        ByteString.CopyFrom(targetCharacterId),
                }, cancel);
        }

        public Awaitable<ulong> RequestLeaderTransfer(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            return _sender.SendAsync(
                C2SPartyLeaderTransfer, new C2SPartyLeaderTransfer
                {
                    OperationId = OpId(),
                    TargetCharacterId =
                        ByteString.CopyFrom(targetCharacterId),
                }, cancel);
        }

        public Awaitable<ulong> RequestBoardPost(
            string dungeonId, uint desiredSize, string note,
            CancellationToken cancel)
        {
            return _sender.SendAsync(
                C2SPartyBoardPost, new C2SPartyBoardPost
                {
                    OperationId = OpId(),
                    DungeonId = dungeonId,
                    DesiredSize = desiredSize,
                    Note = note ?? string.Empty,
                }, cancel);
        }

        public Awaitable<ulong> RequestBoardCancel(CancellationToken cancel)
        {
            return _sender.SendAsync(
                C2SPartyBoardCancel, new C2SPartyBoardCancel
                {
                    OperationId = OpId(),
                }, cancel);
        }

        private static ByteString OpId()
        {
            return ByteString.CopyFrom(Guid.NewGuid().ToByteArray());
        }
    }
}
