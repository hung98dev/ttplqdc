using NUnit.Framework;
using ThinhThan.Systems.Skills;
using ThinhThan.UI.Skills;

namespace ThinhThan.Tests.PlayMode.SkillUi
{
    /// <summary>
    /// IMP-015 PlayMode: skill telegraphs present the resolved
    /// authoritative geometry (never sprite bounds) for every compiled
    /// launch skill; cost/cooldown/targeting presentation follows the
    /// resolved descriptor.
    /// </summary>
    public sealed class SkillUiTests
    {
        private static readonly string[] AllSkillIds =
        {
            "skill.kim.basic.kiem_thuc",
            "skill.kim.basic.truy_phong_kiem",
            "skill.kim.basic.pha_khong_kiem",
            "skill.kim.basic.vo_song_kiem",
            "skill.kim.active.xuyen_phong",
            "skill.kim.active.hoi_kiem",
            "skill.kim.active.pha_giap",
            "skill.kim.active.kiem_tran",
            "skill.kim.active.nhat_kiem_dinh_hon",
            "skill.moc.basic.linh_diep",
            "skill.moc.basic.thao_kich",
            "skill.moc.basic.truc_phi_tieu",
            "skill.moc.basic.co_thu_kich",
            "skill.moc.active.moc_bo",
            "skill.moc.active.hoi_xuan",
            "skill.moc.active.thanh_dang",
            "skill.moc.active.van_doc",
            "skill.moc.active.van_moc_hoi_sinh",
            "skill.thuy.basic.thuy_tien",
            "skill.thuy.basic.bang_phien",
            "skill.thuy.basic.am_luu",
            "skill.thuy.basic.huyen_bang_kich",
            "skill.thuy.active.luu_bo",
            "skill.thuy.active.trieu_quyen",
            "skill.thuy.active.thuy_kinh",
            "skill.thuy.active.han_trieu",
            "skill.thuy.active.thien_ha",
            "skill.hoa.basic.hoa_phu",
            "skill.hoa.basic.viem_dan",
            "skill.hoa.basic.hoa_xa",
            "skill.hoa.basic.lua_tao_quan",
            "skill.hoa.active.boc_bo",
            "skill.hoa.active.lien_bao",
            "skill.hoa.active.hoa_giap",
            "skill.hoa.active.hoa_vuc",
            "skill.hoa.active.cuu_hoa_lien",
            "skill.tho.basic.tran_quyen",
            "skill.tho.basic.pha_thach_kich",
            "skill.tho.basic.dia_liet_kich",
            "skill.tho.basic.kim_cang_quyen",
            "skill.tho.active.thach_kich",
            "skill.tho.active.tho_giap",
            "skill.tho.active.dia_chan",
            "skill.tho.active.son_bich",
            "skill.tho.active.thien_son_tran",
        };

