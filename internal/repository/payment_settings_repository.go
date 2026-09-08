package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

type PaymentSettingsRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewPaymentSettingsRepository(db ksql.DB, cfg config.Config) *PaymentSettingsRepository {
	return &PaymentSettingsRepository{db: db, cfg: cfg}
}

func (r *PaymentSettingsRepository) Get(ctx context.Context) (*models.PaymentSettings, error) {
	var settings models.PaymentSettings
	query := fmt.Sprintf("FROM payment_settings WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &settings, query, 1)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *PaymentSettingsRepository) Update(ctx context.Context, environment, publicKey, secretKey, platformRecipientID string) error {
	query := fmt.Sprintf(
		"UPDATE payment_settings SET environment = %s, public_key = %s, secret_key = %s, platform_recipient_id = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5),
	)
	_, err := r.db.Exec(ctx, query, environment, publicKey, secretKey, platformRecipientID, 1)
	return err
}
