package pos

import (
	"errors"
	"fmt"
)

// Validation errors returned by Order.Validate. Where an error concerns a
// specific line it is wrapped with that line's index; use errors.Is to test.
var (
	ErrNoItems          = errors.New("order has no line items")
	ErrEmptyDescription = errors.New("line item description is empty")
	ErrNonPositivePrice = errors.New("line item unit price must be positive")
	ErrNonPositiveQty   = errors.New("line item quantity must be positive")
	ErrUnknownVATRate   = errors.New("unsupported VAT rate")
	ErrUnknownPayment   = errors.New("unsupported payment method")
)

// LineItem is a single priced line on an order. UnitPrice is the net
// (VAT-exclusive) price of one unit, in euro cents.
type LineItem struct {
	Description string
	UnitPrice   Cents
	Quantity    int
	VATRate     VATRate
}

// Net is the VAT-exclusive amount for the line.
func (li LineItem) Net() Cents {
	return li.UnitPrice * Cents(li.Quantity)
}

// VAT is the value-added tax due on the line, rounded to the nearest cent.
func (li LineItem) VAT() Cents {
	// VAT is expressed in whole cents, so each line is rounded half-up before
	// the lines are summed (per-line rounding, not on the order total).
	net := int64(li.Net())
	return Cents((net*int64(li.VATRate) + 50) / 100)
}

// Gross is the VAT-inclusive amount for the line.
func (li LineItem) Gross() Cents {
	return li.Net() + li.VAT()
}

// PaymentRecord is evidence that the tender was taken at the till. It is
// recorded locally and is not sent to any payment processor.
type PaymentRecord struct {
	Method PaymentMethod
	Amount Cents
}

// Order is a B2C sale captured at the till.
type Order struct {
	ID      string
	Items   []LineItem
	Method  PaymentMethod
	Status  Status
	Payment *PaymentRecord
}

// Net is the VAT-exclusive total of the order.
func (o *Order) Net() Cents {
	var total Cents
	for _, li := range o.Items {
		total += li.Net()
	}
	return total
}

// VAT is the order's total VAT, summed from the per-line VAT.
func (o *Order) VAT() Cents {
	var total Cents
	for _, li := range o.Items {
		total += li.VAT()
	}
	return total
}

// Gross is the VAT-inclusive total of the order.
func (o *Order) Gross() Cents {
	return o.Net() + o.VAT()
}

// Validate reports whether the order is well formed for completion.
func (o *Order) Validate() error {
	if len(o.Items) == 0 {
		return ErrNoItems
	}
	for i, li := range o.Items {
		switch {
		case li.Description == "":
			return fmt.Errorf("line %d: %w", i, ErrEmptyDescription)
		case li.UnitPrice <= 0:
			return fmt.Errorf("line %d: %w", i, ErrNonPositivePrice)
		case li.Quantity <= 0:
			return fmt.Errorf("line %d: %w", i, ErrNonPositiveQty)
		case !li.VATRate.valid():
			return fmt.Errorf("line %d: %w", i, ErrUnknownVATRate)
		}
	}
	if !o.Method.valid() {
		return fmt.Errorf("%w: %q", ErrUnknownPayment, o.Method)
	}
	return nil
}
