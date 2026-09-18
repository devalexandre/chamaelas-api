package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/handlers"
	"chamaelas-api/internal/repository"
)

// newTestApp builds a fully-wired app (same buildApp main() uses) against a
// fresh temp-file SQLite database, migrated from scratch — each test gets
// its own isolated database. A temp file (not ":memory:") is used
// deliberately: SQLite's :memory: database is private per-connection, and
// golang-migrate opens its own connection separate from the app's pool, so
// migrations would silently land in a DB the app can never see.
func newTestApp(t *testing.T) (*echo.Echo, config.Config) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := config.Config{
		Port:                   "0",
		DBDriver:               "sqlite",
		DBDSN:                  dbPath,
		AdminSessionSecret:     "test-secret",
		AdminBootstrapEmail:    "admin@test.com",
		AdminBootstrapPassword: "test1234",
	}
	e, err := buildApp(cfg)
	if err != nil {
		t.Fatalf("buildApp: %v", err)
	}
	return e, cfg
}

// doJSON issues a JSON request against e without a real network listener
// (httptest.NewRecorder + e.ServeHTTP) and decodes the JSON response body
// into out (if non-nil and the body is non-empty).
func doJSON(t *testing.T, e *echo.Echo, method, path string, body any, out any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("unmarshal response body %q: %v", rec.Body.String(), err)
		}
	}
	return rec
}

type testUser struct {
	ID            string  `json:"id"`
	CreditBalance float64 `json:"creditBalance"`
}

type testDriver struct {
	ID string `json:"id"`
}

type testRide struct {
	ID            string  `json:"id"`
	Price         float64 `json:"price"`
	DriverEarning float64 `json:"driverEarning"`
	PaymentMethod string  `json:"paymentMethod"`
	Status        string  `json:"status"`
}

func signupPassenger(t *testing.T, e *echo.Echo, email string) testUser {
	t.Helper()
	var user testUser
	rec := doJSON(t, e, http.MethodPost, "/api/auth/signup", map[string]any{
		"name": "Passageira Teste", "email": email, "phone": "11988888888",
		"cpf": "98765432100", "birthDate": "1995-01-01", "password": "test1234",
	}, &user)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup passenger: status %d body %s", rec.Code, rec.Body.String())
	}
	return user
}

func signupDriver(t *testing.T, e *echo.Echo, email string) testDriver {
	t.Helper()
	var driver testDriver
	rec := doJSON(t, e, http.MethodPost, "/api/driver/auth/signup", map[string]any{
		"name": "Motorista Teste", "email": email, "phone": "11999999999",
		"cpf": "11122233344", "cnh": "123456", "birthDate": "1990-01-01", "password": "test1234",
		"vehiclePlate": "ABC1234", "vehicleModel": "Onix", "vehicleColor": "Prata", "vehicleYear": "2020",
		"categoryIds": []string{"standard"},
	}, &driver)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup driver: status %d body %s", rec.Code, rec.Body.String())
	}
	return driver
}

// approveAndGoOnline mirrors what an admin approval + the driver going
// online in the app does — required before she can be assigned a ride.
func approveAndGoOnline(t *testing.T, e *echo.Echo, driverID string) {
	t.Helper()
	// Directly exercise the admin approval route: log in as the bootstrap
	// admin first, since it's session-cookie protected.
	form := "email=admin%40test.com&password=test1234"
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("admin login: status %d", rec.Code)
	}
	cookies := rec.Result().Cookies()

	req = httptest.NewRequest(http.MethodPost, "/admin/drivers/"+driverID+"/approve", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("approve driver: status %d", rec.Code)
	}

	rec = doJSON(t, e, http.MethodPost, "/api/driver/"+driverID+"/location", map[string]any{
		"lat": -23.5, "lng": -46.6, "isOnline": true,
	}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set driver online: status %d body %s", rec.Code, rec.Body.String())
	}
}

func seedSaoPauloCity(t *testing.T, e *echo.Echo) {
	t.Helper()
	form := "email=admin%40test.com&password=test1234"
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()

	cityForm := "name=S%C3%A3o+Paulo&uf=SP"
	req = httptest.NewRequest(http.MethodPost, "/admin/cities", bytes.NewBufferString(cityForm))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("seed city: status %d body %s", rec.Code, rec.Body.String())
	}
}

