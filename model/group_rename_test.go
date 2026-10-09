package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupGroupRenameOptions applies group options and restores the previous
// in-memory settings afterwards, since they are package-level state.
func setupGroupRenameOptions(t *testing.T, values map[string]string) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Option{}))
	special, err := common.Marshal(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll())
	require.NoError(t, err)
	previous := map[string]string{
		"GroupRatio":                 ratio_setting.GroupRatio2JSONString(),
		"GroupGroupRatio":            ratio_setting.GroupGroupRatio2JSONString(),
		"TopupGroupRatio":            common.TopupGroupRatio2JSONString(),
		"UserUsableGroups":           setting.UserUsableGroups2JSONString(),
		"AutoGroups":                 setting.AutoGroups2JsonString(),
		"ModelRequestRateLimitGroup": setting.ModelRequestRateLimitGroup2JSONString(),
		"GroupDisplayOrder":          "[]",
		"GroupFallbackGroups":        "{}",
		"group_ratio_setting.group_special_usable_group": string(special),
	}
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		for key, value := range previous {
			require.NoError(t, updateOptionMap(key, value))
		}
		common.OptionMap = previousMap
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Option{}).Error)
	})
	for key, value := range values {
		require.NoError(t, UpdateOption(key, value))
	}
}

func wipeGroupRenameTables(t *testing.T) {
	t.Helper()
	// A fresh chain per delete: a reused GORM chain shares its statement.
	for _, table := range []any{&Channel{}, &Ability{}, &User{}, &Token{}, &SubscriptionPlan{}, &UserSubscription{}} {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table).Error)
	}
}

func optionValue(t *testing.T, key string) string {
	t.Helper()
	var row Option
	require.NoError(t, DB.Where(commonKeyCol+" = ?", key).First(&row).Error)
	return row.Value
}

