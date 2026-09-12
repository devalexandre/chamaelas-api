package main

import (
	"context"
	"embed"
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

func main() {
	_ = godotenv.Load() // optional: only present in local dev

	cfg := config.Load()
	ctx := context.Background()

	if err := database.RunMigrations(cfg, migrationsFS); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to database (driver=%s): %v", cfg.DBDriver, err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db, cfg)
	driverRepo := repository.NewDriverRepository(db, cfg)
	rideRepo := repository.NewRideRepository(db, cfg)
	categoryRepo := repository.NewCategoryRepository(db, cfg)
	cityRepo := repository.NewCityRepository(db, cfg)
	adminRepo := repository.NewAdminRepository(db, cfg)
	billingRepo := repository.NewBillingRepository(db, cfg)
	pricingRepo := repository.NewPricingRepository(db, cfg)
	paymentSettingsRepo := repository.NewPaymentSettingsRepository(db, cfg)
	gatewayFeeRateRepo := repository.NewGatewayFeeRateRepository(db, cfg)
	notificationRepo := repository.NewNotificationRepository(db, cfg)

	authHandler := handlers.NewAuthHandler(userRepo)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	rideHandler := handlers.NewRideHandler(rideRepo, driverRepo, billingRepo, cityRepo, categoryRepo)
	driverHandler := handlers.NewDriverHandler(driverRepo, categoryRepo, rideHandler, billingRepo)
	notificationHandler := handlers.NewNotificationHandler(notificationRepo)
	settingsHandler := handlers.NewSettingsHandler(billingRepo)

	adminModule := admin.NewModule(cfg, adminRepo, userRepo, driverRepo, rideRepo, categoryRepo, cityRepo, billingRepo, pricingRepo, paymentSettingsRepo, gatewayFeeRateRepo, notificationRepo)
	if err := adminModule.Bootstrap(ctx); err != nil {
		log.Fatalf("failed to bootstrap admin account: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok", "dbDriver": cfg.DBDriver})
	})

	adminModule.RegisterRoutes(e)

	api := e.Group("/api")

	api.POST("/auth/signup", authHandler.Signup)
	api.POST("/auth/login", authHandler.Login)

	api.GET("/categories", categoryHandler.ListActive)
	api.GET("/settings", settingsHandler.PublicSettings)

	api.POST("/driver/auth/signup", driverHandler.Signup)
	api.POST("/driver/auth/login", driverHandler.Login)
	api.GET("/drivers/nearby", driverHandler.Nearby)
	api.GET("/driver/:id", driverHandler.GetProfile)
	api.POST("/driver/:id/location", driverHandler.SetLocation)
	api.POST("/driver/:id/categories", driverHandler.AddCategory)
	api.DELETE("/driver/:id/categories/:categoryId", driverHandler.RemoveCategory)
	api.POST("/driver/:id/billing-mode", driverHandler.SetBillingMode)
	api.GET("/driver/:id/credit-transactions", driverHandler.ListCreditTransactions)
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

	log.Printf("chamaelas-api listening on :%s (db driver: %s)", cfg.Port, cfg.DBDriver)
	log.Printf("admin panel at http://localhost:%s/admin", cfg.Port)
	if err := e.Start(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
