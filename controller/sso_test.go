package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSSOClientsOptionValidation(t *testing.T) {
	valid := `[{"client_id":"infinite","name":"Studio","secret_hash":"` + setting.SSOSecretHash("s3cret") + `","redirect_uris":["https://www.example.com/sso/callback","http://localhost:3000/sso/callback"],"launch_url":"https://www.example.com/api/auth/newapi/start"}]`
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"valid", valid, true},
		{"empty list", `[]`, true},
		{"bad client id", `[{"client_id":"Bad Id","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"]}]`, false},
		{"client id too long", `[{"client_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"]}]`, false},
		{"duplicate client id", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"]},{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("y") + `","redirect_uris":["https://a.com/cb"]}]`, false},
		{"secret hash not hex64", `[{"client_id":"a","secret_hash":"abc","redirect_uris":["https://a.com/cb"]}]`, false},
		{"plain http redirect", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["http://a.com/cb"]}]`, false},
		{"redirect with fragment", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb#x"]}]`, false},
		{"no redirect", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":[]}]`, false},
		{"defaults", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"],"defaults":{"group":"vip","image":"gpt-image-1","text":"deepseek-chat"}}]`, true},
		{"default model too long", `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"],"defaults":{"image":"` + strings.Repeat("m", 129) + `"}}]`, false},
		{"app", `[{"client_id":"a","app":"canvas","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"]}]`, true},
		{"app not lowercase letters", `[{"client_id":"a","app":"Canvas-1","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"]}]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := setting.ParseSSOClients(tc.value)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}

	clients, err := setting.ParseSSOClients(valid)
	require.NoError(t, err)
	client := clients[0]
	assert.True(t, client.AllowsRedirect("https://www.example.com/sso/callback"))
	assert.False(t, client.AllowsRedirect("https://www.example.com/sso/callback/"))
	assert.False(t, client.AllowsRedirect("https://evil.com/sso/callback"))
	assert.True(t, client.SecretMatches("s3cret"))
	assert.False(t, client.SecretMatches("S3cret"))
	assert.False(t, client.SecretMatches(""))

	plainHTTP := `[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["http://203.0.113.5:23580/sso/callback"]}]`
	t.Setenv("SSO_ALLOW_INSECURE_REDIRECTS", "true")
	_, err = setting.ParseSSOClients(plainHTTP)
	assert.NoError(t, err, "test deployments may opt in to http callbacks")
	t.Setenv("SSO_ALLOW_INSECURE_REDIRECTS", "")
	_, err = setting.ParseSSOClients(plainHTTP)
	assert.Error(t, err, "http callbacks stay rejected unless explicitly enabled")

	withDefaults, err := setting.ParseSSOClients(`[{"client_id":"a","secret_hash":"` + setting.SSOSecretHash("x") + `","redirect_uris":["https://a.com/cb"],"defaults":{"group":"vip","image":"gpt-image-1"}}]`)
	require.NoError(t, err)
	assert.Equal(t, setting.SSOClientDefaults{Group: "vip", Image: "gpt-image-1"}, withDefaults[0].Defaults)
}

const ssoTestRedirect = "https://www.example.com/sso/callback"
const ssoTestSecret = "s3cret"

func setupSSOTest(t *testing.T) (*gin.Engine, *model.User, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousMaster := common.RedisEnabled, common.IsMasterNode
	previousClients := setting.SSOClients2JsonString()
	ssoExchangeFailures = newSSOExchangeLimiter()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, i18n.Init())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Log{}, &model.AuditLog{}, &model.CasbinRule{}, &model.AuthzRole{},
		&model.UserAccessToken{}, &model.Option{}, &model.AuthFlow{}, &model.Token{}, &model.TwoFA{}, &model.TwoFABackupCode{}, &model.PasskeyCredential{}, &model.UserOAuthBinding{}, &model.SSOConsumeRecord{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.IsMasterNode = true
	require.NoError(t, authz.Init(db))
	require.NoError(t, setting.UpdateSSOClientsByJsonString(`[{"client_id":"infinite","name":"Studio","app":"canvas","secret_hash":"`+setting.SSOSecretHash(ssoTestSecret)+`","redirect_uris":["`+ssoTestRedirect+`"],"launch_url":"https://www.example.com/api/auth/newapi/start"},{"client_id":"other","name":"Other","secret_hash":"`+setting.SSOSecretHash("other-secret")+`","redirect_uris":["https://other.example.com/cb"]}]`))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.IsMasterNode = previousRedis, previousMaster
		require.NoError(t, setting.UpdateSSOClientsByJsonString(previousClients))
	})

	user := &model.User{Username: "alice", Password: "placeholder", DisplayName: "Alice", Email: "a@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 1000, AuthVersion: 1, AffCode: "alice"}
	require.NoError(t, db.Create(user).Error)
	_, jwt := createAccessTokenTestSession(t, user.Id, "sso-test-session")

	router := gin.New()
	api := router.Group("/api")
	api.POST("/sso/code", middleware.UserAuth(), CreateSSOCode)
	api.POST("/sso/exchange", ExchangeSSOCode)
	api.GET("/sso/apps", middleware.UserAuth(), ListSSOApps)
	api.POST("/sso/ticket", middleware.UserAuth(), CreateSSOTicket)
	api.POST("/sso/ticket/preview", middleware.SameSiteJSONOnly(), PreviewSSOTicket)
	api.POST("/sso/ticket/redeem", middleware.SameSiteJSONOnly(), RedeemSSOTicket)
	api.POST("/sso/key", middleware.UserAuth(), CreateSSOKey)
	api.GET("/sso/client", GetSSOClient)
	api.POST("/sso/consume", middleware.UserAuth(), ConsumeSSOQuota)
	api.POST("/sso/permissions", middleware.UserAuth(), GetSSOPermissions)
	return router, user, jwt
}

func ssoPost(router http.Handler, path, bearer, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	request.RemoteAddr = "192.0.2.8:4567"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type ssoCodeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Code        string `json:"code"`
		RedirectURL string `json:"redirect_url"`
	} `json:"data"`
}

func issueSSOTestCode(t *testing.T, router http.Handler, jwt string) string {
	t.Helper()
	response := ssoPost(router, "/api/sso/code", jwt, `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`","state":"st-1"}`)
	var body ssoCodeResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, body.Message)
	return body.Data.Code
}

