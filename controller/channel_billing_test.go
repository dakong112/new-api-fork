package controller

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDeepSeekBalanceUSD(t *testing.T) {
	tests := []struct {
		name            string
		responseJSON    string
		usdExchangeRate float64
		want            float64
		wantErrContains string
	}{
		{
			name:            "prefers USD when USD precedes CNY",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"12.50"},{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			want:            12.5,
		},
		{
			name:            "prefers USD when CNY precedes USD",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"},{"currency":"USD","total_balance":"12.50"}]}`,
			usdExchangeRate: 7.3,
			want:            12.5,
		},
		{
			name:            "converts CNY when USD is absent",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			want:            10,
		},
		{
			name:            "returns error when USD and CNY are absent",
			responseJSON:    `{"balance_infos":[{"currency":"EUR","total_balance":"10.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "currency USD or CNY not found",
		},
		{
			name:            "returns USD parse error instead of falling back to CNY",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"invalid"},{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "invalid syntax",
		},
		{
			name:            "rejects NaN USD balance",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"NaN"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "USD balance must be finite",
		},
		{
			name:            "rejects negative USD balance",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"-1.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "USD balance must be non-negative",
		},
		{
			name:            "rejects positive infinity CNY balance",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"+Inf"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "CNY balance must be finite",
		},
		{
			name:            "rejects negative CNY balance",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"-7.30"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "CNY balance must be non-negative",
		},
		{
			name:            "returns error for non-positive CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 0,
			wantErrContains: "USD exchange rate must be greater than zero",
		},
		{
			name:            "rejects NaN CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: math.NaN(),
			wantErrContains: "USD exchange rate must be finite",
		},
		{
			name:            "rejects positive infinity CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: math.Inf(1),
			wantErrContains: "USD exchange rate must be finite",
		},
		{
			name:            "rejects CNY conversion overflow",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"1.7976931348623157e+308"}]}`,
			usdExchangeRate: math.SmallestNonzeroFloat64,
			wantErrContains: "converted USD balance must be finite",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response DeepSeekUsageResponse
			require.NoError(t, common.Unmarshal([]byte(test.responseJSON), &response))

			balance, err := getDeepSeekBalanceUSD(response, test.usdExchangeRate)
			if test.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.InDelta(t, test.want, balance, 1e-12)
		})
	}
}

func TestGetSub2APIBalanceUSD(t *testing.T) {
	tests := []struct {
		name            string
		responseJSON    string
		want            float64
		wantErrContains string
	}{
		{name: "uses remaining in USD", responseJSON: `{"isValid":true,"mode":"unrestricted","remaining":20.21,"balance":25,"unit":"USD"}`, want: 20.21},
		{name: "falls back to balance without remaining", responseJSON: `{"isValid":true,"balance":1.06,"unit":"USD"}`, want: 1.06},
		{name: "keeps a zero balance", responseJSON: `{"isValid":true,"remaining":0,"unit":"USD"}`, want: 0},
		{name: "rejects an invalid key", responseJSON: `{"isValid":false,"remaining":5,"unit":"USD"}`, wantErrContains: "invalid"},
		{name: "rejects a non USD unit", responseJSON: `{"isValid":true,"remaining":5,"unit":"CNY"}`, wantErrContains: "not USD"},
		{name: "rejects a response without balance", responseJSON: `{"isValid":true,"unit":"USD"}`, wantErrContains: "no remaining balance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response Sub2APIUsageResponse
			require.NoError(t, common.UnmarshalJsonStr(tt.responseJSON, &response))
			got, err := getSub2APIBalanceUSD(response)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 1e-9)
		})
	}
}

func TestGetNewAPIBalanceUSD(t *testing.T) {
	tests := []struct {
		name            string
		hardLimit       float64
		totalUsage      float64
		want            float64
		wantErrContains string
	}{
		{name: "subtracts used quota from the total", hardLimit: 30, totalUsage: 1250, want: 17.5},
		{name: "reports an exhausted token as zero", hardLimit: 12, totalUsage: 1200, want: 0},
		{name: "rejects an unlimited token", hardLimit: newAPIUnlimitedQuotaUSD, totalUsage: 500, wantErrContains: "unlimited quota"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNewAPIBalanceUSD(OpenAISubscriptionResponse{HardLimitUSD: tt.hardLimit}, OpenAIUsageResponse{TotalUsage: tt.totalUsage})
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 1e-9)
		})
	}
}

func TestFetchUpstreamBalanceRequests(t *testing.T) {
	type seen struct{ path, auth, userAgent string }
	var requests []seen
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, seen{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("User-Agent")})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"isValid":true,"remaining":3.5,"unit":"USD"}`))
		case "/v1/dashboard/billing/subscription":
			_, _ = w.Write([]byte(`{"hard_limit_usd":10}`))
		case "/v1/dashboard/billing/usage":
			_, _ = w.Write([]byte(`{"total_usage":250}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	for _, base := range []string{upstream.URL, upstream.URL + "/v1/"} {
		requests = nil
		channel := &model.Channel{Key: "sk-upstream", BaseURL: common.GetPointer(base)}

		balance, err := fetchSub2APIBalanceUSD(channel)
		require.NoError(t, err, base)
		assert.InDelta(t, 3.5, balance, 1e-9)

		balance, err = fetchNewAPIBalanceUSD(channel)
		require.NoError(t, err, base)
		assert.InDelta(t, 7.5, balance, 1e-9)

		require.Len(t, requests, 3, base)
		assert.Equal(t, []string{"/v1/usage", "/v1/dashboard/billing/subscription", "/v1/dashboard/billing/usage"},
			[]string{requests[0].path, requests[1].path, requests[2].path}, "a base URL ending in /v1 must not double it")
		for _, request := range requests {
			assert.Equal(t, "Bearer sk-upstream", request.auth)
			assert.NotEmpty(t, request.userAgent, "Cloudflare-fronted upstreams reject requests without a User-Agent")
		}
	}
}

func TestShouldAlertLowBalanceOncePerLowPeriod(t *testing.T) {
	const channelID = 987654
	t.Cleanup(func() { lowBalanceAlerted.Delete(channelID) })
	steps := []struct {
		name      string
		balance   float64
		threshold float64
		want      bool
	}{
		{name: "above the threshold stays quiet", balance: 12, threshold: 5, want: false},
		{name: "first refresh below the threshold alerts", balance: 3, threshold: 5, want: true},
		{name: "a later refresh still below the threshold does not repeat", balance: 2.5, threshold: 5, want: false},
		{name: "recovering to the threshold resets the alert", balance: 5, threshold: 5, want: false},
		{name: "dropping again alerts again", balance: 4, threshold: 5, want: true},
		{name: "an empty balance is left to auto-disable", balance: 0, threshold: 5, want: false},
		{name: "a positive balance after auto-disable alerts again", balance: 1, threshold: 5, want: true},
		{name: "a zero threshold turns the alert off", balance: 1, threshold: 0, want: false},
	}
	for _, step := range steps {
		assert.Equal(t, step.want, shouldAlertLowBalance(channelID, step.balance, step.threshold), step.name)
	}
}
