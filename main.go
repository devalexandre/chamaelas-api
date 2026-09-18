package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"chamaelas-api/internal/admin"
	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/handlers"
	"chamaelas-api/internal/repository"
)

// migrationsFS embeds every migration into the binary, so deploying just
// means shipping this one executable — no "migrations" directory needs to
// exist alongside it on the server.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// buildApp wires every repository/handler and route exactly once — used by
// main() to actually serve, and directly by tests (via httptest) so
// integration tests exercise the real routing/DI graph instead of a
// hand-rolled subset of it.
func buildApp(cfg config.Config) (*echo.Echo, error) {
	ctx := context.Background()

	if err := database.RunMigrations(cfg, migrationsFS); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database (driver=%s): %w", cfg.DBDriver, err)
	}

	userRepo := repository.NewUserRepository(db, cfg)
	userCreditRepo := repository.NewUserCreditRepository(db, cfg)
	driverRepo := repository.NewDriverRepository(db, cfg)
	rideRepo := repository.NewRideRepository(db, cfg)
	categoryRepo := repository.NewCategoryRepository(db, cfg)
	cityRepo := repository.NewCityRepository(db, cfg)
	adminRepo := repository.NewAdminRepository(db, cfg)
	billingRepo := repository.NewBillingRepository(db, cfg)
	pricingRepo := repository.NewPricingRepository(db, cfg)
	paymentSettingsRepo := repository.NewPaymentSettingsRepository(db, cfg)
	wooviSettingsRepo := repository.NewWooviSettingsRepository(db, cfg)
	gatewayFeeRateRepo := repository.NewGatewayFeeRateRepository(db, cfg)
	notificationRepo := repository.NewNotificationRepository(db, cfg)
	pixKeyChangeRepo := repository.NewPixKeyChangeRepository(db, cfg)
	auditRepo := repository.NewAuditRepository(db, cfg)

	authHandler := handlers.NewAuthHandler(userRepo, userCreditRepo, wooviSettingsRepo, pixKeyChangeRepo, cfg.GoogleClientID)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	rideHandler := handlers.NewRideHandler(rideRepo, driverRepo, userRepo, userCreditRepo, billingRepo, cityRepo, categoryRepo, wooviSettingsRepo)
	driverHandler := handlers.NewDriverHandler(driverRepo, categoryRepo, rideHandler, billingRepo, wooviSettingsRepo, pixKeyChangeRepo, cfg.GoogleClientID)
	notificationHandler := handlers.NewNotificationHandler(notificationRepo)
	settingsHandler := handlers.NewSettingsHandler(billingRepo)
	webhookHandler := handlers.NewWebhookHandler(wooviSettingsRepo, billingRepo, userCreditRepo)

	adminModule := admin.NewModule(cfg, adminRepo, userRepo, driverRepo, rideRepo, categoryRepo, cityRepo, billingRepo, userCreditRepo, pricingRepo, paymentSettingsRepo, wooviSettingsRepo, gatewayFeeRateRepo, notificationRepo, pixKeyChangeRepo, auditRepo)
	if err := adminModule.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap admin account: %w", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok", "dbDriver": cfg.DBDriver})
	})
	// Registered on the root router, not under /api — this exact path is
	// already configured as the endpoint in Woovi's dashboard.
	e.POST("/webhooks/payment", webhookHandler.HandlePaymentWebhook)

	adminModule.RegisterRoutes(e)

	api := e.Group("/api")

	api.POST("/auth/signup", authHandler.Signup)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/google", authHandler.GoogleLogin)
	api.GET("/users/:userId", authHandler.GetProfile)
	api.POST("/users/:userId/link-google", authHandler.LinkGoogleAccount)
	api.POST("/users/:userId/credit/topup", authHandler.CreateCreditTopup)
	api.GET("/users/:userId/credit-transactions", authHandler.ListCreditTransactions)
	api.POST("/users/:userId/pix-key", authHandler.SetPixKey)

	api.GET("/categories", categoryHandler.ListActive)
	api.GET("/settings", settingsHandler.PublicSettings)

	api.POST("/driver/auth/signup", driverHandler.Signup)
	api.POST("/driver/auth/login", driverHandler.Login)
	api.POST("/driver/auth/google", driverHandler.GoogleLogin)
	api.POST("/driver/:id/link-google", driverHandler.LinkGoogleAccount)
	api.GET("/drivers/nearby", driverHandler.Nearby)
	api.GET("/driver/:id", driverHandler.GetProfile)
	api.POST("/driver/:id/location", driverHandler.SetLocation)
	api.POST("/driver/:id/categories", driverHandler.AddCategory)
	api.DELETE("/driver/:id/categories/:categoryId", driverHandler.RemoveCategory)
	api.POST("/driver/:id/billing-mode", driverHandler.SetBillingMode)
	api.GET("/driver/:id/credit-transactions", driverHandler.ListCreditTransactions)
	api.POST("/driver/:id/pix-key", driverHandler.SetPixKey)
	api.POST("/driver/:id/credit/topup", driverHandler.CreateCreditTopup)
	api.GET("/driver/:id/wallet", driverHandler.GetWallet)
	api.POST("/driver/:id/wallet/withdraw", driverHandler.Withdraw)
	api.GET("/driver/:driverId/rides/current", rideHandler.CurrentForDriver)
	api.GET("/driver/:driverId/rides/offer", rideHandler.GetOffer)
	api.GET("/driver/:driverId/notifications", notificationHandler.ListForDriver)
	api.GET("/users/:userId/notifications", notificationHandler.ListForUser)
	api.POST("/notifications/:id/read", notificationHandler.MarkRead)

	api.POST("/rides", rideHandler.Create)
	api.GET("/rides", rideHandler.ListByUser)
	api.GET("/rides/:id", rideHandler.Get)
	api.POST("/rides/:id/cancel", rideHandler.Cancel)
	api.POST("/rides/:id/accept", rideHandler.Accept)
	api.POST("/rides/:id/decline", rideHandler.Decline)
	api.POST("/rides/:id/arrive", rideHandler.Arrive)
	api.POST("/rides/:id/start", rideHandler.Start)
	api.POST("/rides/:id/complete", rideHandler.Complete)
	api.POST("/rides/:id/rating", rideHandler.Rate)

	return e, nil
}

func main() {
	_ = godotenv.Load() // optional: only present in local dev

	cfg := config.Load()
	e, err := buildApp(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("chamaelas-api listening on :%s (db driver: %s)", cfg.Port, cfg.DBDriver)
	log.Printf("admin panel at http://localhost:%s/admin", cfg.Port)
	if err := e.Start(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
