package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	ssoExchangeMaxFailures   = 10
	ssoExchangeFailureWindow = 60 // seconds
)

// ponytail: per-process counter; with several nodes each allows its own
// budget. Move to a Redis counter if multi-node brute force matters.
var ssoExchangeFailures = newSSOExchangeLimiter()

func newSSOExchangeLimiter() *common.InMemoryRateLimiter {
	limiter := &common.InMemoryRateLimiter{}
	limiter.Init(common.RateLimitKeyExpirationDuration)
	return limiter
}

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
	if isWallet, err := model.IsTeamWalletUser(c.GetInt("id")); err != nil || isWallet {
		common.ApiErrorT(c, "Team wallet accounts cannot sign in")
		return
	}
	client, ok := setting.FindSSOClient(request.ClientID)
	if !ok || !client.AllowsRedirect(request.RedirectURI) {
		common.ApiErrorT(c, "Invalid SSO client or redirect URI")
		return
	}
	if !client.AllowsUser(c.GetInt("id")) {
		common.ApiErrorT(c, "You are not allowed to use this application")
		return
	}
	if len(request.State) > ssoStateMaxLength {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	code, err := issueSSOCode(client, c.GetInt("id"), c.GetString("session_id"), request.RedirectURI)
	if err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	common.ApiSuccess(c, gin.H{"code": code, "redirect_url": ssoRedirectURL(request.RedirectURI, "code", code, request.State)})
}

const (
	ssoStateMaxLength = 256
	// ssoAuthorizeCookiePath sits under the Refresh Cookie's Path
	// (/api/user/auth), the only place the browser sends that cookie.
	ssoAuthorizeCookiePath = "/api/user/auth/sso/authorize"
)

// issueSSOCode stores a single-use login code for a registered redirect URI,
// bound to the browser login session that approved it.
func issueSSOCode(client setting.SSOClient, userID int, sid, redirectURI string) (string, error) {
	code, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeSSOCode,
		Provider:  client.ClientID,
		UserId:    userID,
		SessionId: sid,
		Payload:   redirectURI,
		ExpiresAt: time.Now().Add(service.SSOCodeTTL),
	})
	if err != nil {
		common.SysLog(common.LogText("SSO code creation failed for user %d client %s: %v", userID, client.ClientID, err))
	}
	return code, err
}

// ssoRedirectURL adds one result parameter and the caller's state to a
// redirect URI already checked against the client registration.
func ssoRedirectURL(redirectURI, key, value, state string) string {
	target, _ := url.Parse(redirectURI)
	query := target.Query()
	query.Set(key, value)
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	return target.String()
}

// SSOAuthorize signs a user in to a registered application from the browser's
// new-api login session with no page in between: a code when the session is
// usable, otherwise error=login_required (prompt=none) or the web authorize
// page, which signs the user in first. An unknown client, an unregistered
// redirect URI or a bad state is answered here and never redirected.
func SSOAuthorize(c *gin.Context) {
	query := c.Request.URL.Query()
	redirectURI, state := query.Get("redirect_uri"), query.Get("state")
	client, ok := setting.FindSSOClient(query.Get("client_id"))
	if !ok || !client.AllowsRedirect(redirectURI) || state == "" || len(state) > ssoStateMaxLength {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid client, redirect_uri or state"))
		return
	}
	if c.Request.URL.Path != ssoAuthorizeCookiePath {
		c.Redirect(http.StatusFound, ssoAuthorizeCookiePath+"?"+c.Request.URL.RawQuery)
		return
	}
	rawRefreshToken, _ := c.Cookie(service.RefreshCookieName)
	session, ok := service.RefreshCookieSession(rawRefreshToken)
	if !ok {
		if query.Get("prompt") == "none" {
			c.Redirect(http.StatusFound, ssoRedirectURL(redirectURI, "error", "login_required", state))
			return
		}
		// The web page refreshes or signs the user in, then issues the code.
		page := url.Values{"client_id": {client.ClientID}, "redirect_uri": {redirectURI}, "state": {state}}
		c.Redirect(http.StatusFound, "/sso/authorize?"+page.Encode())
		return
	}
	isWallet, err := model.IsTeamWalletUser(session.UserID)
	if err != nil || isWallet || !client.AllowsUser(session.UserID) {
		c.Redirect(http.StatusFound, ssoRedirectURL(redirectURI, "error", "access_denied", state))
		return
	}
	code, err := issueSSOCode(client, session.UserID, session.SID, redirectURI)
	if err != nil {
		c.Redirect(http.StatusFound, ssoRedirectURL(redirectURI, "error", "server_error", state))
		return
	}
	c.Redirect(http.StatusFound, ssoRedirectURL(redirectURI, "code", code, state))
}

