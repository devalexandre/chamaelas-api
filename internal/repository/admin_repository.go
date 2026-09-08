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

var adminsTable = ksql.NewTable("admins", "id")

type AdminRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewAdminRepository(db ksql.DB, cfg config.Config) *AdminRepository {
	return &AdminRepository{db: db, cfg: cfg}
}

func (r *AdminRepository) Create(ctx context.Context, admin *models.Admin) error {
	return r.db.Insert(ctx, adminsTable, admin)
}

func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*models.Admin, error) {
	var admin models.Admin
	query := fmt.Sprintf("FROM admins WHERE email = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &admin, query, email)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *AdminRepository) FindByID(ctx context.Context, id string) (*models.Admin, error) {
	var admin models.Admin
	query := fmt.Sprintf("FROM admins WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &admin, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *AdminRepository) Count(ctx context.Context) (int, error) {
	var result []struct {
		Count int `ksql:"count"`
	}
	err := r.db.Query(ctx, &result, "SELECT COUNT(*) as count FROM admins")
	if err != nil || len(result) == 0 {
		return 0, err
	}
	return result[0].Count, nil
}
