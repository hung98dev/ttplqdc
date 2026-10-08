using System.Collections.Generic;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// IProgressionSink dispatch for the 514/515 pair (messages.md §
    /// Progression): 515 is REPLACEABLE_STATE — it replaces the
    /// authoritative projection; 514 is the per-operation verdict feed
    /// keyed by (request_message_id, operation_id) for the UI's
    /// result line.
    /// </summary>
    public sealed class ProgressionApplier : IProgressionSink
    {
        /// <summary>Retained verdict depth — the panel needs only the
        /// tail.</summary>
        public const int ResultCapacity = 8;

        /// <summary>One committed mutation verdict (514).</summary>
        public readonly struct Result
        {
            public readonly uint RequestMessageId;
            public readonly ByteStringKey OperationId;
            public readonly ResultStatus Status;
            public readonly ErrorCode Error;

            public Result(uint requestMessageId, ByteStringKey operationId,
                ResultStatus status, ErrorCode error)
            {
                RequestMessageId = requestMessageId;
                OperationId = operationId;
                Status = status;
                Error = error;
            }
        }

        /// <summary>16-byte op id wrapper so the ring compares by value.</summary>
        public readonly struct ByteStringKey : System.IEquatable<ByteStringKey>
        {
            public readonly Google.Protobuf.ByteString Bytes;

            public ByteStringKey(Google.Protobuf.ByteString bytes)
            {
                Bytes = bytes;
            }

            public bool Equals(ByteStringKey other)
            {
                return Bytes.Equals(other.Bytes);
            }

            public override bool Equals(object? obj)
            {
                return obj is ByteStringKey k && Equals(k);
            }

            public override int GetHashCode()
            {
                return Bytes.GetHashCode();
            }
        }

        private readonly ProgressionState _state = new ProgressionState();
        private readonly List<Result> _results = new List<Result>(ResultCapacity);

        public ProgressionState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Newest-first verdict tail (≤ ResultCapacity).</summary>
        public IReadOnlyList<Result> Results
        {
            get
            {
                return _results;
            }
        }

        /// <summary>Bumped on every 514/515 — presentation reads at most
        /// once per frame.</summary>
        public ulong ResultRevision
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case WireIds.S2CProgressionState:
                    // Payload!: the decoder always fills it for this id.
                    _state.Apply((S2CProgressionState)frame.Payload!);
                    ResultRevision++;
                    break;
                case WireIds.S2CProgressionMutateResult:
                    // Payload!: the decoder always fills it for this id.
                    S2CProgressionMutateResult r =
                        (S2CProgressionMutateResult)frame.Payload!;
                    OperationResult op = r.Result;
                    _results.Insert(0, new Result(
                        r.RequestMessageId,
                        new ByteStringKey(op != null ? op.OperationId
                            : Google.Protobuf.ByteString.Empty),
                        op != null ? op.Status : ResultStatus.Unspecified,
                        op != null ? op.ErrorCode : ErrorCode.Unspecified));
                    if (_results.Count > ResultCapacity)
                    {
                        _results.RemoveAt(_results.Count - 1);
                    }
                    ResultRevision++;
                    break;
            }
        }
    }
}
