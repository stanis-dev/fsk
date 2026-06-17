package pos

import "os"

// Config is the service configuration, loaded from the environment at startup.
type Config struct {
	StoreName   string // STORE_NAME: human-readable name of the till's store
	Currency    string // CURRENCY: ISO 4217 code; amounts are in euro cents
	Environment string // ENVIRONMENT: deployment environment, e.g. development
}

// LoadConfig reads configuration from the environment, applying defaults for
// any variable that is unset or empty.
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
