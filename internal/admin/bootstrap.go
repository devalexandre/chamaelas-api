package admin

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"chamaelas-api/internal/models"
)

// Bootstrap creates the first admin account from ADMIN_EMAIL/ADMIN_PASSWORD
// if the admins table is still empty — so there's always a way in without
// a manual SQL insert. Change the password from the panel once it exists.
func (m *Module) Bootstrap(ctx context.Context) error {
	count, err := m.admins.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(m.cfg.AdminBootstrapPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &models.Admin{
		ID:           uuid.NewString(),
		Name:         "Admin",
		Email:        m.cfg.AdminBootstrapEmail,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	if err := m.admins.Create(ctx, admin); err != nil {
		return err
	}

	log.Printf("admin: bootstrap account created — email=%s password=%s (change this)", m.cfg.AdminBootstrapEmail, m.cfg.AdminBootstrapPassword)
	return nil
}
