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
