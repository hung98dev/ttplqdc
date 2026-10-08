namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Localization keys carried by the IMP-099 screens
    /// (client_localization.md § Keys). Every user-visible string resolves
    /// through <see cref="ThinhThan.Core.Localization.Loc"/>; both vi-VN and
    /// en-US entries ship in the committed <c>Tables/Screens</c> collection
    /// (<c>Screens.asset</c>, shared data + per-locale tables under
    /// <c>Assets/Localization/Tables/Screens/</c>).
    /// </summary>
    public static class ScreensLoc
    {
        public const string BootChecking = "loc.screens.boot.checking";

        public const string PatchingTitle = "loc.screens.patching.title";

        public const string TitleLogin = "loc.screens.title.login";

        public const string TitleRegister = "loc.screens.title.register";

        public const string TitleProviders = "loc.screens.title.providers";

        public const string AuthInvalid = "loc.screens.auth.invalid";

        public const string AuthUpdateRequired = "loc.screens.auth.update_required";

        public const string AuthRateLimited = "loc.screens.auth.rate_limited";

        public const string AuthBanned = "loc.screens.auth.banned";

        public const string RegisterUsernameTaken = "loc.screens.register.username_taken";

        public const string RegisterEmailTaken = "loc.screens.register.email_taken";

        public const string RegisterUsernameInvalid = "loc.screens.register.username_invalid";

        public const string RegisterEmailInvalid = "loc.screens.register.email_invalid";

        public const string RegisterPasswordInvalid = "loc.screens.register.password_invalid";

        public const string QueueTitle = "loc.screens.queue.title";

        public const string QueuePosition = "loc.screens.queue.position";

        public const string QueueRetrying = "loc.screens.queue.retrying";

        public const string QueueCancel = "loc.screens.queue.cancel";

        public const string SelectTitle = "loc.screens.select.title";

        public const string SelectCreate = "loc.screens.select.create";

        public const string SelectLogout = "loc.screens.select.logout";

        public const string SelectEmpty = "loc.screens.select.empty";

        public const string TransferTitle = "loc.screens.transfer.title";

        public const string TransferWaitingPlacement = "loc.screens.transfer.waiting_placement";

        public const string TransferFailed = "loc.screens.transfer.failed";

        public const string SettingsTitle = "loc.screens.settings.title";

        public const string SettingsQuality = "loc.screens.settings.quality";

        public const string SettingsBatterySaver = "loc.screens.settings.battery_saver";

        public const string SettingsLocale = "loc.screens.settings.locale";

        public const string CreditsTitle = "loc.screens.credits.title";

        public const string CreditsUnavailable = "loc.screens.credits.unavailable";

        public const string DisconnectedTitle = "loc.screens.disconnected.title";

        public const string DisconnectedRetry = "loc.screens.disconnected.retry";

        public const string DisconnectedExit = "loc.screens.disconnected.exit";

        public const string SessionReplaced = "loc.screens.session_replaced";

        public const string ModalOk = "loc.screens.modal.ok";
    }
}
