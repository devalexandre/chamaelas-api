package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"chamaelas-api/internal/googleauth"
	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

type AuthHandler struct {
	users          *repository.UserRepository
	userCredit     *repository.UserCreditRepository
	wooviSettings  *repository.WooviSettingsRepository
	pixKeyChanges  *repository.PixKeyChangeRepository
	googleClientID string
}

func NewAuthHandler(users *repository.UserRepository, userCredit *repository.UserCreditRepository, wooviSettings *repository.WooviSettingsRepository, pixKeyChanges *repository.PixKeyChangeRepository, googleClientID string) *AuthHandler {
	return &AuthHandler{users: users, userCredit: userCredit, wooviSettings: wooviSettings, pixKeyChanges: pixKeyChanges, googleClientID: googleClientID}
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

	user.GoogleLinked = user.GoogleSub != nil
	h.ensureSubaccount(ctx, user)
	return c.JSON(http.StatusOK, user)
}

type googleLoginRequest struct {
	IDToken string `json:"idToken"`
}

// GoogleLogin resolves a Google ID token to a User in three steps — already
// linked by google_sub → login; else a verified-email match on an existing
// password account → auto-link and login; else create a brand-new
// Google-only account (empty PasswordHash, so she simply can't
// password-login until she sets one — no schema change needed for that).
// Ports agenda_api's internal/domain/client/service.go GoogleLogin.
func (h *AuthHandler) GoogleLogin(c echo.Context) error {
	var req googleLoginRequest
	if err := c.Bind(&req); err != nil || req.IDToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "idToken is required")
	}
	if h.googleClientID == "" {
		return echo.NewHTTPError(http.StatusNotImplemented, "login com Google não está configurado")
	}

	ctx := c.Request().Context()
	claims, err := googleauth.Verify(ctx, req.IDToken, h.googleClientID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "idToken inválido")
	}

	if user, err := h.users.FindByGoogleSub(ctx, claims.Sub); err == nil {
		user.GoogleLinked = true
		h.ensureSubaccount(ctx, user)
		return c.JSON(http.StatusOK, user)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if claims.EmailVerified {
		if user, err := h.users.FindByEmail(ctx, claims.Email); err == nil {
			if err := h.users.SetGoogleSub(ctx, user.ID, claims.Sub); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			user.GoogleLinked = true
			h.ensureSubaccount(ctx, user)
			return c.JSON(http.StatusOK, user)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	name := claims.Name
	if name == "" {
		name = claims.Email
	}
	user := &models.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     claims.Email,
		GoogleSub: &claims.Sub,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.users.Create(ctx, user); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	user.GoogleLinked = true
	h.ensureSubaccount(ctx, user)
	return c.JSON(http.StatusCreated, user)
}

// LinkGoogleAccount lets an already-logged-in passenger link her Google
// account from Profile — reached from inside the app with her own known
// id, so it uses the same trust model as every other :userId endpoint here
// (no extra password check, unlike SetPixKey, since this never changes
// where money goes).
func (h *AuthHandler) LinkGoogleAccount(c echo.Context) error {
	var req googleLoginRequest
	if err := c.Bind(&req); err != nil || req.IDToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "idToken is required")
	}
	if h.googleClientID == "" {
		return echo.NewHTTPError(http.StatusNotImplemented, "login com Google não está configurado")
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

	claims, err := googleauth.Verify(ctx, req.IDToken, h.googleClientID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "idToken inválido")
	}

	if existing, err := h.users.FindByGoogleSub(ctx, claims.Sub); err == nil && existing.ID != userID {
		return echo.NewHTTPError(http.StatusConflict, "essa conta Google já está vinculada a outro perfil")
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := h.users.SetGoogleSub(ctx, userID, claims.Sub); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	user.GoogleLinked = true
	return c.JSON(http.StatusOK, user)
}

// GetProfile returns the passenger's current server-side record — used by
// the app to refresh cached fields (credit balance, in particular) that can
// change asynchronously (a Pix top-up confirming via webhook, a ride debit)
// without the app itself doing anything.
func (h *AuthHandler) GetProfile(c echo.Context) error {
	ctx := c.Request().Context()
	user, err := h.users.FindByID(ctx, c.Param("userId"))
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	user.GoogleLinked = user.GoogleSub != nil
	return c.JSON(http.StatusOK, user)
}

// ensureSubaccount auto-provisions a Woovi subaccount (using her CPF as the
// default Pix key) for a passenger who doesn't have one yet — best-effort,
// log-and-continue, so login never fails just because Woovi is unreachable
// or not configured. She can always change the key afterward via SetPixKey
// (subject to admin approval).
func (h *AuthHandler) ensureSubaccount(ctx context.Context, user *models.User) {
	if user.PixKey != "" {
		return
	}
	settings, err := h.wooviSettings.Get(ctx)
	if err != nil || settings.AppID == "" {
		return
	}
	cpf := woovi.OnlyDigits(user.CPF)
	if cpf == "" {
		return
	}
	canonical, err := woovi.EnsureRecipient(settings.AppID, woovi.BaseURL(settings.Environment), cpf, user.Name)
	if err != nil {
		log.Printf("user %s: failed to auto-provision Woovi subaccount: %v", user.ID, err)
		return
	}
	if err := h.userCredit.SetPixKey(ctx, user.ID, canonical); err != nil {
		log.Printf("user %s: created Woovi subaccount but failed to save pix key: %v", user.ID, err)
		return
	}
	user.PixKey = canonical
}

type setUserPixKeyRequest struct {
	PixKey   string `json:"pixKey"`
	Password string `json:"password"`
}

// SetPixKey files a request to change a passenger's Pix key — same
// admin-approval-required flow as DriverHandler.SetPixKey, for the same
// reason: a compromised account shouldn't be able to redirect a payout
// destination on its own.
func (h *AuthHandler) SetPixKey(c echo.Context) error {
	var req setUserPixKeyRequest
	if err := c.Bind(&req); err != nil || req.PixKey == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "pixKey and password are required")
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
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "senha incorreta")
	}

	if _, err := h.pixKeyChanges.Create(ctx, models.PixKeyOwnerUser, userID, user.PixKey, req.PixKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusAccepted, map[string]string{"status": "pending_approval"})
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

	if user.PixKey == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "cadastre uma chave Pix antes de recarregar")
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.AppID == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gateway de pagamento não configurado")
	}

	// Real money for this top-up lands in the PASSENGER's own subaccount,
	// not the platform's — it's later moved OUT of it (split between the
	// driver and the platform) when she actually pays for a ride with it —
	// see Complete().
	correlationID := "passenger-topup-" + uuid.NewString()
	charge, err := woovi.CreateCharge(settings.AppID, woovi.BaseURL(settings.Environment), woovi.ChargeInput{
		CorrelationID: correlationID,
		Cents:         req.AmountCents,
		Comment:       "Recarga de crédito - Chama Elas",
		CustomerName:  user.Name,
		CustomerEmail: user.Email,
		CustomerPhone: user.Phone,
		Subaccount:    user.PixKey,
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
