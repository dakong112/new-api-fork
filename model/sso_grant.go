package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

		key, err := getOrCreateSSOKey(tx, userID, name, maxKeys, newKey)
		if err != nil {
			return err
		}
		result.APIKey = key

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

// GetOrCreateSSOKey returns the user's API key with this name, creating newKey
// when there is none. It holds the user's row like IssueSSOCredentials, so
// concurrent first uses cannot create two keys or exceed maxKeys.
func GetOrCreateSSOKey(userID int, name string, maxKeys int, newKey *Token) (*Token, error) {
	var key *Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, userID).Error; err != nil {
			return err
		}
		var err error
		key, err = getOrCreateSSOKey(tx, userID, name, maxKeys, newKey)
		return err
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

// getOrCreateSSOKey must run inside a transaction holding the user's row.
// A disabled or expired key is reported, never replaced: the user turned it off.
func getOrCreateSSOKey(tx *gorm.DB, userID int, name string, maxKeys int, newKey *Token) (*Token, error) {
	var key Token
	err := tx.Where("user_id = ? AND name = ?", userID, name).First(&key).Error
	if err == nil {
		if key.Status != common.TokenStatusEnabled || (key.ExpiredTime != -1 && key.ExpiredTime < common.GetTimestamp()) {
			return nil, ErrSSOAPIKeyDisabled
		}
		return &key, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var count int64
	if err := tx.Model(&Token{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, err
	}
	if int(count) >= maxKeys {
		return nil, ErrSSOCredentialLimit
	}
	key = *newKey
	if err := tx.Create(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// SSOSessionLink records that a browser login session signed its user in to an
// application, so that application may ask about or end that session and no
// other one.
type SSOSessionLink struct {
	Id        int    `json:"id"`
	ClientId  string `json:"client_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_sso_session_link,priority:1"`
	SID       string `json:"sid" gorm:"column:sid;type:varchar(64);not null;uniqueIndex:idx_sso_session_link,priority:2"`
	ExpiresAt int64  `json:"expires_at" gorm:"type:bigint;not null;index"`
}

func (SSOSessionLink) TableName() string {
	return "sso_session_links"
}

// LinkSSOSession records the link once; repeated sign-ins from the same
// session keep the first row.
func LinkSSOSession(clientID, sid string, expiresAt int64) error {
	link := SSOSessionLink{ClientId: clientID, SID: sid, ExpiresAt: expiresAt}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
}

// LinkedSSOSessionIDs returns which of sids are linked to the application.
func LinkedSSOSessionIDs(clientID string, sids []string) (map[string]bool, error) {
	linked := make(map[string]bool, len(sids))
	if len(sids) == 0 {
		return linked, nil
	}
	var found []string
	if err := DB.Model(&SSOSessionLink{}).Where("client_id = ? AND sid IN ?", clientID, sids).Pluck("sid", &found).Error; err != nil {
		return nil, err
	}
	for _, sid := range found {
		linked[sid] = true
	}
	return linked, nil
}

// DeleteExpiredSSOSessionLinks drops links whose login session has expired.
func DeleteExpiredSSOSessionLinks(now int64) error {
	return DB.Where("expires_at < ?", now).Delete(&SSOSessionLink{}).Error
}
