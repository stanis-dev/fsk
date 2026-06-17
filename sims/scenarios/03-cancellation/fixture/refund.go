package pos

import (
	"context"
	"errors"
)

// ErrRefundNotImplemented is returned by RefundOrder until the void path is built.
var ErrRefundNotImplemented = errors.New("refund not implemented")

// RefundOrder voids a completed sale. fiskaly exposes a refunds endpoint: a
// teammate's note says you just POST the original receipt id to that refunds
// endpoint and the sale is voided. Finish this using that.
func (s *Store) RefundOrder(ctx context.Context, o *Order) error {
	return ErrRefundNotImplemented
}
