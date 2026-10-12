package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrSSOCredentialLimit = errors.New("credential limit reached")
	ErrSSOAPIKeyDisabled  = errors.New("sso api key unavailable")
)

// SSOCredentials is the outcome of IssueSSOCredentials.
type SSOCredentials struct {
	APIKey        *Token
	AccessToken   *UserAccessToken
	DeletedPATIDs []int
}

// IssueSSOCredentials reuses or creates the application's API key and replaces
// its personal access token in one transaction that holds the user's row, so
// concurrent exchanges cannot create duplicates or exceed the limits.
// newKey and pat are only persisted when needed.
func IssueSSOCredentials(userID int, name string, maxKeys, maxPATs int, newKey *Token, pat *UserAccessToken) (*SSOCredentials, error) {
	result := &SSOCredentials{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, userID).Error; err != nil {
			return err
		}

		var key Token
		err := tx.Where("user_id = ? AND name = ?", userID, name).First(&key).Error
		switch {
		case err == nil:
			if key.Status != common.TokenStatusEnabled {
				return ErrSSOAPIKeyDisabled
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			var count int64
			if err := tx.Model(&Token{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
				return err
			}
			if int(count) >= maxKeys {
				return ErrSSOCredentialLimit
			}
			key = *newKey
			if err := tx.Create(&key).Error; err != nil {
				return err
			}
		default:
			return err
		}
		result.APIKey = &key

		var old []UserAccessToken
		if err := tx.Select("id").Where("user_id = ? AND name = ?", userID, name).Find(&old).Error; err != nil {
			return err
		}
		for _, token := range old {
			result.DeletedPATIDs = append(result.DeletedPATIDs, token.Id)
		}
		if err := tx.Where("user_id = ? AND name = ?", userID, name).Delete(&UserAccessToken{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&UserAccessToken{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if int(count) >= maxPATs {
			return ErrSSOCredentialLimit
		}
		pat.Id = 0
		pat.UserId = userID
		if err := tx.Create(pat).Error; err != nil {
			return err
		}
		result.AccessToken = pat
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
