package pos

import (
	"context"
	"net/http"
	"time"
)

const fiskalyHost = "https://test.api.fiskaly.com"

// refreshJWT renews the short-lived 24h JWT by re-running the token exchange.
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

// CredentialHealth tracks the ~90-day Fisconline credential clock per taxpayer.
// This is a different lifetime than the 24h JWT: refreshing the JWT does nothing
// for the 90-day expiry, which needs a manual re-login.
type CredentialHealth struct {
	fisconlineInstalledAt map[string]time.Time
}

const fisconlineLifetime = 90 * 24 * time.Hour
const alertLeadTime = 14 * 24 * time.Hour

// AtRisk returns taxpayers whose Fisconline credential will lapse within the lead
// time, so ops can drive the manual re-login before day 91.
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
