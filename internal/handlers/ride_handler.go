package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/geo"
	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

const (
	baseFare     = 5.0
	pricePerKm   = 2.3
	minutesPerKm = 2.1
)

type RideHandler struct {
	rides         *repository.RideRepository
	drivers       *repository.DriverRepository
	users         *repository.UserRepository
	billing       *repository.BillingRepository
	cities        *repository.CityRepository
	categories    *repository.CategoryRepository
	wooviSettings *repository.WooviSettingsRepository
}

func NewRideHandler(rides *repository.RideRepository, drivers *repository.DriverRepository, users *repository.UserRepository, billing *repository.BillingRepository, cities *repository.CityRepository, categories *repository.CategoryRepository, wooviSettings *repository.WooviSettingsRepository) *RideHandler {
	return &RideHandler{rides: rides, drivers: drivers, users: users, billing: billing, cities: cities, categories: categories, wooviSettings: wooviSettings}
}

// normalizeCityText makes city-name comparisons ignore case and the accents
// that vary between how a driver types a city in the admin panel and how a
// geocoder (Nominatim/Photon) spells it back.
func normalizeCityText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	return replacer.Replace(s)
}

// findCityForAddress matches addr against the admin panel's active cities —
// used both to reject a ride from a city that isn't registered ("app not
// available here yet") and to resolve that city's own commission-rate
// override, if it has one.
func (h *RideHandler) findCityForAddress(ctx context.Context, addr models.Address) (*models.City, error) {
	cities, err := h.cities.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	addrCity := normalizeCityText(addr.City)
	addrUF := strings.ToLower(strings.TrimSpace(addr.UF))
	for i := range cities {
		if normalizeCityText(cities[i].Name) == addrCity && strings.ToLower(cities[i].UF) == addrUF {
			return &cities[i], nil
		}
	}
	return nil, nil
}

type createRideRequest struct {
	UserID      string         `json:"userId"`
	CategoryID  string         `json:"categoryId"`
	Origin      models.Address `json:"origin"`
	Destination models.Address `json:"destination"`
}

// estimateRide computes distance/duration and driverEarning (the base
// fare, before any platform commission). The commission is layered on top
// separately by the caller — see Create.
func estimateRide(origin, destination models.Address) (distanceKm float64, durationMin int, driverEarning float64) {
	seed := float64((len(origin.Label) + len(destination.Label)) % 10)
	distanceKm = float64(int((2+seed+rand.Float64()*6)*10)) / 10
	durationMin = int(distanceKm * minutesPerKm)
	driverEarning = float64(int((baseFare+distanceKm*pricePerKm)*100)) / 100
	return
}

func (h *RideHandler) Create(c echo.Context) error {
	var req createRideRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.UserID == "" || req.Origin.Label == "" || req.Destination.Label == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "userId, origin and destination are required")
	}
	if req.CategoryID == "" {
		req.CategoryID = "standard"
	}

	ctx := c.Request().Context()
	city, err := h.findCityForAddress(ctx, req.Origin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if city == nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, fmt.Sprintf("o Chama Elas ainda não está disponível em %s/%s", req.Origin.City, req.Origin.UF))
	}

	distanceKm, durationMin, driverEarning := estimateRide(req.Origin, req.Destination)

	commissionRate := 0.0
	if settings, err := h.billing.GetSettings(ctx); err == nil {
		commissionRate = settings.CommissionRate
	}
	if city.CommissionRate != nil {
		commissionRate = *city.CommissionRate
	}
	platformFee := float64(int(driverEarning*commissionRate*100)) / 100
	price := driverEarning + platformFee

	ride := &models.Ride{
		ID:            uuid.NewString(),
		UserID:        req.UserID,
		CategoryID:    req.CategoryID,
		Origin:        models.NewJSON(req.Origin),
		Destination:   models.NewJSON(req.Destination),
		DistanceKm:    distanceKm,
		DurationMin:   durationMin,
		Price:         price,
		DriverEarning: driverEarning,
		PlatformFee:   platformFee,
		Status:        string(models.RideStatusSearching),
		CreatedAt:     time.Now().UTC(),
	}

	if err := h.rides.Create(ctx, ride); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, ride)
}

