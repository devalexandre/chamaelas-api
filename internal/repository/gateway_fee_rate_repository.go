package repository

import (
	"context"
	"fmt"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

type GatewayFeeRateRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewGatewayFeeRateRepository(db ksql.DB, cfg config.Config) *GatewayFeeRateRepository {
	return &GatewayFeeRateRepository{db: db, cfg: cfg}
}

func (r *GatewayFeeRateRepository) ListAll(ctx context.Context) ([]models.GatewayFeeRate, error) {
	rates := []models.GatewayFeeRate{}
	err := r.db.Query(ctx, &rates, "FROM gateway_fee_rates ORDER BY sort_order")
	return rates, err
}

func (r *GatewayFeeRateRepository) SetFeePercent(ctx context.Context, id string, feePercent float64) error {
	query := fmt.Sprintf("UPDATE gateway_fee_rates SET fee_percent = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, feePercent, id)
	return err
}
