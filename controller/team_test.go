package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teamTestUser struct {
	user *model.User
	jwt  string
}

// setupTeamTest extends the SSO fixture: alice (quota 1000) owns no team
// yet; bob and carol are plain users with their own sessions.
func setupTeamTest(t *testing.T) (*gin.Engine, teamTestUser, teamTestUser, teamTestUser) {
	t.Helper()
	initModelListColumnNames(t)
	router, alice, aliceJWT := setupSSOTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Team{}, &model.TeamMember{}, &model.Redemption{}, &model.TopUp{}))
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // every connection to :memory: is a separate database

	users := make([]teamTestUser, 0, 2)
	for _, name := range []string{"bob", "carol"} {
		user := &model.User{Username: name, Password: "placeholder", Email: name + "@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 500, AuthVersion: 1, AffCode: name}
		require.NoError(t, model.DB.Create(user).Error)
		_, jwt := createAccessTokenTestSession(t, user.Id, name+"-session")
		users = append(users, teamTestUser{user, jwt})
	}

	api := router.Group("/api")
	team := api.Group("/team", middleware.UserAuth())
	team.POST("", CreateTeam)
	team.GET("/self", GetSelfTeams)
	team.GET("/:id", GetTeam)
	team.DELETE("/:id", DissolveTeam)
	team.POST("/:id/members", AddTeamMember)
	team.PUT("/:id/members/:uid", UpdateTeamMember)
	team.DELETE("/:id/members/:uid", DeleteTeamMember)
	team.POST("/:id/transfer", TransferToTeam)
	team.POST("/:id/redeem", RedeemToTeam)
	team.GET("/:id/usage", GetTeamUsage)
	api.POST("/user/pay", middleware.UserAuth(), RequestEpay)
	api.POST("/sso/teams", middleware.UserAuth(), ListSSOTeams)
	api.POST("/sso/team-key", middleware.UserAuth(), CreateSSOTeamKey)
	return router, teamTestUser{alice, aliceJWT}, users[0], users[1]
}

type teamResponse struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	MessageKey string          `json:"message_key"`
	Data       json.RawMessage `json:"data"`
}

func teamCall(t *testing.T, router http.Handler, method, path, jwt, body string) teamResponse {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+jwt)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var parsed teamResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &parsed), response.Body.String())
	return parsed
}

func createTestTeam(t *testing.T, router http.Handler, owner teamTestUser) model.Team {
	t.Helper()
	response := teamCall(t, router, http.MethodPost, "/api/team", owner.jwt, `{"name":"Studio"}`)
	require.True(t, response.Success, response.Message)
	var team model.Team
	require.NoError(t, common.Unmarshal(response.Data, &team))
	return team
}

func teamPath(team model.Team, suffix string) string {
	return "/api/team/" + strconv.Itoa(team.Id) + suffix
}

func teamMemberToken(t *testing.T, team model.Team, userId int) model.Token {
	t.Helper()
	var member model.TeamMember
	require.NoError(t, model.DB.Where("team_id = ? AND user_id = ?", team.Id, userId).First(&member).Error)
	var token model.Token
	require.NoError(t, model.DB.First(&token, member.TokenId).Error)
	return token
}

func userQuota(t *testing.T, userId int) int {
	t.Helper()
	quota, err := model.GetUserQuota(userId, true)
	require.NoError(t, err)
	return quota
}