func TestCreateSSOCode(t *testing.T) {
	router, _, jwt := setupSSOTest(t)

	response := ssoPost(router, "/api/sso/code", jwt, `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`","state":"st-1"}`)
	var body ssoCodeResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, body.Message)
	require.NotEmpty(t, body.Data.Code)
	target, err := url.Parse(body.Data.RedirectURL)
	require.NoError(t, err)
	assert.Equal(t, "www.example.com", target.Host)
	assert.Equal(t, "/sso/callback", target.Path)
	assert.Equal(t, body.Data.Code, target.Query().Get("code"))
	assert.Equal(t, "st-1", target.Query().Get("state"))

	rejected := []string{
		`{"client_id":"unknown","redirect_uri":"` + ssoTestRedirect + `"}`,
		`{"client_id":"infinite","redirect_uri":"https://evil.com/sso/callback"}`,
		`{"client_id":"infinite","redirect_uri":"` + ssoTestRedirect + `/"}`,
	}
	for _, payload := range rejected {
		response := ssoPost(router, "/api/sso/code", jwt, payload)
		var body ssoCodeResponse
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
		assert.False(t, body.Success, payload)
	}

	unauthenticated := ssoPost(router, "/api/sso/code", "", `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`"}`)
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
}

type ssoExchangeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		User struct {
			Id       int    `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Role     int    `json:"role"`
			Group    string `json:"group"`
			Quota    int    `json:"quota"`
		} `json:"user"`
		AccessToken          string   `json:"access_token"`
		Permissions          []string `json:"permissions"`
		AccessTokenExpiresAt int64    `json:"access_token_expires_at"`
		APIKey               string   `json:"api_key"`
	} `json:"data"`
}

func exchangeBody(code, secret, redirect string) string {
	return `{"client_id":"infinite","client_secret":"` + secret + `","code":"` + code + `","redirect_uri":"` + redirect + `"}`
}

func TestExchangeSSOCode(t *testing.T) {
	router, user, jwt := setupSSOTest(t)

	code := issueSSOTestCode(t, router, jwt)
	response := ssoPost(router, "/api/sso/exchange", "", exchangeBody(code, ssoTestSecret, ssoTestRedirect))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body ssoExchangeResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, body.Message)
	assert.Equal(t, user.Id, body.Data.User.Id)
	assert.Equal(t, "alice", body.Data.User.Username)
	assert.Equal(t, "a@example.com", body.Data.User.Email)
	assert.Equal(t, 1000, body.Data.User.Quota)
	assert.True(t, strings.HasPrefix(body.Data.AccessToken, model.AccessTokenPrefix))
	assert.Greater(t, body.Data.AccessTokenExpiresAt, time.Now().Unix())
	assert.True(t, strings.HasPrefix(body.Data.APIKey, "sk-"))

	// The issued PAT can read the profile.
	tokens, err := model.ListUserAccessTokens(user.Id)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, "sso:infinite", tokens[0].Name)
	assert.ElementsMatch(t, []string{"profile:read", "wallet:read", "wallet:write"}, tokens[0].GetScopes())

	// Replay of the same code is rejected.
	replay := ssoPost(router, "/api/sso/exchange", "", exchangeBody(code, ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusBadRequest, replay.Code)

	// A second login reuses the API key and rotates the PAT.
	second := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	var secondBody ssoExchangeResponse
	require.NoError(t, common.Unmarshal(second.Body.Bytes(), &secondBody))
	require.True(t, secondBody.Success, secondBody.Message)
	assert.Equal(t, body.Data.APIKey, secondBody.Data.APIKey)
	assert.NotEqual(t, body.Data.AccessToken, secondBody.Data.AccessToken)
	tokens, err = model.ListUserAccessTokens(user.Id)
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
	var keyCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ? AND name = ?", user.Id, "sso:infinite").Count(&keyCount).Error)
	assert.Equal(t, int64(1), keyCount)
}

func TestExchangeSSOCodeRejections(t *testing.T) {
	router, user, jwt := setupSSOTest(t)

	wrongSecret := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), "wrong", ssoTestRedirect))
	assert.Equal(t, http.StatusUnauthorized, wrongSecret.Code)

	wrongRedirect := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, "https://www.example.com/other"))
	assert.Equal(t, http.StatusBadRequest, wrongRedirect.Code)

	garbage := ssoPost(router, "/api/sso/exchange", "", exchangeBody("not-a-code", ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusBadRequest, garbage.Code)

	expiredCode := issueSSOTestCode(t, router, jwt)
	require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", model.AuthFlowPurposeSSOCode).Update("expires_at", time.Now().Add(-time.Second)).Error)
	expired := ssoPost(router, "/api/sso/exchange", "", exchangeBody(expiredCode, ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusBadRequest, expired.Code)

	bannedCode := issueSSOTestCode(t, router, jwt)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
	banned := ssoPost(router, "/api/sso/exchange", "", exchangeBody(bannedCode, ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusForbidden, banned.Code)
}

func TestListSSOApps(t *testing.T) {
	router, _, jwt := setupSSOTest(t)
	request := httptest.NewRequest(http.MethodGet, "/api/sso/apps", nil)
	request.Header.Set("Authorization", "Bearer "+jwt)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Contains(t, response.Body.String(), `"launch_url":"https://www.example.com/api/auth/newapi/start"`)
	assert.NotContains(t, response.Body.String(), "secret_hash")
}

func TestSSOCodeRejectsAccessToken(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	raw, _ := createScopedAccessToken(t, user.Id, 0, "profile:read")

	response := ssoPost(router, "/api/sso/code", raw, `{"client_id":"infinite","redirect_uri":"`+ssoTestRedirect+`","state":"st-1"}`)
	assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
}

func TestExchangeSSOCodeBinding(t *testing.T) {
	router, user, jwt := setupSSOTest(t)

	// A code issued for one application cannot be redeemed by another.
	crossClient := `{"client_id":"other","client_secret":"other-secret","code":"` + issueSSOTestCode(t, router, jwt) + `","redirect_uri":"` + ssoTestRedirect + `"}`
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/exchange", "", crossClient).Code)

	// Revoking the browser session cancels codes it issued.
	code := issueSSOTestCode(t, router, jwt)
	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Update("status", model.UserSessionStatusRevoked).Error)
	revoked := ssoPost(router, "/api/sso/exchange", "", exchangeBody(code, ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusBadRequest, revoked.Code)
}

func TestExchangeSSOCodeCredentialConflicts(t *testing.T) {
	router, user, jwt := setupSSOTest(t)

	first := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	// A disabled application key is not handed out again.
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ? AND name = ?", user.Id, "sso:infinite").Update("status", common.TokenStatusDisabled).Error)
	disabled := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusConflict, disabled.Code)

	// An expired key is refused as well.
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", user.Id).Updates(map[string]any{"status": common.TokenStatusEnabled, "expired_time": time.Now().Unix() - 10}).Error)
	expired := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusConflict, expired.Code)
}

func TestExchangeSSOCodeAccessTokenLimit(t *testing.T) {
	router, user, jwt := setupSSOTest(t)
	for range service.AccessTokenMaxPerUser {
		createScopedAccessToken(t, user.Id, 0, "profile:read")
	}

	full := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusConflict, full.Code)

	// The failed grant is rolled back as a whole, API key included.
	var keys int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&keys).Error)
	assert.Zero(t, keys)
}

func TestExchangeSSOCodeThrottlesWrongSecrets(t *testing.T) {
	router, _, jwt := setupSSOTest(t)

	for range ssoExchangeMaxFailures {
		wrong := ssoPost(router, "/api/sso/exchange", "", exchangeBody("x", "wrong", ssoTestRedirect))
		require.Equal(t, http.StatusUnauthorized, wrong.Code)
	}
	code := issueSSOTestCode(t, router, jwt)
	blocked := ssoPost(router, "/api/sso/exchange", "", exchangeBody(code, ssoTestSecret, ssoTestRedirect))
	assert.Equal(t, http.StatusTooManyRequests, blocked.Code)

	// Another caller is not affected by the failures of the first one.
	request := httptest.NewRequest(http.MethodPost, "/api/sso/exchange", strings.NewReader(exchangeBody(code, ssoTestSecret, ssoTestRedirect)))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.99:4567"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

func ssoTicketBody(client, secret string) string {
	return `{"client_id":"` + client + `","client_secret":"` + secret + `"}`
}

// issueSSOTestPAT creates an application-scoped PAT like the SSO grant does.
func issueSSOTestPAT(t *testing.T, userID int, name string) (string, *model.UserAccessToken) {
	t.Helper()
	raw, token := createScopedAccessToken(t, userID, 0, "profile:read")
	require.NoError(t, model.DB.Model(token).Update("name", name).Error)
	return raw, token
}

func issueSSOTestTicket(t *testing.T, router http.Handler, pat string) string {
	t.Helper()
	response := ssoPost(router, "/api/sso/ticket", pat, ssoTicketBody("infinite", ssoTestSecret))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Data struct {
			Ticket    string `json:"ticket"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data.Ticket)
	assert.InDelta(t, time.Now().Add(ssoTicketTTL).Unix(), body.Data.ExpiresAt, 5)
	return body.Data.Ticket
}

