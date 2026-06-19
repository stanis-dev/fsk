package pos

import (
	"context"
	"net/http"
)

type LineItem struct {
	Name    string
	Net     int64
	VAT     int64
	Gross   int64
	VATRate float64
}

type Order struct {
	Lines []LineItem
}

const fiskalyHost = "https://test.api.fiskaly.com"

func token(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/tokens", nil)
	req.Header.Set("X-Api-Version", "2026-02-03")
	req.Header.Set("X-Idempotency-Key", "1a4c97c0-8314-4d30-b4a8-934eaefb0701")
	return "jwt", nil
}

func fiscalize(ctx context.Context, o Order) error {
	jwt, err := token(ctx)
	if err != nil {
		return err
	}
	intent, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	intent.Header.Set("Authorization", "Bearer "+jwt)
	intent.Header.Set("X-Api-Version", "2026-02-03")
	intent.Header.Set("X-Idempotency-Key", "1a4c97c0-8314-4d30-b4a8-934eaefb0702")

	for _, line := range o.Lines {
		pct := line.VATRate
		breakdown := map[string]any{
			"percentage": pct,
			"amount":     line.VAT,
			"exclusive":  line.Net,
			"inclusive":  line.Gross,
		}
		_ = breakdown
	}

	txn, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	txn.Header.Set("Authorization", "Bearer "+jwt)
	txn.Header.Set("X-Api-Version", "2026-02-03")
	txn.Header.Set("X-Idempotency-Key", "1a4c97c0-8314-4d30-b4a8-934eaefb0703")
	return nil
}
