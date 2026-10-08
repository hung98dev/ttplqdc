using System;
using System.Threading;
using ThinhThan.Core.Session;
using UnityEngine;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// AUTH_TITLE presenter (client_experience_contract.md §1): username +
    /// password login, registration (username/password/email), and the
    /// federated Apple/Google/Steam entries. While a call is in flight the
    /// screen is <see cref="Busy"/>; failures surface as the localization
    /// key the view resolves — never the raw credential guess.
    /// </summary>
    public sealed class AuthTitlePresenter
    {
        private readonly AuthFlow _flow;
        private bool _busy;

        public AuthTitlePresenter(AuthFlow flow)
        {
            _flow = flow ?? throw new ArgumentNullException(nameof(flow));
            MessageKey = string.Empty;
        }

        /// <summary>Raised when Busy or the displayed message key changes.</summary>
        public event Action? Changed;

        /// <summary>A login/register request is in flight.</summary>
        public bool Busy
        {
            get
            {
                return _busy;
            }
        }

        /// <summary>Localization key for the current error/notice ("" = none).</summary>
        public string MessageKey
        {
            get;
            private set;
        }

        /// <summary>Đăng nhập: username + password.</summary>
        public Awaitable LoginAsync(
            string username, string password, CancellationToken cancel)
        {
            return RunAsync(
                () => _flow.LoginPasswordAsync(username, password, cancel));
        }

        /// <summary>Đăng ký: username + password + email (ADR-0051).</summary>
        public Awaitable RegisterAsync(
            string username, string password, string email,
            CancellationToken cancel)
        {
            return RunAsync(
                () => _flow.RegisterPasswordAsync(username, password, email, cancel));
        }

        /// <summary>Federated provider button.</summary>
        public Awaitable LoginProviderAsync(
            AuthFlow.Provider provider, CancellationToken cancel)
        {
            return RunAsync(
                () => _flow.LoginFederatedAsync(provider, cancel));
        }

        /// <summary>Manual dismiss of a shown error.</summary>
        public void ClearMessage()
        {
            MessageKey = string.Empty;
            Emit();
        }

        private async Awaitable RunAsync(Func<Awaitable<Result<bool>>> call)
        {
            if (_busy)
            {
                return;
            }

            _busy = true;
            MessageKey = string.Empty;
            Emit();
            try
            {
                Result<bool> result = await call();
                if (!result.Ok)
                {
                    MessageKey = AuthErrorMap.MessageKey(result.ErrorCode);
                }
            }
            finally
            {
                _busy = false;
                Emit();
            }
        }

        private void Emit()
        {
            if (Changed != null)
            {
                Changed();
            }
        }
    }
}
