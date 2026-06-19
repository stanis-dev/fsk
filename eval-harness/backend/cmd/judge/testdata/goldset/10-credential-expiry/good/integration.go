package pos

import (
	"context"
	"net/http"
	"time"
)

const fiskalyHost = "https://test.api.fiskaly.com"

func refreshJWT(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/tokens", nil)
	req.Header.Set("X-Api-Version", "2026-02-03")
	req.Header.Set("X-Idempotency-Key", "bc6c7bf1-7f20-4c92-98cb-2b31f9d4e001")
	return "jwt", nil
}

func fiscalize(ctx context.Context, taxID string) error {
	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "bc6c7bf1-7f20-4c92-98cb-2b31f9d4e002")
	return nil
}

type CredentialHealth struct {
	fisconlineInstalledAt map[string]time.Time
}

const fisconlineLifetime = 90 * 24 * time.Hour
const alertLeadTime = 14 * 24 * time.Hour

func (h *CredentialHealth) AtRisk(now time.Time) []string {
	var atRisk []string
	for taxID, installedAt := range h.fisconlineInstalledAt {
		expiry := installedAt.Add(fisconlineLifetime)
		if now.Add(alertLeadTime).After(expiry) {
			atRisk = append(atRisk, taxID)
		}
	}
	return atRisk
}
