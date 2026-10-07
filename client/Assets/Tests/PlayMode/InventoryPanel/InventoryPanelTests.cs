using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using PbSlotView = ThinhThan.Protocol.V1.InventorySlotView;
using UiSlotView = ThinhThan.UI.Inventory.InventorySlotView;
using UiPanel = ThinhThan.UI.Inventory.InventoryPanel;
using ThinhThan.Systems.Inventory;
using ThinhThan.UI.Inventory;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.Tests.PlayMode.InventoryPanel
{
    /// <summary>
    /// IMP-009 PlayMode: 432/433/435 snapshot apply → row contents,
    /// 401 changed_slots merge → row update, 419 claim → entitlement
    /// row state, and panel separation (ADR-0029: no account-item
    /// deposit path on this surface).
    /// </summary>
    public sealed class InventoryPanelTests
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

        private static UiPanel NewPanel()
        {
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(UiPanel));
            var root = new GameObject("grid", typeof(RectTransform));
            root.transform.SetParent(go.transform, false);
            var panel = go.GetComponent<UiPanel>();
            panel.GridRoot = root.GetComponent<RectTransform>();
            // No TMP components in headless CI (IMP-066 precedent:
            // TMP Settings asset absent -> defaultStyleSheet NRE).
            // Panels null-guard every TMP_Text ref.
            var btn = new GameObject("expand", typeof(Button),
                typeof(RectTransform));
            btn.transform.SetParent(go.transform, false);
            panel.ExpandButton = btn.GetComponent<Button>();
            return panel;
        }

        [Test]
        public void SnapshotApplyFillsRowContents()
        {
            var applier = new InventoryApplier();
            applier.Apply(Frame(WireIds.S2CInventoryState,
                new S2CInventoryState
                {
                    Capacity = 60,
                    InventoryRevision = 7,
                    Slots =
                    {
                        new PbSlotView
                        {
                            Slot = 0,
                            Item = Item(Id16(1), "item.mat.ore", 42),
                            LockedQuantity = 5,
                        },
                    },
                }));

            InventorySlotModel? m = applier.State.SlotAt(0);
            Assert.IsNotNull(m);
            Assert.AreEqual(42U, m!.Item!.Quantity);
            Assert.AreEqual(5U, m.LockedQuantity);
            Assert.AreEqual(60, applier.State.Capacity);
            Assert.AreEqual(7UL, applier.State.Revision);
            Assert.AreEqual(1UL, applier.Version);

            var panel = NewPanel();
            panel.Apply(applier.State);
            Assert.AreEqual(60, panel.LiveCount);
            UiSlotView cell = panel.CellAt(0);
            Assert.AreEqual(0U, cell.Slot);
        }

        [Test]
        public void ChangedSlotsResultUpdatesRow()
        {
            var applier = new InventoryApplier();
            applier.Apply(Frame(WireIds.S2CInventoryState,
                new S2CInventoryState
                {
                    Capacity = 60,
                    InventoryRevision = 1,
                    Slots =
                    {
                        new PbSlotView
                        {
                            Slot = 0,
                            Item = Item(Id16(1), "item.mat.ore", 10),
                        },
                    },
                }));
            applier.Apply(Frame(WireIds.S2CInventoryResult,
                new S2CInventoryResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    Op = InventoryOp.Move,
                    InventoryRevision = 2,
                    ChangedSlots =
                    {
                        new InventoryChangedSlot { Slot = 0 },
                        new InventoryChangedSlot
                        {
                            Slot = 4,
                            ItemInstanceId = ByteString.CopyFrom(Id16(1)),
                            ItemId = "item.mat.ore",
                            Quantity = 10,
                        },
                    },
                }));

            Assert.IsNull(applier.State.SlotAt(0)!.Item);
            Assert.AreEqual("item.mat.ore",
                applier.State.SlotAt(4)!.Item!.ItemId);
            Assert.AreEqual(2UL, applier.State.Revision);
        }

        [Test]
        public void ClaimResultMovesTierToClaimed()
        {
            var applier = new InventoryApplier();
            byte[] ent = Id16(9);
            applier.Apply(Frame(WireIds.S2CEntitlementPanelState,
                new S2CEntitlementPanelState
                {
                    Entitlements =
                    {
                        new EntitlementView
                        {
                            EntitlementId = ByteString.CopyFrom(ent),
                            ProductId = "product.season.1",
                            EntitlementType =
                                EntitlementType.AccountScopedAccess,
                            GrantState = EntitlementGrantState.Granted,
                            SeasonNumber = 1,
                            ClaimableTierIds = { "tier.5" },
                        },
                    },
                }));
            applier.Apply(Frame(WireIds.S2CEntitlementClaimResult,
                new S2CEntitlementClaimResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    RewardTierId = "tier.5",
                }));

            EntitlementRowModel? row = applier.Panel.RowFor(ent);
            Assert.IsNotNull(row);
            Assert.AreEqual(0, row!.ClaimableTierIds.Count);
            Assert.AreEqual(1, row.ClaimedTierIds.Count);
            Assert.AreEqual("tier.5", row.ClaimedTierIds[0]);
        }

        [Test]
        public void PanelSeparationNoItemDeposit()
        {
            // ADR-0029: the entitlement surface is a claim/panel
            // projection only — no path writes an item slot from a 435
            // row, and a claim result never lands in inventory slots.
            var applier = new InventoryApplier();
            applier.Apply(Frame(WireIds.S2CEntitlementPanelState,
                new S2CEntitlementPanelState
                {
                    Entitlements =
                    {
                        new EntitlementView
                        {
                            EntitlementId = ByteString.CopyFrom(Id16(8)),
                            ProductId = "product.cos.1",
                            EntitlementType =
                                EntitlementType.DirectAccountCosmetic,
                            GrantState =
                                EntitlementGrantState.RefundedConsumed,
                        },
                    },
                }));
            EntitlementRowModel? row = applier.Panel.RowFor(Id16(8));
            Assert.IsNotNull(row);
            Assert.IsFalse(row!.PanelEquippable);
            Assert.AreEqual(0, applier.State.Slots.Count);
        }

        [Test]
        public void PresenterAppliesOncePerVersion()
        {
            var applier = new InventoryApplier();
            var go = new GameObject("presenter",
                typeof(InventoryPresenter));
            var presenter = go.GetComponent<InventoryPresenter>();
            var panel = NewPanel();
            presenter.Applier = applier;
            presenter.Inventory = panel;

            presenter.ApplyIfDirty();
            presenter.ApplyIfDirty();
            Assert.AreEqual(0, panel.ApplyCount);

            applier.Apply(Frame(WireIds.S2CWalletState,
                new S2CWalletState
                {
                    WalletRevision = 1,
                    Balances =
                    {
                        new WalletBalance
                        {
                            CurrencyId = "currency.common",
                            Amount = 500,
                            Cap = 2000000000L,
                        },
                    },
                }));
            presenter.ApplyIfDirty();
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);
            Assert.AreEqual(applier.Version,
                presenter.AppliedVersion);
        }
    }
}
