package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/repository"
)

type FavoriteDriverHandler struct {
	favoriteDrivers *repository.FavoriteDriverRepository
}

func NewFavoriteDriverHandler(favoriteDrivers *repository.FavoriteDriverRepository) *FavoriteDriverHandler {
	return &FavoriteDriverHandler{favoriteDrivers: favoriteDrivers}
}

func (h *FavoriteDriverHandler) List(c echo.Context) error {
	drivers, err := h.favoriteDrivers.ListDriversForUser(c.Request().Context(), c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, drivers)
}

func (h *FavoriteDriverHandler) Add(c echo.Context) error {
	if err := h.favoriteDrivers.Add(c.Request().Context(), c.Param("userId"), c.Param("driverId")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *FavoriteDriverHandler) Remove(c echo.Context) error {
	if err := h.favoriteDrivers.Remove(c.Request().Context(), c.Param("userId"), c.Param("driverId")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