func ssoTicketRequest(ticket string) string {
	return `{"ticket":"` + ticket + `"}`
}

func TestCreateSSOTicketRejections(t *testing.T) {
	router, user, jwt := setupSSOTest(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	otherPAT, _ := issueSSOTestPAT(t, user.Id, "sso:other")
	legacyPAT, _ := createScopedAccessToken(t, user.Id, 0, "profile:read")

	cases := []struct {
		name   string
		bearer string
		body   string
		status int
	}{
		{"browser session", jwt, ssoTicketBody("infinite", ssoTestSecret), http.StatusForbidden},
		{"pat of another application", otherPAT, ssoTicketBody("infinite", ssoTestSecret), http.StatusForbidden},
		{"unnamed pat", legacyPAT, ssoTicketBody("infinite", ssoTestSecret), http.StatusForbidden},
		{"wrong secret", pat, ssoTicketBody("infinite", "wrong"), http.StatusUnauthorized},
		{"unknown client", pat, ssoTicketBody("nope", ssoTestSecret), http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := ssoPost(router, "/api/sso/ticket", tc.bearer, tc.body)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), `"ticket"`)
		})
	}
	var flows int64
	require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", model.AuthFlowPurposeSSOTicket).Count(&flows).Error)
	assert.Zero(t, flows)
}

