package middleware

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// FeatureGate hides disabled optional features from direct API access.
func FeatureGate(isEnabled func(context.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isEnabled == nil || !isEnabled(c.Request.Context()) {
			c.Abort()
			response.NotFound(c, "Feature is disabled")
			return
		}
		c.Next()
	}
}
