package pos

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNilOrder     = errors.New("order is nil")
	ErrUnknownOrder = errors.New("order is not in the store")
	ErrNotPending   = errors.New("order is not pending")
)

// Store is the in-memory system of record for orders; there is no database.
// Its methods are safe for concurrent use.
type Store struct {
	mu     sync.Mutex
	seq    int
	orders map[string]*Order
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{orders: make(map[string]*Order)}
}

// Create validates an order, assigns it an ID, marks it pending, and stores it.
func (s *Store) Create(o *Order) (*Order, error) {
	if o == nil {
		return nil, ErrNilOrder
	}
	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	o.ID = fmt.Sprintf("ord-%04d", s.seq)
	o.Status = StatusPending
	o.Payment = nil
	s.orders[o.ID] = o
	return o, nil
}

// Get returns the stored order with the given ID, if any.
func (s *Store) Get(id string) (*Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	return o, ok
}
