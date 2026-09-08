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

var ridesTable = ksql.NewTable("rides", "id")

type RideRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewRideRepository(db ksql.DB, cfg config.Config) *RideRepository {
	return &RideRepository{db: db, cfg: cfg}
}

func (r *RideRepository) Create(ctx context.Context, ride *models.Ride) error {
	return r.db.Insert(ctx, ridesTable, ride)
}

func (r *RideRepository) FindByID(ctx context.Context, id string) (*models.Ride, error) {
	var ride models.Ride
	query := fmt.Sprintf("FROM rides WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &ride, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

func (r *RideRepository) ListByUser(ctx context.Context, userID string) ([]models.Ride, error) {
	// Initialized (not nil) so an empty result serializes as `[]`, not `null`
	// — a nil slice breaks any JSON consumer that calls .length/.map on it.
	rides := []models.Ride{}
	query := fmt.Sprintf("FROM rides WHERE user_id = %s ORDER BY created_at DESC", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &rides, query, userID)
	return rides, err
}

// FindActiveByDriver returns the ride currently assigned to this driver, if
// any (status accepted/arrived/in_progress) — the driver app polls this to
// notice a new match without needing push notifications yet.
func (r *RideRepository) FindActiveByDriver(ctx context.Context, driverID string) (*models.Ride, error) {
	var ride models.Ride
	query := fmt.Sprintf(
		"FROM rides WHERE driver_id = %s AND status IN (%s, %s, %s) ORDER BY created_at DESC",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4),
	)
	err := r.db.QueryOne(ctx, &ride, query, driverID,
		string(models.RideStatusAccepted), string(models.RideStatusArrived), string(models.RideStatusInProgress))
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

// FindSearchingByCategory lists rides still waiting for a driver, used to
// retry matching for a driver who just went online.
func (r *RideRepository) FindSearchingByCategory(ctx context.Context, categoryID string) ([]models.Ride, error) {
	rides := []models.Ride{}
	query := fmt.Sprintf(
		"FROM rides WHERE status = %s AND category_id = %s ORDER BY created_at",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.Query(ctx, &rides, query, string(models.RideStatusSearching), categoryID)
	return rides, err
}

func (r *RideRepository) ListByDriver(ctx context.Context, driverID string) ([]models.Ride, error) {
	rides := []models.Ride{}
	query := fmt.Sprintf(
		"FROM rides WHERE driver_id = %s ORDER BY created_at DESC",
		database.Placeholder(r.cfg, 1),
	)
	err := r.db.Query(ctx, &rides, query, driverID)
	return rides, err
}

func (r *RideRepository) AssignDriver(ctx context.Context, rideID, driverID string) error {
	query := fmt.Sprintf(
		"UPDATE rides SET driver_id = %s, status = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	_, err := r.db.Exec(ctx, query, driverID, string(models.RideStatusAccepted), rideID)
	return err
}

func (r *RideRepository) UpdateStatus(ctx context.Context, rideID string, status models.RideStatus) error {
	query := fmt.Sprintf(
		"UPDATE rides SET status = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	_, err := r.db.Exec(ctx, query, string(status), rideID)
	return err
}

// SetFinalPrice records the ride's fare as of completion — separate from
// Price (the quote at request time) so a future fare-adjustment feature has
// somewhere real to write to; today it's always set equal to Price.
func (r *RideRepository) SetFinalPrice(ctx context.Context, rideID string, finalPrice float64) error {
	query := fmt.Sprintf("UPDATE rides SET final_price = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, finalPrice, rideID)
	return err
}

func (r *RideRepository) SetRating(ctx context.Context, rideID string, rating int) error {
	query := fmt.Sprintf(
		"UPDATE rides SET rating = %s, status = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	_, err := r.db.Exec(ctx, query, rating, string(models.RideStatusCompleted), rideID)
	return err
}

// Reports only count completed rides — a cancelled or still-in-progress
// ride hasn't actually generated revenue yet.

// DateRange narrows a report to rides created on or between these dates
// ("YYYY-MM-DD"); either side left blank means unbounded on that end.
type DateRange struct {
	From string
	To   string
}

// dateRangeFilter appends "AND <dateExpr> >= ?" / "<= ?" clauses for
// whichever bounds are set, returning the extra SQL and its args so callers
// can splice it into their own WHERE clause and argument list.
func (r *RideRepository) dateRangeFilter(dateExpr string, dr DateRange, nextPos int) (clause string, args []interface{}) {
	if dr.From != "" {
		clause += fmt.Sprintf(" AND %s >= %s", dateExpr, database.Placeholder(r.cfg, nextPos))
		args = append(args, dr.From)
		nextPos++
	}
	if dr.To != "" {
		clause += fmt.Sprintf(" AND %s <= %s", dateExpr, database.Placeholder(r.cfg, nextPos))
		args = append(args, dr.To)
		nextPos++
	}
	return clause, args
}

// ListDetailedByDateRange returns every ride created within the range
// (either bound optional), one row per ride, with the passenger's and
// driver's names joined in — this backs the "por data" report, which is a
// transaction listing rather than a grouped summary.
func (r *RideRepository) ListDetailedByDateRange(ctx context.Context, dr DateRange) ([]models.RideDetailRow, error) {
	rows := []models.RideDetailRow{}
	dateExpr := database.DateOnly(r.cfg, "r.created_at")
	var args []interface{}
	where := "1=1"
	extra, extraArgs := r.dateRangeFilter(dateExpr, dr, 1)
	where += extra
	args = append(args, extraArgs...)

	query := fmt.Sprintf(
		"SELECT r.id as id, r.created_at as created_at, u.name as customer_name, d.name as driver_name, "+
			"r.payment_method as payment_method, r.price as price, r.final_price as final_price, "+
			"r.amount_paid as amount_paid, r.status as status "+
			"FROM rides r "+
			"JOIN users u ON u.id = r.user_id "+
			"LEFT JOIN drivers d ON d.id = r.driver_id "+
			"WHERE %s ORDER BY r.created_at DESC",
		where,
	)
	err := r.db.Query(ctx, &rows, query, args...)
	return rows, err
}

func (r *RideRepository) ReportByDate(ctx context.Context, dr DateRange) ([]models.RideReportRow, error) {
	rows := []models.RideReportRow{}
	dateExpr := database.DateOnly(r.cfg, "created_at")
	args := []interface{}{string(models.RideStatusCompleted)}
	extra, extraArgs := r.dateRangeFilter(dateExpr, dr, 2)
	args = append(args, extraArgs...)
	query := fmt.Sprintf(
		"SELECT %s as group_key, COUNT(*) as ride_count, "+
			"COALESCE(SUM(price),0) as total_price, COALESCE(SUM(driver_earning),0) as total_driver_earning, "+
			"COALESCE(SUM(platform_fee),0) as total_platform_fee "+
			"FROM rides WHERE status = %s%s GROUP BY %s ORDER BY %s DESC",
		dateExpr, database.Placeholder(r.cfg, 1), extra, dateExpr, dateExpr,
	)
	err := r.db.Query(ctx, &rows, query, args...)
	return rows, err
}

func (r *RideRepository) ReportByCategory(ctx context.Context, dr DateRange) ([]models.RideReportRow, error) {
	rows := []models.RideReportRow{}
	dateExpr := database.DateOnly(r.cfg, "r.created_at")
	args := []interface{}{string(models.RideStatusCompleted)}
	extra, extraArgs := r.dateRangeFilter(dateExpr, dr, 2)
	args = append(args, extraArgs...)
	query := fmt.Sprintf(
		"SELECT c.name as group_key, COUNT(*) as ride_count, "+
			"COALESCE(SUM(r.price),0) as total_price, COALESCE(SUM(r.driver_earning),0) as total_driver_earning, "+
			"COALESCE(SUM(r.platform_fee),0) as total_platform_fee "+
			"FROM rides r JOIN categories c ON c.id = r.category_id "+
			"WHERE r.status = %s%s GROUP BY c.name ORDER BY total_price DESC",
		database.Placeholder(r.cfg, 1), extra,
	)
	err := r.db.Query(ctx, &rows, query, args...)
	return rows, err
}

func (r *RideRepository) ReportByPaymentMethod(ctx context.Context, dr DateRange) ([]models.RideReportRow, error) {
	rows := []models.RideReportRow{}
	dateExpr := database.DateOnly(r.cfg, "created_at")
	args := []interface{}{string(models.RideStatusCompleted)}
	extra, extraArgs := r.dateRangeFilter(dateExpr, dr, 2)
	args = append(args, extraArgs...)
	query := fmt.Sprintf(
		"SELECT COALESCE(payment_method, 'Não informado') as group_key, COUNT(*) as ride_count, "+
			"COALESCE(SUM(price),0) as total_price, COALESCE(SUM(driver_earning),0) as total_driver_earning, "+
			"COALESCE(SUM(platform_fee),0) as total_platform_fee "+
			"FROM rides WHERE status = %s%s GROUP BY COALESCE(payment_method, 'Não informado') ORDER BY total_price DESC",
		database.Placeholder(r.cfg, 1), extra,
	)
	err := r.db.Query(ctx, &rows, query, args...)
	return rows, err
}
