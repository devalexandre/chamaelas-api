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
)

type AuthHandler struct {
	users *repository.UserRepository
}

func NewAuthHandler(users *repository.UserRepository) *AuthHandler {
	return &AuthHandler{users: users}
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