func TestSSOTicketPreviewDoesNotConsume(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	ticket := issueSSOTestTicket(t, router, pat)

	for range 2 {
		response := ssoPost(router, "/api/sso/ticket/preview", "", ssoTicketRequest(ticket))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), `"username":"alice"`)
		assert.Contains(t, response.Body.String(), `"client_name":"Studio"`)
	}
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/preview", "", ssoTicketRequest("unknown")).Code)
}

func TestRedeemSSOTicket(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	ticket := issueSSOTestTicket(t, router, pat)

	response := ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(ticket))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"success":true`)
	assert.NotContains(t, response.Body.String(), "challenge")
	var method string
	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ? AND sid <> ?", user.Id, "sso-test-session").Select("login_method").Scan(&method).Error)
	assert.Equal(t, "sso_ticket", method)

	// A ticket works once, and the audit trail never holds it.
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(ticket)).Code)
	var audits []model.AuditLog
	require.NoError(t, model.DB.Where("action LIKE ?", "sso.ticket%").Find(&audits).Error)
	require.NotEmpty(t, audits)
	for _, audit := range audits {
		raw, err := common.Marshal(audit)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), ticket)
		assert.NotContains(t, string(raw), pat)
	}
}

func TestRedeemSSOTicketInvalidation(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, token := issueSSOTestPAT(t, user.Id, "sso:infinite")

	expired := issueSSOTestTicket(t, router, pat)
	require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", model.AuthFlowPurposeSSOTicket).Update("expires_at", time.Now().Add(-time.Second)).Error)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(expired)).Code)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/preview", "", ssoTicketRequest(expired)).Code)

	changed := issueSSOTestTicket(t, router, pat)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("auth_version", 2).Error)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(changed)).Code)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("auth_version", 1).Error)

	revoked := issueSSOTestTicket(t, router, pat)
	require.NoError(t, model.DB.Delete(token).Error)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(revoked)).Code)

	var sessions int64
	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessions).Error)
	assert.EqualValues(t, 1, sessions) // only the fixture session
}

func TestRedeemSSOTicketRequiresSecondFactor(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	require.NoError(t, model.DB.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	ticket := issueSSOTestTicket(t, router, pat)

	response := ssoPost(router, "/api/sso/ticket/redeem", "", ssoTicketRequest(ticket))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "flow_token")
	assert.NotContains(t, response.Body.String(), "access_token")
	var sessions int64
	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessions).Error)
	assert.EqualValues(t, 1, sessions) // no session before the challenge is answered
}

func TestSSOTicketRoutesRejectLoginCSRF(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	ticket := issueSSOTestTicket(t, router, pat)

	send := func(contentType string, headers map[string]string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/sso/ticket/redeem", strings.NewReader(ssoTicketRequest(ticket)))
		request.Header.Set("Content-Type", contentType)
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		request.RemoteAddr = "192.0.2.8:4567"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	cases := []struct {
		name        string
		contentType string
		headers     map[string]string
		status      int
	}{
		{"form encoded body", "application/x-www-form-urlencoded", nil, http.StatusUnsupportedMediaType},
		{"text/plain body", "text/plain", nil, http.StatusUnsupportedMediaType},
		{"cross-site fetch", "application/json", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"same-site sibling", "application/json", map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"foreign Origin", "application/json", map[string]string{"Origin": "https://evil.example.com"}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.status, send(tc.contentType, tc.headers).Code)
		})
	}
	// None of the rejected requests spent the ticket.
	response := send("application/json; charset=utf-8", map[string]string{"Origin": "http://example.com", "Sec-Fetch-Site": "same-origin"})
	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"success":true`)
}

