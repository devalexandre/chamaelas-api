package admin

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
)

func (m *Module) ListCities(c echo.Context) error {
	cities, err := m.cities.ListAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "cities", "cities.html", map[string]any{"Cities": cities})
}

func (m *Module) CreateCity(c echo.Context) error {
	name := strings.TrimSpace(c.FormValue("name"))
	uf := strings.ToUpper(strings.TrimSpace(c.FormValue("uf")))
	if name == "" || uf == "" {
		return c.Redirect(http.StatusSeeOther, "/admin/cities")
	}

	city := &models.City{
		ID:     uuid.NewString(),
		Name:   name,
		UF:     uf,
		Active: true,
	}
	if err := m.cities.Create(c.Request().Context(), city); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/cities")
}

func (m *Module) ToggleCity(c echo.Context) error {
	ctx := c.Request().Context()
	cities, err := m.cities.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	id := c.Param("id")
	for _, city := range cities {
		if city.ID == id {
			if err := m.cities.SetActive(ctx, id, !city.Active); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			break
		}
	}
	return c.Redirect(http.StatusSeeOther, "/admin/cities")
}
