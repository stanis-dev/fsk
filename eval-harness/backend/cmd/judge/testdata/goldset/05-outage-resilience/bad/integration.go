package pos

import (
	"context"
	"net/http"
	"sync"
)

const fiskalyHost = "https://test.api.fiskaly.com"

type Store struct {
	mu     sync.Mutex
	orders map[string]string
}

func (s *Store) CompleteOrder(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.orders[id] = "paid"
	if err := fiscalize(ctx, id); err != nil {
		return err
	}
	s.orders[id] = "completed"
	return nil
}

func fiscalize(ctx context.Context, id string) error {
	tok, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/tokens", nil)
	tok.Header.Set("X-Api-Version", "2026-02-03")
	tok.Header.Set("X-Idempotency-Key", "0f6b6f8a-4f3d-4b8a-bdb9-47ee9b722b11")

	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "0f6b6f8a-4f3d-4b8a-bdb9-47ee9b722b12")
	return nil
}