func TestCreateSSOTicketRefusesAdmins(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("role", common.RoleAdminUser).Error)

	response := ssoPost(router, "/api/sso/ticket", pat, ssoTicketBody("infinite", ssoTestSecret))
	assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	var audit model.AuditLog
	require.NoError(t, model.DB.Where("action = ?", "sso.ticket_failed").First(&audit).Error)
	assert.Contains(t, audit.Content, "role_not_allowed")
}

func TestPreviewSSOTicketRejectsWhatRedeemWouldReject(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat, token := issueSSOTestPAT(t, user.Id, "sso:infinite")

	changed := issueSSOTestTicket(t, router, pat)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("auth_version", 2).Error)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/preview", "", ssoTicketRequest(changed)).Code)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("auth_version", 1).Error)

	revoked := issueSSOTestTicket(t, router, pat)
	require.NoError(t, model.DB.Delete(token).Error)
	assert.Equal(t, http.StatusBadRequest, ssoPost(router, "/api/sso/ticket/preview", "", ssoTicketRequest(revoked)).Code)
}

func ssoKeyBody(client, secret, group string) string {
	return `{"client_id":"` + client + `","client_secret":"` + secret + `","group":"` + group + `"}`
}

// setupSSOKeyGroups pins the group settings TokenAuth checks: legacy is usable
// but has no ratio, svip is not usable at all.
func setupSSOKeyGroups(t *testing.T) {
	t.Helper()
	previousUsable, previousRatio := setting.UserUsableGroups2JSONString(), ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default group","vip":"VIP group","legacy":"Legacy group"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0.65}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatio))
	})
}

func requestSSOTestKey(t *testing.T, router http.Handler, pat, group string) (int, string) {
	t.Helper()
	response := ssoPost(router, "/api/sso/key", pat, ssoKeyBody("infinite", ssoTestSecret, group))
	var body struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	return response.Code, body.Data.Key
}

func TestCreateSSOKey(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	setupSSOKeyGroups(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")

	status, key := requestSSOTestKey(t, router, pat, "vip")
	require.Equal(t, http.StatusOK, status)
	assert.True(t, strings.HasPrefix(key, "sk-"))
	var stored model.Token
	require.NoError(t, model.DB.Where("user_id = ? AND name = ?", user.Id, "sso:infinite@vip").First(&stored).Error)
	assert.Equal(t, "vip", stored.Group)
	assert.True(t, stored.UnlimitedQuota)
	assert.Equal(t, int64(-1), stored.ExpiredTime)

	// The same group twice returns the same key; an empty group means the user's own group.
	_, again := requestSSOTestKey(t, router, pat, "vip")
	assert.Equal(t, key, again)
	status, own := requestSSOTestKey(t, router, pat, "")
	require.Equal(t, http.StatusOK, status)
	assert.NotEqual(t, key, own)
	var ownStored model.Token
	require.NoError(t, model.DB.Where("user_id = ? AND name = ?", user.Id, "sso:infinite@default").First(&ownStored).Error)
	assert.Equal(t, "default", ownStored.Group)
	var keys int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&keys).Error)
	assert.Equal(t, int64(2), keys)

	// Admins may fetch keys: a key only spends their own quota and opens no session.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("role", common.RoleAdminUser).Error)
	status, adminKey := requestSSOTestKey(t, router, pat, "vip")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, key, adminKey)

	// A key whose group was edited in the console is refused rather than handed out for the wrong group.
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", stored.Id).Update("group", "default").Error)
	status, _ = requestSSOTestKey(t, router, pat, "vip")
	assert.Equal(t, http.StatusConflict, status)

	var audit model.AuditLog
	require.NoError(t, model.DB.Where("action = ?", "sso.key_issue").First(&audit).Error)
	assert.Contains(t, audit.Content, "vip")
	assert.NotContains(t, audit.Content, strings.TrimPrefix(key, "sk-"))
}

