package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SiteIPBlacklist rejects requests from the active site-wide IP/CIDR blacklist.
// It must run after SessionBindingContext so it uses the same trusted client-IP
// resolution as authentication, audit logs, and API-key ACLs.
func SiteIPBlacklist(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Docker and orchestration probes call this endpoint over loopback. It is
		// an operational liveness check, not an application-access path, and must
		// remain available even when a loopback range is blocked.
		if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		if settingService == nil || !settingService.IsIPBlacklisted(SecurityClientIP(c)) {
			c.Next()
			return
		}
		AbortWithError(c, http.StatusForbidden, "IP_BLACKLISTED", "Access denied")
	}
}
