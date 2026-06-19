package pos

import (
	"context"
	"errors"
)

// ErrCancelNotImplemented is returned until the cancellation path is built.
var ErrCancelNotImplemented = errors.New("cancellation not implemented")

// CancelOrder voids a completed sale. A draft note claims fiskaly exposes a
// refunds endpoint where you POST the original receipt id. Finish this path.
func (s *Store) CancelOrder(ctx context.Context, o *Order) error {
	return ErrCancelNotImplemented
}
