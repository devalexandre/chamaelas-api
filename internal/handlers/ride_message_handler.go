package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
)

const maxRideMessageLength = 1000

type RideMessageHandler struct {
	rides    *repository.RideRepository
	messages *repository.RideMessageRepository
}

func NewRideMessageHandler(rides *repository.RideRepository, messages *repository.RideMessageRepository) *RideMessageHandler {
	return &RideMessageHandler{rides: rides, messages: messages}
}

func (h *RideMessageHandler) List(c echo.Context) error {
	ctx := c.Request().Context()
	if _, err := h.rides.FindByID(ctx, c.Param("id")); errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	} else if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	list, err := h.messages.ListByRide(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, list)
}

type sendRideMessageRequest struct {
	SenderType string `json:"senderType"`
	SenderID   string `json:"senderId"`
	Body       string `json:"body"`
}

// Send posts one chat line. The ride must already have a driver assigned
// (nothing to say to before that) and must still be open — once it's
// completed or cancelled, this rejects new messages, while List keeps
// returning the existing ones as a read-only transcript.
func (h *RideMessageHandler) Send(c echo.Context) error {
	var req sendRideMessageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "body is required")
	}
	if len(body) > maxRideMessageLength {
		body = body[:maxRideMessageLength]
	}
	if req.SenderType != models.RideMessageSenderUser && req.SenderType != models.RideMessageSenderDriver {
		return echo.NewHTTPError(http.StatusBadRequest, "senderType must be \"user\" or \"driver\"")
	}

	ctx := c.Request().Context()
	rideID := c.Param("id")
	ride, err := h.rides.FindByID(ctx, rideID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if ride.Status == string(models.RideStatusCompleted) || ride.Status == string(models.RideStatusCancelled) {
		return echo.NewHTTPError(http.StatusConflict, "essa corrida já terminou — o chat agora é somente leitura")
	}
	if ride.DriverID == nil {
		return echo.NewHTTPError(http.StatusConflict, "ainda não há uma motorista nessa corrida para conversar")
	}
	switch req.SenderType {
	case models.RideMessageSenderUser:
		if req.SenderID != ride.UserID {
			return echo.NewHTTPError(http.StatusForbidden, "senderId does not match this ride's passenger")
		}
	case models.RideMessageSenderDriver:
		if req.SenderID != *ride.DriverID {
			return echo.NewHTTPError(http.StatusForbidden, "senderId does not match this ride's driver")
		}
	}

	msg := &models.RideMessage{
		ID:         uuid.NewString(),
		RideID:     rideID,
		SenderType: req.SenderType,
		SenderID:   req.SenderID,
		Body:       body,
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.messages.Create(ctx, msg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, msg)
}
