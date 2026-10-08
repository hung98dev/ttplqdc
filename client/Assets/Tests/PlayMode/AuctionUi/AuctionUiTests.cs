using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Auction;
using ThinhThan.UI.Auction;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.Tests.PlayMode.AuctionUi
{
    /// <summary>
    /// IMP-030 PlayMode: 744 my-state snapshot apply → section rows,
    /// 736 sold → listing flips + proceeds row, 739 search → page rows,
    /// verdict results stay on the applier, and the presenter applies
    /// at most once per frame version.
    /// </summary>
    public sealed class AuctionUiTests
    {
        private static DecodedFrame Frame(uint id, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = payload,
            };
        }

        private static ItemInstanceView Item(
            byte[] iid, string itemId, uint qty)
        {
            return new ItemInstanceView
            {
                ItemInstanceId = ByteString.CopyFrom(iid),
                ItemId = itemId,
                Quantity = qty,
                EffectiveBinding = ItemBinding.Unbound,
            };
        }

        private static byte[] Id16(byte seed)
        {
            var b = new byte[16];
            b[15] = seed;
            return b;
        }

        private static AuctionPanel NewPanel()
        {
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(AuctionPanel));
            var panel = go.GetComponent<AuctionPanel>();
            panel.ListingsRoot = Root(go, "listings");
            panel.EscrowRoot = Root(go, "escrow");
            panel.ProceedsRoot = Root(go, "proceeds");
            panel.SearchRoot = Root(go, "search");
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

        private static S2CAuctionMyState MyState()
        {
            return new S2CAuctionMyState
            {
                Listings =
                {
                    new AuctionListingView
                    {
                        ListingId = ByteString.CopyFrom(Id16(1)),
                        Item = Item(Id16(11), "item.mat.ore", 4),
                        PriceCommon = 1000,
                        State = AuctionListingState.Active,
                        ExpiresAt = 1_760_000_000_000,
                    },
                },
                EscrowAssets =
                {
                    new AuctionEscrowAssetView
                    {
                        EscrowAssetId = ByteString.CopyFrom(Id16(2)),
                        Item = Item(Id16(12), "item.potion.hp", 1),
                        Reason = AuctionEscrowReason.Cancelled,
                        AutoClaimAt = 1_760_000_000_000,
                    },
                },
                Proceeds =
                {
                    new AuctionProceedsView
                    {
                        ProceedsId = ByteString.CopyFrom(Id16(3)),
                        AmountCommon = 950,
                        State = AuctionProceedsState.Pending,
                    },
                },
            };
        }

        [Test]
        public void MyStateSnapshotFillsSectionRows()
        {
            var applier = new AuctionApplier();
            applier.Apply(Frame(WireIds.S2CAuctionMyState, MyState()));

            Assert.AreEqual(1, applier.State.Listings.Count);
            Assert.AreEqual(1, applier.State.EscrowAssets.Count);
            Assert.AreEqual(1, applier.State.Proceeds.Count);
            Assert.AreEqual("item.mat.ore",
                applier.State.Listings[0].ItemId);
            Assert.AreEqual(950, applier.State.Proceeds[0].AmountCommon);
            Assert.AreEqual(1UL, applier.Version);

            var panel = NewPanel();
            panel.Apply(applier.State);
            Assert.AreEqual(3, panel.LiveCount);
        }

        [Test]
        public void SoldFrameFlipsListingAndAppendsProceeds()
        {
            var applier = new AuctionApplier();
            applier.Apply(Frame(WireIds.S2CAuctionMyState, MyState()));
            applier.Apply(Frame(WireIds.S2CAuctionSold,
                new S2CAuctionSold
                {
                    ListingId = ByteString.CopyFrom(Id16(1)),
                    ItemId = "item.mat.ore",
                    Quantity = 4U,
                    PriceCommon = 1000,
                    Tax = 50,
                    ProceedsId = ByteString.CopyFrom(Id16(4)),
                    ProceedsAmount = 950,
                }));

            Assert.AreEqual(AuctionListingState.Sold,
                applier.State.Listings[0].State);
            Assert.AreEqual(2, applier.State.Proceeds.Count);
            Assert.AreEqual(2UL, applier.Version);
        }

        [Test]
        public void SearchResultReplacesPageRows()
        {
            var applier = new AuctionApplier();
            applier.Apply(Frame(WireIds.S2CAuctionSearchResult,
                new S2CAuctionSearchResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    Rows =
                    {
                        new AuctionSearchRow
                        {
                            ListingId = ByteString.CopyFrom(Id16(5)),
                            Item = Item(Id16(13), "item.weapon.t3", 1),
                            PriceCommon = 4000,
                            SellerDisplayName = "seller.test",
                        },
                    },
                    NextPageCursor = "cGFnZTI",
                }));

            Assert.AreEqual(1, applier.State.SearchRows.Count);
            Assert.AreEqual("item.weapon.t3",
                applier.State.SearchRows[0].ItemId);
            Assert.AreEqual("cGFnZTI", applier.State.NextSearchCursor);

            applier.Apply(Frame(WireIds.S2CAuctionSearchResult,
                new S2CAuctionSearchResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                }));
            Assert.AreEqual(0, applier.State.SearchRows.Count);
            Assert.AreEqual("", applier.State.NextSearchCursor);
        }

        [Test]
        public void VerdictResultsStayOnApplier()
        {
            var applier = new AuctionApplier();
            applier.Apply(Frame(WireIds.S2CAuctionBuyResult,
                new S2CAuctionBuyResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Error,
                        ErrorCode = ErrorCode.SameAccountForbidden,
                    },
                }));
            Assert.AreEqual(ErrorCode.SameAccountForbidden,
                applier.LastBuyResult!.Result.ErrorCode);

            applier.Apply(Frame(WireIds.S2CAuctionListResult,
                new S2CAuctionListResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Error,
                        ErrorCode = ErrorCode.AhPriceFloorNotMet,
                    },
                }));
            Assert.AreEqual(ErrorCode.AhPriceFloorNotMet,
                applier.LastListResult!.Result.ErrorCode);
        }

        [Test]
        public void PresenterAppliesOncePerVersion()
        {
            var applier = new AuctionApplier();
            var go = new GameObject("presenter",
                typeof(AuctionPresenter));
            var presenter = go.GetComponent<AuctionPresenter>();
            var panel = NewPanel();
            presenter.Applier = applier;
            presenter.Panel = panel;

            presenter.ApplyIfDirty();
            presenter.ApplyIfDirty();
            Assert.AreEqual(0, panel.ApplyCount);

            applier.Apply(Frame(WireIds.S2CAuctionMyState, MyState()));
            presenter.ApplyIfDirty();
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);
            Assert.AreEqual(applier.Version,
                presenter.AppliedVersion);
        }
    }
}
