package handlers

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

type WebhookHandler struct {
	wooviSettings *repository.WooviSettingsRepository
	billing       *repository.BillingRepository
	userCredit    *repository.UserCreditRepository
}

func NewWebhookHandler(wooviSettings *repository.WooviSettingsRepository, billing *repository.BillingRepository, userCredit *repository.UserCreditRepository) *WebhookHandler {
	return &WebhookHandler{wooviSettings: wooviSettings, billing: billing, userCredit: userCredit}
}

// HandlePaymentWebhook receives Woovi's payment notifications at
// /webhooks/payment (registered on the root router, not under /api — this
// exact path is already configured in Woovi's dashboard). Order matters:
// the shared-secret check happens before anything else touches the body,
// then the payload is parsed, then its signature is verified, and only
// after all of that is it recorded for dedup — verifying BEFORE dedup means
// a forged request with a real correlationID can't "burn" that event key
// and block the real delivery from ever being processed.
func (h *WebhookHandler) HandlePaymentWebhook(c echo.Context) error {
	ctx := c.Request().Context()

	settings, err := h.wooviSettings.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.WebhookSecret == "" {
		// Hard-required in every environment, including sandbox — this is a
		// real production payment path, unlike a local-dev convenience.
		log.Printf("webhook: rejected, woovi_settings.webhook_secret is not configured")
		return echo.NewHTTPError(http.StatusUnauthorized, "webhook not configured")
	}
	got := strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(settings.WebhookSecret)) != 1 {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid webhook token")
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	evt, err := woovi.ParseWebhookEvent(body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if evt.Type == woovi.EventIgnored {
		return c.NoContent(http.StatusOK)
	}

	if err := woovi.VerifyWebhookSignature(body, c.Request().Header.Get("x-webhook-signature"), settings.WebhookPublicKeyB64); err != nil {
		log.Printf("webhook: signature verification failed: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid signature")
	}

	fresh, err := h.billing.RecordWebhookEventIfFresh(ctx, "woovi", evt.EventKey, string(body))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if !fresh {
		return c.NoContent(http.StatusOK)
	}

	switch {
	case strings.HasPrefix(evt.CorrelationID, "driver-topup-"):
		h.handleDriverTopup(ctx, evt)
	case strings.HasPrefix(evt.CorrelationID, "passenger-topup-"):
		h.handlePassengerTopup(ctx, evt)
	default:
		log.Printf("webhook: unknown correlationID %q, ignoring", evt.CorrelationID)
	}

	return c.NoContent(http.StatusOK)
}

func (h *WebhookHandler) handleDriverTopup(ctx context.Context, evt *woovi.WebhookEvent) {
	topup, err := h.billing.FindCreditTopup(ctx, evt.CorrelationID)
	if err != nil {
		log.Printf("webhook: driver topup %s not found: %v", evt.CorrelationID, err)
		return
	}

	switch evt.Type {
	case woovi.EventPaid:
		note := fmt.Sprintf("Recarga via Pix (%s)", evt.CorrelationID)
		if _, err := h.billing.AdjustCredit(ctx, topup.DriverID, float64(topup.AmountCents)/100, models.CreditTransactionTopup, nil, note); err != nil {
			log.Printf("webhook: failed to credit driver %s for topup %s: %v", topup.DriverID, evt.CorrelationID, err)
			return
		}
		if err := h.billing.MarkCreditTopupPaid(ctx, evt.CorrelationID); err != nil {
			log.Printf("webhook: failed to mark topup %s paid: %v", evt.CorrelationID, err)
		}
	case woovi.EventExpired:
		if err := h.billing.MarkCreditTopupExpired(ctx, evt.CorrelationID); err != nil {
			log.Printf("webhook: failed to mark topup %s expired: %v", evt.CorrelationID, err)
		}
	}
}

func (h *WebhookHandler) handlePassengerTopup(ctx context.Context, evt *woovi.WebhookEvent) {
	topup, err := h.userCredit.FindTopup(ctx, evt.CorrelationID)
	if err != nil {
		log.Printf("webhook: passenger topup %s not found: %v", evt.CorrelationID, err)
		return
	}

	switch evt.Type {
	case woovi.EventPaid:
		note := fmt.Sprintf("Recarga via Pix (%s)", evt.CorrelationID)
		if _, err := h.userCredit.AdjustUserCredit(ctx, topup.UserID, float64(topup.AmountCents)/100, models.UserCreditTopupType, nil, note); err != nil {
			log.Printf("webhook: failed to credit passenger %s for topup %s: %v", topup.UserID, evt.CorrelationID, err)
			return
		}
		if err := h.userCredit.MarkTopupPaid(ctx, evt.CorrelationID); err != nil {
			log.Printf("webhook: failed to mark topup %s paid: %v", evt.CorrelationID, err)
		}
	case woovi.EventExpired:
		if err := h.userCredit.MarkTopupExpired(ctx, evt.CorrelationID); err != nil {
			log.Printf("webhook: failed to mark topup %s expired: %v", evt.CorrelationID, err)
		}
	}
}
