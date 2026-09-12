// Package admin implements the operator-facing panel: driver approval,
// user/driver listings, city and category configuration, and financial
// settings. It's server-rendered Go templates on top of the same
// chamaelas-api process — no separate frontend project, no build step.
package admin

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/repository"
)

//go:embed templates/*.html
var templateFS embed.FS

// templateFuncs are helpers the templates can't express with plain
// html/template syntax — right now just cents-to-reais division for
// amounts coming back from the Pagar.me API.
var templateFuncs = template.FuncMap{
	"div64": func(cents int64, divisor int) float64 {
		return float64(cents) / float64(divisor)
	},
	"mulf64": func(a, b float64) float64 {
		return a * b
	},
	"derefF64": func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	},
}

var pageTemplates = template.Must(template.New("admin").Funcs(templateFuncs).ParseFS(templateFS, "templates/*.html"))

const sessionName = "chamaelas_admin"

var sessionsOptions = sessions.Options{
	Path:     "/",
	MaxAge:   60 * 60 * 8, // 8h
	HttpOnly: true,
	SameSite: http.SameSiteLaxMode,
}

type Module struct {
	cfg           config.Config
	admins        *repository.AdminRepository
	users         *repository.UserRepository
	drivers       *repository.DriverRepository
	rides         *repository.RideRepository
	categories    *repository.CategoryRepository
	cities        *repository.CityRepository
	billing       *repository.BillingRepository
	pricing       *repository.PricingRepository
	payments      *repository.PaymentSettingsRepository
	gatewayFees   *repository.GatewayFeeRateRepository
	notifications *repository.NotificationRepository
}

func NewModule(
	cfg config.Config,
	admins *repository.AdminRepository,
	users *repository.UserRepository,
	drivers *repository.DriverRepository,
	rides *repository.RideRepository,
	categories *repository.CategoryRepository,
	cities *repository.CityRepository,
	billing *repository.BillingRepository,
	pricing *repository.PricingRepository,
	payments *repository.PaymentSettingsRepository,
	gatewayFees *repository.GatewayFeeRateRepository,
	notifications *repository.NotificationRepository,
) *Module {
	return &Module{
		cfg: cfg, admins: admins, users: users, drivers: drivers,
		rides: rides, categories: categories, cities: cities, billing: billing,
		pricing: pricing, payments: payments, gatewayFees: gatewayFees,
		notifications: notifications,
	}
}

// RegisterRoutes wires every /admin/* route onto e, including the session
// middleware — call this once from main().
func (m *Module) RegisterRoutes(e *echo.Echo) {
	store := sessions.NewCookieStore([]byte(m.cfg.AdminSessionSecret))
	e.Use(session.Middleware(store))

	e.GET("/admin/login", m.LoginPage)
	e.POST("/admin/login", m.Login)
	e.POST("/admin/logout", m.Logout)

	g := e.Group("/admin", m.requireAdmin)
	g.GET("", m.Dashboard)
	g.GET("/drivers", m.ListDrivers)
	g.GET("/drivers/:id", m.ViewDriver)
	g.GET("/drivers/:id/edit", m.EditDriverPage)
	g.POST("/drivers/:id/edit", m.UpdateDriver)
	g.POST("/drivers/:id/password", m.ChangeDriverPassword)
	g.POST("/drivers/:id/approve", m.ApproveDriver)
	g.POST("/drivers/:id/block", m.BlockDriver)
	g.GET("/users", m.ListUsers)
	g.GET("/cities", m.ListCities)
	g.POST("/cities", m.CreateCity)
	g.POST("/cities/:id/toggle", m.ToggleCity)
	g.GET("/categories", m.ListCategories)
	g.POST("/categories", m.CreateCategory)
	g.POST("/categories/:id/toggle", m.ToggleCategory)
	g.GET("/billing", m.Billing)
	g.POST("/billing/commission", m.UpdateCommission)
	g.POST("/billing/drivers/:id/adjust", m.AdjustDriverCredit)
	g.POST("/billing/gateway", m.UpdateGatewaySettings)
	g.POST("/billing/gateway-fees", m.UpdateGatewayFeeRates)
	g.GET("/reports/by-date", m.ReportByDate)
	g.GET("/reports/by-date/export", m.ExportReportByDate)
	g.GET("/reports/by-category", m.ReportByCategory)
	g.GET("/reports/by-category/export", m.ExportReportByCategory)
	g.GET("/reports/by-payment-method", m.ReportByPaymentMethod)
	g.GET("/reports/by-payment-method/export", m.ExportReportByPaymentMethod)
	g.GET("/pricing", m.ListPricing)
	g.POST("/pricing", m.CreatePricingRule)
	g.GET("/pricing/:id/edit", m.EditPricingRulePage)
	g.POST("/pricing/:id/update", m.UpdatePricingRule)
	g.POST("/pricing/:id/clone", m.ClonePricingRule)
	g.POST("/pricing/:id/delete", m.DeletePricingRule)
	g.GET("/notifications", m.NotificationsPage)
	g.POST("/notifications", m.SendNotification)
}

// requireAdmin redirects anonymous visitors to the login page. It's the only
// gate — there's a single admin role for now, no permission levels.
func (m *Module) requireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get(sessionName, c)
		if err != nil || sess.Values["adminID"] == nil {
			return c.Redirect(http.StatusSeeOther, "/admin/login")
		}
		return next(c)
	}
}

// render executes a content template by filename, then wraps it in the
// shared layout (skipped for the standalone login page).
func render(c echo.Context, active, page string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}

	var body bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&body, page, data); err != nil {
		return err
	}

	if page == "login.html" {
		return c.HTML(http.StatusOK, body.String())
	}

	var out bytes.Buffer
	layoutData := map[string]any{"Body": template.HTML(body.String()), "Active": active}
	if err := pageTemplates.ExecuteTemplate(&out, "layout.html", layoutData); err != nil {
		return err
	}
	return c.HTML(http.StatusOK, out.String())
}