func TestCreateTeamIsTransactional(t *testing.T) {
	router, alice, _, _ := setupTeamTest(t)

	team := createTestTeam(t, router, alice)
	var wallet model.User
	require.NoError(t, model.DB.First(&wallet, team.WalletUserId).Error)
	assert.True(t, strings.HasPrefix(wallet.Username, "team_"))
	assert.LessOrEqual(t, len(wallet.Username), 20)
	assert.Equal(t, common.RoleCommonUser, wallet.Role)
	assert.Equal(t, common.UserStatusEnabled, wallet.Status)
	assert.Equal(t, alice.user.Group, wallet.Group)
	assert.Zero(t, wallet.Quota)
	assert.Empty(t, wallet.Email)
	assert.NotEmpty(t, wallet.Password)
	ownerKey := teamMemberToken(t, team, alice.user.Id)
	assert.Equal(t, team.WalletUserId, ownerKey.UserId)
	assert.True(t, ownerKey.UnlimitedQuota)

	for _, body := range []string{`{"name":""}`, `{"name":"` + strings.Repeat("n", 65) + `"}`, `{"name":"x","limit_quota":-1}`} {
		assert.False(t, teamCall(t, router, http.MethodPost, "/api/team", alice.jwt, body).Success, body)
	}

	// A failure after the wallet account was created rolls everything back.
	var usersBefore, teamsBefore int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&usersBefore).Error)
	require.NoError(t, model.DB.Model(&model.Team{}).Count(&teamsBefore).Error)
	require.NoError(t, model.DB.Migrator().DropTable(&model.TeamMember{}))
	assert.False(t, teamCall(t, router, http.MethodPost, "/api/team", alice.jwt, `{"name":"Broken"}`).Success)
	require.NoError(t, model.DB.AutoMigrate(&model.TeamMember{}))
	var usersAfter, teamsAfter int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&usersAfter).Error)
	require.NoError(t, model.DB.Model(&model.Team{}).Count(&teamsAfter).Error)
	assert.Equal(t, usersBefore, usersAfter)
	assert.Equal(t, teamsBefore, teamsAfter)
}

func TestTeamMemberManagement(t *testing.T) {
	router, alice, bob, carol := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	members := teamPath(team, "/members")

	added := teamCall(t, router, http.MethodPost, members, alice.jwt, `{"username":"bob","limit_quota":100}`)
	require.True(t, added.Success, added.Message)
	assert.Equal(t, "User is already a team member", teamCall(t, router, http.MethodPost, members, alice.jwt, `{"email":"bob@example.com"}`).MessageKey)
	walletAsMember := teamCall(t, router, http.MethodPost, members, alice.jwt, `{"username":"`+mustWalletUsername(t, team)+`"}`)
	assert.Equal(t, "This user cannot join a team", walletAsMember.MessageKey)

	// A plain member cannot manage and sees only themselves; strangers see nothing.
	assert.Equal(t, "Only the team owner or an admin can do this", teamCall(t, router, http.MethodPost, members, bob.jwt, `{"username":"carol"}`).MessageKey)
	detail := teamCall(t, router, http.MethodGet, teamPath(team, ""), bob.jwt, "")
	require.True(t, detail.Success, detail.Message)
	var view struct {
		Role    string                 `json:"role"`
		Members []model.TeamMemberView `json:"members"`
	}
	require.NoError(t, common.Unmarshal(detail.Data, &view))
	assert.Equal(t, model.TeamRoleMember, view.Role)
	require.Len(t, view.Members, 1)
	assert.Equal(t, 100, view.Members[0].RemainQuota)
	assert.Equal(t, "Team not found", teamCall(t, router, http.MethodGet, teamPath(team, ""), carol.jwt, "").MessageKey)

	// Promoted to admin, bob manages members but never the owner or admins.
	bobPath := members + "/" + strconv.Itoa(bob.user.Id)
	alicePath := members + "/" + strconv.Itoa(alice.user.Id)
	require.True(t, teamCall(t, router, http.MethodPut, bobPath, alice.jwt, `{"role":"admin"}`).Success)
	require.True(t, teamCall(t, router, http.MethodPost, members, bob.jwt, `{"email":"carol@example.com"}`).Success)
	assert.Equal(t, "Only the team owner can manage team admins", teamCall(t, router, http.MethodPut, alicePath, bob.jwt, `{"limit_quota":1}`).MessageKey)
	assert.Equal(t, "Only the team owner can manage team admins", teamCall(t, router, http.MethodDelete, alicePath, bob.jwt, "").MessageKey)
	carolPath := members + "/" + strconv.Itoa(carol.user.Id)
	assert.Equal(t, "Only the team owner can manage team admins", teamCall(t, router, http.MethodPut, carolPath, bob.jwt, `{"role":"admin"}`).MessageKey)
	assert.Equal(t, "The team owner cannot be changed or removed", teamCall(t, router, http.MethodDelete, alicePath, alice.jwt, "").MessageKey)
	assert.Equal(t, "The team owner cannot be changed or removed", teamCall(t, router, http.MethodPut, alicePath, alice.jwt, `{"role":"member"}`).MessageKey)

	// Removing a member deletes their key.
	carolKey := teamMemberToken(t, team, carol.user.Id)
	require.True(t, teamCall(t, router, http.MethodDelete, carolPath, bob.jwt, "").Success)
	var deleted model.Token
	require.NoError(t, model.DB.Unscoped().First(&deleted, carolKey.Id).Error)
	assert.True(t, deleted.DeletedAt.Valid)
	assert.Equal(t, "Team not found", teamCall(t, router, http.MethodGet, teamPath(team, ""), carol.jwt, "").MessageKey)
}

