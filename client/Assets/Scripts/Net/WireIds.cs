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
        public const uint S2CChannelSwitchResult = 110;
        public const uint S2CInteractResult = 116;
        public const uint S2CActionStarted = 203;
        public const uint S2CActionRejected = 204;
        public const uint S2CStatusEvent = 205;
        public const uint S2CDeath = 206;
        public const uint S2CRespawn = 207;
        public const uint S2CWorldBaseline = 300;
        public const uint S2CEntitySpawn = 301;
        public const uint S2CEntityDespawn = 302;
        public const uint S2CStateDelta = 303;
        public const uint S2CCombatEvent = 304;
        public const uint C2SBaselineAck = 306;
        public const uint C2SBaselineResyncRequest = 307;
        public const uint S2CBaselineResyncResult = 308;
        public const uint S2CInventoryResult = 401;
        public const uint S2CLoadoutResult = 403;
        public const uint S2CRewardClaimResult = 409;
        public const uint S2CEntitlementClaimResult = 419;
        public const uint S2CInventoryExpandResult = 429;
        public const uint S2CWalletState = 432;
        public const uint S2CInventoryState = 433;
        public const uint S2CRewardClaimsState = 434;
        public const uint S2CEntitlementPanelState = 435;
        public const uint S2CRewardClaimListResult = 440;
        public const uint S2CRewardClaimDelta = 441;
        public const uint S2CProgressionMutateResult = 514;
        public const uint S2CProgressionState = 515;
        public const uint S2CChatMessage = 601;
        public const uint S2CPartyInvite = 603;
        public const uint S2CPartyState = 607;
        public const uint S2CFriendRequest = 612;
        public const uint S2CFriendState = 616;
        public const uint S2CBlockState = 619;
        public const uint S2CReportPlayerResult = 633;
        public const uint S2CPartyBoardState = 636;
        public const uint S2CPartyResult = 653;
        public const uint S2CSocialResult = 654;
        public const uint S2CChatSendResult = 655;
        public const uint S2CTradeInvite = 701;
        public const uint S2CTradeCancelled = 704;
        public const uint S2CTradeOfferState = 706;
        public const uint S2CTradeResult = 709;
        public const uint S2CTradeRequestResult = 710;
        public const uint S2CAuctionListResult = 731;
        public const uint S2CAuctionBuyResult = 733;
        public const uint S2CAuctionCancelResult = 735;
        public const uint S2CAuctionSold = 736;
        public const uint S2CAuctionSearchResult = 739;
        public const uint S2CAuctionReclaimResult = 741;
        public const uint S2CAuctionProceedsResult = 743;
        public const uint S2CAuctionMyState = 744;
    }
}
