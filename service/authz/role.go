package authz

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	BuiltInRoleRoot  = "root"
	BuiltInRoleAdmin = "admin"
)

var (
	ErrRoleNotFound      = errors.New("authorization role not found")
	ErrRoleExists        = errors.New("authorization role already exists")
	ErrRoleBuiltIn       = errors.New("built-in authorization roles cannot be changed")
	ErrRoleInvalid       = errors.New("invalid authorization role")
	ErrPermissionUnknown = errors.New("unknown permission")
)

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// RoleSpec describes a role. A superuser role is allowed every permission
// without an explicit policy entry.
type RoleSpec struct {
	Key         string
	Name        string
	Description string
	BuiltIn     bool
	Superuser   bool
	Sort        int
}

var builtInRoles = []RoleSpec{
	{
		Key:         BuiltInRoleRoot,
		Name:        "Root",
		Description: "Built-in root authorization role",
		BuiltIn:     true,
		Superuser:   true,
		Sort:        0,
	},
	{
		Key:         BuiltInRoleAdmin,
		Name:        "Admin",
		Description: "Built-in admin authorization role",
		BuiltIn:     true,
		Superuser:   false,
		Sort:        10,
	},
}

// RoleDescriptor exposes a role together with its baseline grant matrix.
type RoleDescriptor struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	BuiltIn     bool           `json:"built_in"`
	Superuser   bool           `json:"superuser"`
	Grants      PermissionsMap `json:"grants"`
}

// Roles returns the built-in roles followed by the custom roles stored in
// authz_roles, each with its grant matrix.
func Roles() ([]RoleDescriptor, error) {
	result := make([]RoleDescriptor, 0, len(builtInRoles))
	for _, spec := range builtInRoles {
		result = append(result, RoleDescriptor{
			Key:         spec.Key,
			Name:        spec.Name,
			Description: spec.Description,
			BuiltIn:     true,
			Superuser:   spec.Superuser,
			Grants:      roleGrants(spec.Key),
		})
	}
	db := currentDB()
	if db == nil {
		return result, nil
	}
	var custom []model.AuthzRole
	if err := db.Where("built_in = ?", false).Order("sort asc, id asc").Find(&custom).Error; err != nil {
		return nil, err
	}
	for _, role := range custom {
		result = append(result, RoleDescriptor{
			Key:         role.Key,
			Name:        role.Name,
			Description: role.Description,
			Grants:      roleGrants(role.Key),
		})
	}
	return result, nil
}

// roleGrants is the grant matrix of a role: everything for a superuser, the
// registry defaults for a built-in role, and the stored policy for a custom one.
func roleGrants(roleKey string) PermissionsMap {
	superuser := isSuperuserRole(roleKey)
	builtIn := isBuiltInRole(roleKey)
	e := currentEnforcer()
	grants := make(PermissionsMap, len(registry))
	for _, resource := range registry {
		actions := make(map[string]bool, len(resource.Actions))
		for _, action := range resource.Actions {
			switch {
			case superuser:
				actions[action.Action] = true
			case builtIn:
				actions[action.Action] = actionHasRole(action, roleKey)
			default:
				actions[action.Action] = e != nil && roleBaselineAllows(e, roleKey, Permission{Resource: resource.Resource, Action: action.Action})
			}
		}
		grants[resource.Resource] = actions
	}
	return grants
}

func roleSpec(roleKey string) (RoleSpec, bool) {
	for _, spec := range builtInRoles {
		if spec.Key == roleKey {
			return spec, true
		}
	}
	return RoleSpec{}, false
}

func isBuiltInRole(roleKey string) bool {
	_, ok := roleSpec(roleKey)
	return ok
}

// isSuperuserRole is true only for root: custom roles never bypass policy.
func isSuperuserRole(roleKey string) bool {
	spec, ok := roleSpec(roleKey)
	return ok && spec.Superuser
}

func builtInRoleSubjects() []string {
	subjects := make([]string, 0, len(builtInRoles))
	for _, spec := range builtInRoles {
		subjects = append(subjects, RoleSubject(spec.Key))
	}
	return subjects
}

// normalizeRoleText trims a role name or description and checks its length.
func normalizeRoleText(value string, required bool, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", false
	}
	return value, utf8.RuneCountInString(value) <= maxRunes
}