func mustWalletUsername(t *testing.T, team model.Team) string {
	t.Helper()
	username, err := model.GetUsernameById(team.WalletUserId, true)
	require.NoError(t, err)
	return username
}

// The member key caps spending with remain_quota while the relay debits the
// wallet account, using the same reserve/settle functions as the relay.
func TestTeamMemberLimitCapsKeyAndDebitsWallet(t *testing.T) {
	router, alice, bob, _ := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob","limit_quota":100}`).Success)
	require.NoError(t, model.TransferQuotaToTeam(alice.user.Id, team.Id, 500))
	key := teamMemberToken(t, team, bob.user.Id)
	assert.Equal(t, 100, key.RemainQuota)
	assert.False(t, key.UnlimitedQuota)

	reserved, err := model.TryReserveTokenQuota(key.Id, key.Key, 150, key.UnlimitedQuota)
	require.NoError(t, err)
	assert.False(t, reserved, "the member limit refuses spending above it")
	reserved, err = model.TryReserveTokenQuota(key.Id, key.Key, 80, key.UnlimitedQuota)
	require.NoError(t, err)
	require.True(t, reserved)
	require.NoError(t, model.DecreaseUserQuota(team.WalletUserId, 80, true))
	key = teamMemberToken(t, team, bob.user.Id)
	assert.Equal(t, 20, key.RemainQuota)
	assert.Equal(t, 80, key.UsedQuota)
	assert.Equal(t, 420, userQuota(t, team.WalletUserId))
	assert.Equal(t, 500, userQuota(t, bob.user.Id), "the member's personal wallet is untouched")

	bobPath := teamPath(team, "/members/"+strconv.Itoa(bob.user.Id))
	cases := []struct {
		limit     int
		remain    int
		unlimited bool
	}{
		{50, 0, false},    // below what was used: exhausted, never negative
		{200, 120, false}, // keeps the 80 already used
		{0, 0, true},      // unlimited
	}
	for _, tc := range cases {
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", key.Id).Update("status", common.TokenStatusExhausted).Error)
		updated := teamCall(t, router, http.MethodPut, bobPath, alice.jwt, `{"limit_quota":`+strconv.Itoa(tc.limit)+`}`)
		require.True(t, updated.Success, updated.Message)
		key = teamMemberToken(t, team, bob.user.Id)
		assert.Equal(t, tc.remain, key.RemainQuota, tc.limit)
		assert.Equal(t, tc.unlimited, key.UnlimitedQuota, tc.limit)
		assert.Equal(t, common.TokenStatusEnabled, key.Status, "a raised limit revives an exhausted key")
	}
}

func TestTransferQuotaToTeamNeverOverdraws(t *testing.T) {
	router, alice, _, carol := setupTeamTest(t)
	team := createTestTeam(t, router, alice)

	var wg sync.WaitGroup
	results := make([]error, 5)
	for i := range 5 {
		wg.Go(func() { results[i] = model.TransferQuotaToTeam(alice.user.Id, team.Id, 300) })
	}
	wg.Wait()
	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
		} else {
			assert.ErrorIs(t, err, model.ErrTeamInsufficientQuota)
		}
	}
	assert.Equal(t, 3, succeeded)
	assert.Equal(t, 100, userQuota(t, alice.user.Id))
	assert.Equal(t, 900, userQuota(t, team.WalletUserId))

	var sent, received int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND type = ?", alice.user.Id, model.LogTypeSystem).Count(&sent).Error)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND type = ?", team.WalletUserId, model.LogTypeTopup).Count(&received).Error)
	assert.Equal(t, int64(3), sent)
	assert.Equal(t, int64(3), received)

	transfer := teamPath(team, "/transfer")
	for _, body := range []string{`{"quota":0}`, `{"quota":-5}`, `{"quota":9007199254740992}`} {
		assert.Equal(t, "Invalid parameters", teamCall(t, router, http.MethodPost, transfer, alice.jwt, body).MessageKey, body)
	}
	assert.Equal(t, "Insufficient balance", teamCall(t, router, http.MethodPost, transfer, alice.jwt, `{"quota":101}`).MessageKey)
	assert.Equal(t, "Team not found", teamCall(t, router, http.MethodPost, transfer, carol.jwt, `{"quota":1}`).MessageKey)
	require.True(t, teamCall(t, router, http.MethodPost, transfer, alice.jwt, `{"quota":100}`).Success)
	assert.Zero(t, userQuota(t, alice.user.Id))
	assert.Equal(t, 1000, userQuota(t, team.WalletUserId))
}

