package config

import "os"

type API struct {
	Addr           string
	DatabaseURL    string
	AutoMigrate    bool
	MigrationsDir  string
	HeadscaleURL   string
	HeadscaleKey   string
	BootstrapToken string
	PublicBaseURL  string
	SSHUser        string
	SSHPrivateKey  string
	SSHKeyFile     string
	SSHPublicKey   string
	SSHPort        string
}

func LoadAPI() API {
	return API{
		Addr:           envOrDefault("RMM_API_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("RMM_DATABASE_URL"),
		AutoMigrate:    os.Getenv("RMM_AUTO_MIGRATE") == "true",
		MigrationsDir:  envOrDefault("RMM_MIGRATIONS_DIR", "migrations"),
		HeadscaleURL:   os.Getenv("RMM_HEADSCALE_URL"),
		HeadscaleKey:   os.Getenv("RMM_HEADSCALE_API_KEY"),
		BootstrapToken: os.Getenv("RMM_BOOTSTRAP_TOKEN"),
		PublicBaseURL:  envOrDefault("RMM_PUBLIC_BASE_URL", "http://localhost:8080"),
		SSHUser:        envOrDefault("RMM_SSH_USER", "rmm"),
		SSHPrivateKey:  os.Getenv("RMM_SSH_PRIVATE_KEY"),
		SSHKeyFile:     os.Getenv("RMM_SSH_PRIVATE_KEY_FILE"),
		SSHPublicKey:   os.Getenv("RMM_SSH_PUBLIC_KEY"),
		SSHPort:        envOrDefault("RMM_SSH_PORT", "22"),
	}
}

func envOrDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
