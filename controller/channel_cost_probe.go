package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Cost probe: sends the same long prompt prefix to one channel several times
// and reports the usage the supplier returns for each round, so admins can see
// whether its prompt cache works. In candy mode each round also asks a fixed
// reasoning question, so a downgraded or swapped model shows up as a wrong
// answer or unusually few reasoning tokens. It talks to the supplier directly
// and records nothing in local billing.

const (
	costProbeMaxRounds        = 10
	costProbeRequestTimeout   = 10 * time.Minute
	costProbeMaxResponseSize  = 1 << 20
	costProbeDefaultPrefix    = 6000
	costProbeMinPrefix        = 1000 // above every provider's minimum cacheable prefix
	costProbeMaxPrefix        = 200000
	costProbeTokensPerClause  = 23 // measured: 280 clauses = 6,508 DeepSeek tokens
	costProbeMaxAnswerRunes   = 2000
	costProbeCandyClaudeLimit = 16000
)

// Candy question from github.com/haowang02/codex-candy-eval; the answer is 21.
const costProbeCandyQuestion = `不使用任何外部工具回答以下问题：

在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

        苹果味  桃子味  西瓜味
圆形       7      9      8
五角星形   7      6      4
`

var costProbeCandyAnswer = regexp.MustCompile(`(?:^|\D)21(?:\D|$)`)

var costProbeEfforts = []string{"", "minimal", "low", "medium", "high", "xhigh"}

var costProbeRunID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

type costProbeRequest struct {
	ChannelId       int    `json:"channel_id"`
	Model           string `json:"model"`
	Rounds          int    `json:"rounds"`
	Mode            string `json:"mode"` // "cache" (default) or "candy"
	PrefixTokens    int    `json:"prefix_tokens"`
	ReasoningEffort string `json:"reasoning_effort"`
	// RunID lets the page run one round per call and still send the same
	// prefix every round; StartRound numbers those rounds. Both optional.
	RunID      string `json:"run_id"`
	StartRound int    `json:"start_round"`
}

type costProbeRound struct {
	InputTokens      int    `json:"input_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	ReasoningTokens  int    `json:"reasoning_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	Answer           string `json:"answer,omitempty"`
	Correct          *bool  `json:"correct,omitempty"`
	RawUsage         string `json:"raw_usage,omitempty"`
	Error            string `json:"error,omitempty"`
}

type costProbeResult struct {
	Protocol      string           `json:"protocol"`
	UpstreamModel string           `json:"upstream_model"`
	Rounds        []costProbeRound `json:"rounds"`
}

// costProbeTarget is everything a round needs to call one supplier.
type costProbeTarget struct {
	client          *http.Client
	baseURL         string
	key             string
	model           string
	isClaude        bool
	candy           bool
	reasoningEffort string
}

