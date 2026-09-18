package admin

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/repository"
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

// RideChat shows the full in-ride chat transcript for one ride — the audit
// trail for "she said she couldn't find him" disputes. Messages are never
// deleted, so this reads the same rows the apps themselves fetched live.
func (m *Module) RideChat(c echo.Context) error {
	ctx := c.Request().Context()
	rideID := c.Param("id")

	ride, err := m.rides.FindByID(ctx, rideID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	passengerName := ""
	if passenger, err := m.users.FindByID(ctx, ride.UserID); err == nil {
		passengerName = passenger.Name
	}
	driverName := ""
	if ride.DriverID != nil {
		if driver, err := m.drivers.FindByID(ctx, *ride.DriverID); err == nil {
			driverName = driver.Name
		}
	}

	messages, err := m.rideMessages.ListByRide(ctx, rideID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return render(c, "rides", "ride_chat.html", map[string]any{
		"Ride":          ride,
		"PassengerName": passengerName,
		"DriverName":    driverName,
		"Messages":      messages,
	})
}
