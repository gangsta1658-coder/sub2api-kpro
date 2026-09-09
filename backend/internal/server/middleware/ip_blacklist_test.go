//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type ipBlacklistMiddlewareRepo struct {
	values map[string]string
}

func (r *ipBlacklistMiddlewareRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *ipBlacklistMiddlewareRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *ipBlacklistMiddlewareRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *ipBlacklistMiddlewareRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *ipBlacklistMiddlewareRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *ipBlacklistMiddlewareRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *ipBlacklistMiddlewareRepo) Delete(context.Context, string) error { return nil }

func TestSiteIPBlacklistRejectsBlockedClientAndPreservesHealthProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewSettingService(&ipBlacklistMiddlewareRepo{values: map[string]string{}}, &config.Config{})
	_, err := svc.SetIPBlacklist(context.Background(), []string{"203.0.113.0/24", "127.0.0.1"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(SiteIPBlacklist(svc))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/private", func(c *gin.Context) { c.Status(http.StatusOK) })

	blocked := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodGet, "/private", nil)
	blockedRequest.RemoteAddr = "203.0.113.42:12345"
	router.ServeHTTP(blocked, blockedRequest)
	require.Equal(t, http.StatusForbidden, blocked.Code)
	require.Contains(t, blocked.Body.String(), "IP_BLACKLISTED")

	health := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRequest.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(health, healthRequest)
	require.Equal(t, http.StatusOK, health.Code)
}
