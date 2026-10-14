package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrSSOInsufficientQuota = errors.New("insufficient quota")

// SSOConsumeRecord remembers an application charge so a retry with the same
// idempotency key returns the first result instead of charging again.
type SSOConsumeRecord struct {
	Id             int    `json:"id"`
	UserId         int    `json:"user_id" gorm:"uniqueIndex:idx_sso_consume_key,priority:1"`
	ClientId       string `json:"client_id" gorm:"size:64;uniqueIndex:idx_sso_consume_key,priority:2"`
	IdempotencyKey string `json:"idempotency_key" gorm:"size:64;uniqueIndex:idx_sso_consume_key,priority:3"`
	Quota          int    `json:"quota"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
}

func (SSOConsumeRecord) TableName() string {
	return "sso_consume_records"
}

// SSOConsumeResult is the outcome of ConsumeSSOQuota. Replayed reports that the
// key was charged before; Quota is then the original amount.
type SSOConsumeResult struct {
	Quota    int
	Balance  int
	Replayed bool
}

// ConsumeSSOQuota charges quota once per (user, client, key). It holds the
// user's row while it looks up the key, checks the balance and deducts, so
// concurrent retries charge once and never overdraw; the unique index is the
// backstop on SQLite, which has no row locks.
// ponytail: like the balance subscription purchase, the balance check reads the
// database row; with BATCH_UPDATE_ENABLED, unflushed relay charges are not seen.
func ConsumeSSOQuota(userID int, clientID, key string, quota int) (*SSOConsumeResult, error) {
	if quota <= 0 {
		return nil, errors.New("quota must be positive")
	}
	result := &SSOConsumeResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").First(&user, userID).Error; err != nil {
			return err
		}
		var record SSOConsumeRecord
		err := tx.Where("user_id = ? AND client_id = ? AND idempotency_key = ?", userID, clientID, key).First(&record).Error
		if err == nil {
			*result = SSOConsumeResult{Quota: record.Quota, Balance: user.Quota, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if user.Quota < quota {
			return ErrSSOInsufficientQuota
		}
		record = SSOConsumeRecord{UserId: userID, ClientId: clientID, IdempotencyKey: key, Quota: quota, CreatedAt: common.GetTimestamp()}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
			"quota":      gorm.Expr("quota - ?", quota),
			"used_quota": gorm.Expr("used_quota + ?", quota),
		}).Error; err != nil {
			return err
		}
		*result = SSOConsumeResult{Quota: quota, Balance: user.Quota - quota}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !result.Replayed {
		if err := cacheDecrUserQuota(userID, int64(quota)); err != nil {
			common.SysLog(common.LogText("failed to decrease user quota cache after SSO charge: %s", err.Error()))
		}
	}
	return result, nil
}