func TestRedeemToTeamCreditsWallet(t *testing.T) {
	router, alice, bob, _ := setupTeamTest(t)
	payment := operation_setting.GetPaymentSetting()
	previous := *payment
	payment.ComplianceConfirmed, payment.ComplianceTermsVersion = true, operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() { *payment = previous })
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob"}`).Success)
	code := &model.Redemption{Key: strings.Repeat("r", 32), Status: common.RedemptionCodeStatusEnabled, Name: "gift", Quota: 400}
	require.NoError(t, model.DB.Create(code).Error)

	response := teamCall(t, router, http.MethodPost, teamPath(team, "/redeem"), bob.jwt, `{"key":"`+code.Key+`"}`)
	require.True(t, response.Success, response.Message)
	assert.Equal(t, 400, userQuota(t, team.WalletUserId))
	assert.Equal(t, 500, userQuota(t, bob.user.Id))
	require.NoError(t, model.DB.First(code, code.Id).Error)
	assert.Equal(t, team.WalletUserId, code.UsedUserId)
	var audit model.AuditLog
	require.NoError(t, model.LOG_DB.Where("action = ?", "team.redeem").First(&audit).Error)
	assert.Equal(t, bob.user.Id, audit.UserId, "the audit names the member who redeemed")

	// Test codes are once per user; crediting one to a team wallet would let
	// every team hand its members another full test credit.
	testCode := &model.Redemption{Key: strings.Repeat("t", 32), Status: common.RedemptionCodeStatusEnabled, Name: "trial", Quota: 300, IsTest: true}
	require.NoError(t, model.DB.Create(testCode).Error)
	assert.False(t, teamCall(t, router, http.MethodPost, teamPath(team, "/redeem"), bob.jwt, `{"key":"`+testCode.Key+`"}`).Success)
	assert.Equal(t, 400, userQuota(t, team.WalletUserId))
	require.NoError(t, model.DB.First(testCode, testCode.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, testCode.Status, "the refused code stays usable")
	quota, err := model.Redeem(testCode.Key, bob.user.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, quota, "the member still gets the full test credit personally")
}

// Relay only checks the wallet account, so a member's own account status must
// reach their team key.
func TestTeamMemberKeyFollowsMemberAccount(t *testing.T) {
	router, alice, bob, _ := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob"}`).Success)
	key := teamMemberToken(t, team, bob.user.Id)

	for _, step := range []struct{ userStatus, keyStatus int }{
		{common.UserStatusDisabled, common.TokenStatusDisabled},
		{common.UserStatusEnabled, common.TokenStatusEnabled},
	} {
		bob.user.Status = step.userStatus
		require.NoError(t, bob.user.Update(false))
		assert.Equal(t, step.keyStatus, teamMemberToken(t, team, bob.user.Id).Status, step.userStatus)
	}
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", key.Id).Update("status", common.TokenStatusExhausted).Error)
	bob.user.Status = common.UserStatusDisabled
	require.NoError(t, bob.user.Update(false))
	bob.user.Status = common.UserStatusEnabled
	require.NoError(t, bob.user.Update(false))
	assert.Equal(t, common.TokenStatusExhausted, teamMemberToken(t, team, bob.user.Id).Status, "an exhausted key is not revived")

	_, err := bob.user.Delete()
	require.NoError(t, err)
	var memberships int64
	require.NoError(t, model.DB.Model(&model.TeamMember{}).Where("user_id = ?", bob.user.Id).Count(&memberships).Error)
	assert.Zero(t, memberships)
	var deleted model.Token
	require.NoError(t, model.DB.Unscoped().First(&deleted, key.Id).Error)
	assert.True(t, deleted.DeletedAt.Valid)

	_, err = alice.user.Delete()
	assert.ErrorIs(t, err, model.ErrTeamOwnerImmutable, "a team owner cannot be deleted")
	_, err = model.GetUserById(alice.user.Id, false)
	assert.NoError(t, err)
}

