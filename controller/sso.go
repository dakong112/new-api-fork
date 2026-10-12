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
	if len(request.State) > 256 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	code, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeSSOCode,
		Provider:  client.ClientID,
		UserId:    c.GetInt("id"),
		Payload:   request.RedirectURI,
		ExpiresAt: time.Now().Add(service.SSOCodeTTL),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	target, err := url.Parse(request.RedirectURI)
	if err != nil {
		common.ApiErrorT(c, "Invalid SSO client or redirect URI")
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
	apps := make([]gin.H, 0, len(setting.SSOClients))
	for _, client := range setting.SSOClients {
		if client.LaunchURL == "" {
			continue
		}
		apps = append(apps, gin.H{"client_id": client.ClientID, "name": client.Name, "launch_url": client.LaunchURL})
	}
	common.ApiSuccess(c, apps)
}

// ExchangeSSOCode redeems a login code for an application server. It is
// anonymous at the dashboard level; the client secret authenticates the caller.
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
	if !ok || !client.SecretMatches(request.ClientSecret) {
		common.ApiErrorStatus(c, http.StatusUnauthorized, errors.New("invalid client credentials"))
		return
	}
	flow, err := model.ConsumeAuthFlow(request.Code, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeSSOCode,
		Provider: client.ClientID,
	})
	if err != nil || flow.Payload != request.RedirectURI {
		common.ApiErrorStatus(c, http.StatusBadRequest, errors.New("invalid or expired code"))
		return
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("user unavailable"))
		return
	}
	grant, err := service.IssueSSOGrant(user, client)
	if errors.Is(err, service.ErrSSOAPIKeyLimit) {
		common.ApiErrorStatus(c, http.StatusConflict, err)
		return
	}
	if err != nil {
		common.SysLog(common.LogText("SSO grant failed for user %d client %s: %v", user.Id, client.ClientID, err))
		common.ApiErrorStatus(c, http.StatusInternalServerError, errors.New("sso grant failed"))
		return
	}
	common.ApiSuccess(c, grant)
}
