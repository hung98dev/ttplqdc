using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using ThinhThan.Systems.Progression;
using ThinhThan.UI.Progression;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.ProgressionPresentation
{
    /// <summary>
    /// IMP-011 presentation contract (packet Tests): the 515 push drives
    /// the authoritative level/EXP/potential/skill-point projection; the
    /// 514 verdict feeds the result line; intents mint durable op ids and
    /// hit the wire unchanged.
    /// </summary>
    public sealed class ProgressionPresentationTests
    {
        private sealed class FakeSender : IMovementSender
        {
            public readonly List<(uint Id, IMessage Msg)> Sent =
                new List<(uint, IMessage)>();

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult(1);
                return src.Awaitable;
            }
        }

        private static S2CProgressionState MakeState()
        {
            var s = new S2CProgressionState
            {
                Level = 25,
                CurrentExp = 1_350_000,
                UnspentSkillPoints = 1,
                UnspentPotentialPoints = 0,
                PotentialAllocated =
                    new PotentialDelta { Str = 6, Vit = 4, Int = 0, Agi = 0 },
                PotentialEarnedTotal = 10,
                ProgressionRevision = 7,
                SkillLoadout = new SkillLoadoutView
                {
                    BasicSkillId = "skill.kim.basic.kiem_thuc",
                },
            };
            s.SkillLoadout.ActiveSlots.Add("skill.kim.active.pha_khong_kiem");
            s.Skills.Add(new SkillView
            {
                SkillId = "skill.kim.basic.kiem_thuc",
                Level = 5,
            });
            s.Skills.Add(new SkillView
            {
                SkillId = "skill.kim.active.pha_khong_kiem",
                Level = 2,
            });
            return s;
        }

        [Test]
        public void ProgressionStateProjectsWireSnapshot()
        {
            var applier = new ProgressionApplier();
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionState,
                Payload = MakeState(),
            });

            ProgressionState st = applier.State;
            Assert.That(st.Level, Is.EqualTo(25));
            Assert.That(st.CurrentExp, Is.EqualTo(1_350_000));
            Assert.That(st.UnspentSkillPoints, Is.EqualTo(1));
            Assert.That(st.UnspentPotentialPoints, Is.EqualTo(0));
            Assert.That(st.PotentialStr, Is.EqualTo(6));
            Assert.That(st.PotentialVit, Is.EqualTo(4));
            Assert.That(st.PotentialEarnedTotal, Is.EqualTo(10));
            Assert.That(st.ProgressionRevision, Is.EqualTo(7));
            Assert.That(st.Skills.Count, Is.EqualTo(2));
            Assert.That(st.Skills[0].SkillId,
                Is.EqualTo("skill.kim.basic.kiem_thuc"));
            Assert.That(st.Skills[0].Level, Is.EqualTo(5));
            Assert.That(st.BasicSkillId,
                Is.EqualTo("skill.kim.basic.kiem_thuc"));
            Assert.That(st.ActiveSlots[0],
                Is.EqualTo("skill.kim.active.pha_khong_kiem"));
            Assert.That(st.ActiveSlots[1], Is.Null);
            Assert.That(st.Revision, Is.EqualTo(1));
        }

        [Test]
        public void ProgressionStateIsReplaceable()
        {
            var applier = new ProgressionApplier();
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionState,
                Payload = MakeState(),
            });
            S2CProgressionState next = MakeState();
            next.Level = 26;
            next.CurrentExp = 1_450_000;
            next.ProgressionRevision = 8;
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionState,
                Payload = next,
            });

            Assert.That(applier.State.Level, Is.EqualTo(26));
            Assert.That(applier.State.ProgressionRevision, Is.EqualTo(8));
            Assert.That(applier.State.Revision, Is.EqualTo(2));
        }

        [Test]
        public void MutateResultFeedsVerdictTail()
        {
            var applier = new ProgressionApplier();
            byte[] op = new byte[16];
            op[0] = 0xAB;
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionMutateResult,
                Payload = new S2CProgressionMutateResult
                {
                    RequestMessageId = ProgressionWireIds.C2SSkillUpgrade,
                    Result = new OperationResult
                    {
                        OperationId = ByteString.CopyFrom(op),
                        Status = ResultStatus.Success,
                    },
                },
            });

            Assert.That(applier.Results.Count, Is.EqualTo(1));
            ProgressionApplier.Result r = applier.Results[0];
            Assert.That(r.RequestMessageId,
                Is.EqualTo(ProgressionWireIds.C2SSkillUpgrade));
            Assert.That(r.Status, Is.EqualTo(ResultStatus.Success));
            Assert.That(r.OperationId.Bytes.ToByteArray(), Is.EqualTo(op));
            Assert.That(applier.ResultRevision, Is.EqualTo(1));
        }

        [Test]
        public void IntentsSendWireFramesUnchanged()
        {
            var sender = new FakeSender();
            var intents = new ProgressionIntents(
                sender, () => new byte[16] { 1, 2, 3 });
            var _ = intents.RequestUpgrade(
                "skill.kim.active.pha_khong_kiem", 2,
                CancellationToken.None);
            _ = intents.RequestAllocate(
                new PotentialDelta { Str = 2, Agi = 1 },
                CancellationToken.None);
            _ = intents.RequestRespec(
                "npc.lang_da.nguoi_dan_duong", RespecKind.Potential,
                CancellationToken.None);
            minted++;

            Assert.That(sender.Sent.Count, Is.EqualTo(3));
            Assert.That(sender.Sent[0].Id,
                Is.EqualTo(ProgressionWireIds.C2SSkillUpgrade));
            var up = (C2SSkillUpgrade)sender.Sent[0].Msg;
            Assert.That(up.SkillId,
                Is.EqualTo("skill.kim.active.pha_khong_kiem"));
            Assert.That(up.ExpectedLevel, Is.EqualTo(2u));
            Assert.That(up.OperationId.Length, Is.EqualTo(16));

            Assert.That(sender.Sent[1].Id,
                Is.EqualTo(ProgressionWireIds.C2SPotentialAllocate));
            var alloc = (C2SPotentialAllocate)sender.Sent[1].Msg;
            Assert.That(alloc.Deltas.Str, Is.EqualTo(2));
            Assert.That(alloc.Deltas.Agi, Is.EqualTo(1));

            Assert.That(sender.Sent[2].Id,
                Is.EqualTo(ProgressionWireIds.C2SRespec));
            var respec = (C2SRespec)sender.Sent[2].Msg;
            Assert.That(respec.NpcId,
                Is.EqualTo("npc.lang_da.nguoi_dan_duong"));
            Assert.That(respec.Kind, Is.EqualTo(RespecKind.Potential));
        }

        [Test]
        public void PresenterMapsStateToModel()
        {
            var applier = new ProgressionApplier();
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionState,
                Payload = MakeState(),
            });
            applier.Apply(new DecodedFrame
            {
                MessageId = WireIds.S2CProgressionMutateResult,
                Payload = new S2CProgressionMutateResult
                {
                    RequestMessageId = ProgressionWireIds.C2SSkillUpgrade,
                    Result = new OperationResult
                    {
                        OperationId = ByteString.CopyFrom(new byte[16]),
                        Status = ResultStatus.Success,
                    },
                },
            });

            var presenter = new ProgressionPresenter(
                applier.State, applier,
                new ProgressionIntents(new FakeSender()));
            presenter.Sync();

            ProgressionPresenter.Model m = presenter.Snapshot;
            Assert.That(m.Level, Is.EqualTo(25));
            Assert.That(m.CurrentExp, Is.EqualTo(1_350_000));
            Assert.That(m.ExpToNext, Is.EqualTo(10000L * 25 * 25));
            Assert.That(m.UnspentSkillPoints, Is.EqualTo(1));
            Assert.That(m.PotentialStr, Is.EqualTo(6));
            Assert.That(m.Skills.Length, Is.EqualTo(2));
            Assert.That(m.BasicSkillId,
                Is.EqualTo("skill.kim.basic.kiem_thuc"));
            Assert.That(m.ActiveSlots[0],
                Is.EqualTo("skill.kim.active.pha_khong_kiem"));
            Assert.That(m.ResultText, Is.EqualTo("Lưu thành công"));
            Assert.That(m.Dirty, Is.EqualTo(
                ProgressionPresenter.Dirty.Vitals |
                ProgressionPresenter.Dirty.Points |
                ProgressionPresenter.Dirty.Skills |
                ProgressionPresenter.Dirty.Result));
        }

        [Test]
        public void StatProjectorMirrorsSpecPipeline()
        {
            // stats.md: kim Lv25 alloc {STR 10, VIT 10, INT 10, AGI 10}.
            string cls = "class.kim";
            double[] base_ = StatProjector.ComputeBase(cls, 25);
            Assert.That(base_[(int)StatProjector.Stat.MaxHP],
                Is.EqualTo(500.0 + 32.0 * 24));
            Assert.That(base_[(int)StatProjector.Stat.Attack],
                Is.EqualTo(40.0 + 5.5 * 24));

            List<StatProjector.Modifier> mods =
                StatProjector.PotentialModifiers(cls, 10, 10, 10, 10);
            double[] fin = StatProjector.ComputeFinal(base_, mods);
            // HP = 1268 + 6·10 = 1328; ATK = 172 + .75·10 + .25·10 = 182;
            // DEF = 68 + .2·10 = 70; crit .055; dodge .034; ms 1.008.
            Assert.That(fin[(int)StatProjector.Stat.MaxHP],
                Is.EqualTo(1328.0));
            Assert.That(fin[(int)StatProjector.Stat.Attack],
                Is.EqualTo(182.0));
            Assert.That(fin[(int)StatProjector.Stat.Defense],
                Is.EqualTo(70.0));
            Assert.That(fin[(int)StatProjector.Stat.CritChance],
                Is.EqualTo(0.055).Within(1e-9));
            Assert.That(fin[(int)StatProjector.Stat.DodgeChance],
                Is.EqualTo(0.034).Within(1e-9));
            Assert.That(fin[(int)StatProjector.Stat.MoveSpeed],
                Is.EqualTo(1.008).Within(1e-9));
            Assert.That(fin[(int)StatProjector.Stat.MaxMP],
                Is.EqualTo(392.0 + 10.0));
        }
    }
}
