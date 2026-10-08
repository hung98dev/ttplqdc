using NUnit.Framework;
using ThinhThan.Protocol.V1;
using ThinhThan.UI.Discovery;

namespace ThinhThan.Tests.PlayMode.DiscoveryPresentation
{
    /// <summary>
    /// IMP-020 PlayMode: a committed DISCOVERY progression event
    /// presents one first-discovery notice (map + EXP + level_after),
    /// and redelivered events for the same reference_id are suppressed
    /// — matching the server's once-only grant.
    /// </summary>
    public sealed class DiscoveryPresentationTests
    {
        private static S2CProgressionEvent DiscoveryEvent(string mapId, long exp, uint levelAfter)
        {
            return new S2CProgressionEvent
            {
                EventKind = ProgressionEventKind.Discovery,
                SourceKind = "DISCOVERY",
                ReferenceId = mapId,
                Amount = exp,
                LevelAfter = levelAfter,
                ServerTimeMs = 12345,
            };
        }

        [Test]
        public void FirstDiscoveryEvent_PresentsNotice()
        {
            var panel = new DiscoveryNoticePanel();
            var presenter = new DiscoveryPresenter(panel);

            Assert.IsTrue(presenter.Apply(DiscoveryEvent("map.lang_da.dinh_lang", 7700, 2)));
            Assert.AreEqual(1, panel.Count);
            Assert.IsTrue(panel.Current.HasValue);
            var shown = panel.Current.GetValueOrDefault();
            Assert.AreEqual("map.lang_da.dinh_lang", shown.MapId);
            Assert.AreEqual(7700, shown.Exp);
            Assert.AreEqual(2, shown.LevelAfter);
        }

        [Test]
        public void DuplicateDiscoveryEvent_IsSuppressed()
        {
            var panel = new DiscoveryNoticePanel();
            var presenter = new DiscoveryPresenter(panel);

            Assert.IsTrue(presenter.Apply(DiscoveryEvent("map.lang_da.dinh_lang", 7700, 2)));
            // Reconnect restore / redelivery: same reference_id never re-presents.
            Assert.IsFalse(presenter.Apply(DiscoveryEvent("map.lang_da.dinh_lang", 7700, 2)));
            Assert.IsFalse(presenter.Apply(DiscoveryEvent("map.lang_da.dinh_lang", 7700, 2)));
            Assert.AreEqual(1, panel.Count);
            Assert.AreEqual(2, presenter.Suppressed);

            // A different map's first discovery still presents.
            Assert.IsTrue(presenter.Apply(DiscoveryEvent("map.lang_da.bo_ruong", 7700, 2)));
            Assert.AreEqual(2, panel.Count);
        }

        [Test]
        public void NonDiscoveryEvents_AreIgnored()
        {
            var panel = new DiscoveryNoticePanel();
            var presenter = new DiscoveryPresenter(panel);

            Assert.IsFalse(presenter.Apply(new S2CProgressionEvent
            {
                EventKind = ProgressionEventKind.ExpGained,
                SourceKind = "KILL",
                ReferenceId = "monster.lang_da.coc_thanh_tinh",
                Amount = 100,
            }));
            // A DISCOVERY event kind with a non-DISCOVERY source is not a
            // first-discovery presentation either.
            Assert.IsFalse(presenter.Apply(new S2CProgressionEvent
            {
                EventKind = ProgressionEventKind.Discovery,
                SourceKind = "QUEST",
                ReferenceId = "map.lang_da.dinh_lang",
            }));
            Assert.AreEqual(0, panel.Count);
        }
    }
}
