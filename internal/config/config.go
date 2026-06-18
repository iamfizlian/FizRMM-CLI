package config

import "os"

type API struct {
	Addr          string
	DatabaseURL   string
	AutoMigrate   bool
	MigrationsDir string
	HeadscaleURL  string
	HeadscaleKey  string
}

func LoadAPI() API {
	return API{
		Addr:          envOrDefault("RMM_API_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("RMM_DATABASE_URL"),
		AutoMigrate:   os.Getenv("RMM_AUTO_MIGRATE") == "true",
		MigrationsDir: envOrDefault("RMM_MIGRATIONS_DIR", "migrations"),
		HeadscaleURL:  os.Getenv("RMM_HEADSCALE_URL"),
		HeadscaleKey:  os.Getenv("RMM_HEADSCALE_API_KEY"),
	}
}

func envOrDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
