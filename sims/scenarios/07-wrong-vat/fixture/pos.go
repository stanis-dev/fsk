// Package pos is the checkout backend for an Italian point-of-sale system.
//
// It finalizes paid business-to-consumer sales: it models line items with
// Italian VAT, validates an order, records the payment taken at the till, and
// moves the order through its lifecycle to completed. Amounts are integer euro
// cents and prices are net (VAT-exclusive); VAT is computed per line and summed
// for the order total.
//
// Fiscalization is legally required before an order may be completed and is not
// implemented yet; CompleteOrder calls the fiscalize function in checkout.go,
// which is currently a no-op.
package pos

// Cents is a monetary amount in integer euro cents (1/100 of a euro).
type Cents int64

// VATRate is an Italian value-added-tax rate expressed as a whole percentage.
type VATRate int

// Recognized Italian VAT rates. Ordinary B2C baskets mix the 10% and 22% rates;
// the reduced 4% and 5% rates are accepted as well.
const (
	VAT4  VATRate = 4
	VAT5  VATRate = 5
	VAT10 VATRate = 10
	VAT22 VATRate = 22
)

func (r VATRate) valid() bool {
	switch r {
	case VAT4, VAT5, VAT10, VAT22:
		return true
	default:
		return false
	}
}

// PaymentMethod is how the customer paid at the till.
type PaymentMethod string

const (
	Cash PaymentMethod = "cash"
	Card PaymentMethod = "card"
)

func (m PaymentMethod) valid() bool {
	switch m {
	case Cash, Card:
		return true
	default:
		return false
	}
}

// Status is where an order sits in its lifecycle.
type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusCompleted Status = "completed"
)
