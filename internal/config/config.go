package config

import "os"

// Config holds every environment-driven setting for the API. DBDriver is the
// single switch that moves the whole app between SQLite (local dev) and
// Postgres (QA/production) — every other package reads this instead of
// checking driver names itself.
type Config struct {
	Port string

	// DBDriver is either "sqlite" or "postgres".
	DBDriver string

	// DBDSN is a plain file path when DBDriver is "sqlite" (e.g. "./data/chamaelas.db")
	// or a full connection URL when DBDriver is "postgres"
	// (e.g. "postgres://user:pass@localhost:5432/chamaelas?sslmode=disable").
	DBDSN string

	// AdminSessionSecret signs the admin panel's session cookie. Use a long
	// random value in any environment reachable outside your own machine.
	AdminSessionSecret string

	// Bootstrap admin account, created on startup if the admins table is empty.
	AdminBootstrapEmail    string
	AdminBootstrapPassword string
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8080"),
		DBDriver:               getEnv("DB_DRIVER", "sqlite"),
		DBDSN:                  getEnv("DB_DSN", "./data/chamaelas.db"),
		AdminSessionSecret:     getEnv("ADMIN_SESSION_SECRET", "dev-only-insecure-secret-change-me"),
		AdminBootstrapEmail:    getEnv("ADMIN_EMAIL", "admin@chamaelas.com"),
		AdminBootstrapPassword: getEnv("ADMIN_PASSWORD", "chamaelas123"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