func TestTeamUsageByMemberKey(t *testing.T) {
	router, alice, bob, _ := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob"}`).Success)
	aliceKey, bobKey := teamMemberToken(t, team, alice.user.Id), teamMemberToken(t, team, bob.user.Id)
	logs := []model.Log{
		{UserId: team.WalletUserId, Type: model.LogTypeConsume, TokenId: aliceKey.Id, Quota: 10, CreatedAt: 100},
		{UserId: team.WalletUserId, Type: model.LogTypeConsume, TokenId: aliceKey.Id, Quota: 20, CreatedAt: 200},
		{UserId: team.WalletUserId, Type: model.LogTypeConsume, TokenId: bobKey.Id, Quota: 5, CreatedAt: 300},
		{UserId: team.WalletUserId, Type: model.LogTypeTopup, TokenId: 0, Quota: 999, CreatedAt: 300},
		{UserId: bob.user.Id, Type: model.LogTypeConsume, TokenId: bobKey.Id, Quota: 777, CreatedAt: 300},
	}
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	response := teamCall(t, router, http.MethodGet, teamPath(team, "/usage"), alice.jwt, "")
	require.True(t, response.Success, response.Message)
	var rows []struct {
		TokenId  int    `json:"token_id"`
		UserId   int    `json:"user_id"`
		Username string `json:"username"`
		Quota    int64  `json:"quota"`
		Requests int64  `json:"requests"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &rows))
	require.Len(t, rows, 2)
	assert.Equal(t, aliceKey.Id, rows[0].TokenId)
	assert.Equal(t, "alice", rows[0].Username)
	assert.Equal(t, int64(30), rows[0].Quota)
	assert.Equal(t, int64(2), rows[0].Requests)
	assert.Equal(t, "bob", rows[1].Username)
	assert.Equal(t, int64(5), rows[1].Quota)

	ranged := teamCall(t, router, http.MethodGet, teamPath(team, "/usage?start_timestamp=150&end_timestamp=250"), alice.jwt, "")
	require.NoError(t, common.Unmarshal(ranged.Data, &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, int64(20), rows[0].Quota)

	assert.Equal(t, "Only the team owner or an admin can do this", teamCall(t, router, http.MethodGet, teamPath(team, "/usage"), bob.jwt, "").MessageKey)
}

func TestTeamWalletAccountCannotSignIn(t *testing.T) {
	router, alice, _, _ := setupTeamTest(t)
	team := createTestTeam(t, router, alice)

	_, err := service.CreateLoginSession(team.WalletUserId, "password", "192.0.2.8", "test")
	assert.ErrorIs(t, err, service.ErrLoginSessionInvalid)
	_, err = service.CreateLoginSession(alice.user.Id, "password", "192.0.2.8", "test")
	assert.NoError(t, err)

	// Even with a session somehow in hand, no SSO code is issued.
	_, walletJWT := createAccessTokenTestSession(t, team.WalletUserId, "wallet-session")
	response := ssoPost(router, "/api/sso/code", walletJWT, `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`"}`)
	assert.Contains(t, response.Body.String(), "Team wallet accounts cannot sign in")

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", team.WalletUserId).Update("email", "wallet@example.com").Error)
	assert.Error(t, model.ResetUserPasswordByEmail("wallet@example.com", "new-password-123"))

	// The wallet lookup fails closed: without it nobody signs in.
	_, aliceJWT := createAccessTokenTestSession(t, alice.user.Id, "alice-before-outage")
	require.NoError(t, model.DB.Migrator().DropTable(&model.Team{}))
	t.Cleanup(func() { _ = model.DB.AutoMigrate(&model.Team{}) })
	_, err = service.CreateLoginSession(alice.user.Id, "password", "192.0.2.8", "test")
	assert.ErrorIs(t, err, service.ErrLoginSessionInvalid)
	response = ssoPost(router, "/api/sso/code", aliceJWT, `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`"}`)
	assert.Contains(t, response.Body.String(), "Team wallet accounts cannot sign in")
	assert.ErrorIs(t, model.ResetUserPasswordByEmail(alice.user.Email, "new-password-123"), model.ErrEmailNotFound)
}

func TestSSOTeamKeyOnlyForMembers(t *testing.T) {
	router, alice, bob, _ := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	alicePAT, _ := issueSSOTestPAT(t, alice.user.Id, "sso:infinite")
	bobPAT, _ := issueSSOTestPAT(t, bob.user.Id, "sso:infinite")
	body := `{"client_id":"infinite","client_secret":"` + ssoTestSecret + `","team_id":` + strconv.Itoa(team.Id) + `}`

	response := ssoPost(router, "/api/sso/team-key", alicePAT, body)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"key":"sk-`+teamMemberToken(t, team, alice.user.Id).Key+`"`)

	assert.Equal(t, http.StatusNotFound, ssoPost(router, "/api/sso/team-key", bobPAT, body).Code)
	assert.Equal(t, http.StatusUnauthorized, ssoPost(router, "/api/sso/team-key", alicePAT, strings.Replace(body, ssoTestSecret, "wrong", 1)).Code)
	assert.Equal(t, http.StatusForbidden, ssoPost(router, "/api/sso/team-key", alice.jwt, body).Code, "browser sessions are not application credentials")

	teams := ssoPost(router, "/api/sso/teams", alicePAT, `{"client_id":"infinite","client_secret":"`+ssoTestSecret+`"}`)
	require.Equal(t, http.StatusOK, teams.Code, teams.Body.String())
	assert.Contains(t, teams.Body.String(), `"role":"owner"`)
	assert.Contains(t, teams.Body.String(), `"name":"Studio"`)
	bobTeams := ssoPost(router, "/api/sso/teams", bobPAT, `{"client_id":"infinite","client_secret":"`+ssoTestSecret+`"}`)
	assert.JSONEq(t, `[]`, string(mustData(t, bobTeams)))
}

func mustData(t *testing.T, response *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var parsed teamResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &parsed))
	return parsed.Data
}

func TestRequestEpayForTeamRequiresManager(t *testing.T) {
	router, alice, bob, carol := setupTeamTest(t)
	previousAddress, previousID, previousKey := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
	operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = "https://pay.example.com", "1001", "epay-key"
	t.Cleanup(func() {
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = previousAddress, previousID, previousKey
	})
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob"}`).Success)
	body := `{"amount":10,"payment_method":"alipay","team_id":` + strconv.Itoa(team.Id) + `}`
	pay := func(jwt string) map[string]any {
		request := httptest.NewRequest(http.MethodPost, "/api/user/pay", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+jwt)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &parsed), response.Body.String())
		return parsed
	}

	assert.Equal(t, "Team not found", pay(carol.jwt)["message_key"])
	assert.Equal(t, "Only the team owner or an admin can do this", pay(bob.jwt)["message_key"])
	var orders int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&orders).Error)
	assert.Zero(t, orders)

	ok := pay(alice.jwt)
	require.Equal(t, "success", ok["message"], ok)
	var order model.TopUp
	require.NoError(t, model.DB.First(&order).Error)
	assert.Equal(t, team.WalletUserId, order.UserId)
	_, err := model.RechargeEpay(order.TradeNo, "alipay", "192.0.2.8")
	require.NoError(t, err)
	assert.Positive(t, userQuota(t, team.WalletUserId), "the callback credits the order's user, the team wallet")
	assert.Equal(t, 1000, userQuota(t, alice.user.Id))
}

