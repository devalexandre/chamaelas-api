package handlers

import (
	"context"
	"log"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
	"chamaelas-api/internal/woovi"
)

// creditDriverWalletEarning transfers a completed ride's earning into the
// driver's Woovi revenue wallet, sourced from fromPixKey — wherever the
// platform actually collected the fare for that specific ride (the
// passenger's own subaccount for a "credit" ride, the platform's
// operational subaccount for a "pix" ride). It's only ever called once
// that money has actually arrived there (see RideHandler.settleDriverEarning
// and WebhookHandler.handleRidePayment). A "cash" ride never reaches this
// function: the driver already has that money in hand directly from the
// passenger, so crediting the wallet too would pay her twice for the same
// ride.
func creditDriverWalletEarning(
	ctx context.Context,
	billing *repository.BillingRepository,
	wooviSettings *repository.WooviSettingsRepository,
	driver *models.Driver,
	fromPixKey string,
	rideID string,
	driverEarning float64,
) {
	if driver.PixKey == "" {
		if err := billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, driverEarning, &rideID, nil, "sem chave Pix cadastrada"); err != nil {
			log.Printf("ride %s: failed to record skipped wallet credit: %v", rideID, err)
		}
		return
	}
	if fromPixKey == "" {
		if err := billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, driverEarning, &rideID, nil, "sem subconta de origem configurada"); err != nil {
			log.Printf("ride %s: failed to record skipped wallet credit: %v", rideID, err)
		}
		return
	}

	settings, err := wooviSettings.Get(ctx)
	if err != nil || settings.AppID == "" {
		if err := billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, driverEarning, &rideID, nil, "gateway de pagamento não configurado"); err != nil {
			log.Printf("ride %s: failed to record skipped wallet credit: %v", rideID, err)
		}
		return
	}

	cents := int(driverEarning * 100)
	baseURL := woovi.BaseURL(settings.Environment)
	if err := woovi.RecipientTransfer(settings.AppID, baseURL, fromPixKey, driver.PixKey, cents); err != nil {
		log.Printf("ride %s: failed to credit driver %s's wallet: %v", rideID, driver.ID, err)
		if err := billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, driverEarning, &rideID, nil, "falha ao transferir: "+err.Error()); err != nil {
			log.Printf("ride %s: failed to record failed wallet credit: %v", rideID, err)
		}
		return
	}

	if err := billing.RecordWalletTransaction(ctx, driver.ID, models.WalletTransactionRideEarning, driverEarning, &rideID, nil, ""); err != nil {
		log.Printf("ride %s: wallet credited but failed to record ledger row: %v", rideID, err)
	}
}
