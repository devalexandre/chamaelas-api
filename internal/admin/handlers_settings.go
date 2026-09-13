package admin

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// SettingsPage is the one home for platform-wide app configuration that
// isn't specifically financial (that stays on Financeiro) — starting with
// the live map's refresh interval, with room to grow as more knobs like it
// show up instead of getting bolted onto whichever page happened to exist.
func (m *Module) SettingsPage(c echo.Context) error {
	settings, err := m.billing.GetSettings(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "settings", "settings.html", map[string]any{
		"MapPollSeconds": settings.MapPollSeconds,
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
