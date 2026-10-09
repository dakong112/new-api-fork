package model

import (
	"github.com/QuantumNous/new-api/common"
)

// GroupUsage counts what still references a group name. Groups are bound by
// name only, so a group renamed in the pricing table leaves channels, tokens
// and users on the old name; the admin UI uses these counts to surface that.
type GroupUsage struct {
	Name            string `json:"name"`
	Channels        int    `json:"channels"`
	EnabledChannels int    `json:"enabled_channels"`
	Tokens          int64  `json:"tokens"`
	Users           int64  `json:"users"`
}

func GetGroupUsage() ([]*GroupUsage, error) {
	usage := map[string]*GroupUsage{}
	entry := func(name string) *GroupUsage {
		if usage[name] == nil {
			usage[name] = &GroupUsage{Name: name}
		}
		return usage[name]
	}

	var channels []Channel
	if err := DB.Select("id", commonGroupCol, "status").Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		// A channel listing the same group twice still counts once.
		seen := map[string]bool{}
		for _, name := range channel.GetGroups() {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			e := entry(name)
			e.Channels++
			if channel.Status == common.ChannelStatusEnabled {
				e.EnabledChannels++
			}
		}
	}

	type groupCount struct {
		Name  string
		Count int64
	}
	var tokenCounts, userCounts []groupCount
	if err := DB.Model(&Token{}).Select(commonGroupCol + " AS name, COUNT(*) AS count").
		Where(commonGroupCol + " <> ''").Group(commonGroupCol).Scan(&tokenCounts).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Select(commonGroupCol + " AS name, COUNT(*) AS count").
		Where(commonGroupCol + " <> ''").Group(commonGroupCol).Scan(&userCounts).Error; err != nil {
		return nil, err
	}
	for _, row := range tokenCounts {
		entry(row.Name).Tokens = row.Count
	}
	for _, row := range userCounts {
		entry(row.Name).Users = row.Count
	}

	result := make([]*GroupUsage, 0, len(usage))
	for _, e := range usage {
		result = append(result, e)
	}
	return result, nil
}
