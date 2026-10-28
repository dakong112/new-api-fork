package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// SSOCodeTTL bounds how long an application has to redeem a login code.
const SSOCodeTTL = 60 * time.Second

const ssoAccessTokenTTL = 30 * 24 * time.Hour

type SSOGrantUser struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        int    `json:"role"`
	Group       string `json:"group"`
	Quota       int    `json:"quota"`
	UsedQuota   int    `json:"used_quota"`
}

// SSOGrant is what an application receives for a redeemed login code.
type SSOGrant struct {
	User                 SSOGrantUser `json:"user"`
	AccessToken          string       `json:"access_token"`
	AccessTokenExpiresAt int64        `json:"access_token_expires_at"`
	APIKey               string       `json:"api_key"`
	// Permissions are the user's codes for the client's application.
	Permissions []string `json:"permissions"`
	// SessionID is the browser login session the code was issued from; the
	// application polls /api/sso/session/status with it.
	SessionID string `json:"session_id"`
}

// SSOPermissions returns the permission codes the user holds in the client's
// application; none when the client names no application.
func SSOPermissions(user *model.User, client setting.SSOClient) []string {
	if client.App == "" {
		return []string{}
	}
	return authz.CodesForApp(user.Id, user.Role, client.App)
}

// SSOGrantResult carries the grant plus the identifiers needed for auditing.
type SSOGrantResult struct {
	Grant         *SSOGrant
	APIKeyID      int
	AccessTokenID int
	TokenRef      string
	DeletedPATIDs []int
}

// IssueSSOGrant returns the user profile together with credentials scoped to
// one application: a reused API key billed to the user and a freshly rotated
// profile-read personal access token.
func IssueSSOGrant(user *model.User, client setting.SSOClient) (*SSOGrantResult, error) {
	name := "sso:" + client.ClientID
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	apiKey := &model.Token{
		UserId:         user.Id,
		Name:           name,
		Key:            key,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	if setting.DefaultUseAutoGroup {
		apiKey.Group = "auto"
	}

	suffix, err := common.GenerateRandomCharsKey(43)
	if err != nil {
		return nil, err
	}
	raw := model.AccessTokenPrefix + suffix
	created := time.Now()
	expiresAt := created.Add(ssoAccessTokenTTL).Unix()
	pat := &model.UserAccessToken{
		Name:      name,
		TokenHash: model.AccessTokenFingerprint(raw),
		TokenHint: model.AccessTokenHint(raw),
		ExpiresAt: expiresAt,
		CreatedAt: created.Unix(),
	}
	scopes, err := NormalizeAccessTokenScopes(user.Id, user.Role, []string{"profile:read", "wallet:read", "wallet:write"})
	if err != nil {
		return nil, err
	}
	if err := pat.SetScopes(scopes); err != nil {
		return nil, err
	}

	issued, err := model.IssueSSOCredentials(user.Id, name, operation_setting.GetMaxUserTokens(), AccessTokenMaxPerUser, apiKey, pat)
	if err != nil {
		return nil, err
	}
	return &SSOGrantResult{
		Grant: &SSOGrant{
			User: SSOGrantUser{
				Id:          user.Id,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				Email:       user.Email,
				Role:        user.Role,
				Group:       user.Group,
				Quota:       user.Quota,
				UsedQuota:   user.UsedQuota,
			},
			AccessToken:          raw,
			AccessTokenExpiresAt: expiresAt,
			APIKey:               "sk-" + issued.APIKey.Key,
			Permissions:          SSOPermissions(user, client),
		},
		APIKeyID:      issued.APIKey.Id,
		AccessTokenID: issued.AccessToken.Id,
		TokenRef:      pat.TokenHash,
		DeletedPATIDs: issued.DeletedPATIDs,
	}, nil
}

// IssueSSOGroupKey returns the application's API key for one group of the
// user, creating an unlimited, non-expiring key named sso:<client>@<group> on
// first use. Usage is billed to the user at that group's ratio as usual.
func IssueSSOGroupKey(user *model.User, client setting.SSOClient, group string) (*model.Token, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	name := "sso:" + client.ClientID + "@" + group
	now := common.GetTimestamp()
	token, err := model.GetOrCreateSSOKey(user.Id, name, operation_setting.GetMaxUserTokens(), &model.Token{
		UserId:         user.Id,
		Name:           name,
		Key:            key,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          group,
	})
	if err != nil {
		return nil, err
	}
	if token.Group != group {
		return nil, model.ErrSSOAPIKeyDisabled
	}
	return token, nil
}