// ListSSOApps lists the applications a signed-in user can launch.
func ListSSOApps(c *gin.Context) {
	clients := setting.GetSSOClients()
	apps := make([]gin.H, 0, len(clients))
	for _, client := range clients {
		if client.LaunchURL == "" || !client.AllowsUser(c.GetInt("id")) {
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
	client, ok := ssoClientSecretAuth(c, "exchange", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	flow, err := model.ConsumeAuthFlow(request.Code, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeSSOCode,
		Provider: client.ClientID,
	})
	if err != nil || flow.Payload != request.RedirectURI {
		recordSSOExchangeFailure(c, 0, client.ClientID, "invalid_code", http.StatusBadRequest)
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid or expired code"))
		return
	}
	session, err := model.GetUserSessionCached(flow.SessionId)
	// Signing out or revoking the issuing session cancels its pending codes.
	if err != nil || session.Status != model.UserSessionStatusActive || session.RevokedAt != 0 || session.ExpiresAt <= time.Now().Unix() {
		recordSSOExchangeFailure(c, flow.UserId, client.ClientID, "session_inactive", http.StatusBadRequest)
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid or expired code"))
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		recordSSOExchangeFailure(c, flow.UserId, client.ClientID, "user_unavailable", http.StatusForbidden)
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("user unavailable"))
		return
	}
	// The allow-list may have changed since the code was issued.
	if !client.AllowsUser(user.Id) {
		recordSSOExchangeFailure(c, user.Id, client.ClientID, "user_not_allowed", http.StatusForbidden)
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("user not allowed"))
		return
	}
	// The link lets this application poll and end the session later.
	if err := model.LinkSSOSession(client.ClientID, session.SID, session.ExpiresAt); err != nil {
		common.SysLog(common.LogText("SSO session link failed for user %d client %s: %v", user.Id, client.ClientID, err))
		recordSSOExchangeFailure(c, user.Id, client.ClientID, "grant_failed", http.StatusInternalServerError)
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso grant failed"))
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
	recordSSOAudit(c, user.Id, user.Role, "sso.grant", params, http.StatusOK, true)
	result.Grant.SessionID = session.SID
	common.ApiSuccess(c, result.Grant)
}

// ssoClientSecretAuth authenticates an application server by its client
// secret (constant-time compare). Failures are throttled per client and caller,
// so a stranger cannot lock the real app out, and audited as
// "sso.<kind>_failed". On failure it writes the response and returns false.
func ssoClientSecretAuth(c *gin.Context, kind, clientID, clientSecret string) (setting.SSOClient, bool) {
	client, ok := setting.FindSSOClient(clientID)
	if !ok {
		// No audit row: client_id is attacker-controlled for unknown clients.
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return setting.SSOClient{}, false
	}
	attempt := ssoExchangeFailures.Reserve("sso-"+kind+"-fail:"+client.ClientID+":"+c.ClientIP(), ssoExchangeMaxFailures, ssoExchangeFailureWindow)
	if attempt == nil {
		recordSSOAudit(c, 0, 0, "sso."+kind+"_failed", map[string]any{"client_id": client.ClientID, "reason": "throttled"}, http.StatusTooManyRequests, false)
		common.ApiErrorStatus(c, http.StatusTooManyRequests, errors.New("too many failed attempts"))
		return setting.SSOClient{}, false
	}
	secretOK := client.SecretMatches(clientSecret)
	attempt.Complete(!secretOK) // only failures consume the budget
	if !secretOK {
		recordSSOAudit(c, 0, 0, "sso."+kind+"_failed", map[string]any{"client_id": client.ClientID, "reason": "invalid_client"}, http.StatusUnauthorized, false)
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return setting.SSOClient{}, false
	}
	return client, true
}

const ssoSessionStatusMaxIDs = 100

// GetSSOSessionStatus tells an application which login sessions it signed
// users in from are still active, so it can end its own sessions after the
// user signs out of new-api, changes the password or is disabled. Sessions
// never linked to the client read as inactive.
func GetSSOSessionStatus(c *gin.Context) {
	var request struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		SessionIDs   []string `json:"session_ids"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || len(request.SessionIDs) > ssoSessionStatusMaxIDs {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, ok := ssoClientSecretAuth(c, "session_status", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	linked, err := model.LinkedSSOSessionIDs(client.ClientID, request.SessionIDs)
	if err != nil {
		common.SysLog(common.LogText("SSO session status failed for client %s: %v", client.ClientID, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso session status failed"))
		return
	}
	active := make(map[string]bool, len(request.SessionIDs))
	for _, sid := range request.SessionIDs {
		active[sid] = linked[sid] && service.LoginSessionActive(sid)
	}
	common.ApiSuccess(c, gin.H{"active": active})
}

// LogoutSSOSession ends a new-api login session for an application (single
// logout), with the effect of the user signing that session out. Only sessions
// linked to the client may be ended; repeating the call succeeds.
func LogoutSSOSession(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		SessionID    string `json:"session_id"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.SessionID == "" || len(request.SessionID) > 64 {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, ok := ssoClientSecretAuth(c, "session_logout", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	linked, err := model.LinkedSSOSessionIDs(client.ClientID, []string{request.SessionID})
	if err != nil {
		common.SysLog(common.LogText("SSO session logout failed for client %s: %v", client.ClientID, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso session logout failed"))
		return
	}
	if !linked[request.SessionID] {
		recordSSOAudit(c, 0, 0, "sso.session_logout_failed", map[string]any{"client_id": client.ClientID, "reason": "not_linked"}, http.StatusForbidden, false)
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("session not linked to this client"))
		return
	}
	// A session already cleaned up has nothing left to end.
	session, err := model.GetUserSessionBySID(request.SessionID)
	if err != nil {
		common.ApiSuccess(c, gin.H{"revoked": false})
		return
	}
	revoked, err := model.RevokeUserSession(session.UserID, session.SID, "sso_client_logout")
	if err != nil {
		common.SysLog(common.LogText("SSO session logout failed for client %s: %v", client.ClientID, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso session logout failed"))
		return
	}
	// The session id stays out of the audit row; user and client suffice.
	recordSSOAudit(c, session.UserID, 0, "sso.session_logout", map[string]any{"client_id": client.ClientID, "revoked": revoked}, http.StatusOK, true)
	common.ApiSuccess(c, gin.H{"revoked": revoked})
}

// recordSSOExchangeFailure audits a rejected exchange with the client and a
// reason only; codes, secrets and credentials never enter the log.
func recordSSOExchangeFailure(c *gin.Context, userID int, clientID, reason string, status int) {
	recordSSOAudit(c, userID, 0, "sso.exchange_failed", map[string]any{"client_id": clientID, "reason": reason}, status, false)
}

// recordSSOAudit writes a security audit row for a request authenticated by an
// application secret, not by a user session.
func recordSSOAudit(c *gin.Context, userID, role int, action string, params map[string]any, status int, success bool) {
	username, _ := model.GetUsernameById(userID, false)
	model.RecordAuditLog(c, model.AuditLog{
		UserId: userID, Username: username, ActorRole: role, Category: model.AuditCategorySecurity,
		Action: action, Content: auditContentEN(action, params), Ip: c.ClientIP(), AuthMethod: "sso_client",
		Status: status, Success: success,
		Other: model.AuditOther{
			Op:        &model.AuditOperation{Action: action, Params: params},
			AuditInfo: &model.AuditRequestInfo{Method: c.Request.Method, Route: c.FullPath(), Path: c.FullPath(), Status: status, Success: success},
		},
	})
}

// ssoClientTokenAuth checks that the caller holds the application secret and
// the personal access token the SSO grant issued to that application. Browser
// sessions and unrelated tokens are refused. On failure it audits
// "sso.<kind>_failed", writes the response and returns ok=false.
func ssoClientTokenAuth(c *gin.Context, kind, clientID, clientSecret string) (setting.SSOClient, *model.User, bool) {
	client, ok := setting.FindSSOClient(clientID)
	if !ok {
		// No audit row: client_id is attacker-controlled for unknown clients.
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return setting.SSOClient{}, nil, false
	}
	userID := c.GetInt("id")
	fail := func(reason string, status int, message string) {
		recordSSOAudit(c, userID, c.GetInt("role"), "sso."+kind+"_failed", map[string]any{"client_id": client.ClientID, "reason": reason}, status, false)
		common.ApiErrorStatus(c, status, errors.New(message))
	}
	attempt := ssoExchangeFailures.Reserve("sso-"+kind+"-fail:"+client.ClientID+":"+c.ClientIP(), ssoExchangeMaxFailures, ssoExchangeFailureWindow)
	if attempt == nil {
		fail("throttled", http.StatusTooManyRequests, "too many failed attempts")
		return setting.SSOClient{}, nil, false
	}
	secretOK := client.SecretMatches(clientSecret)
	attempt.Complete(!secretOK) // only failures consume the budget
	if !secretOK {
		fail("invalid_client", http.StatusUnauthorized, "invalid client credentials")
		return setting.SSOClient{}, nil, false
	}
	// Browser sessions and legacy tokens (id 0) are not application credentials.
	tokenID := c.GetInt("access_token_id")
	if !c.GetBool("use_access_token") || tokenID <= 0 {
		fail("not_scoped_token", http.StatusForbidden, "token not issued for this client")
		return setting.SSOClient{}, nil, false
	}
	token, err := model.GetUserAccessToken(userID, tokenID)
	if err != nil || token.Name != "sso:"+client.ClientID || token.Expired(time.Now().Unix()) {
		fail("token_mismatch", http.StatusForbidden, "token not issued for this client")
		return setting.SSOClient{}, nil, false
	}
	user, err := model.GetUserById(userID, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		fail("user_unavailable", http.StatusForbidden, "user unavailable")
		return setting.SSOClient{}, nil, false
	}
	// Removing a user from the allow-list also cuts off their issued credentials.
	if !client.AllowsUser(user.Id) {
		fail("user_not_allowed", http.StatusForbidden, "user not allowed")
		return setting.SSOClient{}, nil, false
	}
	return client, user, true
}

// CreateSSOKey returns the user's API key for one group, creating it on first
// use, so an application can bill each call at the group the user picked
// without the user managing keys. The group rule is TokenAuth's: the group
// must be usable by the user and priced (or "auto"). Admins are allowed: the
// key only spends their own quota and opens no session.
func CreateSSOKey(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Group        string `json:"group"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, user, ok := ssoClientTokenAuth(c, "key", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	fail := func(reason string, status int, err error) {
		recordSSOAudit(c, user.Id, user.Role, "sso.key_failed", map[string]any{"client_id": client.ClientID, "reason": reason}, status, false)
		common.ApiErrorStatus(c, status, err)
	}
	group := request.Group
	if group == "" {
		group = user.Group
	}
	_, usable := service.GetUserUsableGroups(user.Group)[group]
	if len(group) > 64 || !usable || !ratio_setting.ContainsGroupRatio(group) && group != "auto" {
		fail("group_unavailable", http.StatusBadRequest, errors.New("group not available"))
		return
	}
	key, err := service.IssueSSOGroupKey(user, client, group)
	if errors.Is(err, model.ErrSSOCredentialLimit) || errors.Is(err, model.ErrSSOAPIKeyDisabled) {
		fail("credential_unavailable", http.StatusConflict, err)
		return
	}
	if err != nil {
		common.SysLog(common.LogText("SSO group key failed for user %d client %s: %v", user.Id, client.ClientID, err))
		fail("key_failed", http.StatusInternalServerError, errors.New("sso key failed"))
		return
	}
	recordSSOAudit(c, user.Id, user.Role, "sso.key_issue", map[string]any{"client_id": client.ClientID, "group": group, "api_key_id": key.Id}, http.StatusOK, true)
	common.ApiSuccess(c, gin.H{"key": "sk-" + key.Key})
}

// GetSSOPermissions returns the user's current permission codes for the
// client's application, so it can refresh them without a new sign-in.
// Authentication is CreateSSOKey's.
func GetSSOPermissions(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, user, ok := ssoClientTokenAuth(c, "permissions", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	common.ApiSuccess(c, gin.H{"permissions": service.SSOPermissions(user, client)})
}

// GetSSOCatalog returns the /api/pricing payload as the user sees it, so an
// application's model catalog keeps working when the pricing page is switched
// off in the header navigation. Authentication is CreateSSOKey's.
func GetSSOCatalog(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	_, user, ok := ssoClientTokenAuth(c, "catalog", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, pricingCatalog(user.Group))
}

// ListSSOTeams lists the user's teams for an application's space switcher.
// Authentication is CreateSSOKey's.
func ListSSOTeams(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	_, user, ok := ssoClientTokenAuth(c, "teams", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	teams, err := model.ListUserTeams(user.Id)
	if err != nil {
		common.SysLog(common.LogText("SSO team list failed for user %d: %v", user.Id, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso teams failed"))
		return
	}
	common.ApiSuccess(c, teams)
}

// CreateSSOTeamKey returns the user's member key of a team, so an application
// bills team-space calls to the team wallet within the member's limit.
// Authentication is CreateSSOKey's; non-members get 404.
func CreateSSOTeamKey(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		TeamID       int    `json:"team_id"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, user, ok := ssoClientTokenAuth(c, "team_key", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	fail := func(reason string, status int, err error) {
		recordSSOAudit(c, user.Id, user.Role, "sso.team_key_failed", map[string]any{"client_id": client.ClientID, "team_id": request.TeamID, "reason": reason}, status, false)
		common.ApiErrorStatus(c, status, err)
	}
	key, err := model.GetTeamMemberToken(request.TeamID, user.Id)
	if errors.Is(err, model.ErrTeamNotFound) {
		fail("not_member", http.StatusNotFound, errors.New("team not found"))
		return
	}
	if err != nil {
		common.SysLog(common.LogText("SSO team key failed for user %d team %d: %v", user.Id, request.TeamID, err))
		fail("key_failed", http.StatusInternalServerError, errors.New("sso team key failed"))
		return
	}
	if key.Status != common.TokenStatusEnabled && key.Status != common.TokenStatusExhausted {
		fail("credential_unavailable", http.StatusConflict, model.ErrSSOAPIKeyDisabled)
		return
	}
	recordSSOAudit(c, user.Id, user.Role, "sso.team_key_issue", map[string]any{"client_id": client.ClientID, "team_id": request.TeamID, "api_key_id": key.Id}, http.StatusOK, true)
	common.ApiSuccess(c, gin.H{"key": "sk-" + key.Key})
}

// GetSSOClient tells an application server its display name and the defaults
// the administrator chose for it. The application authenticates with HTTP
// Basic client credentials; failures share the exchange throttle.
func GetSSOClient(c *gin.Context) {
	clientID, secret, _ := c.Request.BasicAuth()
	client, ok := ssoClientSecretAuth(c, "client", clientID, secret)
	if !ok {
		return
	}
	common.ApiSuccess(c, gin.H{"name": client.Name, "defaults": client.Defaults})
}

// ConsumeSSOQuota charges the user a fee the application sets (for example a
// paid template), once per idempotency key: a retry returns the first result
// without charging again. Authentication is CreateSSOKey's.
func ConsumeSSOQuota(c *gin.Context) {
	var request struct {
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		IdempotencyKey string `json:"idempotency_key"`
		Quota          int    `json:"quota"`
		Title          string `json:"title"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, user, ok := ssoClientTokenAuth(c, "consume", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	title := strings.TrimSpace(request.Title)
	titleLen := utf8.RuneCountInString(title)
	// The per-client cap keeps a client bug from draining a wallet in one call.
	maxQuota := client.ConsumeMaxQuota()
	if request.IdempotencyKey == "" || len(request.IdempotencyKey) > 64 || request.Quota < 1 || request.Quota > maxQuota ||
		titleLen < 1 || titleLen > 100 || !utf8.ValidString(title) {
		recordSSOAudit(c, user.Id, user.Role, "sso.consume_failed", map[string]any{"client_id": client.ClientID, "reason": "invalid_request"}, http.StatusBadRequest, false)
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	result, err := model.ConsumeSSOQuota(user.Id, client.ClientID, request.IdempotencyKey, request.Quota)
	if errors.Is(err, model.ErrSSOInsufficientQuota) {
		common.ApiErrorStatus(c, http.StatusPaymentRequired, err)
		return
	}
	if err != nil {
		common.SysLog(common.LogText("SSO charge failed for user %d client %s: %v", user.Id, client.ClientID, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso consume failed"))
		return
	}
	if !result.Replayed {
		model.RecordConsumeLog(c, user.Id, model.RecordConsumeLogParams{
			ModelName: client.ClientID + ":" + title,
			Quota:     result.Quota,
			Content:   []*common.Message{common.NewMessage("Charged by application {{app}}", map[string]any{"app": client.ClientID})},
			Other:     model.NewLogOther(),
		})
	}
	common.ApiSuccess(c, gin.H{"quota": result.Quota, "balance": result.Balance})
}
