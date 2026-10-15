package setting

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// SSOClientDefaults are the group and models an application preselects for
// its users. Every field is optional and the models need not exist yet.
type SSOClientDefaults struct {
	Group string `json:"group"`
	Image string `json:"image"`
	Text  string `json:"text"`
	Video string `json:"video"`
	Audio string `json:"audio"`
}

// SSOClient is an application allowed to obtain one-time login codes for
// new-api users and exchange them server-to-server.
type SSOClient struct {
	ClientID     string            `json:"client_id"`
	Name         string            `json:"name"`
	SecretHash   string            `json:"secret_hash"`
	RedirectURIs []string          `json:"redirect_uris"`
	LaunchURL    string            `json:"launch_url"`
	Defaults     SSOClientDefaults `json:"defaults"`
	// App names the application whose permission codes ("canvas.template.read")
	// the grant carries. Empty means the application receives none.
	App string `json:"app,omitempty"`
	// AllowedUserIDs limits who may sign in to the application; empty admits
	// every user.
	AllowedUserIDs []int `json:"allowed_user_ids,omitempty"`
	// ConsumeMaxUSD caps one /api/sso/consume charge; 0 means
	// SSODefaultConsumeMaxUSD.
	ConsumeMaxUSD float64 `json:"consume_max_usd,omitempty"`
}

const (
	SSODefaultConsumeMaxUSD = 10
	ssoConsumeMaxUSDLimit   = 10000
	ssoAllowedUserIDsLimit  = 1000
)

var (
	ssoClients   = make([]SSOClient, 0)
	ssoClientsMu sync.RWMutex
)

var ssoClientIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,46}$`)
var ssoSecretHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var ssoAppPattern = regexp.MustCompile(`^[a-z]{2,32}$`)

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
		if client.App != "" && !ssoAppPattern.MatchString(client.App) {
			return nil, fmt.Errorf("SSOClients: app of %q must be 2-32 lowercase letters", client.ClientID)
		}
		if client.LaunchURL != "" && !isAllowedSSORedirect(client.LaunchURL) {
			return nil, fmt.Errorf("SSOClients: launch_url of %q must be https (or http://localhost)", client.ClientID)
		}
		for _, value := range []string{client.Defaults.Group, client.Defaults.Image, client.Defaults.Text, client.Defaults.Video, client.Defaults.Audio} {
			if len(value) > 128 {
				return nil, fmt.Errorf("SSOClients: defaults of %q must be at most 128 characters each", client.ClientID)
			}
		}
		if len(client.AllowedUserIDs) > ssoAllowedUserIDsLimit || slices.ContainsFunc(client.AllowedUserIDs, func(id int) bool { return id <= 0 }) {
			return nil, fmt.Errorf("SSOClients: allowed_user_ids of %q must be at most %d positive user ids", client.ClientID, ssoAllowedUserIDsLimit)
		}
		if math.IsNaN(client.ConsumeMaxUSD) || client.ConsumeMaxUSD < 0 || client.ConsumeMaxUSD > ssoConsumeMaxUSDLimit {
			return nil, fmt.Errorf("SSOClients: consume_max_usd of %q must be between 0 and %d", client.ClientID, ssoConsumeMaxUSDLimit)
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
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	// SSO_ALLOW_INSECURE_REDIRECTS lets IP-only test deployments register http callbacks; never enable it in production.
	return host == "localhost" || host == "127.0.0.1" || os.Getenv("SSO_ALLOW_INSECURE_REDIRECTS") == "true"
}

func UpdateSSOClientsByJsonString(value string) error {
	clients, err := ParseSSOClients(value)
	if err != nil {
		return err
	}
	ssoClientsMu.Lock()
	ssoClients = clients
	ssoClientsMu.Unlock()
	return nil
}

func SSOClients2JsonString() string {
	ssoClientsMu.RLock()
	data, err := common.Marshal(ssoClients)
	ssoClientsMu.RUnlock()
	if err != nil {
		return "[]"
	}
	return string(data)
}

// GetSSOClients returns a copy of the registered applications.
func GetSSOClients() []SSOClient {
	ssoClientsMu.RLock()
	defer ssoClientsMu.RUnlock()
	return slices.Clone(ssoClients)
}

func FindSSOClient(clientID string) (SSOClient, bool) {
	ssoClientsMu.RLock()
	defer ssoClientsMu.RUnlock()
	for _, client := range ssoClients {
		if client.ClientID == clientID {
			return client, true
		}
	}
	return SSOClient{}, false
}

func (client SSOClient) AllowsRedirect(redirectURI string) bool {
	return slices.Contains(client.RedirectURIs, redirectURI)
}

// AllowsUser reports whether userID may sign in to the application.
func (client SSOClient) AllowsUser(userID int) bool {
	return len(client.AllowedUserIDs) == 0 || slices.Contains(client.AllowedUserIDs, userID)
}

// ConsumeMaxQuota is the largest quota one /api/sso/consume call may charge,
// kept within the single-request quota bound.
func (client SSOClient) ConsumeMaxQuota() int {
	maxUSD := client.ConsumeMaxUSD
	if maxUSD == 0 {
		maxUSD = SSODefaultConsumeMaxUSD
	}
	return common.QuotaFromFloat(min(maxUSD*common.QuotaPerUnit, common.MaxQuota))
}

func (client SSOClient) SecretMatches(secret string) bool {
	if secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(SSOSecretHash(secret)), []byte(client.SecretHash)) == 1
}

// IsSSOClientOrigin reports whether origin (scheme://host[:port]) is the origin
// of a redirect_uri of any registered application.
func IsSSOClientOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	ssoClientsMu.RLock()
	defer ssoClientsMu.RUnlock()
	for _, client := range ssoClients {
		for _, redirectURI := range client.RedirectURIs {
			parsed, err := url.Parse(redirectURI)
			if err == nil && parsed.Scheme+"://"+parsed.Host == origin {
				return true
			}
		}
	}
	return false
}
