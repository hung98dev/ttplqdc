using System;
using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Localization;
using ThinhThan.Protocol.V1;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Screens
{

    /// <summary>AUTH_TITLE view: username/password + register + providers.</summary>
    public sealed class AuthTitleView : ScreenView
    {
        [SerializeField] private TMP_InputField? _username;
        [SerializeField] private TMP_InputField? _password;
        [SerializeField] private TMP_InputField? _email;
        [SerializeField] private Button? _loginButton;
        [SerializeField] private Button? _registerButton;
        [SerializeField] private Button? _appleButton;
        [SerializeField] private Button? _googleButton;
        [SerializeField] private Button? _steamButton;
        [SerializeField] private TMP_Text? _message;
        [SerializeField] private TMP_Text? _loginLabel;
        [SerializeField] private TMP_Text? _registerLabel;
        [SerializeField] private TMP_Text? _providersLabel;

        private AuthTitlePresenter? _presenter;
        private CancellationToken _cancel;

        /// <summary>Binds the presenter; composition injects the token.</summary>
        public void Bind(AuthTitlePresenter presenter, CancellationToken cancel)
        {
            _presenter = presenter;
            _cancel = cancel;
            if (_loginButton != null)
            {
                _loginButton.onClick.AddListener(OnLogin);
            }

            if (_registerButton != null)
            {
                _registerButton.onClick.AddListener(OnRegister);
            }

            if (_appleButton != null)
            {
                _appleButton.onClick.AddListener(delegate()
                {
                    OnProvider(AuthFlow.Provider.Apple);
                });
            }

            if (_googleButton != null)
            {
                _googleButton.onClick.AddListener(delegate()
                {
                    OnProvider(AuthFlow.Provider.Google);
                });
            }

            if (_steamButton != null)
            {
                _steamButton.onClick.AddListener(delegate()
                {
                    OnProvider(AuthFlow.Provider.Steam);
                });
            }

            SetLabel(_loginLabel, ScreensLoc.TitleLogin);
            SetLabel(_registerLabel, ScreensLoc.TitleRegister);
            SetLabel(_providersLabel, ScreensLoc.TitleProviders);
            presenter.Changed += Refresh;
            Refresh();
        }

        private void OnLogin()
        {
            if (_presenter == null)
            {
                return;
            }

            _ = _presenter.LoginAsync(
                _username != null ? _username.text : string.Empty,
                _password != null ? _password.text : string.Empty,
                _cancel);
        }

        private void OnRegister()
        {
            if (_presenter == null)
            {
                return;
            }

            _ = _presenter.RegisterAsync(
                _username != null ? _username.text : string.Empty,
                _password != null ? _password.text : string.Empty,
                _email != null ? _email.text : string.Empty,
                _cancel);
        }

        private void OnProvider(AuthFlow.Provider provider)
        {
            if (_presenter == null)
            {
                return;
            }

            _ = _presenter.LoginProviderAsync(provider, _cancel);
        }

        private void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetLabel(_message, _presenter.MessageKey);
            bool interactable = !_presenter.Busy;
            if (_loginButton != null)
            {
                _loginButton.interactable = interactable;
            }

            if (_registerButton != null)
            {
                _registerButton.interactable = interactable;
            }
        }
    }
}
