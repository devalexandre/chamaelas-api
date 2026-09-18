package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var favoriteDriversTable = ksql.NewTable("favorite_drivers", "user_id", "driver_id")

type FavoriteDriverRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewFavoriteDriverRepository(db ksql.DB, cfg config.Config) *FavoriteDriverRepository {
	return &FavoriteDriverRepository{db: db, cfg: cfg}
}

type favoriteDriverRow struct {
	UserID    string    `ksql:"user_id"`
	DriverID  string    `ksql:"driver_id"`
	CreatedAt time.Time `ksql:"created_at"`
}

// IsFavorite reports whether driverID is already one of userID's favorites.
func (r *FavoriteDriverRepository) IsFavorite(ctx context.Context, userID, driverID string) (bool, error) {
	var row favoriteDriverRow
	query := fmt.Sprintf(
		"FROM favorite_drivers WHERE user_id = %s AND driver_id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.QueryOne(ctx, &row, query, userID, driverID)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Add marks driverID as a favorite of userID — idempotent, so a double-tap
// on the star button in the app never surfaces as an error.
func (r *FavoriteDriverRepository) Add(ctx context.Context, userID, driverID string) error {
	isFavorite, err := r.IsFavorite(ctx, userID, driverID)
	if err != nil {
		return err
	}
	if isFavorite {
		return nil
	}
	row := favoriteDriverRow{UserID: userID, DriverID: driverID, CreatedAt: time.Now().UTC()}
	return r.db.Insert(ctx, favoriteDriversTable, &row)
}

func (r *FavoriteDriverRepository) Remove(ctx context.Context, userID, driverID string) error {
	query := fmt.Sprintf(
		"DELETE FROM favorite_drivers WHERE user_id = %s AND driver_id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	_, err := r.db.Exec(ctx, query, userID, driverID)
	return err
}

// ListDriverIDsForUser is the cheap check GetOffer needs on every poll — just
// the IDs, not full driver records.
func (r *FavoriteDriverRepository) ListDriverIDsForUser(ctx context.Context, userID string) ([]string, error) {
	rows := []favoriteDriverRow{}
	query := fmt.Sprintf("FROM favorite_drivers WHERE user_id = %s ORDER BY created_at DESC", database.Placeholder(r.cfg, 1))
	if err := r.db.Query(ctx, &rows, query, userID); err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.DriverID
	}
	return ids, nil
}

// ListDriversForUser returns the full driver records for a "Favoritos"
// screen — joined and ordered by when each was favorited, most recent first.
func (r *FavoriteDriverRepository) ListDriversForUser(ctx context.Context, userID string) ([]models.Driver, error) {
	drivers := []models.Driver{}
	query := fmt.Sprintf(
		"SELECT d.* FROM drivers d JOIN favorite_drivers f ON f.driver_id = d.id "+
			"WHERE f.user_id = %s ORDER BY f.created_at DESC",
		database.Placeholder(r.cfg, 1),
	)
	err := r.db.Query(ctx, &drivers, query, userID)
	return drivers, err
}
