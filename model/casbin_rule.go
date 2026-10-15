package model

import (
	"strconv"

	"gorm.io/gorm"
)

type CasbinRule struct {
	Id    uint   `gorm:"primaryKey;autoIncrement"`
	Ptype string `gorm:"size:100;index:idx_casbin_rule,priority:1;uniqueIndex:idx_casbin_rule_unique,priority:1"`
	V0    string `gorm:"size:100;index:idx_casbin_rule,priority:2;uniqueIndex:idx_casbin_rule_unique,priority:2"`
	V1    string `gorm:"size:100;index:idx_casbin_rule,priority:3;uniqueIndex:idx_casbin_rule_unique,priority:3"`
	V2    string `gorm:"size:100;index:idx_casbin_rule,priority:4;uniqueIndex:idx_casbin_rule_unique,priority:4"`
	V3    string `gorm:"size:100;index:idx_casbin_rule,priority:5;uniqueIndex:idx_casbin_rule_unique,priority:5"`
	V4    string `gorm:"size:100;index:idx_casbin_rule,priority:6;uniqueIndex:idx_casbin_rule_unique,priority:6"`
	V5    string `gorm:"size:100;index:idx_casbin_rule,priority:7;uniqueIndex:idx_casbin_rule_unique,priority:7"`
}

func (CasbinRule) TableName() string {
	return "casbin_rule"
}

// DeleteUserRoleAssignmentsTx removes the user's role assignments ("g" rows,
// subject "user:<id>" as authz.UserSubject builds it). Demotion and deletion
// call it so custom roles never outlive the account or its admin status.
// Demotion reloads the authz policy after commit; a deleted account cannot
// authenticate, so its stale in-memory rule is inert until the next reload.
func DeleteUserRoleAssignmentsTx(tx *gorm.DB, userId int) error {
	return tx.Where("ptype = ? AND v0 = ?", "g", "user:"+strconv.Itoa(userId)).Delete(&CasbinRule{}).Error
}