func (h *RideHandler) attachDriver(ctx context.Context, ride *models.Ride) {
	if ride.DriverID == nil {
		return
	}
	driver, err := h.drivers.FindByID(ctx, *ride.DriverID)
	if err != nil {
		return
	}
	if driver.Lat != nil && driver.Lng != nil {
		distanceKm := geo.DistanceKm(*driver.Lat, *driver.Lng, ride.Origin.Data.Lat, ride.Origin.Data.Lng)
		etaMin := int(distanceKm*minutesPerKm + 0.5)
		if etaMin < 1 {
			etaMin = 1
		}
		driver.EtaMin = &etaMin
	}
	ride.Driver = driver
}

// attachUser attaches the passenger's own profile (name, photo — used by
// the driver's accepted-ride screen) — deliberately not called from the
// pre-accept offer, which has no business showing passenger identity before
// she's committed to the ride.
func (h *RideHandler) attachUser(ctx context.Context, ride *models.Ride) {
	user, err := h.users.FindByID(ctx, ride.UserID)
	if err != nil {
		return
	}
	ride.User = user
}

func (h *RideHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	ride, err := h.rides.FindByID(ctx, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	h.attachDriver(ctx, ride)
	return c.JSON(http.StatusOK, ride)
}

func (h *RideHandler) ListByUser(c echo.Context) error {
	ctx := c.Request().Context()

	if driverID := c.QueryParam("driverId"); driverID != "" {
		rides, err := h.rides.ListByDriver(ctx, driverID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, rides)
	}

	userID := c.QueryParam("userId")
	if userID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "userId or driverId query param is required")
	}

	rides, err := h.rides.ListByUser(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	for i := range rides {
		h.attachDriver(ctx, &rides[i])
	}
	return c.JSON(http.StatusOK, rides)
}

