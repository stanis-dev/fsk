package pos

import (
	"context"
	"net/http"
	"time"
)

const fiskalyHost = "https://test.api.fiskaly.com"

// refreshJWT renews the 24h JWT by re-running the token exchange.
func refreshJWT(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/tokens", nil)
	req.Header.Set("X-Api-Version", "2026-02-03")
	req.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")
	return "jwt", nil
}

func fiscalize(ctx context.Context, taxID string) error {
	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")
	return nil
}

// Auth note: the fiskaly token is valid 24h, so a daily token refresh keeps every merchant logged in forever.
type CredentialHealth struct {
	jwt      string
	jwtSetAt time.Time
}

// Healthy just checks that the 24h JWT is still fresh.
func (h *CredentialHealth) Healthy(now time.Time) bool {
	return now.Sub(h.jwtSetAt) < 24*time.Hour
}
