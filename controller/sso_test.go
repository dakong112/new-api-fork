package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSOClientsOptionValidation(t *testing.T) {
	valid := `[{"client_id":"infinite","name":"Studio","secret_hash":"` + sha256Hex("s3cret") + `","redirect_uris":["https://www.example.com/sso/callback","http://localhost:3000/sso/callback"],"launch_url":"https://www.example.com/api/auth/newapi/start"}]`
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"valid", valid, true},
		{"empty list", `[]`, true},
		{"bad client id", `[{"client_id":"Bad Id","secret_hash":"` + sha256Hex("x") + `","redirect_uris":["https://a.com/cb"]}]`, false},
		{"duplicate client id", `[{"client_id":"a","secret_hash":"` + sha256Hex("x") + `","redirect_uris":["https://a.com/cb"]},{"client_id":"a","secret_hash":"` + sha256Hex("y") + `","redirect_uris":["https://a.com/cb"]}]`, false},
		{"secret hash not hex64", `[{"client_id":"a","secret_hash":"abc","redirect_uris":["https://a.com/cb"]}]`, false},
		{"plain http redirect", `[{"client_id":"a","secret_hash":"` + sha256Hex("x") + `","redirect_uris":["http://a.com/cb"]}]`, false},
		{"redirect with fragment", `[{"client_id":"a","secret_hash":"` + sha256Hex("x") + `","redirect_uris":["https://a.com/cb#x"]}]`, false},
		{"no redirect", `[{"client_id":"a","secret_hash":"` + sha256Hex("x") + `","redirect_uris":[]}]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := setting.ParseSSOClients(tc.value)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}

	clients, err := setting.ParseSSOClients(valid)
	require.NoError(t, err)
	client := clients[0]
	assert.True(t, client.AllowsRedirect("https://www.example.com/sso/callback"))
	assert.False(t, client.AllowsRedirect("https://www.example.com/sso/callback/"))
	assert.False(t, client.AllowsRedirect("https://evil.com/sso/callback"))
	assert.True(t, client.SecretMatches("s3cret"))
	assert.False(t, client.SecretMatches("S3cret"))
	assert.False(t, client.SecretMatches(""))
}

func sha256Hex(value string) string {
	return setting.SSOSecretHash(value)
}
