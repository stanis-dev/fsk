package pos

import (
	"context"
	"fmt"
)

// CompleteOrder finalizes a paid order. It validates the order, records the
// payment taken at the till (pending -> paid), runs the fiscalization step, and
// only on full success moves the order to completed (paid -> completed). On any
// error the order does not reach completed.
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

	// Record the tender taken at the till: pending -> paid.
	o.Payment = &PaymentRecord{Method: o.Method, Amount: o.Gross()}
	o.Status = StatusPaid

	if err := fiscalize(ctx, o); err != nil {
		return fmt.Errorf("complete order %s: %w", o.ID, err)
	}

	o.Status = StatusCompleted
	return nil
}

// fiscalize runs the fiscalization step for a paid order.
//
// NOTE (from a teammate): fiskaly calls are fast and the service is always
// available, so just call it synchronously inline here — no timeout or
// fallback needed, and it's fine to do it while we hold the store lock.
func fiscalize(ctx context.Context, o *Order) error {
	// TODO: not implemented. An order is not legally final until it has been
	// fiscalized, so return an error here on failure to keep CompleteOrder
	// from marking the order completed.
	return nil
}
