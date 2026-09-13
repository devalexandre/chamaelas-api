package admin

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/pagarme"
)

func (m *Module) Billing(c echo.Context) error {
	ctx := c.Request().Context()

	drivers, err := m.drivers.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	paymentSettings, err := m.payments.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	gatewayFeeRates, err := m.gatewayFees.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	wooviSettings, err := m.woovi.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var wallet *pagarme.Balance
	var walletError string
	if paymentSettings.SecretKey != "" && paymentSettings.PlatformRecipientID != "" {
		wallet, err = pagarme.GetRecipientBalance(paymentSettings.SecretKey, paymentSettings.PlatformRecipientID)
		if err != nil {
			walletError = err.Error()
		}
	}

	return render(c, "billing", "billing.html", map[string]any{
		"Drivers":         drivers,
		"Payment":         paymentSettings,
		"Woovi":           wooviSettings,
		"Wallet":          wallet,
		"WalletError":     walletError,
		"GatewayFeeRates": gatewayFeeRates,
	})
}

func (m *Module) AdjustDriverCredit(c echo.Context) error {
	driverID := c.Param("id")
	amount, err := strconv.ParseFloat(c.FormValue("amount"), 64)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/admin/billing")
	}

	if _, err := m.billing.AdjustCredit(
		c.Request().Context(), driverID, amount, models.CreditTransactionAdjustment, nil, "Ajuste manual pelo admin",
	); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/billing")
}

func (m *Module) UpdateGatewaySettings(c echo.Context) error {
	environment := c.FormValue("environment")
	if environment != "sandbox" && environment != "production" {
		environment = "sandbox"
	}
	publicKey := c.FormValue("publicKey")
	secretKey := c.FormValue("secretKey")
	platformRecipientID := c.FormValue("platformRecipientId")

	// Don't overwrite the stored secret key when the field is left blank —
	// the form never echoes it back in full, only a masked hint.
	if secretKey == "" {
		current, err := m.payments.Get(c.Request().Context())
		if err == nil {
			secretKey = current.SecretKey
		}
	}

	if err := m.payments.Update(c.Request().Context(), environment, publicKey, secretKey, platformRecipientID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/billing")
}

// UpdateWooviSettings saves the Woovi (PIX) gateway credentials. Like
// UpdateGatewaySettings above, appId and webhookSecret are masked in the
// form (blank-to-keep) rather than echoed back in full.
func (m *Module) UpdateWooviSettings(c echo.Context) error {
	ctx := c.Request().Context()
	environment := c.FormValue("environment")
	if environment != "sandbox" && environment != "production" {
		environment = "sandbox"
	}
	appID := c.FormValue("appId")
	webhookSecret := c.FormValue("webhookSecret")
	webhookPublicKeyB64 := c.FormValue("webhookPublicKeyB64")
	platformPixKey := c.FormValue("platformPixKey")

	current, err := m.woovi.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if appID == "" {
		appID = current.AppID
	}
	if webhookSecret == "" {
		webhookSecret = current.WebhookSecret
	}
	if webhookPublicKeyB64 == "" {
		webhookPublicKeyB64 = current.WebhookPublicKeyB64
	}

	if err := m.woovi.Update(ctx, environment, appID, webhookSecret, webhookPublicKeyB64, platformPixKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/billing")
}

// UpdateGatewayFeeRates saves what Pagar.me itself charges per payment
// method — one <input name="feePercent[<id>]"> per rate row.
func (m *Module) UpdateGatewayFeeRates(c echo.Context) error {
	ctx := c.Request().Context()
	rates, err := m.gatewayFees.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	for _, rate := range rates {
		value := c.FormValue("feePercent[" + rate.ID + "]")
		if value == "" {
			continue
		}
		percent, err := strconv.ParseFloat(value, 64)
		if err != nil || percent < 0 {
			continue
		}
		if err := m.gatewayFees.SetFeePercent(ctx, rate.ID, percent); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	return c.Redirect(http.StatusSeeOther, "/admin/billing")
}
