package admin

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
	"chamaelas-api/internal/repository"
)

func sumReport(rows []models.RideReportRow) (rideCount int, totalPrice, totalDriverEarning, totalPlatformFee float64) {
	for _, row := range rows {
		rideCount += row.RideCount
		totalPrice += row.TotalPrice
		totalDriverEarning += row.TotalDriverEarning
		totalPlatformFee += row.TotalPlatformFee
	}
	return
}

// dateRangeFromQuery reads the "from"/"to" ("YYYY-MM-DD") filter shared by
// every report page and its export link.
func dateRangeFromQuery(c echo.Context) repository.DateRange {
	return repository.DateRange{From: c.QueryParam("from"), To: c.QueryParam("to")}
}

// writeReportCSV streams a report as a ";"-delimited CSV with a UTF-8 BOM —
// the combination Excel's pt-BR build expects to open the file directly
// (comma is the decimal separator there, so comma-delimited would split
// currency values into extra columns) with accented labels rendering right.
func writeReportCSV(c echo.Context, filename, groupLabel string, rows []models.RideReportRow) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Response().WriteHeader(http.StatusOK)

	_, _ = c.Response().Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	w := csv.NewWriter(c.Response())
	w.Comma = ';'
	_ = w.Write([]string{groupLabel, "Corridas", "Ganho das motoristas (R$)", "Taxa da plataforma (R$)", "Total cobrado (R$)"})
	for _, row := range rows {
		_ = w.Write([]string{
			row.GroupKey,
			fmt.Sprintf("%d", row.RideCount),
			fmt.Sprintf("%.2f", row.TotalDriverEarning),
			fmt.Sprintf("%.2f", row.TotalPlatformFee),
			fmt.Sprintf("%.2f", row.TotalPrice),
		})
	}
	w.Flush()
	return w.Error()
}

// paymentMethodLabel and statusLabel give the detailed "por data" report
// the same Portuguese labels the rest of the panel uses.
func paymentMethodLabel(pm *string) string {
	if pm == nil || *pm == "" {
		return "Não informado"
	}
	return *pm
}

func statusLabel(status string) string {
	switch status {
	case "completed":
		return "Concluída"
	case "cancelled":
		return "Cancelada"
	case "searching":
		return "Buscando motorista"
	case "accepted":
		return "Aceita"
	case "arrived":
		return "Motorista chegou"
	case "in_progress":
		return "Em andamento"
	default:
		return status
	}
}

func sumDetailRows(rows []models.RideDetailRow) (rideCount int, totalPrice float64) {
	for _, row := range rows {
		rideCount++
		totalPrice += row.Price
	}
	return
}

func (m *Module) ReportByDate(c echo.Context) error {
	dr := dateRangeFromQuery(c)
	rows, err := m.rides.ListDetailedByDateRange(c.Request().Context(), dr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rideCount, totalPrice := sumDetailRows(rows)
	return render(c, "reports-date", "report_by_date.html", map[string]any{
		"Rows": rows, "RideCount": rideCount, "TotalPrice": totalPrice,
		"From": dr.From, "To": dr.To,
	})
}

func (m *Module) ExportReportByDate(c echo.Context) error {
	rows, err := m.rides.ListDetailedByDateRange(c.Request().Context(), dateRangeFromQuery(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="relatorio-por-data.csv"`)
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(c.Response())
	w.Comma = ';'
	_ = w.Write([]string{
		"Data", "Cliente", "Motorista", "Método de pagamento",
		"Valor inicial (R$)", "Valor final (R$)", "Valor pago (R$)", "Status",
	})
	for _, row := range rows {
		driverName := "—"
		if row.DriverName != nil {
			driverName = *row.DriverName
		}
		finalPrice := "—"
		if row.FinalPrice != nil {
			finalPrice = fmt.Sprintf("%.2f", *row.FinalPrice)
		}
		amountPaid := "—"
		if row.AmountPaid != nil {
			amountPaid = fmt.Sprintf("%.2f", *row.AmountPaid)
		}
		_ = w.Write([]string{
			row.CreatedAt.Format("02/01/2006 15:04"),
			row.CustomerName,
			driverName,
			paymentMethodLabel(row.PaymentMethod),
			fmt.Sprintf("%.2f", row.Price),
			finalPrice,
			amountPaid,
			statusLabel(row.Status),
		})
	}
	w.Flush()
	return w.Error()
}

func (m *Module) ReportByCategory(c echo.Context) error {
	dr := dateRangeFromQuery(c)
	rows, err := m.rides.ReportByCategory(c.Request().Context(), dr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rideCount, totalPrice, totalDriverEarning, totalPlatformFee := sumReport(rows)
	return render(c, "reports-category", "report_by_category.html", map[string]any{
		"Rows": rows, "RideCount": rideCount, "TotalPrice": totalPrice,
		"TotalDriverEarning": totalDriverEarning, "TotalPlatformFee": totalPlatformFee,
		"From": dr.From, "To": dr.To,
	})
}

func (m *Module) ExportReportByCategory(c echo.Context) error {
	rows, err := m.rides.ReportByCategory(c.Request().Context(), dateRangeFromQuery(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return writeReportCSV(c, "relatorio-por-categoria.csv", "Categoria", rows)
}

func (m *Module) ReportByPaymentMethod(c echo.Context) error {
	dr := dateRangeFromQuery(c)
	rows, err := m.rides.ReportByPaymentMethod(c.Request().Context(), dr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rideCount, totalPrice, totalDriverEarning, totalPlatformFee := sumReport(rows)
	return render(c, "reports-payment", "report_by_payment_method.html", map[string]any{
		"Rows": rows, "RideCount": rideCount, "TotalPrice": totalPrice,
		"TotalDriverEarning": totalDriverEarning, "TotalPlatformFee": totalPlatformFee,
		"From": dr.From, "To": dr.To,
	})
}

func (m *Module) ExportReportByPaymentMethod(c echo.Context) error {
	rows, err := m.rides.ReportByPaymentMethod(c.Request().Context(), dateRangeFromQuery(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return writeReportCSV(c, "relatorio-por-pagamento.csv", "Meio de pagamento", rows)
}
