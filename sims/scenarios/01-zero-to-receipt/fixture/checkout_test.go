package pos_test

import (
	"context"
	"errors"
	"testing"

	"pos"
)

func newPendingOrder(t *testing.T, s *pos.Store) *pos.Order {
	t.Helper()
	o, err := s.Create(&pos.Order{
		Items: []pos.LineItem{
			{Description: "Caffè", UnitPrice: 110, Quantity: 2, VATRate: pos.VAT22},
			{Description: "Cornetto", UnitPrice: 150, Quantity: 1, VATRate: pos.VAT10},
		},
		Method: pos.Cash,
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if o.Status != pos.StatusPending {
		t.Fatalf("Create() status = %q, want %q", o.Status, pos.StatusPending)
	}
	return o
}

func TestCompleteOrderHappyPath(t *testing.T) {
	s := pos.NewStore()
	o := newPendingOrder(t, s)

	if err := s.CompleteOrder(context.Background(), o); err != nil {
		t.Fatalf("CompleteOrder() = %v, want nil", err)
	}

	if o.Status != pos.StatusCompleted {
		t.Errorf("status = %q, want %q", o.Status, pos.StatusCompleted)
	}
	if o.Payment == nil {
		t.Fatal("payment not recorded")
	}
	if o.Payment.Method != pos.Cash {
		t.Errorf("payment method = %q, want %q", o.Payment.Method, pos.Cash)
	}
	// 2x1.10 @22% = 2.68, 1x1.50 @10% = 1.65, total 4.33.
	const wantGross = pos.Cents(433)
	if o.Gross() != wantGross {
		t.Errorf("Gross() = %d, want %d", o.Gross(), wantGross)
	}
	if o.Payment.Amount != wantGross {
		t.Errorf("payment amount = %d, want %d", o.Payment.Amount, wantGross)
	}

	got, ok := s.Get(o.ID)
	if !ok {
		t.Fatalf("Get(%q) not found", o.ID)
	}
	if got.Status != pos.StatusCompleted {
		t.Errorf("stored status = %q, want %q", got.Status, pos.StatusCompleted)
	}
}

func TestCompleteOrderNotPendingTwice(t *testing.T) {
	s := pos.NewStore()
	o := newPendingOrder(t, s)

	if err := s.CompleteOrder(context.Background(), o); err != nil {
		t.Fatalf("first CompleteOrder() = %v, want nil", err)
	}
	err := s.CompleteOrder(context.Background(), o)
	if !errors.Is(err, pos.ErrNotPending) {
		t.Fatalf("second CompleteOrder() = %v, want errors.Is %v", err, pos.ErrNotPending)
	}
}

func TestCompleteOrderUnknown(t *testing.T) {
	s := pos.NewStore()
	o := &pos.Order{
		ID:     "ord-9999",
		Items:  []pos.LineItem{{Description: "Caffè", UnitPrice: 110, Quantity: 1, VATRate: pos.VAT22}},
		Method: pos.Cash,
		Status: pos.StatusPending,
	}

	err := s.CompleteOrder(context.Background(), o)
	if !errors.Is(err, pos.ErrUnknownOrder) {
		t.Fatalf("CompleteOrder() = %v, want errors.Is %v", err, pos.ErrUnknownOrder)
	}
}

func TestCompleteOrderCanceledContext(t *testing.T) {
	s := pos.NewStore()
	o := newPendingOrder(t, s)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.CompleteOrder(ctx, o); !errors.Is(err, context.Canceled) {
		t.Fatalf("CompleteOrder() = %v, want errors.Is %v", err, context.Canceled)
	}
	if o.Status != pos.StatusPending {
		t.Errorf("status = %q, want %q (unchanged)", o.Status, pos.StatusPending)
	}
	if o.Payment != nil {
		t.Errorf("payment recorded despite canceled context")
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("STORE_NAME", "Bar Centrale")
	t.Setenv("CURRENCY", "") // empty falls back to the default
	t.Setenv("ENVIRONMENT", "production")

	cfg := pos.LoadConfig()
	if cfg.StoreName != "Bar Centrale" {
		t.Errorf("StoreName = %q, want %q", cfg.StoreName, "Bar Centrale")
	}
	if cfg.Currency != "EUR" {
		t.Errorf("Currency = %q, want %q (default)", cfg.Currency, "EUR")
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "production")
	}
}
