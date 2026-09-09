package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ipBlacklistRequest struct {
	IP string `json:"ip"`
}

// GetIPBlacklist returns the site-wide client IP/CIDR blacklist.
// GET /api/v1/admin/settings/ip-blacklist
func (h *SettingHandler) GetIPBlacklist(c *gin.Context) {
	items, err := h.settingService.GetIPBlacklist(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// AddIPBlacklistEntry adds one normalized IP or CIDR entry.
// POST /api/v1/admin/settings/ip-blacklist
func (h *SettingHandler) AddIPBlacklistEntry(c *gin.Context) {
	var req ipBlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ipEntry := strings.TrimSpace(req.IP)
	if ipEntry == "" {
		response.Error(c, http.StatusBadRequest, "ip is required")
		return
	}
	items, err := h.settingService.AddIPBlacklistEntry(c.Request.Context(), ipEntry)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// DeleteIPBlacklistEntry removes one IP or CIDR entry.
// DELETE /api/v1/admin/settings/ip-blacklist/:ip
func (h *SettingHandler) DeleteIPBlacklistEntry(c *gin.Context) {
	ipEntry := strings.TrimSpace(c.Param("ip"))
	if ipEntry == "" {
		ipEntry = strings.TrimSpace(c.Query("ip"))
	}
	if ipEntry == "" {
		var req ipBlacklistRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			ipEntry = strings.TrimSpace(req.IP)
		}
	}
	if ipEntry == "" {
		response.Error(c, http.StatusBadRequest, "ip is required")
		return
	}
	normalized, err := service.NormalizeIPBlacklist([]string{ipEntry})
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_IP_BLACKLIST", err.Error()))
		return
	}
	items, err := h.settingService.RemoveIPBlacklistEntry(c.Request.Context(), normalized[0])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
