using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Trade;
using ThinhThan.UI.Trade;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.TradeUi
{
    /// <summary>
    /// IMP-029 PlayMode: the trade FSM over S2C frames — 701 invite,
    /// 706 open/lock offer states, 710 request verdicts, 709 final
    /// result (actor op id + partner empty op), 704 cancel/disconnect
    /// — plus the once-per-frame presenter dirty flag.
    /// </summary>
    public sealed class TradeUiTests
    {
        private static DecodedFrame Frame(uint id, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = payload,
            };
        }

        private static byte[] Id16(byte seed)
        {
            var b = new byte[16];
            b[15] = seed;
            return b;
        }

        private static TradePanel NewPanel()
        {
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(TradePanel));
            var panel = go.GetComponent<TradePanel>();
            panel.OwnRoot = Root(go, "own");
            panel.PartnerRoot = Root(go, "partner");
            // No TMP components in headless CI (IMP-066 precedent);
            // every view null-guards its TMP_Text refs.
            return panel;
        }

        private static RectTransform Root(GameObject parent, string name)
        {
            var go = new GameObject(name, typeof(RectTransform));
            go.transform.SetParent(parent.transform, false);
            return go.GetComponent<RectTransform>();
        }

        private static S2CTradeOfferState OfferState(byte[] tradeId,
            ulong revision, TradeOfferState state)
        {
            return new S2CTradeOfferState
            {
                TradeId = ByteString.CopyFrom(tradeId),
                Revision = revision,
                State = state,
                FeePreview = 5,
                Sides =
                {
                    new TradeSide
                    {
                        CharacterId = ByteString.CopyFrom(Id16(1)),
                        CommonAmount = 0,
                        Confirmed = state == TradeOfferState.Locked ||
                            state == TradeOfferState.Committing,
                        Items =
                        {
                            new ItemInstanceView
                            {
                                ItemInstanceId = ByteString.CopyFrom(
                                    Id16(11)),
                                ItemId = "item.sword",
                                Quantity = 1,
                            },
                        },
                    },
                    new TradeSide
                    {
                        CharacterId = ByteString.CopyFrom(Id16(2)),
                        CommonAmount = 100,
                        Confirmed = state == TradeOfferState.Locked ||
                            state == TradeOfferState.Committing,
                    },
                },
            };
        }

        [Test]
        public void InviteThenOpenLocksAndSettles()
        {
            var applier = new TradeApplier();
            applier.SetLocalCharacterId(Id16(1));
            byte[] trade = Id16(9);
            applier.Apply(Frame(WireIds.S2CTradeInvite,
                new S2CTradeInvite
                {
                    TradeId = ByteString.CopyFrom(trade),
                    InviterCharacterId = ByteString.CopyFrom(Id16(2)),
                    InviterName = "partner",
                    ExpiresInSeconds = 60,
                }));
            Assert.AreEqual(TradePhase.Invited, applier.State.Phase);
            Assert.AreEqual("partner", applier.State.InviterName);

            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 1, TradeOfferState.Open)));
            Assert.AreEqual(TradePhase.Open, applier.State.Phase);
            Assert.AreEqual(1UL, applier.State.Revision);
            Assert.AreEqual(1, applier.State.OwnSide.Items.Count);
            Assert.AreEqual(100, applier.State.PartnerSide.CommonAmount);

            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 2, TradeOfferState.Locked)));
            Assert.AreEqual(TradePhase.Locked, applier.State.Phase);
            Assert.IsTrue(applier.State.OwnSide.Confirmed);

            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 2, TradeOfferState.Committing)));
            Assert.AreEqual(TradePhase.Committing, applier.State.Phase);

            applier.Apply(Frame(WireIds.S2CTradeResult,
                new S2CTradeResult
                {
                    Result = new OperationResult
                    {
                        OperationId = ByteString.CopyFrom(Id16(77)),
                        Status = ResultStatus.Success,
                    },
                    TradeId = ByteString.CopyFrom(trade),
                    SettlementId = ByteString.CopyFrom(Id16(3)),
                    CommonReceived = 95,
                    Fee = 5,
                }));
            Assert.AreEqual(TradePhase.Completed, applier.State.Phase);
            Assert.AreEqual(95, applier.State.LastResult!.CommonReceived);
            Assert.AreEqual(Id16(77),
                applier.State.LastResult!.Result.OperationId.ToByteArray());
        }

        [Test]
        public void RequestResultPerRequestAndCancelFlow()
        {
            var applier = new TradeApplier();
            byte[] trade = Id16(9);
            applier.Apply(Frame(WireIds.S2CTradeRequestResult,
                new S2CTradeRequestResult
                {
                    Result = new OperationResult
                    {
                        OperationId = ByteString.CopyFrom(Id16(50)),
                        Status = ResultStatus.Error,
                        ErrorCode = ErrorCode.StateConflict,
                    },
                    RequestMessageId = TradeApplier.C2STradeOfferUpdate,
                    TradeId = ByteString.CopyFrom(trade),
                }));
            Assert.AreEqual(TradeApplier.C2STradeOfferUpdate,
                applier.LastRequestResult!.RequestMessageId);
            Assert.AreEqual(ErrorCode.StateConflict,
                applier.LastRequestResult!.Result.ErrorCode);

            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 1, TradeOfferState.Open)));
            applier.Apply(Frame(WireIds.S2CTradeCancelled,
                new S2CTradeCancelled
                {
                    TradeId = ByteString.CopyFrom(trade),
                    Reason = TradeCancelReason.Cancelled,
                }));
            Assert.AreEqual(TradePhase.Cancelled, applier.State.Phase);
            Assert.AreEqual(TradeCancelReason.Cancelled,
                applier.State.CancelReason);
        }

        [Test]
        public void DisconnectCancelsAndPartnerResultHasEmptyOperationId()
        {
            var applier = new TradeApplier();
            byte[] trade = Id16(9);
            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 1, TradeOfferState.Open)));
            applier.Apply(Frame(WireIds.S2CTradeCancelled,
                new S2CTradeCancelled
                {
                    TradeId = ByteString.CopyFrom(trade),
                    Reason = TradeCancelReason.PartnerDisconnected,
                }));
            Assert.AreEqual(TradePhase.Cancelled, applier.State.Phase);
            Assert.AreEqual(TradeCancelReason.PartnerDisconnected,
                applier.State.CancelReason);

            // partner-side 709 carries an empty operation_id.
            var second = new TradeApplier();
            second.Apply(Frame(WireIds.S2CTradeResult,
                new S2CTradeResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    TradeId = ByteString.CopyFrom(trade),
                    SettlementId = ByteString.CopyFrom(Id16(3)),
                }));
            Assert.AreEqual(0,
                second.State.LastResult!.Result.OperationId.Length);
            Assert.AreEqual(TradePhase.Completed, second.State.Phase);
        }

        [Test]
        public void PanelRebuildsRowsAndButtonsByPhase()
        {
            var panel = NewPanel();
            var applier = new TradeApplier();
            applier.SetLocalCharacterId(Id16(1));
            var presenterGo = new GameObject("presenter",
                typeof(TradePresenter));
            var presenter = presenterGo.GetComponent<TradePresenter>();
            presenter.Applier = applier;
            presenter.Panel = panel;
            byte[] trade = Id16(9);
            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 3, TradeOfferState.Open)));
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.LiveCount);
            Assert.AreEqual(3UL, panel.LastRevision);
            int applied = panel.ApplyCount;
            presenter.ApplyIfDirty();
            Assert.AreEqual(applied, panel.ApplyCount,
                "presenter must not re-apply an unchanged version");

            applier.Apply(Frame(WireIds.S2CTradeOfferState,
                OfferState(trade, 4, TradeOfferState.Locked)));
            presenter.ApplyIfDirty();
            Assert.AreEqual(applied + 1, panel.ApplyCount);
            Assert.AreEqual(4UL, panel.LastRevision);
        }
    }
}