// CreateRole stores a custom role without permissions.
func CreateRole(key, name, description string) error {
	db := currentDB()
	if db == nil {
		return errors.New("authz enforcer is not initialized")
	}
	name, nameOK := normalizeRoleText(name, true, 100)
	description, descriptionOK := normalizeRoleText(description, false, 500)
	if !roleKeyPattern.MatchString(key) || !nameOK || !descriptionOK {
		return ErrRoleInvalid
	}
	if isBuiltInRole(key) {
		return ErrRoleExists
	}
	var count int64
	if err := db.Model(&model.AuthzRole{}).Where(map[string]any{"key": key}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrRoleExists
	}
	return db.Create(&model.AuthzRole{Key: key, Name: name, Description: description, Enabled: true, Sort: 100}).Error
}

// UpdateRole renames a custom role or changes its description.
func UpdateRole(key, name, description string) error {
	db := currentDB()
	if db == nil {
		return errors.New("authz enforcer is not initialized")
	}
	if isBuiltInRole(key) {
		return ErrRoleBuiltIn
	}
	name, nameOK := normalizeRoleText(name, true, 100)
	description, descriptionOK := normalizeRoleText(description, false, 500)
	if !nameOK || !descriptionOK {
		return ErrRoleInvalid
	}
	result := db.Model(&model.AuthzRole{}).Where(map[string]any{"key": key, "built_in": false}).
		Updates(map[string]any{"name": name, "description": description})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// RowsAffected can be 0 on MySQL when nothing changed, so confirm.
		if _, err := customRole(db, key); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRole removes a custom role, its grants and its assignments.
func DeleteRole(key string) error {
	db := currentDB()
	if db == nil {
		return errors.New("authz enforcer is not initialized")
	}
	if isBuiltInRole(key) {
		return ErrRoleBuiltIn
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where(map[string]any{"key": key, "built_in": false}).Delete(&model.AuthzRole{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrRoleNotFound
		}
		if err := tx.Where("ptype = ? AND v0 = ?", "p", RoleSubject(key)).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		return tx.Where("ptype = ? AND v1 = ?", "g", RoleSubject(key)).Delete(&model.CasbinRule{}).Error
	}); err != nil {
		return err
	}
	return ReloadPolicy()
}

// SetRolePermissions replaces the grants of a custom role with the given
// permission codes ("resource.action").
func SetRolePermissions(key string, codes []string) error {
	db := currentDB()
	if db == nil {
		return errors.New("authz enforcer is not initialized")
	}
	if isBuiltInRole(key) {
		return ErrRoleBuiltIn
	}
	rules := make([]model.CasbinRule, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		permission, ok := ParsePermissionCode(code)
		if !ok {
			return ErrPermissionUnknown
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		rules = append(rules, newRule("p", []string{RoleSubject(key), permission.Resource, permission.Action, EffectAllow}))
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := customRole(tx, key); err != nil {
			return err
		}
		if err := tx.Where("ptype = ? AND v0 = ?", "p", RoleSubject(key)).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		if len(rules) == 0 {
			return nil
		}
		return tx.Create(&rules).Error
	}); err != nil {
		return err
	}
	return ReloadPolicy()
}

// SetUserRoles replaces the custom roles assigned to a user. An empty list
// returns the user to the baseline of their system role.
func SetUserRoles(userID int, keys []string) error {
	db := currentDB()
	if db == nil {
		return errors.New("authz enforcer is not initialized")
	}
	keys = slices.Clone(keys)
	slices.Sort(keys)
	keys = slices.Compact(keys)
	if slices.ContainsFunc(keys, isBuiltInRole) {
		return ErrRoleInvalid
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(keys) > 0 {
			var count int64
			if err := tx.Model(&model.AuthzRole{}).Where(map[string]any{"key": keys, "built_in": false}).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(keys)) {
				return ErrRoleNotFound
			}
		}
		// Legacy assignments to built-in roles are inert and kept for review.
		if err := tx.Where("ptype = ? AND v0 = ? AND v1 NOT IN ?", "g", UserSubject(userID), builtInRoleSubjects()).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		rules := make([]model.CasbinRule, 0, len(keys))
		for _, key := range keys {
			rules = append(rules, newRule("g", []string{UserSubject(userID), RoleSubject(key)}))
		}
		return tx.Create(&rules).Error
	}); err != nil {
		return err
	}
	return ReloadPolicy()
}

func customRole(db *gorm.DB, key string) (*model.AuthzRole, error) {
	var role model.AuthzRole
	err := db.Where(map[string]any{"key": key, "built_in": false}).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}
