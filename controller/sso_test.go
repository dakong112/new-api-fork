package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting"
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
		&model.UserAccessToken{}, &model.Option{}, &model.AuthFlow{}, &model.Token{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.IsMasterNode = true
	require.NoError(t, authz.Init(db))
	require.NoError(t, setting.UpdateSSOClientsByJsonString(`[{"client_id":"infinite","name":"Studio","secret_hash":"`+setting.SSOSecretHash(ssoTestSecret)+`","redirect_uris":["`+ssoTestRedirect+`"],"launch_url":"https://www.example.com/api/auth/newapi/start"},{"client_id":"other","name":"Other","secret_hash":"`+setting.SSOSecretHash("other-secret")+`","redirect_uris":["https://other.example.com/cb"]}]`))
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
		AccessToken          string `json:"access_token"`
		AccessTokenExpiresAt int64  `json:"access_token_expires_at"`
		APIKey               string `json:"api_key"`
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
