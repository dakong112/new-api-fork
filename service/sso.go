package service

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// SSOCodeTTL bounds how long an application has to redeem a login code.
const SSOCodeTTL = 60 * time.Second

const ssoAccessTokenTTL = 30 * 24 * time.Hour

var ErrSSOAPIKeyLimit = errors.New("api key limit reached")

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
}

// IssueSSOGrant returns the user profile together with credentials scoped to
// one application: a reused API key billed to the user and a freshly rotated
// profile-read personal access token.
func IssueSSOGrant(user *model.User, client setting.SSOClient) (*SSOGrant, error) {
	credentialName := "sso:" + client.ClientID
	apiKey, err := ensureSSOAPIKey(user, credentialName)
	if err != nil {
		return nil, err
	}
	accessToken, expiresAt, err := rotateSSOAccessToken(user, credentialName)
	if err != nil {
		return nil, err
	}
	return &SSOGrant{
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
		AccessToken:          accessToken,
		AccessTokenExpiresAt: expiresAt,
		APIKey:               "sk-" + apiKey,
	}, nil
}

func ensureSSOAPIKey(user *model.User, name string) (string, error) {
	var token model.Token
	err := model.DB.Where("user_id = ? AND name = ?", user.Id, name).First(&token).Error
	if err == nil {
		return token.Key, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	count, err := model.CountUserTokens(user.Id)
	if err != nil {
		return "", err
	}
	if int(count) >= operation_setting.GetMaxUserTokens() {
		return "", ErrSSOAPIKeyLimit
	}
	key, err := common.GenerateKey()
	if err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	token = model.Token{
		UserId:             user.Id,
		Name:               name,
		Key:                key,
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        -1,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
	}
	if setting.DefaultUseAutoGroup {
		token.Group = "auto"
	}
	if err := token.Insert(); err != nil {
		return "", err
	}
	return key, nil
}

func rotateSSOAccessToken(user *model.User, name string) (string, int64, error) {
	existing, err := model.ListUserAccessTokens(user.Id)
	if err != nil {
		return "", 0, err
	}
	for _, token := range existing {
		if token.Name != name {
			continue
		}
		if _, err := model.DeleteUserAccessToken(user.Id, token.Id); err != nil {
			return "", 0, err
		}
	}
	suffix, err := common.GenerateRandomCharsKey(43)
	if err != nil {
		return "", 0, err
	}
	raw := model.AccessTokenPrefix + suffix
	now := time.Now()
	expiresAt := now.Add(ssoAccessTokenTTL).Unix()
	token := &model.UserAccessToken{
		Name:      name,
		TokenHash: model.AccessTokenFingerprint(raw),
		TokenHint: model.AccessTokenHint(raw),
		ExpiresAt: expiresAt,
		CreatedAt: now.Unix(),
	}
	scopes, err := NormalizeAccessTokenScopes(user.Id, user.Role, []string{"profile:read"})
	if err != nil {
		return "", 0, err
	}
	if err := token.SetScopes(scopes); err != nil {
		return "", 0, err
	}
	if err := model.CreateUserAccessToken(user.Id, token, AccessTokenMaxPerUser); err != nil {
		return "", 0, err
	}
	return raw, expiresAt, nil
}
