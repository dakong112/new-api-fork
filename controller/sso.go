package controller

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const (
	ssoExchangeMaxFailures   = 10
	ssoExchangeFailureWindow = 60 // seconds
)

// ponytail: per-process counter; with several nodes each allows its own
// budget. Move to a Redis counter if multi-node brute force matters.
var ssoExchangeFailures common.InMemoryRateLimiter

// CreateSSOCode issues a single-use login code for a registered application.
// The caller must hold a browser session; personal access tokens are rejected
// by the access token route rules.
func CreateSSOCode(c *gin.Context) {
	var request struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		State       string `json:"state"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	client, ok := setting.FindSSOClient(request.ClientID)
	if !ok || !client.AllowsRedirect(request.RedirectURI) {
		common.ApiErrorT(c, "Invalid SSO client or redirect URI")
		return
	}
	target, err := url.Parse(request.RedirectURI)
	if err != nil || len(request.State) > 256 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	code, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeSSOCode,
		Provider:  client.ClientID,
		UserId:    c.GetInt("id"),
		SessionId: c.GetString("session_id"),
		Payload:   request.RedirectURI,
		ExpiresAt: time.Now().Add(service.SSOCodeTTL),
	})
	if err != nil {
		common.SysLog(common.LogText("SSO code creation failed for user %d client %s: %v", c.GetInt("id"), client.ClientID, err))
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	query := target.Query()
	query.Set("code", code)
	if request.State != "" {
		query.Set("state", request.State)
	}
	target.RawQuery = query.Encode()
	common.ApiSuccess(c, gin.H{"code": code, "redirect_url": target.String()})
}

// ListSSOApps lists the applications a signed-in user can launch.
func ListSSOApps(c *gin.Context) {
	clients := setting.GetSSOClients()
	apps := make([]gin.H, 0, len(clients))
	for _, client := range clients {
		if client.LaunchURL == "" {
			continue
		}
		apps = append(apps, gin.H{"client_id": client.ClientID, "name": client.Name, "launch_url": client.LaunchURL})
	}
	common.ApiSuccess(c, apps)
}

// ExchangeSSOCode redeems a login code for an application server. It is
// anonymous at the dashboard level; the client secret authenticates the caller.
// Secret failures are throttled per client_id rather than per IP, because every
// exchange of an application comes from the same server address.
func ExchangeSSOCode(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, ok := setting.FindSSOClient(request.ClientID)
	if !ok {
		recordSSOExchangeFailure(c, 0, request.ClientID, "invalid_client", http.StatusUnauthorized)
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return
	}
	ssoExchangeFailures.Init(common.RateLimitKeyExpirationDuration)
	attempt := ssoExchangeFailures.Reserve("sso-exchange-fail:"+client.ClientID, ssoExchangeMaxFailures, ssoExchangeFailureWindow)
	if attempt == nil {
		recordSSOExchangeFailure(c, 0, client.ClientID, "throttled", http.StatusTooManyRequests)
		common.ApiErrorStatus(c, http.StatusTooManyRequests, errors.New("too many failed attempts"))
		return
	}
	secretOK := client.SecretMatches(request.ClientSecret)
	attempt.Complete(!secretOK) // only failures consume the budget
	if !secretOK {
		recordSSOExchangeFailure(c, 0, client.ClientID, "invalid_client", http.StatusUnauthorized)
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return
	}
	flow, err := model.ConsumeAuthFlow(request.Code, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeSSOCode,
		Provider: client.ClientID,
	})
	if err != nil || flow.Payload != request.RedirectURI || !ssoSessionActive(flow.SessionId) {
		recordSSOExchangeFailure(c, 0, client.ClientID, "invalid_code", http.StatusBadRequest)
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid or expired code"))
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		recordSSOExchangeFailure(c, flow.UserId, client.ClientID, "user_unavailable", http.StatusForbidden)
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("user unavailable"))
		return
	}
	result, err := service.IssueSSOGrant(user, client)
	if errors.Is(err, model.ErrSSOCredentialLimit) || errors.Is(err, model.ErrSSOAPIKeyDisabled) {
		recordSSOExchangeFailure(c, user.Id, client.ClientID, "credential_unavailable", http.StatusConflict)
		common.ApiErrorStatus(c, http.StatusConflict, err)
		return
	}
	if err != nil {
		common.SysLog(common.LogText("SSO grant failed for user %d client %s: %v", user.Id, client.ClientID, err))
		recordSSOExchangeFailure(c, user.Id, client.ClientID, "grant_failed", http.StatusInternalServerError)
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso grant failed"))
		return
	}
	params := map[string]any{
		"client_id":                client.ClientID,
		"api_key_id":               result.APIKeyID,
		"access_token_id":          result.AccessTokenID,
		"token_ref":                result.TokenRef,
		"deleted_access_token_ids": result.DeletedPATIDs,
	}
	model.RecordOperationAuditLog(user.Id, user.Role, auditContentEN("sso.grant", params), c.ClientIP(), "sso.grant", params, nil,
		&model.AuditRequestInfo{Method: c.Request.Method, Route: c.FullPath(), Path: c.FullPath(), Status: http.StatusOK, Success: true}, c)
	common.ApiSuccess(c, result.Grant)
}

// ssoSessionActive reports whether the browser session that issued a code is
// still usable, so signing out or revoking it cancels pending codes.
func ssoSessionActive(sid string) bool {
	session, err := model.GetUserSessionCached(sid)
	return err == nil && session.Status == model.UserSessionStatusActive && session.RevokedAt == 0 && session.ExpiresAt > time.Now().Unix()
}

// recordSSOExchangeFailure audits a rejected exchange with the client and a
// reason only; codes, secrets and credentials never enter the log.
func recordSSOExchangeFailure(c *gin.Context, userID int, clientID, reason string, status int) {
	params := map[string]any{"client_id": clientID, "reason": reason}
	model.RecordOperationAuditLog(userID, 0, auditContentEN("sso.exchange_failed", params), c.ClientIP(), "sso.exchange_failed", params, nil,
		&model.AuditRequestInfo{Method: c.Request.Method, Route: c.FullPath(), Path: c.FullPath(), Status: status, Success: false}, c)
}
