package pos

import (
	"context"
	"fmt"
)

// CompleteOrder completes a valid pending order only after fiscalization succeeds.
func (s *Store) CompleteOrder(ctx context.Context, o *Order) error {
	if o == nil {
		return ErrNilOrder
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := o.Validate(); err != nil {
		return fmt.Errorf("complete order: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if stored, ok := s.orders[o.ID]; !ok || stored != o {
		return fmt.Errorf("complete order %s: %w", o.ID, ErrUnknownOrder)
	}
	if o.Status != StatusPending {
		return fmt.Errorf("complete order %s: %w", o.ID, ErrNotPending)
	}

	o.Payment = &PaymentRecord{Method: o.Method, Amount: o.Gross()}
	o.Status = StatusPaid

	if err := fiscalize(ctx, o); err != nil {
		return fmt.Errorf("complete order %s: %w", o.ID, err)
	}

	o.Status = StatusCompleted
	return nil
}

func fiscalize(ctx context.Context, o *Order) error {
	// TODO: implement fiscalization; completion must fail if this fails.
	return nil
}
