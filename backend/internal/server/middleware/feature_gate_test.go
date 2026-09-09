package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFeatureGate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, testCase := range []struct {
		name    string
		enabled bool
		status  int
	}{
		{name: "allows enabled feature", enabled: true, status: http.StatusNoContent},
		{name: "hides disabled feature", enabled: false, status: http.StatusNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/feature", FeatureGate(func(context.Context) bool {
				return testCase.enabled
			}), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/feature", nil))

			require.Equal(t, testCase.status, response.Code)
		})
	}
}
