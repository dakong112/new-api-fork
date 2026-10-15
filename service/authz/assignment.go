package authz

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// resolveSubjectRoles returns the role keys a subject acts under. Root is
// always the superuser role. Custom roles assigned through "g" rules replace
// the admin baseline, so an administrator holding a custom role is limited to
// that role; without one an administrator keeps the built-in admin role and a
// common user has no role.
var resolveSubjectRoles = func(userID int, systemRole int) []string {
	if systemRole >= common.RoleRootUser {
		return []string{BuiltInRoleRoot}
	}
	if roles := UserRoles(userID); len(roles) > 0 {
		return roles
	}
	if systemRole >= common.RoleAdminUser {
		return []string{BuiltInRoleAdmin}
	}
	return nil
}

// UserRoles returns the sorted custom role keys assigned to the user.
// Assignments to built-in roles are ignored: those follow the system role.
func UserRoles(userID int) []string {
	e := currentEnforcer()
	if e == nil {
		return nil
	}
	rules, err := e.GetFilteredGroupingPolicy(0, UserSubject(userID))
	if err != nil {
		return nil
	}
	roles := make([]string, 0, len(rules))
	for _, rule := range rules {
		if len(rule) < 2 {
			continue
		}
		key, ok := strings.CutPrefix(rule[1], RoleSubject(""))
		if ok && key != "" && !isBuiltInRole(key) {
			roles = append(roles, key)
		}
	}
	slices.Sort(roles)
	return slices.Compact(roles)
}

// managedRoleKey is the role whose baseline per-user overrides are expressed
// relative to.
const managedRoleKey = BuiltInRoleAdmin
