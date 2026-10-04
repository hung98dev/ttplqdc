// Package aoi implements the server-owned Area-of-Interest model of
// docs/05_network/synchronization.md: spatial enter/leave radii with distance
// hysteresis, the MAX_ENTITIES_IN_AOI_PER_CLIENT cap with integer count
// hysteresis, and the fixed shed-priority order.
//
// The package is a leaf: it imports only the standard library. All buffers
// are caller-owned so the per-tick interest update is allocation-free
// (capacity.md HOT-001).
package aoi
