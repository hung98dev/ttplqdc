using ThinhThan.Core.Runtime;

namespace ThinhThan.Systems.Effects
{
    /// <summary>
    /// Pooled presenter objects for status icons, duration bars and
    /// damage-number cues. Allocation-free in steady state through the
    /// single first-party Pool&lt;T&gt; (engineering_conventions.md
    /// §2.6). These are pure view-model presenters — the UI layer
    /// binds them; nothing here touches UnityEngine or allocates per
    /// frame.
    /// </summary>
    public sealed class EffectPresenters
    {
        /// <summary>Icon presenter: one live status's icon view.</summary>
        public sealed class IconPresenter
        {
            /// <summary>Entity the icon is attached to.</summary>
            public ulong EntityId;
            /// <summary>Canonical template id (atlas key).</summary>
            public string EffectId = string.Empty;
            /// <summary>Presentation group (control/slow/dot/buff/debuff).</summary>
            public string StatusKind = string.Empty;
            /// <summary>Stack count badge (0 = hidden).</summary>
            public uint Stacks;
            /// <summary>Authoritative expiry tick.</summary>
            public ulong ExpiresAtTick;

            /// <summary>Resets for reuse.</summary>
            public void Reset()
            {
                EntityId = 0;
                EffectId = string.Empty;
                StatusKind = string.Empty;
                Stacks = 0;
                ExpiresAtTick = 0;
            }
        }

        /// <summary>Duration-bar presenter driven off an icon.</summary>
        public sealed class BarPresenter
        {
            /// <summary>Entity the bar is attached to.</summary>
            public ulong EntityId;
            /// <summary>Canonical template id.</summary>
            public string EffectId = string.Empty;
            /// <summary>Remaining ticks at last Evaluate.</summary>
            public ulong RemainingTicks;
            /// <summary>Authoritative expiry tick.</summary>
            public ulong ExpiresAtTick;

            /// <summary>Resets for reuse.</summary>
            public void Reset()
            {
                EntityId = 0;
                EffectId = string.Empty;
                RemainingTicks = 0;
                ExpiresAtTick = 0;
            }
        }

        /// <summary>Damage/heal-number cue presenter.</summary>
        public sealed class NumberPresenter
        {
            /// <summary>Entity the number floats over.</summary>
            public ulong EntityId;
            /// <summary>Signed amount (heal positive when caller
            /// presents it that way).</summary>
            public long Amount;
            /// <summary>Damage element (KIM/MOC/HOA/THUY/THO) for tint.</summary>
            public string Element = string.Empty;

            /// <summary>Resets for reuse.</summary>
            public void Reset()
            {
                EntityId = 0;
                Amount = 0;
                Element = string.Empty;
            }
        }

        private readonly Pool<IconPresenter> _icons =
            new Pool<IconPresenter>(() => new IconPresenter(), p => p.Reset());
        private readonly Pool<BarPresenter> _bars =
            new Pool<BarPresenter>(() => new BarPresenter(), p => p.Reset());
        private readonly Pool<NumberPresenter> _numbers =
            new Pool<NumberPresenter>(() => new NumberPresenter(), p => p.Reset());

        /// <summary>Prewarms the three presenter pools.</summary>
        public void Prewarm(int icons, int bars, int numbers)
        {
            _icons.Prewarm(icons);
            _bars.Prewarm(bars);
            _numbers.Prewarm(numbers);
        }

        /// <summary>Rents an icon presenter.</summary>
        public IconPresenter RentIcon()
        {
            return _icons.Rent();
        }

        /// <summary>Returns an icon presenter.</summary>
        public void ReturnIcon(IconPresenter p)
        {
            _icons.Return(p);
        }

        /// <summary>Rents a bar presenter.</summary>
        public BarPresenter RentBar()
        {
            return _bars.Rent();
        }

        /// <summary>Returns a bar presenter.</summary>
        public void ReturnBar(BarPresenter p)
        {
            _bars.Return(p);
        }

        /// <summary>Rents a number presenter.</summary>
        public NumberPresenter RentNumber()
        {
            return _numbers.Rent();
        }

        /// <summary>Returns a number presenter.</summary>
        public void ReturnNumber(NumberPresenter p)
        {
            _numbers.Return(p);
        }

        /// <summary>Live icon presenters (debug/metrics).</summary>
        public int LiveIcons
        {
            get
            {
                return _icons.LiveCount;
            }
        }
    }
}
