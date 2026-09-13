package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var pixKeyChangeRequestsTable = ksql.NewTable("pix_key_change_requests", "id")

type PixKeyChangeRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewPixKeyChangeRepository(db ksql.DB, cfg config.Config) *PixKeyChangeRepository {
	return &PixKeyChangeRepository{db: db, cfg: cfg}
}

// Create records a pending Pix key change request — it never takes effect
// on its own; an admin must approve it (ApprovePixKeyRequest) before the
// owner's actual pix_key column changes.
func (r *PixKeyChangeRepository) Create(ctx context.Context, ownerType, ownerID, oldPixKey, newPixKey string) (*models.PixKeyChangeRequest, error) {
	req := &models.PixKeyChangeRequest{
		ID:          uuid.NewString(),
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		OldPixKey:   oldPixKey,
		NewPixKey:   newPixKey,
		Status:      string(models.PixKeyChangePending),
		RequestedAt: time.Now().UTC(),
	}
	if err := r.db.Insert(ctx, pixKeyChangeRequestsTable, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *PixKeyChangeRepository) FindByID(ctx context.Context, id string) (*models.PixKeyChangeRequest, error) {
	var req models.PixKeyChangeRequest
	query := fmt.Sprintf("FROM pix_key_change_requests WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &req, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *PixKeyChangeRepository) ListPending(ctx context.Context) ([]models.PixKeyChangeRequest, error) {
	requests := []models.PixKeyChangeRequest{}
	query := fmt.Sprintf("FROM pix_key_change_requests WHERE status = %s ORDER BY requested_at", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &requests, query, string(models.PixKeyChangePending))
	return requests, err
}

func (r *PixKeyChangeRepository) ListForOwner(ctx context.Context, ownerType, ownerID string) ([]models.PixKeyChangeRequest, error) {
	requests := []models.PixKeyChangeRequest{}
	query := fmt.Sprintf(
		"FROM pix_key_change_requests WHERE owner_type = %s AND owner_id = %s ORDER BY requested_at DESC",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.Query(ctx, &requests, query, ownerType, ownerID)
	return requests, err
}

func (r *PixKeyChangeRepository) SetStatus(ctx context.Context, id, status, reviewedBy, note string) error {
	query := fmt.Sprintf(
		"UPDATE pix_key_change_requests SET status = %s, reviewed_at = %s, reviewed_by = %s, note = %s WHERE id = %s",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
		database.Placeholder(r.cfg, 3), database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5),
	)
	_, err := r.db.Exec(ctx, query, status, time.Now().UTC(), reviewedBy, note, id)
	return err
}
