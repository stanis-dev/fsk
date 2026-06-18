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
	// The lock guards in-memory state only; release it before any network IO.
	s.mu.Lock()
	s.orders[id] = "paid"
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := fiscalize(ctx, id); err != nil {
		// Outage: never freeze the till. Route to the legal fallback —
		// paper document at the till + electronic invoice within 12 days.
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
	tok.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")

	rec, _ := http.NewRequestWithContext(ctx, "POST", fiskalyHost+"/records", nil)
	rec.Header.Set("X-Api-Version", "2026-02-03")
	rec.Header.Set("X-Idempotency-Key", "5f9b...uuid-v4")
	return nil
}
