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
    /// <summary>
    /// Thin view helpers shared by the IMP-099 screens: TMP/UGUI refs are
    /// serialized nullable and every setter null-guards — headless PlayMode
    /// has no TMP Essentials (IMP-066 precedent), so tests exercise the
    /// presenters and never instantiate views with components.
    /// Buttons keep a ≥44 pt touch target per § Accessibility.
    /// </summary>
    public abstract class ScreenView : MonoBehaviour
    {
        /// <summary>Minimum touch-target points (client_experience_contract §6).</summary>
        public const float MinTouchTargetPoints = 44f;

        /// <summary>Loc-aware TMP setter; null-safe.</summary>
        protected static void SetLabel(TMP_Text? label, string key)
        {
            if (label != null)
            {
                label.text = Loc.Get(key);
            }
        }

        /// <summary>Loc-aware formatted TMP setter; null-safe.</summary>
        protected static void SetLabel(
            TMP_Text? label, string key, IReadOnlyDictionary<string, object> args)
        {
            if (label != null)
            {
                label.text = Loc.Get(key, args);
            }
        }

        /// <summary>Plain TMP setter; null-safe.</summary>
        protected static void SetText(TMP_Text? label, string text)
        {
            if (label != null)
            {
                label.text = text;
            }
        }
    }

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

    /// <summary>LOGIN_QUEUED view: position line + cancel button.</summary>
    public sealed class LoginQueueView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _position;
        [SerializeField] private TMP_Text? _retrying;
        [SerializeField] private Button? _cancelButton;
        [SerializeField] private TMP_Text? _cancelLabel;

        private LoginQueuePresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(LoginQueuePresenter presenter)
        {
            _presenter = presenter;
            if (_cancelButton != null)
            {
                _cancelButton.onClick.AddListener(OnCancel);
            }

            SetLabel(_title, ScreensLoc.QueueTitle);
            SetLabel(_retrying, ScreensLoc.QueueRetrying);
            SetLabel(_cancelLabel, ScreensLoc.QueueCancel);
            Refresh();
        }

        /// <summary>UI-phase refresh of the live position.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetLabel(
                _position, _presenter.PositionKey,
                new Dictionary<string, object>
                {
                    { "position", _presenter.Position },
                });
        }

        private void OnCancel()
        {
            if (_presenter != null)
            {
                _presenter.Cancel();
            }
        }
    }

    /// <summary>CHARACTER_SELECT view: slot rows + create + logout.</summary>
    public sealed class CharacterSelectView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _emptyLabel;
        [SerializeField] private RectTransform? _slotRoot;
        [SerializeField] private Button? _createButton;
        [SerializeField] private Button? _logoutButton;
        [SerializeField] private TMP_Text? _message;
        [SerializeField] private TMP_InputField? _nameInput;
        [SerializeField] private TMP_InputField? _classInput;

        private CharacterSelectPresenter? _presenter;
        private CancellationToken _cancel;

        /// <summary>Binds the presenter.</summary>
        public void Bind(CharacterSelectPresenter presenter, CancellationToken cancel)
        {
            _presenter = presenter;
            _cancel = cancel;
            if (_createButton != null)
            {
                _createButton.onClick.AddListener(OnCreate);
            }

            if (_logoutButton != null)
            {
                _logoutButton.onClick.AddListener(OnLogout);
            }

            SetLabel(_title, ScreensLoc.SelectTitle);
            SetLabel(_emptyLabel, ScreensLoc.SelectEmpty);
            presenter.Changed += Refresh;
            Refresh();
        }

        /// <summary>Presenter-driven refresh of slots + busy state.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            if (_emptyLabel != null)
            {
                _emptyLabel.gameObject.SetActive(_presenter.Slots.Count == 0);
            }

            SetLabel(_message, _presenter.MessageKey);
            if (_createButton != null)
            {
                _createButton.interactable =
                    !_presenter.Busy && _presenter.Slots.Count < CharacterSelectPresenter.MaxSlots;
            }
        }

        private void OnCreate()
        {
            if (_presenter == null)
            {
                return;
            }

            _ = _presenter.CreateAsync(
                _nameInput != null ? _nameInput.text : string.Empty,
                _classInput != null ? _classInput.text : string.Empty,
                _cancel);
        }

        private void OnLogout()
        {
            if (_presenter != null)
            {
                _ = _presenter.LogoutAsync(_cancel);
            }
        }
    }

    /// <summary>BOOT view: a single checking label.</summary>
    public sealed class BootView : ScreenView
    {
        [SerializeField] private TMP_Text? _checking;

        private void Awake()
        {
            SetLabel(_checking, ScreensLoc.BootChecking);
        }
    }

    /// <summary>PATCHING_UPDATE view: progress percent + bar.</summary>
    public sealed class PatchingView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _percent;
        [SerializeField] private Image? _bar;

        private PatchingPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(PatchingPresenter presenter)
        {
            _presenter = presenter;
            SetLabel(_title, ScreensLoc.PatchingTitle);
            Refresh();
        }

        /// <summary>Progress refresh driven by the presenter.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetText(_percent, ((int)(_presenter.Percent * 100f)).ToString() + "%");
            if (_bar != null)
            {
                _bar.fillAmount = _presenter.Percent;
            }
        }
    }

    /// <summary>TRANSFERRING_MAP view: progress bar + status + folk art backdrop.</summary>
    public sealed class TransferView : ScreenView
    {
        [SerializeField] private TMP_Text? _status;
        [SerializeField] private TMP_Text? _percent;
        [SerializeField] private Image? _bar;
        [SerializeField] private GameObject? _progressRoot;

        private TransferPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(TransferPresenter presenter)
        {
            _presenter = presenter;
            Refresh();
        }

        /// <summary>UI-phase refresh of progress + status.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetLabel(_status, _presenter.StatusKey);
            if (_progressRoot != null)
            {
                _progressRoot.SetActive(
                    _presenter.Progress.Visible && !_presenter.PlacementPending);
            }

            SetText(
                _percent,
                ((int)(_presenter.Progress.Progress * 100f)).ToString() + "%");
            if (_bar != null)
            {
                _bar.fillAmount = _presenter.Progress.Progress;
            }
        }
    }

    /// <summary>Settings view: preset cycle + battery toggle + locale buttons.</summary>
    public sealed class SettingsView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _qualityLabel;
        [SerializeField] private TMP_Text? _batteryLabel;
        [SerializeField] private TMP_Text? _localeLabel;
        [SerializeField] private TMP_Text? _presetValue;
        [SerializeField] private Button? _presetNext;
        [SerializeField] private Toggle? _batteryToggle;
        [SerializeField] private Button? _viButton;
        [SerializeField] private Button? _enButton;

        private SettingsPresenter? _presenter;
        private int _presetIndex = 1;

        /// <summary>Binds the presenter; hydrates from persisted state.</summary>
        public void Bind(SettingsPresenter presenter)
        {
            _presenter = presenter;
            Core.Rendering.QualityPreset current = presenter.Preset;
            Core.Rendering.QualityPreset[] presets = SettingsPresenter.Presets;
            _presetIndex = 0;
            for (int i = 0; i < presets.Length; i++)
            {
                if (presets[i] == current)
                {
                    _presetIndex = i;
                }
            }

            SetLabel(_title, ScreensLoc.SettingsTitle);
            SetLabel(_qualityLabel, ScreensLoc.SettingsQuality);
            SetLabel(_batteryLabel, ScreensLoc.SettingsBatterySaver);
            SetLabel(_localeLabel, ScreensLoc.SettingsLocale);
            if (_batteryToggle != null)
            {
                _batteryToggle.isOn = presenter.BatterySaverEnabled;
                _batteryToggle.onValueChanged.AddListener(OnBattery);
            }

            if (_presetNext != null)
            {
                _presetNext.onClick.AddListener(OnPresetNext);
            }

            if (_viButton != null)
            {
                _viButton.onClick.AddListener(delegate()
                {
                    presenter.SetLocale("vi-VN");
                });
            }

            if (_enButton != null)
            {
                _enButton.onClick.AddListener(delegate()
                {
                    presenter.SetLocale("en-US");
                });
            }

            Refresh();
        }

        private void OnPresetNext()
        {
            if (_presenter == null)
            {
                return;
            }

            Core.Rendering.QualityPreset[] presets = SettingsPresenter.Presets;
            _presetIndex = (_presetIndex + 1) % presets.Length;
            _presenter.SetPreset(presets[_presetIndex]);
            Refresh();
        }

        private void OnBattery(bool enabled)
        {
            if (_presenter != null)
            {
                _presenter.SetBatterySaver(enabled);
            }
        }

        private void Refresh()
        {
            if (_presetValue != null && _presenter != null)
            {
                _presetValue.text = _presenter.Preset.ToString();
            }
        }
    }

    /// <summary>Credits view: resolves the credits TextAsset body.</summary>
    public sealed class CreditsView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _body;

        private CreditsPresenter? _presenter;

        /// <summary>Binds the presenter; composition then calls LoadAsync.</summary>
        public void Bind(CreditsPresenter presenter)
        {
            _presenter = presenter;
            SetLabel(_title, ScreensLoc.CreditsTitle);
            presenter.Changed += Refresh;
            Refresh();
        }

        private void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            if (_presenter.Unavailable)
            {
                SetLabel(_body, ScreensLoc.CreditsUnavailable);
            }
            else
            {
                SetText(_body, _presenter.Text);
            }
        }
    }

    /// <summary>DISCONNECTED modal view: attempt line + retry/exit buttons.</summary>
    public sealed class DisconnectedModalView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _attempt;
        [SerializeField] private Button? _retryButton;
        [SerializeField] private Button? _exitButton;
        [SerializeField] private TMP_Text? _retryLabel;
        [SerializeField] private TMP_Text? _exitLabel;

        private DisconnectedModalPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(DisconnectedModalPresenter presenter)
        {
            _presenter = presenter;
            if (_retryButton != null)
            {
                _retryButton.onClick.AddListener(delegate()
                {
                    _presenter.RetryNow();
                });
            }

            if (_exitButton != null)
            {
                _exitButton.onClick.AddListener(delegate()
                {
                    _presenter.ExitToTitle();
                });
            }

            SetLabel(_title, ScreensLoc.DisconnectedTitle);
            SetLabel(_retryLabel, ScreensLoc.DisconnectedRetry);
            SetLabel(_exitLabel, ScreensLoc.DisconnectedExit);
            Refresh();
        }

        /// <summary>UI-phase refresh of the attempt counter.</summary>
        public void Refresh()
        {
            if (_presenter != null)
            {
                SetText(_attempt, _presenter.AttemptLine);
            }
        }
    }

    /// <summary>Non-dismissible SESSION_REPLACED modal (§5): message + Đồng ý.</summary>
    public sealed class SessionReplacedModalView : ScreenView
    {
        [SerializeField] private TMP_Text? _message;
        [SerializeField] private Button? _okButton;
        [SerializeField] private TMP_Text? _okLabel;

        private Action? _acknowledge;

        /// <summary>Binds the acknowledge intent (UiFsmDriver.AcknowledgeSessionReplaced).</summary>
        public void Bind(Action acknowledge)
        {
            _acknowledge = acknowledge;
            if (_okButton != null)
            {
                _okButton.onClick.AddListener(delegate()
                {
                    if (_acknowledge != null)
                    {
                        _acknowledge();
                    }
                });
            }

            SetLabel(_message, ScreensLoc.SessionReplaced);
            SetLabel(_okLabel, ScreensLoc.ModalOk);
        }
    }
}
