package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/woovi"
)

// pixKeyRequestRow adds a human-readable owner name to a request row for
// display — never stored, just resolved for this page.
type pixKeyRequestRow struct {
	models.PixKeyChangeRequest
	OwnerName string
}

func (m *Module) ownerName(ctx context.Context, ownerType, ownerID string) string {
	if ownerType == models.PixKeyOwnerDriver {
		if d, err := m.drivers.FindByID(ctx, ownerID); err == nil {
			return d.Name
		}
		return "—"
	}
	if u, err := m.users.FindByID(ctx, ownerID); err == nil {
		return u.Name
	}
	return "—"
}

// PixKeyRequestsPage lists every pending Pix key change request — the
// second, independent check (besides the driver/passenger's own password)
// before a payout destination can change, so this list is where an admin
// confirms it's really the account holder's own key before approving.
func (m *Module) PixKeyRequestsPage(c echo.Context) error {
	ctx := c.Request().Context()
	requests, err := m.pixKeyChanges.ListPending(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rows := make([]pixKeyRequestRow, 0, len(requests))
	for _, req := range requests {
		rows = append(rows, pixKeyRequestRow{PixKeyChangeRequest: req, OwnerName: m.ownerName(ctx, req.OwnerType, req.OwnerID)})
	}

	return render(c, "pix-key-requests", "pix_key_requests.html", map[string]any{
		"Requests": rows,
	})
}

// ApprovePixKeyRequest actually applies the requested Pix key: registers
// (or confirms) the Woovi subaccount for the NEW key, persists its
// canonical form to the driver's/passenger's own pix_key column, and logs
// the change in the audit trail.
func (m *Module) ApprovePixKeyRequest(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	req, err := m.pixKeyChanges.FindByID(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "solicitação não encontrada")
	}
	if req.Status != string(models.PixKeyChangePending) {
		return c.Redirect(http.StatusSeeOther, "/admin/pix-key-requests")
	}

	settings, err := m.woovi.Get(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if settings.AppID == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gateway de pagamento não configurado")
	}

	ownerName := m.ownerName(ctx, req.OwnerType, req.OwnerID)
	canonical, err := woovi.EnsureRecipient(settings.AppID, woovi.BaseURL(settings.Environment), req.NewPixKey, ownerName)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	if req.OwnerType == models.PixKeyOwnerDriver {
		if err := m.billing.SetDriverPixKey(ctx, req.OwnerID, canonical); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	} else {
		if err := m.userCredit.SetPixKey(ctx, req.OwnerID, canonical); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	admin := m.currentAdmin(c)
	reviewedBy := ""
	if admin != nil {
		reviewedBy = admin.ID
	}
	if err := m.pixKeyChanges.SetStatus(ctx, id, string(models.PixKeyChangeApproved), reviewedBy, ""); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	m.recordAudit(c, "approve_pix_key_change", req.OwnerType, req.OwnerID,
		fmt.Sprintf("%s: de %q para %q (canônica: %q)", ownerName, req.OldPixKey, req.NewPixKey, canonical))

	return c.Redirect(http.StatusSeeOther, "/admin/pix-key-requests")
}

// RejectPixKeyRequest declines a Pix key change request — the account's
// actual pix_key never changes.
func (m *Module) RejectPixKeyRequest(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	req, err := m.pixKeyChanges.FindByID(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "solicitação não encontrada")
	}

	admin := m.currentAdmin(c)
	reviewedBy := ""
	if admin != nil {
		reviewedBy = admin.ID
	}
	note := c.FormValue("note")
	if err := m.pixKeyChanges.SetStatus(ctx, id, string(models.PixKeyChangeRejected), reviewedBy, note); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	m.recordAudit(c, "reject_pix_key_change", req.OwnerType, req.OwnerID, note)

	return c.Redirect(http.StatusSeeOther, "/admin/pix-key-requests")
}