func createRide(t *testing.T, e *echo.Echo, userID, paymentMethod string) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{
		"userId":      userID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
	}
	if paymentMethod != "" {
		body["paymentMethod"] = paymentMethod
	}
	return doJSON(t, e, http.MethodPost, "/api/rides", body, nil)
}

func runRideToCompletion(t *testing.T, e *echo.Echo, rideID, driverID string) {
	t.Helper()
	for _, action := range []string{"accept", "arrive", "start", "complete"} {
		rec := doJSON(t, e, http.MethodPost, fmt.Sprintf("/api/rides/%s/%s", rideID, action), map[string]any{"driverId": driverID}, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s ride: status %d body %s", action, rec.Code, rec.Body.String())
		}
	}
}

func TestRidePaidWithCredit_InsufficientBalanceIsRejected(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "pax1@test.com")

	rec := createRide(t, e, passenger.ID, "credit")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for insufficient credit, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRidePaidWithCredit_DebitsPassengerAndCreditsDriverWallet(t *testing.T) {
	e, cfg := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "pax2@test.com")
	driver := signupDriver(t, e, "mot2@test.com")
	approveAndGoOnline(t, e, driver.ID)

	// Give the passenger enough credit directly via the repository layer —
	// there's no "grant credit" HTTP endpoint (credit only ever arrives via
	// a real Woovi top-up + webhook), so a test seeding it directly is the
	// pragmatic way to set up this scenario without a live payment gateway.
	// (Equivalent to what a completed Pix top-up would have done.)
	seedCredit(t, cfg, passenger.ID, 100.0)

	var ride testRide
	rec := doJSON(t, e, http.MethodPost, "/api/rides", map[string]any{
		"userId":        passenger.ID,
		"categoryId":    "standard",
		"origin":        map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination":   map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
		"paymentMethod": "credit",
	}, &ride)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}
	if ride.PaymentMethod != "credit" {
		t.Fatalf("expected paymentMethod \"credit\" on the created ride, got %q", ride.PaymentMethod)
	}

	runRideToCompletion(t, e, ride.ID, driver.ID)

	var profile testUser
	rec = doJSON(t, e, http.MethodPost, "/api/auth/login", map[string]any{"email": "pax2@test.com", "password": "test1234"}, &profile)
	if rec.Code != http.StatusOK {
		t.Fatalf("login passenger: status %d", rec.Code)
	}
	wantBalance := 100.0 - ride.Price
	if diff := profile.CreditBalance - wantBalance; diff > 0.01 || diff < -0.01 {
		t.Errorf("passenger credit balance = %.2f, want %.2f (100 - ride price %.2f)", profile.CreditBalance, wantBalance, ride.Price)
	}

	// The driver's wallet-earning attempt must ALWAYS leave an audit trail —
	// this driver never registered a Pix key, so it should be a recorded
	// skip, not silence (silence is exactly the symptom that made a real
	// production bug undiagnosable).
	var wallet struct {
		Transactions []struct {
			Type   string  `json:"type"`
			Note   string  `json:"note"`
			Amount float64 `json:"amount"`
		} `json:"transactions"`
	}
	rec = doJSON(t, e, http.MethodGet, "/api/driver/"+driver.ID+"/wallet", nil, &wallet)
	if rec.Code != http.StatusOK {
		t.Fatalf("get wallet: status %d body %s", rec.Code, rec.Body.String())
	}
	if len(wallet.Transactions) != 1 {
		t.Fatalf("expected exactly 1 wallet_transactions row for the completed ride, got %d", len(wallet.Transactions))
	}
	if wallet.Transactions[0].Type != "ride_earning" {
		t.Errorf("wallet transaction type = %q, want \"ride_earning\"", wallet.Transactions[0].Type)
	}
	if wallet.Transactions[0].Note == "" {
		t.Error("wallet transaction with no Pix key on file should record a note explaining the skip, got empty note")
	}
}

