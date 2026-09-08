package admin

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
)

func (m *Module) ListCategories(c echo.Context) error {
	categories, err := m.categories.ListAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return render(c, "categories", "categories.html", map[string]any{"Categories": categories})
}

func (m *Module) CreateCategory(c echo.Context) error {
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect(http.StatusSeeOther, "/admin/categories")
	}

	category := &models.Category{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		Active:      true,
	}
	if err := m.categories.Create(c.Request().Context(), category); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/categories")
}

func (m *Module) ToggleCategory(c echo.Context) error {
	ctx := c.Request().Context()
	categories, err := m.categories.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	id := c.Param("id")
	for _, category := range categories {
		if category.ID == id {
			if err := m.categories.SetActive(ctx, id, !category.Active); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			break
		}
	}
	return c.Redirect(http.StatusSeeOther, "/admin/categories")
}
