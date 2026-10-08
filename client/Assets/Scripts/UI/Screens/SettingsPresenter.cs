using System;
using ThinhThan.Core.Localization;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Session;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Settings presenter (packet IMP-099 § Acceptance): quality preset,
    /// battery saver and locale persist through IMP-095's surfaces —
    /// <see cref="QualityBenchmark.Persist"/> /
    /// <see cref="QualityGovernor.SetPreset"/> for the preset,
    /// <see cref="BatterySaver.Enabled"/> (self-persisting) for the cap and
    /// <see cref="Loc.SetLocale"/> for the language. Reads come from the same
    /// seams so a persisted choice survives relaunch.
    /// </summary>
    public sealed class SettingsPresenter
    {
        private readonly ISessionStorage _storage;
        private readonly QualityGovernor _governor;
        private readonly BatterySaver _batterySaver;
        private readonly Action<QualityPreset> _applyPreset;
        private readonly Action<string> _setLocale;

        /// <param name="applyPreset">Composition preset applier
        /// (QualityGovernor.SetPreset + PresetApplier sinks); tests fake it.</param>
        /// <param name="setLocale">Bound to <see cref="Loc.SetLocale"/>;
        /// injected so tests stay headless.</param>
        public SettingsPresenter(
            ISessionStorage storage,
            QualityGovernor governor,
            BatterySaver batterySaver,
            Action<QualityPreset> applyPreset,
            Action<string> setLocale)
        {
            _storage = storage ?? throw new ArgumentNullException(nameof(storage));
            _governor = governor ?? throw new ArgumentNullException(nameof(governor));
            _batterySaver = batterySaver ??
                throw new ArgumentNullException(nameof(batterySaver));
            _applyPreset = applyPreset ??
                throw new ArgumentNullException(nameof(applyPreset));
            _setLocale = setLocale ?? throw new ArgumentNullException(nameof(setLocale));
        }

        /// <summary>Persisted quality preset (Medium when never saved).</summary>
        public QualityPreset Preset
        {
            get
            {
                return QualityBenchmark.TryGetPersisted(_storage, out QualityPreset preset)
                    ? preset
                    : QualityPreset.Medium;
            }
        }

        /// <summary>Battery-saver toggle (persisted by BatterySaver).</summary>
        public bool BatterySaverEnabled
        {
            get
            {
                return _batterySaver.Enabled;
            }
        }

        /// <summary>All supported presets in display order.</summary>
        public static QualityPreset[] Presets
        {
            get
            {
                return new[]
                {
                    QualityPreset.Low,
                    QualityPreset.Medium,
                    QualityPreset.High,
                };
            }
        }

        /// <summary>Apply + persist a quality preset.</summary>
        public void SetPreset(QualityPreset preset)
        {
            _applyPreset(preset);
            _governor.SetPreset(preset);
            QualityBenchmark.Persist(_storage, preset);
        }

        /// <summary>Toggle the 30 FPS battery saver (persisted).</summary>
        public void SetBatterySaver(bool enabled)
        {
            _batterySaver.Enabled = enabled;
        }

        /// <summary>Switch locale; invalid codes are ignored by Loc.</summary>
        public void SetLocale(string code)
        {
            _setLocale(code);
        }
    }
}
