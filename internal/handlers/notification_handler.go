package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
)

type NotificationHandler struct {
	notifications *repository.NotificationRepository
}

func NewNotificationHandler(notifications *repository.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{notifications: notifications}
}

func (h *NotificationHandler) ListForDriver(c echo.Context) error {
	list, err := h.notifications.ListForRecipient(c.Request().Context(), models.NotificationRecipientDriver, c.Param("driverId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, list)
}

func (h *NotificationHandler) ListForUser(c echo.Context) error {
	list, err := h.notifications.ListForRecipient(c.Request().Context(), models.NotificationRecipientUser, c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, list)
}

func (h *NotificationHandler) MarkRead(c echo.Context) error {
	if err := h.notifications.MarkRead(c.Request().Context(), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
