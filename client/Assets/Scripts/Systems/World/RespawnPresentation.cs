using System;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// Dead-state respawn intent (death_respawn.md, ADR-0082): the dead
    /// overlay is already owned by the landed UI FSM; this type owns the
    /// 208 C2S_RESPAWN_REQUEST intent — a single in-flight send per death,
    /// re-armed when the server answers S2C_ACTION_REJECTED (204) or a new
    /// death begins. Composition binds <paramref name="sendRespawn"/> to
    /// the real 208 send.
    /// </summary>
    public sealed class RespawnPresentation
    {
        private readonly Action _sendRespawn;

        /// <summary>sendRespawn issues one 208 intent.</summary>
        public RespawnPresentation(Action sendRespawn)
        {
            _sendRespawn = sendRespawn ?? throw new ArgumentNullException(nameof(sendRespawn));
        }

        /// <summary>Whether the character is dead (overlay shown).</summary>
        public bool Dead
        {
            get;
            private set;
        }

        /// <summary>Whether a 208 is already in flight for this death.</summary>
        public bool Requested
        {
            get;
            private set;
        }

        /// <summary>Whether <see cref="RequestRespawn"/> will send.</summary>
        public bool CanRequest
        {
            get
            {
                return Dead && !Requested;
            }
        }

        /// <summary>Death observed — dead state (and overlay) is up.</summary>
        public void OnDead()
        {
            Dead = true;
            Requested = false;
        }

        /// <summary>207 respawned / left dead state — clears everything.</summary>
        public void OnAlive()
        {
            Dead = false;
            Requested = false;
        }

        /// <summary>
        /// Issues the 208 intent once per death. Returns false when not
        /// dead or a request is already in flight.
        /// </summary>
        public bool RequestRespawn()
        {
            if (!CanRequest)
            {
                return false;
            }
            Requested = true;
            _sendRespawn();
            return true;
        }

        /// <summary>
        /// 204 rejected the in-flight request (not dead / too early on the
        /// server view) — re-arms so the retry cadence may send again.
        /// </summary>
        public void OnRejected()
        {
            Requested = false;
        }
    }
}
