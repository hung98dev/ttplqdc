using System;
using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// CHARACTER_SELECT presenter (client_experience_contract.md §1): up to
    /// three characters, create or select, no delete. Slots come from the
    /// session's character-list snapshot; select attaches, create calls
    /// C2S_CHARACTER_CREATE via <see cref="CharacterSessionController"/>,
    /// logout returns to title.
    /// </summary>
    public sealed class CharacterSelectPresenter
    {
        /// <summary>Character slots per the §1 contract.</summary>
        public const int MaxSlots = 3;

        private readonly CharacterSessionController _controller;
        private readonly Func<CancellationToken, Awaitable> _logout;
        private bool _busy;

        /// <param name="logout">Bound to SessionOrchestrator.LogoutAsync.</param>
        public CharacterSelectPresenter(
            CharacterSessionController controller,
            Func<CancellationToken, Awaitable> logout)
        {
            _controller = controller ??
                throw new ArgumentNullException(nameof(controller));
            _logout = logout ?? throw new ArgumentNullException(nameof(logout));
            MessageKey = string.Empty;
        }

        /// <summary>Raised when slots or Busy change.</summary>
        public event Action? Changed;

        /// <summary>An attach/create/logout call is in flight.</summary>
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

        /// <summary>Existing characters (at most <see cref="MaxSlots"/>).</summary>
        public IReadOnlyList<CharacterSummary> Slots
        {
            get
            {
                S2CCharacterList? list = _controller.CurrentList;
                if (list == null)
                {
                    return Array.Empty<CharacterSummary>();
                }

                return list.Characters;
            }
        }

        /// <summary>Chọn nhân vật: attach → TRANSFERRING_MAP → IN_WORLD.</summary>
        public async Awaitable SelectAsync(
            CharacterSummary character, CancellationToken cancel)
        {
            if (_busy)
            {
                return;
            }

            await RunAsync(
                async () => await _controller.AttachAsync(
                    character.CharacterId.ToByteArray(), cancel))
                ;
        }

        /// <summary>Tạo nhân vật mới (name + class; selection follows).</summary>
        public async Awaitable CreateAsync(
            string characterName, string classId, CancellationToken cancel)
        {
            if (_busy)
            {
                return;
            }

            await RunAsync(
                async () => await _controller.CreateAsync(
                    characterName, classId, cancel))
                ;
        }

        /// <summary>Đăng xuất → AUTH_TITLE.</summary>
        public async Awaitable LogoutAsync(CancellationToken cancel)
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
                await _logout(cancel);
            }
            finally
            {
                _busy = false;
                Emit();
            }
        }

        private async Awaitable RunAsync<T>(Func<Awaitable<Result<T>>> call)
        {
            _busy = true;
            MessageKey = string.Empty;
            Emit();
            try
            {
                Result<T> result = await call();
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
