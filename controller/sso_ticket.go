package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const ssoTicketTTL = 60 * time.Second

var errSSOTicketInvalid = errors.New("invalid or expired ticket")

// CreateSSOTicket issues a one-time login ticket for a user who is signed in to
// a registered application. The caller authenticates with the user's
// application-scoped PAT plus the application secret; browser sessions and
// unrelated tokens are refused.
func CreateSSOTicket(c *gin.Context) {
	var request struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid request"))
		return
	}
	client, ok := setting.FindSSOClient(request.ClientID)
	if !ok {
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return
	}
	userID := c.GetInt("id")
	fail := func(reason string, status int, message string) {
		recordSSOAudit(c, userID, c.GetInt("role"), "sso.ticket_failed", map[string]any{"client_id": client.ClientID, "reason": reason}, status, false)
		common.ApiErrorStatus(c, status, errors.New(message))
	}
	attempt := ssoExchangeFailures.Reserve("sso-ticket-fail:"+client.ClientID+":"+c.ClientIP(), ssoExchangeMaxFailures, ssoExchangeFailureWindow)
	if attempt == nil {
		fail("throttled", http.StatusTooManyRequests, "too many failed attempts")
		return
	}
	secretOK := client.SecretMatches(request.ClientSecret)
	attempt.Complete(!secretOK) // only failures consume the budget
	if !secretOK {
		fail("invalid_client", http.StatusUnauthorized, "invalid client credentials")
		return
	}
	// Browser sessions and legacy tokens (id 0) must not mint tickets.
	tokenID := c.GetInt("access_token_id")
	if !c.GetBool("use_access_token") || tokenID <= 0 {
		fail("not_scoped_token", http.StatusForbidden, "token not issued for this client")
		return
	}
	token, err := model.GetUserAccessToken(userID, tokenID)
	if err != nil || token.Name != "sso:"+client.ClientID || token.Expired(time.Now().Unix()) {
		fail("token_mismatch", http.StatusForbidden, "token not issued for this client")
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		fail("user_unavailable", http.StatusForbidden, "user unavailable")
		return
	}
	payload, err := common.Marshal(map[string]any{"auth_version": user.AuthVersion})
	if err != nil {
		fail("ticket_failed", http.StatusInternalServerError, "ticket creation failed")
		return
	}
	expiresAt := time.Now().Add(ssoTicketTTL)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeSSOTicket,
		Provider:  client.ClientID,
		UserId:    user.Id,
		SessionId: model.AccessTokenSessionID(tokenID),
		Payload:   string(payload),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		common.SysLog(common.LogText("SSO ticket creation failed for user %d client %s: %v", user.Id, client.ClientID, err))
		fail("ticket_failed", http.StatusInternalServerError, "ticket creation failed")
		return
	}
	recordSSOAudit(c, user.Id, user.Role, "sso.ticket_issue", map[string]any{"client_id": client.ClientID, "token_id": tokenID}, http.StatusOK, true)
	common.ApiSuccess(c, gin.H{"ticket": ticket, "expires_at": expiresAt.Unix()})
}

func readSSOTicket(c *gin.Context) (string, bool) {
	var request struct {
		Ticket string `json:"ticket"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Ticket == "" || len(request.Ticket) > 128 {
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
		return "", false
	}
	return request.Ticket, true
}

// PreviewSSOTicket tells the confirmation page whom a ticket would sign in,
// without consuming it, so a forged link cannot silently log a victim into
// another account: the user sees the identity and must confirm.
func PreviewSSOTicket(c *gin.Context) {
	ticket, ok := readSSOTicket(c)
	if !ok {
		return
	}
	flow, err := model.GetAuthFlow(ticket, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeSSOTicket})
	if err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	client, clientOK := setting.FindSSOClient(flow.Provider)
	if err != nil || user.Status != common.UserStatusEnabled || !clientOK {
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
		return
	}
	common.ApiSuccess(c, gin.H{"username": user.Username, "display_name": user.DisplayName, "client_name": client.Name})
}

// RedeemSSOTicket turns a confirmed ticket into a normal login through
// setupLogin, so two-factor and passkey challenges still apply.
func RedeemSSOTicket(c *gin.Context) {
	ticket, ok := readSSOTicket(c)
	if !ok {
		return
	}
	flow, err := model.ConsumeAuthFlow(ticket, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeSSOTicket})
	if err != nil {
		// No audit row: nothing identifies the client or user of an unknown ticket.
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
		return
	}
	reject := func(reason string) {
		recordSSOAudit(c, flow.UserId, 0, "sso.ticket_failed", map[string]any{"client_id": flow.Provider, "reason": reason}, http.StatusBadRequest, false)
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
	}
	var payload struct {
		AuthVersion int64 `json:"auth_version"`
	}
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		reject("payload_invalid")
		return
	}
	tokenID, ok := model.ParseAccessTokenSessionID(flow.SessionId)
	if !ok {
		reject("token_invalid")
		return
	}
	// The issuing PAT must still exist; revoking it cancels pending tickets.
	token, err := model.GetUserAccessToken(flow.UserId, tokenID)
	if err != nil || token.Name != "sso:"+flow.Provider || token.Expired(time.Now().Unix()) {
		reject("token_revoked")
		return
	}
	if _, found := setting.FindSSOClient(flow.Provider); !found {
		reject("client_removed")
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		reject("user_unavailable")
		return
	}
	// A password change or revocation bumps AuthVersion and kills old tickets.
	if user.AuthVersion != payload.AuthVersion {
		reject("auth_version_changed")
		return
	}
	recordSSOAudit(c, user.Id, user.Role, "sso.ticket_redeem", map[string]any{"client_id": flow.Provider, "token_id": tokenID}, http.StatusOK, true)
	c.Set("login_method", "sso_ticket")
	setupLogin(user, nil, c)
}
