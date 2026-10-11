package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetGroupUsageCountsReferencesByGroupName(t *testing.T) {
	wipe := func() {
		// A fresh chain per delete: a reused GORM chain shares its statement.
		for _, table := range []any{&Channel{}, &Token{}, &User{}, &Ability{}} {
			require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table).Error)
		}
		InitChannelCache()
		InvalidatePricingCache()
	}
	wipe()
	t.Cleanup(wipe)

	require.NoError(t, DB.Create(&[]Channel{
		{Name: "a", Key: "k1", Group: "default,vip", Status: common.ChannelStatusEnabled},
		{Name: "b", Key: "k2", Group: "vip, vip", Status: common.ChannelStatusEnabled},
		{Name: "c", Key: "k3", Group: "old-vip", Status: common.ChannelStatusManuallyDisabled},
	}).Error)
	require.NoError(t, DB.Create(&[]User{
		{Username: "u1", Password: "password", Group: "vip", AffCode: "g1"},
		{Username: "u2", Password: "password", Group: "old-vip", AffCode: "g2"},
	}).Error)
	require.NoError(t, DB.Create(&[]Token{
		{Name: "t1", Key: "gu1", Group: "old-vip"},
		{Name: "t2", Key: "gu2", Group: "old-vip"},
		{Name: "t3", Key: "gu3", Group: ""}, // empty follows the user's group
	}).Error)

	usage, err := GetGroupUsage()
	require.NoError(t, err)
	byName := map[string]GroupUsage{}
	for _, u := range usage {
		byName[u.Name] = *u
	}

	assert.Equal(t, GroupUsage{Name: "default", Channels: 1, EnabledChannels: 1}, byName["default"])
	assert.Equal(t, GroupUsage{Name: "vip", Channels: 2, EnabledChannels: 2, Users: 1}, byName["vip"])
	assert.Equal(t, GroupUsage{Name: "old-vip", Channels: 1, EnabledChannels: 0, Tokens: 2, Users: 1}, byName["old-vip"])
	assert.NotContains(t, byName, "")
}

func TestGetGroupUsageListsVendorsOfEnabledModels(t *testing.T) {
	resetPricingEndpointTestTables(t)
	require.NoError(t, DB.Create(&Channel{
		Id: 1, Type: 1, Key: "key-1", Name: "channel-1",
		Status: common.ChannelStatusEnabled, Group: "mix,misc",
	}).Error)
	for _, ability := range []Ability{
		{Group: "mix", Model: "deepseek-v4-flash", ChannelId: 1, Enabled: true},
		{Group: "mix", Model: "glm-5.3", ChannelId: 1, Enabled: true},
		{Group: "mix", Model: "mimo-v2.5", ChannelId: 1, Enabled: true},
		{Group: "misc", Model: "acme-unknown-1", ChannelId: 1, Enabled: true},
		// Disabled: its vendor (OpenAI) must not show up in mix's vendors.
		{Group: "mix", Model: "gpt-5.5", ChannelId: 1, Enabled: false},
	} {
		require.NoError(t, DB.Create(&ability).Error)
	}
	InitChannelCache()
	InvalidatePricingCache()

	usage, err := GetGroupUsage()
	require.NoError(t, err)
	byName := map[string]*GroupUsage{}
	for _, u := range usage {
		byName[u.Name] = u
	}

	require.Contains(t, byName, "mix")
	assert.Equal(t, []string{"DeepSeek", "小米 MiMo", "智谱 GLM"}, byName["mix"].Vendors)
	assert.Equal(t, 1, byName["mix"].Channels)
	assert.Equal(t, 1, byName["mix"].EnabledChannels)
	require.Contains(t, byName, "misc")
	// "" stands for "Other": an enabled model with no vendor.
	assert.Equal(t, []string{""}, byName["misc"].Vendors)
	assert.Equal(t, 1, byName["misc"].Channels)
	assert.Equal(t, 1, byName["misc"].EnabledChannels)
}