func TestCreateSSOKeyRejections(t *testing.T) {
	router, user, jwt := setupSSOTest(t)
	setupSSOKeyGroups(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	otherPAT, _ := issueSSOTestPAT(t, user.Id, "sso:other")

	cases := []struct {
		name   string
		bearer string
		body   string
		status int
	}{
		{"browser session", jwt, ssoKeyBody("infinite", ssoTestSecret, "vip"), http.StatusForbidden},
		{"pat of another application", otherPAT, ssoKeyBody("infinite", ssoTestSecret, "vip"), http.StatusForbidden},
		{"wrong secret", pat, ssoKeyBody("infinite", "wrong", "vip"), http.StatusUnauthorized},
		{"unknown client", pat, ssoKeyBody("nope", ssoTestSecret, "vip"), http.StatusUnauthorized},
		{"group not usable", pat, ssoKeyBody("infinite", ssoTestSecret, "svip"), http.StatusBadRequest},
		{"usable group without ratio", pat, ssoKeyBody("infinite", ssoTestSecret, "legacy"), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := ssoPost(router, "/api/sso/key", tc.bearer, tc.body)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), "sk-")
		})
	}
	var keys int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&keys).Error)
	assert.Zero(t, keys)
}

func TestCreateSSOKeyCredentialConflicts(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	setupSSOKeyGroups(t)
	pat, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	previousMax := operation_setting.GetTokenSetting().MaxUserTokens
	operation_setting.GetTokenSetting().MaxUserTokens = 1
	t.Cleanup(func() { operation_setting.GetTokenSetting().MaxUserTokens = previousMax })

	status, _ := requestSSOTestKey(t, router, pat, "vip")
	require.Equal(t, http.StatusOK, status)
	status, _ = requestSSOTestKey(t, router, pat, "default")
	assert.Equal(t, http.StatusConflict, status, "the limit counts every key of the user")

	// A disabled group key is not handed out again and is not replaced.
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ? AND name = ?", user.Id, "sso:infinite@vip").Update("status", common.TokenStatusDisabled).Error)
	status, key := requestSSOTestKey(t, router, pat, "vip")
	assert.Equal(t, http.StatusConflict, status)
	assert.Empty(t, key)
}

