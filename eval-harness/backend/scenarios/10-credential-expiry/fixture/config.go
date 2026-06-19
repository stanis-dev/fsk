package pos

import "os"

// Config is the service configuration, loaded from the environment at startup.
type Config struct {
	StoreName   string
	Currency    string
	Environment string
}

// LoadConfig reads configuration from the environment.
func LoadConfig() Config {
	return Config{
		StoreName:   getenv("STORE_NAME", "POS"),
		Currency:    getenv("CURRENCY", "EUR"),
		Environment: getenv("ENVIRONMENT", "development"),
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
