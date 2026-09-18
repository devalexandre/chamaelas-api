package repository

import (
	"context"
	"fmt"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var rideMessagesTable = ksql.NewTable("ride_messages", "id")

type RideMessageRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewRideMessageRepository(db ksql.DB, cfg config.Config) *RideMessageRepository {
	return &RideMessageRepository{db: db, cfg: cfg}
}

func (r *RideMessageRepository) Create(ctx context.Context, msg *models.RideMessage) error {
	return r.db.Insert(ctx, rideMessagesTable, msg)
}

func (r *RideMessageRepository) ListByRide(ctx context.Context, rideID string) ([]models.RideMessage, error) {
	// Initialized (not nil) so an empty result serializes as `[]`, not
	// `null` — matches the convention in RideRepository.ListByUser.
	messages := []models.RideMessage{}
	query := fmt.Sprintf("FROM ride_messages WHERE ride_id = %s ORDER BY created_at ASC", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &messages, query, rideID)
	return messages, err
}
