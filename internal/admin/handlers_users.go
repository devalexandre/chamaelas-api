package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (m *Module) ListUsers(c echo.Context) error {
	users, err := m.users.ListAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "users", "users.html", map[string]any{"Users": users})
}
