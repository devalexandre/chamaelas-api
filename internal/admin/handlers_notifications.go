package admin

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
)

func (m *Module) NotificationsPage(c echo.Context) error {
	ctx := c.Request().Context()
	drivers, err := m.drivers.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	users, err := m.users.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	recent, err := m.notifications.ListRecentSent(ctx, 30)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Look up a display name for each recent row without a JOIN — the
	// recipient lists are already loaded, and this history is capped small.
	driverNames := map[string]string{}
	for _, d := range drivers {
		driverNames[d.ID] = d.Name
	}
	userNames := map[string]string{}
	for _, u := range users {
		userNames[u.ID] = u.Name
	}
	type recentRow struct {
		models.Notification
		RecipientName string
	}
	recentRows := make([]recentRow, 0, len(recent))
	for _, n := range recent {
		name := driverNames[n.RecipientID]
		if n.RecipientType == models.NotificationRecipientUser {
			name = userNames[n.RecipientID]
		}
		if name == "" {
			name = "(removida)"
		}
		recentRows = append(recentRows, recentRow{Notification: n, RecipientName: name})
	}

	return render(c, "notifications", "notifications.html", map[string]any{
		"Drivers": drivers,
		"Users":   users,
		"Recent":  recentRows,
		"Sent":    c.QueryParam("sent") == "1",
	})
}

func (m *Module) SendNotification(c echo.Context) error {
	title := strings.TrimSpace(c.FormValue("title"))
	message := strings.TrimSpace(c.FormValue("message"))
	driverIDs := c.Request().Form["driverIds"]
	userIDs := c.Request().Form["userIds"]

	if title == "" || message == "" || (len(driverIDs) == 0 && len(userIDs) == 0) {
		return c.Redirect(http.StatusSeeOther, "/admin/notifications")
	}

	ctx := c.Request().Context()
	if len(driverIDs) > 0 {
		if err := m.notifications.SendToMany(ctx, models.NotificationRecipientDriver, driverIDs, title, message); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	if len(userIDs) > 0 {
		if err := m.notifications.SendToMany(ctx, models.NotificationRecipientUser, userIDs, title, message); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	return c.Redirect(http.StatusSeeOther, "/admin/notifications?sent=1")
}
