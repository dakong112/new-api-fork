package middleware

import (
	"crypto/subtle"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// SessionCookieOriginGuard protects cookie-authenticated refresh/logout
// endpoints when secure cookie mode is enabled. In insecure local development
// mode it preserves the legacy behavior and intentionally performs no Origin
// validation. It never adds CORS response headers and must not be installed on
// relay routes.
func SessionCookieOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.SessionCookieSecure {
			c.Next()
			return
		}
		origin, ok := requestBrowserOrigin(c.Request)
		if !ok || !isAllowedSessionOrigin(c.Request, origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"code":    "AUTH_ORIGIN_FORBIDDEN",
				"message": "request origin is not allowed",
			})
			return
		}
		c.Next()
	}
}

func requestBrowserOrigin(request *http.Request) (string, bool) {
	originValues := request.Header.Values("Origin")
	if len(originValues) > 1 {
		return "", false
	}
	if len(originValues) == 1 {
		if strings.Contains(originValues[0], ",") {
			return "", false
		}
		origin, err := common.NormalizeOrigin(originValues[0])
		return origin, err == nil
	}
	refererValues := request.Header.Values("Referer")
	if len(refererValues) != 1 {
		return "", false
	}
	referer, err := url.Parse(strings.TrimSpace(refererValues[0]))
	if err != nil || referer.Scheme == "" || referer.Host == "" || referer.User != nil {
		return "", false
	}
	origin, err := common.NormalizeOrigin(referer.Scheme + "://" + referer.Host)
	return origin, err == nil
}

func isAllowedSessionOrigin(request *http.Request, origin string) bool {
	requestScheme := "http"
	if request.TLS != nil {
		requestScheme = "https"
	}
	requestOrigin, err := common.NormalizeOrigin(requestScheme + "://" + request.Host)
	if err == nil && subtle.ConstantTimeCompare([]byte(origin), []byte(requestOrigin)) == 1 {
		return true
	}
	for _, trustedOrigin := range common.SessionCookieTrustedURLs {
		if subtle.ConstantTimeCompare([]byte(origin), []byte(trustedOrigin)) == 1 {
			return true
		}
	}
	return false
}

// SameSiteJSONOnly blocks login CSRF on anonymous endpoints that create a
// session: the body must be application/json (a cross-site form can only send
// simple content types), Sec-Fetch-Site must be same-origin when the browser
// sends it, and an Origin header, when present, must be an allowed origin.
func SameSiteJSONOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if mediaType != "application/json" {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"success": false, "message": "content type must be application/json"})
			return
		}
		if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_ORIGIN_FORBIDDEN", "message": "request origin is not allowed"})
			return
		}
		if len(c.Request.Header.Values("Origin")) > 0 {
			origin, ok := requestBrowserOrigin(c.Request)
			if !ok || !isAllowedSessionOrigin(c.Request, origin) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_ORIGIN_FORBIDDEN", "message": "request origin is not allowed"})
				return
			}
		}
		c.Next()
	}
}
