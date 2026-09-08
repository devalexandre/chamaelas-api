package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/repository"
)

type CategoryHandler struct {
	categories *repository.CategoryRepository
}

func NewCategoryHandler(categories *repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

func (h *CategoryHandler) ListActive(c echo.Context) error {
	categories, err := h.categories.ListActive(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, categories)
}
