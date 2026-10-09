package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectAutoGroupsTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMaxTokenAutoGroups := setting.GetMaxTokenAutoGroups()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMaxTokenAutoGroups)))

		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func createChannelSelectAutoGroupsChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func TestCacheGetRandomSatisfiedChannelUsesTokenAutoGroupsWhenGlobalAutoIsEmpty(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-groups-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2101, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2102, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "auto",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2101, first.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
	assert.Empty(t, setting.GetAutoGroups(), "the selection must not depend on the global Auto list")

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2102, second.Id)
	assert.Equal(t, "default", selectedGroup)
	assert.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

func TestCacheGetRandomSatisfiedChannelFallsBackToConfiguredGroups(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	originalFallbacks := common.GroupFallbackGroups
	t.Cleanup(func() { common.GroupFallbackGroups = originalFallbacks })
	// "secret" is not usable by the user, so it must be skipped.
	common.GroupFallbackGroups = map[string][]string{"default": {"secret", "vip", "default"}}

	const modelName = "fallback-groups-model"
	const vipOnlyModel = "fallback-groups-vip-only-model"
	createChannelSelectAutoGroupsChannel(t, db, 2201, "default", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2202, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2203, "secret", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2204, "vip", vipOnlyModel)
	model.InitChannelCache()
	gin.SetMode(gin.TestMode)

	newParam := func(modelName string) (*gin.Context, *RetryParam) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)
		return ctx, &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, Retry: common.GetPointer(0)}
	}

	// RetryTimes is 0: the own group gets one attempt, then the fallback one.
	ctx, param := newParam(modelName)
	first, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2201, first.Id)
	assert.Equal(t, "default", group)
	assert.Equal(t, 1, param.RemainingRetries(), "a pending fallback group must keep one retry")

	// The relay loop retries with its own RetryParam, not the distributor's.
	param = &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, Retry: common.GetPointer(0)}
	require.Equal(t, 1, param.RemainingRetries(), "a pending fallback group must keep one retry")
	param.IncreaseRetry()
	require.Equal(t, 0, param.GetRetry(), "the fallback group starts at its first priority")
	second, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2202, second.Id)
	assert.Equal(t, "vip", group)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup), "billing must use the fallback group")
	assert.Equal(t, 0, param.RemainingRetries(), "the last group must not switch again")

	// A group without a channel for the model goes straight to its fallback.
	_, param = newParam(vipOnlyModel)
	channel, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2204, channel.Id)
	assert.Equal(t, "vip", group)

	// With the token switch off, the request stays in its own group.
	ctx, param = newParam(modelName)
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, false)
	channel, group, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2201, channel.Id)
	assert.Equal(t, "default", group)
	assert.Equal(t, 0, param.RemainingRetries(), "no fallback group without the token switch")
	_, param = newParam(vipOnlyModel)
	common.SetContextKey(param.Ctx, constant.ContextKeyTokenCrossGroupRetry, false)
	channel, _, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	assert.Nil(t, channel, "a group without the model must not borrow a fallback group")
}

func TestCacheGetRandomSatisfiedChannelNetworkRetrySkipsTriedChannels(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	originalFallbacks := common.GroupFallbackGroups
	t.Cleanup(func() { common.GroupFallbackGroups = originalFallbacks })
	common.GroupFallbackGroups = map[string][]string{"default": {"vip"}}
	common.RetryTimes = 5

	const modelName = "network-retry-model"
	createChannelSelectAutoGroupsChannel(t, db, 2301, "default", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2302, "default", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2303, "default", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2304, "vip", modelName)
	// 2301 and 2302 share the top tier; 2303 is the lower tier.
	for _, id := range []int{2301, 2302} {
		require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", id).Update("priority", 10).Error)
		require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", id).Update("priority", 10).Error)
	}
	model.InitChannelCache()
	gin.SetMode(gin.TestMode)

	// next simulates a retry after the given channels failed with a network error.
	next := func(ctx *gin.Context, tokenGroup string, retry int, tried ...string) (int, string) {
		ctx.Set("use_channel", tried)
		common.SetContextKey(ctx, constant.ContextKeyNetworkRetry, true)
		param := &RetryParam{Ctx: ctx, TokenGroup: tokenGroup, ModelName: modelName, Retry: common.GetPointer(retry)}
		channel, group, err := CacheGetRandomSatisfiedChannel(param)
		require.NoError(t, err)
		if channel == nil {
			return 0, group
		}
		return channel.Id, group
	}
	newCtx := func(crossGroup bool) *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, crossGroup)
		return ctx
	}

	ctx := newCtx(true)
	id, group := next(ctx, "default", 1, "2301")
	assert.Equal(t, 2302, id, "the untried top-tier channel goes before the lower tier")
	assert.Equal(t, "default", group)
	id, _ = next(ctx, "default", 2, "2301", "2302")
	assert.Equal(t, 2303, id)
	id, group = next(ctx, "default", 3, "2301", "2302", "2303")
	assert.Equal(t, 2304, id, "an exhausted group moves on to its fallback group")
	assert.Equal(t, "vip", group)

	id, _ = next(newCtx(false), "default", 3, "2301", "2302", "2303")
	assert.Zero(t, id, "without cross-group retry the request stays in its group")

	ctx = newCtx(false)
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"default", "vip"})
	id, _ = next(ctx, "auto", 3, "2301", "2302", "2303")
	assert.Zero(t, id, "an auto token without cross-group retry stays in its group")
}
