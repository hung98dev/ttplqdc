// Code generated test helpers for IMP-061 protocol conformance. NOT generated
// by protoc — authored test support for server/internal/testing/protocol.
package protocoltest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// registryEntry is one row of the messages.md network registry.
type registryEntry struct {
	ID     uint32
	Name   string
	GoType string
	File   string
	C2S    bool
}

// networkRegistry mirrors the messages.md registry tables verbatim:
// 217 allocated IDs (711-729 reserved-unassigned, 737 retired, all absent).
var networkRegistry = []registryEntry{
	{ID: 1, Name: "C2S_HELLO", GoType: "C2SHello", File: "session", C2S: true},
	{ID: 2, Name: "S2C_HELLO_OK", GoType: "S2CHelloOk", File: "session", C2S: false},
	{ID: 3, Name: "S2C_ERROR", GoType: "S2CError", File: "session", C2S: false},
	{ID: 4, Name: "C2S_HEARTBEAT", GoType: "C2SHeartbeat", File: "session", C2S: true},
	{ID: 5, Name: "S2C_HEARTBEAT", GoType: "S2CHeartbeat", File: "session", C2S: false},
	{ID: 6, Name: "C2S_CHARACTER_ATTACH", GoType: "C2SCharacterAttach", File: "session", C2S: true},
	{ID: 7, Name: "S2C_CHARACTER_ATTACH_OK", GoType: "S2CCharacterAttachOk", File: "session", C2S: false},
	{ID: 8, Name: "S2C_SESSION_REPLACED", GoType: "S2CSessionReplaced", File: "session", C2S: false},
	{ID: 9, Name: "S2C_SERVER_DRAINING", GoType: "S2CServerDraining", File: "session", C2S: false},
	{ID: 10, Name: "C2S_CHARACTER_DETACH", GoType: "C2SCharacterDetach", File: "session", C2S: true},
	{ID: 11, Name: "S2C_CHARACTER_DETACH_OK", GoType: "S2CCharacterDetachOk", File: "session", C2S: false},
	{ID: 12, Name: "C2S_CHARACTER_CREATE", GoType: "C2SCharacterCreate", File: "session", C2S: true},
	{ID: 13, Name: "S2C_CHARACTER_CREATE_RESULT", GoType: "S2CCharacterCreateResult", File: "session", C2S: false},
	{ID: 14, Name: "S2C_CHARACTER_LIST", GoType: "S2CCharacterList", File: "session", C2S: false},
	{ID: 15, Name: "S2C_PLACEMENT_PENDING", GoType: "S2CPlacementPending", File: "session", C2S: false},
	{ID: 16, Name: "S2C_RESUME_CREDENTIAL", GoType: "S2CResumeCredential", File: "session", C2S: false},
	{ID: 100, Name: "C2S_INPUT_STATE", GoType: "C2SInputState", File: "movement", C2S: true},
	{ID: 101, Name: "C2S_JUMP", GoType: "C2SJump", File: "movement", C2S: true},
	{ID: 102, Name: "C2S_DROP_THROUGH", GoType: "C2SDropThrough", File: "movement", C2S: true},
	{ID: 103, Name: "C2S_INTERACT", GoType: "C2SInteract", File: "movement", C2S: true},
	{ID: 104, Name: "C2S_PORTAL_USE", GoType: "C2SPortalUse", File: "movement", C2S: true},
	{ID: 105, Name: "S2C_TRANSFER_PREPARE", GoType: "S2CTransferPrepare", File: "movement", C2S: false},
	{ID: 106, Name: "C2S_PRESENTATION_READY", GoType: "C2SPresentationReady", File: "movement", C2S: true},
	{ID: 107, Name: "S2C_MOVEMENT_CORRECTION", GoType: "S2CMovementCorrection", File: "movement", C2S: false},
	{ID: 108, Name: "C2S_MOVEMENT_EDGE", GoType: "C2SMovementEdge", File: "movement", C2S: true},
	{ID: 109, Name: "C2S_CHANNEL_SWITCH", GoType: "C2SChannelSwitch", File: "movement", C2S: true},
	{ID: 110, Name: "S2C_CHANNEL_SWITCH_RESULT", GoType: "S2CChannelSwitchResult", File: "movement", C2S: false},
	{ID: 111, Name: "C2S_DUNGEON_ENTER_REQUEST", GoType: "C2SDungeonEnterRequest", File: "movement", C2S: true},
	{ID: 112, Name: "S2C_DUNGEON_ENTRY_STATE", GoType: "S2CDungeonEntryState", File: "movement", C2S: false},
	{ID: 113, Name: "C2S_DUNGEON_ENTRY_RESPOND", GoType: "C2SDungeonEntryRespond", File: "movement", C2S: true},
	{ID: 114, Name: "C2S_DUNGEON_LEAVE", GoType: "C2SDungeonLeave", File: "movement", C2S: true},
	{ID: 115, Name: "S2C_DUNGEON_LEAVE_RESULT", GoType: "S2CDungeonLeaveResult", File: "movement", C2S: false},
	{ID: 116, Name: "S2C_INTERACT_RESULT", GoType: "S2CInteractResult", File: "movement", C2S: false},
	{ID: 117, Name: "C2S_DUNGEON_ENTRY_CANCEL", GoType: "C2SDungeonEntryCancel", File: "movement", C2S: true},
	{ID: 200, Name: "C2S_SKILL_USE", GoType: "C2SSkillUse", File: "combat", C2S: true},
	{ID: 201, Name: "C2S_BASIC_ATTACK", GoType: "C2SBasicAttack", File: "combat", C2S: true},
	{ID: 202, Name: "C2S_TARGET_INTENT", GoType: "C2STargetIntent", File: "combat", C2S: true},
	{ID: 203, Name: "S2C_ACTION_STARTED", GoType: "S2CActionStarted", File: "combat", C2S: false},
	{ID: 204, Name: "S2C_ACTION_REJECTED", GoType: "S2CActionRejected", File: "combat", C2S: false},
	{ID: 205, Name: "S2C_STATUS_EVENT", GoType: "S2CStatusEvent", File: "combat", C2S: false},
	{ID: 206, Name: "S2C_DEATH", GoType: "S2CDeath", File: "combat", C2S: false},
	{ID: 207, Name: "S2C_RESPAWN", GoType: "S2CRespawn", File: "combat", C2S: false},
	{ID: 208, Name: "C2S_RESPAWN_REQUEST", GoType: "C2SRespawnRequest", File: "combat", C2S: true},
	{ID: 300, Name: "S2C_WORLD_BASELINE", GoType: "S2CWorldBaseline", File: "combat", C2S: false},
	{ID: 301, Name: "S2C_ENTITY_SPAWN", GoType: "S2CEntitySpawn", File: "combat", C2S: false},
	{ID: 302, Name: "S2C_ENTITY_DESPAWN", GoType: "S2CEntityDespawn", File: "combat", C2S: false},
	{ID: 303, Name: "S2C_STATE_DELTA", GoType: "S2CStateDelta", File: "combat", C2S: false},
	{ID: 304, Name: "S2C_COMBAT_EVENT", GoType: "S2CCombatEvent", File: "combat", C2S: false},
	{ID: 305, Name: "S2C_ENCOUNTER_EVENT", GoType: "S2CEncounterEvent", File: "combat", C2S: false},
	{ID: 306, Name: "C2S_BASELINE_ACK", GoType: "C2SBaselineAck", File: "combat", C2S: true},
	{ID: 307, Name: "C2S_BASELINE_RESYNC_REQUEST", GoType: "C2SBaselineResyncRequest", File: "combat", C2S: true},
	{ID: 308, Name: "S2C_BASELINE_RESYNC_RESULT", GoType: "S2CBaselineResyncResult", File: "combat", C2S: false},
	{ID: 400, Name: "C2S_INVENTORY_MUTATE", GoType: "C2SInventoryMutate", File: "durable", C2S: true},
	{ID: 401, Name: "S2C_INVENTORY_RESULT", GoType: "S2CInventoryResult", File: "durable", C2S: false},
	{ID: 402, Name: "C2S_LOADOUT_CHANGE", GoType: "C2SLoadoutChange", File: "durable", C2S: true},
	{ID: 403, Name: "S2C_LOADOUT_RESULT", GoType: "S2CLoadoutResult", File: "durable", C2S: false},
	{ID: 404, Name: "C2S_CRAFT", GoType: "C2SCraft", File: "durable", C2S: true},
	{ID: 405, Name: "S2C_CRAFT_RESULT", GoType: "S2CCraftResult", File: "durable", C2S: false},
	{ID: 406, Name: "C2S_ENHANCE", GoType: "C2SEnhance", File: "durable", C2S: true},
	{ID: 407, Name: "S2C_ENHANCE_RESULT", GoType: "S2CEnhanceResult", File: "durable", C2S: false},
	{ID: 408, Name: "C2S_REWARD_CLAIM", GoType: "C2SRewardClaim", File: "durable", C2S: true},
	{ID: 409, Name: "S2C_REWARD_CLAIM_RESULT", GoType: "S2CRewardClaimResult", File: "durable", C2S: false},
	{ID: 410, Name: "C2S_BEAST_SET_ACTIVE", GoType: "C2SBeastSetActive", File: "durable", C2S: true},
	{ID: 411, Name: "S2C_BEAST_SET_ACTIVE_RESULT", GoType: "S2CBeastSetActiveResult", File: "durable", C2S: false},
	{ID: 412, Name: "C2S_BEAST_EQUIP", GoType: "C2SBeastEquip", File: "durable", C2S: true},
	{ID: 413, Name: "S2C_BEAST_EQUIP_RESULT", GoType: "S2CBeastEquipResult", File: "durable", C2S: false},
	{ID: 414, Name: "C2S_BEAST_UNEQUIP", GoType: "C2SBeastUnequip", File: "durable", C2S: true},
	{ID: 415, Name: "S2C_BEAST_UNEQUIP_RESULT", GoType: "S2CBeastUnequipResult", File: "durable", C2S: false},
	{ID: 416, Name: "C2S_BEAST_FEED", GoType: "C2SBeastFeed", File: "durable", C2S: true},
	{ID: 417, Name: "S2C_BEAST_FEED_RESULT", GoType: "S2CBeastFeedResult", File: "durable", C2S: false},
	{ID: 418, Name: "C2S_ENTITLEMENT_CLAIM", GoType: "C2SEntitlementClaim", File: "durable", C2S: true},
	{ID: 419, Name: "S2C_ENTITLEMENT_CLAIM_RESULT", GoType: "S2CEntitlementClaimResult", File: "durable", C2S: false},
	{ID: 420, Name: "C2S_NPC_SHOP_BUY", GoType: "C2SNpcShopBuy", File: "durable", C2S: true},
	{ID: 421, Name: "S2C_NPC_SHOP_BUY_RESULT", GoType: "S2CNpcShopBuyResult", File: "durable", C2S: false},
	{ID: 422, Name: "C2S_COSMETIC_REDEEM", GoType: "C2SCosmeticRedeem", File: "durable", C2S: true},
	{ID: 423, Name: "S2C_COSMETIC_REDEEM_RESULT", GoType: "S2CCosmeticRedeemResult", File: "durable", C2S: false},
	{ID: 424, Name: "C2S_COSMETIC_EQUIP", GoType: "C2SCosmeticEquip", File: "durable", C2S: true},
	{ID: 425, Name: "S2C_COSMETIC_EQUIP_RESULT", GoType: "S2CCosmeticEquipResult", File: "durable", C2S: false},
	{ID: 426, Name: "C2S_NPC_SHOP_SELL", GoType: "C2SNpcShopSell", File: "durable", C2S: true},
	{ID: 427, Name: "S2C_NPC_SHOP_SELL_RESULT", GoType: "S2CNpcShopSellResult", File: "durable", C2S: false},
	{ID: 428, Name: "C2S_INVENTORY_EXPAND", GoType: "C2SInventoryExpand", File: "durable", C2S: true},
	{ID: 429, Name: "S2C_INVENTORY_EXPAND_RESULT", GoType: "S2CInventoryExpandResult", File: "durable", C2S: false},
	{ID: 430, Name: "C2S_BEAST_LEVEL_UP", GoType: "C2SBeastLevelUp", File: "durable", C2S: true},
	{ID: 431, Name: "S2C_BEAST_LEVEL_UP_RESULT", GoType: "S2CBeastLevelUpResult", File: "durable", C2S: false},
	{ID: 432, Name: "S2C_WALLET_STATE", GoType: "S2CWalletState", File: "durable", C2S: false},
	{ID: 433, Name: "S2C_INVENTORY_STATE", GoType: "S2CInventoryState", File: "durable", C2S: false},
	{ID: 434, Name: "S2C_REWARD_CLAIMS_STATE", GoType: "S2CRewardClaimsState", File: "durable", C2S: false},
	{ID: 435, Name: "S2C_ENTITLEMENT_PANEL_STATE", GoType: "S2CEntitlementPanelState", File: "durable", C2S: false},
	{ID: 436, Name: "S2C_BEAST_STATE", GoType: "S2CBeastState", File: "durable", C2S: false},
	{ID: 437, Name: "S2C_SOUL_STATE", GoType: "S2CSoulState", File: "durable", C2S: false},
	{ID: 438, Name: "S2C_COSMETIC_STATE", GoType: "S2CCosmeticState", File: "durable", C2S: false},
	{ID: 439, Name: "C2S_REWARD_CLAIM_LIST_REQUEST", GoType: "C2SRewardClaimListRequest", File: "durable", C2S: true},
	{ID: 440, Name: "S2C_REWARD_CLAIM_LIST_RESULT", GoType: "S2CRewardClaimListResult", File: "durable", C2S: false},
	{ID: 441, Name: "S2C_REWARD_CLAIM_DELTA", GoType: "S2CRewardClaimDelta", File: "durable", C2S: false},
	{ID: 442, Name: "C2S_SOUL_LIST_REQUEST", GoType: "C2SSoulListRequest", File: "durable", C2S: true},
	{ID: 443, Name: "S2C_SOUL_LIST_RESULT", GoType: "S2CSoulListResult", File: "durable", C2S: false},
	{ID: 500, Name: "C2S_QUEST_ACCEPT", GoType: "C2SQuestAccept", File: "content", C2S: true},
	{ID: 501, Name: "S2C_QUEST_ACCEPT_RESULT", GoType: "S2CQuestAcceptResult", File: "content", C2S: false},
	{ID: 502, Name: "C2S_QUEST_TURN_IN", GoType: "C2SQuestTurnIn", File: "content", C2S: true},
	{ID: 503, Name: "S2C_QUEST_UPDATE", GoType: "S2CQuestUpdate", File: "content", C2S: false},
	{ID: 504, Name: "C2S_ATLAS_CLAIM", GoType: "C2SAtlasClaim", File: "content", C2S: true},
	{ID: 505, Name: "S2C_ATLAS_CLAIM_RESULT", GoType: "S2CAtlasClaimResult", File: "content", C2S: false},
	{ID: 506, Name: "S2C_PROGRESSION_EVENT", GoType: "S2CProgressionEvent", File: "content", C2S: false},
	{ID: 507, Name: "C2S_QUEST_ABANDON", GoType: "C2SQuestAbandon", File: "content", C2S: true},
	{ID: 508, Name: "S2C_QUEST_ABANDON_RESULT", GoType: "S2CQuestAbandonResult", File: "content", C2S: false},
	{ID: 509, Name: "C2S_STORY_BRANCH_CHOOSE", GoType: "C2SStoryBranchChoose", File: "content", C2S: true},
	{ID: 510, Name: "S2C_STORY_BRANCH_RESULT", GoType: "S2CStoryBranchResult", File: "content", C2S: false},
	{ID: 511, Name: "C2S_SKILL_UPGRADE", GoType: "C2SSkillUpgrade", File: "content", C2S: true},
	{ID: 512, Name: "C2S_POTENTIAL_ALLOCATE", GoType: "C2SPotentialAllocate", File: "content", C2S: true},
	{ID: 513, Name: "C2S_RESPEC", GoType: "C2SRespec", File: "content", C2S: true},
	{ID: 514, Name: "S2C_PROGRESSION_MUTATE_RESULT", GoType: "S2CProgressionMutateResult", File: "content", C2S: false},
	{ID: 515, Name: "S2C_PROGRESSION_STATE", GoType: "S2CProgressionState", File: "content", C2S: false},
	{ID: 516, Name: "C2S_DAILY_BOARD_REQUEST", GoType: "C2SDailyBoardRequest", File: "content", C2S: true},
	{ID: 517, Name: "S2C_DAILY_BOARD_STATE", GoType: "S2CDailyBoardState", File: "content", C2S: false},
	{ID: 518, Name: "S2C_ATLAS_STATE", GoType: "S2CAtlasState", File: "content", C2S: false},
	{ID: 600, Name: "C2S_CHAT_SEND", GoType: "C2SChatSend", File: "social", C2S: true},
	{ID: 601, Name: "S2C_CHAT_MESSAGE", GoType: "S2CChatMessage", File: "social", C2S: false},
	{ID: 602, Name: "C2S_PARTY_INVITE", GoType: "C2SPartyInvite", File: "social", C2S: true},
	{ID: 603, Name: "S2C_PARTY_INVITE", GoType: "S2CPartyInvite", File: "social", C2S: false},
	{ID: 604, Name: "C2S_PARTY_ACCEPT", GoType: "C2SPartyAccept", File: "social", C2S: true},
	{ID: 605, Name: "C2S_PARTY_LEAVE", GoType: "C2SPartyLeave", File: "social", C2S: true},
	{ID: 606, Name: "C2S_PARTY_KICK", GoType: "C2SPartyKick", File: "social", C2S: true},
	{ID: 607, Name: "S2C_PARTY_STATE", GoType: "S2CPartyState", File: "social", C2S: false},
	{ID: 608, Name: "C2S_GUILD_INVITE", GoType: "C2SGuildInvite", File: "social", C2S: true},
	{ID: 609, Name: "S2C_GUILD_INVITE", GoType: "S2CGuildInvite", File: "social", C2S: false},
	{ID: 610, Name: "C2S_GUILD_ACCEPT", GoType: "C2SGuildAccept", File: "social", C2S: true},
	{ID: 611, Name: "C2S_FRIEND_REQUEST", GoType: "C2SFriendRequest", File: "social", C2S: true},
	{ID: 612, Name: "S2C_FRIEND_REQUEST", GoType: "S2CFriendRequest", File: "social", C2S: false},
	{ID: 613, Name: "C2S_FRIEND_ACCEPT", GoType: "C2SFriendAccept", File: "social", C2S: true},
	{ID: 614, Name: "C2S_FRIEND_DECLINE", GoType: "C2SFriendDecline", File: "social", C2S: true},
	{ID: 615, Name: "C2S_FRIEND_REMOVE", GoType: "C2SFriendRemove", File: "social", C2S: true},
	{ID: 616, Name: "S2C_FRIEND_STATE", GoType: "S2CFriendState", File: "social", C2S: false},
	{ID: 617, Name: "C2S_BLOCK_ADD", GoType: "C2SBlockAdd", File: "social", C2S: true},
	{ID: 618, Name: "C2S_BLOCK_REMOVE", GoType: "C2SBlockRemove", File: "social", C2S: true},
	{ID: 619, Name: "S2C_BLOCK_STATE", GoType: "S2CBlockState", File: "social", C2S: false},
	{ID: 620, Name: "C2S_PARTY_DECLINE", GoType: "C2SPartyDecline", File: "social", C2S: true},
	{ID: 621, Name: "C2S_PARTY_INVITE_CANCEL", GoType: "C2SPartyInviteCancel", File: "social", C2S: true},
	{ID: 622, Name: "C2S_PARTY_LEADER_TRANSFER", GoType: "C2SPartyLeaderTransfer", File: "social", C2S: true},
	{ID: 623, Name: "C2S_GUILD_DECLINE", GoType: "C2SGuildDecline", File: "social", C2S: true},
	{ID: 624, Name: "C2S_GUILD_LEAVE", GoType: "C2SGuildLeave", File: "social", C2S: true},
	{ID: 625, Name: "C2S_GUILD_KICK", GoType: "C2SGuildKick", File: "social", C2S: true},
	{ID: 626, Name: "C2S_GUILD_ROLE_UPDATE", GoType: "C2SGuildRoleUpdate", File: "social", C2S: true},
	{ID: 627, Name: "C2S_GUILD_LEADER_TRANSFER", GoType: "C2SGuildLeaderTransfer", File: "social", C2S: true},
	{ID: 628, Name: "S2C_GUILD_STATE", GoType: "S2CGuildState", File: "social", C2S: false},
	{ID: 629, Name: "C2S_GUILD_STORAGE_DEPOSIT", GoType: "C2SGuildStorageDeposit", File: "social", C2S: true},
	{ID: 630, Name: "C2S_GUILD_STORAGE_WITHDRAW", GoType: "C2SGuildStorageWithdraw", File: "social", C2S: true},
	{ID: 631, Name: "S2C_GUILD_STORAGE_STATE", GoType: "S2CGuildStorageState", File: "social", C2S: false},
	{ID: 632, Name: "C2S_REPORT_PLAYER", GoType: "C2SReportPlayer", File: "social", C2S: true},
	{ID: 633, Name: "S2C_REPORT_PLAYER_RESULT", GoType: "S2CReportPlayerResult", File: "social", C2S: false},
	{ID: 634, Name: "C2S_PARTY_BOARD_POST", GoType: "C2SPartyBoardPost", File: "social", C2S: true},
	{ID: 635, Name: "C2S_PARTY_BOARD_CANCEL", GoType: "C2SPartyBoardCancel", File: "social", C2S: true},
	{ID: 636, Name: "S2C_PARTY_BOARD_STATE", GoType: "S2CPartyBoardState", File: "social", C2S: false},
	{ID: 637, Name: "C2S_GUILD_CREATE", GoType: "C2SGuildCreate", File: "social", C2S: true},
	{ID: 638, Name: "C2S_GUILD_DISBAND", GoType: "C2SGuildDisband", File: "social", C2S: true},
	{ID: 639, Name: "C2S_GUILD_APPLY", GoType: "C2SGuildApply", File: "social", C2S: true},
	{ID: 640, Name: "C2S_GUILD_APPLICATION_DECIDE", GoType: "C2SGuildApplicationDecide", File: "social", C2S: true},
	{ID: 641, Name: "S2C_GUILD_APPLICATIONS", GoType: "S2CGuildApplications", File: "social", C2S: false},
	{ID: 642, Name: "C2S_GUILD_MOTD_SET", GoType: "C2SGuildMotdSet", File: "social", C2S: true},
	{ID: 643, Name: "C2S_GUILD_LEADERSHIP_CLAIM", GoType: "C2SGuildLeadershipClaim", File: "social", C2S: true},
	{ID: 644, Name: "C2S_GUILD_STORAGE_MOVE", GoType: "C2SGuildStorageMove", File: "social", C2S: true},
	{ID: 645, Name: "C2S_GUILD_STORAGE_CLAIM_REQUEST", GoType: "C2SGuildStorageClaimRequest", File: "social", C2S: true},
	{ID: 646, Name: "C2S_GUILD_STORAGE_CLAIM_DECIDE", GoType: "C2SGuildStorageClaimDecide", File: "social", C2S: true},
	{ID: 647, Name: "S2C_GUILD_STORAGE_CLAIMS", GoType: "S2CGuildStorageClaims", File: "social", C2S: false},
	{ID: 648, Name: "C2S_GUILD_BLESSING_VOTE", GoType: "C2SGuildBlessingVote", File: "social", C2S: true},
	{ID: 649, Name: "S2C_GUILD_RESULT", GoType: "S2CGuildResult", File: "social", C2S: false},
	{ID: 650, Name: "C2S_GUILD_SETTINGS_SET", GoType: "C2SGuildSettingsSet", File: "social", C2S: true},
	{ID: 651, Name: "C2S_GUILD_INVITE_CANCEL", GoType: "C2SGuildInviteCancel", File: "social", C2S: true},
	{ID: 652, Name: "C2S_GUILD_APPLICATION_CANCEL", GoType: "C2SGuildApplicationCancel", File: "social", C2S: true},
	{ID: 653, Name: "S2C_PARTY_RESULT", GoType: "S2CPartyResult", File: "social", C2S: false},
	{ID: 654, Name: "S2C_SOCIAL_RESULT", GoType: "S2CSocialResult", File: "social", C2S: false},
	{ID: 655, Name: "S2C_CHAT_SEND_RESULT", GoType: "S2CChatSendResult", File: "social", C2S: false},
	{ID: 656, Name: "C2S_GUILD_COSMETIC_EQUIP", GoType: "C2SGuildCosmeticEquip", File: "social", C2S: true},
	{ID: 700, Name: "C2S_TRADE_INVITE", GoType: "C2STradeInvite", File: "market", C2S: true},
	{ID: 701, Name: "S2C_TRADE_INVITE", GoType: "S2CTradeInvite", File: "market", C2S: false},
	{ID: 702, Name: "C2S_TRADE_ACCEPT", GoType: "C2STradeAccept", File: "market", C2S: true},
	{ID: 703, Name: "C2S_TRADE_CANCEL", GoType: "C2STradeCancel", File: "market", C2S: true},
	{ID: 704, Name: "S2C_TRADE_CANCELLED", GoType: "S2CTradeCancelled", File: "market", C2S: false},
	{ID: 705, Name: "C2S_TRADE_OFFER_UPDATE", GoType: "C2STradeOfferUpdate", File: "market", C2S: true},
	{ID: 706, Name: "S2C_TRADE_OFFER_STATE", GoType: "S2CTradeOfferState", File: "market", C2S: false},
	{ID: 707, Name: "C2S_TRADE_CONFIRM", GoType: "C2STradeConfirm", File: "market", C2S: true},
	{ID: 708, Name: "C2S_TRADE_FINALISE", GoType: "C2STradeFinalise", File: "market", C2S: true},
	{ID: 709, Name: "S2C_TRADE_RESULT", GoType: "S2CTradeResult", File: "market", C2S: false},
	{ID: 710, Name: "S2C_TRADE_REQUEST_RESULT", GoType: "S2CTradeRequestResult", File: "market", C2S: false},
	{ID: 730, Name: "C2S_AUCTION_LIST", GoType: "C2SAuctionList", File: "market", C2S: true},
	{ID: 731, Name: "S2C_AUCTION_LIST_RESULT", GoType: "S2CAuctionListResult", File: "market", C2S: false},
	{ID: 732, Name: "C2S_AUCTION_BUY", GoType: "C2SAuctionBuy", File: "market", C2S: true},
	{ID: 733, Name: "S2C_AUCTION_BUY_RESULT", GoType: "S2CAuctionBuyResult", File: "market", C2S: false},
	{ID: 734, Name: "C2S_AUCTION_CANCEL_LISTING", GoType: "C2SAuctionCancelListing", File: "market", C2S: true},
	{ID: 735, Name: "S2C_AUCTION_CANCEL_RESULT", GoType: "S2CAuctionCancelResult", File: "market", C2S: false},
	{ID: 736, Name: "S2C_AUCTION_SOLD", GoType: "S2CAuctionSold", File: "market", C2S: false},
	{ID: 738, Name: "C2S_AUCTION_SEARCH", GoType: "C2SAuctionSearch", File: "market", C2S: true},
	{ID: 739, Name: "S2C_AUCTION_SEARCH_RESULT", GoType: "S2CAuctionSearchResult", File: "market", C2S: false},
	{ID: 740, Name: "C2S_AUCTION_RECLAIM", GoType: "C2SAuctionReclaim", File: "market", C2S: true},
	{ID: 741, Name: "S2C_AUCTION_RECLAIM_RESULT", GoType: "S2CAuctionReclaimResult", File: "market", C2S: false},
	{ID: 742, Name: "C2S_AUCTION_PROCEEDS_CLAIM", GoType: "C2SAuctionProceedsClaim", File: "market", C2S: true},
	{ID: 743, Name: "S2C_AUCTION_PROCEEDS_RESULT", GoType: "S2CAuctionProceedsResult", File: "market", C2S: false},
	{ID: 744, Name: "S2C_AUCTION_MY_STATE", GoType: "S2CAuctionMyState", File: "market", C2S: false},
	{ID: 800, Name: "C2S_SPARRING_REQUEST", GoType: "C2SSparringRequest", File: "pvp", C2S: true},
	{ID: 801, Name: "C2S_SPARRING_ACCEPT", GoType: "C2SSparringAccept", File: "pvp", C2S: true},
	{ID: 802, Name: "C2S_RANKED_QUEUE_JOIN", GoType: "C2SRankedQueueJoin", File: "pvp", C2S: true},
	{ID: 803, Name: "C2S_RANKED_QUEUE_LEAVE", GoType: "C2SRankedQueueLeave", File: "pvp", C2S: true},
	{ID: 804, Name: "S2C_RANKED_QUEUE_UPDATE", GoType: "S2CRankedQueueUpdate", File: "pvp", C2S: false},
	{ID: 805, Name: "C2S_MATCH_READY", GoType: "C2SMatchReady", File: "pvp", C2S: true},
	{ID: 806, Name: "S2C_MATCH_STATE", GoType: "S2CMatchState", File: "pvp", C2S: false},
	{ID: 807, Name: "C2S_MATCH_SURRENDER", GoType: "C2SMatchSurrender", File: "pvp", C2S: true},
	{ID: 808, Name: "C2S_GUILD_WAR_QUEUE_JOIN", GoType: "C2SGuildWarQueueJoin", File: "pvp", C2S: true},
	{ID: 809, Name: "C2S_GUILD_WAR_QUEUE_LEAVE", GoType: "C2SGuildWarQueueLeave", File: "pvp", C2S: true},
	{ID: 810, Name: "S2C_GUILD_WAR_STATE", GoType: "S2CGuildWarState", File: "pvp", C2S: false},
	{ID: 811, Name: "S2C_SPARRING_CHALLENGE", GoType: "S2CSparringChallenge", File: "pvp", C2S: false},
	{ID: 812, Name: "C2S_SPARRING_DECLINE", GoType: "C2SSparringDecline", File: "pvp", C2S: true},
	{ID: 813, Name: "S2C_SPARRING_OUTCOME", GoType: "S2CSparringOutcome", File: "pvp", C2S: false},
	{ID: 814, Name: "C2S_DUEL_CHALLENGE", GoType: "C2SDuelChallenge", File: "pvp", C2S: true},
	{ID: 815, Name: "S2C_DUEL_CHALLENGE", GoType: "S2CDuelChallenge", File: "pvp", C2S: false},
	{ID: 816, Name: "C2S_DUEL_RESPOND", GoType: "C2SDuelRespond", File: "pvp", C2S: true},
	{ID: 817, Name: "C2S_DUEL_CANCEL", GoType: "C2SDuelCancel", File: "pvp", C2S: true},
	{ID: 818, Name: "S2C_DUEL_OUTCOME", GoType: "S2CDuelOutcome", File: "pvp", C2S: false},
	{ID: 819, Name: "S2C_PVP_RESULT", GoType: "S2CPvpResult", File: "pvp", C2S: false},
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// server/internal/testing/protocol/helpers_test.go -> repo root.
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "proto", "thinhthan", "v1")); err != nil {
		t.Fatalf("repo root %q missing proto tree: %v", root, err)
	}
	return root
}