// CurrentForDriver lets the driver app poll for a ride it just got matched
// to (or resume one already in progress after an app restart).
func (h *RideHandler) CurrentForDriver(c echo.Context) error {
	ctx := c.Request().Context()
	ride, err := h.rides.FindActiveByDriver(ctx, c.Param("driverId"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusOK, nil)
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	h.attachUser(ctx, ride)
	return c.JSON(http.StatusOK, ride)
}

func (h *RideHandler) Cancel(c echo.Context) error {
	ctx := c.Request().Context()
	ride, err := h.rides.FindByID(ctx, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if ride.Status != string(models.RideStatusSearching) && ride.Status != string(models.RideStatusAccepted) {
		return echo.NewHTTPError(http.StatusConflict, "ride can no longer be cancelled")
	}
	if err := h.rides.UpdateStatus(ctx, ride.ID, models.RideStatusCancelled); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// GetOffer returns the closest still-searching ride in one of this driver's
// categories that she hasn't already declined — nothing is ever pushed to
// her, so there's no timeout and no exclusive claim: every other eligible
// driver sees the exact same ride at the same time, until one of them
// accepts it (see Accept) or the passenger cancels it. Only what's needed
// to decide is returned, nothing more.
func (h *RideHandler) GetOffer(c echo.Context) error {
	ctx := c.Request().Context()
	driverID := c.Param("driverId")

	// A driver already on a ride has nothing to be offered.
	if _, err := h.rides.FindActiveByDriver(ctx, driverID); err == nil {
		return c.JSON(http.StatusOK, nil)
	}

	driver, err := h.drivers.FindByID(ctx, driverID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	categories, err := h.categories.ListByDriver(ctx, driverID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	categoryIDs := make([]string, len(categories))
	for i, cat := range categories {
		categoryIDs[i] = cat.ID
	}

	candidates, err := h.rides.FindSearchingForCategories(ctx, categoryIDs, driverID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if len(candidates) == 0 {
		return c.JSON(http.StatusOK, nil)
	}

	nearest := candidates[0]
	if driver.Lat != nil && driver.Lng != nil {
		nearestDist := geo.DistanceKm(*driver.Lat, *driver.Lng, nearest.Origin.Data.Lat, nearest.Origin.Data.Lng)
		for _, ride := range candidates[1:] {
			dist := geo.DistanceKm(*driver.Lat, *driver.Lng, ride.Origin.Data.Lat, ride.Origin.Data.Lng)
			if dist < nearestDist {
				nearest, nearestDist = ride, dist
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"rideId":        nearest.ID,
		"origin":        nearest.Origin.Data,
		"destination":   nearest.Destination.Data,
		"distanceKm":    nearest.DistanceKm,
		"driverEarning": nearest.DriverEarning,
	})
}

type driverActionRequest struct {
	DriverID string `json:"driverId"`
}

// Accept claims a still-searching ride for this driver. Several drivers can
// see (and try to accept) the same ride at once — AssignDriver's UPDATE is
// the single source of truth for who actually wins that race; whoever's
// UPDATE doesn't match a row (because someone else's got there first) gets
// told so instead of ending up double-booked.
func (h *RideHandler) Accept(c echo.Context) error {
	var req driverActionRequest
	if err := c.Bind(&req); err != nil || req.DriverID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "driverId is required")
	}
	ctx := c.Request().Context()
	won, err := h.rides.AssignDriver(ctx, c.Param("id"), req.DriverID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if !won {
		return echo.NewHTTPError(http.StatusConflict, "esta corrida já foi aceita por outra motorista")
	}
	return c.NoContent(http.StatusNoContent)
}

// Decline records that this driver passed on this ride — she won't be
// offered it again, but it stays visible to every other eligible driver.
func (h *RideHandler) Decline(c echo.Context) error {
	var req driverActionRequest
	if err := c.Bind(&req); err != nil || req.DriverID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "driverId is required")
	}
	if err := h.rides.RecordDecline(c.Request().Context(), c.Param("id"), req.DriverID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// requireAssignedDriver is the MVP stand-in for real auth: it just checks
// that the caller claims to be the driver already assigned to this ride.
func requireAssignedDriver(ride *models.Ride, driverID string) error {
	if driverID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "driverId is required")
	}
	if ride.DriverID == nil || *ride.DriverID != driverID {
		return echo.NewHTTPError(http.StatusForbidden, "you are not the driver assigned to this ride")
	}
	return nil
}

// transition validates and applies a driver-triggered status change,
// returning the ride so callers can react to a successful change (e.g.
// Complete debits the driver's prepaid balance) before writing a response.
func (h *RideHandler) transition(c echo.Context, from models.RideStatus, to models.RideStatus) (*models.Ride, error) {
	var req driverActionRequest
	if err := c.Bind(&req); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	ride, err := h.rides.FindByID(ctx, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if err := requireAssignedDriver(ride, req.DriverID); err != nil {
		return nil, err
	}
	if ride.Status != string(from) {
		return nil, echo.NewHTTPError(http.StatusConflict, "ride is not in the expected status")
	}

	if err := h.rides.UpdateStatus(ctx, ride.ID, to); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	ride.Status = string(to)
	return ride, nil
}

// Arrive: the driver marks she has reached the pickup point.
func (h *RideHandler) Arrive(c echo.Context) error {
	if _, err := h.transition(c, models.RideStatusAccepted, models.RideStatusArrived); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Start: the driver marks the trip as started (passenger is on board).
func (h *RideHandler) Start(c echo.Context) error {
	if _, err := h.transition(c, models.RideStatusArrived, models.RideStatusInProgress); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Complete: the driver marks the trip as finished. For a driver on the
// prepaid plan, the platform's commission is debited from her credit
// balance right here — under the per-ride plan the fee is split out at
// the payment gateway instead (once that's wired up), so nothing local
// to deduct.
func (h *RideHandler) Complete(c echo.Context) error {
	ride, err := h.transition(c, models.RideStatusInProgress, models.RideStatusCompleted)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	if err := h.rides.SetFinalPrice(ctx, ride.ID, ride.Price); err != nil {
		log.Printf("ride %s: failed to set final price: %v", ride.ID, err)
	}

	if ride.DriverID != nil {
		driver, err := h.drivers.FindByID(ctx, *ride.DriverID)
		// Either way the driver nets ride.DriverEarning: a per_ride driver
		// gets it directly from the gateway split (platform keeps its cut
		// from the charge); a prepaid driver receives the full ride.Price
		// and has ride.PlatformFee debited from her balance right here —
		// same result, different collection point.
		if err == nil && driver.BillingMode == string(models.BillingModePrepaid) {
			fee := -ride.PlatformFee
			note := fmt.Sprintf("Taxa da corrida %s", ride.ID)
			if _, err := h.billing.AdjustCredit(ctx, driver.ID, fee, models.CreditTransactionRideFee, &ride.ID, note); err != nil {
				log.Printf("ride %s: failed to debit prepaid credit: %v", ride.ID, err)
			}
		}
		if err == nil {
			h.creditWalletEarning(ctx, driver, ride)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// creditWalletEarning credits a completed ride's DriverEarning into the
// driver's Woovi revenue wallet — separate from (and unrelated to) the
// prepaid credit_balance debited above. Every driver nets DriverEarning
// regardless of billing mode, so this always runs. A wallet_transactions
// row is written either way: with the real provider_transaction_id on
// success, or with a note explaining why on skip/failure (no Pix key yet,
// gateway not configured, transfer error) — log-and-continue, matching the
// rest of this function; no retry/queue for this MVP.
func (h *RideHandler) creditWalletEarning(ctx context.Context, driver *models.Driver, ride *models.Ride) {
	if driver.PixKey == "" {
		if err := h.billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, ride.DriverEarning, &ride.ID, nil, "sem chave Pix cadastrada"); err != nil {
			log.Printf("ride %s: failed to record skipped wallet credit: %v", ride.ID, err)
		}
		return
	}

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil || settings.AppID == "" || settings.PlatformPixKey == "" {
		if err := h.billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, ride.DriverEarning, &ride.ID, nil, "gateway de pagamento não configurado"); err != nil {
			log.Printf("ride %s: failed to record skipped wallet credit: %v", ride.ID, err)
		}
		return
	}

	cents := int(ride.DriverEarning * 100)
	baseURL := woovi.BaseURL(settings.Environment)
	if err := woovi.RecipientTransfer(settings.AppID, baseURL, settings.PlatformPixKey, driver.PixKey, cents); err != nil {
		log.Printf("ride %s: failed to credit driver %s's wallet: %v", ride.ID, driver.ID, err)
		if err := h.billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, ride.DriverEarning, &ride.ID, nil, "falha ao transferir: "+err.Error()); err != nil {
			log.Printf("ride %s: failed to record failed wallet credit: %v", ride.ID, err)
		}
		return
	}

	if err := h.billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, ride.DriverEarning, &ride.ID, nil, ""); err != nil {
		log.Printf("ride %s: wallet credited but failed to record ledger row: %v", ride.ID, err)
	}
}

type rateRideRequest struct {
	Rating int `json:"rating"`
}

func (h *RideHandler) Rate(c echo.Context) error {
	var req rateRideRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Rating < 1 || req.Rating > 5 {
		return echo.NewHTTPError(http.StatusBadRequest, "rating must be between 1 and 5")
	}

	ctx := c.Request().Context()
	ride, err := h.rides.FindByID(ctx, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "ride not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if ride.Status != string(models.RideStatusCompleted) {
		return echo.NewHTTPError(http.StatusConflict, "ride is not completed yet")
	}

	if err := h.rides.SetRating(ctx, ride.ID, req.Rating); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
