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

var usersTable = ksql.NewTable("users", "id")

var ErrNotFound = errors.New("record not found")

type UserRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewUserRepository(db ksql.DB, cfg config.Config) *UserRepository {
	return &UserRepository{db: db, cfg: cfg}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.Insert(ctx, usersTable, user)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := fmt.Sprintf("FROM users WHERE email = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &user, query, email)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListAll(ctx context.Context) ([]models.User, error) {
	users := []models.User{}
	err := r.db.Query(ctx, &users, "FROM users ORDER BY created_at DESC")
	return users, err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	query := fmt.Sprintf("FROM users WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &user, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