func TestRenameGroupRewritesEveryReference(t *testing.T) {
	wipeGroupRenameTables(t)
	t.Cleanup(func() { wipeGroupRenameTables(t) })
	setupGroupRenameOptions(t, map[string]string{
		"GroupRatio":                 `{"default":1,"vip":2}`,
		"TopupGroupRatio":            `{"vip":1.5}`,
		"UserUsableGroups":           `{"default":"Default","vip":"VIP"}`,
		"GroupGroupRatio":            `{"vip":{"default":0.9},"default":{"vip":1.1}}`,
		"AutoGroups":                 `["default","vip"]`,
		"GroupDisplayOrder":          `["vip","default"]`,
		"GroupFallbackGroups":        `{"vip":["default"],"default":["vip","backup"]}`,
		"ModelRequestRateLimitGroup": `{"vip":[10,5]}`,
		"group_ratio_setting.group_special_usable_group": `{"default":{"+:vip":"VIP","-:vip":""},"vip":{"default":"D"}}`,
	})

	require.NoError(t, DB.Create(&[]Channel{
		{Id: 1, Name: "a", Key: "k1", Group: "default,vip", Status: common.ChannelStatusEnabled},
		{Id: 2, Name: "b", Key: "k2", Group: "vip", Status: common.ChannelStatusEnabled},
		{Id: 3, Name: "c", Key: "k3", Group: "default", Status: common.ChannelStatusEnabled},
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "m", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "m", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "m", ChannelId: 2, Enabled: true},
	}).Error)
	require.NoError(t, DB.Create(&[]User{
		{Id: 1, Username: "u1", Password: "password", Group: "vip", AffCode: "r1"},
		{Id: 2, Username: "u2", Password: "password", Group: "default", AffCode: "r2"},
	}).Error)
	require.NoError(t, DB.Create(&[]Token{
		{Id: 1, Name: "t1", Key: "rename1", Group: "vip"},
		{Id: 2, Name: "t2", Key: "rename2", Group: "auto", AutoGroups: `["default","vip"]`},
		{Id: 3, Name: "t3", Key: "rename3", Group: "default"},
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionPlan{Id: 1, Title: "p", UpgradeGroup: "vip", DowngradeGroup: "default"}).Error)
	require.NoError(t, DB.Create(&UserSubscription{Id: 1, UserId: 2, PlanId: 1, UpgradeGroup: "vip", PrevUserGroup: "default"}).Error)

	result, err := RenameGroup("vip", "premium")
	require.NoError(t, err)

	assert.Equal(t, 2, result.Channels)
	assert.Equal(t, int64(1), result.Users)
	assert.Equal(t, 2, result.Tokens)
	assert.Equal(t, int64(1), result.SubscriptionPlans)
	assert.Equal(t, int64(1), result.UserSubscriptions)

	var channels []Channel
	require.NoError(t, DB.Order("id").Find(&channels).Error)
	assert.Equal(t, []string{"default,premium", "premium", "default"},
		[]string{channels[0].Group, channels[1].Group, channels[2].Group})
	var abilityGroups []string
	require.NoError(t, DB.Model(&Ability{}).Order("channel_id, "+commonGroupCol).Pluck(commonGroupCol, &abilityGroups).Error)
	assert.Equal(t, []string{"default", "premium", "premium"}, abilityGroups)

	var user User
	require.NoError(t, DB.First(&user, 1).Error)
	assert.Equal(t, "premium", user.Group)
	var tokens []Token
	require.NoError(t, DB.Order("id").Find(&tokens).Error)
	assert.Equal(t, "premium", tokens[0].Group)
	assert.Equal(t, `["default","premium"]`, tokens[1].AutoGroups)
	assert.Equal(t, "default", tokens[2].Group)
	var plan SubscriptionPlan
	require.NoError(t, DB.First(&plan, 1).Error)
	assert.Equal(t, "premium", plan.UpgradeGroup)
	assert.Equal(t, "default", plan.DowngradeGroup)
	var sub UserSubscription
	require.NoError(t, DB.First(&sub, 1).Error)
	assert.Equal(t, "premium", sub.UpgradeGroup)

	assert.JSONEq(t, `{"default":1,"premium":2}`, optionValue(t, "GroupRatio"))
	assert.JSONEq(t, `{"premium":1.5}`, optionValue(t, "TopupGroupRatio"))
	assert.JSONEq(t, `{"default":"Default","premium":"VIP"}`, optionValue(t, "UserUsableGroups"))
	assert.JSONEq(t, `{"premium":{"default":0.9},"default":{"premium":1.1}}`, optionValue(t, "GroupGroupRatio"))
	assert.JSONEq(t, `["default","premium"]`, optionValue(t, "AutoGroups"))
	assert.JSONEq(t, `["premium","default"]`, optionValue(t, "GroupDisplayOrder"))
	assert.JSONEq(t, `{"premium":["default"],"default":["premium","backup"]}`, optionValue(t, "GroupFallbackGroups"))
	assert.JSONEq(t, `{"premium":[10,5]}`, optionValue(t, "ModelRequestRateLimitGroup"))
	assert.JSONEq(t, `{"default":{"+:premium":"VIP","-:premium":""},"premium":{"default":"D"}}`,
		optionValue(t, "group_ratio_setting.group_special_usable_group"))

	// The running process sees the new name without a restart.
	ratios := ratio_setting.GetGroupRatioCopy()
	assert.Contains(t, ratios, "premium")
	assert.NotContains(t, ratios, "vip")
	assert.Equal(t, []string{"premium", "default"}, common.GroupDisplayOrder)
}

func TestRenameGroupRejectsUnsafeNames(t *testing.T) {
	wipeGroupRenameTables(t)
	t.Cleanup(func() { wipeGroupRenameTables(t) })
	setupGroupRenameOptions(t, map[string]string{"GroupRatio": `{"default":1,"vip":2}`})
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "k1", Group: "legacy"}).Error)

	cases := []struct{ from, to string }{
		{"vip", "default"}, // exists in the pricing table
		{"vip", "legacy"},  // still used by a channel
		{"vip", "vip"},
		{"vip", ""},
		{"vip", "a,b"},
		{"vip", "+:x"},
		{"auto", "x"},
		{"vip", "auto"},
	}
	for _, tc := range cases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			_, err := RenameGroup(tc.from, tc.to)
			assert.Error(t, err)
		})
	}
	assert.JSONEq(t, `{"default":1,"vip":2}`, ratio_setting.GroupRatio2JSONString())
}