        /// <summary>Every compiled launch skill resolves a telegraph
        /// from the authoritative geometry — never sprite bounds.</summary>
        [Test]
        public void TestSkillTelegraphsUseResolvedGeometry()
        {
            var tg = new SkillTelegraphs();
            Assert.AreEqual(45, AllSkillIds.Length);
            foreach (string id in AllSkillIds)
            {
                Assert.IsTrue(
                    tg.TryDescribe(id, 10000, 4000, false, 17000, 4000,
                        out SkillTelegraphs.TelegraphDescriptor d),
                    "telegraph must resolve for " + id);
                Assert.IsTrue(d.Resolved, id);
            }

            // MELEE_BOX: telegraph spans the authored reach, anchored at
            // the resolved origin (anchor + 0.9m) — not any sprite width.
            tg.TryDescribe("skill.kim.basic.kiem_thuc", 10000, 4000, false, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor melee);
            Assert.AreEqual(10000, melee.Primitive.MinX);
            Assert.AreEqual(11900, melee.Primitive.MaxX);
            Assert.AreEqual(4900 - 900, melee.Primitive.MinY);
            Assert.AreEqual(4900 + 900, melee.Primitive.MaxY);

            // Facing-left mirrors the resolved box.
            tg.TryDescribe("skill.kim.basic.kiem_thuc", 10000, 4000, true, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor left);
            Assert.AreEqual(8100, left.Primitive.MinX);
            Assert.AreEqual(10000, left.Primitive.MaxX);

            // Lưu Bộ MOVE_CONTACT_LINE: the sweep path telegraph spans
            // the authored 4.5m, half-height 1.0m.
            tg.TryDescribe("skill.thuy.active.luu_bo", 10000, 4000, false, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor luu);
            Assert.AreEqual(10000, luu.Primitive.PathMinX);
            Assert.AreEqual(14500, luu.Primitive.PathMaxX);
            Assert.AreEqual(4900 - 1000, luu.Primitive.MinY);
            Assert.AreEqual(4900 + 1000, luu.Primitive.MaxY);

            // Sơn Bích BARRIER_POSITION: telegraph is the authored
            // barrier AABB at the cast anchor (0.8m x 4.0m).
            tg.TryDescribe("skill.tho.active.son_bich", 10000, 4000, false, 16000, 4000,
                out SkillTelegraphs.TelegraphDescriptor wall);
            Assert.AreEqual(15600, wall.Primitive.MinX);
            Assert.AreEqual(16400, wall.Primitive.MaxX);
            Assert.AreEqual(4000, wall.Primitive.MinY);
            Assert.AreEqual(8000, wall.Primitive.MaxY);

            // Preview stages exactly one resolved primitive; Clear drops it.
            Assert.IsTrue(tg.Preview("skill.thuy.active.han_trieu", 0, 0, false, 0, 0));
            Assert.AreEqual(1, tg.Primitives.Count);
            tg.Clear();
            Assert.AreEqual(0, tg.Primitives.Count);

            // Unknown ids never resolve.
            Assert.IsFalse(tg.Preview("skill.kim.passive.kiem_tam", 0, 0, false, 0, 0));
            Assert.AreEqual(0, tg.Primitives.Count);
        }

        /// <summary>Resolved descriptors carry the targeting/cost
        /// presentation the HUD needs: area casts present the authored
        /// circle at the requested center; single-target presents the
        /// authored range circle; outer reach follows the resolved
        /// geometry.</summary>
        [Test]
        public void TestSkillTelegraphTargetingPresentation()
        {
            var tg = new SkillTelegraphs();

            // AREA_POSITION: telegraph center is the requested point.
            tg.TryDescribe("skill.hoa.active.lien_bao", 10000, 4000, false, 17000, 4000,
                out SkillTelegraphs.TelegraphDescriptor area);
            Assert.AreEqual(17000, area.Primitive.CenterX);
            Assert.AreEqual(4000, area.Primitive.CenterY);
            Assert.AreEqual(2800, area.Primitive.RadiusMM);

            // SINGLE_TARGET_RANGE: authored range circle on the origin.
            tg.TryDescribe("skill.kim.active.nhat_kiem_dinh_hon", 10000, 4000, false, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor sig);
            Assert.AreEqual(10000, sig.Primitive.CenterX);
            Assert.AreEqual(4900, sig.Primitive.CenterY);
            Assert.AreEqual(2600, sig.Primitive.RadiusMM);

            // SELF: a resolved descriptor with no spatial primitive.
            tg.TryDescribe("skill.thuy.active.thuy_kinh", 10000, 4000, false, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor self);
            Assert.IsTrue(self.Resolved);
            Assert.AreEqual(SkillGeometry.ShapeKind.Self, self.Primitive.Kind);
            Assert.AreEqual(0, self.OuterReachMM);
        }

        /// <summary>Cooldown/cost presentation: the resolved descriptor
        /// exposes the authored reach so the HUD binds cooldown slots
        /// and cost text to the same geometry-driven skill identity —
        /// the presentation never invents reach of its own.</summary>
        [Test]
        public void TestSkillTelegraphCooldownCostPresentation()
        {
            var tg = new SkillTelegraphs();
            tg.TryDescribe("skill.tho.active.son_bich", 10000, 4000, false, 16000, 4000,
                out SkillTelegraphs.TelegraphDescriptor wall);
            Assert.AreEqual(800, wall.OuterReachMM); // barrier thickness extent

            tg.TryDescribe("skill.thuy.active.han_trieu", 10000, 4000, false, 0, 0,
                out SkillTelegraphs.TelegraphDescriptor wide);
            Assert.AreEqual(5500, wide.OuterReachMM); // direction-box reach
        }
    }
}
