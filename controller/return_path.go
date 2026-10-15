package controller

import (
	"net/url"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func paymentReturnPath(suffix string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return base + suffix
}

// ssoClientReturnURL returns requested when its origin (scheme+host+port)
// matches a redirect_uri or launch_url origin of a registered SSO client,
// otherwise fallback. It keeps the payment return link from becoming an open
// redirect: only the browser landing page changes, never notify URLs.
func ssoClientReturnURL(requested string, fallback string) string {
	target, err := url.Parse(requested)
	if requested == "" || err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Host == "" || target.User != nil {
		return fallback
	}
	for _, client := range setting.GetSSOClients() {
		for _, registered := range append(slices.Clone(client.RedirectURIs), client.LaunchURL) {
			origin, err := url.Parse(registered)
			if registered != "" && err == nil && origin.Scheme == target.Scheme && strings.EqualFold(origin.Host, target.Host) {
				return requested
			}
		}
	}
	return fallback
}
