package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ListRides shows every ride regardless of status, most recent first, with
// its current matching state — who it's offered to and when that offer
// expires — so an operator can see whether the matcher is actually working
// without having to query the database by hand.
func (m *Module) ListRides(c echo.Context) error {
	rides, err := m.rides.ListRecentLog(c.Request().Context(), 100)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "rides", "rides.html", map[string]any{"Rides": rides})
}

// ListRidesData is polled by the Corridas page (every few seconds) so it
// stays live without a manual reload.
func (m *Module) ListRidesData(c echo.Context) error {
	rides, err := m.rides.ListRecentLog(c.Request().Context(), 100)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, rides)
}
