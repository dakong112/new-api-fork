package authz

import (
	"slices"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAuthzTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	wasMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() {
		common.IsMasterNode = wasMaster
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CasbinRule{}, &model.AuthzRole{}))
	return db
}

func TestInitSeedsBuiltInRolesAndPoliciesOnce(t *testing.T) {
	db := newAuthzTestDB(t)

	require.NoError(t, Init(db))
	require.NoError(t, Init(db))

	// root is a superuser role and is granted everything implicitly, so only the
	// admin baseline and the preset roles are written as explicit policy rows.
	presetGrants := 0
	for _, preset := range presetRoles {
		presetGrants += len(preset.Permissions)
	}
	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Count(&count).Error)
	assert.Equal(t, int64(len(PermissionsForRole(BuiltInRoleAdmin))+presetGrants), count)

	var roles []model.AuthzRole
	require.NoError(t, db.Order("sort asc, id asc").Find(&roles).Error)
	keys := make([]string, 0, len(roles))
	for _, role := range roles {
		keys = append(keys, role.Key)
		assert.Equal(t, role.Key == BuiltInRoleRoot || role.Key == BuiltInRoleAdmin, role.BuiltIn, role.Key)
	}
	assert.Equal(t, []string{BuiltInRoleRoot, BuiltInRoleAdmin, "operations", "support", "finance"}, keys)

	assert.True(t, Can(1, common.RoleRootUser, ChannelSensitiveWrite))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelRead))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelOperate))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelWrite))
	assert.False(t, Can(2, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(3, common.RoleCommonUser, ChannelRead))
}

func TestInitOnSlaveOnlyLoadsPolicies(t *testing.T) {
	wasMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() {
		common.IsMasterNode = wasMaster
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CasbinRule{}, &model.AuthzRole{}))

	require.NoError(t, Init(db))

	var roleCount int64
	require.NoError(t, db.Model(&model.AuthzRole{}).Count(&roleCount).Error)
	assert.Equal(t, int64(0), roleCount)
	var policyCount int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Count(&policyCount).Error)
	assert.Equal(t, int64(0), policyCount)
	assert.False(t, Can(2, common.RoleAdminUser, ChannelRead))
}

func TestLegacyScopedPoliciesDoNotExpandPermissions(t *testing.T) {
	for _, master := range []bool{true, false} {
		name := "master"
		if !master {
			name = "slave"
		}
		t.Run(name, func(t *testing.T) {
			db := newAuthzTestDB(t)
			common.IsMasterNode = master
			rules := []model.CasbinRule{
				{Ptype: "p", V0: "role:vendor", V1: "channel", V2: "read", V3: "allow", V4: "own"},
				{Ptype: "p", V0: UserSubject(42), V1: "channel", V2: "read", V3: "allow", V4: "own"},
				{Ptype: "p", V0: UserSubject(43), V1: "channel", V2: "sensitive_write", V3: "allow", V4: "all"},
				{Ptype: "p", V0: UserSubject(44), V1: "channel", V2: "secret_view", V3: "deny", V4: "all"},
				{Ptype: "p", V0: UserSubject(45), V1: "channel", V2: "read"},
				{Ptype: "p", V0: UserSubject(46), V1: "channel", V2: "read", V3: "allow", V4: "unknown-scope"},
				{Ptype: "p", V0: UserSubject(47), V1: "channel", V2: "read", V3: "allow", V5: "own"},
				{Ptype: "p", V0: UserSubject(48), V1: "channel", V2: "read", V3: "allow", V4: "own"},
				{Ptype: "p", V0: UserSubject(48), V1: "channel", V2: "read", V3: "allow", V4: "all"},
				{Ptype: "p", V0: RoleSubject(BuiltInRoleAdmin), V1: "channel", V2: "operate", V3: "allow", V4: "own"},
				{Ptype: "p", V0: UserSubject(50), V1: "channel", V2: "sensitive_write", V3: "allow"},
				{Ptype: "g", V0: UserSubject(99), V1: RoleSubject(BuiltInRoleAdmin)},
			}
			require.NoError(t, db.Create(&rules).Error)
			ids := make([]uint, len(rules))
			for i := range rules {
				ids[i] = rules[i].Id
			}
			for range 2 {
				require.NoError(t, Init(db))
				require.NoError(t, ReloadPolicy())
				assert.False(t, Can(42, common.RoleAdminUser, ChannelRead), "own must not fall back to the admin allow baseline")
				assert.True(t, Can(43, common.RoleAdminUser, ChannelSensitiveWrite))
				assert.False(t, Can(44, common.RoleAdminUser, ChannelSecretView))
				assert.True(t, Can(45, common.RoleAdminUser, ChannelRead))
				for _, userID := range []int{46, 47, 48} {
					assert.False(t, Can(userID, common.RoleAdminUser, ChannelRead))
				}
				assert.False(t, Can(51, common.RoleAdminUser, ChannelOperate), "reseed must not erase a scoped role restriction")
				assert.True(t, Can(50, common.RoleAdminUser, ChannelSensitiveWrite))
				assert.False(t, Can(99, common.RoleCommonUser, ChannelRead))
				var stored []model.CasbinRule
				require.NoError(t, db.Where("id IN ?", ids).Order("id").Find(&stored).Error)
				assert.Equal(t, rules, stored, "legacy rows must remain available for administrator review")
			}
		})
	}
}

