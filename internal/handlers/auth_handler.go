package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

type AuthHandler struct {
	users         *repository.UserRepository
	userCredit    *repository.UserCreditRepository
	wooviSettings *repository.WooviSettingsRepository
}

func NewAuthHandler(users *repository.UserRepository, userCredit *repository.UserCreditRepository, wooviSettings *repository.WooviSettingsRepository) *AuthHandler {
	return &AuthHandler{users: users, userCredit: userCredit, wooviSettings: wooviSettings}
}

type signupRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CPF       string `json:"cpf"`
	BirthDate string `json:"birthDate"`
	Password  string `json:"password"`
	PhotoURL  string `json:"photoUrl"`
}

func (h *AuthHandler) Signup(c echo.Context) error {
	var req signupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, email and password are required")
	}

	ctx := c.Request().Context()
	if _, err := h.users.FindByEmail(ctx, req.Email); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "email already registered")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
	}

	user := &models.User{
		ID:           uuid.NewString(),
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		CPF:          req.CPF,
		BirthDate:    req.BirthDate,
		PhotoURL:     req.PhotoURL,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.users.Create(ctx, user); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, user)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	user, err := h.users.FindByEmail(ctx, req.Email)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	return c.JSON(http.StatusOK, user)
}

type createUserCreditTopupRequest struct {
	AmountCents int `json:"amountCents"`
}

// CreateCreditTopup creates a plain (no-split) Pix charge to top up a
// passenger's spend-only prepaid credit — mirrors the driver's
// CreateCreditTopup, just with a "passenger-topup-" correlationID prefix so
// the webhook knows which ledger to credit. Actually spending this credit
// on a ride's price is a separate, later piece of work — this is top-up
// plumbing only.
func (h *AuthHandler) CreateCreditTopup(c echo.Context) error {
	var req createUserCreditTopupRequest
	if err := c.Bind(&req); err != nil || req.AmountCents <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "amountCents must be positive")
	}

	ctx := c.Request().Context()
	userID := c.Param("userId")
	user, err := h.users.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.AppID == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gateway de pagamento não configurado")
	}

	correlationID := "passenger-topup-" + uuid.NewString()
	charge, err := woovi.CreateCharge(settings.AppID, woovi.BaseURL(settings.Environment), woovi.ChargeInput{
		CorrelationID: correlationID,
		Cents:         req.AmountCents,
		Comment:       "Recarga de crédito - Chama Elas",
		CustomerName:  user.Name,
		CustomerEmail: user.Email,
		CustomerPhone: user.Phone,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if err := h.userCredit.CreateTopup(ctx, correlationID, userID, req.AmountCents); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, creditTopupResponse{
		CorrelationID: correlationID,
		BRCode:        charge.BRCode,
		QRCodeImage:   charge.QRCodeImage,
		ExpiresAt:     charge.ExpiresAt,
	})
}

// ListCreditTransactions returns a passenger's credit ledger — top-ups and
// (once ride-payment-by-credit exists) debits — for her extrato.
func (h *AuthHandler) ListCreditTransactions(c echo.Context) error {
	transactions, err := h.userCredit.ListTransactions(c.Request().Context(), c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, transactions)
}
