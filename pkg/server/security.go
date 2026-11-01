package server

import "github.com/gin-gonic/gin"

// SecurityHeadersMiddleware applies conservative security response headers to
// every response the application serves.
//
// Only the three headers below are set globally. A Content-Security-Policy is
// intentionally NOT set here: the SPA, the embedded play client and the
// WebSocket upgrade all load scripts/styles/connections in ways that a single
// global policy would break. The dedicated /activate page keeps its own
// restrictive CSP (see handler.setActivateHeaders).
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
