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

type WooviSettingsRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewWooviSettingsRepository(db ksql.DB, cfg config.Config) *WooviSettingsRepository {
	return &WooviSettingsRepository{db: db, cfg: cfg}
}

func (r *WooviSettingsRepository) Get(ctx context.Context) (*models.WooviSettings, error) {
	var settings models.WooviSettings
	query := fmt.Sprintf("FROM woovi_settings WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &settings, query, 1)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *WooviSettingsRepository) Update(ctx context.Context, environment, appID, webhookSecret, webhookPublicKeyB64, platformPixKey string) error {
	query := fmt.Sprintf(
		"UPDATE woovi_settings SET environment = %s, app_id = %s, webhook_secret = %s, webhook_public_key_b64 = %s, platform_pix_key = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4),
		database.Placeholder(r.cfg, 5), database.Placeholder(r.cfg, 6),
	)
	_, err := r.db.Exec(ctx, query, environment, appID, webhookSecret, webhookPublicKeyB64, platformPixKey, 1)
	return err
}
