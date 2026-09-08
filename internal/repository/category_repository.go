package repository

import (
	"context"
	"fmt"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var categoriesTable = ksql.NewTable("categories", "id")

type CategoryRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewCategoryRepository(db ksql.DB, cfg config.Config) *CategoryRepository {
	return &CategoryRepository{db: db, cfg: cfg}
}

func (r *CategoryRepository) ListActive(ctx context.Context) ([]models.Category, error) {
	categories := []models.Category{}
	query := fmt.Sprintf("FROM categories WHERE active = %s ORDER BY name", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &categories, query, true)
	return categories, err
}

func (r *CategoryRepository) ListAll(ctx context.Context) ([]models.Category, error) {
	categories := []models.Category{}
	err := r.db.Query(ctx, &categories, "FROM categories ORDER BY name")
	return categories, err
}

func (r *CategoryRepository) Create(ctx context.Context, category *models.Category) error {
	return r.db.Insert(ctx, categoriesTable, category)
}

func (r *CategoryRepository) SetActive(ctx context.Context, id string, active bool) error {
	query := fmt.Sprintf("UPDATE categories SET active = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, active, id)
	return err
}

func (r *CategoryRepository) ListByDriver(ctx context.Context, driverID string) ([]models.Category, error) {
	categories := []models.Category{}
	query := fmt.Sprintf(
		"SELECT c.id, c.name, c.description, c.active FROM categories c "+
			"JOIN driver_categories dc ON dc.category_id = c.id WHERE dc.driver_id = %s ORDER BY c.name",
		database.Placeholder(r.cfg, 1),
	)
	err := r.db.Query(ctx, &categories, query, driverID)
	return categories, err
}

func (r *CategoryRepository) AddToDriver(ctx context.Context, driverID, categoryID string) error {
	query := fmt.Sprintf(
		"INSERT INTO driver_categories (driver_id, category_id) VALUES (%s, %s)",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	_, err := r.db.Exec(ctx, query, driverID, categoryID)
	return err
}

func (r *CategoryRepository) RemoveFromDriver(ctx context.Context, driverID, categoryID string) error {
	query := fmt.Sprintf(
		"DELETE FROM driver_categories WHERE driver_id = %s AND category_id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	_, err := r.db.Exec(ctx, query, driverID, categoryID)
	return err
}
