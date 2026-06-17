package pos_test

import (
	"errors"
	"testing"

	"pos"
)

func TestLineItemVAT(t *testing.T) {
	tests := []struct {
		name      string
		item      pos.LineItem
		wantNet   pos.Cents
		wantVAT   pos.Cents
		wantGross pos.Cents
	}{
		{"22% standard", pos.LineItem{Description: "Caffè", UnitPrice: 100, Quantity: 3, VATRate: pos.VAT22}, 300, 66, 366},
		{"10% reduced", pos.LineItem{Description: "Acqua", UnitPrice: 50, Quantity: 2, VATRate: pos.VAT10}, 100, 10, 110},
		{"22% rounds half up", pos.LineItem{Description: "Item", UnitPrice: 9, Quantity: 1, VATRate: pos.VAT22}, 9, 2, 11}, // 1.98 -> 2
		{"10% half rounds up", pos.LineItem{Description: "Item", UnitPrice: 5, Quantity: 1, VATRate: pos.VAT10}, 5, 1, 6},  // 0.50 -> 1
		{"10% rounds down", pos.LineItem{Description: "Item", UnitPrice: 14, Quantity: 1, VATRate: pos.VAT10}, 14, 1, 15},  // 1.40 -> 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.Net(); got != tt.wantNet {
				t.Errorf("Net() = %d, want %d", got, tt.wantNet)
			}
			if got := tt.item.VAT(); got != tt.wantVAT {
				t.Errorf("VAT() = %d, want %d", got, tt.wantVAT)
			}
			if got := tt.item.Gross(); got != tt.wantGross {
				t.Errorf("Gross() = %d, want %d", got, tt.wantGross)
			}
		})
	}
}

func TestOrderTotalsMixedVAT(t *testing.T) {
	o := &pos.Order{
		Items: []pos.LineItem{
			{Description: "Pranzo", UnitPrice: 1000, Quantity: 1, VATRate: pos.VAT10}, // net 1000, vat 100
			{Description: "Vino", UnitPrice: 500, Quantity: 2, VATRate: pos.VAT22},    // net 1000, vat 220
		},
		Method: pos.Card,
	}

	if got, want := o.Net(), pos.Cents(2000); got != want {
		t.Errorf("Net() = %d, want %d", got, want)
	}
	if got, want := o.VAT(), pos.Cents(320); got != want {
		t.Errorf("VAT() = %d, want %d", got, want)
	}
	if got, want := o.Gross(), pos.Cents(2320); got != want {
		t.Errorf("Gross() = %d, want %d", got, want)
	}
}

func TestOrderValidate(t *testing.T) {
	valid := []pos.LineItem{{Description: "Caffè", UnitPrice: 110, Quantity: 1, VATRate: pos.VAT22}}

	tests := []struct {
		name    string
		order   pos.Order
		wantErr error
	}{
		{"valid", pos.Order{Items: valid, Method: pos.Cash}, nil},
		{"no items", pos.Order{Items: nil, Method: pos.Cash}, pos.ErrNoItems},
		{"empty description", pos.Order{Items: []pos.LineItem{{Description: "", UnitPrice: 110, Quantity: 1, VATRate: pos.VAT22}}, Method: pos.Cash}, pos.ErrEmptyDescription},
		{"zero price", pos.Order{Items: []pos.LineItem{{Description: "x", UnitPrice: 0, Quantity: 1, VATRate: pos.VAT22}}, Method: pos.Cash}, pos.ErrNonPositivePrice},
		{"zero quantity", pos.Order{Items: []pos.LineItem{{Description: "x", UnitPrice: 110, Quantity: 0, VATRate: pos.VAT22}}, Method: pos.Cash}, pos.ErrNonPositiveQty},
		{"bad VAT rate", pos.Order{Items: []pos.LineItem{{Description: "x", UnitPrice: 110, Quantity: 1, VATRate: pos.VATRate(7)}}, Method: pos.Cash}, pos.ErrUnknownVATRate},
		{"bad payment", pos.Order{Items: valid, Method: pos.PaymentMethod("cheque")}, pos.ErrUnknownPayment},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want errors.Is %v", err, tt.wantErr)
			}
		})
	}
}
