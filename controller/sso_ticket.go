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

// Long enough for a person to read the confirmation page.
const ssoTicketTTL = 120 * time.Second

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
	client, user, ok := ssoClientTokenAuth(c, "ticket", request.ClientID, request.ClientSecret)
	if !ok {
		return
	}
	fail := func(reason string, status int, message string) {
		recordSSOAudit(c, user.Id, user.Role, "sso.ticket_failed", map[string]any{"client_id": client.ClientID, "reason": reason}, status, false)
		common.ApiErrorStatus(c, status, errors.New(message))
	}
	// ponytail: admins keep to the sign-in page; a stolen app PAT must not open an admin console.
	if user.Role >= common.RoleAdminUser {
		fail("role_not_allowed", http.StatusForbidden, "role not allowed")
		return
	}
	tokenID := c.GetInt("access_token_id")
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
	user, _, reason := checkSSOTicketFlow(flow)
	if reason != "" {
		common.ApiErrorStatus(c, http.StatusBadRequest, errSSOTicketInvalid)
		return
	}
	client, _ := setting.FindSSOClient(flow.Provider)
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
	user, tokenID, reason := checkSSOTicketFlow(flow)
	if reason != "" {
		reject(reason)
		return
	}
	recordSSOAudit(c, user.Id, user.Role, "sso.ticket_accepted", map[string]any{"client_id": flow.Provider, "token_id": tokenID}, http.StatusOK, true)
	// Accepted means the ticket passed every check; setupLogin may still answer with a 2FA challenge.
	c.Set("login_method", "sso_ticket")
	setupLogin(user, nil, c)
}

// checkSSOTicketFlow re-validates everything a ticket depends on and returns
// the user and issuing PAT id, or a failure reason.
func checkSSOTicketFlow(flow *model.AuthFlow) (*model.User, int, string) {
	var payload struct {
		AuthVersion int64 `json:"auth_version"`
	}
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		return nil, 0, "payload_invalid"
	}
	tokenID, ok := model.ParseAccessTokenSessionID(flow.SessionId)
	if !ok {
		return nil, 0, "token_invalid"
	}
	// The issuing PAT must still exist; revoking it cancels pending tickets.
	token, err := model.GetUserAccessToken(flow.UserId, tokenID)
	if err != nil || token.Name != "sso:"+flow.Provider || token.Expired(time.Now().Unix()) {
		return nil, 0, "token_revoked"
	}
	if _, found := setting.FindSSOClient(flow.Provider); !found {
		return nil, 0, "client_removed"
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		return nil, 0, "user_unavailable"
	}
	// A password change or revocation bumps AuthVersion and kills old tickets.
	if user.AuthVersion != payload.AuthVersion {
		return nil, 0, "auth_version_changed"
	}
	return user, tokenID, ""
}