func TestGetSSOClient(t *testing.T) {
	router, _, _ := setupSSOTest(t)
	require.NoError(t, setting.UpdateSSOClientsByJsonString(`[{"client_id":"infinite","name":"Studio","secret_hash":"`+setting.SSOSecretHash(ssoTestSecret)+`","redirect_uris":["`+ssoTestRedirect+`"],"defaults":{"group":"vip","image":"gpt-image-1","text":"deepseek-chat"}}]`))
	get := func(clientID, secret string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/sso/client", nil)
		request.SetBasicAuth(clientID, secret)
		request.RemoteAddr = "192.0.2.8:4567"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	ok := get("infinite", ssoTestSecret)
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	var body struct {
		Data struct {
			Name     string                    `json:"name"`
			Defaults setting.SSOClientDefaults `json:"defaults"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(ok.Body.Bytes(), &body))
	assert.Equal(t, "Studio", body.Data.Name)
	assert.Equal(t, setting.SSOClientDefaults{Group: "vip", Image: "gpt-image-1", Text: "deepseek-chat"}, body.Data.Defaults)
	assert.NotContains(t, ok.Body.String(), "secret_hash")
	assert.NotContains(t, ok.Body.String(), "redirect_uris")

	assert.Equal(t, http.StatusUnauthorized, get("infinite", "wrong").Code)
	assert.Equal(t, http.StatusUnauthorized, get("nope", ssoTestSecret).Code)
	var audit model.AuditLog
	require.NoError(t, model.DB.Where("action = ?", "sso.client_failed").First(&audit).Error)
	assert.Contains(t, audit.Content, "invalid_client")
}

func ssoConsumeBody(client, secret, key string, quota int, title string) string {
	return `{"client_id":"` + client + `","client_secret":"` + secret + `","idempotency_key":"` + key + `","quota":` + strconv.Itoa(quota) + `,"title":"` + title + `"}`
}

// issueSSOWalletPAT returns a token like the one the SSO grant issues, which
// carries wallet:write for charges.
func issueSSOWalletPAT(t *testing.T, userID int, name string) string {
	t.Helper()
	raw, token := createScopedAccessToken(t, userID, 0, "profile:read", "wallet:write")
	require.NoError(t, model.DB.Model(token).Update("name", name).Error)
	return raw
}

type ssoConsumeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Quota   int `json:"quota"`
		Balance int `json:"balance"`
	} `json:"data"`
}

func consumeSSOTestQuota(t *testing.T, router http.Handler, pat, key string, quota int) (int, ssoConsumeResponse) {
	t.Helper()
	response := ssoPost(router, "/api/sso/consume", pat, ssoConsumeBody("infinite", ssoTestSecret, key, quota, "Poster"))
	var body ssoConsumeResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body), response.Body.String())
	return response.Code, body
}

func TestConsumeSSOQuota(t *testing.T) {
	router, user, _ := setupSSOTest(t)
	pat := issueSSOWalletPAT(t, user.Id, "sso:infinite")
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // every connection to :memory: is a separate database
	userState := func() model.User {
		var stored model.User
		require.NoError(t, model.DB.First(&stored, user.Id).Error)
		return stored
	}
	consumeLogs := func() []model.Log {
		var logs []model.Log
		require.NoError(t, model.DB.Where("user_id = ? AND type = ?", user.Id, model.LogTypeConsume).Order("id").Find(&logs).Error)
		return logs
	}

	status, body := consumeSSOTestQuota(t, router, pat, "job-1", 300)
	require.Equal(t, http.StatusOK, status, body.Message)
	assert.Equal(t, 300, body.Data.Quota)
	assert.Equal(t, 700, body.Data.Balance)
	assert.Equal(t, 700, userState().Quota)
	assert.Equal(t, 300, userState().UsedQuota)
	logs := consumeLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, "infinite:Poster", logs[0].ModelName)
	assert.Equal(t, 300, logs[0].Quota)
	assert.Contains(t, logs[0].Content, "infinite")

	// A retry returns the first result, even with another amount, and charges nothing.
	status, body = consumeSSOTestQuota(t, router, pat, "job-1", 500)
	require.Equal(t, http.StatusOK, status, body.Message)
	assert.Equal(t, 300, body.Data.Quota)
	assert.Equal(t, 700, body.Data.Balance)
	assert.Equal(t, 700, userState().Quota)
	assert.Len(t, consumeLogs(), 1)

	// More than the balance is refused whole.
	status, body = consumeSSOTestQuota(t, router, pat, "job-2", 701)
	assert.Equal(t, http.StatusPaymentRequired, status)
	assert.Equal(t, "insufficient quota", body.Message)
	assert.Equal(t, 700, userState().Quota)

	// Concurrent retries of one key charge once.
	var wg sync.WaitGroup
	statuses := make([]int, 5)
	charged := make([]int, 5)
	for i := range 5 {
		wg.Go(func() {
			response := ssoPost(router, "/api/sso/consume", pat, ssoConsumeBody("infinite", ssoTestSecret, "job-3", 100, "Poster"))
			var body ssoConsumeResponse
			_ = common.Unmarshal(response.Body.Bytes(), &body)
			statuses[i], charged[i] = response.Code, body.Data.Quota
		})
	}
	wg.Wait()
	assert.Equal(t, []int{200, 200, 200, 200, 200}, statuses)
	assert.Equal(t, []int{100, 100, 100, 100, 100}, charged)
	assert.Equal(t, 600, userState().Quota)
	assert.Equal(t, 400, userState().UsedQuota)
	assert.Len(t, consumeLogs(), 2)
	var records int64
	require.NoError(t, model.DB.Model(&model.SSOConsumeRecord{}).Where("user_id = ?", user.Id).Count(&records).Error)
	assert.Equal(t, int64(2), records)
}

func TestConsumeSSOQuotaRejections(t *testing.T) {
	router, user, jwt := setupSSOTest(t)
	pat := issueSSOWalletPAT(t, user.Id, "sso:infinite")
	otherPAT := issueSSOWalletPAT(t, user.Id, "sso:other")
	plainPAT, _ := createScopedAccessToken(t, user.Id, 0, "profile:read", "wallet:write")
	readOnlyPAT, _ := issueSSOTestPAT(t, user.Id, "sso:infinite")
	maxQuota := 10 * int(common.QuotaPerUnit)
	// Enough balance that only the request checks can refuse.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 2*maxQuota).Error)

	cases := []struct {
		name   string
		bearer string
		body   string
		status int
	}{
		{"browser session", jwt, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, "t"), http.StatusForbidden},
		{"access token not issued by sso", plainPAT, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, "t"), http.StatusForbidden},
		{"pat of another application", otherPAT, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, "t"), http.StatusForbidden},
		{"pat without wallet scope", readOnlyPAT, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, "t"), http.StatusForbidden},
		{"wrong secret", pat, ssoConsumeBody("infinite", "wrong", "k", 10, "t"), http.StatusUnauthorized},
		{"unknown client", pat, ssoConsumeBody("nope", ssoTestSecret, "k", 10, "t"), http.StatusUnauthorized},
		{"over the per-charge limit", pat, ssoConsumeBody("infinite", ssoTestSecret, "k", maxQuota+1, "t"), http.StatusBadRequest},
		{"zero quota", pat, ssoConsumeBody("infinite", ssoTestSecret, "k", 0, "t"), http.StatusBadRequest},
		{"negative quota", pat, ssoConsumeBody("infinite", ssoTestSecret, "k", -10, "t"), http.StatusBadRequest},
		{"empty key", pat, ssoConsumeBody("infinite", ssoTestSecret, "", 10, "t"), http.StatusBadRequest},
		{"key too long", pat, ssoConsumeBody("infinite", ssoTestSecret, strings.Repeat("k", 65), 10, "t"), http.StatusBadRequest},
		{"empty title", pat, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, " "), http.StatusBadRequest},
		{"title too long", pat, ssoConsumeBody("infinite", ssoTestSecret, "k", 10, strings.Repeat("标", 101)), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := ssoPost(router, "/api/sso/consume", tc.bearer, tc.body)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
	var stored model.User
	require.NoError(t, model.DB.First(&stored, user.Id).Error)
	assert.Equal(t, 2*maxQuota, stored.Quota)
	var records int64
	require.NoError(t, model.DB.Model(&model.SSOConsumeRecord{}).Count(&records).Error)
	assert.Zero(t, records)

	// The limit itself is allowed.
	status, body := consumeSSOTestQuota(t, router, pat, "max", maxQuota)
	assert.Equal(t, http.StatusOK, status, body.Message)
}

func TestSSOClientReturnURL(t *testing.T) {
	previousClients := setting.SSOClients2JsonString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateSSOClientsByJsonString(previousClients)) })
	require.NoError(t, setting.UpdateSSOClientsByJsonString(`[{"client_id":"infinite","name":"Studio","secret_hash":"`+setting.SSOSecretHash(ssoTestSecret)+`","redirect_uris":["https://studio.example.com/api/auth/newapi/callback"],"launch_url":"https://launch.example.com:8443/start"}]`))
	const fallback = "https://gateway.example.com/usage-logs"

	for requested, expected := range map[string]string{
		"https://studio.example.com/wallet?paid=1":           "https://studio.example.com/wallet?paid=1",
		"https://STUDIO.example.com/wallet":                  "https://STUDIO.example.com/wallet",
		"https://launch.example.com:8443/wallet":             "https://launch.example.com:8443/wallet",
		"":                                                   fallback,
		"https://launch.example.com/wallet":                  fallback,
		"http://studio.example.com/wallet":                   fallback,
		"https://studio.example.com.evil.com/x":              fallback,
		"https://evil.com/?next=https://studio.example.com/": fallback,
		"https://studio.example.com@evil.com/":               fallback,
		"//studio.example.com/wallet":                        fallback,
		"/wallet":                                            fallback,
		"javascript:alert(1)//studio.example.com":            fallback,
		"data:text/html,https://studio.example.com":          fallback,
		"https://studio.example.com/%zz":                     fallback,
	} {
		assert.Equal(t, expected, ssoClientReturnURL(requested, fallback), requested)
	}
}

// The grant and /api/sso/permissions carry the codes of the client's
// application only, and follow role changes without a new sign-in.
func TestSSOPermissionCodes(t *testing.T) {
	router, user, jwt := setupSSOTest(t)

	response := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	var grant ssoExchangeResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &grant))
	require.True(t, grant.Success, grant.Message)
	require.NotNil(t, grant.Data.Permissions)
	assert.Empty(t, grant.Data.Permissions, "a common user without roles holds no codes")

	permissions := func(pat, client, secret string) (int, []string) {
		response := ssoPost(router, "/api/sso/permissions", pat, `{"client_id":"`+client+`","client_secret":"`+secret+`"}`)
		var body struct {
			Data struct {
				Permissions []string `json:"permissions"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
		return response.Code, body.Data.Permissions
	}

	// The preset support role also grants user.read, a gateway code the canvas app never sees.
	require.NoError(t, authz.SetUserRoles(user.Id, []string{"support"}))
	status, codes := permissions(grant.Data.AccessToken, "infinite", ssoTestSecret)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []string{authz.CanvasUserRead.Code(), authz.CanvasOrderRead.Code()}, codes)

	second := ssoPost(router, "/api/sso/exchange", "", exchangeBody(issueSSOTestCode(t, router, jwt), ssoTestSecret, ssoTestRedirect))
	require.NoError(t, common.Unmarshal(second.Body.Bytes(), &grant))
	assert.Equal(t, []string{authz.CanvasUserRead.Code(), authz.CanvasOrderRead.Code()}, grant.Data.Permissions)

	// An administrator without a custom role holds every canvas code.
	require.NoError(t, authz.SetUserRoles(user.Id, nil))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("role", common.RoleAdminUser).Error)
	_, codes = permissions(grant.Data.AccessToken, "infinite", ssoTestSecret)
	assert.Len(t, codes, 8)

	// A client naming no application receives none.
	otherPAT, _ := issueSSOTestPAT(t, user.Id, "sso:other")
	status, codes = permissions(otherPAT, "other", "other-secret")
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, codes)

	// Same authentication as /api/sso/key: client secret plus the PAT issued to that client.
	status, _ = permissions(grant.Data.AccessToken, "infinite", "wrong-secret")
	assert.Equal(t, http.StatusUnauthorized, status)
	status, _ = permissions(otherPAT, "infinite", ssoTestSecret)
	assert.Equal(t, http.StatusForbidden, status)
	status, _ = permissions(jwt, "infinite", ssoTestSecret)
	assert.Equal(t, http.StatusForbidden, status)
}
