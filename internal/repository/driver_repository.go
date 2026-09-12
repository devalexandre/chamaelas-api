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

var driversTable = ksql.NewTable("drivers", "id")

type DriverRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewDriverRepository(db ksql.DB, cfg config.Config) *DriverRepository {
	return &DriverRepository{db: db, cfg: cfg}
}

func (r *DriverRepository) Create(ctx context.Context, driver *models.Driver) error {
	return r.db.Insert(ctx, driversTable, driver)
}

func (r *DriverRepository) FindByEmail(ctx context.Context, email string) (*models.Driver, error) {
	var driver models.Driver
	query := fmt.Sprintf("FROM drivers WHERE email = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &driver, query, email)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) SetPasswordHash(ctx context.Context, id, passwordHash string) error {
	query := fmt.Sprintf("UPDATE drivers SET password_hash = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, passwordHash, id)
	return err
}

func (r *DriverRepository) ListAll(ctx context.Context) ([]models.Driver, error) {
	drivers := []models.Driver{}
	err := r.db.Query(ctx, &drivers, "FROM drivers ORDER BY created_at DESC")
	return drivers, err
}

func (r *DriverRepository) SetStatus(ctx context.Context, id string, status models.DriverStatus) error {
	query := fmt.Sprintf("UPDATE drivers SET status = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, string(status), id)
	return err
}

func (r *DriverRepository) FindByID(ctx context.Context, id string) (*models.Driver, error) {
	var driver models.Driver
	query := fmt.Sprintf("FROM drivers WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &driver, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

// SetLocation is called by the driver app while online (periodic GPS ping)
// and whenever the driver flips the online/offline switch.
func (r *DriverRepository) SetLocation(ctx context.Context, driverID string, lat, lng float64, isOnline bool) error {
	query := fmt.Sprintf(
		"UPDATE drivers SET lat = %s, lng = %s, is_online = %s, location_updated_at = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5),
	)
	_, err := r.db.Exec(ctx, query, lat, lng, isOnline, time.Now().UTC(), driverID)
	return err
}

func (r *DriverRepository) SetPagarmeRecipientID(ctx context.Context, driverID, recipientID string) error {
	query := fmt.Sprintf("UPDATE drivers SET pagarme_recipient_id = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, recipientID, driverID)
	return err
}

// UpdateProfile applies the fields an admin can edit from the panel:
// contact/vehicle info and bank account (categories and billing mode have
// their own dedicated updates).
func (r *DriverRepository) UpdateProfile(ctx context.Context, driver *models.Driver) error {
	query := fmt.Sprintf(
		"UPDATE drivers SET name = %s, phone = %s, vehicle_plate = %s, vehicle_model = %s, "+
			"vehicle_color = %s, vehicle_year = %s, bank_code = %s, bank_branch = %s, "+
			"bank_account_number = %s, bank_account_type = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
		database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5), database.Placeholder(r.cfg, 6),
		database.Placeholder(r.cfg, 7), database.Placeholder(r.cfg, 8), database.Placeholder(r.cfg, 9),
		database.Placeholder(r.cfg, 10), database.Placeholder(r.cfg, 11),
	)
	_, err := r.db.Exec(ctx, query,
		driver.Name, driver.Phone, driver.VehiclePlate, driver.VehicleModel,
		driver.VehicleColor, driver.VehicleYear, driver.BankCode, driver.BankBranch,
		driver.BankAccountNumber, driver.BankAccountType, driver.ID,
	)
	return err
}

func (r *DriverRepository) SetOnline(ctx context.Context, driverID string, isOnline bool) error {
	query := fmt.Sprintf("UPDATE drivers SET is_online = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, isOnline, driverID)
	return err
}

// FindOnlineByCategory lists every approved, online driver with a known
// location that can serve the given category and isn't already on another
// ride — the ride matcher then ranks these by distance itself (see
// internal/matching). Excluding drivers with a live ride matters more now
// than it used to: a driver mid-ride stays "online" the whole time, and
// without this she could be offered — and accept — a second, overlapping
// ride.
func (r *DriverRepository) FindOnlineByCategory(ctx context.Context, categoryID string) ([]models.Driver, error) {
	drivers := []models.Driver{}
	query := fmt.Sprintf(
		"SELECT d.* FROM drivers d "+
			"JOIN driver_categories dc ON dc.driver_id = d.id "+
			"WHERE dc.category_id = %s AND d.is_online = %s AND d.status = %s "+
			"AND d.lat IS NOT NULL AND d.lng IS NOT NULL "+
			"AND NOT EXISTS (SELECT 1 FROM rides r WHERE r.driver_id = d.id AND r.status IN (%s, %s, %s))",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
		database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5), database.Placeholder(r.cfg, 6),
	)
	err := r.db.Query(ctx, &drivers, query, categoryID, true, string(models.DriverStatusApproved),
		string(models.RideStatusAccepted), string(models.RideStatusArrived), string(models.RideStatusInProgress))
	return drivers, err
}

// FindAllOnline lists every approved, online driver with a known location,
// regardless of category — used for the ambient "nearby drivers" map shown
// to passengers before requesting a ride, and to a driver wanting to see
// where other drivers currently are.
func (r *DriverRepository) FindAllOnline(ctx context.Context) ([]models.Driver, error) {
	drivers := []models.Driver{}
	query := fmt.Sprintf(
		"SELECT * FROM drivers WHERE is_online = %s AND status = %s "+
			"AND lat IS NOT NULL AND lng IS NOT NULL",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.Query(ctx, &drivers, query, true, string(models.DriverStatusApproved))
	return drivers, err
}
