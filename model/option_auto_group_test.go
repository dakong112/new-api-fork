package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOptionValueRejectsInvalidMaxTokenAutoGroups(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "1.5", "invalid"} {
		t.Run(value, func(t *testing.T) {
			assert.Error(t, validateOptionValue("MaxTokenAutoGroups", value))
		})
	}
	require.NoError(t, validateOptionValue("MaxTokenAutoGroups", "999999"))
}

func TestValidateOptionValueRejectsInvalidTestRedemptionQuota(t *testing.T) {
	for _, key := range []string{"RedemptionTestQuota", "RedemptionTestRepeatQuota"} {
		for _, value := range []string{"", "0", "-1", "1.5", "invalid"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				assert.Error(t, validateOptionValue(key, value))
			})
		}
		require.NoError(t, validateOptionValue(key, "750000"))
	}
}

func TestValidateOptionValueRejectsInvalidGroupDisplayOrder(t *testing.T) {
	for _, value := range []string{"", "{}", `"vip"`, `[1,2]`, "not json"} {
		t.Run(value, func(t *testing.T) {
			assert.Error(t, validateOptionValue("GroupDisplayOrder", value))
		})
	}
	require.NoError(t, validateOptionValue("GroupDisplayOrder", `["vip","default"]`))
	require.NoError(t, validateOptionValue("GroupDisplayOrder", `[]`))
}
