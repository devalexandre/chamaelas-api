package admin

import (
	"context"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/pagarme"
	"chamaelas-api/internal/woovi"
)

func (m *Module) ListDrivers(c echo.Context) error {
	ctx := c.Request().Context()
	drivers, err := m.drivers.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	statusFilter := c.QueryParam("status")
	if statusFilter != "" {
		filtered := make([]models.Driver, 0, len(drivers))
		for _, d := range drivers {
			if d.Status == statusFilter {
				filtered = append(filtered, d)
			}
		}
		drivers = filtered
	}

	return render(c, "drivers", "drivers.html", map[string]any{
		"Drivers":      drivers,
		"StatusFilter": statusFilter,
	})
}

// driverOnlineStatus is the minimal shape the Motoristas list polls (every
// few seconds) to keep the Online/Offline pill live without a full reload
// or re-fetching every other column.
type driverOnlineStatus struct {
	ID       string `json:"id"`
	IsOnline bool   `json:"isOnline"`
}

func (m *Module) DriverOnlineStatuses(c echo.Context) error {
	drivers, err := m.drivers.ListAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	statuses := make([]driverOnlineStatus, len(drivers))
	for i, d := range drivers {
		statuses[i] = driverOnlineStatus{ID: d.ID, IsOnline: d.IsReallyOnline()}
	}
	return c.JSON(http.StatusOK, statuses)
}

func (m *Module) ViewDriver(c echo.Context) error {
	ctx := c.Request().Context()

	driver, err := m.drivers.FindByID(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "motorista não encontrada")
	}
	categories, _ := m.categories.ListByDriver(ctx, driver.ID)
	transactions, err := m.billing.ListTransactions(ctx, driver.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rides, err := m.rides.ListByDriver(ctx, driver.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	walletTransactions, err := m.billing.ListWalletTransactions(ctx, driver.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Best-effort: the live Woovi balance shouldn't break this whole page if
	// the gateway isn't configured yet or is briefly unreachable.
	hasWalletBalance := false
	walletBalance := 0.0
	if driver.PixKey != "" {
		if settings, err := m.woovi.Get(ctx); err == nil && settings.AppID != "" {
			if cents, err := woovi.RecipientBalanceCents(settings.AppID, woovi.BaseURL(settings.Environment), driver.PixKey); err == nil {
				hasWalletBalance = true
				walletBalance = float64(cents) / 100
			}
		}
	}

	return render(c, "drivers", "driver_view.html", map[string]any{
		"Driver":             driver,
		"Categories":         categories,
		"Transactions":       transactions,
		"Rides":              rides,
		"WalletTransactions": walletTransactions,
		"HasWalletBalance":   hasWalletBalance,
		"WalletBalance":      walletBalance,
	})
}

func (m *Module) ChangeDriverPassword(c echo.Context) error {
	driverID := c.Param("id")
	newPassword := c.FormValue("newPassword")
	if len(newPassword) < 6 {
		return echo.NewHTTPError(http.StatusBadRequest, "a senha deve ter pelo menos 6 caracteres")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if err := m.drivers.SetPasswordHash(c.Request().Context(), driverID, string(hash)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/drivers/"+driverID)
}

func (m *Module) EditDriverPage(c echo.Context) error {
	driver, err := m.drivers.FindByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "motorista não encontrada")
	}
	return render(c, "drivers", "driver_edit.html", map[string]any{"Driver": driver})
}

func (m *Module) UpdateDriver(c echo.Context) error {
	ctx := c.Request().Context()
	driver, err := m.drivers.FindByID(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "motorista não encontrada")
	}

	driver.Name = c.FormValue("name")
	driver.Phone = c.FormValue("phone")
	driver.VehiclePlate = c.FormValue("vehiclePlate")
	driver.VehicleModel = c.FormValue("vehicleModel")
	driver.VehicleColor = c.FormValue("vehicleColor")
	driver.VehicleYear = c.FormValue("vehicleYear")
	driver.BankCode = c.FormValue("bankCode")
	driver.BankBranch = c.FormValue("bankBranch")
	driver.BankAccountNumber = c.FormValue("bankAccountNumber")
	driver.BankAccountType = c.FormValue("bankAccountType")

	if err := m.drivers.UpdateProfile(ctx, driver); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if billingMode := c.FormValue("billingMode"); billingMode != "" {
		if err := m.billing.SetDriverBillingMode(ctx, driver.ID, models.BillingMode(billingMode)); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return c.Redirect(http.StatusSeeOther, "/admin/drivers/"+driver.ID)
}

func (m *Module) ApproveDriver(c echo.Context) error {
	ctx := c.Request().Context()
	driverID := c.Param("id")

	if err := m.drivers.SetStatus(ctx, driverID, models.DriverStatusApproved); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Best-effort: onboard her as a Pagar.me Recebedor so future ride
	// payments can be split to her. This never blocks the approval itself —
	// a missing gateway key or missing bank info just means the recipient
	// gets created later once that's filled in (re-approve, or a future
	// "criar recebedor" retry button).
	go m.createRecipientForDriver(driverID)

	return c.Redirect(http.StatusSeeOther, "/admin/drivers")
}

func (m *Module) createRecipientForDriver(driverID string) {
	ctx := context.Background()
	driver, err := m.drivers.FindByID(ctx, driverID)
	if err != nil || driver.PagarmeRecipientID != "" {
		return
	}
	settings, err := m.payments.Get(ctx)
	if err != nil || settings.SecretKey == "" {
		log.Printf("driver %s: skipping Pagar.me recipient creation (gateway not configured)", driverID)
		return
	}
	if driver.BankCode == "" || driver.BankAccountNumber == "" {
		log.Printf("driver %s: skipping Pagar.me recipient creation (no bank account on file)", driverID)
		return
	}

	recipient, err := pagarme.CreateRecipient(settings.SecretKey, pagarme.RecipientInput{
		Name:              driver.Name,
		Email:             driver.Email,
		Document:          driver.CPF,
		BankCode:          driver.BankCode,
		BankBranch:        driver.BankBranch,
		BankAccountNumber: driver.BankAccountNumber,
		BankAccountType:   driver.BankAccountType,
	})
	if err != nil {
		log.Printf("driver %s: failed to create Pagar.me recipient: %v", driverID, err)
		return
	}

	if err := m.drivers.SetPagarmeRecipientID(ctx, driverID, recipient.ID); err != nil {
		log.Printf("driver %s: created Pagar.me recipient %s but failed to save it: %v", driverID, recipient.ID, err)
	}
}

func (m *Module) BlockDriver(c echo.Context) error {
	if err := m.drivers.SetStatus(c.Request().Context(), c.Param("id"), models.DriverStatusBlocked); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/drivers")
}
