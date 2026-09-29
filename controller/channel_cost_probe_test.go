package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func probeTarget(server *httptest.Server, key string, isClaude, candy bool) costProbeTarget {
	return costProbeTarget{client: server.Client(), baseURL: server.URL, key: key, model: "m", isClaude: isClaude, candy: candy}
}

func TestRunCostProbeRoundNormalizesUsage(t *testing.T) {
	cases := []struct {
		name     string
		isClaude bool
		path     string
		body     string
		want     costProbeRound
	}{
		{
			name:     "anthropic input excludes cache",
			isClaude: true,
			path:     "/v1/messages",
			body:     `{"content":[{"type":"text","text":"OK"}],"usage":{"input_tokens":20,"cache_read_input_tokens":6000,"cache_creation_input_tokens":0,"output_tokens":3}}`,
			want:     costProbeRound{InputTokens: 20, CacheReadTokens: 6000, OutputTokens: 3, Answer: "OK"},
		},
		{
			name: "openai prompt tokens include cached and creation",
			path: "/v1/chat/completions",
			body: `{"choices":[{"message":{"content":"OK"}}],"usage":{"prompt_tokens":6020,"completion_tokens":3,"prompt_tokens_details":{"cached_tokens":5000,"cached_creation_tokens":1000}}}`,
			want: costProbeRound{InputTokens: 20, CacheReadTokens: 5000, CacheWriteTokens: 1000, OutputTokens: 3, Answer: "OK"},
		},
		{
			name: "deepseek cache hit field and reasoning tokens",
			path: "/v1/chat/completions",
			body: `{"usage":{"prompt_tokens":6020,"completion_tokens":3,"prompt_cache_hit_tokens":5888,"completion_tokens_details":{"reasoning_tokens":2}}}`,
			want: costProbeRound{InputTokens: 132, CacheReadTokens: 5888, OutputTokens: 3, ReasoningTokens: 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotKey string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotKey = r.Header.Get("x-api-key") + r.Header.Get("Authorization")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got := runCostProbeRound(context.Background(), probeTarget(server, "sk-test", tc.isClaude, false), "prefix", 1)

			assert.Equal(t, tc.path, gotPath)
			assert.Contains(t, gotKey, "sk-test")
			assert.Empty(t, got.Error)
			got.LatencyMs, got.RawUsage = 0, ""
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunCostProbeRoundGradesCandyAnswer(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{answer: "最少需要取出 21 个糖果。", want: true},
		{answer: "答案是21", want: true},
		{answer: "答案是 121 个", want: false},
		{answer: "至少 20 个", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.answer, func(t *testing.T) {
			var sent []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sent, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + tc.answer + `"}}],"usage":{"prompt_tokens":100,"completion_tokens":900,"completion_tokens_details":{"reasoning_tokens":880}}}`))
			}))
			defer server.Close()

			got := runCostProbeRound(context.Background(), probeTarget(server, "sk-test", false, true), "prefix", 1)

			require.NotNil(t, got.Correct)
			assert.Equal(t, tc.want, *got.Correct)
			assert.Equal(t, 880, got.ReasoningTokens)
			// Reasoning models reject max_tokens, so candy mode must not send it.
			assert.False(t, gjson.GetBytes(sent, "max_tokens").Exists())
			assert.Contains(t, gjson.GetBytes(sent, "messages.1.content").String(), "糖果")
		})
	}
}

func TestBuildCostProbePrefixScalesWithTokens(t *testing.T) {
	small := buildCostProbePrefix("id", 6000)
	large := buildCostProbePrefix("id", 100000)

	// ~4.5 characters per token for the clause text.
	assert.InDelta(t, 100000.0/6000.0, float64(len(large))/float64(len(small)), 0.5)
}

func TestRunCostProbeRoundHidesKeyInUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key sk-secret"}}`))
	}))
	defer server.Close()

	got := runCostProbeRound(context.Background(), probeTarget(server, "sk-secret", false, false), "prefix", 1)

	assert.Equal(t, "status 401: invalid key ***", got.Error)
}
