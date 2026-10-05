using System;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using ThinhThan.Core.Session;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Net
{
    /// <summary>
    /// Character lifecycle client path (IMP-065): create (12 -> 13 + 14),
    /// attach (6 -> 7), detach (10 -> 11), list (14) — thin coordinator over
    /// <see cref="SessionOrchestrator"/> that tracks per-operation results
    /// and surfaces them to UI. Lives in Net/: Task-based async is legal
    /// only inside the transport layer (engineering_conventions.md §2.5).
    /// </summary>
    public sealed class CharacterSessionController
    {
        public const int WaitTimeoutMs = 10000;

        private readonly SessionOrchestrator _orchestrator;
        private int _nextOperationSeed;

        public CharacterSessionController(SessionOrchestrator orchestrator)
        {
            _orchestrator = orchestrator ??
                throw new ArgumentNullException(nameof(orchestrator));
        }

        /// <summary>Latest pushed character list.</summary>
        public S2CCharacterList? CurrentList
        {
            get
            {
                return _orchestrator.CharacterList;
            }
        }

        /// <summary>CREATE_RESULT of the most recent create request.</summary>
        public S2CCharacterCreateResult? LastCreateResult
        {
            get
            {
                return _orchestrator.LastCreateResult;
            }
        }

        /// <summary>
        /// Sends C2S_CHARACTER_CREATE and awaits its result (13) plus the
        /// follow-up list push (14).
        /// </summary>
        public async Task<Result<S2CCharacterCreateResult>> CreateAsync(
            string characterName, string classId, CancellationToken cancel)
        {
            byte[] operationId = NewOperationId();
            Task<DecodedFrame?> wait = _orchestrator.WaitForAsync(
                WireIds.S2CCharacterCreateResult,
                frame =>
                    // Payload!: the decoder always fills it for this id.
                    ((S2CCharacterCreateResult)frame.Payload!).Result
                        .OperationId.SequenceEqual(operationId),
                WaitTimeoutMs, cancel);
            Result<bool> sent = await _orchestrator
                .CreateCharacterAsync(operationId, characterName, classId, cancel)
                .ConfigureAwait(false);
            if (!sent.Ok)
            {
                return Result<S2CCharacterCreateResult>.Failure(sent.ErrorCode);
            }

            DecodedFrame? frame = await wait.ConfigureAwait(false);
            if (frame == null)
            {
                return Result<S2CCharacterCreateResult>.Failure(
                    "TEMPORARY_DEPENDENCY_FAILURE");
            }

            // Payload!: the decoder always fills it for this id.
            return Result<S2CCharacterCreateResult>.Success(
                (S2CCharacterCreateResult)frame.Payload!);
        }

        /// <summary>Sends C2S_CHARACTER_ATTACH and awaits ATTACH_OK.</summary>
        public async Task<Result<S2CCharacterAttachOk>> AttachAsync(
            byte[] characterId, CancellationToken cancel)
        {
            Task<DecodedFrame?> wait = _orchestrator.WaitForAsync(
                WireIds.S2CCharacterAttachOk, null, WaitTimeoutMs, cancel);
            Result<bool> sent = await _orchestrator
                .AttachAsync(characterId, cancel).ConfigureAwait(false);
            if (!sent.Ok)
            {
                return Result<S2CCharacterAttachOk>.Failure(sent.ErrorCode);
            }

            DecodedFrame? frame = await wait.ConfigureAwait(false);
            if (frame == null)
            {
                return Result<S2CCharacterAttachOk>.Failure(
                    "TEMPORARY_DEPENDENCY_FAILURE");
            }

            // Payload!: the decoder always fills it for this id.
            return Result<S2CCharacterAttachOk>.Success(
                (S2CCharacterAttachOk)frame.Payload!);
        }

        /// <summary>Sends C2S_CHARACTER_DETACH and awaits DETACH_OK.</summary>
        public async Task<Result<S2CCharacterDetachOk>> DetachAsync(
            CancellationToken cancel)
        {
            Task<DecodedFrame?> wait = _orchestrator.WaitForAsync(
                WireIds.S2CCharacterDetachOk, null, WaitTimeoutMs, cancel);
            Result<bool> sent =
                await _orchestrator.DetachAsync(cancel).ConfigureAwait(false);
            if (!sent.Ok)
            {
                return Result<S2CCharacterDetachOk>.Failure(sent.ErrorCode);
            }

            DecodedFrame? frame = await wait.ConfigureAwait(false);
            if (frame == null)
            {
                return Result<S2CCharacterDetachOk>.Failure(
                    "TEMPORARY_DEPENDENCY_FAILURE");
            }

            // Payload!: the decoder always fills it for this id.
            return Result<S2CCharacterDetachOk>.Success(
                (S2CCharacterDetachOk)frame.Payload!);
        }

        /// <summary>operation_id = UUIDv7 (ids.md) as 16 bytes.</summary>
        private byte[] NewOperationId()
        {
            long unixMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
            int seed = Interlocked.Increment(ref _nextOperationSeed);
            var bytes = new byte[16];
            bytes[0] = (byte)(unixMs >> 40);
            bytes[1] = (byte)(unixMs >> 32);
            bytes[2] = (byte)(unixMs >> 24);
            bytes[3] = (byte)(unixMs >> 16);
            bytes[4] = (byte)(unixMs >> 8);
            bytes[5] = (byte)unixMs;
            bytes[6] = (byte)(0x70 | (seed >> 8 & 0x0F));
            bytes[7] = (byte)(seed & 0xFF);
            Guid tail = Guid.NewGuid();
            byte[] tailBytes = tail.ToByteArray();
            for (int i = 8; i < 16; i++)
            {
                bytes[i] = tailBytes[i - 8];
            }

            bytes[8] = (byte)(bytes[8] & 0x3F | 0x80);
            return bytes;
        }
    }
}
