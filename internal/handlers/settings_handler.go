package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/repository"
)

type SettingsHandler struct {
	billing *repository.BillingRepository
}

func NewSettingsHandler(billing *repository.BillingRepository) *SettingsHandler {
	return &SettingsHandler{billing: billing}
}

// PublicSettings exposes only the handful of platform settings the apps
// themselves need at runtime (how often to refresh the live map) — never
// anything sensitive like the commission rate.
func (h *SettingsHandler) PublicSettings(c echo.Context) error {
	settings, err := h.billing.GetSettings(c.Request().Context())
	mapPollSeconds := 60
	if err == nil && settings.MapPollSeconds > 0 {
		mapPollSeconds = settings.MapPollSeconds
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"mapPollSeconds": mapPollSeconds,
	})
}
