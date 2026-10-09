using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.LifeSkills.Cooking;
using ThinhThan.UI.LifeSkills.Cooking;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.CookingBonfirePresentation
{
    /// <summary>
    /// IMP-059 client facet: KINDLE / COOK / BONFIRE_REST intents mint
    /// UUIDv7 operation ids and ship the right 103 fields; recorded 116
    /// results drive the bonfire/rest/grants presentation; the Rượu Nếp
    /// buff tracks its 205 status stream; the panel projects craftable
    /// rows from inventory counts.
    /// </summary>
    public sealed class CookingBonfirePresentationTests
    {
        private sealed class FakeSender : ICookingSender
        {
            public readonly List<(uint MessageId, IMessage Payload)> Sent =
                new List<(uint, IMessage)>();

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult((ulong)Sent.Count);
                return src.Awaitable;
            }
        }

        private static DecodedFrame Frame(uint messageId, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = messageId,
                Payload = payload,
            };
        }

        private static S2CInteractResult Result(
            InteractKind kind, ResultStatus status, ErrorCode code)
        {
            return new S2CInteractResult
            {
                Result = new OperationResult
                {
                    Status = status,
                    ErrorCode = code,
                },
                InteractKind = kind,
            };
        }

        /// <summary>103 intents carry kind + target + minted op id;
        /// COOK additionally carries recipe_id.</summary>
        [Test]
        public void IntentsSendInteractKinds()
        {
            var sender = new FakeSender();
            var op = new byte[16];
            op[0] = 9;
            op[1] = 8;
            op[2] = 7;
            var intents = new CookingIntents(sender, () => op);

            _ = intents.RequestKindle("bonfire", CancellationToken.None);
            _ = intents.RequestCook("hearth", "recipe.food.tom_nuong",
                CancellationToken.None);
            _ = intents.RequestBonfireRest("bonfire", CancellationToken.None);

            Assert.AreEqual(3, sender.Sent.Count);
            var kindle = (C2SInteract)sender.Sent[0].Payload;
            Assert.AreEqual((uint)103, sender.Sent[0].MessageId);
            Assert.AreEqual(InteractKind.Kindle, kindle.InteractKind);
            Assert.AreEqual("bonfire", kindle.TargetId);
            Assert.AreEqual(16, kindle.OperationId.Length);
            Assert.IsFalse(kindle.HasRecipeId);
            var cook = (C2SInteract)sender.Sent[1].Payload;
            Assert.AreEqual(InteractKind.Cook, cook.InteractKind);
            Assert.AreEqual("recipe.food.tom_nuong", cook.RecipeId);
            var rest = (C2SInteract)sender.Sent[2].Payload;
            Assert.AreEqual(InteractKind.BonfireRest, rest.InteractKind);
        }

        /// <summary>KINDLE SUCCESS activates the bonfire section; the
        /// sink ignores non-cooking kinds and failed kinds mark
        /// FailureCode.</summary>
        [Test]
        public void KindleResultActivatesBonfire()
        {
            var presentation = new CookingPresentation();
            var sink = new CookingSinkApplier(presentation);

            presentation.BeginRequest(InteractKind.Kindle);
            sink.Apply(Frame(116, Result(
                InteractKind.Kindle, ResultStatus.Success,
                ErrorCode.Unspecified)));

            Assert.AreEqual(
                CookingPresentation.ActionState.Succeeded,
                presentation.Kindle);
            Assert.IsTrue(presentation.BonfireActive);

            // A foreign kind on the shared 116 frame is ignored.
            presentation.BeginRequest(InteractKind.Cook);
            sink.Apply(Frame(116, Result(
                InteractKind.Talk, ResultStatus.Success,
                ErrorCode.Unspecified)));
            Assert.AreEqual(
                CookingPresentation.ActionState.Pending,
                presentation.Cook);

            // Failed kindle leaves the bonfire inactive and reports the code.
            var fresh = new CookingPresentation();
            fresh.BeginRequest(InteractKind.Kindle);
            fresh.Apply(Result(InteractKind.Kindle, ResultStatus.Error,
                ErrorCode.StateConflict));
            Assert.AreEqual(
                CookingPresentation.ActionState.Failed, fresh.Kindle);
            Assert.AreEqual(ErrorCode.StateConflict, fresh.FailureCode);
            Assert.IsFalse(fresh.BonfireActive);
        }

        /// <summary>COOK SUCCESS records the guaranteed granted rows —
        /// the food output plus the kindling extra.</summary>
        [Test]
        public void CookResultGrantsFoodAndKindling()
        {
            var presentation = new CookingPresentation();
            presentation.BeginRequest(InteractKind.Cook);
            var result = Result(InteractKind.Cook, ResultStatus.Success,
                ErrorCode.Unspecified);
            result.Granted.Add(new ItemQuantity
            {
                ItemId = "item.consumable.food.tom_nuong",
                Quantity = 1,
            });
            result.Granted.Add(new ItemQuantity
            {
                ItemId = CookingRecipes.ExtraKindlingItemId,
                Quantity = 1,
            });
            presentation.Apply(result);

            Assert.AreEqual(
                CookingPresentation.ActionState.Succeeded,
                presentation.Cook);
            Assert.AreEqual(2, presentation.Granted.Count);
            Assert.AreEqual("item.consumable.food.tom_nuong",
                presentation.Granted[0].ItemId);
            Assert.AreEqual(CookingRecipes.ExtraKindlingItemId,
                presentation.Granted[1].ItemId);
        }

        /// <summary>BONFIRE_REST SUCCESS toggles the rest session; the
        /// movement hook ends it without a wire result.</summary>
        [Test]
        public void RestTogglesAndMovementEnds()
        {
            var presentation = new CookingPresentation();
            presentation.BeginRequest(InteractKind.BonfireRest);
            presentation.Apply(Result(InteractKind.BonfireRest,
                ResultStatus.Success, ErrorCode.Unspecified));
            Assert.IsTrue(presentation.Resting);
            Assert.AreEqual(
                CookingPresentation.ActionState.Succeeded,
                presentation.Rest);

            // Stand-up toggle through a second admitted result.
            presentation.BeginRequest(InteractKind.BonfireRest);
            presentation.Apply(Result(InteractKind.BonfireRest,
                ResultStatus.Success, ErrorCode.Unspecified));
            Assert.IsFalse(presentation.Resting);

            // Rest again, then a movement break ends it locally.
            presentation.Apply(Result(InteractKind.BonfireRest,
                ResultStatus.Success, ErrorCode.Unspecified));
            Assert.IsTrue(presentation.Resting);
            presentation.EndRest();
            Assert.IsFalse(presentation.Resting);
        }

        /// <summary>buff.ruou_nep_am_long APPLIED arms the buff at the
        /// reported expiry; EXPIRED clears it; foreign effect ids and
        /// other targets are ignored.</summary>
        [Test]
        public void BuffTracksStatusEvents()
        {
            var buff = new CookingBuffTracker(selfEntityId: 77);
            buff.Apply(new S2CStatusEvent
            {
                TargetEntityId = 77,
                EffectId = CookingBuffTracker.RuouNepEffectId,
                Event = StatusEventKind.Applied,
                ExpiresAtTick = 36000,
                ServerTick = 100,
            });
            Assert.IsTrue(buff.Active);
            Assert.AreEqual((ulong)36000, buff.ExpiresAtTick);
            Assert.AreEqual((ulong)35900, buff.RemainingTicks(100));

            // Foreign target/effect do not touch the state.
            buff.Apply(new S2CStatusEvent
            {
                TargetEntityId = 5,
                EffectId = CookingBuffTracker.RuouNepEffectId,
                Event = StatusEventKind.Expired,
            });
            buff.Apply(new S2CStatusEvent
            {
                TargetEntityId = 77,
                EffectId = "buff.other",
                Event = StatusEventKind.Expired,
            });
            Assert.IsTrue(buff.Active);

            buff.Apply(new S2CStatusEvent
            {
                TargetEntityId = 77,
                EffectId = CookingBuffTracker.RuouNepEffectId,
                Event = StatusEventKind.Expired,
            });
            Assert.IsFalse(buff.Active);
            Assert.AreEqual((ulong)0, buff.RemainingTicks(0));
        }

        /// <summary>The panel projects craftable rows from the
        /// inventory lookup and mirrors bonfire/rest/buff section
        /// state.</summary>
        [Test]
        public void PresenterProjectsCraftableRows()
        {
            var panel = new CookingPanel();
            var presentation = new CookingPresentation();
            var buff = new CookingBuffTracker(selfEntityId: 1);
            var inv = new Dictionary<string, uint>
            {
                ["item.material.tom_song"] = 1,
                ["item.material.rau_ram"] = 1,
                ["item.material.ca_chep"] = 1,
                ["item.material.gung_lang"] = 0,
            };
            var presenter = new CookingPresenter(
                panel, presentation, buff, id =>
                    inv.TryGetValue(id, out uint q) ? q : 0);

            presenter.ServerTick = 50;
            presenter.Apply();

            CookingPanelModel m = panel.Current!;
            Assert.AreEqual(CookingRecipes.All.Length, m.Rows.Count);
            // tom_nuong inputs fully stocked.
            Assert.IsTrue(m.Rows[2].Craftable);
            Assert.AreEqual("item.consumable.food.tom_nuong",
                m.Rows[2].OutputItemId);
            // ruou_nep needs 2 ca_chep + 1 gung_lang: short by 2 units.
            Assert.IsFalse(m.Rows[4].Craftable);
            Assert.AreEqual((uint)2, m.Rows[4].MissingCount);

            // Section state mirrors the model.
            presentation.Apply(Result(InteractKind.Kindle,
                ResultStatus.Success, ErrorCode.Unspecified));
            presentation.Apply(Result(InteractKind.BonfireRest,
                ResultStatus.Success, ErrorCode.Unspecified));
            buff.Apply(new S2CStatusEvent
            {
                TargetEntityId = 1,
                EffectId = CookingBuffTracker.RuouNepEffectId,
                Event = StatusEventKind.Applied,
                ExpiresAtTick = 36050,
            });
            presenter.Apply();
            m = panel.Current!;
            Assert.IsTrue(m.BonfireActive);
            Assert.IsTrue(m.Resting);
            Assert.IsTrue(m.BuffActive);
            Assert.AreEqual((ulong)36000, m.BuffRemainingTicks);
        }
    }
}
