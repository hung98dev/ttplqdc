package gates

import (
	"fmt"
	"strings"
)

// Role is the PR role derived from the head-branch prefix.
type Role string

const (
	RoleSpecOwner   Role = "spec-owner"
	RoleCoordinator Role = "coordinator"
	RoleImplementer Role = "implementer"
	RoleMergeGuard  Role = "merge-guard"
)

// RoleForBranch maps the branch prefix to a role (audit_gates.md Protected
// Paths; every other prefix is an error — fail closed).
func RoleForBranch(branch string) (Role, error) {
	switch {
	case strings.HasPrefix(branch, "spec/"):
		return RoleSpecOwner, nil
	case strings.HasPrefix(branch, "claim/"), strings.HasPrefix(branch, "ops/"):
		return RoleCoordinator, nil
	case strings.HasPrefix(branch, "imp/"), strings.HasPrefix(branch, "block/"):
		return RoleImplementer, nil
	case strings.HasPrefix(branch, "revert/"):
		return RoleMergeGuard, nil
	default:
		return "", fmt.Errorf("unknown branch role prefix: %q", branch)
	}
}
