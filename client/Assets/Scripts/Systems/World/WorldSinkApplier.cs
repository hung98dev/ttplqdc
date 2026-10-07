using System;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// The <see cref="IWorldSink"/> the session driver forwards
    /// world-scoped inbound frames to (110 / 116 / 204): the recorded
    /// channel-switch result goes to <see cref="ChannelSwitchPresentation"/>,
    /// the interact result and action rejection go to the action sinks
    /// composition wires in (composition feeds
    /// <c>ICombatEventFeed.InteractResult</c> / <c>ActionRejected</c>).
    /// </summary>
    public sealed class WorldSinkApplier : IWorldSink
    {
        /// <summary>S2C_CHANNEL_SWITCH_RESULT.</summary>
        public const uint MessageChannelSwitchResult = 110;

        /// <summary>S2C_INTERACT_RESULT.</summary>
        public const uint MessageInteractResult = 116;

        /// <summary>S2C_ACTION_REJECTED.</summary>
        public const uint MessageActionRejected = 204;

        private readonly ChannelSwitchPresentation _channelSwitch;
        private readonly Action<S2CInteractResult> _interactResult;
        private readonly Action<S2CActionRejected> _actionRejected;

        /// <summary>
        /// channelSwitch — required; interactResult / actionRejected — optional
        /// composition sinks (may be null before binding).
        /// </summary>
        public WorldSinkApplier(
            ChannelSwitchPresentation channelSwitch,
            Action<S2CInteractResult>? interactResult = null,
            Action<S2CActionRejected>? actionRejected = null)
        {
            _channelSwitch = channelSwitch ?? throw new ArgumentNullException(nameof(channelSwitch));
            _interactResult = interactResult ?? (_ => { });
            _actionRejected = actionRejected ?? (_ => { });
        }

        /// <summary>Dispatches a decoded world frame to its surface.</summary>
        public void Apply(DecodedFrame frame)
        {
            if (frame?.Payload == null)
            {
                return;
            }
            switch (frame.MessageId)
            {
                case MessageChannelSwitchResult:
                    if (frame.Payload is S2CChannelSwitchResult channelSwitch)
                    {
                        _channelSwitch.Apply(channelSwitch);
                    }
                    break;
                case MessageInteractResult:
                    if (frame.Payload is S2CInteractResult interactResult)
                    {
                        _interactResult(interactResult);
                    }
                    break;
                case MessageActionRejected:
                    if (frame.Payload is S2CActionRejected rejected)
                    {
                        _actionRejected(rejected);
                    }
                    break;
            }
        }
    }
}
