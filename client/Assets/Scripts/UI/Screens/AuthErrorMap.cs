namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Auth error-code → localization-key mapping (errors.md, auth.md).
    /// Password-login failure is always AUTH_INVALID and renders as the
    /// generic message — the UI never oracles which credential part failed.
    /// Registration validation and ticket-class errors map onto their own
    /// keys; everything unmapped falls back to the generic auth key.
    /// </summary>
    public static class AuthErrorMap
    {
        /// <summary>Registration-input error codes with dedicated keys.</summary>
        public static string MessageKey(string errorCode)
        {
            switch (errorCode)
            {
                case "AUTH_INVALID":
                    return ScreensLoc.AuthInvalid;
                case "CLIENT_UPDATE_REQUIRED":
                case "CONTENT_INCOMPATIBLE":
                case "PROTOCOL_UNSUPPORTED":
                    return ScreensLoc.AuthUpdateRequired;
                case "RATE_LIMITED":
                case "TEMPORARY_DEPENDENCY_FAILURE":
                    return ScreensLoc.AuthRateLimited;
                case "ACCOUNT_BANNED":
                case "ACCOUNT_SUSPENDED":
                case "CREDENTIAL_CHANGE_LOCKED":
                    return ScreensLoc.AuthBanned;
                case "USERNAME_TAKEN":
                    return ScreensLoc.RegisterUsernameTaken;
                case "EMAIL_TAKEN":
                    return ScreensLoc.RegisterEmailTaken;
                case "USERNAME_INVALID":
                    return ScreensLoc.RegisterUsernameInvalid;
                case "EMAIL_INVALID":
                    return ScreensLoc.RegisterEmailInvalid;
                case "PASSWORD_INVALID":
                    return ScreensLoc.RegisterPasswordInvalid;
                default:
                    return ScreensLoc.AuthInvalid;
            }
        }
    }
}
