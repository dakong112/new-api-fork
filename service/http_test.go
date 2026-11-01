package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// A relayed response carries only the headers a client needs; the rest would
// identify the upstream behind this site.
func TestIOCopyBytesGracefullyForwardsOnlyClientHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	upstream := &http.Response{StatusCode: http.StatusOK, Header: http.Header{
		"Content-Type":          {"application/json"},
		"Retry-After":           {"3"},
		"Retry-After-Ms":        {"3000"},
		"X-Should-Retry":        {"true"},
		"X-Codex-Turn-State":    {"turn"},
		"Server":                {"openresty"},
		"Set-Cookie":            {"sid=1; Domain=reseller.example"},
		"Cf-Ray":                {"8a1b-LAX"},
		"Openai-Organization":   {"reseller-org"},
		"X-New-Api-Version":     {"v0.9.0"},
		"X-Ratelimit-Remaining": {"10"},
		common.RequestIdKey:     {"upstream-request"},
	}}

	IOCopyBytesGracefully(c, upstream, []byte(`{}`))

	header := recorder.Header()
	assert.Equal(t, "application/json", header.Get("Content-Type"))
	assert.Equal(t, "3", header.Get("Retry-After"))
	assert.Equal(t, "3000", header.Get("Retry-After-Ms"))
	assert.Equal(t, "true", header.Get("X-Should-Retry"))
	assert.Equal(t, "turn", header.Get("X-Codex-Turn-State"))
	for _, name := range []string{"Server", "Set-Cookie", "Cf-Ray", "Openai-Organization", "X-New-Api-Version", "X-Ratelimit-Remaining", common.RequestIdKey} {
		assert.Empty(t, header.Get(name), name)
	}
	assert.Equal(t, "upstream-request", c.GetString(common.UpstreamRequestIdKey))
}
