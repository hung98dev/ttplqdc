using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// Channel-switch UI state driven by 109 submit + 110
    /// S2C_CHANNEL_SWITCH_RESULT (messages.md). The 10 s switch cooldown
    /// lives server-side; the client only mirrors the requested target and
    /// the recorded outcome (including BACKOFF retry_after_ms).
    /// </summary>
    public sealed class ChannelSwitchPresentation
    {
        /// <summary>Switch state.</summary>
        public enum State
        {
            /// <summary>No request in flight.</summary>
            Idle = 0,

            /// <summary>109 sent; waiting on the recorded 110.</summary>
            Pending = 1,

            /// <summary>110 SUCCESS — switch accepted.</summary>
            Succeeded = 2,

            /// <summary>110 ERROR — carries retry_after_ms when BACKOFF.</summary>
            Failed = 3,
        }

        /// <summary>Current switch state.</summary>
        public State Current
        {
            get;
            private set;
        }

        /// <summary>Channel index the in-flight/last request targeted.</summary>
        public uint TargetChannelIndex
        {
            get;
            private set;
        }

        /// <summary>retry_after_ms from a failed result (0 = retry now).</summary>
        public uint RetryAfterMs
        {
            get;
            private set;
        }

        /// <summary>error_code from a failed result.</summary>
        public ErrorCode FailureCode
        {
            get;
            private set;
        }

        /// <summary>109 submitted for a target channel.</summary>
        public void BeginRequest(uint targetChannelIndex)
        {
            Current = State.Pending;
            TargetChannelIndex = targetChannelIndex;
            RetryAfterMs = 0;
            FailureCode = ErrorCode.Unspecified;
        }

        /// <summary>Applies the recorded 110.</summary>
        public void Apply(S2CChannelSwitchResult result)
        {
            if (result.Result.Status == ResultStatus.Success)
            {
                Current = State.Succeeded;
                TargetChannelIndex = result.TargetChannelIndex;
                RetryAfterMs = 0;
            }
            else
            {
                Current = State.Failed;
                RetryAfterMs = result.RetryAfterMs;
                FailureCode = result.Result.ErrorCode;
            }
        }

        /// <summary>Returns to idle (UI dismissed / new request cycle).</summary>
        public void Clear()
        {
            Current = State.Idle;
            TargetChannelIndex = 0;
            RetryAfterMs = 0;
            FailureCode = ErrorCode.Unspecified;
        }
    }
}
