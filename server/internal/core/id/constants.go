package id

// Pinned, immutable identity constants (technology_versions.md § Pinned
// Content System Constants, data_model.md). Never rotate or rederive.
var (
	// ContentGrantNamespaceUUID is the project UUID v5 namespace for all
	// deterministic content-grant idempotency keys and sim/content-grant
	// operation IDs (ids.md § Deterministic Content-Grant Idempotency Keys).
	ContentGrantNamespaceUUID = mustParseUUID("f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b")

	// ServerJobNamespaceUUID is the project UUID v5 namespace for
	// server-initiated job operation IDs (ids.md § Operation IDs, ADR-0070).
	ServerJobNamespaceUUID = mustParseUUID("64d34c40-8657-462b-887f-5970db9eaa5f")

	// WorldOwnerID is the reserved owner of world-scoped operations
	// (data_model.md); world commands use it, never a partition-derived owner.
	WorldOwnerID = mustParseUUID("00000000-0000-0000-0000-000000000002")
)

func mustParseUUID(s string) UUID {
	u, err := ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}
