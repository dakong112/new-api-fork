package authz

import (
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedBuiltInRoles(db *gorm.DB) error {
	for _, spec := range builtInRoles {
		role := model.AuthzRole{
			Key:         spec.Key,
			Name:        spec.Name,
			Description: spec.Description,
			BuiltIn:     spec.BuiltIn,
			Enabled:     true,
			Sort:        spec.Sort,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name",
				"description",
				"built_in",
				"enabled",
				"sort",
			}),
		}).Create(&role).Error; err != nil {
			return err
		}
	}
	return nil
}

// presetRole is a custom role created on first start. Operators may edit or
// delete it afterwards; seeding never overwrites an existing row.
type presetRole struct {
	Key         string
	Name        string
	Description string
	Permissions []Permission
}

var presetRoles = []presetRole{
	{
		Key:         "operations",
		Name:        "运营",
		Description: "管理画布模板",
		Permissions: []Permission{CanvasTemplateRead, CanvasTemplateWrite, CanvasTemplatePublish},
	},
	{
		Key:         "support",
		Name:        "客服",
		Description: "查看用户与订单",
		Permissions: []Permission{CanvasUserRead, CanvasOrderRead, UserRead},
	},
	{
		Key:         "finance",
		Name:        "财务",
		Description: "查看日志、充值与订单",
		Permissions: []Permission{LogRead, BillingRead, CanvasOrderRead},
	},
}

// seedPresetRoles inserts each preset role that does not exist yet, together
// with its grants. Existing roles keep whatever the operator configured.
// ponytail: a deleted preset comes back on the next start; store a "seeded"
// marker if operators need to delete presets for good.
func seedPresetRoles(db *gorm.DB) error {
	for _, preset := range presetRoles {
		err := db.Transaction(func(tx *gorm.DB) error {
			var count int64
			if err := tx.Model(&model.AuthzRole{}).Where(map[string]any{"key": preset.Key}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			if err := tx.Create(&model.AuthzRole{Key: preset.Key, Name: preset.Name, Description: preset.Description, Enabled: true, Sort: 100}).Error; err != nil {
				return err
			}
			rules := make([]model.CasbinRule, 0, len(preset.Permissions))
			for _, permission := range preset.Permissions {
				rules = append(rules, newRule("p", []string{RoleSubject(preset.Key), permission.Resource, permission.Action, EffectAllow}))
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rules).Error
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func resetBuiltInRolePolicies(db *gorm.DB) error {
	subjects := builtInRoleSubjects()
	// Scoped legacy rules must survive baseline reseeding so the adapter can
	// retain their restrictions instead of replacing them with global grants.
	return db.Where("ptype = ? AND v0 IN ?", "p", subjects).
		Where("(v4 = ? OR v4 IS NULL) AND (v5 = ? OR v5 IS NULL)", "", "").
		Delete(&model.CasbinRule{}).Error
}

func seedDefaultPolicies() error {
	e := currentEnforcer()
	if e == nil {
		return fmt.Errorf("authz enforcer is not initialized")
	}

	for _, spec := range builtInRoles {
		if spec.Superuser {
			continue
		}
		for _, permission := range PermissionsForRole(spec.Key) {
			if _, err := e.AddPolicy(RoleSubject(spec.Key), permission.Resource, permission.Action, EffectAllow); err != nil {
				return err
			}
		}
	}
	return nil
}
