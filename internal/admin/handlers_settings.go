package admin

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// SettingsPage is the one home for platform-wide app configuration that
// isn't a payment-gateway credential (those stay on Financeiro) — the live
// map's refresh interval and the per-ride commission rate (global, with an
// optional per-city override).
func (m *Module) SettingsPage(c echo.Context) error {
	ctx := c.Request().Context()
	settings, err := m.billing.GetSettings(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	cities, err := m.cities.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "settings", "settings.html", map[string]any{
		"MapPollSeconds":    settings.MapPollSeconds,
		"CommissionPercent": settings.CommissionRate * 100,
		"Cities":            cities,
	})
}

// UpdateMapPollInterval sets how often (in seconds) the passenger and
// driver apps' live maps refresh — GET /api/settings is what they actually
// read this from.
func (m *Module) UpdateMapPollInterval(c echo.Context) error {
	seconds, err := strconv.Atoi(c.FormValue("mapPollSeconds"))
	if err != nil || seconds < 5 {
		return c.Redirect(http.StatusSeeOther, "/admin/settings")
	}
	if err := m.billing.SetMapPollSeconds(c.Request().Context(), seconds); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/settings")
}

// UpdateCommission sets the platform-wide default commission rate — a
// city's own override (see UpdateCityCommissionRate) takes precedence over
// this for rides originating there.
func (m *Module) UpdateCommission(c echo.Context) error {
	percent, err := strconv.ParseFloat(c.FormValue("commissionPercent"), 64)
	if err != nil || percent < 0 || percent > 100 {
		return c.Redirect(http.StatusSeeOther, "/admin/settings")
	}
	if err := m.billing.SetCommissionRate(c.Request().Context(), percent/100); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/settings")
}

// UpdateCityCommissionRate overrides the commission rate for one city; a
// blank input clears the override back to "use the platform default".
func (m *Module) UpdateCityCommissionRate(c echo.Context) error {
	cityID := c.Param("id")
	raw := c.FormValue("commissionPercent")

	var rate *float64
	if raw != "" {
		percent, err := strconv.ParseFloat(raw, 64)
		if err != nil || percent < 0 || percent > 100 {
			return c.Redirect(http.StatusSeeOther, "/admin/settings")
		}
		value := percent / 100
		rate = &value
	}

	if err := m.cities.SetCommissionRate(c.Request().Context(), cityID, rate); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/settings")
}
