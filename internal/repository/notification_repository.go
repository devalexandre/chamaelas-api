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

var notificationsTable = ksql.NewTable("notifications", "id")

type NotificationRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewNotificationRepository(db ksql.DB, cfg config.Config) *NotificationRepository {
	return &NotificationRepository{db: db, cfg: cfg}
}

// SendToMany creates one notification row per recipient — the admin panel's
// "send to these drivers/passengers" broadcast is just this called once per
// audience with its own recipientType and id list.
func (r *NotificationRepository) SendToMany(ctx context.Context, recipientType string, recipientIDs []string, title, message string) error {
	now := time.Now().UTC()
	for _, id := range recipientIDs {
		n := &models.Notification{
			ID:            uuid.NewString(),
			RecipientType: recipientType,
			RecipientID:   id,
			Title:         title,
			Message:       message,
			CreatedAt:     now,
		}
		if err := r.db.Insert(ctx, notificationsTable, n); err != nil {
			return err
		}
	}
	return nil
}

// ListForRecipient returns the most recent notifications for one driver or
// user, newest first — the app shows these and marks read ones as seen.
func (r *NotificationRepository) ListForRecipient(ctx context.Context, recipientType, recipientID string) ([]models.Notification, error) {
	notifications := []models.Notification{}
	query := fmt.Sprintf(
		"FROM notifications WHERE recipient_type = %s AND recipient_id = %s ORDER BY created_at DESC",
		database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2),
	)
	err := r.db.Query(ctx, &notifications, query, recipientType, recipientID)
	return notifications, err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id string) error {
	query := fmt.Sprintf("UPDATE notifications SET read_at = %s WHERE id = %s", database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2))
	_, err := r.db.Exec(ctx, query, time.Now().UTC(), id)
	return err
}

// ListRecentSent lists the most recent notifications across every
// recipient, for the admin panel's "recently sent" log — capped since this
// grows without bound otherwise.
func (r *NotificationRepository) ListRecentSent(ctx context.Context, limit int) ([]models.Notification, error) {
	notifications := []models.Notification{}
	query := fmt.Sprintf("FROM notifications ORDER BY created_at DESC LIMIT %s", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &notifications, query, limit)
	return notifications, err
}
