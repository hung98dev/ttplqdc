using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Combat;
using CombatPres = ThinhThan.Systems.Combat.CombatPresentation;

namespace ThinhThan.Tests.PlayMode.CombatPresentation
{
    /// <summary>
    /// IMP-014 PlayMode: authoritative action start/reject/interrupt/
    /// death presentation over Systems/Combat. The client presents — it
    /// never predicts acceptance (combat.md authority boundary).
    /// </summary>
    public sealed class CombatPresentationTests
    {
        private const ulong Self = 0x100001;
        private const ulong Foe = 0x200001;

        private static DecodedFrame Frame(uint id, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = payload,
            };
        }

        private static S2CActionStarted Started(ulong actionId)
        {
            return new S2CActionStarted
            {
                ActionInstanceId = actionId,
                SourceEntityId = Self,
                SkillId = "basic_1",
                ClientSeq = 7,
                ServerTick = 10,
                Facing = Facing.Right,
                TargetEntityId = Foe,
                CooldownEndsAtTick = 20,
                MpAfter = 950,
                ActiveStartsAtTick = 12,
                ActiveEndsAtTick = 13,
                RecoveryEndsAtTick = 15,
            };
        }

        /// <summary>Action start: an accepted 203 moves the state to
        /// Startup and the presentation emits ActionStart with the
        /// timing contract available for the HUD.</summary>
        [Test]
        public void ActionStartPresentsFromAuthoritativeStarted()
        {
            var applier = new CombatApplier(Self);
            var pres = new CombatPres(applier.State);

            applier.Apply(Frame(CombatApplier.S2CActionStarted,
                Started(1)));
            var cue = pres.Evaluate(10);

            Assert.AreEqual(CombatPresentationCue.ActionStart, cue);
            Assert.AreEqual(CombatActionState.Startup, applier.State.State);
            Assert.AreEqual(1UL, applier.State.ActionInstanceId);
            Assert.AreEqual(20UL, applier.State.CooldownEndsAtTick);
            Assert.AreEqual(15UL, applier.State.RecoveryEndsAtTick);
        }

        /// <summary>Action reject: a 204 for a combat request surfaces
        /// the error code + seq for the reject flash; unrelated request
        /// ids (world surface) are filtered.</summary>
        [Test]
        public void ActionRejectPresentsOnlyCombatRequests()
        {
            var applier = new CombatApplier(Self);
            var pres = new CombatPres(applier.State);

            // A world-owned request id must not touch combat state.
            applier.Apply(Frame(CombatApplier.S2CActionRejected,
                new S2CActionRejected
                {
                    ClientSeq = 1,
                    RequestMessageId = 104, // portal: IWorldSink domain
                    ErrorCode = ErrorCode.OutOfRange,
                }));
            Assert.AreEqual(ErrorCode.Unspecified,
                applier.State.LastReject);

            applier.Apply(Frame(CombatApplier.S2CActionRejected,
                new S2CActionRejected
                {
                    ClientSeq = 9,
                    RequestMessageId = CombatApplier.C2SBasicAttack,
                    ErrorCode = ErrorCode.CooldownActive,
                }));
            var cue = pres.Evaluate(10);

            Assert.AreEqual(ErrorCode.CooldownActive,
                applier.State.LastReject);
            Assert.AreEqual(9UL, applier.State.LastRejectSeq);
            Assert.AreEqual(CombatPresentationCue.Rejected, cue);
        }

        /// <summary>Interrupt: a hit on self enters the HitReaction
        /// presentation marker; the marker clears on its deadline; a
        /// JG success produces the JustGuardSuccess hitstop cue.</summary>
        [Test]
        public void InterruptAndJustGuardPresentOnDamage()
        {
            var applier = new CombatApplier(Self);
            var pres = new CombatPres(applier.State);
            applier.Apply(Frame(CombatApplier.S2CActionStarted,
                Started(2)));

            applier.Apply(Frame(CombatApplier.S2CCombatEvent,
                new S2CCombatEvent
                {
                    EventId = 1,
                    ServerTick = 12,
                    ActionInstanceId = 2,
                    SourceEntityId = Foe,
                    TargetEntityId = Self,
                    ResultKind = CombatResultKind.Damage,
                    Outcome = CombatOutcome.Hit,
                    PostMitigationDamage = 40,
                    HpDamage = 40,
                    TargetHpAfter = 460,
                    TargetShieldAfter = 0,
                }));
            var cue = pres.Evaluate(12);

            Assert.AreEqual(CombatPresentationCue.HitReaction, cue);
            Assert.AreEqual(CombatActionState.HitReaction,
                applier.State.State);
            Assert.IsTrue(applier.State.InCombat);
            Assert.AreEqual(460L, applier.State.SelfHp);
            // Hit-reaction marker clears at its ~120 ms deadline.
            pres.Evaluate(16);
            Assert.AreEqual(CombatActionState.Startup,
                applier.State.State);

            // JG success on a later hit → hitstop cue, not hit-reaction.
            applier.Apply(Frame(CombatApplier.S2CCombatEvent,
                new S2CCombatEvent
                {
                    EventId = 2,
                    ServerTick = 20,
                    SourceEntityId = Foe,
                    TargetEntityId = Self,
                    ResultKind = CombatResultKind.Damage,
                    Outcome = CombatOutcome.Hit,
                    PostMitigationDamage = 30,
                    HpDamage = 18,
                    TargetHpAfter = 442,
                    JustGuardWindow = true,
                    JustGuardTriggered = true,
                }));
            cue = pres.Evaluate(20);
            Assert.AreEqual(CombatPresentationCue.JustGuardSuccess, cue);
        }