func TestRidePaidWithCash_DoesNotTouchPassengerCredit(t *testing.T) {
	e, cfg := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "pax3@test.com")
	driver := signupDriver(t, e, "mot3@test.com")
	approveAndGoOnline(t, e, driver.ID)
	seedCredit(t, cfg, passenger.ID, 50.0)

	var ride testRide
	rec := doJSON(t, e, http.MethodPost, "/api/rides", map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
		// no paymentMethod field at all — must default to "cash"
	}, &ride)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}
	if ride.PaymentMethod != "cash" {
		t.Fatalf("expected paymentMethod to default to \"cash\", got %q", ride.PaymentMethod)
	}

	runRideToCompletion(t, e, ride.ID, driver.ID)

	var profile testUser
	rec = doJSON(t, e, http.MethodPost, "/api/auth/login", map[string]any{"email": "pax3@test.com", "password": "test1234"}, &profile)
	if rec.Code != http.StatusOK {
		t.Fatalf("login passenger: status %d", rec.Code)
	}
	if profile.CreditBalance != 50.0 {
		t.Errorf("a cash ride must never touch credit_balance: got %.2f, want 50.00 unchanged", profile.CreditBalance)
	}
}

func TestPerCityCommissionOverridesGlobalRate(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "pax4@test.com")

	// Global default commission is 20% (platform_settings seed) — override
	// São Paulo specifically to 10% and confirm the ride actually uses it.
	form := "email=admin%40test.com&password=test1234"
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()

	req = httptest.NewRequest(http.MethodPost, "/admin/settings/cities/sao-paulo-sp/commission", bytes.NewBufferString("commissionPercent=10"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("set city commission: status %d body %s", rec.Code, rec.Body.String())
	}

	var ride testRide
	rec2 := doJSON(t, e, http.MethodPost, "/api/rides", map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
	}, &ride)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec2.Code, rec2.Body.String())
	}

	got := (ride.Price - ride.DriverEarning) / ride.DriverEarning
	if got < 0.09 || got > 0.11 {
		t.Errorf("platform fee ratio = %.3f, want ~0.10 (the city override), driverEarning=%.2f price=%.2f", got, ride.DriverEarning, ride.Price)
	}
}

// seedCredit grants a passenger credit directly via the same repository
// code the app itself uses (AdjustUserCredit) — the pragmatic stand-in for
// "a real Woovi top-up completed", since these tests have no live payment
// gateway to actually pay a Pix charge against, and crediting is otherwise
// only reachable via the webhook by design (no public "grant credit"
// endpoint exists, nor should one). Opens its own connection to the same
// SQLite file the app under test is using — safe for a file-backed
// database (unlike ":memory:", which is private per-connection).
func seedCredit(t *testing.T, cfg config.Config, userID string, amount float64) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("seedCredit: connect: %v", err)
	}
	defer db.Close()
	userCreditRepo := repository.NewUserCreditRepository(db, cfg)
	if _, err := userCreditRepo.AdjustUserCredit(ctx, userID, amount, "topup", nil, "seed for test"); err != nil {
		t.Fatalf("seedCredit: %v", err)
	}
}

func TestFrequentPlaces_ReturnsOnlyPlacesUsedAtLeastTwice(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "frequent@test.com")

	// "Origem"/"Destino" (createRide's default pair) each ride twice.
	if rec := createRide(t, e, passenger.ID, ""); rec.Code != http.StatusCreated {
		t.Fatalf("create ride 1: status %d body %s", rec.Code, rec.Body.String())
	}
	if rec := createRide(t, e, passenger.ID, ""); rec.Code != http.StatusCreated {
		t.Fatalf("create ride 2: status %d body %s", rec.Code, rec.Body.String())
	}

	// A third ride reuses "Origem" but goes to a destination used nowhere else.
	body := map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Só uma vez"},
	}
	if rec := doJSON(t, e, http.MethodPost, "/api/rides", body, nil); rec.Code != http.StatusCreated {
		t.Fatalf("create ride 3: status %d body %s", rec.Code, rec.Body.String())
	}

	var places []struct {
		Address  struct{ Label string } `json:"address"`
		UseCount int                    `json:"useCount"`
	}
	rec := doJSON(t, e, http.MethodGet, "/api/users/"+passenger.ID+"/frequent-places", nil, &places)
	if rec.Code != http.StatusOK {
		t.Fatalf("frequent places: status %d body %s", rec.Code, rec.Body.String())
	}

	counts := map[string]int{}
	for _, p := range places {
		counts[p.Address.Label] = p.UseCount
	}
	if counts["Origem"] != 3 {
		t.Errorf("Origem useCount = %d, want 3 (used in all 3 rides): %+v", counts["Origem"], places)
	}
	if counts["Destino"] != 2 {
		t.Errorf("Destino useCount = %d, want 2: %+v", counts["Destino"], places)
	}
	if _, ok := counts["Só uma vez"]; ok {
		t.Errorf("a place used only once must not appear in frequent places: %+v", places)
	}
}

