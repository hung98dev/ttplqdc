using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Social
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class SocialIntents : ISocialIntents
    {
        private readonly ISocialSender _sender;
        private readonly Func<byte[]> _mint;

        public SocialIntents(ISocialSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> SendChat(ChatChannel channel,
            byte[] targetCharacterId, string text,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SChatSend
            {
                OperationId = ByteString.CopyFrom(operationId),
                Channel = channel,
                MessageText = text ?? "",
            };
            if (targetCharacterId != null && targetCharacterId.Length > 0)
            {
                req.TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId);
            }
            await _sender.SendAsync(
                SocialApplier.C2SChatSendId, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestFriend(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SFriendRequest
            {
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
            };
            return await Send(611, req, cancel);
        }

        public async Awaitable<byte[]> AcceptFriend(
            byte[] requesterCharacterId, CancellationToken cancel)
        {
            var req = new C2SFriendAccept
            {
                RequesterCharacterId = ByteString.CopyFrom(
                    requesterCharacterId ?? new byte[0]),
            };
            return await Send(613, req, cancel);
        }

        public async Awaitable<byte[]> DeclineFriend(
            byte[] requesterCharacterId, CancellationToken cancel)
        {
            var req = new C2SFriendDecline
            {
                RequesterCharacterId = ByteString.CopyFrom(
                    requesterCharacterId ?? new byte[0]),
            };
            return await Send(614, req, cancel);
        }

        public async Awaitable<byte[]> RemoveFriend(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SFriendRemove
            {
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
            };
            return await Send(615, req, cancel);
        }

        public async Awaitable<byte[]> AddBlock(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SBlockAdd
            {
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
            };
            return await Send(617, req, cancel);
        }

        public async Awaitable<byte[]> RemoveBlock(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            var req = new C2SBlockRemove
            {
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
            };
            return await Send(618, req, cancel);
        }

        public async Awaitable<byte[]> ReportPlayer(
            byte[] targetCharacterId, ReportReason reason,
            byte[] chatMessageId, string notes,
            CancellationToken cancel)
        {
            var req = new C2SReportPlayer
            {
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
                Reason = reason,
            };
            if (chatMessageId != null && chatMessageId.Length > 0)
            {
                req.ChatMessageId = ByteString.CopyFrom(chatMessageId);
            }
            if (!string.IsNullOrEmpty(notes))
            {
                req.ReporterNotes = notes;
            }
            return await Send(632, req, cancel);
        }

        private async Awaitable<byte[]> Send(uint messageId,
            IMessage req, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            SetOperationId(req, operationId);
            await _sender.SendAsync(messageId, req, cancel);
            return operationId;
        }

        private static void SetOperationId(IMessage req, byte[] operationId)
        {
            ByteString bs = ByteString.CopyFrom(operationId);
            switch (req)
            {
                case C2SFriendRequest r:
                    r.OperationId = bs;
                    break;
                case C2SFriendAccept r:
                    r.OperationId = bs;
                    break;
                case C2SFriendDecline r:
                    r.OperationId = bs;
                    break;
                case C2SFriendRemove r:
                    r.OperationId = bs;
                    break;
                case C2SBlockAdd r:
                    r.OperationId = bs;
                    break;
                case C2SBlockRemove r:
                    r.OperationId = bs;
                    break;
                case C2SReportPlayer r:
                    r.OperationId = bs;
                    break;
            }
        }

        private static byte[] MintOperationId()
        {
            var b = new byte[16];
            Guid.NewGuid().TryWriteBytes(b);
            return b;
        }
    }
}