func TestDissolveTeamReturnsBalanceToOwner(t *testing.T) {
	router, alice, bob, carol := setupTeamTest(t)
	team := createTestTeam(t, router, alice)
	require.True(t, teamCall(t, router, http.MethodPost, teamPath(team, "/members"), alice.jwt, `{"username":"bob"}`).Success)
	require.NoError(t, model.TransferQuotaToTeam(alice.user.Id, team.Id, 600))
	require.NoError(t, model.DecreaseUserQuota(team.WalletUserId, 50, true))
	keys := []model.Token{teamMemberToken(t, team, alice.user.Id), teamMemberToken(t, team, bob.user.Id)}
	aliceIdentity, aliceJWT := createAccessTokenTestSession(t, alice.user.Id, "alice-dissolve")
	proofFor := func(teamId int) string {
		context, err := common.Marshal(service.TeamContext{TeamID: teamId})
		require.NoError(t, err)
		return issueSecurityEnrollmentProof(t, aliceIdentity, service.VerificationOperation{Scope: service.VerificationScopeTeamDissolve, Context: context}, "password")
	}
	dissolve := func(jwt, proof string) teamResponse {
		var parsed teamResponse
		response := accessTokenRequest(router, http.MethodDelete, teamPath(team, ""), jwt, proof, "")
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &parsed), response.Body.String())
		return parsed
	}

	assert.Equal(t, "Only the team owner can dissolve the team", dissolve(bob.jwt, "").MessageKey)
	assert.Equal(t, "Team not found", dissolve(carol.jwt, "").MessageKey)
	assert.False(t, dissolve(aliceJWT, "").Success, "a security proof is required")
	assert.False(t, dissolve(aliceJWT, proofFor(team.Id+1)).Success, "the proof is bound to the team")
	assert.Equal(t, 550, userQuota(t, team.WalletUserId))

	response := dissolve(aliceJWT, proofFor(team.Id))
	require.True(t, response.Success, response.Message)
	assert.JSONEq(t, `{"quota":550}`, string(response.Data))
	assert.Equal(t, 950, userQuota(t, alice.user.Id), "the owner gets back exactly the remaining balance")
	assert.Zero(t, userQuota(t, team.WalletUserId))
	var wallet model.User
	require.NoError(t, model.DB.First(&wallet, team.WalletUserId).Error)
	assert.Equal(t, common.UserStatusDisabled, wallet.Status, "the wallet account is kept, disabled")
	require.NoError(t, model.DB.First(&team, team.Id).Error)
	assert.Equal(t, model.TeamStatusDissolved, team.Status)
	var memberships int64
	require.NoError(t, model.DB.Model(&model.TeamMember{}).Where("team_id = ?", team.Id).Count(&memberships).Error)
	assert.Zero(t, memberships)
	for _, key := range keys {
		_, err := model.ValidateUserToken(key.Key)
		assert.ErrorIs(t, err, model.ErrTokenInvalid, "member keys stop working")
	}
	var returned int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND content LIKE ?", alice.user.Id, "%dissolved team%").Count(&returned).Error)
	assert.Equal(t, int64(1), returned)
	assert.Equal(t, "Team not found", dissolve(aliceJWT, proofFor(team.Id)).MessageKey, "a team is dissolved once")

	_, err := alice.user.Delete()
	assert.NoError(t, err, "an owner without active teams can be deleted")
}