func ProbeChannelCost(c *gin.Context) {
	var req costProbeRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.PrefixTokens == 0 {
		req.PrefixTokens = costProbeDefaultPrefix
	}
	if req.StartRound == 0 {
		req.StartRound = 1
	}
	if req.RunID == "" {
		// A random run id at the very start keeps each run cold, so round one
		// measures a cache write and later rounds measure cache reads.
		nonce := make([]byte, 8)
		_, _ = rand.Read(nonce)
		req.RunID = hex.EncodeToString(nonce)
	}
	if req.Model == "" || req.Rounds < 1 || req.Rounds > costProbeMaxRounds ||
		req.PrefixTokens < costProbeMinPrefix || req.PrefixTokens > costProbeMaxPrefix ||
		(req.Mode != "" && req.Mode != "cache" && req.Mode != "candy") ||
		!slices.Contains(costProbeEfforts, req.ReasoningEffort) ||
		!costProbeRunID.MatchString(req.RunID) || req.StartRound < 1 || req.StartRound > costProbeMaxRounds {
		common.ApiErrorMsg(c, fmt.Sprintf("invalid probe: model required, rounds 1-%d, prefix_tokens %d-%d, mode cache|candy",
			costProbeMaxRounds, costProbeMinPrefix, costProbeMaxPrefix))
		return
	}
	channel, err := model.GetChannelById(req.ChannelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keys := channel.GetKeys()
	if len(keys) == 0 || strings.TrimSpace(keys[0]) == "" {
		common.ApiErrorMsg(c, "channel has no key")
		return
	}
	target := costProbeTarget{
		key:             strings.TrimSpace(keys[0]),
		baseURL:         strings.TrimRight(channel.GetBaseURL(), "/"),
		model:           req.Model,
		isClaude:        channel.Type == constant.ChannelTypeAnthropic,
		candy:           req.Mode == "candy",
		reasoningEffort: req.ReasoningEffort,
	}
	if target.baseURL == "" {
		target.baseURL = strings.TrimRight(constant.GetChannelBaseURL(channel.Type), "/")
	}
	target.client, err = service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	mapping := map[string]string{}
	if raw := channel.GetModelMapping(); raw != "" && common.UnmarshalJsonStr(raw, &mapping) == nil && mapping[req.Model] != "" {
		target.model = mapping[req.Model]
	}

	result := costProbeResult{Protocol: "openai", UpstreamModel: target.model}
	if target.isClaude {
		result.Protocol = "anthropic"
	}

	prefix := buildCostProbePrefix(req.RunID, req.PrefixTokens)
	for i := range req.Rounds {
		result.Rounds = append(result.Rounds, runCostProbeRound(c.Request.Context(), target, prefix, req.StartRound+i))
	}
	common.ApiSuccess(c, result)
}

func buildCostProbePrefix(runID string, tokens int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run id: %s\nYou are auditing a service agreement. Read every clause below.\n", runID)
	for i := range tokens / costProbeTokensPerClause {
		fmt.Fprintf(&b, "Clause %d: The supplier must report token usage accurately and bill cached prompt tokens at the discounted cache rate.\n", i+1)
	}
	return b.String()
}

func runCostProbeRound(ctx context.Context, target costProbeTarget, prefix string, roundNo int) costProbeRound {
	question := fmt.Sprintf("Round %d: reply with the single word OK.", roundNo)
	if target.candy {
		question = costProbeCandyQuestion
	}
	var url string
	var body map[string]any
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	if target.isClaude {
		url = target.baseURL + "/v1/messages"
		header.Set("x-api-key", target.key)
		header.Set("anthropic-version", "2023-06-01")
		maxTokens := 16
		if target.candy {
			maxTokens = costProbeCandyClaudeLimit
		}
		body = map[string]any{
			"model":      target.model,
			"max_tokens": maxTokens,
			"system": []map[string]any{{
				"type":          "text",
				"text":          prefix,
				"cache_control": map[string]string{"type": "ephemeral"},
			}},
			"messages": []map[string]string{{"role": "user", "content": question}},
		}
	} else {
		url = target.baseURL + "/v1/chat/completions"
		header.Set("Authorization", "Bearer "+target.key)
		body = map[string]any{
			"model": target.model,
			"messages": []map[string]string{
				{"role": "system", "content": prefix},
				{"role": "user", "content": question},
			},
		}
		// Reasoning models reject max_tokens, so candy mode leaves the output
		// length to the model default.
		if !target.candy {
			body["max_tokens"] = 16
		}
		if target.reasoningEffort != "" {
			body["reasoning_effort"] = target.reasoningEffort
		}
	}
	payload, err := common.Marshal(body)
	if err != nil {
		return costProbeRound{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, costProbeRequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return costProbeRound{Error: err.Error()}
	}
	httpReq.Header = header
	start := time.Now()
	resp, err := target.client.Do(httpReq)
	if err != nil {
		return costProbeRound{Error: sanitizeCostProbeError(err.Error(), target.key)}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, costProbeMaxResponseSize))
	round := costProbeRound{LatencyMs: time.Since(start).Milliseconds()}
	if err != nil {
		round.Error = sanitizeCostProbeError(err.Error(), target.key)
		return round
	}
	if resp.StatusCode != http.StatusOK {
		msg := detectErrorMessageFromJSONBytes(respBody)
		if msg == "" {
			msg = string(respBody)
		}
		if r := []rune(msg); len(r) > 300 {
			msg = string(r[:300])
		}
		round.Error = sanitizeCostProbeError(fmt.Sprintf("status %d: %s", resp.StatusCode, msg), target.key)
		return round
	}
	usage := gjson.GetBytes(respBody, "usage")
	if !usage.Exists() {
		round.Error = "response has no usage"
		return round
	}
	round.RawUsage = usage.Raw
	if target.isClaude {
		// Anthropic input_tokens already excludes cache reads and writes.
		round.InputTokens = int(usage.Get("input_tokens").Int())
		round.CacheReadTokens = int(usage.Get("cache_read_input_tokens").Int())
		round.CacheWriteTokens = int(usage.Get("cache_creation_input_tokens").Int())
		round.OutputTokens = int(usage.Get("output_tokens").Int())
		var text strings.Builder
		for _, block := range gjson.GetBytes(respBody, "content").Array() {
			if block.Get("type").String() == "text" {
				text.WriteString(block.Get("text").String())
			}
		}
		round.Answer = text.String()
	} else {
		// OpenAI-style prompt_tokens includes cached tokens; DeepSeek reports hits separately.
		cacheRead := usage.Get("prompt_tokens_details.cached_tokens").Int()
		if cacheRead == 0 {
			cacheRead = usage.Get("prompt_cache_hit_tokens").Int()
		}
		cacheWrite := usage.Get("prompt_tokens_details.cached_creation_tokens").Int()
		round.CacheReadTokens = int(cacheRead)
		round.CacheWriteTokens = int(cacheWrite)
		round.InputTokens = int(max(usage.Get("prompt_tokens").Int()-cacheRead-cacheWrite, 0))
		round.OutputTokens = int(usage.Get("completion_tokens").Int())
		round.ReasoningTokens = int(usage.Get("completion_tokens_details.reasoning_tokens").Int())
		round.Answer = gjson.GetBytes(respBody, "choices.0.message.content").String()
	}
	if target.candy {
		correct := costProbeCandyAnswer.MatchString(round.Answer)
		round.Correct = &correct
	}
	if r := []rune(round.Answer); len(r) > costProbeMaxAnswerRunes {
		// Keep the tail: the final answer is usually stated last.
		round.Answer = "…" + string(r[len(r)-costProbeMaxAnswerRunes:])
	}
	return round
}

func sanitizeCostProbeError(msg, key string) string {
	if key == "" {
		return msg
	}
	return strings.ReplaceAll(msg, key, "***")
}