// errorsMdCodes mechanically enumerates the canonical codes from errors.md:
// every fenced block inside section "Canonical Codes" (ending before section
// "Disconnect Policy"), uppercase tokens matching [A-Z][A-Z0-9_]{2,}, row-major
// order, unique. Baseline: 112 codes.
func errorsMdCodes(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "05_network", "errors.md"))
	if err != nil {
		t.Fatalf("read errors.md: %v", err)
	}
	text := string(data)
	start := strings.Index(text, "## Canonical Codes")
	end := strings.Index(text, "## Disconnect Policy")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("errors.md canonical section markers not found")
	}
	body := text[start:end]
	fenced := regexp.MustCompile("(?s)"+"```[a-zA-Z]*\n(.*?)```").FindAllStringSubmatch(body, -1)
	if len(fenced) == 0 {
		t.Fatal("errors.md: no fenced blocks in Canonical Codes")
	}
	token := regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)
	var codes []string
	for _, block := range fenced {
		for _, line := range strings.Split(block[1], "\n") {
			for _, tok := range strings.Fields(line) {
				if token.MatchString(tok) {
					codes = append(codes, tok)
				}
			}
		}
	}
	return codes
}

// goldenFixture is one entry of proto/testdata/golden/index.json; the file is
// an object ({"fixtures":[...]}) so Unity JsonUtility can read it too.
type goldenFixture struct {
	File    string `json:"file"`
	Type    string `json:"type"`
	Journal bool   `json:"journal"`
}

type goldenIndexFile struct {
	Fixtures []goldenFixture `json:"fixtures"`
}

func goldenIndex(t *testing.T) []goldenFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "proto", "testdata", "golden", "index.json"))
	if err != nil {
		t.Fatalf("read golden index: %v", err)
	}
	var index goldenIndexFile
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("parse golden index: %v", err)
	}
	return index.Fixtures
}

func goldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "proto", "testdata", "golden")
}

// messageType resolves a proto message type by full name.
func messageType(t *testing.T, fullName string) protoreflect.MessageType {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(fullName))
	if err != nil {
		t.Fatalf("message type %s not registered: %v", fullName, err)
	}
	return mt
}

// registryIDs returns the network registry IDs sorted ascending.
func registryIDs() []uint32 {
	ids := make([]uint32, 0, len(networkRegistry))
	for _, e := range networkRegistry {
		ids = append(ids, e.ID)
	}
	return ids
}
