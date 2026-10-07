namespace ThinhThan.Net
{
    /// <summary>
    /// Wire constants for the session/transport surface of IMP-065
    /// (messages.md registry, protocol.md § Envelope/Framing).
    /// </summary>
    public static class WireIds
    {
        public const uint ProtocolMajor = 1;

        public const uint ProtocolMinor = 0;

        /// <summary>Inbound hard frame limit (protocol.md § Framing).</summary>
        public const int InboundFrameMaxBytes = 65536;

        /// <summary>Outbound hard frame limit.</summary>
        public const int OutboundFrameMaxBytes = 262144;

        public const uint C2SHello = 1;
        public const uint S2CHelloOk = 2;
        public const uint S2CError = 3;
        public const uint C2SHeartbeat = 4;
        public const uint S2CHeartbeat = 5;
        public const uint C2SCharacterAttach = 6;
        public const uint S2CCharacterAttachOk = 7;
        public const uint S2CSessionReplaced = 8;
        public const uint S2CServerDraining = 9;
        public const uint C2SCharacterDetach = 10;
        public const uint S2CCharacterDetachOk = 11;
        public const uint C2SCharacterCreate = 12;
        public const uint S2CCharacterCreateResult = 13;
        public const uint S2CCharacterList = 14;
        public const uint S2CPlacementPending = 15;
        public const uint S2CResumeCredential = 16;
        public const uint S2CTransferPrepare = 105;
        public const uint C2SPresentationReady = 106;
        public const uint S2CMovementCorrection = 107;
        public const uint S2CDeath = 206;
        public const uint S2CRespawn = 207;
        public const uint S2CWorldBaseline = 300;
        public const uint S2CEntitySpawn = 301;
        public const uint S2CEntityDespawn = 302;
        public const uint S2CStateDelta = 303;
        public const uint C2SBaselineAck = 306;
        public const uint C2SBaselineResyncRequest = 307;
        public const uint S2CBaselineResyncResult = 308;
        public const uint S2CInventoryResult = 401;
        public const uint S2CEntitlementClaimResult = 419;
        public const uint S2CInventoryExpandResult = 429;
        public const uint S2CWalletState = 432;
        public const uint S2CInventoryState = 433;
        public const uint S2CEntitlementPanelState = 435;
    }
}
