package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// GroupRenameResult counts what a group rename rewrote.
type GroupRenameResult struct {
	Channels          int      `json:"channels"`
	Users             int64    `json:"users"`
	Tokens            int      `json:"tokens"`
	SubscriptionPlans int64    `json:"subscription_plans"`
	UserSubscriptions int64    `json:"user_subscriptions"`
	Options           []string `json:"options"`
}

// groupNameOptionKinds lists every option keyed by or listing group names, and
// how its JSON is shaped. Aliases under group_ratio_setting are rewritten only
// when a row exists, so they cannot resurrect the old name on restart.
var groupNameOptionKinds = []struct {
	key       string
	shape     string // "map", "nestedMap", "specialUsable", "list", "mapOfLists"
	aliasOnly bool
}{
	{key: "GroupRatio", shape: "map"},
	{key: "TopupGroupRatio", shape: "map"},
	{key: "UserUsableGroups", shape: "map"},
	{key: "ModelRequestRateLimitGroup", shape: "map"},
	{key: "GroupGroupRatio", shape: "nestedMap"},
	{key: "group_ratio_setting.group_special_usable_group", shape: "specialUsable"},
	{key: "AutoGroups", shape: "list"},
	{key: "GroupDisplayOrder", shape: "list"},
	{key: "GroupFallbackGroups", shape: "mapOfLists"},
	{key: "GroupNetworkRetryGroups", shape: "list"},
	{key: "group_ratio_setting.group_ratio", shape: "map", aliasOnly: true},
	{key: "group_ratio_setting.group_group_ratio", shape: "nestedMap", aliasOnly: true},
}

var ErrGroupRenameTargetExists = errors.New("target group name is already in use")

func validateGroupRename(oldName, newName string) error {
	if oldName == "" || newName == "" {
		return errors.New("group names must not be empty")
	}
	if oldName == newName {
		return errors.New("new group name is the same as the old one")
	}
	if utf8.RuneCountInString(newName) > 64 {
		return errors.New("group name must be at most 64 characters")
	}
	if newName == "auto" || oldName == "auto" {
		return errors.New("the auto group cannot be renamed")
	}
	if strings.ContainsAny(newName, ", \t\n") || strings.HasPrefix(newName, "+:") || strings.HasPrefix(newName, "-:") {
		return errors.New("group name must not contain commas or spaces, or start with +: or -:")
	}
	return nil
}

