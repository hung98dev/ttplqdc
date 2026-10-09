using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Crafting;
using ThinhThan.UI.Crafting;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.CraftingUi
{
    /// <summary>
    /// IMP-027 PlayMode: crafting consult UI over recorded S2C frames —
    /// 405 craft result (consumed/currency/granted), 407 enhance
    /// result (success/level/rate/pity) — recipe list + input preview
    /// + affordability + the at-most-one-charm enhance block, plus
    /// the once-per-frame presenter dirty flag.
    /// </summary>
    public sealed class CraftingUiTests
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

        private static CraftingPanel NewPanel()
        {
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(CraftingPanel));
            var panel = go.GetComponent<CraftingPanel>();
            panel.ListRoot = Root(go, "list");
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

        [Test]
        public void CraftResultAppliesAndListsAllRecipes()
        {
            var applier = new CraftingApplier();
            applier.Apply(Frame(WireIds.S2CCraftResult,
                new S2CCraftResult
                {
                    Result = ResultStatus.Success,
                    RecipeId = "recipe.eq.t1.dinh_lang.weapon",
                    BatchQuantity = 2,
                    Consumed =
                    {
                        new ItemQuantity
                        {
                            ItemId = "item.material.lang_da.manh_dong",
                            Quantity = 10,
                        },
                    },
                    CurrencyDelta =
                    {
                        new CurrencyDelta
                        {
                            CurrencyId = "currency.common",
                            Amount = -500,
                        },
                    },
                    Granted =
                    {
                        new ItemGrant
                        {
                            ItemInstanceId = ByteString.CopyFrom(Id16(9)),
                            ItemId = "item.eq.t1.dinh_lang.weapon",
                            Quantity = 2,
                        },
                    },
                }));
            Assert.AreEqual(1ul, applier.Version);
            Assert.NotNull(applier.LastCraftResult);
            Assert.AreEqual(ResultStatus.Success,
                applier.LastCraftResult!.Result);
            Assert.AreEqual(1, applier.LastCraftResult.Granted.Count);
            Assert.AreEqual(-500L,
                applier.LastCraftResult.CurrencyDelta[0].Amount);

            var panel = NewPanel();
            panel.Apply(applier.State);
            Assert.AreEqual(168, panel.LiveCount);
            Assert.AreEqual(1, panel.ApplyCount);
        }

        [Test]
        public void EnhanceResultProjectsRateAndPity()
        {
            var applier = new CraftingApplier();
            applier.Apply(Frame(WireIds.S2CEnhanceResult,
                new S2CEnhanceResult
                {
                    Result = ResultStatus.Success,
                    ItemInstanceId = ByteString.CopyFrom(Id16(3)),
                    Success = false,
                    LevelBefore = 12,
                    LevelAfter = 12,
                    FinalRateBp = 1100,
                    PityFailCount = 3,
                    Consumed =
                    {
                        new ItemQuantity
                        {
                            ItemId = "item.consumable.bua_may.so_cap",
                            Quantity = 1,
                        },
                    },
                }));
            Assert.NotNull(applier.LastEnhanceResult);
            Assert.IsFalse(applier.LastEnhanceResult!.Success);
            Assert.AreEqual(12u, applier.LastEnhanceResult.LevelAfter);
            Assert.AreEqual(1100u, applier.LastEnhanceResult.FinalRateBp);
            Assert.AreEqual(3u, applier.LastEnhanceResult.PityFailCount);

            var panel = NewPanel();
            panel.Apply(applier.State);
            Assert.AreEqual(1, panel.ApplyCount);
        }

        [Test]
        public void RecipeCatalogMirrorMatchesWireIds()
        {
            Assert.AreEqual(168, CraftingRecipes.Equipment.Length);
            Assert.IsTrue(CraftingRecipes.TryFind(
                "recipe.eq.t6.nui_thieng.weapon",
                out CraftingRecipes.Row row));
            Assert.AreEqual(5ul, row.InputQuantity);
            Assert.AreEqual(17500L, row.CommonCost);
            Assert.AreEqual(51, row.Tier.MinLevel);
            // Charm eligibility: so_cap lucky rejects at +8+; cao_cap
            // eligible everywhere.
            Assert.IsTrue(CraftingRecipes.TryCharmGrade(
                CraftingRecipes.LuckyPrefix + ".so_cap",
                CraftingRecipes.LuckyPrefix,
                CraftingRecipes.LuckyGrades,
                out CraftingRecipes.LuckyGrade g));
            Assert.AreEqual(8, g.MaxCurrent);
            // Rate assembly: +0 base 10000; pity capped +500.
            Assert.AreEqual(10000, CraftingRecipes.BaseRateBP[0]);
            Assert.AreEqual(500, CraftingRecipes.PityBonusBP(9));
            Assert.AreEqual(9500, CraftingRecipes.FinalRateBP(
                0, true, 500, 9));
            Assert.AreEqual(0, CraftingRecipes.FloorOf(3));
            Assert.AreEqual(4, CraftingRecipes.FloorOf(7));
            Assert.AreEqual(8, CraftingRecipes.FloorOf(11));
            Assert.AreEqual(12, CraftingRecipes.FloorOf(15));
        }

        [Test]
        public void PresenterAppliesOncePerVersion()
        {
            var presenterGo = new GameObject("p",
                typeof(CraftingPresenter));
            var presenter = presenterGo.GetComponent<CraftingPresenter>();
            var applier = new CraftingApplier();
            var panel = NewPanel();
            presenter.Applier = applier;
            presenter.Panel = panel;

            applier.Apply(Frame(WireIds.S2CCraftResult,
                new S2CCraftResult
                {
                    Result = ResultStatus.Success,
                    RecipeId = "recipe.eq.t1.dinh_lang.weapon",
                    BatchQuantity = 1,
                }));
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);

            applier.Apply(Frame(WireIds.S2CEnhanceResult,
                new S2CEnhanceResult
                {
                    Result = ResultStatus.Success,
                    Success = true,
                    LevelBefore = 0,
                    LevelAfter = 1,
                    FinalRateBp = 10000,
                }));
            presenter.ApplyIfDirty();
            Assert.AreEqual(2, panel.ApplyCount);
        }
    }
}
