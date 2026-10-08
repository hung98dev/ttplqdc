using System.Collections.Generic;
using ThinhThan.Core.Localization;
using UnityEditor;
using UnityEngine;
using UnityEngine.Localization;
using UnityEngine.Localization.Settings;
using UnityEngine.Localization.Tables;

namespace ThinhThan.UI.Screens.EditorTools
{
    /// <summary>
    /// Seeds the IMP-099 screen keys into the Core string-table collection at
    /// editor launch, mirroring LocalizationProvisioner's upsert semantics so
    /// every screen string is a localization key in vi-VN and en-US
    /// (packet § Acceptance). Runs after LocalizationProvisioner via
    /// delayCall ordering — both are idempotent upserts, so ordering does not
    /// matter beyond the tables existing.
    /// </summary>
    public static class ScreensLocalizationSeed
    {
        private const string TablesDir = "Assets/Localization/Tables/Core";

        private const string CollectionName = "Core";

        private static readonly (string Key, string ViVn, string EnUs)[] _entries =
        {
            ("loc.screens.boot.checking", "Đang kiểm tra phiên bản...", "Checking version..."),
            ("loc.screens.patching.title", "Đang cập nhật tài nguyên", "Updating resources"),
            ("loc.screens.title.login", "Đăng nhập", "Sign in"),
            ("loc.screens.title.register", "Đăng ký", "Register"),
            ("loc.screens.title.providers", "Hoặc đăng nhập bằng", "Or sign in with"),
            ("loc.screens.auth.invalid", "Tên đăng nhập hoặc mật khẩu không đúng.", "Invalid username or password."),
            ("loc.screens.auth.update_required", "Cần cập nhật bản mới để tiếp tục.", "Update required to continue."),
            ("loc.screens.auth.rate_limited", "Máy chủ đang bận, vui lòng thử lại sau.", "Server busy, please retry later."),
            ("loc.screens.auth.banned", "Tài khoản đang bị khóa.", "Account is locked."),
            ("loc.screens.register.username_taken", "Tên đăng nhập đã được sử dụng.", "Username is already taken."),
            ("loc.screens.register.email_taken", "Email đã được sử dụng.", "Email is already registered."),
            ("loc.screens.register.username_invalid", "Tên đăng nhập không hợp lệ.", "Invalid username."),
            ("loc.screens.register.email_invalid", "Email không hợp lệ.", "Invalid email."),
            ("loc.screens.register.password_invalid", "Mật khẩu không hợp lệ.", "Invalid password."),
            ("loc.screens.queue.title", "Đang vào hàng chờ", "In the login queue"),
            ("loc.screens.queue.position", "Vị trí của bạn: {position}", "Your position: {position}"),
            ("loc.screens.queue.retrying", "Đang tự động thử lại...", "Retrying automatically..."),
            ("loc.screens.queue.cancel", "Hủy", "Cancel"),
            ("loc.screens.select.title", "Chọn nhân vật", "Select character"),
            ("loc.screens.select.create", "Tạo nhân vật", "Create character"),
            ("loc.screens.select.logout", "Đăng xuất", "Sign out"),
            ("loc.screens.select.empty", "Chưa có nhân vật nào", "No characters yet"),
            ("loc.screens.transfer.title", "Đang chuyển vùng...", "Transferring..."),
            ("loc.screens.transfer.waiting_placement", "Đang chờ chỗ trong khu vực", "Waiting for a spot in the zone"),
            ("loc.screens.transfer.failed", "Chuyển vùng thất bại, đang quay lại điểm an toàn", "Transfer failed, returning to safe point"),
            ("loc.screens.settings.title", "Cài đặt", "Settings"),
            ("loc.screens.settings.quality", "Chất lượng đồ họa", "Graphics quality"),
            ("loc.screens.settings.battery_saver", "Tiết kiệm pin (30 FPS)", "Battery saver (30 FPS)"),
            ("loc.screens.settings.locale", "Ngôn ngữ", "Language"),
            ("loc.screens.credits.title", "Tín dụng", "Credits"),
            ("loc.screens.credits.unavailable", "Không tải được nội dung tín dụng.", "Credits unavailable."),
            ("loc.screens.disconnected.title", "Mất kết nối tới máy chủ.", "Connection lost."),
            ("loc.screens.disconnected.retry", "Đang thử kết nối lại... (Lần {attempt}/{max})", "Retrying... (Attempt {attempt}/{max})"),
            ("loc.screens.disconnected.exit", "Thoát ra màn hình chính", "Exit to title"),
            ("loc.screens.session_replaced", "Tài khoản của bạn đã được đăng nhập từ một thiết bị khác.", "Your account was signed in from another device."),
            ("loc.screens.modal.ok", "Đồng ý", "OK"),
        };

        [InitializeOnLoadMethod]
        private static void Initialize()
        {
            EditorApplication.delayCall += Seed;
        }

        /// <summary>Idempotent upsert of every screen key into both tables.</summary>
        public static void Seed()
        {
            var collection = AssetDatabase.LoadAssetAtPath<StringTableCollection>(
                TablesDir + "/" + CollectionName + ".asset");
            if (collection == null)
            {
                return;
            }

            var viTable = collection.GetTable(
                new LocaleIdentifier(ThinhThanLocale.VietnameseCode)) as StringTable;
            var enTable = collection.GetTable(
                new LocaleIdentifier(ThinhThanLocale.EnglishCode)) as StringTable;
            if (viTable == null || enTable == null)
            {
                return;
            }

            var changed = false;
            foreach (var (key, viVn, enUs) in _entries)
            {
                changed |= Upsert(viTable, key, viVn);
                changed |= Upsert(enTable, key, enUs);
            }

            if (changed)
            {
                EditorUtility.SetDirty(viTable);
                EditorUtility.SetDirty(enTable);
                if (viTable.SharedData != null)
                {
                    EditorUtility.SetDirty(viTable.SharedData);
                }

                AssetDatabase.SaveAssets();
            }
        }

        private static bool Upsert(StringTable table, string key, string value)
        {
            var entry = table.GetEntry(key);
            var changed = false;
            if (entry == null || entry.Value != value)
            {
                entry = table.AddEntry(key, value);
                changed = true;
            }

            var smart = value != null && value.Contains("{") && value.Contains("}");
            if (entry != null && entry.IsSmart != smart)
            {
                entry.IsSmart = smart;
                changed = true;
            }

            return changed;
        }
    }
}