// RenameGroup rewrites a group name everywhere it is stored so requests keep
// routing: channels (and their abilities), users, tokens, subscriptions and
// every group option. It runs in one transaction; history (logs, metrics,
// tasks) keeps the old name. The new name must be unused, so a rename never
// silently merges two groups.
func RenameGroup(oldName, newName string) (*GroupRenameResult, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if err := validateGroupRename(oldName, newName); err != nil {
		return nil, err
	}
	if _, ok := ratio_setting.GetGroupRatioCopy()[newName]; ok {
		return nil, ErrGroupRenameTargetExists
	}
	if _, ok := setting.GetUserUsableGroupsCopy()[newName]; ok {
		return nil, ErrGroupRenameTargetExists
	}
	usage, err := GetGroupUsage()
	if err != nil {
		return nil, err
	}
	for _, u := range usage {
		if u.Name == newName {
			return nil, ErrGroupRenameTargetExists
		}
	}

	result := &GroupRenameResult{Options: []string{}}
	var userIds []int
	var tokens []Token
	var planIds []int
	changedOptions := map[string]string{}

	err = DB.Transaction(func(tx *gorm.DB) error {
		// Channels store a comma list; abilities are derived per group, and
		// since no channel uses newName yet the key update cannot collide.
		var channels []Channel
		if err := tx.Select("id", commonGroupCol).Find(&channels).Error; err != nil {
			return err
		}
		for _, channel := range channels {
			groups := channel.GetGroups()
			if !slices.Contains(groups, oldName) {
				continue
			}
			for i, g := range groups {
				if g == oldName {
					groups[i] = newName
				}
			}
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).
				Update("group", strings.Join(groups, ",")).Error; err != nil {
				return err
			}
			result.Channels++
		}
		if err := tx.Model(&Ability{}).Where(commonGroupCol+" = ?", oldName).
			Update("group", newName).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Model(&User{}).Where(commonGroupCol+" = ?", oldName).
			Pluck("id", &userIds).Error; err != nil {
			return err
		}
		res := tx.Unscoped().Model(&User{}).Where(commonGroupCol+" = ?", oldName).Update("group", newName)
		if res.Error != nil {
			return res.Error
		}
		result.Users = res.RowsAffected

		// Tokens: the group column and the JSON auto-group list.
		if err := tx.Unscoped().Select("id", commonKeyCol, commonGroupCol, "auto_groups").
			Where(commonGroupCol+" = ? OR auto_groups LIKE ?", oldName, "%\""+oldName+"\"%").
			Find(&tokens).Error; err != nil {
			return err
		}
		for i := range tokens {
			updates := map[string]any{}
			if tokens[i].Group == oldName {
				updates["group"] = newName
			}
			if autoGroups, err := tokens[i].GetAutoGroups(); err == nil && slices.Contains(autoGroups, oldName) {
				for j, g := range autoGroups {
					if g == oldName {
						autoGroups[j] = newName
					}
				}
				if err := tokens[i].SetAutoGroups(autoGroups); err != nil {
					return err
				}
				updates["auto_groups"] = tokens[i].AutoGroups
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Unscoped().Model(&Token{}).Where("id = ?", tokens[i].Id).Updates(updates).Error; err != nil {
				return err
			}
			result.Tokens++
		}

		for _, col := range []string{"upgrade_group", "downgrade_group"} {
			var ids []int
			if err := tx.Model(&SubscriptionPlan{}).Where(col+" = ?", oldName).Pluck("id", &ids).Error; err != nil {
				return err
			}
			if len(ids) == 0 {
				continue
			}
			res := tx.Model(&SubscriptionPlan{}).Where(col+" = ?", oldName).Update(col, newName)
			if res.Error != nil {
				return res.Error
			}
			result.SubscriptionPlans += res.RowsAffected
			planIds = append(planIds, ids...)
		}
		for _, col := range []string{"upgrade_group", "prev_user_group", "downgrade_group"} {
			res := tx.Model(&UserSubscription{}).Where(col+" = ?", oldName).Update(col, newName)
			if res.Error != nil {
				return res.Error
			}
			result.UserSubscriptions += res.RowsAffected
		}

		for _, kind := range groupNameOptionKinds {
			var row Option
			found := tx.Where(commonKeyCol+" = ?", kind.key).Limit(1).Find(&row).RowsAffected > 0
			current := row.Value
			if !found {
				if kind.aliasOnly {
					continue
				}
				common.OptionMapRWMutex.RLock()
				current = common.OptionMap[kind.key]
				common.OptionMapRWMutex.RUnlock()
			}
			next, changed, err := renameGroupInOption(kind.shape, current, oldName, newName)
			if err != nil {
				return fmt.Errorf("%s: %w", kind.key, err)
			}
			if !changed {
				continue
			}
			if err := tx.Save(&Option{Key: kind.key, Value: next}).Error; err != nil {
				return err
			}
			changedOptions[kind.key] = next
			result.Options = append(result.Options, kind.key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Apply in memory and drop caches that still carry the old name.
	for key, value := range changedOptions {
		if err := updateOptionMap(key, value); err != nil {
			common.SysError(fmt.Sprintf("group rename: apply option %s: %v", key, err))
		}
	}
	InitChannelCache()
	InvalidatePricingCache()
	for _, id := range userIds {
		_ = invalidateUserCache(id)
	}
	_ = invalidateTokensCache(tokens)
	for _, id := range planIds {
		InvalidateSubscriptionPlanCache(id)
	}
	return result, nil
}

// renameGroupInOption rewrites oldName to newName inside one option value.
func renameGroupInOption(shape, value, oldName, newName string) (string, bool, error) {
	if strings.TrimSpace(value) == "" {
		return value, false, nil
	}
	changed := false
	rename := func(m map[string]json.RawMessage) {
		if v, ok := m[oldName]; ok {
			m[newName] = v
			delete(m, oldName)
			changed = true
		}
	}
	var out any
	switch shape {
	case "map":
		m := map[string]json.RawMessage{}
		if err := common.UnmarshalJsonStr(value, &m); err != nil {
			return "", false, err
		}
		rename(m)
		out = m
	case "nestedMap", "specialUsable":
		m := map[string]map[string]json.RawMessage{}
		if err := common.UnmarshalJsonStr(value, &m); err != nil {
			return "", false, err
		}
		if inner, ok := m[oldName]; ok {
			m[newName] = inner
			delete(m, oldName)
			changed = true
		}
		for _, inner := range m {
			rename(inner)
			if shape != "specialUsable" {
				continue
			}
			// Entries may carry "+:" (add) or "-:" (remove) prefixes.
			for _, prefix := range []string{"+:", "-:"} {
				if v, ok := inner[prefix+oldName]; ok {
					inner[prefix+newName] = v
					delete(inner, prefix+oldName)
					changed = true
				}
			}
		}
		out = m
	case "list":
		var list []string
		if err := common.UnmarshalJsonStr(value, &list); err != nil {
			return "", false, err
		}
		for i, g := range list {
			if g == oldName {
				list[i] = newName
				changed = true
			}
		}
		out = list
	case "mapOfLists":
		m := map[string][]string{}
		if err := common.UnmarshalJsonStr(value, &m); err != nil {
			return "", false, err
		}
		if list, ok := m[oldName]; ok {
			m[newName] = list
			delete(m, oldName)
			changed = true
		}
		for _, list := range m {
			for i, g := range list {
				if g == oldName {
					list[i] = newName
					changed = true
				}
			}
		}
		out = m
	default:
		return "", false, fmt.Errorf("unknown option shape %q", shape)
	}
	if !changed {
		return value, false, nil
	}
	data, err := common.Marshal(out)
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}
