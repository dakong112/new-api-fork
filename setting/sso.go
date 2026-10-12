package setting

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// SSOClient is an application allowed to obtain one-time login codes for
// new-api users and exchange them server-to-server.
type SSOClient struct {
	ClientID     string   `json:"client_id"`
	Name         string   `json:"name"`
	SecretHash   string   `json:"secret_hash"`
	RedirectURIs []string `json:"redirect_uris"`
	LaunchURL    string   `json:"launch_url"`
}

var SSOClients = make([]SSOClient, 0)

var ssoClientIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,56}$`)
var ssoSecretHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func SSOSecretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ParseSSOClients decodes and validates the SSOClients option value.
func ParseSSOClients(value string) ([]SSOClient, error) {
	clients := make([]SSOClient, 0)
	if strings.TrimSpace(value) == "" {
		return clients, nil
	}
	if err := common.UnmarshalJsonStr(value, &clients); err != nil {
		return nil, fmt.Errorf("SSOClients must be a JSON array: %w", err)
	}
	seen := make(map[string]bool, len(clients))
	for _, client := range clients {
		if !ssoClientIDPattern.MatchString(client.ClientID) {
			return nil, fmt.Errorf("SSOClients: invalid client_id %q", client.ClientID)
		}
		if seen[client.ClientID] {
			return nil, fmt.Errorf("SSOClients: duplicate client_id %q", client.ClientID)
		}
		seen[client.ClientID] = true
		if !ssoSecretHashPattern.MatchString(client.SecretHash) {
			return nil, fmt.Errorf("SSOClients: secret_hash of %q must be lowercase sha256 hex", client.ClientID)
		}
		if len(client.RedirectURIs) == 0 {
			return nil, fmt.Errorf("SSOClients: %q needs at least one redirect_uri", client.ClientID)
		}
		for _, redirectURI := range client.RedirectURIs {
			if !isAllowedSSORedirect(redirectURI) {
				return nil, fmt.Errorf("SSOClients: redirect_uri %q of %q must be https (or http://localhost) without fragment", redirectURI, client.ClientID)
			}
		}
		if client.LaunchURL != "" && !isAllowedSSORedirect(client.LaunchURL) {
			return nil, fmt.Errorf("SSOClients: launch_url of %q must be https (or http://localhost)", client.ClientID)
		}
	}
	return clients, nil
}

func isAllowedSSORedirect(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	host := parsed.Hostname()
	return parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1")
}

func UpdateSSOClientsByJsonString(value string) error {
	clients, err := ParseSSOClients(value)
	if err != nil {
		return err
	}
	SSOClients = clients
	return nil
}

func SSOClients2JsonString() string {
	data, err := common.Marshal(SSOClients)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func FindSSOClient(clientID string) (SSOClient, bool) {
	for _, client := range SSOClients {
		if client.ClientID == clientID {
			return client, true
		}
	}
	return SSOClient{}, false
}

func (client SSOClient) AllowsRedirect(redirectURI string) bool {
	return slices.Contains(client.RedirectURIs, redirectURI)
}

func (client SSOClient) SecretMatches(secret string) bool {
	if secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(SSOSecretHash(secret)), []byte(client.SecretHash)) == 1
}
