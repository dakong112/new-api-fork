package authz

import (
	"strconv"
	"strings"
)

// Permission identifies a single action on a resource.
type Permission struct {
	Resource string
	Action   string
}

// PermissionsMap is a resource -> action -> allowed lookup.
type PermissionsMap map[string]map[string]bool

const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// UserSubject is the casbin subject string for a single user.
func UserSubject(userID int) string {
	return "user:" + strconv.Itoa(userID)
}

// RoleSubject is the casbin subject string for a role.
func RoleSubject(roleKey string) string {
	return "role:" + roleKey
}

// Code is the permission as one "resource.action" string. Resource names may
// contain dots ("canvas.template"); the action is the last segment.
func (permission Permission) Code() string {
	return permission.Resource + "." + permission.Action
}

// ParsePermissionCode parses a "resource.action" code and reports whether it
// names a registered permission.
func ParsePermissionCode(code string) (Permission, bool) {
	idx := strings.LastIndexByte(code, '.')
	if idx <= 0 || idx == len(code)-1 {
		return Permission{}, false
	}
	permission := Permission{Resource: code[:idx], Action: code[idx+1:]}
	return permission, isKnownPermission(permission)
}
