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

func (r *DriverRepository) FindByGoogleSub(ctx context.Context, sub string) (*models.Driver, error) {
	var driver models.Driver
	query := fmt.Sprintf("FROM drivers WHERE google_sub = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &driver, query, sub)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) SetGoogleSub(ctx context.Context, id, sub string) error {
	query := fmt.Sprintf("UPDATE drivers SET google_sub = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, sub, id)
	return err
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

// FindAllOnline lists every approved driver who's both flagged online *and*
// has pinged her location recently (see onlineStaleness) — used for the
// ambient "nearby drivers" map shown to passengers before requesting a
// ride, the admin dashboard's live map, and a driver wanting to see where
// other drivers currently are. Without the freshness check, a driver whose
// app died without cleanly going offline would show up as online forever.
func (r *DriverRepository) FindAllOnline(ctx context.Context) ([]models.Driver, error) {
	drivers := []models.Driver{}
	query := fmt.Sprintf(
		"SELECT * FROM drivers WHERE is_online = %s AND status = %s "+
			"AND lat IS NOT NULL AND lng IS NOT NULL AND location_updated_at > %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	err := r.db.Query(ctx, &drivers, query, true, string(models.DriverStatusApproved), time.Now().UTC().Add(-models.OnlineStaleness))
	return drivers, err
}
