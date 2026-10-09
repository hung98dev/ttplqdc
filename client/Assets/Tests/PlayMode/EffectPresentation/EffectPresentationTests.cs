using NUnit.Framework;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Effects;

namespace ThinhThan.Tests.PlayMode.EffectPresentation
{
    /// <summary>
    /// IMP-016 PlayMode: status-effect presentation — EffectState
    /// consumes S2C_STATUS_EVENT (205) deltas and EntityState baseline
    /// statuses; EffectPresentation reconciles pooled icon/bar
    /// presenters inside the FrameLoop Evaluate step. The client
    /// presents the authoritative stream — never predicts.
    /// </summary>
    public sealed class EffectPresentationTests
    {
        private const ulong Self = 0x100001;
        private const ulong Foe = 0x200001;

        private static S2CStatusEvent Event(ulong target, string effectId,
            StatusEventKind kind, uint stacks, ulong expires)
        {
            return new S2CStatusEvent
            {
                TargetEntityId = target,
                SourceEntityId = Self,
                EffectId = effectId,
                StatusKind = "debuff",
                Event = kind,
                Stacks = stacks,
                ExpiresAtTick = expires,
                ServerTick = 10,
            };
        }

        /// <summary>APPLIED upserts an entry and bumps the model
        /// version; EXPIRED removes it.</summary>
        [Test]
        public void StatusEventsDriveModel()
        {
            var state = new EffectState();
            Assert.AreEqual(0, state.Count(Foe));

            state.ApplyEvent(Event(Foe, "effect.basic.slow_20_3s",
                StatusEventKind.Applied, 1, 70));
            Assert.AreEqual(1, state.Count(Foe));
            var entries = state.Entries(Foe);
            Assert.AreEqual("effect.basic.slow_20_3s", entries[0].EffectId);
            Assert.AreEqual(70ul, entries[0].ExpiresAtTick);

            state.ApplyEvent(Event(Foe, "effect.basic.slow_20_3s",
                StatusEventKind.StackChanged, 2, 80));
            entries = state.Entries(Foe);
            Assert.AreEqual(2u, entries[0].Stacks);

            state.ApplyEvent(Event(Foe, "effect.basic.slow_20_3s",
                StatusEventKind.Expired, 0, 100));
            Assert.AreEqual(0, state.Count(Foe));
        }

        /// <summary>Baseline snapshot replaces the entity's model.</summary>
        [Test]
        public void BaselineSnapshotReplacesModel()
        {
            var state = new EffectState();
            state.ApplyEvent(Event(Foe, "effect.basic.stun_400ms",
                StatusEventKind.Applied, 1, 20));

            var snap = new EntityState { EntityId = Foe };
            snap.Statuses.Add(new EntityStatus
            {
                EffectId = "effect.basic.slow_15_2s",
                SourceEntityId = Self,
                Stacks = 1,
                ExpiresAtTick = 40,
            });
            snap.Statuses.Add(new EntityStatus
            {
                EffectId = "effect.basic.burn_3s",
                SourceEntityId = Self,
                Stacks = 1,
                ExpiresAtTick = 60,
            });
            state.SyncSnapshot(snap);

            Assert.AreEqual(2, state.Count(Foe));
            var ids = state.Entries(Foe);
            Assert.AreEqual("effect.basic.burn_3s", ids[0].EffectId);
            Assert.AreEqual("effect.basic.slow_15_2s", ids[1].EffectId);
        }

        /// <summary>Presentation reconciles pooled icon + bar
        /// presenters: show on apply, refresh on stack change, remove
        /// on expiry — pooled objects are reused.</summary>
        [Test]
        public void PresentationReconcilesPooledPresenters()
        {
            var state = new EffectState();
            var pools = new EffectPresenters();
            var pres = new global::ThinhThan.Systems.Effects.EffectPresentation(state, pools);

            state.ApplyEvent(Event(Foe, "effect.basic.burn_3s",
                StatusEventKind.Applied, 1, 60));
            state.ApplyEvent(Event(Foe, "effect.basic.slow_20_3s",
                StatusEventKind.Applied, 1, 70));
            pres.NotifyEntity(Foe, 10);

            Assert.AreEqual(2, pres.IconCount(Foe));
            Assert.AreEqual(2, pres.BarCount(Foe));
            Assert.AreEqual(2, pres.Cues.Count);
            Assert.AreEqual(ThinhThan.Systems.Effects.EffectPresentation.CueKind.IconShown,
                pres.Cues[0].Kind);

            // Stack change refreshes, not re-shows.
            state.ApplyEvent(Event(Foe, "effect.basic.burn_3s",
                StatusEventKind.StackChanged, 2, 65));
            pres.NotifyEntity(Foe, 10);
            Assert.AreEqual(2, pres.IconCount(Foe));
            Assert.AreEqual(1, pres.Cues.Count);
            Assert.AreEqual(ThinhThan.Systems.Effects.EffectPresentation.CueKind.IconRefreshed,
                pres.Cues[0].Kind);

            // Expiry removes the icon + bar and returns them to pool.
            state.ApplyEvent(Event(Foe, "effect.basic.burn_3s",
                StatusEventKind.Expired, 0, 60));
            pres.NotifyEntity(Foe, 60);
            Assert.AreEqual(1, pres.IconCount(Foe));
            Assert.AreEqual(1, pres.BarCount(Foe));

            // Re-application reuses the pooled presenter.
            state.ApplyEvent(Event(Foe, "effect.basic.stun_500ms",
                StatusEventKind.Applied, 1, 80));
            pres.NotifyEntity(Foe, 60);
            Assert.AreEqual(2, pres.IconCount(Foe));
            Assert.AreEqual(2, pools.LiveIcons);
        }

        /// <summary>FrameLoop Evaluate updates bar remaining ticks
        /// from the authoritative tick.</summary>
        [Test]
        public void EvaluateAdvancesBarsFromAuthoritativeTick()
        {
            var state = new EffectState();
            var pools = new EffectPresenters();
            var pres = new global::ThinhThan.Systems.Effects.EffectPresentation(state, pools);

            state.ApplyEvent(Event(Foe, "effect.basic.slow_20_3s",
                StatusEventKind.Applied, 1, 100));
            pres.NotifyEntity(Foe, 40);
            pres.Evaluate(40);
            pres.Evaluate(60);

            var entries = state.Entries(Foe);
            Assert.AreEqual(1, entries.Count);
            Assert.AreEqual(100ul, entries[0].ExpiresAtTick);
            Assert.AreEqual(1, pres.BarCount(Foe));
        }
    }
}
