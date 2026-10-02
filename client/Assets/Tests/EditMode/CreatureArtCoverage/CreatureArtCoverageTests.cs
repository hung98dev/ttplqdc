using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.CreatureArtCoverage
{
    /// <summary>
    /// IMP-104 creature-art coverage (presentation_asset_manifest.md §3
    /// NPC rule, §3.7 animation contract, §3.8 style packs, §5 provenance
    /// + folklore cards; monster_catalog/boss_catalog/spirit_beast_catalog/
    /// npc_shop_catalog rosters; physics_geometry_contract.md §3 sizes):
    /// every launch creature/NPC id resolves to a prefab or a declared
    /// one-hop PresentationAlias, clips match the size-profile contract,
    /// import settings match the profile table, and the provenance
    /// fragment carries a folklore_card free of banned motifs.
    /// </summary>
    public class CreatureArtCoverageTests
    {
        private const string FragmentPath =
            "client/Assets/Art/Provenance/fragments/actors_creatures.json";
        private const string ActorsRoot = "client/Assets/Art/Actors";
        private const string StyleRoot =
            "client/Assets/Art/StyleRef/actors_creatures";
        private const string GroupsDir =
            "client/Assets/AddressableAssetsData/AssetGroups";

        // id|kind|profile|group|technique|aliasTarget|clipList|phases
        private const string RosterText = @"beast.hoa.ga_than|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.hoa.hoa_diep|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.kim.ho_vang|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.kim.nghe_dong|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.moc.chim_lac|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.moc.huou_sao|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.tho.coc_than|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.tho.trau_dong|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.thuy.rai_ca|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
beast.thuy.rua_than|beast|SPIRIT_BEAST|beast.shared|frame_by_frame||idle,move,cast|1
boss.ho_tinh_chin_duoi|boss|BOSS_LARGE|dungeon.den_tran|skeletal||idle,move,attack_cuu_anh,attack_lua_ma,attack_lo_chan_than,attack_hoi_phuc,attack_chay_rua,hit,defeat,phase_transition_2|2
boss.ho_tinh|boss|BOSS_LARGE|dungeon.hang_ma_tranh|skeletal||idle,move,attack_vet_vuot,attack_vo_moi,attack_uy_son,attack_lan_vo_moi,attack_chay_tan,hit,defeat,phase_transition_2|2
boss.ma_da_chua|boss|WORLD_BOSS|region.ben_nuoc_den|skeletal||idle,move,attack_song_day,attack_keo_chim,attack_tran_nuoc,attack_thuy_trieu,attack_song_than,attack_cuon_nuoc,hit,defeat,phase_transition_2|2
boss.moc_tinh_da|boss|BOSS_LARGE|dungeon.mieu_ba_trong_rung|skeletal||idle,move,attack_re_gia,attack_mam_am,attack_thu_than,attack_bung_no,attack_thuc_tinh,hit,defeat,phase_transition_2|2
boss.ngu_tinh|boss|WORLD_BOSS|region.nui_thieng|skeletal||idle,move,attack_quet_nuoc,attack_manh_vay,attack_fragment,attack_phun_mau_vay,attack_cuoc_bung,hit,defeat,phase_transition_2|2
boss.quy_nhap_trang|boss|BOSS_LARGE|dungeon.dinh_lang_bo_hoang|skeletal||idle,move,attack_bat_day,attack_duoi_den,attack_bat_day_gian,attack_chiem_hon,hit,defeat,phase_transition_2|2
boss.than_trung|boss|WORLD_BOSS|dungeon.finale|skeletal||idle,move,attack_lang_da,attack_rung_u_minh,attack_ben_nuoc_den,attack_deo_may,attack_thanh_co,attack_nui_thieng,hit,defeat,phase_transition_2,phase_transition_3|3
boss.thuong_luong|boss|BOSS_LARGE|dungeon.xom_chim|skeletal||idle,move,attack_quet_duoi,attack_nuoc_dang,attack_cuon_song,attack_vung_du,attack_cuon_nuoc,hit,defeat,phase_transition_2|2
monster.ben_nuoc_den.bong_nuoc_ma|monster|MONSTER_SMALL|region.ben_nuoc_den|frame_by_frame||idle,move,attack_shot,attack_burst,hit,defeat|1
monster.ben_nuoc_den.ca_tinh_gia|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal|monster.ben_nuoc_den.ca_tinh|idle,move,attack_basic,attack_rush,hit,defeat|1
monster.ben_nuoc_den.ca_tinh|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.ben_nuoc_den.hon_chet_duoi|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_shot,hit,defeat|1
monster.ben_nuoc_den.ma_da_gia|monster|MONSTER_ELITE|region.ben_nuoc_den|skeletal||idle,move,attack_basic,attack_control_hit,attack_slam,hit,defeat|1
monster.ben_nuoc_den.ma_da|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.ben_nuoc_den.nguoi_song_co|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_shot,hit,defeat|1
monster.ben_nuoc_den.quy_song_dem|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_shot,hit,defeat|1
monster.ben_nuoc_den.thuong_luong|monster|MONSTER_MEDIUM|region.ben_nuoc_den|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.ben_nuoc_den.thuy_quai|monster|MONSTER_ELITE|region.ben_nuoc_den|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.deo_may.ho_con_tinh|monster|MONSTER_MEDIUM|region.deo_may|skeletal|monster.deo_may.ho_tinh|idle,move,attack_basic,attack_rush,hit,defeat|1
monster.deo_may.ho_tinh_lon|monster|MONSTER_MEDIUM|region.deo_may|skeletal|monster.deo_may.ho_tinh|idle,move,attack_basic,attack_rush,hit,defeat|1
monster.deo_may.ho_tinh_ve|monster|MONSTER_ELITE|region.deo_may|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.deo_may.ho_tinh|monster|MONSTER_MEDIUM|region.deo_may|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.deo_may.khi_nui|monster|MONSTER_MEDIUM|region.deo_may|skeletal||idle,move,attack_shot,hit,defeat|1
monster.deo_may.ma_tranh_gia|monster|MONSTER_ELITE|region.deo_may|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.deo_may.ma_tranh|monster|MONSTER_MEDIUM|region.deo_may|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.deo_may.ma_van_dem|monster|MONSTER_SMALL|region.deo_may|frame_by_frame||idle,move,attack_shot,attack_burst,hit,defeat|1
monster.deo_may.vong_nui_gia|monster|MONSTER_MEDIUM|region.deo_may|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.deo_may.vong_rung|monster|MONSTER_MEDIUM|region.deo_may|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.lang_da.bu_nhin_rom|monster|MONSTER_MEDIUM|region.lang_da|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.lang_da.coc_thanh_tinh|monster|MONSTER_SMALL|region.lang_da|frame_by_frame||idle,move,attack_shot,hit,defeat|1
monster.lang_da.dom_dom_ma|monster|MONSTER_SMALL|region.lang_da|frame_by_frame||idle,move,attack_shot,attack_burst,hit,defeat|1
monster.lang_da.hon_do_trang|monster|MONSTER_MEDIUM|region.lang_da|skeletal||idle,move,attack_basic,hit,defeat|1
monster.lang_da.hon_ma_co_thu|monster|MONSTER_MEDIUM|region.lang_da|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.lang_da.hon_xo_non|monster|MONSTER_ELITE|region.lang_da|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.lang_da.ma_xo|monster|MONSTER_ELITE|region.lang_da|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.lang_da.quy_nhap_trang|monster|MONSTER_MEDIUM|region.lang_da|skeletal||idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.lang_da.vong_hon_gia|monster|MONSTER_MEDIUM|region.lang_da|skeletal|monster.lang_da.vong_hon|idle,move,attack_basic,hit,defeat|1
monster.lang_da.vong_hon|monster|MONSTER_MEDIUM|region.lang_da|skeletal||idle,move,attack_basic,hit,defeat|1
monster.nui_thieng.bong_vong|monster|MONSTER_ELITE|region.nui_thieng|skeletal||idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.nui_thieng.dai_vong_linh|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal|monster.nui_thieng.vong_linh|idle,move,attack_basic,attack_rush,hit,defeat|1
monster.nui_thieng.hon_binh_co|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,hit,defeat|1
monster.nui_thieng.linh_ve|monster|MONSTER_ELITE|region.nui_thieng|skeletal||idle,move,attack_basic,attack_guard,attack_counter,hit,defeat|1
monster.nui_thieng.ma_nui|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.nui_thieng.ngu_tinh|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_shot,hit,defeat|1
monster.nui_thieng.than_rung_dem|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.nui_thieng.than_trung|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.nui_thieng.tinh_nui_gia|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.nui_thieng.tinh_thu|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_shot,hit,defeat|1
monster.nui_thieng.vong_linh|monster|MONSTER_MEDIUM|region.nui_thieng|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.rung_u_minh.bong_nguoi|monster|MONSTER_MEDIUM|region.rung_u_minh|skeletal||idle,move,attack_basic,hit,defeat|1
monster.rung_u_minh.dai_tinh_cay|monster|MONSTER_MEDIUM|region.rung_u_minh|skeletal|monster.rung_u_minh.tinh_cay|idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.rung_u_minh.dom_lua|monster|MONSTER_SMALL|region.rung_u_minh|frame_by_frame||idle,move,attack_shot,attack_burst,hit,defeat|1
monster.rung_u_minh.ma_rung|monster|MONSTER_MEDIUM|region.rung_u_minh|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.rung_u_minh.ma_tranh|monster|MONSTER_ELITE|region.rung_u_minh|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.rung_u_minh.moc_tinh|monster|MONSTER_ELITE|region.rung_u_minh|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.rung_u_minh.tinh_cay|monster|MONSTER_MEDIUM|region.rung_u_minh|skeletal||idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.rung_u_minh.vong_rung_sau|monster|MONSTER_MEDIUM|region.rung_u_minh|skeletal||idle,move,attack_basic,attack_rush,hit,defeat|1
monster.thanh_co.hon_binh|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_basic,hit,defeat|1
monster.thanh_co.hon_tran_linh|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_basic,hit,defeat|1
monster.thanh_co.hon_tuong|monster|MONSTER_ELITE|region.thanh_co|skeletal||idle,move,attack_basic,attack_summon,hit,defeat|1
monster.thanh_co.ma_co|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_basic,attack_zone,hit,defeat|1
monster.thanh_co.oan_hon_dem|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_basic,attack_control_hit,hit,defeat|1
monster.thanh_co.qua_tinh_lon|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_shot,hit,defeat|1
monster.thanh_co.qua_tinh|monster|MONSTER_SMALL|region.thanh_co|frame_by_frame||idle,move,attack_shot,hit,defeat|1
monster.thanh_co.thach_ve|monster|MONSTER_ELITE|region.thanh_co|skeletal||idle,move,attack_basic,attack_guard,attack_counter,hit,defeat|1
monster.thanh_co.tuong_da|monster|MONSTER_MEDIUM|region.thanh_co|skeletal||idle,move,attack_basic,hit,defeat|1
npc.ben_nuoc_den.ambient_day|npc_shared|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame||idle,interact|1
npc.ben_nuoc_den.ambient_night|npc_shared|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame||idle,interact|1
npc.ben_nuoc_den.cho_1|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame|npc.ben_nuoc_den.ambient_day|idle,interact|1
npc.ben_nuoc_den.cho_2|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame|npc.ben_nuoc_den.ambient_day|idle,interact|1
npc.ben_nuoc_den.dem_1|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame|npc.ben_nuoc_den.ambient_night|idle,interact|1
npc.ben_nuoc_den.dem_2|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame|npc.ben_nuoc_den.ambient_night|idle,interact|1
npc.ben_nuoc_den.hang_quan|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame||idle,interact|1
npc.ben_nuoc_den.nguoi_dan_duong|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame||idle,interact|1
npc.ben_nuoc_den.tho_nghe|npc|NPC_HUMANOID|region.ben_nuoc_den|frame_by_frame||idle,interact|1
npc.deo_may.ambient_day|npc_shared|NPC_HUMANOID|region.deo_may|frame_by_frame||idle,interact|1
npc.deo_may.ambient_night|npc_shared|NPC_HUMANOID|region.deo_may|frame_by_frame||idle,interact|1
npc.deo_may.cho_1|npc|NPC_HUMANOID|region.deo_may|frame_by_frame|npc.deo_may.ambient_day|idle,interact|1
npc.deo_may.cho_2|npc|NPC_HUMANOID|region.deo_may|frame_by_frame|npc.deo_may.ambient_day|idle,interact|1
npc.deo_may.dem_1|npc|NPC_HUMANOID|region.deo_may|frame_by_frame|npc.deo_may.ambient_night|idle,interact|1
npc.deo_may.dem_2|npc|NPC_HUMANOID|region.deo_may|frame_by_frame|npc.deo_may.ambient_night|idle,interact|1
npc.deo_may.hang_quan|npc|NPC_HUMANOID|region.deo_may|frame_by_frame||idle,interact|1
npc.deo_may.nguoi_dan_duong|npc|NPC_HUMANOID|region.deo_may|frame_by_frame||idle,interact|1
npc.deo_may.tho_nghe|npc|NPC_HUMANOID|region.deo_may|frame_by_frame||idle,interact|1
npc.lang_da.ambient_day|npc_shared|NPC_HUMANOID|region.lang_da|frame_by_frame||idle,interact|1
npc.lang_da.ambient_night|npc_shared|NPC_HUMANOID|region.lang_da|frame_by_frame||idle,interact|1
npc.lang_da.cho_1|npc|NPC_HUMANOID|region.lang_da|frame_by_frame|npc.lang_da.ambient_day|idle,interact|1
npc.lang_da.cho_2|npc|NPC_HUMANOID|region.lang_da|frame_by_frame|npc.lang_da.ambient_day|idle,interact|1
npc.lang_da.dem_1|npc|NPC_HUMANOID|region.lang_da|frame_by_frame|npc.lang_da.ambient_night|idle,interact|1
npc.lang_da.dem_2|npc|NPC_HUMANOID|region.lang_da|frame_by_frame|npc.lang_da.ambient_night|idle,interact|1
npc.lang_da.hang_quan|npc|NPC_HUMANOID|region.lang_da|frame_by_frame||idle,interact|1
npc.lang_da.nguoi_dan_duong|npc|NPC_HUMANOID|region.lang_da|frame_by_frame||idle,interact|1
npc.lang_da.tho_nghe|npc|NPC_HUMANOID|region.lang_da|frame_by_frame||idle,interact|1
npc.nui_thieng.ambient_day|npc_shared|NPC_HUMANOID|region.nui_thieng|frame_by_frame||idle,interact|1
npc.nui_thieng.ambient_night|npc_shared|NPC_HUMANOID|region.nui_thieng|frame_by_frame||idle,interact|1
npc.nui_thieng.cho_1|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame|npc.nui_thieng.ambient_day|idle,interact|1
npc.nui_thieng.cho_2|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame|npc.nui_thieng.ambient_day|idle,interact|1
npc.nui_thieng.dem_1|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame|npc.nui_thieng.ambient_night|idle,interact|1
npc.nui_thieng.dem_2|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame|npc.nui_thieng.ambient_night|idle,interact|1
npc.nui_thieng.hang_quan|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame||idle,interact|1
npc.nui_thieng.nguoi_dan_duong|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame||idle,interact|1
npc.nui_thieng.tho_nghe|npc|NPC_HUMANOID|region.nui_thieng|frame_by_frame||idle,interact|1
npc.rung_u_minh.ambient_day|npc_shared|NPC_HUMANOID|region.rung_u_minh|frame_by_frame||idle,interact|1
npc.rung_u_minh.ambient_night|npc_shared|NPC_HUMANOID|region.rung_u_minh|frame_by_frame||idle,interact|1
npc.rung_u_minh.cho_1|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame|npc.rung_u_minh.ambient_day|idle,interact|1
npc.rung_u_minh.cho_2|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame|npc.rung_u_minh.ambient_day|idle,interact|1
npc.rung_u_minh.dem_1|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame|npc.rung_u_minh.ambient_night|idle,interact|1
npc.rung_u_minh.dem_2|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame|npc.rung_u_minh.ambient_night|idle,interact|1
npc.rung_u_minh.hang_quan|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame||idle,interact|1
npc.rung_u_minh.nguoi_dan_duong|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame||idle,interact|1
npc.rung_u_minh.tho_nghe|npc|NPC_HUMANOID|region.rung_u_minh|frame_by_frame||idle,interact|1
npc.thanh_co.ambient_day|npc_shared|NPC_HUMANOID|region.thanh_co|frame_by_frame||idle,interact|1
npc.thanh_co.ambient_night|npc_shared|NPC_HUMANOID|region.thanh_co|frame_by_frame||idle,interact|1
npc.thanh_co.cho_1|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame|npc.thanh_co.ambient_day|idle,interact|1
npc.thanh_co.cho_2|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame|npc.thanh_co.ambient_day|idle,interact|1
npc.thanh_co.dem_1|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame|npc.thanh_co.ambient_night|idle,interact|1
npc.thanh_co.dem_2|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame|npc.thanh_co.ambient_night|idle,interact|1
npc.thanh_co.hang_quan|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame||idle,interact|1
npc.thanh_co.nguoi_dan_duong|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame||idle,interact|1
npc.thanh_co.tho_nghe|npc|NPC_HUMANOID|region.thanh_co|frame_by_frame||idle,interact|1";

        // profile|silW|silH|cellW|cellH|colW,colH-or-empty
        private const string ProfilesText = @"MONSTER_SMALL|50|50|64|64|30,30
MONSTER_MEDIUM|75|100|96|128|50,70
MONSTER_ELITE|125|150|160|192|80,120
BOSS_LARGE|200|220|256|256|120,160
WORLD_BOSS|250|280|320|320|150,200
SPIRIT_BEAST|48|48|64|64|
NPC_HUMANOID|64|96|96|128|";

        private static readonly string[] ForbiddenMotifs =
        {
            "torii", "qing_robes", "jiangshi_hats", "kimono", "hanbok",
            "modern_religious_symbols", "modern_political_symbols",
            "meaningless_han_nom_chars",
        };

        private sealed class Roster
        {
            public string Id = string.Empty;
            public string Kind = string.Empty;
            public string Profile = string.Empty;
            public string Group = string.Empty;
            public string Technique = string.Empty;
            public string AliasTarget = string.Empty;
            public readonly List<string> Clips = new List<string>();
            public int Phases;
            public string Short = string.Empty;
        }

        private sealed class Profile
        {
            public int SilW;
            public int SilH;
            public int CellW;
            public int CellH;
            public int ColW;
            public int ColH;
        }

        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
            while (dir != null && !Directory.Exists(Path.Combine(dir, "docs")))
            {
                dir = Directory.GetParent(dir)?.FullName;
            }
            Assert.IsNotNull(dir, "repo root not found above Assets");
            return dir!;
        }

        private static string Abs(string rel)
        {
            return Path.Combine(RepoRoot(),
                rel.Replace('/', Path.DirectorySeparatorChar));
        }

        private static List<Roster> RosterRows()
        {
            var list = new List<Roster>();
            foreach (var line in RosterText.Split('\n'))
            {
                var t = line.Trim();
                if (t.Length == 0)
                {
                    continue;
                }
                var f = t.Split('|');
                Assert.AreEqual(8, f.Length, "bad roster line " + t);
                var r = new Roster();
                r.Id = f[0];
                r.Kind = f[1];
                r.Profile = f[2];
                r.Group = f[3];
                r.Technique = f[4];
                r.AliasTarget = f[5];
                foreach (var c in f[6].Split(','))
                {
                    if (c.Length > 0)
                    {
                        r.Clips.Add(c);
                    }
                }
                r.Phases = int.Parse(f[7], CultureInfo.InvariantCulture);
                r.Short = r.Id.Substring(r.Id.LastIndexOf('.') + 1);
                list.Add(r);
            }
            return list;
        }

        private static Dictionary<string, Profile> Profiles()
        {
            var map = new Dictionary<string, Profile>(StringComparer.Ordinal);
            foreach (var line in ProfilesText.Split('\n'))
            {
                var t = line.Trim();
                if (t.Length == 0)
                {
                    continue;
                }
                var f = t.Split('|');
                var p = new Profile();
                p.SilW = int.Parse(f[1], CultureInfo.InvariantCulture);
                p.SilH = int.Parse(f[2], CultureInfo.InvariantCulture);
                p.CellW = int.Parse(f[3], CultureInfo.InvariantCulture);
                p.CellH = int.Parse(f[4], CultureInfo.InvariantCulture);
                if (f[5].Length > 0)
                {
                    var c = f[5].Split(',');
                    p.ColW = int.Parse(c[0], CultureInfo.InvariantCulture);
                    p.ColH = int.Parse(c[1], CultureInfo.InvariantCulture);
                }
                map[f[0]] = p;
            }
            return map;
        }

        private static string MetaGuid(string assetRel)
        {
            var metaPath = Abs(assetRel + ".meta");
            Assert.IsTrue(File.Exists(metaPath), "missing .meta " + assetRel);
            var m = System.Text.RegularExpressions.Regex.Match(
                File.ReadAllText(metaPath), @"guid: ([0-9a-f]{32})");
            Assert.IsTrue(m.Success, "no guid in " + assetRel + ".meta");
            return m.Groups[1].Value;
        }

        private static List<(string Guid, string Address)> GroupEntries(
            string group)
        {
            var path = Abs(GroupsDir + "/" + group + ".asset");
            Assert.IsTrue(File.Exists(path), "missing group " + group);
            var list = new List<(string, string)>();
            var text = File.ReadAllText(path);
            var rx = new System.Text.RegularExpressions.Regex(
                @"m_GUID: ([0-9a-f]{32})\s*\n\s*m_Address: (\S+)");
            foreach (System.Text.RegularExpressions.Match m
                in rx.Matches(text))
            {
                list.Add((m.Groups[1].Value, m.Groups[2].Value));
            }
            return list;
        }

        private static string FindGroupOfAddress(string address)
        {
            foreach (var f in Directory.GetFiles(Abs(GroupsDir), "*.asset"))
            {
                var name = Path.GetFileNameWithoutExtension(f);
                foreach (var e in GroupEntries(name))
                {
                    if (e.Address == address)
                    {
                        return name;
                    }
                }
            }
            return null!;
        }

        // ---------- roster coverage / alias resolution ----------

        private static string EntityDir(Roster r)
        {
            if (r.Kind == "boss")
            {
                return ActorsRoot + "/Creatures/bosses/" + r.Short;
            }
            if (r.Kind == "beast")
            {
                return ActorsRoot + "/Creatures/spirit_beasts/" + r.Short;
            }
            if (r.Kind == "monster")
            {
                var region = r.Group.Substring("region.".Length);
                return ActorsRoot + "/Creatures/" + region + "/" + r.Short;
            }
            var nreg = r.Group.Substring("region.".Length);
            return ActorsRoot + "/Npcs/" + nreg + "/" + r.Short;
        }

        private static string AliasPath(Roster r, Roster target)
        {
            // alias .asset lives in the alias id's own zone dir
            var parts = r.Id.Split('.');
            var zone = parts[1];
            var parent = (r.Kind == "npc" || r.Kind == "npc_shared")
                ? ActorsRoot + "/Npcs/" + zone
                : ActorsRoot + "/Creatures/" + zone;
            return parent + "/" + r.Short + ".asset";
        }

        [Test]
        public void TestRosterCoverageAndAliasResolution()
        {
            var rows = RosterRows();
            var byId = new Dictionary<string, Roster>(StringComparer.Ordinal);
            foreach (var r in rows)
            {
                Assert.IsFalse(byId.ContainsKey(r.Id),
                    "duplicate roster id " + r.Id);
                byId[r.Id] = r;
            }
            int catalogIds = 0;
            foreach (var r in rows)
            {
                if (r.Kind != "npc_shared")
                {
                    catalogIds++;
                }
            }
            Assert.AreEqual(118, catalogIds,
                "launch roster must be 118 ids (58 monsters + 8 bosses + " +
                "10 beasts + 42 npcs); npc_shared visuals are internal");
            int aliases = 0;
            foreach (var r in rows)
            {
                var key = "asset." + r.Id + ".prefab";
                var group = FindGroupOfAddress(key);
                Assert.AreEqual(r.Group, group,
                    r.Id + " addressed in wrong group: " + (group ?? "none"));
                if (r.AliasTarget.Length > 0)
                {
                    aliases++;
                    Assert.IsTrue(byId.ContainsKey(r.AliasTarget),
                        r.Id + " alias target not in roster: " + r.AliasTarget);
                    var target = byId[r.AliasTarget];
                    Assert.AreEqual(0, target.AliasTarget.Length,
                        r.Id + " alias chain >1 hop via " + r.AliasTarget);
                    Assert.AreEqual(r.Profile, target.Profile,
                        r.Id + " alias profile differs from target");
                    var apath = AliasPath(r, target);
                    Assert.IsTrue(File.Exists(Abs(apath)),
                        "alias asset missing " + apath);
                    var atext = File.ReadAllText(Abs(apath));
                    Assert.IsTrue(atext.Contains("target_key: asset."
                        + r.AliasTarget + ".prefab"),
                        "alias target_key wrong in " + apath);
                    // alias .asset carries the addressable entry itself
                    var entries = GroupEntries(r.Group);
                    bool found = false;
                    foreach (var e in entries)
                    {
                        if (e.Address == key)
                        {
                            Assert.AreEqual(MetaGuid(apath), e.Guid,
                                r.Id + " alias entry guid mismatch");
                            found = true;
                        }
                    }
                    Assert.IsTrue(found, r.Id + " alias addressable missing");
                    continue;
                }
                var prefab = EntityDir(r) + "/" + r.Short + ".prefab";
                Assert.IsTrue(File.Exists(Abs(prefab)),
                    "prefab missing " + prefab);
                var g = MetaGuid(prefab);
                bool hit = false;
                foreach (var e in GroupEntries(r.Group))
                {
                    if (e.Address == key)
                    {
                        Assert.AreEqual(g, e.Guid,
                            r.Id + " addressable guid mismatch");
                        hit = true;
                    }
                }
                Assert.IsTrue(hit, r.Id + " addressable entry missing");
            }
            Assert.Greater(aliases, 0, "no declared shared visuals");
        }

        // ---------- technique + clips per profile ----------

        private static string ExpectedTechnique(string profile)
        {
            switch (profile)
            {
                case "MONSTER_MEDIUM":
                case "MONSTER_ELITE":
                case "BOSS_LARGE":
                case "WORLD_BOSS":
                    return "skeletal";
                case "MONSTER_SMALL":
                case "SPIRIT_BEAST":
                case "NPC_HUMANOID":
                    return "frame_by_frame";
            }
            Assert.Fail("unknown profile " + profile);
            return null!;
        }

        [Test]
        public void TestAnimationTechniqueAndClipsPerSizeProfile()
        {
            foreach (var r in RosterRows())
            {
                if (r.AliasTarget.Length > 0)
                {
                    continue;
                }
                Assert.AreEqual(ExpectedTechnique(r.Profile), r.Technique,
                    r.Id + " technique for " + r.Profile);
                var dir = EntityDir(r);
                var imp = Abs(dir + "/actor_import.json");
                Assert.IsTrue(File.Exists(imp), "no actor_import " + dir);
                var doc = RegisterJson.Parse(File.ReadAllText(imp));
                Assert.AreEqual("ACTOR", doc.Get("asset_class")!.Str,
                    r.Id + " asset_class");
                Assert.AreEqual(r.Profile, doc.Get("profile")!.Str,
                    r.Id + " import profile");
                Assert.AreEqual(r.Technique, doc.Get("technique")!.Str,
                    r.Id + " import technique");
                var declared = doc.Get("clips")!.Arr!;
                Assert.AreEqual(r.Clips.Count, declared.Count,
                    r.Id + " clip count mismatch in actor_import");
                foreach (var c in declared)
                {
                    Assert.IsTrue(r.Clips.Contains(c.Str),
                        r.Id + " import clip not in catalog set: " + c.Str);
                }
                // minimum required set per profile
                var want = new HashSet<string>(StringComparer.Ordinal);
                if (r.Profile == "SPIRIT_BEAST")
                {
                    want.Add("idle");
                    want.Add("move");
                    want.Add("cast");
                }
                else if (r.Profile == "NPC_HUMANOID")
                {
                    want.Add("idle");
                    want.Add("interact");
                }
                else
                {
                    want.Add("idle");
                    want.Add("move");
                    want.Add("hit");
                    want.Add("defeat");
                }
                int attacks = 0;
                foreach (var c in r.Clips)
                {
                    if (c.StartsWith("attack_", StringComparison.Ordinal))
                    {
                        attacks++;
                    }
                    bool ok = want.Contains(c)
                        || c.StartsWith("attack_", StringComparison.Ordinal)
                        || (r.Kind == "boss" && c.StartsWith(
                            "phase_transition_", StringComparison.Ordinal));
                    Assert.IsTrue(ok, r.Id + " unexpected clip " + c);
                }
                foreach (var c in want)
                {
                    Assert.IsTrue(r.Clips.Contains(c),
                        r.Id + " missing required clip " + c);
                }
                if (r.Profile == "NPC_HUMANOID")
                {
                    Assert.IsFalse(r.Clips.Contains("move"),
                        r.Id + " NPC must not ship a move clip (no PATROL)");
                }
                else if (r.Profile != "SPIRIT_BEAST")
                {
                    Assert.GreaterOrEqual(attacks, 1,
                        r.Id + " needs >=1 attack clip");
                }
                foreach (var c in r.Clips)
                {
                    var anim = dir + "/" + c + ".anim";
                    Assert.IsTrue(File.Exists(Abs(anim)),
                        "missing clip " + anim);
                    var text = File.ReadAllText(Abs(anim));
                    var rate = r.Technique == "skeletal" ? 30 : 12;
                    Assert.IsTrue(
                        text.Contains("m_SampleRate: " + rate),
                        anim + " sample rate != " + rate);
                    bool loops = r.Technique == "frame_by_frame"
                        ? (c == "idle" || c == "move")
                        : (c == "idle" || c == "move");
                    Assert.AreEqual(
                        text.Contains("m_LoopTime: 1"), loops,
                        anim + " loop flag wrong for " + c);
                }
                var ctrl = dir + "/" + r.Short + ".controller";
                Assert.IsTrue(File.Exists(Abs(ctrl)),
                    "missing controller " + ctrl);
                Assert.IsTrue(File.Exists(Abs(ctrl + ".meta")),
                    "missing controller meta " + ctrl);
                if (r.Kind == "boss")
                {
                    for (int p = 2; p <= r.Phases; p++)
                    {
                        Assert.IsTrue(
                            r.Clips.Contains("phase_transition_" + p),
                            r.Id + " missing phase_transition_" + p);
                    }
                }
                else
                {
                    foreach (var c in r.Clips)
                    {
                        Assert.IsFalse(
                            c.StartsWith("phase_transition",
                                StringComparison.Ordinal),
                            r.Id + " non-boss must not carry phase clips");
                    }
                }
                // negative checks: technique/roster rule violations must fail
                if (r.Profile == "NPC_HUMANOID")
                {
                    Assert.IsFalse(
                        File.Exists(Abs(dir + "/move.anim")),
                        r.Id + " NPC ships a move clip (no PATROL declared)");
                }
                if (r.Technique == "frame_by_frame")
                {
                    Assert.IsFalse(
                        Directory.GetFiles(Abs(dir), "*_parts.png").Length > 0,
                        r.Id + " frame-by-frame entity ships a parts sheet");
                }
            }
        }

        // ---------- import profile / meta ----------

        private static (int W, int H) PngSize(string path)
        {
            var b = new byte[24];
            using (var fs = File.OpenRead(path))
            {
                Assert.AreEqual(24, fs.Read(b, 0, 24), "short png " + path);
            }
            Assert.AreEqual(0x89, b[0], "not png " + path);
            int w = (b[16] << 24) | (b[17] << 16) | (b[18] << 8) | b[19];
            int h = (b[20] << 24) | (b[21] << 16) | (b[22] << 8) | b[23];
            return (w, h);
        }

        private sealed class SpriteRow
        {
            public string Name = string.Empty;
            public int X;
            public int Y;
            public int W;
            public int H;
            public double Px;
            public double Py;
            public long Iid;
        }

        private static List<SpriteRow> MetaSprites(string pngRel)
        {
            var text = File.ReadAllText(Abs(pngRel + ".meta"));
            var list = new List<SpriteRow>();
            var rx = new System.Text.RegularExpressions.Regex(
                @"name: (\S+)\s*\n\s*rect:\s*\n\s*serializedVersion: 2\s*\n" +
                @"\s*x: (\d+)\s*\n\s*y: (\d+)\s*\n\s*width: (\d+)\s*\n" +
                @"\s*height: (\d+)\s*\n\s*alignment: 0\s*\n" +
                @"\s*pivot: \{x: ([\d.eE+-]+), y: ([\d.eE+-]+)\}.*?" +
                @"internalID: (-?\d+)",
                System.Text.RegularExpressions.RegexOptions.Singleline);
            foreach (System.Text.RegularExpressions.Match m
                in rx.Matches(text))
            {
                var s = new SpriteRow();
                s.Name = m.Groups[1].Value;
                s.X = int.Parse(m.Groups[2].Value, CultureInfo.InvariantCulture);
                s.Y = int.Parse(m.Groups[3].Value, CultureInfo.InvariantCulture);
                s.W = int.Parse(m.Groups[4].Value, CultureInfo.InvariantCulture);
                s.H = int.Parse(m.Groups[5].Value, CultureInfo.InvariantCulture);
                s.Px = double.Parse(m.Groups[6].Value, CultureInfo.InvariantCulture);
                s.Py = double.Parse(m.Groups[7].Value, CultureInfo.InvariantCulture);
                s.Iid = long.Parse(m.Groups[8].Value, CultureInfo.InvariantCulture);
                list.Add(s);
            }
            return list;
        }

        [Test]
        public void TestSpriteImportProfileAndPivots()
        {
            var profiles = Profiles();
            foreach (var r in RosterRows())
            {
                if (r.AliasTarget.Length > 0)
                {
                    continue;
                }
                var prof = profiles[r.Profile];
                var dir = EntityDir(r);
                if (r.Technique == "frame_by_frame")
                {
                    foreach (var c in r.Clips)
                    {
                        var png = dir + "/" + r.Short + "_" + c + ".png";
                        var dim = PngSize(Abs(png));
                        Assert.AreEqual(prof.CellW * 8, dim.W,
                            png + " sheet width != 4 x 2x cell");
                        Assert.AreEqual(prof.CellH * 2, dim.H,
                            png + " sheet height != 2x cell");
                        var sprites = MetaSprites(png);
                        Assert.AreEqual(4, sprites.Count,
                            png + " must slice to 4 frames");
                        foreach (var s in sprites)
                        {
                            Assert.AreEqual(0.5, s.Px, 0.001, s.Name);
                            Assert.AreEqual(0.0, s.Py, 0.001,
                                s.Name + " pivot must be bottom-center");
                            Assert.AreEqual(prof.CellW * 2, s.W, s.Name);
                            Assert.AreEqual(prof.CellH * 2, s.H, s.Name);
                        }
                        var text = File.ReadAllText(Abs(png + ".meta"));
                        Assert.IsTrue(text.Contains("spritePixelsToUnits: 100"),
                            png + " PPU != 100");
                        Assert.IsTrue(text.Contains("spriteMode: 2"),
                            png + " must be spriteMode 2");
                    }
                }
                else
                {
                    var png = dir + "/" + r.Short + "_parts.png";
                    var dim = PngSize(Abs(png));
                    Assert.AreEqual(prof.CellW * 2, dim.W,
                        png + " parts sheet width != 2x cell");
                    Assert.AreEqual(prof.CellH * 2, dim.H,
                        png + " parts sheet height != 2x cell");
                    var sprites = MetaSprites(png);
                    Assert.AreEqual(9, sprites.Count,
                        png + " parts sheet must have 9 sprites");
                    var names = new HashSet<string>(StringComparer.Ordinal);
                    foreach (var s in sprites)
                    {
                        Assert.IsTrue(
                            s.Name.StartsWith(r.Short + "_",
                                StringComparison.Ordinal),
                            s.Name + " sprite name prefix != " + r.Short);
                        names.Add(s.Name.Substring(r.Short.Length + 1));
                    }
                    var parts = new[]
                    {
                        "head", "hair", "torso", "arm_front", "arm_back",
                        "leg_front", "leg_back", "accessory_tail", "weapon",
                    };
                    foreach (var part in parts)
                    {
                        Assert.IsTrue(names.Contains(part),
                            png + " missing PSB part " + part);
                    }
                    var text = File.ReadAllText(Abs(png + ".meta"));
                    Assert.IsTrue(text.Contains("spritePixelsToUnits: 100"),
                        png + " PPU != 100");
                }
                // collider contract (§3.10): monsters carry BoxCollider2D
                var prefab = File.ReadAllText(
                    Abs(dir + "/" + r.Short + ".prefab"));
                int boxIdx = prefab.IndexOf("BoxCollider2D",
                    StringComparison.Ordinal);
                Assert.AreEqual(prof.ColW > 0, boxIdx >= 0,
                    r.Id + " collider presence wrong for " + r.Profile);
                if (boxIdx >= 0)
                {
                    var m = System.Text.RegularExpressions.Regex.Match(
                        prefab.Substring(boxIdx),
                        @"m_Size: \{x: ([\d.eE+-]+), y: ([\d.eE+-]+)\}");
                    Assert.IsTrue(m.Success, r.Id + " collider m_Size");
                    Assert.AreEqual(prof.ColW / 100.0,
                        double.Parse(m.Groups[1].Value,
                            CultureInfo.InvariantCulture), 0.001,
                        r.Id + " collider width");
                    Assert.AreEqual(prof.ColH / 100.0,
                        double.Parse(m.Groups[2].Value,
                            CultureInfo.InvariantCulture), 0.001,
                        r.Id + " collider height");
                }
            }
        }

        // ---------- style packs + palette gate ----------

        private static RegisterJson.Node Fragment()
        {
            var path = Abs(FragmentPath);
            Assert.IsTrue(File.Exists(path), "actors_creatures fragment missing");
            var root = RegisterJson.Parse(File.ReadAllText(path));
            Assert.AreEqual(RegisterJson.Node.Kind.Obj, root.Type);
            Assert.AreEqual(1.0, root.Get("schema_version")!.Num);
            return root;
        }

        private static List<RegisterJson.Node> Rows()
        {
            return Fragment().Get("assets")!.Arr!;
        }

        private static string? Str(RegisterJson.Node n, string f)
        {
            var v = n.Get(f);
            return v != null && v.Type == RegisterJson.Node.Kind.Str
                ? v.Str : null;
        }

        private sealed class PaletteEntry
        {
            public LabPixels.Lab Lab;
            public string Hex = string.Empty;
        }

        private static List<PaletteEntry> LoadPalette(string packId)
        {
            var path = Abs("client/Assets/Art/StyleRef/" + packId
                + "/palette.json");
            Assert.IsTrue(File.Exists(path), "palette missing " + packId);
            var root = RegisterJson.Parse(File.ReadAllText(path));
            var list = new List<PaletteEntry>();
            foreach (var row in root.Arr!)
            {
                var lab = row.Get("lab")!.Arr!;
                var pe = new PaletteEntry();
                pe.Lab = new LabPixels.Lab
                {
                    L = lab[0].Num, A = lab[1].Num, B = lab[2].Num,
                };
                pe.Hex = Str(row, "hex") ?? string.Empty;
                list.Add(pe);
            }
            return list;
        }

        [Test]
        public void TestStylePackAndPaletteGate()
        {
            var packs = new Dictionary<string, List<PaletteEntry>>(
                StringComparer.Ordinal);
            foreach (var row in Rows())
            {
                var pack = Str(row, "style_pack_id");
                Assert.IsNotNull(pack,
                    "row without style_pack_id " + Str(row, "file_path"));
                Assert.IsTrue(pack!.StartsWith("actors_creatures/",
                    StringComparison.Ordinal), pack);
                if (packs.ContainsKey(pack))
                {
                    continue;
                }
                var dir = "client/Assets/Art/StyleRef/" + pack;
                Assert.IsTrue(Directory.Exists(Abs(dir)),
                    "style pack dir missing " + pack);
                Assert.IsTrue(File.Exists(Abs(dir + "/style.md")),
                    "style.md missing " + pack);
                Assert.GreaterOrEqual(
                    Directory.GetFiles(Abs(dir), "*.png").Length, 4,
                    pack + " needs >=4 anchor/reference pngs");
                if (pack.StartsWith("actors_creatures/boss_",
                    StringComparison.Ordinal))
                {
                    int turns = 0;
                    foreach (var f in Directory.GetFiles(Abs(dir), "*.png"))
                    {
                        if (Path.GetFileName(f).StartsWith("turnaround_",
                            StringComparison.Ordinal))
                        {
                            turns++;
                        }
                    }
                    Assert.AreEqual(4, turns,
                        pack + " boss pack needs 4 turnaround views");
                }
                packs[pack] = LoadPalette(pack);
                Assert.GreaterOrEqual(packs[pack].Count, 8,
                    pack + " palette <8 colors");
            }
            // §3.8 gate: >=85% of silhouette px within DeltaE00 <= 8 of the
            // nearest palette color. Deterministic strided sample (~6000 px
            // per image) so the O(px x colors) check stays EditMode-fast.
            foreach (var row in Rows())
            {
                var fp = Str(row, "file_path")!;
                var pack = Str(row, "style_pack_id")!;
                var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(fp))),
                    "decode failed " + fp);
                var px = tex.GetPixels32();
                UnityEngine.Object.DestroyImmediate(tex);
                var pal = packs[pack];
                int stride = px.Length > 6000 ? px.Length / 6000 : 1;
                int sil = 0, ok = 0;
                for (int i = 0; i < px.Length; i += stride)
                {
                    var p = px[i];
                    if (p.a < 128)
                    {
                        continue;
                    }
                    sil++;
                    var lab = LabPixels.ToLab(p.r, p.g, p.b);
                    double best = double.MaxValue;
                    foreach (var pe in pal)
                    {
                        var d = LabPixels.DeltaE00(lab, pe.Lab);
                        if (d < best)
                        {
                            best = d;
                        }
                    }
                    if (best <= 8.0)
                    {
                        ok++;
                    }
                }
                Assert.Greater(sil, 0, fp + " empty silhouette");
                double frac = ok / (double)sil;
                Assert.GreaterOrEqual(frac, 0.85,
                    fp + " palette coverage " + frac.ToString("F3",
                    CultureInfo.InvariantCulture) + " < 0.85");
            }
        }

        // ---------- frame consistency ----------

        [Test]
        public void TestFrameConsistency()
        {
            var profiles = Profiles();
            foreach (var r in RosterRows())
            {
                if (r.AliasTarget.Length > 0
                    || r.Technique != "frame_by_frame")
                {
                    continue;
                }
                var prof = profiles[r.Profile];
                var dir = EntityDir(r);
                foreach (var c in r.Clips)
                {
                    var png = dir + "/" + r.Short + "_" + c + ".png";
                    var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                    Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(png))),
                        "decode failed " + png);
                    var px = tex.GetPixels32();
                    int fw = prof.CellW * 2, fh = prof.CellH * 2;
                    var widths = new List<int>();
                    for (int f = 0; f < 4; f++)
                    {
                        int minx = fw, maxx = -1;
                        for (int y = 0; y < fh; y++)
                        {
                            for (int x = 0; x < fw; x++)
                            {
                                var p = px[y * tex.width + f * fw + x];
                                if (p.a > 0)
                                {
                                    if (x < minx)
                                    {
                                        minx = x;
                                    }
                                    if (x > maxx)
                                    {
                                        maxx = x;
                                    }
                                }
                            }
                        }
                        Assert.Greater(maxx, minx, png + " frame " + f + " empty");
                        widths.Add(maxx - minx + 1);
                    }
                    UnityEngine.Object.DestroyImmediate(tex);
                    int wmax = 0, wmin = int.MaxValue;
                    foreach (var wv in widths)
                    {
                        if (wv > wmax)
                        {
                            wmax = wv;
                        }
                        if (wv < wmin)
                        {
                            wmin = wv;
                        }
                    }
                    int fence = (c == "idle" || c == "move") ? 8 : 32;
                    Assert.LessOrEqual(wmax - wmin, fence,
                        png + " frame width spread " + (wmax - wmin)
                        + " > " + fence);
                }
            }
        }

        // ---------- provenance ----------

        private static string Sha256File(string path)
        {
            using (var sha = System.Security.Cryptography.SHA256.Create())
            {
                var bytes = sha.ComputeHash(File.ReadAllBytes(path));
                var sb = new StringBuilder(64);
                for (int i = 0; i < bytes.Length; i++)
                {
                    sb.Append(bytes[i].ToString("x2",
                        CultureInfo.InvariantCulture));
                }
                return sb.ToString();
            }
        }

        [Test]
        public void TestProvenanceFragmentIntegrity()
        {
            var rows = Rows();
            Assert.Greater(rows.Count, 0, "fragment empty");
            string? prev = null;
            var seen = new HashSet<string>(StringComparer.Ordinal);
            var covered = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in rows)
            {
                var path = Str(row, "file_path");
                Assert.IsNotNull(path, "row without file_path");
                Assert.IsTrue(File.Exists(Abs(path!)),
                    "row file missing " + path);
                if (prev != null)
                {
                    Assert.IsTrue(
                        string.CompareOrdinal(prev, path) < 0,
                        "assets not sorted by file_path: " + path);
                }
                prev = path;
                Assert.IsTrue(seen.Add(path!), "dup file_path " + path);
                Assert.AreEqual("AI_CREATED", Str(row, "source_kind"), path);
                Assert.AreEqual("PENDING", Str(row, "review_state"), path);
                var fh = Str(row, "final_sha256");
                Assert.AreEqual(fh, Sha256File(Abs(path!)),
                    "final_sha256 mismatch " + path);
                Assert.AreEqual("sha256:" + fh, Str(row, "content_id"), path);
                var key = Str(row, "asset_key");
                Assert.IsNotNull(key, path + " asset_key");
                covered.Add(path!);
                // no-placeholder: >=4 distinct opaque colors
                var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
                Assert.IsTrue(tex.LoadImage(File.ReadAllBytes(Abs(path!))));
                var px = tex.GetPixels32();
                UnityEngine.Object.DestroyImmediate(tex);
                var colors = new HashSet<int>();
                int opaque = 0;
                foreach (var p in px)
                {
                    if (p.a >= 128)
                    {
                        opaque++;
                        colors.Add((p.r << 16) | (p.g << 8) | p.b);
                    }
                }
                Assert.Greater(opaque, 100, path + " no silhouette");
                Assert.GreaterOrEqual(colors.Count, 4,
                    path + " looks like a placeholder (<4 colors)");
            }
            // every produced png under the owned Actors dirs has a row
            foreach (var ownedDir in new[] { "Creatures", "Npcs" })
            {
                var dir = Path.Combine(Abs(ActorsRoot), ownedDir);
                if (!Directory.Exists(dir))
                {
                    continue;
                }
                foreach (var png in Directory.GetFiles(dir, "*.png",
                    SearchOption.AllDirectories))
                {
                    var rel = png.Substring(RepoRoot().Length + 1)
                        .Replace(Path.DirectorySeparatorChar, '/');
                    Assert.IsTrue(covered.Contains(rel),
                        "unprovenanced png " + rel);
                }
            }
        }

        [Test]
        public void TestFolkloreCardsNoForbiddenMotif()
        {
            foreach (var row in Rows())
            {
                var fp = Str(row, "file_path")!;
                var card = row.Get("folklore_card");
                Assert.IsNotNull(card, fp + " folklore_card missing");
                if (card!.Type == RegisterJson.Node.Kind.Null)
                {
                    // only style-reference anchor rows may omit the card
                    Assert.IsTrue(fp.Contains("/Art/StyleRef/"),
                        fp + " produced asset lacks folklore_card");
                    continue;
                }
                Assert.AreEqual(RegisterJson.Node.Kind.Obj, card.Type, fp);
                var tales = card.Get("source_tales");
                Assert.IsNotNull(tales, fp + " source_tales missing");
                Assert.Greater(tales!.Arr!.Count, 0, fp + " empty tales");
                var variants = card.Get("regional_variants");
                Assert.IsNotNull(variants, fp + " regional_variants missing");
                Assert.AreEqual(RegisterJson.Node.Kind.Arr, variants!.Type,
                    fp + " regional_variants must be an array");
                Assert.Greater(variants.Arr!.Count, 0,
                    fp + " regional_variants empty");
                var motifs = card.Get("motifs_checked");
                Assert.IsNotNull(motifs, fp + " motifs_checked missing");
                Assert.Greater(motifs!.Arr!.Count, 0,
                    fp + " motifs_checked empty");
                var checkedSet = new HashSet<string>(StringComparer.Ordinal);
                foreach (var m in motifs.Arr!)
                {
                    checkedSet.Add(m.Str ?? string.Empty);
                }
                foreach (var m in ForbiddenMotifs)
                {
                    Assert.IsTrue(checkedSet.Contains(m),
                        fp + " motifs_checked does not cover " + m);
                }
                var prompt = Str(row.Get("generation_record")!, "prompt")
                    ?? string.Empty;
                var talesText = string.Empty;
                foreach (var t in tales!.Arr!)
                {
                    talesText += " " + (t.Str ?? string.Empty);
                }
                var varText = string.Empty;
                foreach (var v in variants!.Arr!)
                {
                    varText += " " + (v.Str ?? string.Empty);
                }
                var hay = (fp + " " + prompt + " " + talesText + " "
                    + varText).ToLowerInvariant();
                foreach (var m in new[] { "torii", "jiangshi", "kimono",
                    "hanbok" })
                {
                    Assert.IsFalse(
                        System.Text.RegularExpressions.Regex.IsMatch(
                            hay, "\\b" + m + "\\b"),
                        fp + " forbidden motif token " + m);
                }
            }
        }

        [Test]
        public void TestAiCreatedToolMatchesOwnerSetup()
        {
            var techDoc = File.ReadAllText(
                Abs("docs/00_context/technology_versions.md"));
            Assert.IsTrue(techDoc.Contains("Direct AI Generation"),
                "technology_versions.md no longer names the AI tool");
            int ai = 0;
            foreach (var row in Rows())
            {
                if (Str(row, "source_kind") != "AI_CREATED")
                {
                    continue;
                }
                ai++;
                var gen = row.Get("generation_record");
                Assert.IsNotNull(gen,
                    "generation_record missing " + Str(row, "file_path"));
                Assert.AreEqual(
                    "Direct AI Generation (In-Session Multimodal)",
                    Str(gen!, "tool"), Str(row, "file_path"));
                var model = Str(gen!, "model_id");
                Assert.IsNotNull(model, "model_id missing");
                Assert.IsTrue(model!.Length > 0);
                var termsUri = Str(gen!, "terms_uri");
                Assert.IsNotNull(termsUri, "terms_uri missing");
                Assert.IsTrue(termsUri!.StartsWith("https://",
                    StringComparison.Ordinal));
                var snap = Str(gen!, "terms_snapshot_sha256");
                Assert.IsNotNull(snap, "terms_snapshot_sha256 missing");
                var pack = Str(row, "style_pack_id");
                Assert.IsNotNull(pack, "style_pack_id missing");
                var frag = pack!.Split('/')[0];
                var snapPath = "client/Assets/Art/Provenance/terms/"
                    + frag + "/" + snap + ".txt";
                Assert.IsTrue(File.Exists(Abs(snapPath)),
                    "terms snapshot missing " + snapPath);
                Assert.AreEqual(snap, Sha256File(Abs(snapPath)),
                    "terms snapshot hash mismatch");
                var prompt = Str(gen!, "prompt") ?? string.Empty;
                Assert.IsTrue(prompt.Contains("volume")
                    || prompt.Contains("light"),
                    Str(row, "file_path") + " prompt lacks §3.5 directives");
            }
            Assert.Greater(ai, 0, "no AI_CREATED rows");
        }
    }
}
