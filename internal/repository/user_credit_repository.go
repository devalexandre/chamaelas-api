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

// UserCreditRepository is the passenger-side mirror of BillingRepository's
// driver-credit methods: a spend-only prepaid balance (pays for ride
// prices), never withdrawable.
type UserCreditRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewUserCreditRepository(db ksql.DB, cfg config.Config) *UserCreditRepository {
	return &UserCreditRepository{db: db, cfg: cfg}
}

var userCreditTransactionsTable = ksql.NewTable("user_credit_transactions", "id")

// AdjustUserCredit applies a signed amount to a passenger's prepaid balance
// and records it in the ledger, atomically — the passenger-side equivalent
// of BillingRepository.AdjustCredit.
func (r *UserCreditRepository) AdjustUserCredit(ctx context.Context, userID string, amount float64, txType string, rideID *string, note string) (float64, error) {
	var newBalance float64

	err := r.db.Transaction(ctx, func(tx ksql.Provider) error {
		var user models.User
		selectQuery := fmt.Sprintf("FROM users WHERE id = %s", database.Placeholder(r.cfg, 1))
		if err := tx.QueryOne(ctx, &user, selectQuery, userID); err != nil {
			return err
		}

		newBalance = user.CreditBalance + amount

		updateQuery := fmt.Sprintf("UPDATE users SET credit_balance = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
		if _, err := tx.Exec(ctx, updateQuery, newBalance, userID); err != nil {
			return err
		}

		entry := &models.UserCreditTransaction{
			ID:           uuid.NewString(),
			UserID:       userID,
			Type:         txType,
			Amount:       amount,
			RideID:       rideID,
			BalanceAfter: newBalance,
			Note:         note,
			CreatedAt:    time.Now().UTC(),
		}
		return tx.Insert(ctx, userCreditTransactionsTable, entry)
	})

	return newBalance, err
}

func (r *UserCreditRepository) ListTransactions(ctx context.Context, userID string) ([]models.UserCreditTransaction, error) {
	transactions := []models.UserCreditTransaction{}
	query := fmt.Sprintf("FROM user_credit_transactions WHERE user_id = %s ORDER BY created_at DESC", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &transactions, query, userID)
	return transactions, err
}

var userCreditTopupsTable = ksql.NewTable("user_credit_topups", "id")

// CreateTopup records a pending credit top-up charge. id must be the exact
// correlationID sent to Woovi, so the webhook can look the row up by it.
func (r *UserCreditRepository) CreateTopup(ctx context.Context, id, userID string, amountCents int) error {
	topup := &models.UserCreditTopup{
		ID:          id,
		UserID:      userID,
		AmountCents: amountCents,
		Status:      string(models.CreditTopupPending),
		CreatedAt:   time.Now().UTC(),
	}
	return r.db.Insert(ctx, userCreditTopupsTable, topup)
}

func (r *UserCreditRepository) FindTopup(ctx context.Context, id string) (*models.UserCreditTopup, error) {
	var topup models.UserCreditTopup
	query := fmt.Sprintf("FROM user_credit_topups WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &topup, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &topup, nil
}

func (r *UserCreditRepository) MarkTopupPaid(ctx context.Context, id string) error {
	query := fmt.Sprintf(
		"UPDATE user_credit_topups SET status = %s, paid_at = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	_, err := r.db.Exec(ctx, query, string(models.CreditTopupPaid), time.Now().UTC(), id)
	return err
}

func (r *UserCreditRepository) MarkTopupExpired(ctx context.Context, id string) error {
	query := fmt.Sprintf("UPDATE user_credit_topups SET status = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, string(models.CreditTopupExpired), id)
	return err
}