func TestRideNote_StoredAndReturnedInOffer(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "note-pax@test.com")
	driver := signupDriver(t, e, "note-driver@test.com")
	approveAndGoOnline(t, e, driver.ID)

	body := map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
		"note":        "  Toque a campainha, apartamento 42  ",
	}
	var ride struct {
		ID   string  `json:"id"`
		Note *string `json:"note"`
	}
	rec := doJSON(t, e, http.MethodPost, "/api/rides", body, &ride)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}
	if ride.Note == nil || *ride.Note != "Toque a campainha, apartamento 42" {
		t.Fatalf("ride note = %v, want trimmed note", ride.Note)
	}

	var offer struct {
		RideID string  `json:"rideId"`
		Note   *string `json:"note"`
	}
	rec = doJSON(t, e, http.MethodGet, "/api/driver/"+driver.ID+"/rides/offer", nil, &offer)
	if rec.Code != http.StatusOK {
		t.Fatalf("get offer: status %d body %s", rec.Code, rec.Body.String())
	}
	if offer.RideID != ride.ID {
		t.Fatalf("offer rideId = %q, want %q", offer.RideID, ride.ID)
	}
	if offer.Note == nil || *offer.Note != "Toque a campainha, apartamento 42" {
		t.Errorf("offer note = %v, want the ride's note", offer.Note)
	}
}

func TestFavoriteDrivers_OfferedFirstThenEveryone(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "fav-pax@test.com")
	favoriteDriver := signupDriver(t, e, "fav-driver@test.com")
	otherDriver := signupDriver(t, e, "other-driver@test.com")
	approveAndGoOnline(t, e, favoriteDriver.ID)
	approveAndGoOnline(t, e, otherDriver.ID)

	rec := doJSON(t, e, http.MethodPost, "/api/users/"+passenger.ID+"/favorite-drivers/"+favoriteDriver.ID, nil, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add favorite: status %d body %s", rec.Code, rec.Body.String())
	}

	rec = createRide(t, e, passenger.ID, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}

	// Still inside the grace window: the non-favorite driver sees nothing...
	var noOffer map[string]any
	rec = doJSON(t, e, http.MethodGet, "/api/driver/"+otherDriver.ID+"/rides/offer", nil, &noOffer)
	if rec.Code != http.StatusOK {
		t.Fatalf("get offer (other driver): status %d body %s", rec.Code, rec.Body.String())
	}
	if noOffer != nil {
		t.Errorf("non-favorite driver should see no offer during the grace window, got %+v", noOffer)
	}

	// ...while the favorite is offered the ride right away.
	var favOffer struct {
		RideID string `json:"rideId"`
	}
	rec = doJSON(t, e, http.MethodGet, "/api/driver/"+favoriteDriver.ID+"/rides/offer", nil, &favOffer)
	if rec.Code != http.StatusOK {
		t.Fatalf("get offer (favorite driver): status %d body %s", rec.Code, rec.Body.String())
	}
	if favOffer.RideID == "" {
		t.Fatal("favorite driver should be offered the ride during the grace window")
	}

	// Once the grace window elapses, the ride opens up to everyone.
	original := handlers.FavoriteDriverGraceWindow
	handlers.FavoriteDriverGraceWindow = 1 * time.Millisecond
	defer func() { handlers.FavoriteDriverGraceWindow = original }()
	time.Sleep(5 * time.Millisecond)

	var opened struct {
		RideID string `json:"rideId"`
	}
	rec = doJSON(t, e, http.MethodGet, "/api/driver/"+otherDriver.ID+"/rides/offer", nil, &opened)
	if rec.Code != http.StatusOK {
		t.Fatalf("get offer (other driver, after window): status %d body %s", rec.Code, rec.Body.String())
	}
	if opened.RideID == "" {
		t.Error("non-favorite driver should be offered the ride once the grace window elapses")
	}
}

