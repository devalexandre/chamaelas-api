package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

func (r *RideRepository) ListByDriver(ctx context.Context, driverID string) ([]models.Ride, error) {
	rides := []models.Ride{}
	query := fmt.Sprintf(
		"FROM rides WHERE driver_id = %s ORDER BY created_at DESC",
		database.Placeholder(r.cfg, 1),
	)
	err := r.db.Query(ctx, &rides, query, driverID)
	return rides, err
}

// AssignDriver finalizes a ride to the driver who accepted it — guarded so
// that of several drivers racing to accept the same ride, only the first
// one's UPDATE actually matches a row (status must still be "searching" and
// no driver assigned yet); everyone else's AssignDriver call affects zero
// rows and reports it via the bool return, letting the caller tell a driver
// "someone else already took this one" instead of silently double-booking.
func (r *RideRepository) AssignDriver(ctx context.Context, rideID, driverID string) (bool, error) {
	query := fmt.Sprintf(
		"UPDATE rides SET driver_id = %s, status = %s WHERE id = %s AND status = %s AND driver_id IS NULL",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4),
	)
	result, err := r.db.Exec(ctx, query, driverID, string(models.RideStatusAccepted), rideID, string(models.RideStatusSearching))
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// RecordDecline marks that this driver passed on this ride — she won't be
// shown it again, but every other eligible driver still sees it until
// someone accepts.
func (r *RideRepository) RecordDecline(ctx context.Context, rideID, driverID string) error {
	query := fmt.Sprintf(
		"INSERT INTO ride_declines (ride_id, driver_id, created_at) VALUES (%s, %s, %s)",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
	)
	_, err := r.db.Exec(ctx, query, rideID, driverID, time.Now().UTC())
	return err
}

// FindNearestSearchingForDriver returns the closest still-searching ride in
// one of these categories that this driver hasn't declined, or ErrNotFound
// if there's nothing for her right now. Distance is computed by the caller
// (Go-side haversine) since candidate counts are small at this scale —
// there's no need for a spatial SQL extension.
func (r *RideRepository) FindSearchingForCategories(ctx context.Context, categoryIDs []string, driverID string) ([]models.Ride, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}
	rides := []models.Ride{}
	placeholders := make([]string, len(categoryIDs))
	args := []interface{}{string(models.RideStatusSearching)}
	for i, id := range categoryIDs {
		placeholders[i] = database.Placeholder(r.cfg, i+2)
		args = append(args, id)
	}
	args = append(args, driverID)
	query := fmt.Sprintf(
		"FROM rides WHERE status = %s AND category_id IN (%s) "+
			"AND NOT EXISTS (SELECT 1 FROM ride_declines rd WHERE rd.ride_id = rides.id AND rd.driver_id = %s)",
		database.Placeholder(r.cfg, 1), strings.Join(placeholders, ", "), database.Placeholder(r.cfg, len(categoryIDs)+2),
	)
	err := r.db.Query(ctx, &rides, query, args...)
	return rides, err
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

// Stats summarizes ride activity within dr (unbounded = all-time) — total
// rides, how many completed/cancelled, and revenue from the completed ones.
// Backs the admin dashboard's "hoje" and "total" stat rows.
func (r *RideRepository) Stats(ctx context.Context, dr DateRange) (models.DashboardStats, error) {
	var stats models.DashboardStats
	dateExpr := database.DateOnly(r.cfg, "created_at")
	where := "1=1"
	extra, args := r.dateRangeFilter(dateExpr, dr, 1)
	where += extra

	query := "SELECT " +
		"COUNT(*) as total_rides, " +
		"COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0) as completed_rides, " +
		"COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) as cancelled_rides, " +
		"COALESCE(SUM(CASE WHEN status = 'completed' THEN driver_earning ELSE 0 END), 0) as total_driver_earnings, " +
		"COALESCE(SUM(CASE WHEN status = 'completed' THEN platform_fee ELSE 0 END), 0) as total_platform_fee " +
		"FROM rides WHERE " + where
	err := r.db.QueryOne(ctx, &stats, query, args...)
	return stats, err
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

// ListRecentLog returns the most recent rides regardless of status, with
// matching state joined in, for the admin panel's live debugging view. A
// "searching" ride's decline_count is how many drivers already passed on it
// — a ride stuck at "buscando" with a nonzero count means it was seen and
// turned down, not that nobody's looking; a zero count means truly nobody
// eligible is online yet.
func (r *RideRepository) ListRecentLog(ctx context.Context, limit int) ([]models.RideLogRow, error) {
	rows := []models.RideLogRow{}
	query := fmt.Sprintf(
		"SELECT r.id as id, r.created_at as created_at, r.status as status, r.category_id as category_id, "+
			"u.name as customer_name, d.name as driver_name, "+
			"(SELECT COUNT(*) FROM ride_declines rd WHERE rd.ride_id = r.id) as decline_count, "+
			"r.distance_km as distance_km, r.price as price "+
			"FROM rides r "+
			"JOIN users u ON u.id = r.user_id "+
			"LEFT JOIN drivers d ON d.id = r.driver_id "+
			"ORDER BY r.created_at DESC LIMIT %s",
		database.Placeholder(r.cfg, 1),
	)
	err := r.db.Query(ctx, &rows, query, limit)
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
