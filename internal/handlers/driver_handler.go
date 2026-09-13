package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"chamaelas-api/internal/geo"
	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

type DriverHandler struct {
	drivers       *repository.DriverRepository
	categories    *repository.CategoryRepository
	rides         *RideHandler
	billing       *repository.BillingRepository
	wooviSettings *repository.WooviSettingsRepository
}

func NewDriverHandler(drivers *repository.DriverRepository, categories *repository.CategoryRepository, rides *RideHandler, billing *repository.BillingRepository, wooviSettings *repository.WooviSettingsRepository) *DriverHandler {
	return &DriverHandler{drivers: drivers, categories: categories, rides: rides, billing: billing, wooviSettings: wooviSettings}
}

func (h *DriverHandler) withCategories(ctx context.Context, driver *models.Driver) {
	categories, err := h.categories.ListByDriver(ctx, driver.ID)
	if err == nil {
		driver.Categories = categories
	}
}

type driverSignupRequest struct {
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Phone        string   `json:"phone"`
	CPF          string   `json:"cpf"`
	CNH          string   `json:"cnh"`
	BirthDate    string   `json:"birthDate"`
	Password     string   `json:"password"`
	PhotoURL     string   `json:"photoUrl"`
	VehiclePlate string   `json:"vehiclePlate"`
	VehicleModel string   `json:"vehicleModel"`
	VehicleColor string   `json:"vehicleColor"`
	VehicleYear  string   `json:"vehicleYear"`
	CategoryIDs  []string `json:"categoryIds"`
	// Bank account payouts land on — required later to create her Pagar.me
	// Recebedor when an admin approves the account.
	BankCode          string `json:"bankCode"`
	BankBranch        string `json:"bankBranch"`
	BankAccountNumber string `json:"bankAccountNumber"`
	BankAccountType   string `json:"bankAccountType"`
}