func TestSetUserPermissionsStoresOnlyOverrides(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, SetUserPermissions(42, PermissionsMap{
		ResourceChannel: {
			ActionRead:           true,
			ActionOperate:        true,
			ActionWrite:          false,
			ActionSensitiveWrite: true,
			ActionSecretView:     false,
			"unknown":            true,
		},
		"unknown": {
			ActionRead: true,
		},
	}))

	assert.True(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(42, common.RoleAdminUser, ChannelWrite))
	explicit := ExplicitUserPermissions(42)
	assert.Equal(t, map[string]bool{
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          false,
		ActionSensitiveWrite: true,
		ActionSecretView:     false,
	}, explicit[ResourceChannel])
	assert.Equal(t, map[string]bool{ActionBind: false}, explicit[ResourceTaskPlugin])
	assert.Equal(t, map[string]bool{ActionRead: true, ActionWrite: true}, explicit[UserRead.Resource])
	assert.Equal(t, PermissionsMap{
		ResourceChannel: {
			ActionSensitiveWrite: true,
			ActionWrite:          false,
		},
	}, ExplicitUserOverrides(42))

	var userPolicyCount int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("v0 = ?", UserSubject(42)).Count(&userPolicyCount).Error)
	assert.Equal(t, int64(2), userPolicyCount)

	require.NoError(t, SetUserPermissions(42, PermissionsMap{ResourceChannel: {
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          true,
		ActionSensitiveWrite: false,
		ActionSecretView:     false,
	}}))
	assert.False(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.Equal(t, map[string]bool{
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          true,
		ActionSensitiveWrite: false,
		ActionSecretView:     false,
	}, ExplicitUserPermissions(42)[ResourceChannel])
	assert.Empty(t, ExplicitUserOverrides(42))
}

