package model

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// VendorRule maps model names to a display vendor: a model belongs to the rule
// owning the longest keyword contained in its name (case-insensitive).
// Rule order is the display order.
type VendorRule struct {
	Name     string   `json:"name"`
	Icon     string   `json:"icon,omitempty"`
	Keywords []string `json:"keywords"`
}

const maxVendorRuleIconLength = 128

var (
	vendorRules     = DefaultVendorRules()
	vendorRulesLock sync.RWMutex
)

// DefaultVendorRules returns a fresh copy of the built-in rules.
func DefaultVendorRules() []VendorRule {
	return []VendorRule{
		{Name: "OpenAI", Icon: "OpenAI", Keywords: []string{"gpt", "chatgpt", "o1", "o3", "o4", "dall-e", "whisper", "tts-", "sora", "codex", "text-embedding"}},
		{Name: "Claude", Icon: "Claude.Color", Keywords: []string{"claude"}},
		{Name: "Gemini", Icon: "Gemini.Color", Keywords: []string{"gemini", "gemma", "imagen", "veo"}},
		{Name: "DeepSeek", Icon: "DeepSeek.Color", Keywords: []string{"deepseek"}},
		{Name: "千问", Icon: "Qwen.Color", Keywords: []string{"qwen", "qwq", "qvq", "wanx", "wan2"}},
		{Name: "豆包", Icon: "Doubao.Color", Keywords: []string{"doubao", "seedream", "seedance", "seededit"}},
		{Name: "智谱 GLM", Icon: "Zhipu.Color", Keywords: []string{"glm", "cogview", "cogvideo"}},
		{Name: "Kimi", Icon: "Kimi.Color", Keywords: []string{"kimi", "moonshot"}},
		{Name: "MiniMax", Icon: "Minimax.Color", Keywords: []string{"minimax", "abab", "hailuo"}},
		{Name: "小米 MiMo", Icon: "XiaomiMiMo", Keywords: []string{"mimo"}},
		{Name: "Grok", Icon: "Grok", Keywords: []string{"grok"}},
		{Name: "Llama", Icon: "Meta.Color", Keywords: []string{"llama"}},
		{Name: "Mistral", Icon: "Mistral.Color", Keywords: []string{"mistral", "mixtral", "codestral", "magistral", "devstral", "ministral", "pixtral"}},
		{Name: "文心", Icon: "Wenxin.Color", Keywords: []string{"ernie"}},
		{Name: "混元", Icon: "Hunyuan.Color", Keywords: []string{"hunyuan"}},
		{Name: "星火", Icon: "Spark.Color", Keywords: []string{"spark"}},
		{Name: "阶跃星辰", Icon: "Stepfun", Keywords: []string{"step-"}},
		{Name: "百川", Icon: "Baichuan.Color", Keywords: []string{"baichuan"}},
		{Name: "零一万物", Icon: "Yi.Color", Keywords: []string{"yi-"}},
		{Name: "LongCat", Icon: "LongCat.Color", Keywords: []string{"longcat"}},
		{Name: "可灵", Icon: "Kling.Color", Keywords: []string{"kling"}},
		{Name: "即梦", Icon: "Jimeng.Color", Keywords: []string{"jimeng"}},
		{Name: "Vidu", Icon: "Vidu.Color", Keywords: []string{"vidu"}},
		{Name: "Midjourney", Icon: "Midjourney", Keywords: []string{"midjourney", "mj_"}},
		{Name: "Flux", Icon: "Flux", Keywords: []string{"flux"}},
		{Name: "Suno", Icon: "Suno", Keywords: []string{"suno", "chirp"}},
		{Name: "Cohere", Icon: "Cohere.Color", Keywords: []string{"command-", "embed-"}},
		{Name: "Perplexity", Icon: "Perplexity.Color", Keywords: []string{"sonar"}},
		{Name: "Jina", Icon: "Jina", Keywords: []string{"jina"}},
		{Name: "360", Icon: "Ai360.Color", Keywords: []string{"360"}},
	}
}

// SetVendorRules replaces the active rules and reports whether they changed.
// Names and icons are trimmed for display; keywords are normalized by
// matchVendorRule.
func SetVendorRules(rules []VendorRule) bool {
	normalized := make([]VendorRule, 0, len(rules))
	for _, rule := range rules {
		normalized = append(normalized, VendorRule{Name: strings.TrimSpace(rule.Name), Icon: strings.TrimSpace(rule.Icon), Keywords: slices.Clone(rule.Keywords)})
	}
	vendorRulesLock.Lock()
	defer vendorRulesLock.Unlock()
	if reflect.DeepEqual(vendorRules, normalized) {
		return false
	}
	vendorRules = normalized
	return true
}

// currentVendorRules returns the active rules. SetVendorRules always swaps in a
// new slice and nothing mutates it in place, so callers may read it freely.
func currentVendorRules() []VendorRule {
	vendorRulesLock.RLock()
	defer vendorRulesLock.RUnlock()
	return vendorRules
}

// ValidateVendorRules checks a VendorRules option value. An empty value is
// valid and restores the built-in rules.
func ValidateVendorRules(value string) error {
	if value == "" {
		return nil
	}
	var rules []VendorRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return fmt.Errorf("VendorRules must be a JSON array of vendor rules: %w", err)
	}
	seen := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return fmt.Errorf("VendorRules[%d] must have a name", i)
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("VendorRules has duplicate vendor name %q", name)
		}
		seen[key] = struct{}{}
		if len(rule.Icon) > maxVendorRuleIconLength {
			return fmt.Errorf("VendorRules[%d] icon must be at most %d characters", i, maxVendorRuleIconLength)
		}
		if !slices.ContainsFunc(rule.Keywords, func(k string) bool { return strings.TrimSpace(k) != "" }) {
			return fmt.Errorf("VendorRules[%d] must have at least one keyword", i)
		}
	}
	return nil
}

// matchVendorRule returns the rule owning the longest keyword contained in
// modelName; on equal length the earlier rule wins.
func matchVendorRule(modelName string, rules []VendorRule) (index int, ok bool) {
	modelLower := strings.ToLower(modelName)
	best := 0
	for i, rule := range rules {
		for _, keyword := range rule.Keywords {
			keyword = strings.ToLower(strings.TrimSpace(keyword))
			if len(keyword) > best && strings.Contains(modelLower, keyword) {
				best, index, ok = len(keyword), i, true
			}
		}
	}
	return index, ok
}

func defaultVendorRulesJSON() string {
	data, _ := common.Marshal(DefaultVendorRules())
	return string(data)
}
