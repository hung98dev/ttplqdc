using System;
using ThinhThan.Core.Runtime;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Sole HUD write path (PERF-022): one <see cref="FramePhase.UI"/> tick
    /// polls the four trackers into <see cref="HudDataModel"/> dirty flags,
    /// then applies the dirty sections to the view at most once per frame —
    /// a HUD value change never rebuilds static layout.
    /// </summary>
    public sealed class CoreHudSystem : IFrameSystem
    {
        private readonly VitalsTracker _vitals;
        private readonly TargetTracker _target;
        private readonly StatusTracker _status;
        private readonly CooldownTracker _cooldown;
        private readonly HudDataModel _model;
        private readonly CoreHudView _view;
        private readonly IWorldContextFeed? _world;
        private readonly ILatencyProbe? _latency;
        private readonly Func<ulong> _serverTick;

        /// <summary>
        /// <paramref name="serverTick"/> supplies the authoritative server
        /// tick for cooldown fill + expiry (composition wires it to the
        /// baseline tracker); null → cooldowns never fill/expire.
        /// </summary>
        public CoreHudSystem(
            VitalsTracker vitals,
            TargetTracker target,
            StatusTracker status,
            CooldownTracker cooldown,
            HudDataModel model,
            CoreHudView view,
            IWorldContextFeed? world = null,
            ILatencyProbe? latency = null,
            Func<ulong>? serverTick = null)
        {
            _vitals = vitals ??
                throw new ArgumentNullException(nameof(vitals));
            _target = target ??
                throw new ArgumentNullException(nameof(target));
            _status = status ??
                throw new ArgumentNullException(nameof(status));
            _cooldown = cooldown ??
                throw new ArgumentNullException(nameof(cooldown));
            _model = model ?? throw new ArgumentNullException(nameof(model));
            _view = view ?? throw new ArgumentNullException(nameof(view));
            _world = world;
            _latency = latency;
            _serverTick = serverTick ?? (() => 0UL);
        }

        /// <summary>Whether the last tick produced a view apply (tests).</summary>
        public bool AppliedThisTick
        {
            get;
            private set;
        }

        public void Tick(in FrameTime time)
        {
            ulong tick = _serverTick();
            _vitals.Refresh();
            _target.Refresh();
            _status.Refresh();
            _cooldown.Refresh(tick);

            if (_world != null || _latency != null)
            {
                _model.SetWorldContext(
                    _world?.DisplayName, _world?.ChannelIndex ?? 0U,
                    _latency?.RttMs ?? -1);
            }

            AppliedThisTick = false;
            if (_model.Dirty != HudDirty.None)
            {
                _view.Apply(_model, tick);
                _model.ClearDirty();
                AppliedThisTick = true;
            }
        }
    }
}
