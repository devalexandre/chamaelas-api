package admin

import (
	"encoding/json"
	"html/template"

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

func (m *Module) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()

	drivers, _ := m.drivers.ListAll(ctx)
	users, _ := m.users.ListAll(ctx)

	pendingCount := 0
	onlineCount := 0
	markers := []dashboardDriverMarker{}
	for _, d := range drivers {
		if d.Status == string(models.DriverStatusPending) {
			pendingCount++
		}
		if d.IsOnline {
			onlineCount++
			if d.Lat != nil && d.Lng != nil {
				markers = append(markers, dashboardDriverMarker{
					ID: d.ID, Name: d.Name, PhotoURL: d.PhotoURL, Lat: *d.Lat, Lng: *d.Lng,
				})
			}
		}
	}
	markersJSON, err := json.Marshal(markers)
	if err != nil {
		markersJSON = []byte("[]")
	}

	return render(c, "dashboard", "dashboard.html", map[string]any{
		"DriverCount":   len(drivers),
		"UserCount":     len(users),
		"PendingCount":  pendingCount,
		"OnlineCount":   onlineCount,
		"DriverMarkers": template.JS(markersJSON),
	})
}