func TestClearUserAuthorizationRemovesOverrides(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, SetUserPermissions(90, PermissionsMap{ResourceChannel: {
		ActionWrite:          false,
		ActionSensitiveWrite: true,
	}}))

	assert.True(t, Can(90, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(90, common.RoleAdminUser, ChannelWrite))

	require.NoError(t, ClearUserAuthorization(90))

	assert.Empty(t, ExplicitUserOverrides(90))
	assert.True(t, Can(90, common.RoleAdminUser, ChannelRead))
	assert.True(t, Can(90, common.RoleAdminUser, ChannelWrite))
	assert.False(t, Can(90, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(90, common.RoleCommonUser, ChannelRead))
}

func TestSetUserPermissionsInTxDoesNotMutateEnforcerBeforeReload(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return SetUserPermissionsInTx(tx, 42, PermissionsMap{ResourceChannel: {
			ActionRead:           true,
			ActionOperate:        true,
			ActionWrite:          true,
			ActionSensitiveWrite: true,
			ActionSecretView:     false,
		}})
	}))

	assert.False(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	require.NoError(t, ReloadPolicy())
	assert.True(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
}

func TestSetUserPermissionsInTxRollbackLeavesNoPolicy(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	tx := db.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, SetUserPermissionsInTx(tx, 43, PermissionsMap{ResourceChannel: {
		ActionSensitiveWrite: true,
	}}))
	require.NoError(t, tx.Rollback().Error)
	require.NoError(t, ReloadPolicy())

	assert.False(t, Can(43, common.RoleAdminUser, ChannelSensitiveWrite))
	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("v0 = ?", UserSubject(43)).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestAdapterAddPolicyIsIdempotent(t *testing.T) {
	db := newAuthzTestDB(t)
	adapter := newGormAdapter(db)
	rule := []string{UserSubject(55), ResourceChannel, ActionSensitiveWrite, EffectAllow}

	require.NoError(t, adapter.AddPolicy("p", "p", rule))
	require.NoError(t, adapter.AddPolicy("p", "p", rule))

	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		UserSubject(55),
		ResourceChannel,
		ActionSensitiveWrite,
		EffectAllow,
	).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCapabilitiesUseCatalogShape(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	capabilities := Capabilities(7, common.RoleAdminUser)

	assert.True(t, capabilities[ResourceChannel][ActionRead])
	assert.True(t, capabilities[ResourceChannel][ActionOperate])
	assert.True(t, capabilities[ResourceChannel][ActionWrite])
	assert.False(t, capabilities[ResourceChannel][ActionSensitiveWrite])
	assert.False(t, capabilities[ResourceChannel][ActionSecretView])
	assert.False(t, capabilities[ResourceTaskPlugin][ActionBind])
}

func TestTaskPluginBindIsRootOnlyUntilGranted(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	var bindAction *ActionDefinition
	for _, resource := range Catalog() {
		if resource.Resource != ResourceTaskPlugin {
			continue
		}
		assert.Equal(t, "Task Plugin", resource.LabelKey)
		for i := range resource.Actions {
			if resource.Actions[i].Action == ActionBind {
				bindAction = &resource.Actions[i]
			}
		}
	}
	require.NotNil(t, bindAction)
	assert.Equal(t, "Bind task plugins", bindAction.LabelKey)
	assert.Equal(t, "List registered task plugins and bind them when creating or editing task plugin channels.", bindAction.DescriptionKey)
	assert.Empty(t, bindAction.DefaultRoles)

	assert.False(t, Can(2, common.RoleAdminUser, TaskPluginBind))
	assert.True(t, Can(1, common.RoleRootUser, TaskPluginBind))

	enforcer := currentEnforcer()
	require.NotNil(t, enforcer)
	_, err := enforcer.AddPolicy(RoleSubject(BuiltInRoleAdmin), ResourceTaskPlugin, ActionBind, EffectAllow)
	require.NoError(t, err)
	assert.True(t, Can(2, common.RoleAdminUser, TaskPluginBind))

	_, err = enforcer.RemovePolicy(RoleSubject(BuiltInRoleAdmin), ResourceTaskPlugin, ActionBind, EffectAllow)
	require.NoError(t, err)
	assert.False(t, Can(2, common.RoleAdminUser, TaskPluginBind))
}

func TestPresetRolesAreSeededOnceAndNeverOverwritten(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))
	assert.Equal(t, []string{CanvasOrderRead.Code(), CanvasUserRead.Code(), UserRead.Code()}, sortedCodes(roleGrants("support")))

	require.NoError(t, SetRolePermissions("support", []string{CanvasUserRead.Code()}))
	require.NoError(t, UpdateRole("support", "Help desk", ""))
	require.NoError(t, Init(db))

	assert.Equal(t, []string{CanvasUserRead.Code()}, sortedCodes(roleGrants("support")))
	var role model.AuthzRole
	require.NoError(t, db.Where(map[string]any{"key": "support"}).First(&role).Error)
	assert.Equal(t, "Help desk", role.Name)
	var count int64
	require.NoError(t, db.Model(&model.AuthzRole{}).Where(map[string]any{"key": "support"}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func sortedCodes(grants PermissionsMap) []string {
	codes := make([]string, 0)
	for resource, actions := range grants {
		for action, allowed := range actions {
			if allowed {
				codes = append(codes, Permission{Resource: resource, Action: action}.Code())
			}
		}
	}
	slices.Sort(codes)
	return codes
}

func TestCustomRoleResolutionMatrix(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))
	require.NoError(t, CreateRole("auditor", "Auditor", ""))
	require.NoError(t, SetRolePermissions("auditor", []string{LogRead.Code(), CanvasOrderRead.Code(), LogRead.Code()}))
	require.NoError(t, SetUserRoles(10, []string{"auditor"}))
	require.NoError(t, SetUserRoles(11, []string{"auditor"}))
	require.NoError(t, SetUserRoles(1, []string{"auditor"}))

	allCanvas := []string{
		CanvasTemplateRead.Code(), CanvasTemplateWrite.Code(), CanvasTemplatePublish.Code(), CanvasUserRead.Code(),
		CanvasOrderRead.Code(), CanvasSettingWrite.Code(), CanvasPromptWrite.Code(), CanvasAssetWrite.Code(),
	}
	cases := []struct {
		name        string
		userID      int
		role        int
		newapi      []string
		canvas      []string
		userWrite   bool
		channelRead bool
	}{
		{"root keeps everything despite a custom role", 1, common.RoleRootUser, nil, allCanvas, true, true},
		{"admin without custom role keeps the admin baseline", 2, common.RoleAdminUser, nil, allCanvas, true, true},
		{"admin with custom role is restricted to it", 10, common.RoleAdminUser, []string{LogRead.Code()}, []string{CanvasOrderRead.Code()}, false, false},
		{"common user with custom role gets only its codes", 11, common.RoleCommonUser, []string{LogRead.Code()}, []string{CanvasOrderRead.Code()}, false, false},
		{"common user without role has nothing", 12, common.RoleCommonUser, []string{}, []string{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.newapi != nil {
				assert.Equal(t, tc.newapi, CodesForApp(tc.userID, tc.role, ""))
			}
			assert.Equal(t, tc.canvas, CodesForApp(tc.userID, tc.role, AppCanvas))
			assert.Equal(t, tc.userWrite, Can(tc.userID, tc.role, UserWrite))
			assert.Equal(t, tc.channelRead, Can(tc.userID, tc.role, ChannelRead))
		})
	}
	assert.Equal(t, []string{"auditor"}, UserRoles(10))

	// Assignments are stored as g rows and survive a restart.
	var assignments int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("ptype = ? AND v1 = ?", "g", RoleSubject("auditor")).Count(&assignments).Error)
	assert.Equal(t, int64(3), assignments)
	require.NoError(t, Init(db))
	assert.Equal(t, []string{LogRead.Code()}, CodesForApp(10, common.RoleAdminUser, ""))

	// Another node writes straight to the database; the policy sync reload picks it up.
	require.NoError(t, db.Create(&model.CasbinRule{Ptype: "g", V0: UserSubject(13), V1: RoleSubject("auditor")}).Error)
	assert.True(t, Can(13, common.RoleAdminUser, UserWrite))
	require.NoError(t, ReloadPolicy())
	assert.False(t, Can(13, common.RoleAdminUser, UserWrite))
	assert.True(t, Can(13, common.RoleAdminUser, LogRead))

	assert.ErrorIs(t, SetUserRoles(10, []string{"missing"}), ErrRoleNotFound)
	assert.ErrorIs(t, SetUserRoles(10, []string{BuiltInRoleAdmin}), ErrRoleInvalid)
	assert.ErrorIs(t, SetRolePermissions("auditor", []string{"log.delete"}), ErrPermissionUnknown)
	assert.ErrorIs(t, SetRolePermissions(BuiltInRoleAdmin, []string{LogRead.Code()}), ErrRoleBuiltIn)
	assert.ErrorIs(t, CreateRole("auditor", "Again", ""), ErrRoleExists)
	assert.ErrorIs(t, CreateRole("Bad Key", "Bad", ""), ErrRoleInvalid)
	assert.Equal(t, []string{"auditor"}, UserRoles(10), "a failed assignment must not change the stored one")

	// Removing the role returns the administrator to the admin baseline.
	require.NoError(t, DeleteRole("auditor"))
	assert.Empty(t, UserRoles(10))
	assert.True(t, Can(10, common.RoleAdminUser, UserWrite))
	assert.Empty(t, CodesForApp(11, common.RoleCommonUser, AppCanvas))
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("v0 = ? OR v1 = ?", RoleSubject("auditor"), RoleSubject("auditor")).Count(&assignments).Error)
	assert.Equal(t, int64(0), assignments)
}
