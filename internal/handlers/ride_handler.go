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

	"chamaelas-api/internal/matching"
	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
)

// Short delay before the first matching attempt, just so the "procurando
// motorista" state is visible for a beat instead of resolving instantly.
const matchingDelay = 1500 * time.Millisecond

const (
	baseFare     = 5.0
	pricePerKm   = 2.3
	minutesPerKm = 2.1
)

type RideHandler struct {
	rides   *repository.RideRepository
	drivers *repository.DriverRepository
	billing *repository.BillingRepository
	cities  *repository.CityRepository
}

func NewRideHandler(rides *repository.RideRepository, drivers *repository.DriverRepository, billing *repository.BillingRepository, cities *repository.CityRepository) *RideHandler {
	return &RideHandler{rides: rides, drivers: drivers, billing: billing, cities: cities}
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

// cityIsServed checks the origin against the admin panel's active cities —
// requesting a ride from a city that isn't registered there is rejected,
// same as any other "app not available here yet" gate.
func (h *RideHandler) cityIsServed(ctx context.Context, origin models.Address) (bool, error) {
	cities, err := h.cities.ListActive(ctx)
	if err != nil {
		return false, err
	}
	originCity := normalizeCityText(origin.City)
	originUF := strings.ToLower(strings.TrimSpace(origin.UF))
	for _, city := range cities {
		if normalizeCityText(city.Name) == originCity && strings.ToLower(city.UF) == originUF {
			return true, nil
		}
	}
	return false, nil
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
	served, err := h.cityIsServed(ctx, req.Origin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if !served {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, fmt.Sprintf("o Chama Elas ainda não está disponível em %s/%s", req.Origin.City, req.Origin.UF))
	}

	distanceKm, durationMin, driverEarning := estimateRide(req.Origin, req.Destination)

	commissionRate := 0.0
	if settings, err := h.billing.GetSettings(ctx); err == nil {
		commissionRate = settings.CommissionRate
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

	go h.tryMatch(ride.ID, req.Origin, req.CategoryID)

	return c.JSON(http.StatusCreated, ride)
}

// tryMatch looks for the nearest online driver in the ride's category,
// expanding the search radius until one is found. There's no real driver
// app push mechanism yet, so a driver who's already offline when this runs
// only gets a chance again if RetryMatching is called after they log in —
// see DriverHandler.GoOnline.
func (h *RideHandler) tryMatch(rideID string, origin models.Address, categoryID string) {
	time.Sleep(matchingDelay)
	h.matchOnce(context.Background(), rideID, origin, categoryID)
}

func (h *RideHandler) matchOnce(ctx context.Context, rideID string, origin models.Address, categoryID string) bool {
	candidates, err := h.drivers.FindOnlineByCategory(ctx, categoryID)
	if err != nil {
		log.Printf("ride %s: failed to list online drivers: %v", rideID, err)
		return false
	}

	driver, ok := matching.Nearest(origin, candidates)
	if !ok {
		return false
	}

	if err := h.rides.AssignDriver(ctx, rideID, driver.ID); err != nil {
		log.Printf("ride %s: failed to assign driver %s: %v", rideID, driver.ID, err)
		return false
	}
	return true
}

// RetryMatchingForCategory is called when a driver comes online, so rides
// that were left "searching" because nobody was available get matched
// without the passenger having to re-request.
func (h *RideHandler) RetryMatchingForCategory(ctx context.Context, categoryID string) {
	rides, err := h.rides.FindSearchingByCategory(ctx, categoryID)
	if err != nil || len(rides) == 0 {
		return
	}
	for _, ride := range rides {
		h.matchOnce(ctx, ride.ID, ride.Origin.Data, categoryID)
	}
}

func (h *RideHandler) attachDriver(ctx context.Context, ride *models.Ride) {
	if ride.DriverID == nil {
		return
	}
	driver, err := h.drivers.FindByID(ctx, *ride.DriverID)
	if err != nil {
		return
	}
	ride.Driver = driver
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

type driverActionRequest struct {
	DriverID string `json:"driverId"`
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
	}

	return c.NoContent(http.StatusNoContent)
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