func TestEpayOrderPaidAfterDissolveCreditsOwner(t *testing.T) {
	router, alice, _, _ := setupTeamTest(t)
	previousAddress, previousID, previousKey := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
	operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = "https://pay.example.com", "1001", "epay-key"
	t.Cleanup(func() {
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = previousAddress, previousID, previousKey
	})
	team := createTestTeam(t, router, alice)
	body := `{"amount":10,"payment_method":"alipay","team_id":` + strconv.Itoa(team.Id) + `}`
	for range 2 {
		response := teamCall(t, router, http.MethodPost, "/api/user/pay", alice.jwt, body)
		require.Equal(t, "success", response.Message)
	}
	var orders []model.TopUp
	require.NoError(t, model.DB.Order("id").Find(&orders).Error)
	require.Len(t, orders, 2)
	_, err := model.DissolveTeam(team.Id, alice.user.Id)
	require.NoError(t, err)

	assert.Equal(t, "Team not found", teamCall(t, router, http.MethodPost, "/api/user/pay", alice.jwt, body).MessageKey, "no new orders for a dissolved team")
	alreadyDone, err := model.RechargeEpay(orders[0].TradeNo, "alipay", "192.0.2.8")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	orderQuota := int(10 * common.QuotaPerUnit)
	assert.Equal(t, 1000+orderQuota, userQuota(t, alice.user.Id), "the late payment goes to the owner, amount unchanged")
	assert.Zero(t, userQuota(t, team.WalletUserId))
	alreadyDone, err = model.RechargeEpay(orders[0].TradeNo, "alipay", "192.0.2.8")
	require.NoError(t, err)
	assert.True(t, alreadyDone, "a repeated callback credits nothing")
	require.NoError(t, model.ManualCompleteTopUp(orders[1].TradeNo, "192.0.2.8"))
	assert.Equal(t, 1000+2*orderQuota, userQuota(t, alice.user.Id), "an admin-completed order follows the same rule")
	assert.Zero(t, userQuota(t, team.WalletUserId))
	var order model.TopUp
	require.NoError(t, model.DB.First(&order, orders[0].Id).Error)
	assert.Equal(t, team.WalletUserId, order.UserId, "the order history keeps the wallet as payer")
}