        /// <summary>Death: a killed combat event takes the presentation
        /// to Dead and stays dead; the respawn floor lands for the
        /// respawn-request affordance (execution is IMP-084).</summary>
        [Test]
        public void DeathPresentsTerminalStateAndRespawnFloor()
        {
            var applier = new CombatApplier(Self);
            var pres = new CombatPres(applier.State);
            applier.Apply(Frame(CombatApplier.S2CActionStarted,
                Started(3)));

            applier.Apply(Frame(CombatApplier.S2CCombatEvent,
                new S2CCombatEvent
                {
                    EventId = 3,
                    ServerTick = 12,
                    SourceEntityId = Foe,
                    TargetEntityId = Self,
                    ResultKind = CombatResultKind.Damage,
                    Outcome = CombatOutcome.Hit,
                    PostMitigationDamage = 999,
                    HpDamage = 999,
                    TargetHpAfter = 0,
                    Killed = true,
                }));
            var cue = pres.Evaluate(12);

            Assert.AreEqual(CombatPresentationCue.Death, cue);
            Assert.AreEqual(CombatActionState.Dead, applier.State.State);
            Assert.AreEqual(0L, applier.State.SelfHp);

            // A later frame cannot resurrect presentation state.
            applier.Apply(Frame(CombatApplier.S2CCombatEvent,
                new S2CCombatEvent
                {
                    EventId = 4,
                    ServerTick = 13,
                    SourceEntityId = Foe,
                    TargetEntityId = Self,
                    HpDamage = 0,
                    TargetHpAfter = 0,
                }));
            pres.Evaluate(13);
            Assert.AreEqual(CombatActionState.Dead, applier.State.State);
        }

        /// <summary>Intents stamp client_mono_ms on the injected clock
        /// and send 200/201/202 on the shared sequence.</summary>
        [Test]
        public void IntentsStampMonoMsAndSend()
        {
            var sender = new FakeSender();
            long now = 0;
            var intents = new CombatIntents(sender, () => now);
            now = 12345;
            var c = System.Threading.CancellationToken.None;

            _ = intents.BasicAttackAsync(Facing.Right, Foe, c);
            Assert.AreEqual(CombatApplier.C2SBasicAttack,
                sender.LastId);
            var atk = sender.LastMsg as C2SBasicAttack;
            Assert.NotNull(atk);
            Assert.AreEqual(12345UL, atk!.ClientMonoMs);
            Assert.AreEqual(Foe, atk.TargetEntityId);

            _ = intents.UseSkillAsync("s1", Facing.Left, 0, 100, 0, c);
            var sk = sender.LastMsg as C2SSkillUse;
            Assert.NotNull(sk);
            Assert.AreEqual(CombatApplier.C2SSkillUse, sender.LastId);
            Assert.AreEqual("s1", sk!.SkillId);
            Assert.AreEqual(Facing.Left, sk.Facing);
            Assert.AreEqual(100, sk.AreaCenterXMm);

            _ = intents.SetTargetAsync(Foe, c);
            Assert.AreEqual(CombatApplier.C2STargetIntent,
                sender.LastId);
            var ti = sender.LastMsg as C2STargetIntent;
            Assert.NotNull(ti);
            Assert.AreEqual(Foe, ti!.TargetEntityId);
        }

        private sealed class FakeSender : ICombatSender
        {
            public uint LastId;
            public IMessage? LastMsg;

            public UnityEngine.Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload,
                System.Threading.CancellationToken cancel)
            {
                LastId = messageId;
                LastMsg = payload;
                var src = new UnityEngine.AwaitableCompletionSource<ulong>();
                src.SetResult(1UL);
                return src.Awaitable;
            }
        }
    }
}
