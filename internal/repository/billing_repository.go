package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

type BillingRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewBillingRepository(db ksql.DB, cfg config.Config) *BillingRepository {
	return &BillingRepository{db: db, cfg: cfg}
}

func (r *BillingRepository) GetSettings(ctx context.Context) (*models.PlatformSettings, error) {
	var settings models.PlatformSettings
	query := fmt.Sprintf("FROM platform_settings WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &settings, query, 1)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *BillingRepository) SetCommissionRate(ctx context.Context, rate float64) error {
	query := fmt.Sprintf("UPDATE platform_settings SET commission_rate = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, rate, 1)
	return err
}

func (r *BillingRepository) SetMapPollSeconds(ctx context.Context, seconds int) error {
	query := fmt.Sprintf("UPDATE platform_settings SET map_poll_seconds = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, seconds, 1)
	return err
}

func (r *BillingRepository) SetDriverBillingMode(ctx context.Context, driverID string, mode models.BillingMode) error {
	query := fmt.Sprintf("UPDATE drivers SET billing_mode = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, string(mode), driverID)
	return err
}

func (r *BillingRepository) ListTransactions(ctx context.Context, driverID string) ([]models.CreditTransaction, error) {
	transactions := []models.CreditTransaction{}
	query := fmt.Sprintf("FROM credit_transactions WHERE driver_id = %s ORDER BY created_at DESC", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &transactions, query, driverID)
	return transactions, err
}

// AdjustCredit applies a signed amount to a driver's prepaid balance
// (positive for a topup/adjustment credit, negative for a debit such as a
// ride's commission fee) and records it in the ledger, atomically.
func (r *BillingRepository) AdjustCredit(ctx context.Context, driverID string, amount float64, txType models.CreditTransactionType, rideID *string, note string) (float64, error) {
	var newBalance float64

	err := r.db.Transaction(ctx, func(tx ksql.Provider) error {
		var driver models.Driver
		selectQuery := fmt.Sprintf("FROM drivers WHERE id = %s", database.Placeholder(r.cfg, 1))
		if err := tx.QueryOne(ctx, &driver, selectQuery, driverID); err != nil {
			return err
		}

		newBalance = driver.CreditBalance + amount

		updateQuery := fmt.Sprintf("UPDATE drivers SET credit_balance = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
		if _, err := tx.Exec(ctx, updateQuery, newBalance, driverID); err != nil {
			return err
		}

		entry := &models.CreditTransaction{
			ID:           uuid.NewString(),
			DriverID:     driverID,
			Type:         string(txType),
			Amount:       amount,
			RideID:       rideID,
			BalanceAfter: newBalance,
			Note:         note,
			CreatedAt:    time.Now().UTC(),
		}
		return tx.Insert(ctx, creditTransactionsTable, entry)
	})

	return newBalance, err
}

var creditTransactionsTable = ksql.NewTable("credit_transactions", "id")

func (r *BillingRepository) SetDriverPixKey(ctx context.Context, driverID, pixKey string) error {
	query := fmt.Sprintf("UPDATE drivers SET pix_key = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, pixKey, driverID)
	return err
}

var walletTransactionsTable = ksql.NewTable("wallet_transactions", "id")

// RecordWalletTransaction writes one audit row to a driver's revenue-wallet
// ledger. The wallet's spendable balance is never derived from this table
// (it's always read live from Woovi) — this is purely for visibility, so a
// skipped/failed credit shows up as a reconcilable row instead of silently
// vanishing.
func (r *BillingRepository) RecordWalletTransaction(ctx context.Context, driverID string, txType models.WalletTransactionType, amount float64, rideID *string, providerTransactionID *string, note string) error {
	entry := &models.WalletTransaction{
		ID:                    uuid.NewString(),
		DriverID:              driverID,
		Type:                  string(txType),
		Amount:                amount,
		RideID:                rideID,
		ProviderTransactionID: providerTransactionID,
		Note:                  note,
		CreatedAt:             time.Now().UTC(),
	}
	return r.db.Insert(ctx, walletTransactionsTable, entry)
}

func (r *BillingRepository) ListWalletTransactions(ctx context.Context, driverID string) ([]models.WalletTransaction, error) {
	transactions := []models.WalletTransaction{}
	query := fmt.Sprintf("FROM wallet_transactions WHERE driver_id = %s ORDER BY created_at DESC", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &transactions, query, driverID)
	return transactions, err
}

var creditTopupsTable = ksql.NewTable("credit_topups", "id")

// CreateCreditTopup records a pending credit top-up charge. id must be the
// exact correlationID sent to Woovi, so the webhook can look the row up by
// it later.
func (r *BillingRepository) CreateCreditTopup(ctx context.Context, id, driverID string, amountCents int) error {
	topup := &models.CreditTopup{
		ID:          id,
		DriverID:    driverID,
		AmountCents: amountCents,
		Status:      string(models.CreditTopupPending),
		CreatedAt:   time.Now().UTC(),
	}
	return r.db.Insert(ctx, creditTopupsTable, topup)
}

func (r *BillingRepository) FindCreditTopup(ctx context.Context, id string) (*models.CreditTopup, error) {
	var topup models.CreditTopup
	query := fmt.Sprintf("FROM credit_topups WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &topup, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &topup, nil
}

func (r *BillingRepository) MarkCreditTopupPaid(ctx context.Context, id string) error {
	query := fmt.Sprintf(
		"UPDATE credit_topups SET status = %s, paid_at = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	_, err := r.db.Exec(ctx, query, string(models.CreditTopupPaid), time.Now().UTC(), id)
	return err
}

func (r *BillingRepository) MarkCreditTopupExpired(ctx context.Context, id string) error {
	query := fmt.Sprintf("UPDATE credit_topups SET status = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, string(models.CreditTopupExpired), id)
	return err
}

var webhookEventsTable = ksql.NewTable("webhook_events", "id")

// RecordWebhookEventIfFresh records a webhook delivery and reports whether
// it's the first time this exact (provider, eventKey) has been seen — a
// redelivery of the same event returns fresh=false so the caller can
// no-op instead of double-processing it (e.g. double-crediting a top-up).
// The check-then-insert isn't atomic; a genuinely concurrent duplicate
// delivery would hit the table's unique index and surface as an Insert
// error rather than a clean fresh=false — acceptable for how Woovi actually
// retries (sequential, backed off), not worth a cross-DB upsert here.
func (r *BillingRepository) RecordWebhookEventIfFresh(ctx context.Context, provider, eventKey, payload string) (bool, error) {
	checkQuery := fmt.Sprintf(
		"FROM webhook_events WHERE provider = %s AND event_key = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	var existing models.WebhookEventRecord
	err := r.db.QueryOne(ctx, &existing, checkQuery, provider, eventKey)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, ksql.ErrRecordNotFound) {
		return false, err
	}

	record := &models.WebhookEventRecord{
		ID:        uuid.NewString(),
		Provider:  provider,
		EventKey:  eventKey,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.Insert(ctx, webhookEventsTable, record); err != nil {
		return false, err
	}
	return true, nil
}
