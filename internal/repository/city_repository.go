package repository

import (
	"context"
	"fmt"

	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var citiesTable = ksql.NewTable("cities", "id")

type CityRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewCityRepository(db ksql.DB, cfg config.Config) *CityRepository {
	return &CityRepository{db: db, cfg: cfg}
}

func (r *CityRepository) ListAll(ctx context.Context) ([]models.City, error) {
	cities := []models.City{}
	err := r.db.Query(ctx, &cities, "FROM cities ORDER BY name")
	return cities, err
}

func (r *CityRepository) ListActive(ctx context.Context) ([]models.City, error) {
	cities := []models.City{}
	query := fmt.Sprintf("FROM cities WHERE active = %s ORDER BY name", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &cities, query, true)
	return cities, err
}

func (r *CityRepository) Create(ctx context.Context, city *models.City) error {
	return r.db.Insert(ctx, citiesTable, city)
}

func (r *CityRepository) SetActive(ctx context.Context, id string, active bool) error {
	query := fmt.Sprintf("UPDATE cities SET active = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, active, id)
	return err
}
