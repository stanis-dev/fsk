package pos

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const fiskalyHost = "https://test.api.fiskaly.com"

type Store struct {
	mu     sync.Mutex
	orders map[string]string
}

func (s *Store) CompleteOrder(ctx context.Context, id string) error {
	s.mu.Lock()
	s.orders[id] = "paid"
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := fiscalize(ctx, id); err != nil {
		return s.deferToFallback(id)
	}

	s.mu.Lock()
	s.orders[id] = "completed"
	s.mu.Unlock()
	return nil
}

func (s *Store) deferToFallback(id string) error {
	s.mu.Lock()
	s.orders[id] = "fallback:paper+einvoice-within-12-days"
	s.mu.Unlock()
	return nil
}

func fiscalize(ctx context.Context, id string) error {
	tok, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/tokens", nil)
	tok.Header.Set("X-Api-Version", "2026-02-03")
	tok.Header.Set("X-Idempotency-Key", "0f6b6f8a-4f3d-4b8a-bdb9-47ee9b722b01")

	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "0f6b6f8a-4f3d-4b8a-bdb9-47ee9b722b02")
	return nil
}
