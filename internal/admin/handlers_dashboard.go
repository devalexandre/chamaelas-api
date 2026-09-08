package admin

import (
	"chamaelas-api/internal/models"

	"github.com/labstack/echo/v4"
)

func (m *Module) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()

	drivers, _ := m.drivers.ListAll(ctx)
	users, _ := m.users.ListAll(ctx)

	pendingCount := 0
	onlineCount := 0
	for _, d := range drivers {
		if d.Status == string(models.DriverStatusPending) {
			pendingCount++
		}
		if d.IsOnline {
			onlineCount++
		}
	}

	return render(c, "dashboard", "dashboard.html", map[string]any{
		"DriverCount":  len(drivers),
		"UserCount":    len(users),
		"PendingCount": pendingCount,
		"OnlineCount":  onlineCount,
	})
}
