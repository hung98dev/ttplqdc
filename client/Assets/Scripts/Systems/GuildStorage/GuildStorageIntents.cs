using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.GuildStorage
{
    /// <summary>
    /// Guild-storage intent sender: builds each C2S request with a
    /// fresh operation_id and hands it to the send seam. Wire ids
    /// follow messages.md; every request commits exactly one 649
    /// verdict.
    /// </summary>
    public sealed class GuildStorageIntents : IGuildStorageIntents
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SGuildStorageDeposit = 629;
        public const uint C2SGuildStorageWithdraw = 630;
        public const uint C2SGuildStorageMove = 644;
        public const uint C2SGuildStorageClaimRequest = 645;
        public const uint C2SGuildStorageClaimDecide = 646;

        private readonly IGuildStorageSender _sender;

        public GuildStorageIntents(IGuildStorageSender sender)
        {
            _sender = sender;
        }

        public async Awaitable<ulong> RequestDeposit(
            ByteString itemInstanceId, uint quantity,
            GuildStorageSection section, CancellationToken cancel)
        {
            var req = new C2SGuildStorageDeposit
            {
                OperationId = OpId(),
                ItemInstanceId = itemInstanceId,
                Quantity = quantity,
                Section = section,
            };
            return await _sender.SendAsync(C2SGuildStorageDeposit, req, cancel);
        }

        public async Awaitable<ulong> RequestWithdraw(
            ByteString itemInstanceId, uint quantity,
            GuildStorageSection section, CancellationToken cancel)
        {
            var req = new C2SGuildStorageWithdraw
            {
                OperationId = OpId(),
                ItemInstanceId = itemInstanceId,
                Quantity = quantity,
                Section = section,
            };
            return await _sender.SendAsync(C2SGuildStorageWithdraw, req, cancel);
        }

        public async Awaitable<ulong> RequestMove(
            ByteString itemInstanceId, GuildStorageSection toSection,
            CancellationToken cancel)
        {
            var req = new C2SGuildStorageMove
            {
                OperationId = OpId(),
                ItemInstanceId = itemInstanceId,
                ToSection = toSection,
            };
            return await _sender.SendAsync(C2SGuildStorageMove, req, cancel);
        }

        public async Awaitable<ulong> RequestClaim(
            ByteString itemInstanceId, uint quantity, CancellationToken cancel)
        {
            var req = new C2SGuildStorageClaimRequest
            {
                OperationId = OpId(),
                ItemInstanceId = itemInstanceId,
                Quantity = quantity,
            };
            return await _sender.SendAsync(C2SGuildStorageClaimRequest, req, cancel);
        }

        public async Awaitable<ulong> RequestClaimDecide(
            ByteString claimId, GuildStorageClaimDecision decision,
            CancellationToken cancel)
        {
            var req = new C2SGuildStorageClaimDecide
            {
                OperationId = OpId(),
                ClaimId = claimId,
                Decision = decision,
            };
            return await _sender.SendAsync(C2SGuildStorageClaimDecide, req, cancel);
        }

        private static ByteString OpId()
        {
            return ByteString.CopyFrom(Guid.NewGuid().ToByteArray());
        }
    }
}
