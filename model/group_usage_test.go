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
		for _, table := range []any{&Channel{}, &Token{}, &User{}} {
			require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table).Error)
		}
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
