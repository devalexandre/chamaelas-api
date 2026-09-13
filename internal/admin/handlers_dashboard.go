package admin

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"

	"chamaelas-api/internal/models"

	"github.com/labstack/echo/v4"
)

// dashboardDriverMarker is the minimal shape the dashboard's live map needs
// per online driver — marshaled to JSON and embedded in the page for a
// small inline script to render, no separate API call required.
type dashboardDriverMarker struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	PhotoURL string  `json:"photoUrl"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// onlineDriverMarkers returns the map markers for every driver currently
// online (see models.Driver.IsReallyOnline), plus how many that is —
// shared by the initial page render and the polling endpoint that keeps the
// map's markers fresh without a full page reload.
func (m *Module) onlineDriverMarkers(ctx context.Context) ([]dashboardDriverMarker, int, int, error) {
	drivers, err := m.drivers.ListAll(ctx)
	if err != nil {
		return nil, 0, 0, err
	}

	pendingCount := 0
	onlineCount := 0
	markers := []dashboardDriverMarker{}
	for _, d := range drivers {
		if d.Status == string(models.DriverStatusPending) {
			pendingCount++
		}
		if d.IsReallyOnline() {
			onlineCount++
			if d.Lat != nil && d.Lng != nil {
				markers = append(markers, dashboardDriverMarker{
					ID: d.ID, Name: d.Name, PhotoURL: d.PhotoURL, Lat: *d.Lat, Lng: *d.Lng,
				})
			}
		}
	}
	return markers, len(drivers), pendingCount, err
}

func (m *Module) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()

	markers, driverCount, pendingCount, err := m.onlineDriverMarkers(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	users, _ := m.users.ListAll(ctx)

	markersJSON, err := json.Marshal(markers)
	if err != nil {
		markersJSON = []byte("[]")
	}

	return render(c, "dashboard", "dashboard.html", map[string]any{
		"DriverCount":   driverCount,
		"UserCount":     len(users),
		"PendingCount":  pendingCount,
		"OnlineCount":   len(markers),
		"DriverMarkers": template.JS(markersJSON),
	})
}

// DashboardDriversOnline is polled by the dashboard's live map (every 60s)
// to refresh online-driver positions without a full page reload.
func (m *Module) DashboardDriversOnline(c echo.Context) error {
	markers, _, _, err := m.onlineDriverMarkers(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, markers)
}
