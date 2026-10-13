package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// SSOClientCORS lets the browser pages of registered SSO applications call the
// sign-in APIs cross-origin. Only an exact match with the origin of a
// registered redirect_uri is allowed, and credentials (cookies) are never
// enabled: the pages authenticate with a bearer token. A preflight from any
// other origin is answered 204 without CORS headers, so the browser blocks it.
func SSOClientCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := setting.IsSSOClientOrigin(origin)
		if allowed {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
			if c.Request.Method == http.MethodOptions {
				header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				header.Set("Access-Control-Max-Age", "600")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