func TestRideChat_BlockedBeforeDriverAndAfterCompletion(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "chat-pax@test.com")
	driver := signupDriver(t, e, "chat-driver@test.com")
	approveAndGoOnline(t, e, driver.ID)

	var ride testRide
	rec := doJSON(t, e, http.MethodPost, "/api/rides", map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
	}, &ride)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}

	// No driver assigned yet — nobody to chat with.
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "user", "senderId": passenger.ID, "body": "oi?",
	}, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("send before driver assigned: status %d body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/accept", map[string]any{"driverId": driver.ID}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("accept ride: status %d body %s", rec.Code, rec.Body.String())
	}

	// A passenger message impersonating the driver (or vice versa) is rejected.
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "driver", "senderId": passenger.ID, "body": "não sou a motorista",
	}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("spoofed sender: status %d body %s", rec.Code, rec.Body.String())
	}

	var msg struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "driver", "senderId": driver.ID, "body": "Cheguei, não te encontro, pode confirmar o local?",
	}, &msg)
	if rec.Code != http.StatusCreated {
		t.Fatalf("driver send: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "user", "senderId": passenger.ID, "body": "Estou no portão azul!",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("passenger send: status %d body %s", rec.Code, rec.Body.String())
	}

	var list []struct {
		SenderType string `json:"senderType"`
		Body       string `json:"body"`
	}
	rec = doJSON(t, e, http.MethodGet, "/api/rides/"+ride.ID+"/messages", nil, &list)
	if rec.Code != http.StatusOK {
		t.Fatalf("list messages: status %d body %s", rec.Code, rec.Body.String())
	}
	if len(list) != 2 || list[0].SenderType != "driver" || list[1].SenderType != "user" {
		t.Fatalf("unexpected message list: %+v", list)
	}

	// Already accepted above — runRideToCompletion would re-accept and fail,
	// so the remaining transitions are driven by hand here instead.
	for _, action := range []string{"arrive", "start", "complete"} {
		rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/"+action, map[string]any{"driverId": driver.ID}, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s ride: status %d body %s", action, rec.Code, rec.Body.String())
		}
	}

	// Chat is locked for new messages once the ride ends...
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "user", "senderId": passenger.ID, "body": "Valeu!",
	}, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("send after completion: status %d body %s", rec.Code, rec.Body.String())
	}

	// ...but the existing transcript stays fully readable.
	rec = doJSON(t, e, http.MethodGet, "/api/rides/"+ride.ID+"/messages", nil, &list)
	if rec.Code != http.StatusOK || len(list) != 2 {
		t.Fatalf("transcript after completion: status %d body %v", rec.Code, list)
	}
}

func TestAdminRideChat_RendersTranscript(t *testing.T) {
	e, _ := newTestApp(t)
	seedSaoPauloCity(t, e)
	passenger := signupPassenger(t, e, "admin-chat-pax@test.com")
	driver := signupDriver(t, e, "admin-chat-driver@test.com")
	approveAndGoOnline(t, e, driver.ID)

	var ride testRide
	rec := doJSON(t, e, http.MethodPost, "/api/rides", map[string]any{
		"userId":      passenger.ID,
		"categoryId":  "standard",
		"origin":      map[string]any{"uf": "SP", "city": "São Paulo", "label": "Origem"},
		"destination": map[string]any{"uf": "SP", "city": "São Paulo", "label": "Destino"},
	}, &ride)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ride: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/accept", map[string]any{"driverId": driver.ID}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("accept ride: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e, http.MethodPost, "/api/rides/"+ride.ID+"/messages", map[string]any{
		"senderType": "driver", "senderId": driver.ID, "body": "Cheguei, não te encontro",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("driver send: status %d body %s", rec.Code, rec.Body.String())
	}

	form := "email=admin%40test.com&password=test1234"
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, req)
	cookies := loginRec.Result().Cookies()

	req = httptest.NewRequest(http.MethodGet, "/admin/rides/"+ride.ID+"/chat", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	chatRec := httptest.NewRecorder()
	e.ServeHTTP(chatRec, req)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("admin ride chat: status %d body %s", chatRec.Code, chatRec.Body.String())
	}
	if !strings.Contains(chatRec.Body.String(), "Cheguei, não te encontro") {
		t.Errorf("admin ride chat page should contain the message body, got: %s", chatRec.Body.String())
	}
	if !strings.Contains(chatRec.Body.String(), "Passageira Teste") {
		t.Errorf("admin ride chat page should show the passenger's name, got: %s", chatRec.Body.String())
	}
}
