package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMatchVendorRuleDefaults(t *testing.T) {
	rules := DefaultVendorRules()
	for modelName, want := range map[string]string{
		"doubao-seedream-4":     "豆包",
		"gpt-4o":                "OpenAI",
		"GPT-4O-MINI":           "OpenAI",
		"sora-2":                "OpenAI",
		"claude-sonnet-5":       "Claude",
		"kimi-k3":               "Kimi",
		"yi-large":              "零一万物",
		"mimo-v2.5":             "小米 MiMo",
		"glm-5.3":               "智谱 GLM",
		"qwen3-max":             "千问",
		"deepseek-v4-flash":     "DeepSeek",
		"gemini-3-pro":          "Gemini",
		"step-3":                "阶跃星辰",
		"mistral-large-latest":  "Mistral",
		"chatgpt-deepseek-tune": "DeepSeek", // longest keyword wins over gpt/chatgpt
	} {
		index, ok := matchVendorRule(modelName, rules)
		require.True(t, ok, modelName)
		assert.Equal(t, want, rules[index].Name, modelName)
	}
	_, ok := matchVendorRule("acme-unknown-1", rules)
	assert.False(t, ok)
}

func TestMatchVendorRuleTieKeepsRuleOrder(t *testing.T) {
	rules := []VendorRule{{Name: "A", Keywords: []string{"abc"}}, {Name: "B", Keywords: []string{"xyz"}}}
	index, ok := matchVendorRule("xyz-abc", rules)
	require.True(t, ok)
	assert.Equal(t, 0, index)
}

func TestValidateVendorRules(t *testing.T) {
	for name, value := range map[string]string{
		"not json":        `{`,
		"empty name":      `[{"name":" ","keywords":["a"]}]`,
		"duplicate name":  `[{"name":"Acme","keywords":["a"]},{"name":"acme ","keywords":["b"]}]`,
		"no keywords":     `[{"name":"Acme","keywords":[]}]`,
		"blank keywords":  `[{"name":"Acme","keywords":[" ",""]}]`,
		"icon too long":   `[{"name":"Acme","icon":"` + strings.Repeat("x", 129) + `","keywords":["a"]}]`,
		"missing keyword": `[{"name":"Acme"}]`,
	} {
		assert.Error(t, ValidateVendorRules(value), name)
	}
	assert.NoError(t, ValidateVendorRules(`[{"name":"Acme","icon":"OpenAI","keywords":["acme"]}]`))
	assert.NoError(t, ValidateVendorRules(""))
	defaults, err := common.Marshal(DefaultVendorRules())
	require.NoError(t, err)
	assert.NoError(t, ValidateVendorRules(string(defaults)))
}

func TestVendorRulesOptionDrivesPricingAndRestoresDefaults(t *testing.T) {
	resetPricingEndpointTestTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		SetVendorRules(DefaultVendorRules())
		common.OptionMap = previousMap
		InvalidatePricingCache()
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Option{}).Error)
	})
	insertPricingEndpointChannel(t, 1, 1, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 1, "acme-chat-1")
	insertPricingEndpointAbility(t, 1, "gpt-5")

	vendorOf := func(modelName string) (string, string) {
		vendors := map[int]PricingVendor{}
		for _, v := range GetVendors() {
			vendors[v.ID] = v
		}
		for _, p := range GetPricing() {
			if p.ModelName == modelName {
				v := vendors[p.VendorID]
				return v.Name, v.Icon
			}
		}
		t.Fatalf("model %s not in pricing", modelName)
		return "", ""
	}
	name, icon := vendorOf("gpt-5")
	assert.Equal(t, "OpenAI", name)
	assert.Equal(t, "OpenAI", icon)
	name, _ = vendorOf("acme-chat-1")
	assert.Empty(t, name)

	require.NoError(t, UpdateOption("VendorRules", `[{"name":"Acme","icon":"Acme.Color","keywords":["ACME"]}]`))
	name, icon = vendorOf("acme-chat-1")
	assert.Equal(t, "Acme", name)
	assert.Equal(t, "Acme.Color", icon)
	name, _ = vendorOf("gpt-5")
	assert.Empty(t, name)

	require.Error(t, UpdateOption("VendorRules", `[{"name":"Acme","keywords":[]}]`))

	// "" restores the built-in list, now and after a restart.
	require.NoError(t, UpdateOption("VendorRules", ""))
	defaults, err := common.Marshal(DefaultVendorRules())
	require.NoError(t, err)
	assert.Equal(t, string(defaults), common.OptionMap["VendorRules"])
	name, _ = vendorOf("gpt-5")
	assert.Equal(t, "OpenAI", name)

	SetVendorRules([]VendorRule{{Name: "Stale", Keywords: []string{"gpt"}}})
	InvalidatePricingCache()
	name, _ = vendorOf("gpt-5")
	require.Equal(t, "Stale", name)
	loadOptionsFromDatabase() // must invalidate pricing by itself when rules change
	name, _ = vendorOf("gpt-5")
	assert.Equal(t, "OpenAI", name)
}

func TestSavedVendorRulesDoNotInheritDefaults(t *testing.T) {
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		SetVendorRules(DefaultVendorRules())
		common.OptionMap = previousMap
		InvalidatePricingCache()
	})
	require.NoError(t, updateOptionMap("VendorRules", `[{"name":"Acme","keywords":["acme"]}]`))
	assert.Equal(t, []VendorRule{{Name: "Acme", Keywords: []string{"acme"}}}, currentVendorRules())
}

func TestDatabaseVendorTakesPrecedenceOverRule(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 1, 1, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 1, "claude-sonnet-5")
	saved := Vendor{Name: "claude", Status: 1}
	require.NoError(t, DB.Create(&saved).Error)
	InvalidatePricingCache()

	GetPricing()
	vendors := GetVendors()
	require.Len(t, vendors, 1)
	assert.Equal(t, saved.Id, vendors[0].ID)
	assert.Equal(t, "claude", vendors[0].Name)
	assert.Equal(t, "Claude.Color", vendors[0].Icon, "empty DB icon falls back to the rule icon")
	var stored Vendor
	require.NoError(t, DB.First(&stored, saved.Id).Error)
	assert.Empty(t, stored.Icon, "display fallback is not written back")
}

func TestDatabaseVendorWithPreviousDefaultNameKeepsItsModels(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 1, 1, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 1, "gemini-3-pro")
	saved := Vendor{Name: "Google", Status: 1}
	require.NoError(t, DB.Create(&saved).Error)
	InvalidatePricingCache()

	GetPricing()
	vendors := GetVendors()
	require.Len(t, vendors, 1, "no second Gemini vendor next to Google")
	assert.Equal(t, saved.Id, vendors[0].ID)
	assert.Equal(t, "Google", vendors[0].Name)
}