func (h *DriverHandler) Signup(c echo.Context) error {
	var req driverSignupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Name == "" || req.Email == "" || req.Password == "" || req.CNH == "" || req.VehiclePlate == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, email, password, cnh and vehiclePlate are required")
	}
	if len(req.CategoryIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "select at least one category")
	}

	ctx := c.Request().Context()
	if _, err := h.drivers.FindByEmail(ctx, req.Email); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "email already registered")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	activeCategories, err := h.categories.ListActive(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	validIDs := map[string]bool{}
	for _, cat := range activeCategories {
		validIDs[cat.ID] = true
	}
	var chosenIDs []string
	for _, id := range req.CategoryIDs {
		if validIDs[id] {
			chosenIDs = append(chosenIDs, id)
		}
	}
	if len(chosenIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no valid category selected")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
	}

	driver := &models.Driver{
		ID:                uuid.NewString(),
		Name:              req.Name,
		Email:             req.Email,
		Phone:             req.Phone,
		CPF:               req.CPF,
		CNH:               req.CNH,
		BirthDate:         req.BirthDate,
		PhotoURL:          req.PhotoURL,
		PasswordHash:      string(hash),
		VehiclePlate:      req.VehiclePlate,
		VehicleModel:      req.VehicleModel,
		VehicleColor:      req.VehicleColor,
		VehicleYear:       req.VehicleYear,
		Rating:            5.0,
		Status:            string(models.DriverStatusPending),
		// Every driver is prepaid — there is no postpaid plan.
		BillingMode:       string(models.BillingModePrepaid),
		BankCode:          req.BankCode,
		BankBranch:        req.BankBranch,
		BankAccountNumber: req.BankAccountNumber,
		BankAccountType:   req.BankAccountType,
		CreatedAt:         time.Now().UTC(),
	}
	if err := h.drivers.Create(ctx, driver); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	for _, categoryID := range chosenIDs {
		if err := h.categories.AddToDriver(ctx, driver.ID, categoryID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	h.withCategories(ctx, driver)

	return c.JSON(http.StatusCreated, driver)
}

type driverLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *DriverHandler) Login(c echo.Context) error {
	var req driverLoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	driver, err := h.drivers.FindByEmail(ctx, req.Email)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if bcrypt.CompareHashAndPassword([]byte(driver.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	h.withCategories(ctx, driver)
	return c.JSON(http.StatusOK, driver)
}

func (h *DriverHandler) GetProfile(c echo.Context) error {
	ctx := c.Request().Context()
	driver, err := h.drivers.FindByID(ctx, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "driver not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	h.withCategories(ctx, driver)
	return c.JSON(http.StatusOK, driver)
}

type driverCategoryRequest struct {
	CategoryID string `json:"categoryId"`
}

func (h *DriverHandler) AddCategory(c echo.Context) error {
	var req driverCategoryRequest
	if err := c.Bind(&req); err != nil || req.CategoryID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "categoryId is required")
	}
	if err := h.categories.AddToDriver(c.Request().Context(), c.Param("id"), req.CategoryID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *DriverHandler) RemoveCategory(c echo.Context) error {
	if err := h.categories.RemoveFromDriver(c.Request().Context(), c.Param("id"), c.Param("categoryId")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

type setLocationRequest struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	IsOnline bool    `json:"isOnline"`
}

// SetLocation is pinged periodically by the driver app while online (and
// whenever the online/offline switch changes). Going online retries
// matching for any ride left "searching" in this driver's categories.
func (h *DriverHandler) SetLocation(c echo.Context) error {
	var req setLocationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	driverID := c.Param("id")

	if req.IsOnline {
		driver, err := h.drivers.FindByID(ctx, driverID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if driver.Status != string(models.DriverStatusApproved) {
			return echo.NewHTTPError(http.StatusForbidden, "sua conta ainda não foi aprovada")
		}
	}

	if err := h.drivers.SetLocation(ctx, driverID, req.Lat, req.Lng, req.IsOnline); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

type setBillingModeRequest struct {
	BillingMode string `json:"billingMode"`
}

// SetBillingMode lets a driver switch between the two commission plans
// (per_ride / prepaid) any time after signup, from her profile.
func (h *DriverHandler) SetBillingMode(c echo.Context) error {
	var req setBillingModeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.BillingMode != string(models.BillingModePerRide) && req.BillingMode != string(models.BillingModePrepaid) {
		return echo.NewHTTPError(http.StatusBadRequest, "billingMode must be \"per_ride\" or \"prepaid\"")
	}
	if err := h.billing.SetDriverBillingMode(c.Request().Context(), c.Param("id"), models.BillingMode(req.BillingMode)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// ListCreditTransactions returns the prepaid credit ledger — top-ups,
// per-ride fee debits, and manual adjustments — for a driver's extrato.
func (h *DriverHandler) ListCreditTransactions(c echo.Context) error {
	transactions, err := h.billing.ListTransactions(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, transactions)
}

type setPixKeyRequest struct {
	PixKey   string `json:"pixKey"`
	Password string `json:"password"`
}

// SetPixKey registers (or replaces) the Pix key backing a driver's revenue
// wallet. Requires her password — unlike the rest of this file's driver
// routes, this one moves real money, so trusting the :id path param alone
// isn't acceptable (see Withdraw for the same reasoning). Only the
// CANONICAL key Woovi returns is ever persisted, never the raw input.
func (h *DriverHandler) SetPixKey(c echo.Context) error {
	var req setPixKeyRequest
	if err := c.Bind(&req); err != nil || req.PixKey == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "pixKey and password are required")
	}

	ctx := c.Request().Context()
	driverID := c.Param("id")
	driver, err := h.drivers.FindByID(ctx, driverID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "driver not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if bcrypt.CompareHashAndPassword([]byte(driver.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "senha incorreta")
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.AppID == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gateway de pagamento não configurado")
	}

	canonical, err := woovi.EnsureRecipient(settings.AppID, woovi.BaseURL(settings.Environment), req.PixKey, driver.Name)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if err := h.billing.SetDriverPixKey(ctx, driverID, canonical); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

type createCreditTopupRequest struct {
	AmountCents int `json:"amountCents"`
}

type creditTopupResponse struct {
	CorrelationID string     `json:"correlationId"`
	BRCode        string     `json:"brCode"`
	QRCodeImage   string     `json:"qrCodeImage"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

// CreateCreditTopup creates a plain (no-split) Pix charge to top up a
// driver's prepaid credit balance — 100% of it stays with the platform,
// since she's paying the platform, not another party. The webhook at
// /webhooks/payment credits credit_balance once it's actually paid.
func (h *DriverHandler) CreateCreditTopup(c echo.Context) error {
	var req createCreditTopupRequest
	if err := c.Bind(&req); err != nil || req.AmountCents <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "amountCents must be positive")
	}

	ctx := c.Request().Context()
	driverID := c.Param("id")
	driver, err := h.drivers.FindByID(ctx, driverID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "driver not found")
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

	correlationID := "driver-topup-" + uuid.NewString()
	charge, err := woovi.CreateCharge(settings.AppID, woovi.BaseURL(settings.Environment), woovi.ChargeInput{
		CorrelationID: correlationID,
		Cents:         req.AmountCents,
		Comment:       "Recarga de crédito - Chama Elas",
		CustomerName:  driver.Name,
		CustomerEmail: driver.Email,
		CustomerPhone: driver.Phone,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if err := h.billing.CreateCreditTopup(ctx, correlationID, driverID, req.AmountCents); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, creditTopupResponse{
		CorrelationID: correlationID,
		BRCode:        charge.BRCode,
		QRCodeImage:   charge.QRCodeImage,
		ExpiresAt:     charge.ExpiresAt,
	})
}

type walletResponse struct {
	BalanceCents     int                        `json:"balanceCents"`
	WithdrawFeeCents int                        `json:"withdrawFeeCents"`
	HasPixKey        bool                       `json:"hasPixKey"`
	Transactions     []models.WalletTransaction `json:"transactions"`
}

// GetWallet returns a driver's revenue-wallet summary: the live Woovi
// subaccount balance (never cached locally), the withdrawal fee she'd pay
// right now, and her local transaction history. Best-effort on the live
// balance — an unreachable gateway degrades to "no balance shown" rather
// than failing the whole page, since the history is still useful on its own.
func (h *DriverHandler) GetWallet(c echo.Context) error {
	ctx := c.Request().Context()
	driverID := c.Param("id")
	driver, err := h.drivers.FindByID(ctx, driverID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "driver not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	transactions, err := h.billing.ListWalletTransactions(ctx, driverID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	resp := walletResponse{HasPixKey: driver.PixKey != "", Transactions: transactions}
	if driver.PixKey == "" {
		return c.JSON(http.StatusOK, resp)
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil || settings.AppID == "" {
		return c.JSON(http.StatusOK, resp)
	}
	balance, err := woovi.RecipientBalanceCents(settings.AppID, woovi.BaseURL(settings.Environment), driver.PixKey)
	if err != nil {
		log.Printf("driver %s: failed to fetch live wallet balance: %v", driverID, err)
		return c.JSON(http.StatusOK, resp)
	}
	resp.BalanceCents = balance
	resp.WithdrawFeeCents = woovi.WithdrawFeeCents(balance)
	return c.JSON(http.StatusOK, resp)
}

type withdrawRequest struct {
	Password string `json:"password"`
}

type withdrawResponse struct {
	AmountCents int    `json:"amountCents"`
	FeeCents    int    `json:"feeCents"`
	Status      string `json:"status"`
}

// Withdraw sweeps a driver's ENTIRE wallet balance out to her Pix key,
// after collecting the withdrawal fee (if any) into the platform's own Pix
// key first. Requires her password (see SetPixKey for why). This is
// inherently a multi-step sequence over HTTP calls to Woovi, not a local DB
// transaction — on failure after the fee was taken, best-effort transfers
// it back and just logs if that also fails, matching how this exact flow
// already works in the sibling agenda_api project.
func (h *DriverHandler) Withdraw(c echo.Context) error {
	var req withdrawRequest
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "password is required")
	}

	ctx := c.Request().Context()
	driverID := c.Param("id")
	driver, err := h.drivers.FindByID(ctx, driverID)
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "driver not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if bcrypt.CompareHashAndPassword([]byte(driver.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "senha incorreta")
	}
	if driver.PixKey == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "cadastre sua chave Pix para usar a carteira")
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.AppID == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gateway de pagamento não configurado")
	}
	baseURL := woovi.BaseURL(settings.Environment)

	balance, err := woovi.RecipientBalanceCents(settings.AppID, baseURL, driver.PixKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if balance <= 0 {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "saldo insuficiente para saque")
	}

	fee := woovi.WithdrawFeeCents(balance)
	if fee > 0 && settings.PlatformPixKey != "" {
		if err := woovi.RecipientTransfer(settings.AppID, baseURL, driver.PixKey, settings.PlatformPixKey, fee); err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "falha ao cobrar taxa de saque: "+err.Error())
		}
	} else {
		fee = 0
	}

	txID, amount, err := woovi.RecipientWithdraw(settings.AppID, baseURL, driver.PixKey)
	if err != nil {
		if fee > 0 {
			if rollbackErr := woovi.RecipientTransfer(settings.AppID, baseURL, settings.PlatformPixKey, driver.PixKey, fee); rollbackErr != nil {
				log.Printf("driver %s: withdraw failed AND fee rollback failed: %v (original error: %v)", driverID, rollbackErr, err)
			}
		}
		return echo.NewHTTPError(http.StatusBadGateway, "falha ao realizar o saque: "+err.Error())
	}

	note := fmt.Sprintf("Saque via Pix (taxa R$ %.2f)", float64(fee)/100)
	if err := h.billing.RecordWalletTransaction(ctx, driverID, models.WalletTransactionWithdrawal, float64(amount)/100, nil, &txID, note); err != nil {
		log.Printf("driver %s: withdrawal succeeded but failed to record ledger row: %v", driverID, err)
	}

	return c.JSON(http.StatusOK, withdrawResponse{AmountCents: amount, FeeCents: fee, Status: "completed"})
}

// nearbyDriverDTO is deliberately minimal — just enough to place a marker on
// the map. A passenger's or driver's "who's around" view has no business
// seeing the other side's name, plate, rating, etc.
type nearbyDriverDTO struct {
	ID  string  `json:"id"`
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Nearby lists every online driver's current position within radiusKm of
// (lat, lng), for the live ambient map on the passenger's home screen and on
// the driver's own "where's everyone else" view. excludeId (optional) drops
// one driver from the results — a driver looking at this map shouldn't see
// her own marker mixed in with everyone else's.
func (h *DriverHandler) Nearby(c echo.Context) error {
	lat, err := strconv.ParseFloat(c.QueryParam("lat"), 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "lat is required and must be a number")
	}
	lng, err := strconv.ParseFloat(c.QueryParam("lng"), 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "lng is required and must be a number")
	}
	radiusKm := 15.0
	if raw := c.QueryParam("radiusKm"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			radiusKm = parsed
		}
	}
	excludeID := c.QueryParam("excludeId")

	drivers, err := h.drivers.FindAllOnline(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	nearby := make([]nearbyDriverDTO, 0, len(drivers))
	for _, d := range drivers {
		if d.ID == excludeID || d.Lat == nil || d.Lng == nil {
			continue
		}
		if geo.DistanceKm(lat, lng, *d.Lat, *d.Lng) <= radiusKm {
			nearby = append(nearby, nearbyDriverDTO{ID: d.ID, Lat: *d.Lat, Lng: *d.Lng})
		}
	}
	return c.JSON(http.StatusOK, nearby)
}
