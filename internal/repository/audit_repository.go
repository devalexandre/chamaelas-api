package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var auditLogTable = ksql.NewTable("admin_audit_log", "id")

type AuditRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewAuditRepository(db ksql.DB, cfg config.Config) *AuditRepository {
	return &AuditRepository{db: db, cfg: cfg}
}

// Record logs one admin action — every money-touching admin action should
// call this so there's a clear, auditable trail of who did what.
func (r *AuditRepository) Record(ctx context.Context, adminID, adminName, action, targetType, targetID, details string) error {
	entry := &models.AuditLogEntry{
		ID:         uuid.NewString(),
		AdminID:    adminID,
		AdminName:  adminName,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		CreatedAt:  time.Now().UTC(),
	}
	return r.db.Insert(ctx, auditLogTable, entry)
}

func (r *AuditRepository) ListForTarget(ctx context.Context, targetType, targetID string) ([]models.AuditLogEntry, error) {
	entries := []models.AuditLogEntry{}
	query := fmt.Sprintf(
		"FROM admin_audit_log WHERE target_type = %s AND target_id = %s ORDER BY created_at DESC",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.Query(ctx, &entries, query, targetType, targetID)
	return entries, err
}

func (r *AuditRepository) ListRecent(ctx context.Context, limit int) ([]models.AuditLogEntry, error) {
	entries := []models.AuditLogEntry{}
	query := fmt.Sprintf("FROM admin_audit_log ORDER BY created_at DESC LIMIT %s", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &entries, query, limit)
	return entries, err
}
