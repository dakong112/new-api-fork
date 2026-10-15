package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func initRouterAuthz(t *testing.T) {
	t.Helper()
	wasMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = wasMaster })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CasbinRule{}, &model.AuthzRole{}))
	require.NoError(t, authz.Init(db))
}

func TestGetTaskPluginOptionsAdminForbiddenRootAllowed(t *testing.T) {
	initRouterAuthz(t)

	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name       string
		id         int
		role       int
		wantStatus int
	}{
		{name: "admin", id: 2, role: common.RoleAdminUser, wantStatus: http.StatusForbidden},
		{name: "root", id: 1, role: common.RoleRootUser, wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/task_plugin_options", nil)
			context.Set("id", testCase.id)
			context.Set("role", testCase.role)
			middleware.RequirePermission(authz.TaskPluginBind)(context)
			if !context.IsAborted() {
				controller.GetTaskPluginOptions(context)
			}
			assert.Equal(t, testCase.wantStatus, recorder.Code)
		})
	}
}

// Through the real router: authorization management is root-only, and an
// administrator holding a custom role reaches only the admin routes it grants.
func TestCustomRolesThroughAdminRoutes(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedis, previousSecret := common.RedisEnabled, common.SessionSecret
	previousMaster := common.IsMasterNode
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled, common.SessionSecret = previousRedis, previousSecret
		common.IsMasterNode = previousMaster
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.AuditLog{}, &model.UserAccessToken{}, &model.Option{}, &model.CasbinRule{}, &model.AuthzRole{}))
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.SessionSecret, common.IsMasterNode = false, "router-authz-test-secret", true
	require.NoError(t, model.EnsureLegacyAccessTokenRetireAt(time.Now().Unix()))
	require.NoError(t, authz.Init(db))
	require.NoError(t, authz.CreateRole("group_viewer", "Group viewer", ""))
	require.NoError(t, authz.SetRolePermissions("group_viewer", []string{authz.GroupRead.Code()}))

	bearer := map[string]string{}
	for _, account := range []struct {
		name string
		role int
	}{{"root", common.RoleRootUser}, {"admin", common.RoleAdminUser}, {"viewer", common.RoleAdminUser}} {
		user := &model.User{Username: account.name, Password: "placeholder", Role: account.role, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "aff-" + account.name}
		require.NoError(t, db.Create(user).Error)
		now := time.Now().Unix()
		require.NoError(t, model.CreateUserSession(&model.UserSession{SID: "sid-" + account.name, UserID: user.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: "refresh-" + account.name, LoginMethod: "password", LastActiveAt: now, ExpiresAt: now + 3600}))
		jwt, _, err := service.IssueAccessToken(service.AuthIdentity{UserID: user.Id, SessionID: "sid-" + account.name, UserAuthVersion: 1, SessionVersion: 1})
		require.NoError(t, err)
		bearer[account.name] = jwt
		if account.name == "viewer" {
			require.NoError(t, authz.SetUserRoles(user.Id, []string{"group_viewer"}))
		}
	}

	engine := newAccessTokenScopeTestEngine(t)
	for _, tc := range []struct {
		account, method, path string
		wantStatus            int
	}{
		{"root", http.MethodGet, "/api/authz/roles", http.StatusOK},
		{"root", http.MethodGet, "/api/authz/catalog?app=canvas", http.StatusOK},
		{"admin", http.MethodGet, "/api/authz/roles", http.StatusForbidden},
		{"admin", http.MethodGet, "/api/authz/catalog", http.StatusForbidden},
		{"admin", http.MethodGet, "/api/user/1/roles", http.StatusForbidden},
		{"admin", http.MethodGet, "/api/group/", http.StatusOK},
		{"viewer", http.MethodGet, "/api/group/", http.StatusOK},
		{"viewer", http.MethodGet, "/api/redemption/", http.StatusForbidden},
		{"viewer", http.MethodGet, "/api/log/stat", http.StatusForbidden},
	} {
		t.Run(tc.account+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("Authorization", "Bearer "+bearer[tc.account])
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
		})
	}
}
