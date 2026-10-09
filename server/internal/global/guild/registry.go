package guild

// GuildResultRequests is the closed C2S set every one of which commits
// exactly one S2C_GUILD_RESULT (649) — ADR-0064 wire-shape contract.
// Storage ids (629, 630, 644, 645, 646) are implemented by IMP-037's
// durable/guild_storage; cosmetic 656 by IMP-038 — coverage here is the
// wire contract for the whole guild surface.
var GuildResultRequests = []uint32{
	608, 610, 623, 624, 625, 626, 627,
	629, 630,
	637, 638, 639, 640, 642, 643, 644, 645, 646, 648,
	650, 651, 652, 656,
}

// GuildResultMessageID is the single S2C result id.
const GuildResultMessageID uint32 = 649

// Covers reports whether a request id belongs to the guild closed set.
func Covers(requestID uint32) bool {
	for _, id := range GuildResultRequests {
		if id == requestID {
			return true
		}
	}
	return false
}
