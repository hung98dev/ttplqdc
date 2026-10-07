using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Net
{
    /// <summary>
    /// Session driver (IMP-065): runs the HTTPS-ticket / resume handshake,
    /// drains the bounded receive queue on the NetReceive frame phase,
    /// handles every session-lifecycle frame (2..16, 105), forwards
    /// replication frames (107, 206..207, 300..308) to
    /// <see cref="Replication"/>, and drives reconnect/resume per
    /// reconnect.md. A session is replaced by a newer login exactly once;
    /// the SESSION_REPLACED modal is non-dismissible until acknowledged.
    /// </summary>
    public sealed class SessionOrchestrator : IFrameSystem, IDisposable
    {
        private readonly SessionStateMachine _fsm;
        private readonly SessionCredentials _credentials;
        private readonly SessionStore _store;
        private readonly AuthClient _auth;
        private readonly Func<long> _nowMs;
        private readonly Func<long> _wallMs;
        private readonly Func<int, CancellationToken, Task> _delay;
        private readonly ReconnectPolicy _reconnect;
        private readonly List<Waiter> _waiters = new List<Waiter>();
        private readonly object _waitersGate = new object();
        private readonly Queue<Action> _posted = new Queue<Action>();
        private readonly object _postedGate = new object();

        private NetSession? _session;
        private HeartbeatLoop? _heartbeat;
        private Uri? _lastWssUri;
        private bool _helloOkReceived;
        private bool _attachedCharacterLive;
        private bool _shuttingDown;
        private bool _reconnectInFlight;
        private bool _resumeFallbackPending;

        public SessionOrchestrator(
            SessionStateMachine fsm,
            SessionCredentials credentials,
            SessionStore store,
            AuthClient auth,
            Func<long>? nowMs = null,
            Func<int, CancellationToken, Task>? delay = null,
            ReconnectPolicy? reconnect = null,
            Func<long>? wallMs = null)
        {
            _fsm = fsm ?? throw new ArgumentNullException(nameof(fsm));
            _credentials =
                credentials ?? throw new ArgumentNullException(nameof(credentials));
            _store = store ?? throw new ArgumentNullException(nameof(store));
            _auth = auth ?? throw new ArgumentNullException(nameof(auth));
            _nowMs = nowMs ??
                (() => Stopwatch.GetTimestamp() /
                    (Stopwatch.Frequency / 1000L));
            _wallMs = wallMs ??
                (() => DateTimeOffset.UtcNow.ToUnixTimeMilliseconds());
            _delay = delay ??
                ((ms, cancel) => Task.Delay(ms, cancel));
            _reconnect = reconnect ?? new ReconnectPolicy();
            _fsm.Changed += snapshot =>
            {
                if (StateChanged != null)
                {
                    StateChanged(snapshot);
                }
            };
        }

        /// <summary>Replication frames land here (300..308, 107, 206..207).</summary>
        public IReplicationSink? Replication
        {
            get;
            set;
        }

        /// <summary>Progression frames land here (514, 515) —
        /// Systems/Progression (IMP-011).</summary>
        public IProgressionSink? Progression
        {
            get;
            set;
        }

        /// <summary>Latest CHARACTER_LIST snapshot (REPLACEABLE_STATE).</summary>
        public S2CCharacterList? CharacterList
        {
            get;
            private set;
        }

        /// <summary>Latest CREATE_RESULT by operation correlation.</summary>
        public S2CCharacterCreateResult? LastCreateResult
        {
            get;
            private set;
        }

        /// <summary>Last S2C_ERROR observed (for tests/UI surface).</summary>
        public S2CError? LastError
        {
            get;
            private set;
        }

        /// <summary>Set from HELLO_OK.pending_deletion — the UI may offer only
        /// cancel-deletion/logout while true (data_protection.md).</summary>
        public bool PendingDeletion
        {
            get;
            private set;
        }

        /// <summary>True after SESSION_REPLACED arrived and was honored.</summary>
        public bool ReplacedByNewerSession
        {
            get;
            private set;
        }

        /// <summary>True once HELLO_OK landed on the current connection.</summary>
        public bool HelloOkReceived
        {
            get
            {
                return _helloOkReceived;
            }
        }

        /// <summary>True while a live character is attached to the session.</summary>
        public bool AttachedCharacterLive
        {
            get
            {
                return _attachedCharacterLive;
            }
        }

        /// <summary>Destination of the in-flight TRANSFERRING_MAP entry
        /// (attach_ok / transfer_prepare); null outside a transfer.</summary>
        public TransferDestination? PendingTransfer
        {
            get;
            private set;
        }

        /// <summary>Current session epoch accepted by HELLO_OK.</summary>
        public ulong SessionEpoch
        {
            get
            {
                return _credentials.SessionEpoch;
            }
        }

        /// <summary>Raises after each FSM transition/flag change.</summary>
        public event Action<SessionStateSnapshot>? StateChanged;

        /// <summary>Raises on every CHARACTER_LIST push (14).</summary>
        public event Action<S2CCharacterList>? CharacterListReceived;

        /// <summary>Raises on every S2C_ERROR (3).</summary>
        public event Action<S2CError>? WireError;

        /// <summary>
        /// Full bootstrap (client_experience_contract.md): ticket -> connect
        /// -> HELLO(ticket) -> HELLO_OK -> CHARACTER_SELECT or IN_WORLD when
        /// the server resumed a live character.
        /// </summary>
        public async Task<Result<bool>> ConnectWithTicketAsync(
            CancellationToken cancel)
        {
            _fsm.TransitionPhase(SessionPhase.Connecting);
            Result<TicketResponseDto> ticket = await _auth
                .RequestTicketAsync(_credentials, cancel).ConfigureAwait(false);
            while (!ticket.Ok &&
                ticket.ErrorCode == "SERVER_OVERLOADED")
            {
                EnsureUiAuthTitle();
                _fsm.TransitionUi(ClientUiState.LoginQueued);
                await _delay(
                    (int)Math.Max(ticket.RetryAfterMs, 1), cancel)
                    .ConfigureAwait(false);
                ticket = await _auth
                    .RequestTicketAsync(_credentials, cancel)
                    .ConfigureAwait(false);
            }

            if (!ticket.Ok)
            {
                return FailAuth(ticket.ErrorCode);
            }

            var wss = new Uri(ticket.Value.wss_url);
            _lastWssUri = wss;
            return await OpenAndHelloAsync(wss, ticket.Value.ticket, cancel)
                .ConfigureAwait(false);
        }

        /// <summary>Resume path: HELLO carries the resume credential.</summary>
        public async Task<Result<bool>> ResumeAsync(
            Uri wssUri, CancellationToken cancel)
        {
            _fsm.TransitionPhase(SessionPhase.Connecting);
            _lastWssUri = wssUri;
            return await OpenAndHelloAsync(wssUri, null, cancel)
                .ConfigureAwait(false);
        }

        /// <summary>C2S_CHARACTER_ATTACH (6) -> ATTACH_OK (7).</summary>
        public async Task<Result<bool>> AttachAsync(
            byte[] characterId, CancellationToken cancel)
        {
            if (_session == null)
            {
                return Result<bool>.Failure("INVALID_STATE");
            }

            var request = new C2SCharacterAttach();
            request.CharacterId = ByteString.CopyFrom(characterId);
            await _session.SendAsync(
                WireIds.C2SCharacterAttach, request, cancel)
                .ConfigureAwait(false);
            return Result<bool>.Success(true);
        }

        /// <summary>C2S_CHARACTER_DETACH (10) -> DETACH_OK (11).</summary>
        public async Task<Result<bool>> DetachAsync(CancellationToken cancel)
        {
            if (_session == null)
            {
                return Result<bool>.Failure("INVALID_STATE");
            }

            await _session.SendAsync(
                WireIds.C2SCharacterDetach, new C2SCharacterDetach(), cancel)
                .ConfigureAwait(false);
            return Result<bool>.Success(true);
        }

        /// <summary>C2S_CHARACTER_CREATE (12) -> CREATE_RESULT (13).</summary>
        public async Task<Result<bool>> CreateCharacterAsync(
            byte[] operationId, string characterName, string classId,
            CancellationToken cancel)
        {
            if (_session == null)
            {
                return Result<bool>.Failure("INVALID_STATE");
            }

            var request = new C2SCharacterCreate
            {
                OperationId = ByteString.CopyFrom(operationId),
                CharacterName = characterName,
                ClassId = classId,
            };
            await _session.SendAsync(
                WireIds.C2SCharacterCreate, request, cancel)
                .ConfigureAwait(false);
            return Result<bool>.Success(true);
        }

        /// <summary>C2S_BASELINE_ACK (306) after a baseline applies.</summary>
        public Task SendBaselineAckAsync(
            ulong baselineId, CancellationToken cancel)
        {
            if (_session == null)
            {
                return Task.CompletedTask;
            }

            var ack = new C2SBaselineAck { BaselineId = baselineId };
            return _session.SendAsync(WireIds.C2SBaselineAck, ack, cancel);
        }

        /// <summary>C2S_BASELINE_RESYNC_REQUEST (307): one outstanding at a
        /// time, rate 1 / 5 s (synchronization.md § Resync).</summary>
        public Task SendResyncRequestAsync(
            ulong requestId, ulong knownBaselineId, CancellationToken cancel)
        {
            if (_session == null)
            {
                return Task.CompletedTask;
            }

            var request = new C2SBaselineResyncRequest
            {
                RequestId = requestId,
                KnownBaselineId = knownBaselineId,
                Reason = ResyncReason.UnknownBaseline,
            };
            return _session.SendAsync(
                WireIds.C2SBaselineResyncRequest, request, cancel);
        }

        /// <summary>C2S_PRESENTATION_READY (106) answering TRANSFER_PREPARE.</summary>
        public Task SendPresentationReadyAsync(
            byte[] transferId, CancellationToken cancel)
        {
            if (_session == null)
            {
                return Task.CompletedTask;
            }

            var ready = new C2SPresentationReady
            {
                TransferId = ByteString.CopyFrom(transferId),
            };
            return _session.SendAsync(
                WireIds.C2SPresentationReady, ready, cancel);
        }

        /// <summary>
        /// IFrameSystem driver for the NetReceive phase: executes posted
        /// callbacks then drains the bounded queue, applying each frame under
        /// its lease.
        /// </summary>
        public void Tick(in FrameTime time)
        {
            while (true)
            {
                Action? action = null;
                lock (_postedGate)
                {
                    if (_posted.Count > 0)
                    {
                        action = _posted.Dequeue();
                    }
                }

                if (action == null)
                {
                    break;
                }

                action();
            }

            // ApplyFrame may retire the session mid-drain (SESSION_REPLACED
            // -> ShutdownAsync nulls _session) — snapshot once and stop when
            // the live session changes.
            NetSession? session = _session;
            if (session == null)
            {
                return;
            }

            while (_session == session &&
                session.TryDequeue(out ReceiveLease lease))
            {
                using (lease)
                {
                    ApplyFrame(lease.Frame);
                }
            }

            DetectTransportLoss();
        }

        /// <summary>Hard close + disposal; idempotent.</summary>
        public async Task ShutdownAsync()
        {
            _shuttingDown = true;
            PendingTransfer = null;
            HeartbeatLoop? heartbeat = _heartbeat;
            _heartbeat = null;
            if (heartbeat != null)
            {
                heartbeat.Dispose();
            }

            NetSession? session = _session;
            _session = null;
            if (session != null)
            {
                await session.AbortAsync().ConfigureAwait(false);
                session.Dispose();
            }

            lock (_waitersGate)
            {
                foreach (Waiter waiter in _waiters)
                {
                    waiter.Completion.TrySetCanceled();
                }

                _waiters.Clear();
            }

            Replication?.Invalidate();
        }

        /// <summary>
        /// SESSION_REPLACED modal acknowledge: releases to AUTH_TITLE.
        /// </summary>
        public void AcknowledgeSessionReplaced()
        {
            _fsm.AcknowledgeSessionReplaced();
        }

        /// <summary>Auth logout + local credential wipe.</summary>
        public async Task LogoutAsync(CancellationToken cancel)
        {
            await _auth.LogoutAsync(_credentials, cancel).ConfigureAwait(false);
            await ShutdownAsync().ConfigureAwait(false);
            _store.ClearCredentials();
            _fsm.Reset();
            _fsm.TransitionUi(ClientUiState.AuthTitle);
        }

        public void Dispose()
        {
            _heartbeat?.Dispose();
            _session?.Dispose();
        }

        private async Task<Result<bool>> OpenAndHelloAsync(
            Uri wssUri, string? ticket, CancellationToken cancel)
        {
            var session = new NetSession();
            try
            {
                await session.ConnectAsync(wssUri, cancel).ConfigureAwait(false);
            }
            catch (Exception e)
            {
                Log.Error("ws connect failed: " + e);
                session.Dispose();
                _fsm.DropToDisconnected();
                return Result<bool>.Failure("TEMPORARY_DEPENDENCY_FAILURE");
            }

            NetSession? previous = _session;
            _session = session;
            if (previous != null)
            {
                await previous.AbortAsync().ConfigureAwait(false);
                previous.Dispose();
            }

            if (_fsm.Phase == SessionPhase.Disconnected)
            {
                // Retry attempts land here from DISCONNECTED; the contract
                // path runs through CONNECTING before AUTHENTICATING.
                _fsm.TransitionPhase(SessionPhase.Connecting);
            }

            _fsm.TransitionPhase(SessionPhase.Authenticating);
            _helloOkReceived = false;
            var hello = new C2SHello
            {
                ClientBuild = _credentials.ClientBuild,
                Platform = ClientPlatform.Windows,
                ContentRevision = _credentials.ContentRevision,
                DeviceId = ByteString.CopyFrom(_credentials.DeviceId),
                Locale = ClientLocale.ViVn,
            };
            if (ticket != null)
            {
                hello.GameplayTicket = ticket;
            }
            else
            {
                hello.ResumeCredential = _credentials.ResumeCredential;
            }

            try
            {
                await session.SendHelloAsync(hello, cancel).ConfigureAwait(false);
            }
            catch (Exception e)
            {
                Log.Error("hello send failed: " + e);
                _fsm.DropToDisconnected();
                return Result<bool>.Failure("TEMPORARY_DEPENDENCY_FAILURE");
            }

            return Result<bool>.Success(true);
        }

        private Result<bool> FailAuth(string errorCode)
        {
            if (errorCode == "CLIENT_UPDATE_REQUIRED" ||
                errorCode == "CONTENT_INCOMPATIBLE" ||
                errorCode == "PROTOCOL_UNSUPPORTED")
            {
                _fsm.ForceUpdateRequired();
            }
            else
            {
                _fsm.DropToDisconnected();
                _fsm.TransitionUi(ClientUiState.AuthTitle);
            }

            return Result<bool>.Failure(errorCode);
        }

        private void ApplyFrame(DecodedFrame frame)
        {
            _heartbeat?.NoteInbound();
            // Payload! below: EnvelopeCodec fills Payload for every mapped id
            // in this switch, so the cast is the decoder's own guarantee.
            switch (frame.MessageId)
            {
                case WireIds.S2CHelloOk:
                    ApplyHelloOk((S2CHelloOk)frame.Payload!);
                    break;
                case WireIds.S2CError:
                    ApplyError((S2CError)frame.Payload!, frame.CorrelationId);
                    break;
                case WireIds.S2CHeartbeat:
                    ApplyHeartbeat((S2CHeartbeat)frame.Payload!);
                    break;
                case WireIds.S2CCharacterAttachOk:
                    ApplyAttachOk((S2CCharacterAttachOk)frame.Payload!);
                    break;
                case WireIds.S2CSessionReplaced:
                    ApplySessionReplaced();
                    break;
                case WireIds.S2CServerDraining:
                    _fsm.SetServerDraining();
                    break;
                case WireIds.S2CCharacterDetachOk:
                    _attachedCharacterLive = false;
                    _fsm.SetDeadOverlay(false);
                    TransitionWorldTo(SessionPhase.CharacterSelect, ClientUiState.CharacterSelect);
                    break;
                case WireIds.S2CCharacterCreateResult:
                    LastCreateResult = (S2CCharacterCreateResult)frame.Payload!;
                    break;
                case WireIds.S2CCharacterList:
                    CharacterList = (S2CCharacterList)frame.Payload!;
                    if (CharacterListReceived != null)
                    {
                        CharacterListReceived(CharacterList);
                    }
                    break;
                case WireIds.S2CPlacementPending:
                    _fsm.SetPlacementPending(true);
                    break;
                case WireIds.S2CResumeCredential:
                    ApplyResumeCredential(
                        (S2CResumeCredential)frame.Payload!);
                    break;
                case WireIds.S2CTransferPrepare:
                    ApplyTransferPrepare((S2CTransferPrepare)frame.Payload!);
                    break;
                case WireIds.S2CWorldBaseline:
                    Replication?.Apply(frame);
                    EnterWorldOnBaseline();
                    break;
                case WireIds.S2CDeath:
                case WireIds.S2CRespawn:
                case WireIds.S2CEntitySpawn:
                case WireIds.S2CEntityDespawn:
                case WireIds.S2CStateDelta:
                case WireIds.S2CBaselineResyncResult:
                case WireIds.S2CMovementCorrection:
                    Replication?.Apply(frame);
                    break;
                case WireIds.S2CProgressionMutateResult:
                case WireIds.S2CProgressionState:
                    Progression?.Apply(frame);
                    break;
                default:
                    break;
            }

            ReleaseWaiters(frame);
        }

        private void ApplyHelloOk(S2CHelloOk ok)
        {
            _helloOkReceived = true;
            _credentials.SessionEpoch = ok.SessionEpoch;
            _credentials.SessionId = ok.SessionId.ToByteArray();
            _credentials.AccountId =
                ok.AccountId.Length == 16
                    ? new Guid(ok.AccountId.ToByteArray()).ToString()
                    : _credentials.AccountId;
            _credentials.ResumedCharacterId =
                ok.ResumedCharacterId.ToByteArray();
            _credentials.AcceptResumeRotation(
                ok.ResumeCredential, ok.ResumeExpiresAtMs);
            PendingDeletion = ok.PendingDeletion;
            _session?.AcceptEpoch(ok.SessionEpoch);
            _store.Save(_credentials);
            _fsm.AcceptSessionEpoch(ok.SessionEpoch);
            if (ok.ResumedCharacterId.Length == 16)
            {
                _attachedCharacterLive = true;
                // Resume enters TRANSFERRING_MAP (client_experience_contract
                // .md §1: DISCONNECTED -> TRANSFERRING_MAP), never through
                // IN_WORLD; the following attach_ok carries the destination
                // and the entry baseline lands IN_WORLD.
                TransitionWorldTo(
                    SessionPhase.TransferringMap, ClientUiState.TransferringMap);
            }
            else
            {
                _fsm.TransitionPhase(SessionPhase.CharacterSelect);
                StepUiTo(ClientUiState.CharacterSelect);
            }

            HeartbeatLoop? heartbeat = _heartbeat;
            heartbeat?.Dispose();
            _heartbeat = new HeartbeatLoop(
                (int)ok.HeartbeatIntervalMs,
                (int)ok.ConnectionTimeoutMs,
                _nowMs,
                SendHeartbeatAsync,
                OnHeartbeatLossAsync);
            _heartbeat.Start();
        }

        private void ApplyHeartbeat(S2CHeartbeat pong)
        {
            _heartbeat?.NotePong((long)pong.ServerMs);
        }

        private void ApplyAttachOk(S2CCharacterAttachOk ok)
        {
            _attachedCharacterLive = true;
            _credentials.ResumedCharacterId = ok.CharacterId.ToByteArray();
            // CHARACTER_SELECT -> TRANSFERRING_MAP -> IN_WORLD
            // (client_experience_contract.md §1): the attach destination
            // authorizes the map entry; IN_WORLD lands on the new baseline.
            PendingTransfer = new TransferDestination(
                ok.MapId, ok.ChannelIndex, ok.InstanceId.ToByteArray(),
                ok.ContentRevision);
            TransitionWorldTo(
                SessionPhase.TransferringMap, ClientUiState.TransferringMap);
        }

        private void ApplySessionReplaced()
        {
            ReplacedByNewerSession = true;
            _ = ShutdownAsync();
            _fsm.SetSessionReplaced();
        }

        private void ApplyResumeCredential(S2CResumeCredential rotation)
        {
            _credentials.AcceptResumeRotation(
                rotation.ResumeCredential, rotation.ResumeExpiresAtMs);
            _store.Save(_credentials);
        }

        private void ApplyTransferPrepare(S2CTransferPrepare prepare)
        {
            _fsm.SetPlacementPending(false);
            _fsm.SetDeadOverlay(false);
            PendingTransfer = new TransferDestination(
                prepare.MapId, prepare.ChannelIndex,
                prepare.InstanceId.ToByteArray(), prepare.ContentRevision);
            TransitionWorldTo(SessionPhase.TransferringMap, ClientUiState.TransferringMap);
            byte[] transferId = prepare.TransferId.ToByteArray();
            _ = SendPresentationReadyAsync(transferId, CancellationToken.None);
        }

        private void ApplyError(S2CError error, ulong correlationId)
        {
            LastError = error;
            if (WireError != null)
            {
                WireError(error);
            }
            if (error.QueuePosition > 0)
            {
                _fsm.SetQueuePosition((int)error.QueuePosition);
            }

            switch (error.ErrorCode)
            {
                case ErrorCode.SessionReplaced:
                    ApplySessionReplaced();
                    return;
                case ErrorCode.ClientUpdateRequired:
                case ErrorCode.ProtocolUnsupported:
                case ErrorCode.ContentIncompatible:
                    _fsm.ForceUpdateRequired();
                    return;
                case ErrorCode.ServerOverloaded:
                    if (_fsm.UiState == ClientUiState.AuthTitle ||
                        _fsm.UiState == ClientUiState.LoginQueued)
                    {
                        _fsm.TransitionUi(ClientUiState.LoginQueued);
                    }

                    return;
                default:
                    break;
            }

            if (error.CloseAfter)
            {
                _ = BeginReconnectAsync();
            }
        }

        /// <summary>IN_WORLD is entered on the post-transfer baseline
        /// (protocol.md § Phase Legality: "7 + baseline (300) received").
        /// </summary>
        private void EnterWorldOnBaseline()
        {
            if (_fsm.Phase != SessionPhase.TransferringMap)
            {
                return;
            }

            PendingTransfer = null;
            _fsm.SetPlacementPending(false);
            TransitionWorldTo(SessionPhase.InWorld, ClientUiState.InWorld);
        }

        private void TransitionWorldTo(SessionPhase phase, ClientUiState ui)
        {
            _fsm.TransitionPhase(phase);
            StepUiTo(ui);
        }

        /// <summary>
        /// UI edges per client_experience_contract.md: BOOT/PATCHING_UPDATE
        /// enter the flow through AUTH_TITLE; AUTH_TITLE and LOGIN_QUEUED
        /// reach IN_WORLD only through CHARACTER_SELECT.
        /// </summary>
        private void StepUiTo(ClientUiState ui)
        {
            if (_fsm.UiState == ClientUiState.Boot ||
                _fsm.UiState == ClientUiState.PatchingUpdate)
            {
                _fsm.TransitionUi(ClientUiState.AuthTitle);
            }

            if ((ui == ClientUiState.InWorld ||
                    ui == ClientUiState.TransferringMap) &&
                (_fsm.UiState == ClientUiState.AuthTitle ||
                    _fsm.UiState == ClientUiState.LoginQueued))
            {
                _fsm.TransitionUi(ClientUiState.CharacterSelect);
            }

            _fsm.TransitionUi(ui);
        }

        private Task SendHeartbeatAsync(CancellationToken cancel)
        {
            if (_session == null)
            {
                return Task.CompletedTask;
            }

            var heartbeat = new C2SHeartbeat
            {
                ClientMonoMs = (ulong)Math.Max(_nowMs(), 0L),
                EchoServerMs = (ulong)Math.Max(
                    _heartbeat != null ? _heartbeat.LastServerMs : 0L, 0L),
            };
            return _session.SendAsync(WireIds.C2SHeartbeat, heartbeat, cancel);
        }

        private Task OnHeartbeatLossAsync(CancellationToken cancel)
        {
            lock (_postedGate)
            {
                _posted.Enqueue(() => _ = BeginReconnectAsync());
            }

            return Task.CompletedTask;
        }

        /// <summary>Transport lost: bounded reconnect loop with resume.</summary>
        private async Task BeginReconnectAsync()
        {
            if (_shuttingDown || ReplacedByNewerSession || _reconnectInFlight)
            {
                return;
            }

            _reconnectInFlight = true;
            PendingTransfer = null;
            _fsm.DropToDisconnected();
            _fsm.TransitionPhase(SessionPhase.Reconnecting);
            while (_reconnect.CanRetry && !_shuttingDown)
            {
                int delayMs = _reconnect.NextDelayMs();
                if (delayMs < 0)
                {
                    break;
                }

                await _delay(delayMs, CancellationToken.None).ConfigureAwait(false);
                if (_shuttingDown)
                {
                    return;
                }

                try
                {
                    if (await TryResumeOnceAsync().ConfigureAwait(false))
                    {
                        _reconnect.Reset();
                        _reconnectInFlight = false;
                        return;
                    }
                }
                catch (Exception)
                {
                }

                if (_resumeFallbackPending)
                {
                    _resumeFallbackPending = false;
                    if (await TryTicketFallbackAsync().ConfigureAwait(false))
                    {
                        _reconnect.Reset();
                        _reconnectInFlight = false;
                        return;
                    }
                }
            }

            if (!_shuttingDown)
            {
                _fsm.TransitionUi(ClientUiState.AuthTitle);
            }

            _reconnectInFlight = false;
        }

        /// <summary>
        /// One resume attempt: fresh WSS to the last ticket URL, HELLO with
        /// the newest resume credential. HELLO_OK applies on the drain; an
        /// S2C_ERROR RESUME_EXPIRED / AUTH_* marks the ticket fallback.
        /// </summary>
        private async Task<bool> TryResumeOnceAsync()
        {
            if (_lastWssUri == null ||
                !_credentials.HasUsableResume(_wallMs()))
            {
                _resumeFallbackPending = true;
                return false;
            }

            Result<bool> opened = await OpenAndHelloAsync(
                _lastWssUri, null, CancellationToken.None).ConfigureAwait(false);
            if (!opened.Ok)
            {
                return false;
            }

            DecodedFrame? helloOk = await WaitForAsync(
                WireIds.S2CHelloOk, null, 10000, CancellationToken.None)
                .ConfigureAwait(false);
            if (helloOk != null)
            {
                return true;
            }

            if (LastError != null)
            {
                ErrorCode code = LastError.ErrorCode;
                if (code == ErrorCode.ResumeExpired ||
                    code == ErrorCode.AuthInvalid ||
                    code == ErrorCode.AuthExpired ||
                    code == ErrorCode.SessionEpochStale)
                {
                    _resumeFallbackPending = true;
                }
            }

            return false;
        }

        /// <summary>RESUME_EXPIRED path: refresh the credential family, then
        /// a fresh ticket (bypasses the login queue while the character is
        /// live or inside grace — reconnect.md).</summary>
        private async Task<bool> TryTicketFallbackAsync()
        {
            Result<TokenResponseDto> refreshed = await _auth.RefreshAsync(
                _credentials, CancellationToken.None).ConfigureAwait(false);
            if (!refreshed.Ok)
            {
                return false;
            }

            _credentials.AccessToken = refreshed.Value.access_token;
            _credentials.RefreshToken = refreshed.Value.refresh_token;
            _credentials.AccessExpiresAtMs =
                ParseExpiresMs(refreshed.Value.access_expires_at);
            _credentials.RefreshExpiresAtMs =
                ParseExpiresMs(refreshed.Value.refresh_expires_at);
            _store.Save(_credentials);
            Result<bool> joined = await ConnectWithTicketAsync(
                CancellationToken.None).ConfigureAwait(false);
            return joined.Ok;
        }

        /// <summary>RFC3339 auth timestamps -> unix ms (auth.md contract).</summary>
        internal static long ParseExpiresMs(string value)
        {
            if (DateTimeOffset.TryParse(
                value, System.Globalization.CultureInfo.InvariantCulture,
                System.Globalization.DateTimeStyles.RoundtripKind,
                out DateTimeOffset parsed))
            {
                return parsed.ToUnixTimeMilliseconds();
            }

            return 0L;
        }

        /// <summary>Bridge: LOGIN_QUEUED transitions are legal only from
        /// AUTH_TITLE; entering the flow from BOOT passes through title.</summary>
        private void EnsureUiAuthTitle()
        {
            if (_fsm.UiState == ClientUiState.Boot ||
                _fsm.UiState == ClientUiState.PatchingUpdate)
            {
                _fsm.TransitionUi(ClientUiState.AuthTitle);
            }
        }

        private void ReleaseWaiters(DecodedFrame frame)
        {
            lock (_waitersGate)
            {
                for (int i = _waiters.Count - 1; i >= 0; i--)
                {
                    Waiter waiter = _waiters[i];
                    if (waiter.MessageId == frame.MessageId &&
                        (waiter.Predicate == null || waiter.Predicate(frame)))
                    {
                        _waiters.RemoveAt(i);
                        // The waiter outlives the lease — hand it a detached
                        // copy because the pooled frame is Reset() on
                        // release.
                        waiter.Completion.TrySetResult(Detach(frame));
                    }
                }
            }
        }

        private static DecodedFrame Detach(DecodedFrame frame)
        {
            return new DecodedFrame
            {
                ConnectionGeneration = frame.ConnectionGeneration,
                SessionEpoch = frame.SessionEpoch,
                ServerSeq = frame.ServerSeq,
                CorrelationId = frame.CorrelationId,
                MessageId = frame.MessageId,
                BaselineId = frame.BaselineId,
                Envelope = frame.Envelope,
                Payload = frame.Payload,
                BarrierOrdinal = frame.BarrierOrdinal,
                AccountedBytes = frame.AccountedBytes,
                Sequence = frame.Sequence,
            };
        }

        /// <summary>Awaits a frame of <paramref name="messageId"/> matching
        /// <paramref name="predicate"/>, pumping the queue meanwhile.</summary>
        public async Task<DecodedFrame?> WaitForAsync(
            uint messageId, Predicate<DecodedFrame>? predicate,
            int timeoutMs, CancellationToken cancel)
        {
            var waiter = new Waiter(messageId, predicate);
            lock (_waitersGate)
            {
                _waiters.Add(waiter);
            }

            long deadline = _nowMs() + timeoutMs;
            while (_nowMs() < deadline && !cancel.IsCancellationRequested)
            {
                if (waiter.Completion.Task.IsCompletedSuccessfully)
                {
                    return waiter.Completion.Task.Result;
                }

                if (waiter.Completion.Task.IsCanceled)
                {
                    break;
                }

                DrainPending();
                await Task.Delay(1).ConfigureAwait(false);
            }

            lock (_waitersGate)
            {
                _waiters.Remove(waiter);
            }

            return null;
        }

        private void DrainPending()
        {
            // ApplyFrame may retire the session mid-drain (SESSION_REPLACED
            // -> ShutdownAsync nulls _session) — snapshot once and stop when
            // the live session changes.
            NetSession? session = _session;
            if (session == null)
            {
                return;
            }

            while (_session == session &&
                session.TryDequeue(out ReceiveLease lease))
            {
                using (lease)
                {
                    ApplyFrame(lease.Frame);
                }
            }

            DetectTransportLoss();
        }

        /// <summary>
        /// Transport loss is observed here (reconnect.md § Triggers): a
        /// completed or faulted pump while a live session was connected
        /// starts the bounded reconnect ladder — not on close driven by an
        /// S2C message, and never during shutdown.
        /// </summary>
        private void DetectTransportLoss()
        {
            NetSession? session = _session;
            if (session == null || _shuttingDown || _reconnectInFlight)
            {
                return;
            }

            if (!session.PumpCompleted && session.PumpFault == null)
            {
                return;
            }

            if (_fsm.Phase != SessionPhase.InWorld &&
                _fsm.Phase != SessionPhase.CharacterSelect &&
                _fsm.Phase != SessionPhase.TransferringMap)
            {
                return;
            }

            _ = BeginReconnectAsync();
        }

        private sealed class Waiter
        {
            public Waiter(uint messageId, Predicate<DecodedFrame>? predicate)
            {
                MessageId = messageId;
                Predicate = predicate;
            }

            public uint MessageId
            {
                get;
            }

            public Predicate<DecodedFrame>? Predicate
            {
                get;
            }

            public readonly TaskCompletionSource<DecodedFrame> Completion =
                new TaskCompletionSource<DecodedFrame>(
                    TaskCreationOptions.RunContinuationsAsynchronously);
        }
    }
}
