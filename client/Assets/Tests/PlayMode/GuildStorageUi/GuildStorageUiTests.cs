using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.GuildStorage;
using ThinhThan.UI.GuildStorage;
using UnityEngine;
using TMPro;

namespace ThinhThan.Tests.PlayMode.GuildStorageUi
{
    /// <summary>
    /// GuildStorage UI: snapshot render + deposit/withdraw/claim
    /// rejection and retry verdict surfaces (packet-named coverage).
    /// </summary>
    public sealed class GuildStorageUiTests
    {
        private static byte[] Id16(byte seed)
        {
            var b = new byte[16];
            b[0] = seed;
            return b;
        }

        private static DecodedFrame Frame(uint id, object payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = (IMessage)payload,
            };
        }

        private static S2CGuildStorageState State(ulong rev)
        {
            var s = new S2CGuildStorageState
            {
                GuildId = ByteString.CopyFrom(Id16(9)),
                StorageRevision = rev,
            };
            s.Items.Add(new GuildStorageItemView
            {
                ItemInstanceId = ByteString.CopyFrom(Id16(1)),
                ItemId = "item.test",
                Quantity = 3,
                Section = GuildStorageSection.Common,
            });
            s.Items.Add(new GuildStorageItemView
            {
                ItemInstanceId = ByteString.CopyFrom(Id16(2)),
                ItemId = "item.rare",
                Quantity = 1,
                Section = GuildStorageSection.Reserve,
            });
            return s;
        }

        private static S2CGuildResult Verdict(
            ResultStatus status, ErrorCode code, uint reqId)
        {
            return new S2CGuildResult
            {
                RequestMessageId = reqId,
                Result = new OperationResult
                {
                    OperationId = ByteString.CopyFrom(Id16(1)),
                    Status = status,
                    ErrorCode = code,
                },
            };
        }

        private static GuildStoragePanel Panel(GuildStorageApplier a)
        {
            var go = new GameObject("StoragePanel", typeof(GuildStoragePanel));
            var p = go.GetComponent<GuildStoragePanel>();
            p.Applier = a;
            p.CommonText = new GameObject("Common").AddComponent<TMP_Text>();
            p.ReserveText = new GameObject("Reserve").AddComponent<TMP_Text>();
            p.ClaimsText = new GameObject("Claims").AddComponent<TMP_Text>();
            p.ResultText = new GameObject("Result").AddComponent<TMP_Text>();
            return p;
        }

        [Test]
        public void StorageSnapshotRendersBothSections()
        {
            var a = new GuildStorageApplier();
            a.Apply(Frame(GuildStorageApplier.S2CGuildStorageState, State(7)));
            Assert.AreEqual(2, a.State.Items.Count);
            Assert.AreEqual(1, a.State.SectionCount(GuildStorageSection.Reserve));
            Assert.AreEqual(7UL, a.State.StorageRevision);

            var p = Panel(a);
            p.RenderIfDirty();
            Assert.IsTrue(p.CommonText!.text.Contains("item.test"));
            Assert.IsTrue(p.ReserveText!.text.Contains("item.rare"));
            Assert.IsFalse(p.ReserveText!.text.Contains("item.test"));
            Object.Destroy(p.gameObject);
        }

        [Test]
        public void DepositRejectionSurfacesVerdict()
        {
            var a = new GuildStorageApplier();
            a.Apply(Frame(GuildStorageApplier.S2CGuildResult,
                Verdict(ResultStatus.Error, ErrorCode.CapacityFull, 629)));
            Assert.AreEqual(ResultStatus.Error, a.LastResult!.Result!.Status);
            Assert.AreEqual(ErrorCode.CapacityFull,
                a.LastResult!.Result!.ErrorCode);
            Assert.AreEqual(629u, a.LastResult!.RequestMessageId);

            var p = Panel(a);
            p.RenderIfDirty();
            Assert.IsTrue(p.ResultText!.text.Contains("Error"));
            Object.Destroy(p.gameObject);
        }

        [Test]
        public void WithdrawRejectionThenRetrySuccess()
        {
            var a = new GuildStorageApplier();
            // Same-account withdraw rejected, then a retried request
            // (e.g. after switching to an allowed character) succeeds.
            a.Apply(Frame(GuildStorageApplier.S2CGuildResult,
                Verdict(ResultStatus.Error,
                    ErrorCode.GuildStorageSameAccount, 630)));
            Assert.AreEqual(ErrorCode.GuildStorageSameAccount,
                a.LastResult!.Result!.ErrorCode);

            a.Apply(Frame(GuildStorageApplier.S2CGuildResult,
                Verdict(ResultStatus.Success, ErrorCode.Unspecified, 630)));
            Assert.AreEqual(ResultStatus.Success, a.LastResult!.Result!.Status);
            Assert.AreEqual(630u, a.LastResult!.RequestMessageId);
        }

        [Test]
        public void ClaimListAppliesAndDecideRejectionSurfaces()
        {
            var a = new GuildStorageApplier();
            var claims = new S2CGuildStorageClaims
            {
                GuildId = ByteString.CopyFrom(Id16(9)),
                StorageRevision = 9,
            };
            claims.Claims.Add(new GuildStorageClaimView
            {
                ClaimId = ByteString.CopyFrom(Id16(3)),
                RequesterCharacterId = ByteString.CopyFrom(Id16(4)),
                ItemInstanceId = ByteString.CopyFrom(Id16(2)),
                Quantity = 2,
                State = GuildStorageClaimState.Approved,
            });
            a.Apply(Frame(GuildStorageApplier.S2CGuildStorageClaims, claims));
            Assert.AreEqual(1, a.State.Claims.Count);
            Assert.AreEqual(GuildStorageClaimState.Approved,
                a.State.Claims[0].State);

            // Non-requester DELIVER rejected with PERMISSION_DENIED.
            a.Apply(Frame(GuildStorageApplier.S2CGuildResult,
                Verdict(ResultStatus.Error, ErrorCode.PermissionDenied, 646)));
            Assert.AreEqual(ErrorCode.PermissionDenied,
                a.LastResult!.Result!.ErrorCode);
            Assert.AreEqual(646u, a.LastResult!.RequestMessageId);

            var p = Panel(a);
            p.RenderIfDirty();
            Assert.IsTrue(p.ClaimsText!.text.Contains("Approved"));
            Object.Destroy(p.gameObject);
        }
    }
}
