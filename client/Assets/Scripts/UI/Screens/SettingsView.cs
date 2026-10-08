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
}
