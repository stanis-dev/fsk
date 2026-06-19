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
	// fiskaly is fast and always available, so call it inline under the store lock.
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
	tok.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")

	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")
	return nil
}
